package runtime

import (
	"errors"
	"io/fs"
	"path"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// flagValue returns the argument after flag, or "" when flag is absent.
func flagValue(args []string, flag string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}

// bwAnswers is the Bitwarden CLI double. It answers each subcommand the way the
// vault would, and writes the downloaded attachment where --output points.
func bwAnswers(m *memFS, status string) func(*hostCommand) ([]byte, error) {
	return func(command *hostCommand) ([]byte, error) {
		args := command.Args[1:]
		switch args[0] {
		case "status":
			return []byte(`{"status":"` + status + `","serverUrl":"https://vault.example/"}` + "\n"), nil
		case "list":
			return []byte(`[{"id":"1","name":"app","type":1,"login":{"password":"pw"}},{"id":"2","name":"app-other","type":2}]`), nil
		case "encode":
			return []byte("ENCODED"), nil
		case "create":
			if args[1] == "attachment" {
				return []byte(`{"id":"1","name":"app","type":1,"attachments":[{"id":"a1","fileName":"x.txt"}]}`), nil
			}
			return []byte(`{"id":"9","name":"fresh","type":1,"login":{"password":"pw"}}`), nil
		case "get":
			if args[1] == "item" {
				return []byte(`{"id":"1","name":"app","type":1,"login":{"password":"old"},"notes":"kept"}`), nil
			}
			dir := flagValue(args, "--output")
			_ = m.mkdirAll(dir, 0o700)
			m.nodes[path.Join(dir, "x.txt")] = &memNode{perm: 0o600, data: []byte("DATA")}
		}
		return nil, nil
	}
}

// faultError turns a possibly nil vault fault into an error without a typed nil.
func faultError(f *application.VaultFault) error {
	if f == nil {
		return nil
	}
	return errors.New(f.Reason)
}

// vaultScenario runs one Bitwarden call against a fresh double for faultSweep.
func vaultScenario(status string, run func(v BitwardenVault, m *memFS) error) func(t *testing.T, log *callLog) func() error {
	return func(t *testing.T, log *callLog) func() error {
		m := repository(log, map[string]string{"README.md": "r"})
		commands := &fakeCommands{log: log, respond: bwAnswers(m, status)}
		useDoubles(t, m, commands, fakeEnv{"BW_SESSION": "session", "PATH": "/usr/bin"})
		return func() error { return run(BitwardenVault{}, m) }
	}
}

func TestBitwardenVaultStatusRefusesEveryUnfitVault(t *testing.T) {
	cases := []struct {
		name    string
		env     fakeEnv
		status  string
		answer  func(*hostCommand) ([]byte, error)
		server  string
		reason  string
		allowed bool
	}{
		{name: "no session", env: fakeEnv{"PATH": "/usr/bin"}, reason: reasonNoSession},
		{name: "locked", env: fakeEnv{"BW_SESSION": "s", "PATH": "/usr/bin"}, status: "locked", server: "https://vault.example", reason: reasonLocked},
		{name: "other server", env: fakeEnv{"BW_SESSION": "s", "PATH": "/usr/bin"}, status: "unlocked", server: "https://other.example", reason: reasonOtherServer},
		{name: "unanswerable", env: fakeEnv{"BW_SESSION": "s", "PATH": "/usr/bin"}, server: "https://vault.example",
			answer: func(*hostCommand) ([]byte, error) { return []byte("not json"), nil }, reason: reasonAnswer},
		{name: "call failed", env: fakeEnv{"BW_SESSION": "s", "PATH": "/usr/bin"}, server: "https://vault.example",
			answer: func(*hostCommand) ([]byte, error) { return nil, execFailure() }, reason: reasonCallFailed},
		{name: "matching server", env: fakeEnv{"BW_SESSION": "s", "PATH": "/usr/bin"}, status: "unlocked", server: "https://vault.example/", allowed: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := repository(&callLog{}, nil)
			commands := &fakeCommands{log: &callLog{}, respond: bwAnswers(m, tc.status)}
			if tc.answer != nil {
				commands.respond = tc.answer
			}
			useDoubles(t, m, commands, tc.env)
			fault := (BitwardenVault{}).Status(tc.server)
			if tc.allowed {
				if fault != nil {
					t.Fatalf("Status = %+v, want no fault", fault)
				}
				return
			}
			if fault == nil || fault.Reason != tc.reason {
				t.Fatalf("Status = %+v, want reason %q", fault, tc.reason)
			}
		})
	}
	faultSweep(t, vaultScenario("unlocked", func(v BitwardenVault, _ *memFS) error {
		return faultError(v.Status("https://vault.example"))
	}))
}

