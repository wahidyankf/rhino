package application_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
	"github.com/wahidyankf/rhino/internal/domain"
	"github.com/wahidyankf/rhino/tests/bdd/vault"
	"github.com/wahidyankf/rhino/tests/bdd/vault/fakebw"
)

// environmentVaultCorpus is the vault validation feature, relative to this package.
const environmentVaultCorpus = "../../specs/behaviours/environment-vault-validation.feature"

// environmentVaultCommandsCorpus is the vault command feature, relative to this package.
const environmentVaultCommandsCorpus = "../../specs/behaviours/environment-vault.feature"

// vaultMemoryTree is the in-memory repository a unit scenario runs against. Its
// two Git name queries answer from the tracked and ignored paths the scenario
// declares, so no process starts and no file is read from disk.
type vaultMemoryTree struct {
	*memoryTree
	tracked []string
	ignored []string
	asked   []vault.Process
}

// TrackedFiles answers the tracked-name query from the declared paths, and
// records the git invocation the disk tree would have started for it.
func (m *vaultMemoryTree) TrackedFiles() ([]string, error) {
	m.asked = append(m.asked, vault.Process{Program: "git", Args: []string{"-C", "/repository", "ls-files", "-z", "--"}})
	return m.tracked, nil
}

// IgnoredFiles answers the ignored-name query from the declared paths, and
// records the git invocation the disk tree would have started for it.
func (m *vaultMemoryTree) IgnoredFiles() ([]string, error) {
	m.asked = append(m.asked, vault.Process{Program: "git", Args: []string{
		"-C", "/repository", "ls-files", "--others", "--ignored", "--exclude-standard", "-z", "--"}})
	return m.ignored, nil
}

// unitVaultDriver builds one scenario in memory: the repository tree, and a
// fake vault held in process, with the same state the command-line fake keeps.
type unitVaultDriver struct {
	tree  *vaultMemoryTree
	vault *fakebw.Vault
	noKey bool
	// calls records each provider call the run made, as the process record.
	calls []vault.Process
}

func newUnitVaultDriver() *unitVaultDriver {
	return &unitVaultDriver{
		tree:  &vaultMemoryTree{memoryTree: newMemoryTree("/repository")},
		vault: &fakebw.Vault{Status: "unlocked", Blobs: map[string][]byte{}},
	}
}

func (d *unitVaultDriver) Declare(_ context.Context, configuration string) error {
	d.tree.files["repo-config.yml"] = configuration
	d.tree.tracked = append(d.tree.tracked, "repo-config.yml")
	return nil
}

func (d *unitVaultDriver) Track(_ context.Context, path, content string) error {
	d.tree.files[path] = content
	d.tree.tracked = append(d.tree.tracked, path)
	return nil
}

func (d *unitVaultDriver) Ignore(_ context.Context, path, content string) error {
	d.tree.files[path] = content
	d.tree.ignored = append(d.tree.ignored, path)
	return nil
}

func (d *unitVaultDriver) Validate(_ context.Context, asJSON bool) (vault.Result, error) {
	args := []string{"env", "validate"}
	if asJSON {
		args = append(args, "--output", "json")
	}
	d.tree.asked = nil
	outcome := application.Execute(d.tree, args, nil)
	return vault.Result{ExitCode: outcome.ExitCode, Stdout: outcome.Stdout, Stderr: outcome.Stderr,
		Processes: d.tree.asked}, nil
}

// Session sets the state the in-memory vault reports. No key is a session the
// command cannot unlock, which the adapter refuses before any call.
func (d *unitVaultDriver) Session(_ context.Context, state vault.Session, server string) error {
	d.vault.Status, d.vault.ServerURL = "unlocked", server
	if state == vault.SessionLocked {
		d.vault.Status = "locked"
	}
	d.noKey = state == vault.SessionNoKey
	return nil
}

func (d *unitVaultDriver) Login(_ context.Context, name, password string) error {
	d.vault.Create(fakebw.Item{Name: name, Type: 1, Login: &fakebw.Login{Password: &password}})
	return nil
}

