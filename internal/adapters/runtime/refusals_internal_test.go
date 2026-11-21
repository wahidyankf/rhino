package runtime

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// The refusals below are the answers a real tool can give that a well-behaved
// run never gets: an answer the decoder cannot read, a walk that fails, a child
// that reports no status. Each runs against a double, so the host is untouched.

// bwOverride answers the Bitwarden double the way bwAnswers does, except that a
// subcommand whose words start with key answers with out and err instead.
func bwOverride(m *memFS, status, key string, out []byte, err error) func(*hostCommand) ([]byte, error) {
	base := bwAnswers(m, status)
	return func(command *hostCommand) ([]byte, error) {
		if words := strings.Join(command.Args[1:], " "); strings.HasPrefix(words, key) {
			return out, err
		}
		return base(command)
	}
}

func TestBitwardenRefusesAnswersItCannotDecode(t *testing.T) {
	cases := []struct {
		name string
		key  string
		out  []byte
		err  error
		run  func(v BitwardenVault) *application.VaultFault
	}{
		{name: "an undecodable item list", key: "list items", out: []byte("{"), run: func(v BitwardenVault) *application.VaultFault {
			_, fault := v.Find("app")
			return fault
		}},
		{name: "an undecodable created item", key: "create item", out: []byte("{"), run: func(v BitwardenVault) *application.VaultFault {
			_, fault := v.CreateLogin("fresh", "pw")
			return fault
		}},
		{name: "a null item to edit", key: "get item", out: []byte("null"), run: func(v BitwardenVault) *application.VaultFault {
			return v.UpdateLogin(application.VaultItem{ID: "1"}, "new")
		}},
		{name: "an encoder that fails", key: "encode", err: errInjected, run: func(v BitwardenVault) *application.VaultFault {
			return v.UpdateLogin(application.VaultItem{ID: "1"}, "new")
		}},
		{name: "an undecodable created attachment", key: "create attachment", out: []byte("{"), run: func(v BitwardenVault) *application.VaultFault {
			_, fault := v.AddAttachment("1", "x.txt", []byte("data"))
			return fault
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := repository(&callLog{}, nil)
			commands := &fakeCommands{log: &callLog{}, respond: bwOverride(m, "unlocked", tc.key, tc.out, tc.err)}
			useDoubles(t, m, commands, fakeEnv{"BW_SESSION": "s", "PATH": "/usr/bin"})
			if fault := tc.run(BitwardenVault{}); fault == nil {
				t.Fatalf("%s was accepted, want a refusal", tc.name)
			}
		})
	}
}

func TestBitwardenNamesANoteAndFindsAnExecutableBehindAnEmptyPathEntry(t *testing.T) {
	m := repository(&callLog{}, nil)
	useDoubles(t, m, &fakeCommands{log: &callLog{}, respond: bwAnswers(m, "unlocked")}, fakeEnv{"BW_SESSION": "s", "PATH": "/usr/bin"})
	items, fault := (BitwardenVault{}).Find("app-other")
	if fault != nil || len(items) != 1 || items[0].Kind != "note" {
		t.Fatalf("Find(app-other) = %+v, %+v; want one note", items, fault)
	}

	// An empty PATH entry means the working directory. A bare name there is
	// asked by its ./ path, where the permission failure shows.
	lookup := func(file string) (string, error) {
		switch file {
		case "./" + bitwardenExecutable:
			return "", fs.ErrPermission
		default:
			return "", hostErrNotFound
		}
	}
	useDoubles(t, m, &fakeCommands{log: &callLog{}, lookPath: lookup}, fakeEnv{"BW_SESSION": "s", "PATH": ":/usr/bin"})
	fault = (BitwardenVault{}).Status("https://vault.example")
	if fault == nil || fault.Reason != reasonNotExecutable || fault.Exit != statusChildNotExecutable {
		t.Fatalf("Status with an empty PATH entry = %+v, want a not-executable refusal", fault)
	}
}

// walkingFS is a file system double whose walk the test scripts.
type walkingFS struct {
	*memFS
	walk func(visit fs.WalkDirFunc) error
}

func (w walkingFS) WalkDir(_ string, visit fs.WalkDirFunc) error { return w.walk(visit) }

func listedEntry(name string, dir bool) fs.DirEntry {
	return memDirEntry{info: memInfo{name: name, node: &memNode{dir: dir, perm: 0o644}}}
}

func TestTreeListingsKeepWhatTheWalkCanRead(t *testing.T) {
	double := walkingFS{memFS: repository(&callLog{}, nil), walk: func(visit fs.WalkDirFunc) error {
		if err := visit("/repo/locked", listedEntry("locked", true), errInjected); err != nil {
			return err
		}
		if err := visit("/repo/b.txt", listedEntry("b.txt", false), nil); err != nil {
			return err
		}
		return visit("/repo/a.txt", listedEntry("a.txt", false), nil)
	}}
	useDoubles(t, double, nil, nil)
	if got := (&DiskTree{Dir: "/repo"}).Files(); len(got) != 2 || got[0] != "a.txt" || got[1] != "b.txt" {
		t.Fatalf("Files = %v, want the two readable files", got)
	}
}

