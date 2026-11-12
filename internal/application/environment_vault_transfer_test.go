package application_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
	"github.com/wahidyankf/rhino/internal/domain"
	"github.com/wahidyankf/rhino/tests/bdd/vault/fakebw"
)

// The declaration every transfer test starts from: one env entry and one file
// entry under one prefix. A test adds or replaces entries with fixtureConfig.
const (
	fixtureVaultHead = "schema: rhino/repo-config/v2\nenvironment:\n  vault:\n    provider: bitwarden\n" +
		"    server: https://vault.example.com\n    prefix: example/fixture\n" +
		"    environments: [local, stag, prod]\n    entries:\n"
	fixtureEnvEntry = "      - path: config/app.env\n        class: vault\n        form: env\n        app: web\n" +
		"        env: local\n        mode: \"0600\"\n        keys: [ALPHA_KEY, BETA_KEY]\n"
	fixtureFileEntry = "      - path: config/service.json\n        class: vault\n        form: file\n        app: api\n" +
		"        env: local\n        secret: service-config\n        mode: \"0600\"\n"
	fixtureServer = "https://vault.example.com"
)

// fixtureConfig declares the vault with the given entries.
func fixtureConfig(entries ...string) string {
	return fixtureVaultHead + strings.Join(entries, "")
}

// transferTree is an in-memory repository holding the declaration.
func transferTree(configuration string) *memoryTree {
	tree := newMemoryTree("/repository")
	tree.files["repo-config.yml"] = configuration
	return tree
}

// fixtureVault is an unlocked vault serving the declared server.
func fixtureVault() (*fakebw.Vault, *memorySecretVault) {
	v := &fakebw.Vault{Status: "unlocked", ServerURL: fixtureServer, Blobs: map[string][]byte{}}
	return v, &memorySecretVault{vault: v}
}

// recordingStore is a file store that writes into the in-memory tree, records
// every write it accepts, and fails with err when set.
type recordingStore struct {
	tree   *memoryTree
	writes []application.VaultFile
	err    error
}

func (s *recordingStore) Write(_ string, file application.VaultFile) error {
	if s.err != nil {
		return s.err
	}
	if _, exists := s.tree.files[file.Path]; exists && !file.Replace {
		return fmt.Errorf("`%s` already exists", file.Path)
	}
	s.tree.files[file.Path] = string(file.Contents)
	s.writes = append(s.writes, file)
	return nil
}

// faultyVault answers from the wrapped vault, except for each method named in
// fail, which returns that fault instead. Calls it answers are counted, so a
// test can prove a refused run made no provider change.
type faultyVault struct {
	inner application.SecretVault
	fail  map[string]*application.VaultFault
	calls map[string]int
}

func newFaultyVault(inner application.SecretVault, fail map[string]*application.VaultFault) *faultyVault {
	return &faultyVault{inner: inner, fail: fail, calls: map[string]int{}}
}

func (f *faultyVault) fault(method string) *application.VaultFault {
	f.calls[method]++
	return f.fail[method]
}

func (f *faultyVault) Status(server string) *application.VaultFault {
	if fault := f.fault("Status"); fault != nil {
		return fault
	}
	return f.inner.Status(server)
}

func (f *faultyVault) Find(name string) ([]application.VaultItem, *application.VaultFault) {
	if fault := f.fault("Find"); fault != nil {
		return nil, fault
	}
	return f.inner.Find(name)
}

func (f *faultyVault) CreateLogin(name, password string) (application.VaultItem, *application.VaultFault) {
	if fault := f.fault("CreateLogin"); fault != nil {
		return application.VaultItem{}, fault
	}
	return f.inner.CreateLogin(name, password)
}

func (f *faultyVault) UpdateLogin(item application.VaultItem, password string) *application.VaultFault {
	if fault := f.fault("UpdateLogin"); fault != nil {
		return fault
	}
	return f.inner.UpdateLogin(item, password)
}

func (f *faultyVault) CreateNote(name string) (application.VaultItem, *application.VaultFault) {
	if fault := f.fault("CreateNote"); fault != nil {
		return application.VaultItem{}, fault
	}
	return f.inner.CreateNote(name)
}

func (f *faultyVault) AddAttachment(itemID, fileName string, contents []byte) (application.VaultAttachment, *application.VaultFault) {
	if fault := f.fault("AddAttachment"); fault != nil {
		return application.VaultAttachment{}, fault
	}
	return f.inner.AddAttachment(itemID, fileName, contents)
}

func (f *faultyVault) ReadAttachment(itemID, attachmentID string) ([]byte, *application.VaultFault) {
	if fault := f.fault("ReadAttachment"); fault != nil {
		return nil, fault
	}
	return f.inner.ReadAttachment(itemID, attachmentID)
}

func (f *faultyVault) DeleteAttachment(itemID, attachmentID string) *application.VaultFault {
	if fault := f.fault("DeleteAttachment"); fault != nil {
		return fault
	}
	return f.inner.DeleteAttachment(itemID, attachmentID)
}

