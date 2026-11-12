package runtime_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/runtime"
	"github.com/wahidyankf/rhino/internal/application"
	"github.com/wahidyankf/rhino/tests/bdd/vault"
)

// environmentVaultCorpus is the vault validation feature, relative to this package.
const environmentVaultCorpus = "../../../specs/behaviours/environment-vault-validation.feature"

// diskVaultDriver builds one scenario in a real temporary directory: a Git
// repository whose index holds the tracked files, with a .gitignore that names
// the ignored ones. The validator then reads that directory through DiskTree.
type diskVaultDriver struct {
	dir   string
	trace *vault.GitTrace
}

func openDiskVaultDriver() (*diskVaultDriver, error) {
	dir, err := os.MkdirTemp("", "rhino-vault-")
	if err != nil {
		return nil, err
	}
	driver := &diskVaultDriver{dir: dir}
	if err := driver.git(context.Background(), "init", "-q"); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	trace, err := vault.NewGitTrace(dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	driver.trace = trace
	return driver, nil
}

// git runs one Git command inside the scenario repository.
func (d *diskVaultDriver) git(ctx context.Context, args ...string) error {
	output, err := exec.CommandContext(ctx, "git", append([]string{"-C", d.dir}, args...)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v: %w: %s", args, err, output)
	}
	return nil
}

// write creates a repository-relative file with content.
func (d *diskVaultDriver) write(path, content string) error {
	full := filepath.Join(d.dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(content), 0o644)
}

func (d *diskVaultDriver) Declare(ctx context.Context, configuration string) error {
	if err := d.write("repo-config.yml", configuration); err != nil {
		return err
	}
	return d.git(ctx, "add", "--", "repo-config.yml")
}

func (d *diskVaultDriver) Track(ctx context.Context, path, content string) error {
	if err := d.write(path, content); err != nil {
		return err
	}
	return d.git(ctx, "add", "--", path)
}

func (d *diskVaultDriver) Ignore(_ context.Context, path, content string) error {
	if err := d.write(path, content); err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(d.dir, ".gitignore"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.WriteString("/" + path + "\n"); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// Validate runs the validation with PATH reduced to the git trace, so the run
// can start git and no other program found by name. The trace records the git
// invocations, which the result carries.
func (d *diskVaultDriver) Validate(_ context.Context, asJSON bool) (vault.Result, error) {
	args := []string{"env", "validate"}
	if asJSON {
		args = append(args, "--output", "json")
	}
	if err := d.trace.Reset(); err != nil {
		return vault.Result{}, err
	}
	previous := os.Getenv("PATH")
	if err := os.Setenv("PATH", d.trace.PATH()); err != nil {
		return vault.Result{}, err
	}
	outcome := application.Execute(&runtime.DiskTree{Dir: d.dir}, args, nil)
	if err := os.Setenv("PATH", previous); err != nil {
		return vault.Result{}, err
	}
	processes, err := d.trace.Processes()
	if err != nil {
		return vault.Result{}, err
	}
	return vault.Result{ExitCode: outcome.ExitCode, Stdout: outcome.Stdout, Stderr: outcome.Stderr, Processes: processes}, nil
}

func (d *diskVaultDriver) Close() error {
	return os.RemoveAll(d.dir)
}

// TestEnvironmentVaultValidationAtIntegrationAdapter runs every scenario of the
// vault validation corpus against a real temporary Git repository.
func TestEnvironmentVaultValidationAtIntegrationAdapter(t *testing.T) {
	// The developer's Git configuration must not change a scenario's result.
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	vault.RunSuite(t, environmentVaultCorpus, func() (vault.Driver, error) {
		driver, err := openDiskVaultDriver()
		if err != nil {
			return nil, err
		}
		return driver, nil
	})
}