func TestTreeListingsFailLoudlyWhenTheWalkBreaksItsInvariant(t *testing.T) {
	broken := walkingFS{memFS: repository(&callLog{}, nil), walk: func(fs.WalkDirFunc) error { return errInjected }}
	useDoubles(t, broken, nil, nil)
	expectPanic(t, "listing the tree broke", func() { (&DiskTree{Dir: "/repo"}).Files() })
	expectPanic(t, "listing the links broke", func() { (&DiskTree{Dir: "/repo"}).Links() })

	// A walk that produces a path a relative root cannot relate fails the same way.
	relative := walkingFS{memFS: repository(&callLog{}, nil), walk: func(visit fs.WalkDirFunc) error {
		return visit("/elsewhere/a.txt", listedEntry("a.txt", false), nil)
	}}
	useDoubles(t, relative, nil, nil)
	expectPanic(t, "listing the tree broke", func() { (&DiskTree{Dir: "repo"}).Files() })
}

// expectPanic runs one operation and fails the test unless it panics with a
// message that contains want.
func expectPanic(t *testing.T, want string, run func()) {
	t.Helper()
	defer func() {
		got := recover()
		if got == nil || !strings.Contains(fmt.Sprint(got), want) {
			t.Fatalf("recovered %v, want a panic naming %q", got, want)
		}
	}()
	run()
}

func TestStagedFileRemovalJoinsOnlyAFailureThatLeavesTheFileBehind(t *testing.T) {
	failure := errors.New("the write failed")
	useDoubles(t, repository(&callLog{}, map[string]string{"stage": "x"}), nil, nil)
	if err := finishStagedFile(failure, "/repo/stage"); err != failure {
		t.Fatalf("a removed staging file = %v, want the failure unchanged", err)
	}
	if err := finishStagedFile(failure, "/repo/missing"); err != failure {
		t.Fatalf("an absent staging file = %v, want the failure unchanged", err)
	}

	useDoubles(t, repository(&callLog{failOps: map[string]bool{"Remove": true}}, map[string]string{"stage": "x"}), nil, nil)
	err := finishStagedFile(failure, "/repo/stage")
	if !errors.Is(err, failure) || !errors.Is(err, errInjected) || !strings.Contains(err.Error(), "cannot remove staging file") {
		t.Fatalf("a failed removal = %v, want both the failure and the removal reported", err)
	}
}

func TestChildOutputLimitsAndMissingStatusAreRefusals(t *testing.T) {
	over := &boundedCapture{total: maxModelOutput + 1}
	if _, err := finishModel(&hostCommand{}, over); err == nil || !strings.Contains(err.Error(), "too much output") {
		t.Fatalf("finishModel over the limit = %v, want a too-much-output refusal", err)
	}
	if _, err := finishModel(&hostCommand{}, &boundedCapture{}); err == nil || !strings.Contains(err.Error(), "ended by signal") {
		t.Fatalf("finishModel without a status = %v, want an ended-by-signal refusal", err)
	}
	toolOver := &boundedCapture{total: maxToolchainOutput + 1}
	if _, err := finishToolchain(&hostCommand{}, toolOver); err == nil || !strings.Contains(err.Error(), "too much output") {
		t.Fatalf("finishToolchain over the limit = %v, want a too-much-output refusal", err)
	}
	if _, err := finishToolchain(&hostCommand{}, &boundedCapture{}); err == nil || !strings.Contains(err.Error(), "ended by signal") {
		t.Fatalf("finishToolchain without a status = %v, want an ended-by-signal refusal", err)
	}
}

func TestBoundedCaptureKeepsOneByteBeyondItsLimitAndCountsAll(t *testing.T) {
	capture := &boundedCapture{limit: 4}
	if n, err := capture.Write([]byte("abcdefgh")); n != 8 || err != nil {
		t.Fatalf("Write = %d, %v; want 8 and no error", n, err)
	}
	if string(capture.kept) != "abcde" || capture.total != 8 {
		t.Fatalf("kept %q of %d; want the limit plus one byte, counting all 8", capture.kept, capture.total)
	}
	short := &boundedCapture{limit: 4}
	_, _ = short.Write([]byte("ab"))
	if string(short.kept) != "ab" || short.total != 2 {
		t.Fatalf("a short write kept %q of %d; want all of it", short.kept, short.total)
	}
}

func TestKillFailureIgnoresOnlyAChildThatAlreadyExited(t *testing.T) {
	if killFailure(nil) != nil || killFailure(os.ErrProcessDone) != nil {
		t.Fatal("a kill of nothing, or of an exited child, reported a failure")
	}
	if err := killFailure(fmt.Errorf("signal: %w", os.ErrProcessDone)); err != nil {
		t.Fatalf("a wrapped exited child reported %v", err)
	}
	if err := killFailure(errInjected); err != errInjected {
		t.Fatalf("a refused kill = %v, want the failure", err)
	}
}

// A header of five fields with one of them empty is refused by the field check,
// which the count check before it does not reach.
func TestParseRawHeaderRefusesAnEmptyFieldWhenTheCountIsRight(t *testing.T) {
	zeros := "0000000000000000000000000000000000000000"
	if _, _, err := parseRawHeader(": 100644 " + zeros + " abc A"); err == nil {
		t.Fatal("parseRawHeader accepted a header with an empty source mode")
	}
}