// runTransfer runs one env command with the given ports.
func runTransfer(tree *memoryTree, vault application.SecretVault, files application.VaultFileStore, args ...string) application.Outcome {
	return application.ExecuteWithVault(tree, args, nil, nil, nil, nil, nil, nil, nil,
		application.VaultPorts{Vault: vault, Files: files})
}

// addLogin stores a login item with password under name.
func addLogin(v *fakebw.Vault, name, password string) {
	v.Create(fakebw.Item{Name: name, Type: 1, Login: &fakebw.Login{Password: &password}})
}

// addNote stores a secure note under name, holding one attachment per entry of
// generations, keyed by file name.
func addNote(v *fakebw.Vault, name string, generations map[string]string) fakebw.Item {
	note := v.Create(fakebw.Item{Name: name, Type: 2})
	for _, fileName := range sortedKeysOf(generations) {
		note, _ = v.AddAttachment(note.ID, fileName, []byte(generations[fileName]))
	}
	return note
}

func sortedKeysOf(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// expectRefusal asserts exit 2 and a non-empty refusal. A JSON refusal also
// names its closed code, which a text refusal does not print by design.
func expectRefusal(t *testing.T, outcome application.Outcome, code domain.ErrorCode) {
	t.Helper()
	if outcome.ExitCode != 2 {
		t.Fatalf("exit = %d, want 2; stdout %q stderr %q", outcome.ExitCode, outcome.Stdout, outcome.Stderr)
	}
	if outcome.Stderr == "" {
		t.Fatalf("a refusal must explain itself; stderr is empty")
	}
	if strings.HasPrefix(outcome.Stderr, "{") && !strings.Contains(outcome.Stderr, code.String()) {
		t.Fatalf("refusal does not name %s: %q", code, outcome.Stderr)
	}
}

// TestEnvironmentVaultRefusesBeforeAnyVaultCall proves each refusal that the
// invocation, the manifest, or the wiring causes happens before the vault is consulted.
func TestEnvironmentVaultRefusesBeforeAnyVaultCall(t *testing.T) {
	cases := []struct {
		name   string
		config string
		ports  func(*memorySecretVault) application.VaultPorts
		args   []string
		code   domain.ErrorCode
	}{
		{
			name:   "no vault declared",
			config: "schema: rhino/repo-config/v2\n",
			args:   []string{"env", "pull", "--output", "json"},
			code:   domain.ConfigUndeclared,
		},
		{
			name:   "vault is not a mapping",
			config: "schema: rhino/repo-config/v2\nenvironment:\n  vault: 5\n",
			args:   []string{"env", "pull", "--output", "json"},
			code:   domain.ConfigUnusable,
		},
		{
			name:   "manifest breaks a rule",
			config: strings.Replace(fixtureConfig(fixtureEnvEntry), "provider: bitwarden", "provider: other", 1),
			args:   []string{"env", "pull", "--output", "json"},
			code:   domain.ConfigUnusable,
		},
		{
			name: "field override this build does not read",
			config: fixtureConfig("      - path: config/app.env\n        class: vault\n        form: env\n        app: web\n" +
				"        env: local\n        keys:\n          - key: ALPHA_KEY\n            field: notes\n"),
			args: []string{"env", "pull", "--output", "json"},
			code: domain.ConfigUnusable,
		},
		{
			name:   "entry names no declared vault entry",
			config: fixtureConfig(fixtureEnvEntry),
			args:   []string{"env", "diff", "--entry", "config/other.env", "--output", "json"},
			code:   domain.ConfigUndeclared,
		},
		{
			name: "entry names a derived entry",
			config: fixtureConfig(fixtureEnvEntry,
				"      - path: config/derived.env\n        class: derived\n        reason: built by a tool\n        by: tool\n"),
			args: []string{"env", "diff", "--entry", "config/derived.env", "--output", "json"},
			code: domain.ConfigUndeclared,
		},
		{
			name:   "no vault provider is wired",
			config: fixtureConfig(fixtureEnvEntry),
			ports:  func(*memorySecretVault) application.VaultPorts { return application.VaultPorts{} },
			args:   []string{"env", "push", "--output", "json"},
			code:   domain.VaultUnavailable,
		},
		{
			name:   "pull without a local file store",
			config: fixtureConfig(fixtureEnvEntry),
			ports: func(v *memorySecretVault) application.VaultPorts {
				return application.VaultPorts{Vault: v}
			},
			args: []string{"env", "pull", "--output", "json"},
			code: domain.FileUnwritable,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, vault := fixtureVault()
			tree := transferTree(c.config)
			var ports application.VaultPorts
			if c.ports != nil {
				ports = c.ports(vault)
			} else {
				ports = application.VaultPorts{Vault: vault, Files: &recordingStore{tree: tree}}
			}
			outcome := application.ExecuteWithVault(tree, c.args, nil, nil, nil, nil, nil, nil, nil, ports)
			expectRefusal(t, outcome, c.code)
		})
	}
}

