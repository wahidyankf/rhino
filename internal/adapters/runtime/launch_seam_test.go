package runtime

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// failingChildren is a commandRunner whose every child fails with err.
func failingChildren(err error) *fakeCommands {
	return &fakeCommands{log: &callLog{}, respond: func(*hostCommand) ([]byte, error) { return nil, err }}
}

func TestOSLauncherClassifiesEachChildOutcome(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	cases := []struct {
		name string
		argv []string
		err  error
		ctx  context.Context
		want application.Launched
	}{
		{name: "no executable", want: application.Launched{Refused: "declares no executable command"}},
		{name: "success", argv: []string{"gate"}, want: application.Launched{Code: 0}},
		{name: "exit status", argv: []string{"gate"}, err: execFailure(), want: application.Launched{Code: -1}},
		{name: "not found", argv: []string{"gate"}, err: hostErrNotFound, want: application.Launched{NotFound: true}},
		{name: "not executable", argv: []string{"gate"}, err: fs.ErrPermission, want: application.Launched{NotExecutable: true}},
		{name: "refused", argv: []string{"gate"}, err: errInjected, want: application.Launched{Refused: "injected failure"}},
		{name: "interrupted", argv: []string{"gate"}, err: errInjected, ctx: canceled,
			want: application.Launched{Refused: "gate child interrupted"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useDoubles(t, nil, failingChildren(tc.err), fakeEnv{"PATH": "/usr/bin"})
			got := OSLauncher{Ctx: tc.ctx}.Launch(application.GateLaunch{Argv: tc.argv, Surface: "pre-push"})
			if got != tc.want {
				t.Fatalf("Launch = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestOSLauncherGivesTheChildItsStdinAndSurface(t *testing.T) {
	var seen *hostCommand
	commands := &fakeCommands{log: &callLog{}, respond: func(command *hostCommand) ([]byte, error) {
		seen = command
		return nil, nil
	}}
	useDoubles(t, nil, commands, fakeEnv{"HOME": "/home/x", "RHINO_GATE_SURFACE": "forged"})
	stdin := "update\n"
	launch := application.GateLaunch{
		Argv: []string{"gate"}, Directory: "/repo", Surface: "pre-push", Stdin: &stdin,
		Environment: map[string]string{"A": "1"},
	}
	if got := (OSLauncher{}).Launch(launch); got.Code != 0 || got.Refused != "" {
		t.Fatalf("Launch = %+v", got)
	}
	if seen == nil || seen.Stdin == nil || seen.Dir != "/repo" {
		t.Fatalf("child = %+v, want stdin and directory", seen)
	}
	joined := strings.Join(seen.Env, " ")
	for _, want := range []string{"A=1", "HOME=/home/x", "RHINO_GATE_SURFACE=pre-push"} {
		if !strings.Contains(joined, want) {
			t.Errorf("child environment lacks %s: %s", want, joined)
		}
	}
	if strings.Contains(joined, "forged") {
		t.Errorf("a forged surface reached the child: %s", joined)
	}
}

func TestGateEnvironmentOverwritesByNameAndSorts(t *testing.T) {
	useDoubles(t, nil, nil, fakeEnv{"Z": "inherited", "A": "inherited"})
	env := gateEnvironment(application.GateLaunch{
		Surface: "local", Environment: map[string]string{"A": "projected"},
	})
	want := []string{"A=projected", "RHINO_GATE_SURFACE=local", "Z=inherited"}
	if !equalStrings(env, want) {
		t.Fatalf("gateEnvironment = %v, want %v", env, want)
	}
}

func TestStartStatusMapsOnlyTheTwoRequiredStatuses(t *testing.T) {
	cases := map[error]int{
		hostErrNotFound: statusChildNotFound,
		fmt.Errorf("wrapped: %w", fs.ErrNotExist): statusChildNotFound,
		fs.ErrPermission: statusChildNotExecutable,
		errInjected:      0,
	}
	for err, want := range cases {
		if got := startStatus(err); got != want {
			t.Errorf("startStatus(%v) = %d, want %d", err, got, want)
		}
	}
}

func TestDiskToolchainRunnerRefusesInvalidAndUnstartableVectors(t *testing.T) {
	for _, executable := range []string{"", "  ", "-x"} {
		if _, err := (DiskToolchainRunner{}).Run(application.ToolchainLaunch{Executable: executable}); err == nil {
			t.Errorf("Run(%q) succeeded, want a refusal", executable)
		}
	}
	cases := []struct {
		name    string
		err     error
		status  int
		wrapped bool
	}{
		{name: "not found", err: hostErrNotFound, status: statusChildNotFound, wrapped: true},
		{name: "not executable", err: fs.ErrPermission, status: statusChildNotExecutable, wrapped: true},
		{name: "other start failure", err: errInjected},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useDoubles(t, nil, failingChildren(tc.err), fakeEnv{"PATH": "/usr/bin"})
			_, err := (DiskToolchainRunner{}).Run(application.ToolchainLaunch{Executable: "tool"})
			var start *application.ChildStartError
			if tc.wrapped {
				if !errors.As(err, &start) || start.Status != tc.status {
					t.Fatalf("Run = %v, want a child start error with status %d", err, tc.status)
				}
				return
			}
			if err == nil || errors.As(err, &start) {
				t.Fatalf("Run = %v, want a plain start failure", err)
			}
		})
	}
	useDoubles(t, nil, &fakeCommands{log: &callLog{}}, fakeEnv{"PATH": "/usr/bin"})
	timeout := uint64(3600)
	if _, err := (DiskToolchainRunner{}).Run(application.ToolchainLaunch{Executable: "tool", TimeoutSeconds: &timeout}); err == nil {
		t.Fatal("a child that never ran reported a status")
	}
}

func TestRegularFilePathsSelectsRegularFilesFromRawRecords(t *testing.T) {
	good := []byte(":000000 100644 0000000000000000000000000000000000000000 abcdef0 A\x00b.txt\x00" +
		":000000 100755 0000000000000000000000000000000000000000 abcdef1 A\x00a.sh\x00" +
		":100644 000000 abcdef0 0000000000000000000000000000000000000000 D\x00gone.txt\x00")
	paths, err := regularFilePaths(good)
	if err != nil || !equalStrings(paths, []string{"a.sh", "b.txt"}) {
		t.Fatalf("regularFilePaths = %v, %v", paths, err)
	}
	if paths, err := regularFilePaths(nil); err != nil || paths != nil {
		t.Fatalf("regularFilePaths(empty) = %v, %v", paths, err)
	}
	refused := map[string][]byte{
		"truncated":      []byte(":000000 100644 0000000000000000000000000000000000000000 abcdef0 A\x00b.txt"),
		"doubled NUL":    []byte(":h\x00b\x00\x00"),
		"empty header":   []byte("\x00a\x00"),
		"no pathname":    []byte(":000000 100644 0000000000000000000000000000000000000000 abcdef0 A\x00"),
		"empty pathname": []byte(":000000 100644 0000000000000000000000000000000000000000 abcdef0 A\x00\x00x\x00"),
		"bad header":     []byte("bad\x00a\x00"),
		"non-UTF-8 path": []byte(":000000 100644 0000000000000000000000000000000000000000 abcdef0 A\x00\xff\x00"),
		"escaping path":  []byte(":000000 100644 0000000000000000000000000000000000000000 abcdef0 A\x00../x\x00"),
	}
	for name, raw := range refused {
		if _, err := regularFilePaths(raw); err == nil {
			t.Errorf("regularFilePaths(%s) accepted the record", name)
		}
	}
}

func TestParseRawHeaderRefusesEveryMalformedHeader(t *testing.T) {
	zeros := "0000000000000000000000000000000000000000"
	malformed := []string{
		"000000 100644 " + zeros + " abc A",   // no colon
		":000000 100644 " + zeros + " abc",    // too few fields
		":000000  100644 " + zeros + " abc A", // empty field
		":123456 100644 " + zeros + " abc A",  // unsupported mode
		":000000 100644 xyz abc A",            // non-hex object ID
		":000000 100644 " + zeros + " abc AB", // status too long
		":000000 100644 " + zeros + " abc 1",  // digit status
		":000000 100644 " + zeros + " abc X",  // unknown status
		":100644 100644 abcdef0 abcdef1 A",    // source mode disagrees with an add
		":100644 100644 abcdef0 " + zeros + " D",
		":000000 100644 " + zeros + " abc M", // modify with no source
		":100644 000000 abcdef0 " + zeros + " T",
		":000000 000000 " + zeros + " abc A", // add with no destination mode
		":000000 100644 abc " + zeros + " A", // zero source object with a file mode
	}
	for _, header := range malformed {
		if _, _, err := parseRawHeader(header); err == nil {
			t.Errorf("parseRawHeader(%q) accepted the header", header)
		}
	}
	if mode, status, err := parseRawHeader(":100644 100755 abcdef0 abcdef1 M"); err != nil || mode != "100755" || status != 'M' {
		t.Fatalf("parseRawHeader(modify) = %q, %c, %v", mode, status, err)
	}
}

func TestRawModesIdsAndZeroTestsAcceptOnlyTheirForms(t *testing.T) {
	for _, mode := range []string{"000000", "100644", "100755", "120000", "160000"} {
		if !validRawMode(mode) {
			t.Errorf("validRawMode(%s) = false", mode)
		}
	}
	if validRawMode("100600") {
		t.Error("validRawMode accepted an unrecorded mode")
	}
	if !validRawID("abcDEF0123") || validRawID("") || validRawID("xyz") || validRawID(strings.Repeat("a", 65)) {
		t.Error("validRawID accepted a value that is not an object ID")
	}
	if !allZero("000") || allZero("") || allZero("00a") {
		t.Error("allZero misclassified an object ID")
	}
}