func (d *unitVaultDriver) Generation(_ context.Context, name, file, content string) error {
	note, ok := d.vault.NoteNamed(name)
	if !ok {
		note = d.vault.Create(fakebw.Item{Name: name, Type: 2})
	}
	if _, added := d.vault.AddAttachment(note.ID, file, []byte(content)); !added {
		return fmt.Errorf("the note %q vanished before its attachment was added", name)
	}
	return nil
}

// Chmod sets the mode the in-memory tree reports for a file.
func (d *unitVaultDriver) Chmod(_ context.Context, path, mode string) error {
	var perm uint32
	if _, err := fmt.Sscanf(mode, "%o", &perm); err != nil {
		return err
	}
	d.tree.modes[path] = perm
	return nil
}

func (d *unitVaultDriver) Local(_ context.Context, path, content string) error {
	d.tree.files[path] = content
	d.tree.modes[path] = 0o644
	return nil
}

// Command runs one env command against the in-memory tree and vault, and
// fingerprints both either side of the run.
func (d *unitVaultDriver) Command(_ context.Context, arguments []string) (vault.Result, error) {
	localBefore, vaultBefore := d.localDigest(), d.vault.Digest()
	d.calls = nil
	ports := application.VaultPorts{
		Vault: &memorySecretVault{vault: d.vault, noKey: d.noKey, calls: &d.calls},
		Files: memoryFileStore{tree: d.tree},
	}
	outcome := application.ExecuteWithVault(d.tree, arguments, nil, nil, nil, nil, nil, nil, nil, ports)
	return vault.Result{
		ExitCode: outcome.ExitCode, Stdout: outcome.Stdout, Stderr: outcome.Stderr,
		LocalBefore: localBefore, LocalAfter: d.localDigest(),
		VaultBefore: vaultBefore, VaultAfter: d.vault.Digest(), Processes: d.calls,
	}, nil
}