// TestEnvironmentVaultRefusesAnUnusableSession proves each fault the provider
// reports at the status check refuses the run, and nothing is written or changed.
func TestEnvironmentVaultRefusesAnUnusableSession(t *testing.T) {
	for _, command := range []string{"pull", "push", "diff"} {
		t.Run(command, func(t *testing.T) {
			v, inner := fixtureVault()
			addLogin(v, "example/fixture/web/local/alpha-key", "marker-alpha")
			tree := transferTree(fixtureConfig(fixtureEnvEntry))
			tree.files["config/app.env"] = "ALPHA_KEY=marker-local\n"
			vault := newFaultyVault(inner, map[string]*application.VaultFault{
				"Status": {Code: domain.VaultUnavailable, Reason: "the vault is not unlocked"},
			})
			store := &recordingStore{tree: tree}
			outcome := runTransfer(tree, vault, store, "env", command, "--output", "json")
			expectRefusal(t, outcome, domain.VaultUnavailable)
			if len(store.writes) != 0 || vault.calls["Find"] != 0 || vault.calls["CreateLogin"] != 0 {
				t.Fatalf("a refused session still reached the vault or the disk: writes=%d calls=%v",
					len(store.writes), vault.calls)
			}
		})
	}
}

// TestEnvironmentVaultPullWritesOnlyWhatMatches proves pull's refusals and
// findings: each local problem, each vault problem, and the writes of clean entries.
func TestEnvironmentVaultPullWritesOnlyWhatMatches(t *testing.T) {
	t.Run("a local link on the path refuses the run", func(t *testing.T) {
		v, vault := fixtureVault()
		addLogin(v, "example/fixture/web/local/alpha-key", "marker-alpha")
		addLogin(v, "example/fixture/web/local/beta-key", "marker-beta")
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		tree.escapes["config/app.env"] = true
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "pull")
		expectRefusal(t, outcome, domain.PathEscapesRoot)
	})
	t.Run("a local file that holds no text refuses the run", func(t *testing.T) {
		v, vault := fixtureVault()
		addLogin(v, "example/fixture/web/local/alpha-key", "marker-alpha")
		addLogin(v, "example/fixture/web/local/beta-key", "marker-beta")
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		tree.binary["config/app.env"] = true
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "pull")
		expectRefusal(t, outcome, domain.FileUnreadable)
	})
	t.Run("an unreadable local file refuses the run", func(t *testing.T) {
		v, vault := fixtureVault()
		addLogin(v, "example/fixture/web/local/alpha-key", "marker-alpha")
		addLogin(v, "example/fixture/web/local/beta-key", "marker-beta")
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		tree.unreadable["config/app.env"] = true
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "pull")
		expectRefusal(t, outcome, domain.FileUnreadable)
	})
	t.Run("a note missing from the vault is a finding, and the clean entry is written", func(t *testing.T) {
		v, vault := fixtureVault()
		addLogin(v, "example/fixture/web/local/alpha-key", "marker-alpha")
		addLogin(v, "example/fixture/web/local/beta-key", "marker-beta")
		tree := transferTree(fixtureConfig(fixtureEnvEntry, fixtureFileEntry))
		store := &recordingStore{tree: tree}
		outcome := runTransfer(tree, vault, store, "env", "pull")
		if outcome.ExitCode != 1 || !strings.Contains(outcome.Stderr, "item-missing config/service.json") {
			t.Fatalf("exit %d stderr %q, want item-missing for the note", outcome.ExitCode, outcome.Stderr)
		}
		if len(store.writes) != 1 || store.writes[0].Path != "config/app.env" {
			t.Fatalf("writes = %+v, want only the env file", store.writes)
		}
	})
	t.Run("two notes of one name are ambiguous", func(t *testing.T) {
		v, vault := fixtureVault()
		addNote(v, "example/fixture/api/local/service-config", map[string]string{"service.json.g0001": "a"})
		addNote(v, "example/fixture/api/local/service-config", map[string]string{"service.json.g0001": "b"})
		tree := transferTree(fixtureConfig(fixtureFileEntry))
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "pull")
		if outcome.ExitCode != 1 || !strings.Contains(outcome.Stderr, "item-ambiguous config/service.json") {
			t.Fatalf("exit %d stderr %q, want item-ambiguous", outcome.ExitCode, outcome.Stderr)
		}
	})
	t.Run("a note with no generation is attachment-missing", func(t *testing.T) {
		v, vault := fixtureVault()
		addNote(v, "example/fixture/api/local/service-config", map[string]string{"other.txt": "a"})
		tree := transferTree(fixtureConfig(fixtureFileEntry))
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "pull")
		if outcome.ExitCode != 1 || !strings.Contains(outcome.Stderr, "attachment-missing config/service.json") {
			t.Fatalf("exit %d stderr %q, want attachment-missing", outcome.ExitCode, outcome.Stderr)
		}
	})
	t.Run("a failed attachment download refuses the run", func(t *testing.T) {
		v, inner := fixtureVault()
		addNote(v, "example/fixture/api/local/service-config", map[string]string{"service.json.g0001": "x"})
		tree := transferTree(fixtureConfig(fixtureFileEntry))
		vault := newFaultyVault(inner, map[string]*application.VaultFault{
			"ReadAttachment": {Code: domain.VaultUnavailable, Reason: "the vault provider did not complete the call"},
		})
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "pull")
		expectRefusal(t, outcome, domain.VaultUnavailable)
	})
	t.Run("a failed item lookup refuses the run", func(t *testing.T) {
		_, inner := fixtureVault()
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		vault := newFaultyVault(inner, map[string]*application.VaultFault{
			"Find": {Code: domain.VaultRefused, Reason: "the vault provider answered something this build does not understand"},
		})
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "pull")
		expectRefusal(t, outcome, domain.VaultRefused)
	})
	t.Run("a value with a line break refuses the run", func(t *testing.T) {
		v, vault := fixtureVault()
		addLogin(v, "example/fixture/web/local/alpha-key", "marker-first\nmarker-second")
		addLogin(v, "example/fixture/web/local/beta-key", "marker-beta")
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "pull")
		expectRefusal(t, outcome, domain.VaultRefused)
		if strings.Contains(outcome.Stderr, "marker") {
			t.Fatalf("refusal printed a value: %q", outcome.Stderr)
		}
	})
	t.Run("a value that needs quoting is written quoted and reads back", func(t *testing.T) {
		v, vault := fixtureVault()
		addLogin(v, "example/fixture/web/local/alpha-key", "it's a value")
		addLogin(v, "example/fixture/web/local/beta-key", "plain")
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		store := &recordingStore{tree: tree}
		outcome := runTransfer(tree, vault, store, "env", "pull")
		if outcome.ExitCode != 0 {
			t.Fatalf("exit %d, stderr %q", outcome.ExitCode, outcome.Stderr)
		}
		if !strings.Contains(tree.files["config/app.env"], "ALPHA_KEY='it'\\''s a value'\n") {
			t.Fatalf("written file %q does not quote the value", tree.files["config/app.env"])
		}
	})
	t.Run("a write the store refuses is reported and exits 2", func(t *testing.T) {
		v, vault := fixtureVault()
		addLogin(v, "example/fixture/web/local/alpha-key", "marker-alpha")
		addLogin(v, "example/fixture/web/local/beta-key", "marker-beta")
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree, err: fmt.Errorf("disk full")}, "env", "pull")
		expectRefusal(t, outcome, domain.FileUnwritable)
	})
	t.Run("a file that already matches writes nothing and is identical", func(t *testing.T) {
		v, vault := fixtureVault()
		addLogin(v, "example/fixture/web/local/alpha-key", "marker-alpha")
		addLogin(v, "example/fixture/web/local/beta-key", "marker-beta")
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		tree.files["config/app.env"] = vaultHeader() + "ALPHA_KEY=marker-alpha\nBETA_KEY=marker-beta\n"
		store := &recordingStore{tree: tree}
		outcome := runTransfer(tree, vault, store, "env", "pull")
		if outcome.ExitCode != 0 || len(store.writes) != 0 {
			t.Fatalf("exit %d writes %d, want 0 and none", outcome.ExitCode, len(store.writes))
		}
		if !strings.Contains(outcome.Stdout, "0 written, 1 identical") {
			t.Fatalf("summary %q", outcome.Stdout)
		}
	})
	t.Run("an empty selection reports zero entries and exits 0", func(t *testing.T) {
		_, vault := fixtureVault()
		tree := transferTree(fixtureConfig(
			"      - path: config/derived.env\n        class: derived\n        reason: built by a tool\n        by: tool\n"))
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "pull", "--output", "json")
		if outcome.ExitCode != 0 || !strings.Contains(outcome.Stdout, `"count":0`) ||
			!strings.Contains(outcome.Stdout, `"status":"completed"`) {
			t.Fatalf("exit %d stdout %q", outcome.ExitCode, outcome.Stdout)
		}
	})
	t.Run("json findings name the class and the entry", func(t *testing.T) {
		_, vault := fixtureVault()
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "pull", "--output", "json")
		if outcome.ExitCode != 1 || !strings.Contains(outcome.Stdout, `{"class":"item-missing","name":"ALPHA_KEY"}`) ||
			!strings.Contains(outcome.Stdout, `"status":"findings"`) {
			t.Fatalf("exit %d stdout %q", outcome.ExitCode, outcome.Stdout)
		}
	})
}

