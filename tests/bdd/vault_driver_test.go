package bdd_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wahidyankf/rhino/tests/bdd/vault"
	"github.com/wahidyankf/rhino/tests/bdd/vault/fakebw"
)

// sessionKey is the session key a scenario's unlocked session carries. The
// fake vault checks only that one is present, so the value is a fixed label.
const sessionKey = "fixture-session-key"

// cliVaultDriver builds the environment vault scenarios in the scenario's own
// repository and runs the built rhino binary there, observing only argv, exit
// code, and streams. The vault is the fake bw command, installed as the only
// vault executable the run can find.
type cliVaultDriver struct {
	world *world
}

// vaultState and vaultLog are the scenario's fake vault state file and the
// log of the bw calls it received. Both live beside the repository.
func (d *cliVaultDriver) vaultState() string { return filepath.Join(d.world.dir, "vault-state.json") }
func (d *cliVaultDriver) vaultLog() string   { return filepath.Join(d.world.logs, "bw-calls.log") }

// ensureVault creates the fake vault state on first use and installs the bw
// wrapper that runs it.
func (d *cliVaultDriver) ensureVault() error {
	if _, err := os.Stat(d.vaultState()); err == nil {
		return nil
	}
	fresh := &fakebw.Vault{Status: "unlocked", Blobs: map[string][]byte{}}
	if err := fresh.Save(d.vaultState()); err != nil {
		return err
	}
	return fakebw.InstallWrapper(d.world.bin, fakeBWBinary, d.vaultState(), d.vaultLog())
}

// editVault loads the fake vault, applies change, and saves it.
func (d *cliVaultDriver) editVault(change func(*fakebw.Vault) error) error {
	if err := d.ensureVault(); err != nil {
		return err
	}
	current, err := fakebw.Load(d.vaultState())
	if err != nil {
		return err
	}
	if err := change(current); err != nil {
		return err
	}
	return current.Save(d.vaultState())
}

func (d *cliVaultDriver) Declare(ctx context.Context, configuration string) error {
	if err := d.world.ensureGit(ctx); err != nil {
		return err
	}
	d.world.config = configuration
	return d.world.writeRepoFile("repo-config.yml", configuration)
}

func (d *cliVaultDriver) Track(ctx context.Context, path, content string) error {
	if err := d.world.writeRepoFile(path, content); err != nil {
		return err
	}
	return d.world.indexSet(ctx, path, content)
}

// Ignore writes the file and names it in the repository's .gitignore, so Git
// reports it as ignored and untracked.
func (d *cliVaultDriver) Ignore(ctx context.Context, path, content string) error {
	if err := d.world.ensureGit(ctx); err != nil {
		return err
	}
	if err := d.world.writeRepoFile(path, content); err != nil {
		return err
	}
	existing, err := d.world.workingContent(".gitignore")
	if err != nil {
		return err
	}
	return d.world.writeRepoFile(".gitignore", existing+"/"+path+"\n")
}

// Validate runs the built binary with PATH reduced to a git shim that logs each
// invocation, so the run can start git and no other program found by name.
func (d *cliVaultDriver) Validate(ctx context.Context, asJSON bool) (vault.Result, error) {
	arguments := "env|validate"
	if asJSON {
		arguments += "|--output|json"
	}
	trace, err := vault.NewGitTrace(d.world.dir)
	if err != nil {
		return vault.Result{}, err
	}
	d.world.pathOnly = trace.PATH()
	err = d.world.invoke(ctx, arguments)
	d.world.pathOnly = ""
	if err != nil {
		return vault.Result{}, err
	}
	processes, err := trace.Processes()
	if err != nil {
		return vault.Result{}, err
	}
	run, err := d.world.lastRun()
	if err != nil {
		return vault.Result{}, err
	}
	return vault.Result{ExitCode: run.exitCode, Stdout: run.stdout, Stderr: run.stderr, Processes: processes}, nil
}

// Executable makes the fake bw absent from the scenario's bin directory, or
// present there without the execute bit, so the run's PATH meets that state.
func (d *cliVaultDriver) Executable(_ context.Context, state vault.Executable) error {
	if err := d.ensureVault(); err != nil {
		return err
	}
	bw := filepath.Join(d.world.bin, "bw")
	switch state {
	case vault.ExecutableMissing:
		return os.Remove(bw)
	case vault.ExecutableNotRunnable:
		return os.Chmod(bw, 0o644)
	}
	return fmt.Errorf("unknown executable state %q", state)
}

