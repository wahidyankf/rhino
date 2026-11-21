package runtime

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// The child refusals below run through the command double. A respond function
// stands in for the child: it may write the child's output into its capture,
// and it may fail the start. A child that never starts reports no status, so
// each case reaches the refusal that names the missing status or the limit.

func TestModelResolverRefusesAnUnusableCommandOrOutput(t *testing.T) {
	oversized := func(c *hostCommand) ([]byte, error) {
		_, _ = c.Stdout.Write(make([]byte, maxModelOutput+1))
		return nil, nil
	}
	cases := []struct {
		name    string
		launch  application.ModelLaunch
		respond func(c *hostCommand) ([]byte, error)
		fail    bool
		want    string
	}{
		{name: "no executable", launch: application.ModelLaunch{TimeoutSeconds: 5}, want: "declares no executable"},
		{name: "blank executable", launch: application.ModelLaunch{Arguments: []string{" "}, TimeoutSeconds: 5}, want: "executable is invalid"},
		{name: "option as executable", launch: application.ModelLaunch{Arguments: []string{"-x"}, TimeoutSeconds: 5}, want: "executable is invalid"},
		{name: "refused start", launch: application.ModelLaunch{Arguments: []string{"models"}, TimeoutSeconds: 5}, fail: true, want: "could not start model command"},
		{name: "a child with no status", launch: application.ModelLaunch{Arguments: []string{"models"}, TimeoutSeconds: 5}, want: "ended by signal"},
		{name: "oversized output", launch: application.ModelLaunch{Arguments: []string{"models"}, TimeoutSeconds: 5}, respond: oversized, want: "too much output"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log := &callLog{}
			if tc.fail {
				log.failOps = map[string]bool{"Start": true}
			}
			useDoubles(t, repository(&callLog{}, nil), &fakeCommands{log: log, respond: tc.respond}, fakeEnv{"PATH": "/usr/bin"})
			if _, err := (DiskModelResolver{}).Resolve(tc.launch); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Resolve = %v, want a refusal containing %q", err, tc.want)
			}
		})
	}
}

func TestToolchainRunRefusesAnUnusableCommandOrOutput(t *testing.T) {
	oversized := func(c *hostCommand) ([]byte, error) {
		_, _ = c.Stdout.Write(make([]byte, maxToolchainOutput+1))
		return nil, nil
	}
	cases := []struct {
		name    string
		launch  application.ToolchainLaunch
		respond func(c *hostCommand) ([]byte, error)
		fail    bool
		want    string
	}{
		{name: "blank executable", launch: application.ToolchainLaunch{Executable: " "}, want: "executable is invalid"},
		{name: "refused start", launch: application.ToolchainLaunch{Executable: "tool"}, fail: true, want: "could not start declared toolchain"},
		{name: "a child with no status", launch: application.ToolchainLaunch{Executable: "tool"}, want: "ended by signal"},
		{name: "oversized output", launch: application.ToolchainLaunch{Executable: "tool"}, respond: oversized, want: "too much output"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log := &callLog{}
			if tc.fail {
				log.failOps = map[string]bool{"Start": true}
			}
			useDoubles(t, repository(&callLog{}, nil), &fakeCommands{log: log, respond: tc.respond}, fakeEnv{"PATH": "/usr/bin"})
			if _, err := (DiskToolchainRunner{}).Run(tc.launch); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Run = %v, want a refusal containing %q", err, tc.want)
			}
		})
	}
}