// vaultHeader is the first line pull writes, for tests that compare whole files.
func vaultHeader() string {
	return "# Written by rhino env pull from the vault; change the vault, not this file.\n"
}

// TestEnvironmentVaultPushSeedsAndReports proves push's findings and every
// provider change it makes, and the refusal of each change that fails.
func TestEnvironmentVaultPushSeedsAndReports(t *testing.T) {
	t.Run("a missing local file is reported and nothing is pushed", func(t *testing.T) {
		v, vault := fixtureVault()
		tree := transferTree(fixtureConfig(fixtureEnvEntry, fixtureFileEntry))
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "push")
		if outcome.ExitCode != 1 || !strings.Contains(outcome.Stderr, "local-missing config/app.env") ||
			!strings.Contains(outcome.Stderr, "local-missing config/service.json") {
			t.Fatalf("exit %d stderr %q", outcome.ExitCode, outcome.Stderr)
		}
		if len(v.Items) != 0 {
			t.Fatalf("push created items for a missing local file")
		}
	})
	t.Run("an unparseable local file refuses before any change", func(t *testing.T) {
		v, vault := fixtureVault()
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		tree.files["config/app.env"] = "ALPHA_KEY=marker-alpha\nthis is not a line\n"
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "push")
		expectRefusal(t, outcome, domain.FileUnreadable)
		if !strings.Contains(outcome.Stderr, "line 2") || strings.Contains(outcome.Stderr, "this is not") {
			t.Fatalf("refusal must name the line number only: %q", outcome.Stderr)
		}
		if len(v.Items) != 0 {
			t.Fatalf("a refused push still changed the vault")
		}
	})
	t.Run("an undeclared key and a missing key are findings", func(t *testing.T) {
		_, vault := fixtureVault()
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		tree.files["config/app.env"] = "ALPHA_KEY=marker-alpha\nEXTRA_KEY=marker-extra\n"
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "push")
		if outcome.ExitCode != 1 || !strings.Contains(outcome.Stderr, "undeclared-key EXTRA_KEY") ||
			!strings.Contains(outcome.Stderr, "local-missing BETA_KEY") {
			t.Fatalf("exit %d stderr %q", outcome.ExitCode, outcome.Stderr)
		}
	})
	t.Run("two items of one key name are ambiguous", func(t *testing.T) {
		v, vault := fixtureVault()
		addLogin(v, "example/fixture/web/local/alpha-key", "marker-one")
		addLogin(v, "example/fixture/web/local/alpha-key", "marker-two")
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		tree.files["config/app.env"] = "ALPHA_KEY=marker-alpha\nBETA_KEY=marker-beta\n"
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "push")
		if outcome.ExitCode != 1 || !strings.Contains(outcome.Stderr, "item-ambiguous ALPHA_KEY") {
			t.Fatalf("exit %d stderr %q", outcome.ExitCode, outcome.Stderr)
		}
	})
	t.Run("a new note takes generation one and verifies", func(t *testing.T) {
		v, vault := fixtureVault()
		tree := transferTree(fixtureConfig(fixtureFileEntry))
		tree.files["config/service.json"] = "marker-service-new"
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "push")
		if outcome.ExitCode != 0 {
			t.Fatalf("exit %d stderr %q", outcome.ExitCode, outcome.Stderr)
		}
		views := v.Views("example/fixture/api/local/service-config")
		if len(views) != 1 || views[0].Attachments["service.json.g0001"] != "marker-service-new" {
			t.Fatalf("note views = %+v", views)
		}
	})
	t.Run("a note with no generation takes generation one", func(t *testing.T) {
		v, vault := fixtureVault()
		addNote(v, "example/fixture/api/local/service-config", map[string]string{"other.txt": "x"})
		tree := transferTree(fixtureConfig(fixtureFileEntry))
		tree.files["config/service.json"] = "marker-service-new"
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "push")
		if outcome.ExitCode != 0 {
			t.Fatalf("exit %d stderr %q", outcome.ExitCode, outcome.Stderr)
		}
		views := v.Views("example/fixture/api/local/service-config")
		if views[0].Attachments["service.json.g0001"] != "marker-service-new" {
			t.Fatalf("generation one not written: %+v", views[0].Attachments)
		}
	})
	t.Run("a note already holding the same bytes is identical", func(t *testing.T) {
		v, vault := fixtureVault()
		addNote(v, "example/fixture/api/local/service-config", map[string]string{"service.json.g0001": "marker-same"})
		tree := transferTree(fixtureConfig(fixtureFileEntry))
		tree.files["config/service.json"] = "marker-same"
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "push", "--overwrite")
		if outcome.ExitCode != 0 || !strings.Contains(outcome.Stdout, "0 pushed, 1 identical") {
			t.Fatalf("exit %d stdout %q", outcome.ExitCode, outcome.Stdout)
		}
	})
	t.Run("a differing note is a finding without overwrite", func(t *testing.T) {
		v, vault := fixtureVault()
		addNote(v, "example/fixture/api/local/service-config", map[string]string{"service.json.g0001": "marker-old"})
		tree := transferTree(fixtureConfig(fixtureFileEntry))
		tree.files["config/service.json"] = "marker-new"
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "push")
		if outcome.ExitCode != 1 || !strings.Contains(outcome.Stderr, "vault-differs config/service.json") {
			t.Fatalf("exit %d stderr %q", outcome.ExitCode, outcome.Stderr)
		}
	})
	t.Run("an overwrite with one generation keeps it and adds the next", func(t *testing.T) {
		v, vault := fixtureVault()
		addNote(v, "example/fixture/api/local/service-config", map[string]string{"service.json.g0001": "marker-old"})
		tree := transferTree(fixtureConfig(fixtureFileEntry))
		tree.files["config/service.json"] = "marker-new"
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "push", "--overwrite")
		if outcome.ExitCode != 0 {
			t.Fatalf("exit %d stderr %q", outcome.ExitCode, outcome.Stderr)
		}
		attachments := v.Views("example/fixture/api/local/service-config")[0].Attachments
		if attachments["service.json.g0001"] != "marker-old" || attachments["service.json.g0002"] != "marker-new" {
			t.Fatalf("attachments = %+v", attachments)
		}
	})
	t.Run("a note whose upload does not verify keeps the previous generation", func(t *testing.T) {
		v, vault := fixtureVault()
		addNote(v, "example/fixture/api/local/service-config", map[string]string{"service.json.g0001": "marker-old"})
		v.Corrupt = true
		tree := transferTree(fixtureConfig(fixtureFileEntry))
		tree.files["config/service.json"] = "marker-new"
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "push", "--overwrite")
		expectRefusal(t, outcome, domain.VaultRefused)
		v.Corrupt = false
		attachments := v.Views("example/fixture/api/local/service-config")[0].Attachments
		if len(attachments) != 1 || attachments["service.json.g0001"] != "marker-old" {
			t.Fatalf("the unverified upload was not removed: %+v", attachments)
		}
	})
	t.Run("provider faults on each change refuse the run", func(t *testing.T) {
		unavailable := &application.VaultFault{Code: domain.VaultUnavailable, Reason: "the vault provider did not complete the call"}
		cases := []struct {
			name       string
			seed       func(*fakebw.Vault)
			localFiles map[string]string
			fail       string
		}{
			{"create login", func(*fakebw.Vault) {}, map[string]string{"config/app.env": "ALPHA_KEY=a\nBETA_KEY=b\n"}, "CreateLogin"},
			{"update login", func(v *fakebw.Vault) {
				addLogin(v, "example/fixture/web/local/alpha-key", "old")
				addLogin(v, "example/fixture/web/local/beta-key", "old")
			}, map[string]string{"config/app.env": "ALPHA_KEY=a\nBETA_KEY=b\n"}, "UpdateLogin"},
			{"create note", func(*fakebw.Vault) {}, map[string]string{"config/service.json": "x"}, "CreateNote"},
			{"add attachment to a new note", func(*fakebw.Vault) {}, map[string]string{"config/service.json": "x"}, "AddAttachment"},
			{"read back a new attachment", func(*fakebw.Vault) {}, map[string]string{"config/service.json": "x"}, "ReadAttachment"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				v, inner := fixtureVault()
				c.seed(v)
				tree := transferTree(fixtureConfig(fixtureEnvEntry, fixtureFileEntry))
				for path, content := range c.localFiles {
					tree.files[path] = content
				}
				vault := newFaultyVault(inner, map[string]*application.VaultFault{c.fail: unavailable})
				outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "push", "--overwrite")
				expectRefusal(t, outcome, domain.VaultUnavailable)
			})
		}
	})
	t.Run("a failed delete of the previous generation refuses after the upload", func(t *testing.T) {
		v, inner := fixtureVault()
		addNote(v, "example/fixture/api/local/service-config", map[string]string{
			"service.json.g0001": "marker-first", "service.json.g0002": "marker-second",
		})
		tree := transferTree(fixtureConfig(fixtureFileEntry))
		tree.files["config/service.json"] = "marker-third"
		vault := newFaultyVault(inner, map[string]*application.VaultFault{
			"DeleteAttachment": {Code: domain.VaultUnavailable, Reason: "the vault provider did not complete the call"},
		})
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "push", "--overwrite")
		expectRefusal(t, outcome, domain.VaultUnavailable)
	})
	t.Run("a failed item lookup refuses the run", func(t *testing.T) {
		_, inner := fixtureVault()
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		tree.files["config/app.env"] = "ALPHA_KEY=a\nBETA_KEY=b\n"
		vault := newFaultyVault(inner, map[string]*application.VaultFault{
			"Find": {Code: domain.VaultRefused, Reason: "the vault provider answered something this build does not understand"},
		})
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "push")
		expectRefusal(t, outcome, domain.VaultRefused)
	})
	t.Run("a file note that cannot be read back refuses the run", func(t *testing.T) {
		v, inner := fixtureVault()
		addNote(v, "example/fixture/api/local/service-config", map[string]string{"service.json.g0001": "marker-old"})
		tree := transferTree(fixtureConfig(fixtureFileEntry))
		tree.files["config/service.json"] = "marker-new"
		vault := newFaultyVault(inner, map[string]*application.VaultFault{
			"ReadAttachment": {Code: domain.VaultUnavailable, Reason: "the vault provider did not complete the call"},
		})
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "push", "--overwrite")
		expectRefusal(t, outcome, domain.VaultUnavailable)
	})
	t.Run("a file note held twice is ambiguous", func(t *testing.T) {
		v, vault := fixtureVault()
		addNote(v, "example/fixture/api/local/service-config", map[string]string{"service.json.g0001": "a"})
		addNote(v, "example/fixture/api/local/service-config", map[string]string{"service.json.g0001": "b"})
		tree := transferTree(fixtureConfig(fixtureFileEntry))
		tree.files["config/service.json"] = "marker-new"
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "push")
		if outcome.ExitCode != 1 || !strings.Contains(outcome.Stderr, "item-ambiguous config/service.json") {
			t.Fatalf("exit %d stderr %q", outcome.ExitCode, outcome.Stderr)
		}
	})
	t.Run("a push that changes nothing and reports nothing is identical", func(t *testing.T) {
		v, vault := fixtureVault()
		addLogin(v, "example/fixture/web/local/alpha-key", "marker-alpha")
		addLogin(v, "example/fixture/web/local/beta-key", "marker-beta")
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		tree.files["config/app.env"] = "ALPHA_KEY=marker-alpha\nBETA_KEY=marker-beta\n"
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "push", "--output", "json")
		if outcome.ExitCode != 0 || !strings.Contains(outcome.Stdout, `"count":0`) {
			t.Fatalf("exit %d stdout %q", outcome.ExitCode, outcome.Stdout)
		}
	})
}