// Session sets the state the fake vault reports, and the session key the run
// may see. No key removes the variable, so the adapter has nothing to unlock.
func (d *cliVaultDriver) Session(_ context.Context, state vault.Session, server string) error {
	if err := d.editVault(func(v *fakebw.Vault) error {
		v.Status, v.ServerURL = "unlocked", server
		if state == vault.SessionLocked {
			v.Status = "locked"
		}
		return nil
	}); err != nil {
		return err
	}
	if d.world.extraEnv == nil {
		d.world.extraEnv = map[string]string{}
	}
	d.world.extraEnv["BW_SESSION"] = sessionKey
	if state == vault.SessionNoKey {
		d.world.extraEnv["BW_SESSION"] = ""
	}
	return nil
}

func (d *cliVaultDriver) Login(_ context.Context, name, password string) error {
	return d.editVault(func(v *fakebw.Vault) error {
		v.Create(fakebw.Item{Name: name, Type: 1, Login: &fakebw.Login{Password: &password}})
		return nil
	})
}

// Generation adds one attachment to the secure note called name, creating the
// note the first time a name is seen.
func (d *cliVaultDriver) Generation(_ context.Context, name, file, content string) error {
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

// Chmod sets the permission bits of a file in the scenario's repository.
func (d *cliVaultDriver) Chmod(_ context.Context, path, mode string) error {
	var perm uint32
	if _, err := fmt.Sscanf(mode, "%o", &perm); err != nil {
		return err
	}
	file, err := d.world.repoPath(path)
	if err != nil {
		return err
	}
	return os.Chmod(file, os.FileMode(perm))
}

func (d *cliVaultDriver) Local(_ context.Context, path, content string) error {
	return d.world.writeRepoFile(path, content)
}

// Command runs one env command with the scenario's vault on PATH alone, and
// records the repository and the vault state either side of the run.
func (d *cliVaultDriver) Command(ctx context.Context, arguments []string) (vault.Result, error) {
	if err := d.ensureVault(); err != nil {
		return vault.Result{}, err
	}
	vaultBefore, err := digestFile(d.vaultState())
	if err != nil {
		return vault.Result{}, err
	}
	start, err := fakebw.CallCount(d.vaultLog())
	if err != nil {
		return vault.Result{}, err
	}
	d.world.pathOnly = d.world.bin
	err = d.world.invoke(ctx, strings.Join(arguments, "|"))
	d.world.pathOnly = ""
	if err != nil {
		return vault.Result{}, err
	}
	argv, err := fakebw.CallsSince(d.vaultLog(), start)
	if err != nil {
		return vault.Result{}, err
	}
	vaultAfter, err := digestFile(d.vaultState())
	if err != nil {
		return vault.Result{}, err
	}
	run, err := d.world.lastRun()
	if err != nil {
		return vault.Result{}, err
	}
	return vault.Result{
		ExitCode: run.exitCode, Stdout: run.stdout, Stderr: run.stderr,
		LocalBefore: digestSnapshot(run.before), LocalAfter: digestSnapshot(run.after),
		VaultBefore: vaultBefore, VaultAfter: vaultAfter,
		Processes: bwProcesses(argv),
	}, nil
}

// digestSnapshot fingerprints a repository snapshot taken by the world.
func digestSnapshot(snapshot snapshot) string {
	sum := sha256.Sum256([]byte(fmt.Sprint(map[string]string(snapshot))))
	return hex.EncodeToString(sum[:])
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

func (d *cliVaultDriver) LocalFile(_ context.Context, path string) (vault.LocalFile, error) {
	full, err := d.world.repoPath(path)
	if err != nil {
		return vault.LocalFile{}, err
	}
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

// Items reads every item held under name from the fake vault's state.
func (d *cliVaultDriver) Items(_ context.Context, name string) ([]vault.ItemView, error) {
	if err := d.ensureVault(); err != nil {
		return nil, err
	}
	current, err := fakebw.Load(d.vaultState())
	if err != nil {
		return nil, err
	}
	return current.Views(name), nil
}

// Close has nothing to remove: the scenario world the corpus creates owns the
// directory and removes it after the scenario.
func (d *cliVaultDriver) Close() error { return nil }

// bwProcesses names each bw call a run made as a process of the run.
func bwProcesses(argv [][]string) []vault.Process {
	var processes []vault.Process
	for _, call := range argv {
		processes = append(processes, vault.Process{Program: "bw", Args: call})
	}
	return processes
}