// The mutation helpers refuse what they cannot verify: a child that ends by a
// signal or cannot start, a program that is not on the path, a link among the
// parents, a parent that cannot be inspected, and a path that leaves the root.
func TestMutationHelpersRefuseWhatTheyCannotVerify(t *testing.T) {
	signalled := gitResponder(mutationRepository(&callLog{}), defaultScript, func(*memFS, string) error { return execFailure() })
	m := mutationRepository(&callLog{})
	useDoubles(t, m, &fakeCommands{log: &callLog{}, respond: signalled}, fakeEnv{"PATH": "/usr/bin"})
	if _, err := runMutator("/repo", application.MutationLaunch{Argv: []string{"mutator"}, Directory: "/repo"}); err == nil || !strings.Contains(err.Error(), "ended by a signal") {
		t.Fatalf("runMutator with a signalled child = %v, want an ended-by-signal refusal", err)
	}
	refused := gitResponder(mutationRepository(&callLog{}), defaultScript, func(*memFS, string) error { return errInjected })
	useDoubles(t, mutationRepository(&callLog{}), &fakeCommands{log: &callLog{}, respond: refused}, fakeEnv{"PATH": "/usr/bin"})
	if _, err := runMutator("/repo", application.MutationLaunch{Argv: []string{"mutator"}, Directory: "/repo"}); err == nil || !strings.Contains(err.Error(), "could not be started") {
		t.Fatalf("runMutator with a refused child = %v, want a could-not-start refusal", err)
	}
	useDoubles(t, mutationRepository(&callLog{}), &fakeCommands{log: &callLog{}, respond: refused}, fakeEnv{"PATH": "/usr/bin"})
	if _, err := runMutator("/repo", application.MutationLaunch{Argv: []string{"missing"}, Directory: "/repo"}); err == nil {
		t.Fatal("runMutator found a program that is not on the path")
	}

	linked := mutationRepository(&callLog{})
	linked.nodes["/repo/link"] = &memNode{link: "elsewhere"}
	useDoubles(t, linked, nil, nil)
	if err := ensureNoLinkedParents("/repo", "link/x"); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("ensureNoLinkedParents through a link = %v, want a refusal", err)
	}
	if err := replaceWorkingFile("/repo", "/snapshot", "link/x"); err == nil {
		t.Fatal("replaceWorkingFile wrote through a link")
	}
	if err := ensureNoLinkedParents("/repo", "missing/x"); err != nil {
		t.Fatalf("ensureNoLinkedParents through a missing parent = %v, want no refusal", err)
	}
	useDoubles(t, repository(&callLog{failOps: map[string]bool{"Lstat": true}}, nil), nil, nil)
	if err := ensureNoLinkedParents("/repo", "dir/x"); err == nil || !strings.Contains(err.Error(), "could not inspect") {
		t.Fatalf("ensureNoLinkedParents with a failed inspection = %v, want a refusal", err)
	}

	outside := walkingFS{memFS: repository(&callLog{}, nil), walk: func(visit fs.WalkDirFunc) error {
		return visit("/abs/a.txt", listedEntry("a.txt", false), nil)
	}}
	useDoubles(t, outside, nil, nil)
	if _, err := snapshotDigests("snapshot"); err == nil || !strings.Contains(err.Error(), "leaves its root") {
		t.Fatalf("snapshotDigests of a relative root = %v, want a refusal", err)
	}
	escaping := walkingFS{memFS: repository(&callLog{}, nil), walk: func(visit fs.WalkDirFunc) error {
		return visit("/x/a.txt", listedEntry("a.txt", false), nil)
	}}
	useDoubles(t, escaping, nil, nil)
	if _, err := snapshotDigests("/snapshot"); err == nil || !strings.Contains(err.Error(), "escaping path") {
		t.Fatalf("snapshotDigests with an escaping path = %v, want a refusal", err)
	}

	raw := &fakeCommands{log: &callLog{}, respond: func(*hostCommand) ([]byte, error) { return []byte("../escape\x00"), nil }}
	useDoubles(t, mutationRepository(&callLog{}), raw, fakeEnv{"PATH": "/usr/bin"})
	if _, err := gitPaths("/repo", "diff", "-z"); err == nil || !strings.Contains(err.Error(), "escaping changed path") {
		t.Fatalf("gitPaths with an escaping record = %v, want a refusal", err)
	}
}
