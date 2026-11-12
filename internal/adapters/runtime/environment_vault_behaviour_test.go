package runtime_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/runtime"
	"github.com/wahidyankf/rhino/internal/application"
	"github.com/wahidyankf/rhino/tests/bdd/vault"
	"github.com/wahidyankf/rhino/tests/bdd/vault/fakebw"
)

// environmentVaultCorpus is the vault validation feature, relative to this package.
const environmentVaultCorpus = "../../../specs/behaviours/environment-vault-validation.feature"

// environmentVaultCommandsCorpus is the vault command feature, relative to this package.
const environmentVaultCommandsCorpus = "../../../specs/behaviours/environment-vault.feature"

// diskVaultDriver builds one scenario in a real temporary directory: a Git
// repository whose index holds the tracked files, with a .gitignore that names
// the ignored ones. The validator then reads that directory through DiskTree.
type diskVaultDriver struct {
	dir      string
	traceDir string
	trace    *vault.GitTrace
	// Command scenarios add the fake bw, its state and log, and the session.
	fakeBW string
	binDir string
	state  string
	log    string
	noKey  bool
}

// openDiskVaultDriver builds a scenario repository and, beside it, a separate
// directory for the git shim and its log. The shim never sits inside the
// repository under test, so the trace cannot show up as a file the run reads.
func openDiskVaultDriver() (*diskVaultDriver, error) {
	dir, err := os.MkdirTemp("", "rhino-vault-")
	if err != nil {
		return nil, err
	}
	traceDir, err := os.MkdirTemp("", "rhino-vault-trace-")
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	driver := &diskVaultDriver{dir: dir, traceDir: traceDir}
	if err := driver.git(context.Background(), "init", "-q"); err != nil {
		_ = driver.Close()
		return nil, err
	}
	trace, err := vault.NewGitTrace(traceDir)
	if err != nil {
		_ = driver.Close()
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

// ensureVault prepares the fake bw for a command scenario: its state and log,
// and the wrapper that runs it, in a directory of its own.
func (d *diskVaultDriver) ensureVault() error {
	if d.state != "" {
		return nil
	}
	d.binDir = filepath.Join(d.traceDir, "vault-bin")
	d.state = filepath.Join(d.traceDir, "vault-state.json")
	d.log = filepath.Join(d.traceDir, "bw-calls.log")
	fresh := &fakebw.Vault{Status: "unlocked", Blobs: map[string][]byte{}}
	if err := fresh.Save(d.state); err != nil {
		return err
	}
	return fakebw.InstallWrapper(d.binDir, d.fakeBW, d.state, d.log)
}

// editVault loads the fake vault state, applies change, and saves it.
func (d *diskVaultDriver) editVault(change func(*fakebw.Vault) error) error {
	if err := d.ensureVault(); err != nil {
		return err
	}
	current, err := fakebw.Load(d.state)
	if err != nil {
		return err
	}
	if err := change(current); err != nil {
		return err
	}
	return current.Save(d.state)
}

// Session sets the state the fake vault reports. The session key is set in
// the process environment only while a command runs, and never for no-key.
func (d *diskVaultDriver) Session(_ context.Context, state vault.Session, server string) error {
	d.noKey = state == vault.SessionNoKey
	return d.editVault(func(v *fakebw.Vault) error {
		v.Status, v.ServerURL = "unlocked", server
		if state == vault.SessionLocked {
			v.Status = "locked"
		}
		return nil
	})
}

func (d *diskVaultDriver) Login(_ context.Context, name, password string) error {
	return d.editVault(func(v *fakebw.Vault) error {
		v.Create(fakebw.Item{Name: name, Type: 1, Login: &fakebw.Login{Password: &password}})
		return nil
	})
}

func (d *diskVaultDriver) Generation(_ context.Context, name, file, content string) error {
	return d.editVault(func(v *fakebw.Vault) error {
		note, ok := v.NoteNamed(name)
		if !ok {
			note = v.Create(fakebw.Item{Name: name, Type: 2})
		}
		if _, added := v.AddAttachment(note.ID, file, []byte(content)); !added {
			return fmt.Errorf("the note %q vanished before its attachment was added", name)
		}
		return nil
	})
}

func (d *diskVaultDriver) Local(_ context.Context, path, content string) error {
	return d.write(path, content)
}

// Command runs one env command with PATH reduced to the fake bw's directory,
// and the session key set only when the scenario's session has one. It
// fingerprints the repository and the vault either side of the run.
func (d *diskVaultDriver) Command(_ context.Context, arguments []string) (vault.Result, error) {
	if err := d.ensureVault(); err != nil {
		return vault.Result{}, err
	}
	localBefore, err := vault.DigestTree(d.dir)
	if err != nil {
		return vault.Result{}, err
	}
	vaultBefore, err := digestFile(d.state)
	if err != nil {
		return vault.Result{}, err
	}
	restore := setEnvironment(map[string]string{"PATH": d.binDir, "BW_SESSION": d.sessionKey()})
	ports := application.VaultPorts{Vault: runtime.BitwardenVault{}, Files: runtime.DiskVaultFileStore{}}
	outcome := application.ExecuteWithVault(&runtime.DiskTree{Dir: d.dir}, arguments, nil, nil, nil, nil, nil, nil, nil, ports)
	restore()
	localAfter, err := vault.DigestTree(d.dir)
	if err != nil {
		return vault.Result{}, err
	}
	vaultAfter, err := digestFile(d.state)
	if err != nil {
		return vault.Result{}, err
	}
	return vault.Result{
		ExitCode: outcome.ExitCode, Stdout: outcome.Stdout, Stderr: outcome.Stderr,
		LocalBefore: localBefore, LocalAfter: localAfter, VaultBefore: vaultBefore, VaultAfter: vaultAfter,
	}, nil
}

// sessionKey is the session key a run sees: empty for a no-key session, which
// removes the variable.
func (d *diskVaultDriver) sessionKey() string {
	if d.noKey {
		return ""
	}
	return "fixture-session-key"
}

// setEnvironment sets each variable, an empty value unsetting it, and returns
// the function that restores every variable it changed.
func setEnvironment(values map[string]string) func() {
	type saved struct {
		value string
		set   bool
	}
	previous := map[string]saved{}
	for name, value := range values {
		old, set := os.LookupEnv(name)
		previous[name] = saved{value: old, set: set}
		if value == "" {
			_ = os.Unsetenv(name)
		} else {
			_ = os.Setenv(name, value)
		}
	}
	return func() {
		for name, old := range previous {
			if old.set {
				_ = os.Setenv(name, old.value)
			} else {
				_ = os.Unsetenv(name)
			}
		}
	}
}

// digestFile fingerprints the fake vault's state file.
func digestFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read fake vault state: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (d *diskVaultDriver) LocalFile(_ context.Context, path string) (vault.LocalFile, error) {
	full := filepath.Join(d.dir, filepath.FromSlash(path))
	info, err := os.Stat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return vault.LocalFile{}, nil
		}
		return vault.LocalFile{}, err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return vault.LocalFile{}, err
	}
	return vault.LocalFile{Exists: true, Content: string(data), Mode: fmt.Sprintf("%04o", info.Mode().Perm())}, nil
}

func (d *diskVaultDriver) Items(_ context.Context, name string) ([]vault.ItemView, error) {
	if err := d.ensureVault(); err != nil {
		return nil, err
	}
	current, err := fakebw.Load(d.state)
	if err != nil {
		return nil, err
	}
	return current.Views(name), nil
}

// Close removes the scenario repository and the shim directory beside it.
func (d *diskVaultDriver) Close() error {
	repoErr := os.RemoveAll(d.dir)
	traceErr := os.RemoveAll(d.traceDir)
	if repoErr != nil {
		return repoErr
	}
	return traceErr
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

// TestEnvironmentVaultCommandsAtIntegrationAdapter runs every scenario of the
// vault command corpus against a real temporary Git repository and the fake bw.
func TestEnvironmentVaultCommandsAtIntegrationAdapter(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	binary, err := fakebw.Build(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	vault.RunSuite(t, environmentVaultCommandsCorpus, func() (vault.Driver, error) {
		driver, err := openDiskVaultDriver()
		if err != nil {
			return nil, err
		}
		driver.fakeBW = binary
		return driver, nil
	})
}