// localDigest fingerprints every in-memory file and its mode, sorted by path.
func (d *unitVaultDriver) localDigest() string {
	var paths []string
	for path := range d.tree.files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		fmt.Fprintf(hash, "%s %o %q\n", path, d.tree.modes[path], d.tree.files[path])
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (d *unitVaultDriver) LocalFile(_ context.Context, path string) (vault.LocalFile, error) {
	content, ok := d.tree.files[path]
	if !ok {
		return vault.LocalFile{}, nil
	}
	return vault.LocalFile{Exists: true, Content: content, Mode: fmt.Sprintf("%04o", d.tree.modes[path])}, nil
}

func (d *unitVaultDriver) Items(_ context.Context, name string) ([]vault.ItemView, error) {
	return d.vault.Views(name), nil
}

func (d *unitVaultDriver) Close() error { return nil }

// memorySecretVault serves the provider boundary from the scenario's in-process
// fake vault. It answers the same way the command-line fake does: a search is a
// substring match, an edit keeps history, and a session without a key is refused.
type memorySecretVault struct {
	vault *fakebw.Vault
	noKey bool
	calls *[]vault.Process
}

// record notes one provider call the run made. The double starts no process;
// the record stands for the bw call the adapter would start.
func (m *memorySecretVault) record(method string) {
	if m.calls != nil {
		*m.calls = append(*m.calls, vault.Process{Program: "bw", Args: []string{method}})
	}
}

func (m *memorySecretVault) Status(server string) *application.VaultFault {
	if !m.noKey {
		m.record("status")
	}
	switch {
	case m.noKey:
		return &application.VaultFault{Code: domain.VaultUnavailable, Reason: "no session key"}
	case m.vault.Status != "unlocked":
		return &application.VaultFault{Code: domain.VaultUnavailable, Reason: "the vault is not unlocked"}
	case m.vault.ServerURL != server:
		return &application.VaultFault{Code: domain.VaultUnavailable, Reason: "another server"}
	}
	return nil
}

func (m *memorySecretVault) Find(name string) ([]application.VaultItem, *application.VaultFault) {
	m.record("find")
	var found []application.VaultItem
	for _, item := range m.vault.Search(name) {
		found = append(found, memoryItemView(item))
	}
	return found, nil
}

func (m *memorySecretVault) CreateLogin(name, password string) (application.VaultItem, *application.VaultFault) {
	m.record("create-login")
	return memoryItemView(m.vault.Create(fakebw.Item{Name: name, Type: 1, Login: &fakebw.Login{Password: &password}})), nil
}

func (m *memorySecretVault) UpdateLogin(item application.VaultItem, password string) *application.VaultFault {
	m.record("edit-login")
	current, ok := m.vault.Get(item.ID)
	if !ok {
		return &application.VaultFault{Code: domain.VaultUnavailable, Reason: "item not found"}
	}
	current.Login = &fakebw.Login{Password: &password}
	if !m.vault.Edit(current) {
		return &application.VaultFault{Code: domain.VaultUnavailable, Reason: "item not found"}
	}
	return nil
}

func (m *memorySecretVault) CreateNote(name string) (application.VaultItem, *application.VaultFault) {
	m.record("create-note")
	return memoryItemView(m.vault.Create(fakebw.Item{Name: name, Type: 2})), nil
}

func (m *memorySecretVault) AddAttachment(itemID, fileName string, contents []byte) (application.VaultAttachment, *application.VaultFault) {
	m.record("create-attachment")
	item, ok := m.vault.AddAttachment(itemID, fileName, contents)
	if !ok {
		return application.VaultAttachment{}, &application.VaultFault{Code: domain.VaultUnavailable, Reason: "item not found"}
	}
	var named application.VaultAttachment
	for _, attachment := range item.Attachments {
		if attachment.FileName == fileName {
			named = application.VaultAttachment{ID: attachment.ID, FileName: attachment.FileName}
		}
	}
	return named, nil
}

func (m *memorySecretVault) ReadAttachment(itemID, attachmentID string) ([]byte, *application.VaultFault) {
	m.record("get-attachment")
	data, ok := m.vault.ReadAttachment(itemID, attachmentID)
	if !ok {
		return nil, &application.VaultFault{Code: domain.VaultRefused, Reason: "attachment not found"}
	}
	return data, nil
}

func (m *memorySecretVault) DeleteAttachment(itemID, attachmentID string) *application.VaultFault {
	m.record("delete-attachment")
	if !m.vault.DeleteAttachment(itemID, attachmentID) {
		return &application.VaultFault{Code: domain.VaultUnavailable, Reason: "attachment not found"}
	}
	return nil
}

// memoryItemView maps one fake item to the application's view of it.
func memoryItemView(item fakebw.Item) application.VaultItem {
	view := application.VaultItem{ID: item.ID, Name: item.Name, Kind: "other"}
	switch item.Type {
	case 1:
		view.Kind = "login"
		if item.Login == nil || item.Login.Password == nil {
			view.NoPassword = true
		} else {
			view.Password = *item.Login.Password
		}
	case 2:
		view.Kind = "note"
	}
	for _, attachment := range item.Attachments {
		view.Attachments = append(view.Attachments, application.VaultAttachment{ID: attachment.ID, FileName: attachment.FileName})
	}
	return view
}

// memoryFileStore writes a pulled file into the in-memory tree, refusing an
// existing target unless the write replaces it, as the disk store does.
type memoryFileStore struct {
	tree *vaultMemoryTree
}

func (s memoryFileStore) Write(_ string, file application.VaultFile) error {
	if _, exists := s.tree.files[file.Path]; exists && !file.Replace {
		return fmt.Errorf("`%s` already exists", file.Path)
	}
	s.tree.files[file.Path] = string(file.Contents)
	s.tree.modes[file.Path] = file.Mode
	return nil
}

// TestEnvironmentVaultValidationAtUnitAdapter runs every scenario of the vault
// validation corpus in process, against the in-memory tree.
func TestEnvironmentVaultValidationAtUnitAdapter(t *testing.T) {
	vault.RunSuite(t, environmentVaultCorpus, func() (vault.Driver, error) {
		return newUnitVaultDriver(), nil
	})
}

// TestEnvironmentVaultCommandsAtUnitAdapter runs every scenario of the vault
// command corpus in process, against the in-memory tree and vault.
func TestEnvironmentVaultCommandsAtUnitAdapter(t *testing.T) {
	vault.RunSuite(t, environmentVaultCommandsCorpus, func() (vault.Driver, error) {
		return newUnitVaultDriver(), nil
	})
}
