package runtime_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/runtime"
	"github.com/wahidyankf/rhino/internal/application"
	"github.com/wahidyankf/rhino/internal/domain"
	"github.com/wahidyankf/rhino/tests/bdd/vault/fakebw"
)

const fixtureServerURL = "https://vault.example.com"

// fakeVault builds the fake bw for one test, installs its wrapper on a PATH of
// its own, and sets the session key. configure edits the vault before the
// first call. It returns the fake's call log, which records argv and the
// names of the environment variables each call received.
func fakeVault(t *testing.T, configure func(*fakebw.Vault)) string {
	t.Helper()
	dir := t.TempDir()
	binary, err := fakebw.Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(dir, "state.json")
	logPath := filepath.Join(dir, "calls.log")
	vault := &fakebw.Vault{Status: "unlocked", ServerURL: fixtureServerURL, Blobs: map[string][]byte{}}
	if configure != nil {
		configure(vault)
	}
	if err := vault.Save(state); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(dir, "bin")
	if err := fakebw.InstallWrapper(binDir, binary, state, logPath); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("BW_SESSION", "fixture-session-key")
	return logPath
}

// TestBitwardenVaultRefusesWithoutASessionKey proves no call starts without a
// session key, and that the refusal names no value.
func TestBitwardenVaultRefusesWithoutASessionKey(t *testing.T) {
	t.Setenv("BW_SESSION", "")
	t.Setenv("PATH", t.TempDir())
	fault := runtime.BitwardenVault{}.Status(fixtureServerURL)
	if fault == nil || fault.Code != domain.VaultUnavailable || !strings.Contains(fault.Reason, "BW_SESSION") {
		t.Fatalf("fault = %+v, want unavailable naming the session", fault)
	}
}

// TestBitwardenVaultRefusesWhenBwIsNotOnPath proves a missing bw is unavailable,
// not a crash, and that the run never reaches the vault.
func TestBitwardenVaultRefusesWhenBwIsNotOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("BW_SESSION", "fixture-session-key")
	vault := runtime.BitwardenVault{}
	if fault := vault.Status(fixtureServerURL); fault == nil || fault.Code != domain.VaultUnavailable {
		t.Fatalf("status fault = %+v", fault)
	}
	if _, fault := vault.Find("example/fixture/web/local/alpha-key"); fault == nil || fault.Code != domain.VaultUnavailable {
		t.Fatalf("find fault = %+v", fault)
	}
}

// TestBitwardenVaultStatusHoldsTheDeclaredServerAndSession proves a locked
// vault and another server are each refused, and the matching vault passes.
func TestBitwardenVaultStatusHoldsTheDeclaredServerAndSession(t *testing.T) {
	cases := []struct {
		name      string
		configure func(*fakebw.Vault)
		want      string
	}{
		{"locked", func(v *fakebw.Vault) { v.Status = "locked" }, "not unlocked"},
		{"another server", func(v *fakebw.Vault) { v.ServerURL = "https://other.example.com" }, "another server"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeVault(t, c.configure)
			fault := runtime.BitwardenVault{}.Status(fixtureServerURL)
			if fault == nil || fault.Code != domain.VaultUnavailable || !strings.Contains(fault.Reason, c.want) {
				t.Fatalf("fault = %+v, want unavailable with %q", fault, c.want)
			}
		})
	}
	t.Run("unlocked on the declared server passes, with a trailing slash ignored", func(t *testing.T) {
		fakeVault(t, nil)
		if fault := (runtime.BitwardenVault{}).Status(fixtureServerURL + "/"); fault != nil {
			t.Fatalf("fault = %+v, want none", fault)
		}
	})
}