// TestEnvironmentVaultDiffReportsByName proves diff's findings, one per class,
// and that a run with no difference exits 0.
func TestEnvironmentVaultDiffReportsByName(t *testing.T) {
	t.Run("a missing local file is reported", func(t *testing.T) {
		_, vault := fixtureVault()
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "diff")
		if outcome.ExitCode != 1 || !strings.Contains(outcome.Stderr, "local-missing config/app.env") {
			t.Fatalf("exit %d stderr %q", outcome.ExitCode, outcome.Stderr)
		}
	})
	t.Run("an unparseable local file refuses", func(t *testing.T) {
		_, vault := fixtureVault()
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		tree.files["config/app.env"] = "ALPHA_KEY='unterminated\n"
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "diff")
		expectRefusal(t, outcome, domain.FileUnreadable)
	})
	t.Run("undeclared keys, a missing local key, and differences are each named", func(t *testing.T) {
		v, vault := fixtureVault()
		addLogin(v, "example/fixture/web/local/alpha-key", "marker-alpha")
		addLogin(v, "example/fixture/web/local/beta-key", "marker-beta")
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		tree.files["config/app.env"] = "EXTRA=marker-extra\nALPHA_KEY=marker-local\n"
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "diff")
		if outcome.ExitCode != 1 {
			t.Fatalf("exit %d", outcome.ExitCode)
		}
		for _, line := range []string{"undeclared-key EXTRA", "local-missing BETA_KEY", "vault-differs ALPHA_KEY"} {
			if !strings.Contains(outcome.Stderr, line) {
				t.Fatalf("stderr %q does not name %q", outcome.Stderr, line)
			}
		}
	})
	t.Run("an ambiguous item for a key the local file holds is named", func(t *testing.T) {
		v, vault := fixtureVault()
		addLogin(v, "example/fixture/web/local/alpha-key", "marker-alpha")
		addLogin(v, "example/fixture/web/local/beta-key", "marker-beta-one")
		addLogin(v, "example/fixture/web/local/beta-key", "marker-beta-two")
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		tree.files["config/app.env"] = "ALPHA_KEY=marker-alpha\nBETA_KEY=marker-beta-one\n"
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "diff")
		if outcome.ExitCode != 1 || !strings.Contains(outcome.Stderr, "item-ambiguous BETA_KEY") {
			t.Fatalf("exit %d stderr %q", outcome.ExitCode, outcome.Stderr)
		}
	})
	t.Run("an item missing from the vault is named and the hint is printed", func(t *testing.T) {
		_, vault := fixtureVault()
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		tree.files["config/app.env"] = "ALPHA_KEY=a\nBETA_KEY=b\n"
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "diff")
		if outcome.ExitCode != 1 || !strings.Contains(outcome.Stderr, "item-missing ALPHA_KEY") ||
			!strings.Contains(outcome.Stdout, "sync the vault") {
			t.Fatalf("exit %d stdout %q stderr %q", outcome.ExitCode, outcome.Stdout, outcome.Stderr)
		}
	})
	t.Run("an item ambiguous in the vault is named", func(t *testing.T) {
		v, vault := fixtureVault()
		addLogin(v, "example/fixture/web/local/alpha-key", "a")
		addLogin(v, "example/fixture/web/local/alpha-key", "b")
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		tree.files["config/app.env"] = "ALPHA_KEY=a\nBETA_KEY=b\n"
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "diff")
		if outcome.ExitCode != 1 || !strings.Contains(outcome.Stderr, "item-ambiguous ALPHA_KEY") {
			t.Fatalf("exit %d stderr %q", outcome.ExitCode, outcome.Stderr)
		}
	})
	t.Run("a file entry is compared by name, and an absent note or generation is named", func(t *testing.T) {
		v, vault := fixtureVault()
		tree := transferTree(fixtureConfig(fixtureFileEntry))
		tree.files["config/service.json"] = "marker-local"
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "diff")
		if !strings.Contains(outcome.Stderr, "item-missing config/service.json") {
			t.Fatalf("stderr %q", outcome.Stderr)
		}
		addNote(v, "example/fixture/api/local/service-config", map[string]string{"other.txt": "x"})
		outcome = runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "diff")
		if !strings.Contains(outcome.Stderr, "attachment-missing config/service.json") {
			t.Fatalf("stderr %q", outcome.Stderr)
		}
	})
	t.Run("a file entry with two notes is ambiguous, and a differing one is named", func(t *testing.T) {
		v, vault := fixtureVault()
		addNote(v, "example/fixture/api/local/service-config", map[string]string{"service.json.g0001": "a"})
		addNote(v, "example/fixture/api/local/service-config", map[string]string{"service.json.g0001": "b"})
		tree := transferTree(fixtureConfig(fixtureFileEntry))
		tree.files["config/service.json"] = "a"
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "diff")
		if !strings.Contains(outcome.Stderr, "item-ambiguous config/service.json") {
			t.Fatalf("stderr %q", outcome.Stderr)
		}
	})
	t.Run("a file entry that differs from its note is named, and an equal one is identical", func(t *testing.T) {
		v, vault := fixtureVault()
		addNote(v, "example/fixture/api/local/service-config", map[string]string{"service.json.g0001": "marker-vault"})
		tree := transferTree(fixtureConfig(fixtureFileEntry))
		tree.files["config/service.json"] = "marker-local"
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "diff")
		if outcome.ExitCode != 1 || !strings.Contains(outcome.Stderr, "vault-differs config/service.json") {
			t.Fatalf("exit %d stderr %q", outcome.ExitCode, outcome.Stderr)
		}
		tree.files["config/service.json"] = "marker-vault"
		outcome = runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "diff")
		if outcome.ExitCode != 0 || !strings.Contains(outcome.Stdout, "1 entries: 1 identical") {
			t.Fatalf("exit %d stdout %q", outcome.ExitCode, outcome.Stdout)
		}
	})
	t.Run("a file-entry read that fails refuses the run", func(t *testing.T) {
		v, inner := fixtureVault()
		addNote(v, "example/fixture/api/local/service-config", map[string]string{"service.json.g0001": "a"})
		tree := transferTree(fixtureConfig(fixtureFileEntry))
		tree.files["config/service.json"] = "a"
		vault := newFaultyVault(inner, map[string]*application.VaultFault{
			"ReadAttachment": {Code: domain.VaultUnavailable, Reason: "the vault provider did not complete the call"},
		})
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "diff")
		expectRefusal(t, outcome, domain.VaultUnavailable)
	})
	t.Run("a diff of one selected entry leaves the others unchecked", func(t *testing.T) {
		_, vault := fixtureVault()
		tree := transferTree(fixtureConfig(fixtureEnvEntry, fixtureFileEntry))
		tree.files["config/service.json"] = "marker-local"
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "diff", "--entry", "config/service.json")
		if strings.Contains(outcome.Stderr, "config/app.env") {
			t.Fatalf("an unselected entry was reported: %q", outcome.Stderr)
		}
		if !strings.Contains(outcome.Stderr, "item-missing config/service.json") {
			t.Fatalf("stderr %q", outcome.Stderr)
		}
	})
	t.Run("a diff of an identical vault exits 0 and names the count", func(t *testing.T) {
		v, vault := fixtureVault()
		addLogin(v, "example/fixture/web/local/alpha-key", "marker-alpha")
		addLogin(v, "example/fixture/web/local/beta-key", "marker-beta")
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		tree.files["config/app.env"] = "ALPHA_KEY=marker-alpha\nBETA_KEY=marker-beta\n"
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "diff")
		if outcome.ExitCode != 0 || outcome.Stdout != "[env-diff] 1 entries: 1 identical\n" {
			t.Fatalf("exit %d stdout %q", outcome.ExitCode, outcome.Stdout)
		}
	})
	t.Run("a failed lookup refuses a diff with its code", func(t *testing.T) {
		_, inner := fixtureVault()
		tree := transferTree(fixtureConfig(fixtureEnvEntry))
		tree.files["config/app.env"] = "ALPHA_KEY=a\nBETA_KEY=b\n"
		vault := newFaultyVault(inner, map[string]*application.VaultFault{
			"Find": {Code: domain.VaultRefused, Reason: "the vault provider answered something this build does not understand"},
		})
		outcome := runTransfer(tree, vault, &recordingStore{tree: tree}, "env", "diff")
		expectRefusal(t, outcome, domain.VaultRefused)
	})
}