func TestBitwardenVaultLocatesTheExecutableOrSaysWhy(t *testing.T) {
	cases := []struct {
		name   string
		lookup func(file string) (string, error)
		exit   int
		reason string
	}{
		{"absent everywhere", func(file string) (string, error) { return "", hostErrNotFound }, statusChildNotFound, reasonNoExecutable},
		{"present but not executable", func(file string) (string, error) {
			if file == bitwardenExecutable {
				return "", hostErrNotFound
			}
			return "", fs.ErrPermission
		}, statusChildNotExecutable, reasonNotExecutable},
		{"lookup broke", func(file string) (string, error) { return "", errInjected }, 0, reasonNoExecutable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := repository(&callLog{}, nil)
			commands := &fakeCommands{log: &callLog{}, lookPath: tc.lookup}
			useDoubles(t, m, commands, fakeEnv{"BW_SESSION": "s", "PATH": "/usr/bin:/bin"})
			fault := (BitwardenVault{}).Status("https://vault.example")
			if fault == nil || fault.Reason != tc.reason || fault.Exit != tc.exit {
				t.Fatalf("Status = %+v, want reason %q exit %d", fault, tc.reason, tc.exit)
			}
		})
	}
}

func TestBitwardenVaultFindsItemsByExactName(t *testing.T) {
	m := repository(&callLog{}, nil)
	useDoubles(t, m, &fakeCommands{log: &callLog{}, respond: bwAnswers(m, "unlocked")}, fakeEnv{"BW_SESSION": "s", "PATH": "/usr/bin"})
	items, fault := (BitwardenVault{}).Find("app")
	if fault != nil || len(items) != 1 || items[0].ID != "1" || items[0].Kind != "login" || items[0].Password != "pw" {
		t.Fatalf("Find(app) = %+v, %+v", items, fault)
	}
	if items, _ := (BitwardenVault{}).Find("ap"); len(items) != 0 {
		t.Fatalf("Find(ap) matched %+v, want no items", items)
	}
	faultSweep(t, vaultScenario("unlocked", func(v BitwardenVault, _ *memFS) error {
		_, fault := v.Find("app")
		return faultError(fault)
	}))
}

func TestBitwardenVaultWritesItemsAndAttachments(t *testing.T) {
	m := repository(&callLog{}, nil)
	useDoubles(t, m, &fakeCommands{log: &callLog{}, respond: bwAnswers(m, "unlocked")}, fakeEnv{"BW_SESSION": "s", "PATH": "/usr/bin"})
	v := BitwardenVault{}
	if item, fault := v.CreateLogin("fresh", "pw"); fault != nil || item.ID != "9" {
		t.Fatalf("CreateLogin = %+v, %+v", item, fault)
	}
	if item, fault := v.CreateNote("note"); fault != nil || item.ID != "9" {
		t.Fatalf("CreateNote = %+v, %+v", item, fault)
	}
	if fault := v.UpdateLogin(application.VaultItem{ID: "1"}, "new"); fault != nil {
		t.Fatalf("UpdateLogin = %+v", fault)
	}
	if attachment, fault := v.AddAttachment("1", "x.txt", []byte("DATA")); fault != nil || attachment.ID != "a1" {
		t.Fatalf("AddAttachment = %+v, %+v", attachment, fault)
	}
	if data, fault := v.ReadAttachment("1", "a1"); fault != nil || string(data) != "DATA" {
		t.Fatalf("ReadAttachment = %q, %+v", data, fault)
	}
	if fault := v.DeleteAttachment("1", "a1"); fault != nil {
		t.Fatalf("DeleteAttachment = %+v", fault)
	}
	for _, name := range []string{"", "a/b", "a\\b", ".", ".."} {
		if _, fault := v.AddAttachment("1", name, nil); fault == nil || fault.Reason != reasonStaging {
			t.Fatalf("AddAttachment(%q) = %+v, want a staging refusal", name, fault)
		}
	}
	if _, fault := v.AddAttachment("1", "other.txt", []byte("x")); fault == nil || fault.Reason != reasonNoAttachment {
		t.Fatalf("AddAttachment of an unnamed upload = %+v", fault)
	}

	extra := func(command *hostCommand) ([]byte, error) {
		out, err := bwAnswers(m, "unlocked")(command)
		if args := command.Args[1:]; args[0] == "get" && args[1] == "attachment" {
			dir := flagValue(args, "--output")
			m.nodes[path.Join(dir, "y.txt")] = &memNode{perm: 0o600, data: []byte("EXTRA")}
		}
		return out, err
	}
	useDoubles(t, m, &fakeCommands{log: &callLog{}, respond: extra}, nil)
	if _, fault := (BitwardenVault{}).ReadAttachment("1", "a1"); fault == nil || fault.Reason != reasonAnswer {
		t.Fatalf("ReadAttachment with two files = %+v, want an answer refusal", fault)
	}
}

