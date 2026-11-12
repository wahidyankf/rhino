package runtime_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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

// Chmod sets the permission bits of a file the scenario's repository holds.
func (d *diskVaultDriver) Chmod(_ context.Context, path, mode string) error {
	var perm uint32
	if _, err := fmt.Sscanf(mode, "%o", &perm); err != nil {
		return err
	}
	return os.Chmod(filepath.Join(d.dir, filepath.FromSlash(path)), os.FileMode(perm))
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
	start, err := fakebw.CallCount(d.log)
	if err != nil {
		return vault.Result{}, err
	}
	restore := setEnvironment(map[string]string{"PATH": d.binDir, "BW_SESSION": d.sessionKey()})
	ports := application.VaultPorts{Vault: runtime.BitwardenVault{}, Files: runtime.DiskVaultFileStore{}}
	outcome := application.ExecuteWithVault(&runtime.DiskTree{Dir: d.dir}, arguments, nil, nil, nil, nil, nil, nil, nil, ports)
	restore()
	argv, err := fakebw.CallsSince(d.log, start)
	if err != nil {
		return vault.Result{}, err
	}
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
		Processes: bwProcesses(argv),
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
	binary := sharedFakeBW(t)
	vault.RunSuite(t, environmentVaultCommandsCorpus, func() (vault.Driver, error) {
		driver, err := openDiskVaultDriver()
		if err != nil {
			return nil, err
		}
		driver.fakeBW = binary
		return driver, nil
	})
}

// sharedFakeBWDir holds the one fake bw build every integration test in this
// binary shares; TestMain removes it after the last test.
var (
	sharedFakeBWOnce sync.Once
	sharedFakeBWDir  string
	sharedFakeBWPath string
	sharedFakeBWErr  error
)

// sharedFakeBW builds the fake bw once per test binary and returns its path.
func sharedFakeBW(t *testing.T) string {
	t.Helper()
	sharedFakeBWOnce.Do(func() {
		dir, err := os.MkdirTemp("", "rhino-fake-bw-")
		if err != nil {
			sharedFakeBWErr = err
			return
		}
		sharedFakeBWDir = dir
		sharedFakeBWPath, sharedFakeBWErr = fakebw.Build(dir)
	})
	if sharedFakeBWErr != nil {
		t.Fatal(sharedFakeBWErr)
	}
	return sharedFakeBWPath
}

// TestMain removes the shared fake build after every test in the binary has run.
func TestMain(m *testing.M) {
	code := m.Run()
	if sharedFakeBWDir != "" {
		_ = os.RemoveAll(sharedFakeBWDir)
	}
	os.Exit(code)
}

// bwProcesses names each bw call a run made as a process of the run. The
// program is bw; the arguments are the call's own, and only the program is printed.
func bwProcesses(argv [][]string) []vault.Process {
	var processes []vault.Process
	for _, call := range argv {
		processes = append(processes, vault.Process{Program: "bw", Args: call})
	}
	return processes
}

// TestCorruptDownloadRefusesThePushThroughTheRealAdapter proves a push whose
// uploaded generation does not read back is refused through the real adapter:
// the uploaded copy is removed, the previous generation is kept, and no value
// reaches the output.
func TestCorruptDownloadRefusesThePushThroughTheRealAdapter(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	ctx := context.Background()
	driver, err := openDiskVaultDriver()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = driver.Close() })
	driver.fakeBW = sharedFakeBW(t)
	const server = "https://vault.example.com"
	const note = "example/fixture/api/local/service-config"
	const configuration = "schema: rhino/repo-config/v2\nenvironment:\n  vault:\n    provider: bitwarden\n" +
		"    server: " + server + "\n    prefix: example/fixture\n    environments: [local, stag, prod]\n" +
		"    entries:\n      - path: config/service.json\n        class: vault\n        form: file\n" +
		"        app: api\n        env: local\n        secret: service-config\n        mode: \"0600\"\n"
	if err := driver.Declare(ctx, configuration); err != nil {
		t.Fatal(err)
	}
	if err := driver.Session(ctx, vault.SessionUnlocked, server); err != nil {
		t.Fatal(err)
	}
	if err := driver.Generation(ctx, note, "service.json.g0001", "marker-service-one"); err != nil {
		t.Fatal(err)
	}
	if err := driver.editVault(func(v *fakebw.Vault) error {
		v.Corrupt = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := driver.Local(ctx, "config/service.json", "marker-service-two"); err != nil {
		t.Fatal(err)
	}
	result, err := driver.Command(ctx, []string{"env", "push", "--overwrite"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 2 {
		t.Fatalf("push exited %d, want 2", result.ExitCode)
	}
	if strings.Contains(result.Stdout+result.Stderr, "marker-service") {
		t.Fatal("a refused push printed a value")
	}
	held, err := driver.Items(ctx, note)
	if err != nil {
		t.Fatal(err)
	}
	if len(held) != 1 || held[0].Kind != "note" {
		t.Fatalf("the vault holds %d items under the note name, want one secure note", len(held))
	}
	if held[0].Attachments["service.json.g0001"] != "marker-service-one" {
		t.Fatal("the previous generation was not kept")
	}
	if _, ok := held[0].Attachments["service.json.g0002"]; ok {
		t.Fatal("the unverified upload was not removed")
	}
}