// TestBitwardenVaultDropsTheParentVariablesThatChangeTheProvider proves BW_RESPONSE
// and BW_CLEANEXIT in the parent never reach bw. Under BW_RESPONSE the fake would
// wrap every answer, so a status that still passes proves the variable was dropped.
// Under BW_CLEANEXIT the fake would turn a failed call into a silent success.
func TestBitwardenVaultDropsTheParentVariablesThatChangeTheProvider(t *testing.T) {
	logPath := fakeVault(t, nil)
	t.Setenv("BW_RESPONSE", "1")
	t.Setenv("BW_CLEANEXIT", "1")
	vault := runtime.BitwardenVault{}
	if fault := vault.Status(fixtureServerURL); fault != nil {
		t.Fatalf("status = %+v; BW_RESPONSE reached bw", fault)
	}
	fault := vault.UpdateLogin(application.VaultItem{ID: "item-missing"}, "marker-new")
	if fault == nil || fault.Code != domain.VaultUnavailable {
		t.Fatalf("update of a missing item = %+v; BW_CLEANEXIT reached bw", fault)
	}
	calls, err := fakebw.ReadLog(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range calls {
		for _, name := range call.Environment {
			if name == "BW_RESPONSE" || name == "BW_CLEANEXIT" {
				t.Fatalf("call %v received %s", call.Argv, name)
			}
		}
	}
}

// TestBitwardenVaultKeepsValuesOutOfArgumentsAndSync proves every call is argv
// only with --nointeraction, that no call is sync, and that no password appears in argv.
func TestBitwardenVaultKeepsValuesOutOfArgumentsAndSync(t *testing.T) {
	logPath := fakeVault(t, nil)
	vault := runtime.BitwardenVault{}
	item, fault := vault.CreateLogin("example/fixture/web/local/alpha-key", "marker-secret-value")
	if fault != nil {
		t.Fatalf("create login = %+v", fault)
	}
	if fault := vault.UpdateLogin(item, "marker-next-value"); fault != nil {
		t.Fatalf("update login = %+v", fault)
	}
	if _, fault := vault.Find("example/fixture/web/local/alpha-key"); fault != nil {
		t.Fatalf("find = %+v", fault)
	}
	calls, err := fakebw.ReadLog(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range calls {
		joined := strings.Join(call.Argv, " ")
		if !strings.Contains(joined, "--nointeraction") {
			t.Fatalf("call %q lacks --nointeraction", joined)
		}
		if strings.Contains(joined, "sync") {
			t.Fatalf("call %q runs sync", joined)
		}
		if strings.Contains(joined, "marker-") {
			t.Fatalf("call %q carries a value in argv", joined)
		}
	}
}

// TestBitwardenVaultFindsOnlyTheExactName proves the provider's substring search
// is narrowed to the full name the adapter asked for.
func TestBitwardenVaultFindsOnlyTheExactName(t *testing.T) {
	fakeVault(t, func(v *fakebw.Vault) {
		password := "marker-one"
		v.Create(fakebw.Item{Name: "example/fixture/web/local/alpha-key", Type: 1, Login: &fakebw.Login{Password: &password}})
		v.Create(fakebw.Item{Name: "example/fixture/web/local/alpha-key-old", Type: 1, Login: &fakebw.Login{Password: &password}})
	})
	found, fault := runtime.BitwardenVault{}.Find("example/fixture/web/local/alpha-key")
	if fault != nil || len(found) != 1 || found[0].Name != "example/fixture/web/local/alpha-key" || found[0].Password != "marker-one" {
		t.Fatalf("found = %+v, fault = %+v", found, fault)
	}
}

// TestBitwardenVaultMovesAttachmentsAndNotes proves the attachment and note calls
// round-trip: a note takes an attachment, the bytes read back equal the bytes stored,
// and a deleted attachment is gone.
func TestBitwardenVaultMovesAttachmentsAndNotes(t *testing.T) {
	fakeVault(t, nil)
	vault := runtime.BitwardenVault{}
	note, fault := vault.CreateNote("example/fixture/api/local/service-config")
	if fault != nil || note.Kind != "note" {
		t.Fatalf("create note = %+v, %+v", note, fault)
	}
	attachment, fault := vault.AddAttachment(note.ID, "service.json.g0001", []byte("marker-body\nline two"))
	if fault != nil {
		t.Fatalf("add attachment = %+v", fault)
	}
	data, fault := vault.ReadAttachment(note.ID, attachment.ID)
	if fault != nil || string(data) != "marker-body\nline two" {
		t.Fatalf("read attachment = %q, %+v", data, fault)
	}
	if fault := vault.DeleteAttachment(note.ID, attachment.ID); fault != nil {
		t.Fatalf("delete attachment = %+v", fault)
	}
	found, _ := vault.Find("example/fixture/api/local/service-config")
	if len(found) != 1 || len(found[0].Attachments) != 0 {
		t.Fatalf("after delete, note = %+v", found)
	}
}

// TestBitwardenVaultRefusesWhatItCannotStage proves an attachment name that is a
// path is refused before any call, and an unknown item is unavailable.
func TestBitwardenVaultRefusesWhatItCannotStage(t *testing.T) {
	logPath := fakeVault(t, nil)
	vault := runtime.BitwardenVault{}
	for _, name := range []string{"", "a/b", `a\b`, ".", ".."} {
		if _, fault := vault.AddAttachment("item-1", name, []byte("x")); fault == nil || fault.Code != domain.VaultRefused {
			t.Fatalf("attachment name %q = %+v, want refused", name, fault)
		}
	}
	if _, fault := vault.AddAttachment("item-missing", "service.json.g0001", []byte("x")); fault == nil || fault.Code != domain.VaultUnavailable {
		t.Fatalf("attachment to a missing item = %+v", fault)
	}
	if calls, _ := fakebw.ReadLog(logPath); len(calls) != 1 {
		t.Fatalf("refused names reached bw: %d calls", len(calls))
	}
}

// TestBitwardenVaultRefusesAnAnswerItDoesNotUnderstand proves an answer that is
// not one JSON value is refused, and the refusal carries no part of the answer.
func TestBitwardenVaultRefusesAnAnswerItDoesNotUnderstand(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bw")
	script := "#!/bin/sh\necho 'marker-leaked {\"status\":\"unlocked\"} trailing'\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("BW_SESSION", "fixture-session-key")
	fault := runtime.BitwardenVault{}.Status(fixtureServerURL)
	if fault == nil || fault.Code != domain.VaultRefused || strings.Contains(fault.Reason, "marker") {
		t.Fatalf("fault = %+v, want refused without the answer", fault)
	}
}

// TestBitwardenVaultReportsAFailedCallWithoutItsOutput proves a call that exits
// non-zero is unavailable, and its standard error is never relayed.
func TestBitwardenVaultReportsAFailedCallWithoutItsOutput(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bw")
	script := "#!/bin/sh\necho 'marker-stderr' 1>&2\nexit 3\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("BW_SESSION", "fixture-session-key")
	_, fault := runtime.BitwardenVault{}.Find("example/fixture/web/local/alpha-key")
	if fault == nil || fault.Code != domain.VaultUnavailable || strings.Contains(fault.Reason, "marker") {
		t.Fatalf("fault = %+v, want unavailable without the error text", fault)
	}
}