func TestBitwardenVaultEveryCallCanFail(t *testing.T) {
	calls := map[string]func(v BitwardenVault, m *memFS) error{
		"CreateLogin": func(v BitwardenVault, _ *memFS) error {
			_, fault := v.CreateLogin("fresh", "pw")
			return faultError(fault)
		},
		"CreateNote": func(v BitwardenVault, _ *memFS) error {
			_, fault := v.CreateNote("note")
			return faultError(fault)
		},
		"UpdateLogin": func(v BitwardenVault, _ *memFS) error {
			return faultError(v.UpdateLogin(application.VaultItem{ID: "1"}, "new"))
		},
		"AddAttachment": func(v BitwardenVault, _ *memFS) error {
			_, fault := v.AddAttachment("1", "x.txt", []byte("DATA"))
			return faultError(fault)
		},
		"ReadAttachment": func(v BitwardenVault, _ *memFS) error {
			_, fault := v.ReadAttachment("1", "a1")
			return faultError(fault)
		},
		"DeleteAttachment": func(v BitwardenVault, _ *memFS) error {
			return faultError(v.DeleteAttachment("1", "a1"))
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			faultSweep(t, vaultScenario("unlocked", call))
		})
	}
}

func TestBitwardenChildEnvironmentKeepsOnlyTheAllowlist(t *testing.T) {
	kept := childEnvironment([]string{
		"BW_SESSION=s", "PATH=/bin", "LC_ALL=C", "XDG_CONFIG_HOME=/c", "HTTPS_PROXY=p",
		"GITHUB_TOKEN=secret", "AWS_SECRET=x", "BW_CLEANEXIT=1",
	})
	joined := strings.Join(kept, " ")
	for _, want := range []string{"BW_SESSION=s", "PATH=/bin", "LC_ALL=C", "XDG_CONFIG_HOME=/c", "HTTPS_PROXY=p"} {
		if !strings.Contains(joined, want) {
			t.Errorf("childEnvironment dropped %s", want)
		}
	}
	for _, drop := range []string{"GITHUB_TOKEN", "AWS_SECRET", "BW_CLEANEXIT"} {
		if strings.Contains(joined, drop) {
			t.Errorf("childEnvironment kept %s", drop)
		}
	}
}

func TestBitwardenDecodeStrictRefusesTrailingContent(t *testing.T) {
	var value map[string]any
	if err := decodeStrict([]byte(`{"a":1}`), &value); err != nil {
		t.Fatalf("decodeStrict(one value) = %v", err)
	}
	if err := decodeStrict([]byte(`{"a":1} {"b":2}`), &value); err == nil {
		t.Fatal("decodeStrict accepted trailing content")
	}
	if err := decodeStrict([]byte(`{"a":`), &value); err == nil {
		t.Fatal("decodeStrict accepted truncated JSON")
	}
	if decodeStrict(nil, &value) == nil {
		t.Fatal("decodeStrict accepted no value")
	}
}
