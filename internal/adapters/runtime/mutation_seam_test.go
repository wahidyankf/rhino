package runtime

import (
	"path"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// mutationRepository is the working tree a mutation runs against: a.txt and
// b.txt, and a mutator executable on the PATH.
func mutationRepository(log *callLog) *memFS {
	m := repository(log, map[string]string{"a.txt": "old", "b.txt": "keep"})
	_ = m.mkdirAll("/usr/bin", 0o755)
	m.nodes["/usr/bin/mutator"] = &memNode{perm: 0o755}
	return m
}

// gitScript is what the Git repository answers the mutation's queries with.
type gitScript struct {
	index      map[string]string
	diff       string
	untracked  string
	notRepo    bool
	failAdd    bool
	failRemove bool
	failDiff   bool
}

var defaultScript = gitScript{index: map[string]string{"a.txt": "old", "b.txt": "keep"}}

// answerGit answers one Git command from the script. checkout-index writes the
// index into the snapshot it names, as Git does.
func answerGit(m *memFS, script gitScript, args []string) ([]byte, error) {
	switch args[0] {
	case "rev-parse":
		if script.notRepo {
			return nil, execFailure()
		}
		return []byte("true\n"), nil
	case "checkout-index":
		prefix := strings.TrimPrefix(args[1], "--prefix=")
		for name, data := range script.index {
			_ = m.mkdirAll(path.Dir(prefix+name), 0o755)
			m.nodes[path.Clean(prefix+name)] = &memNode{perm: 0o644, data: []byte(data)}
		}
		return nil, nil
	case "ls-files":
		if args[1] == "--stage" {
			name := args[len(args)-1]
			if _, indexed := script.index[name]; !indexed {
				return nil, nil
			}
			return []byte("100644 " + strings.Repeat("a", 40) + " 0\t" + name + "\n"), nil
		}
		return []byte(script.untracked), nil
	case "hash-object":
		return []byte(strings.Repeat("b", 40) + "\n"), nil
	case "diff":
		if script.failDiff {
			return nil, execFailure()
		}
		return []byte(script.diff), nil
	case "worktree":
		if (args[1] == "add" && script.failAdd) || (args[1] == "remove" && script.failRemove) {
			return nil, execFailure()
		}
	}
	return nil, nil
}

// gitResponder answers Git commands from the script, and every other child by
// running the mutator's behaviour in the child's directory.
func gitResponder(m *memFS, script gitScript, mutate func(m *memFS, dir string) error) func(*hostCommand) ([]byte, error) {
	return func(command *hostCommand) ([]byte, error) {
		if command.Args[0] != "git" {
			return nil, mutate(m, command.Dir)
		}
		return answerGit(m, script, command.Args[3:])
	}
}

// noMutation is a mutator that changes nothing and succeeds.
func noMutation(*memFS, string) error { return nil }

// editA is a mutator that rewrites a.txt in its directory.
func editA(m *memFS, dir string) error {
	_ = m.mkdirAll(dir, 0o755)
	m.nodes[dir+"/a.txt"] = &memNode{perm: 0o644, data: []byte("new")}
	return nil
}

// applyLaunch asks for an index mutation over the named paths.
func applyLaunch(selected ...string) application.MutationLaunch {
	return application.MutationLaunch{Argv: []string{"mutator"}, Directory: "/repo", SelectedPaths: selected, Surface: "local"}
}

// mutationScenario builds one mutation run for faultSweep.
func mutationScenario(script gitScript, fixture func(m *memFS), mutate func(m *memFS, dir string) error, run func(runner DiskMutationRunner) error) func(t *testing.T, log *callLog) func() error {
	return func(t *testing.T, log *callLog) func() error {
		m := mutationRepository(log)
		if fixture != nil {
			fixture(m)
		}
		commands := &fakeCommands{log: log, respond: gitResponder(m, script, mutate)}
		useDoubles(t, m, commands, fakeEnv{"PATH": "/usr/bin"})
		return func() error { return run(DiskMutationRunner{}) }
	}
}

func TestDiskMutationApplyIndexWritesBackOnlyTheSelectedChange(t *testing.T) {
	m := mutationRepository(&callLog{})
	commands := &fakeCommands{log: &callLog{}, respond: gitResponder(m, defaultScript, editA)}
	useDoubles(t, m, commands, fakeEnv{"PATH": "/usr/bin"})
	outcome, err := (DiskMutationRunner{}).ApplyIndex(applyLaunch("a.txt"))
	if err != nil || len(outcome.Changes) != 1 || outcome.Changes[0] != "a.txt" || len(outcome.Divergences) != 0 {
		t.Fatalf("ApplyIndex = %+v, %v; want a.txt changed", outcome, err)
	}
	if got := string(m.get("/repo/a.txt").data); got != "new" {
		t.Fatalf("working a.txt = %q, want new", got)
	}
	if got := string(m.get("/repo/b.txt").data); got != "keep" {
		t.Fatalf("unselected b.txt = %q, want keep", got)
	}
}

func TestDiskMutationApplyIndexDeletesAndKeepsUserEdits(t *testing.T) {
	deleteB := func(m *memFS, dir string) error {
		delete(m.nodes, dir+"/b.txt")
		return nil
	}
	m := mutationRepository(&callLog{})
	commands := &fakeCommands{log: &callLog{}, respond: gitResponder(m, defaultScript, deleteB)}
	useDoubles(t, m, commands, fakeEnv{"PATH": "/usr/bin"})
	outcome, err := (DiskMutationRunner{}).ApplyIndex(applyLaunch("b.txt"))
	if err != nil || len(outcome.Changes) != 1 || m.get("/repo/b.txt") != nil {
		t.Fatalf("deletion = %+v, %v; want b.txt removed from the working tree", outcome, err)
	}

	edited := mutationRepository(&callLog{})
	edited.get("/repo/a.txt").data = []byte("user")
	commands = &fakeCommands{log: &callLog{}, respond: gitResponder(edited, defaultScript, editA)}
	useDoubles(t, edited, commands, fakeEnv{"PATH": "/usr/bin"})
	outcome, err = (DiskMutationRunner{}).ApplyIndex(applyLaunch("a.txt"))
	if err != nil || len(outcome.Divergences) != 1 || outcome.Divergences[0] != "a.txt" {
		t.Fatalf("divergence = %+v, %v; want a.txt diverged", outcome, err)
	}
	if got := string(edited.get("/repo/a.txt").data); got != "user" {
		t.Fatalf("a user edit was overwritten with %q", got)
	}
}

func TestDiskMutationApplyIndexRefusesEachUnsafeRun(t *testing.T) {
	cases := map[string]struct {
		script gitScript
		launch application.MutationLaunch
		mutate func(m *memFS, dir string) error
	}{
		"not a work tree": {
			script: gitScript{notRepo: true}, launch: applyLaunch("a.txt"), mutate: noMutation},
		"escaping selection": {
			script: defaultScript, launch: applyLaunch("../x"), mutate: noMutation},
		"no declared command": {
			script: defaultScript, launch: application.MutationLaunch{Directory: "/repo", SelectedPaths: []string{"a.txt"}}, mutate: noMutation},
		"unknown executable": {
			script: defaultScript, launch: application.MutationLaunch{Argv: []string{"missing"}, Directory: "/repo", SelectedPaths: []string{"a.txt"}}, mutate: noMutation},
		"change outside the selection": {
			script: defaultScript, launch: applyLaunch("a.txt"), mutate: func(m *memFS, dir string) error {
				m.get(dir + "/b.txt").data = []byte("changed")
				return nil
			}},
		"mutator ended by a signal": {
			script: defaultScript, launch: applyLaunch("a.txt"), mutate: func(*memFS, string) error { return execFailure() }},
		"mutator could not start": {
			script: defaultScript, launch: applyLaunch("a.txt"), mutate: func(*memFS, string) error { return errInjected }},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := mutationRepository(&callLog{})
			commands := &fakeCommands{log: &callLog{}, respond: gitResponder(m, tc.script, tc.mutate)}
			useDoubles(t, m, commands, fakeEnv{"PATH": "/usr/bin"})
			if _, err := (DiskMutationRunner{}).ApplyIndex(tc.launch); err == nil {
				t.Fatalf("ApplyIndex(%s) succeeded, want a refusal", name)
			}
		})
	}
	empty := application.MutationLaunch{Argv: []string{"mutator"}, Directory: "/repo"}
	useDoubles(t, mutationRepository(&callLog{}), &fakeCommands{log: &callLog{}}, fakeEnv{"PATH": "/usr/bin"})
	if outcome, err := (DiskMutationRunner{}).ApplyIndex(empty); err != nil || outcome.Code != 0 {
		t.Fatalf("an empty selection = %+v, %v; want a no-op", outcome, err)
	}
}

func TestDiskMutationApplyIndexSweepsEveryCall(t *testing.T) {
	apply := func(runner DiskMutationRunner) error {
		_, err := runner.ApplyIndex(applyLaunch("a.txt"))
		return err
	}
	faultSweep(t, mutationScenario(defaultScript, nil, editA, apply), "ReadFile", "Stat")
	remove := func(runner DiskMutationRunner) error {
		_, err := runner.ApplyIndex(applyLaunch("b.txt"))
		return err
	}
	deleteB := func(m *memFS, dir string) error {
		delete(m.nodes, dir+"/b.txt")
		return nil
	}
	faultSweep(t, mutationScenario(defaultScript, nil, deleteB, remove), "ReadFile", "Stat")
}

func TestDiskMutationVerifyCleanReportsWhatTheReplayChanged(t *testing.T) {
	cases := []struct {
		name    string
		script  gitScript
		mutate  func(m *memFS, dir string) error
		code    int
		changes []string
	}{
		{name: "clean replay", script: defaultScript, mutate: noMutation, code: 0},
		{name: "created path", script: gitScript{index: defaultScript.index, untracked: "new.txt\x00"},
			mutate: noMutation, code: 1, changes: []string{"new.txt"}},
		{name: "tracked path", script: gitScript{index: defaultScript.index, diff: "a.txt\x00"},
			mutate: noMutation, code: 1, changes: []string{"a.txt"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := mutationRepository(&callLog{})
			commands := &fakeCommands{log: &callLog{}, respond: gitResponder(m, tc.script, tc.mutate)}
			useDoubles(t, m, commands, fakeEnv{"PATH": "/usr/bin"})
			launch := applyLaunch()
			launch.Revision = "abc123"
			outcome, err := (DiskMutationRunner{}).VerifyClean(launch)
			if err != nil || outcome.Code != tc.code || !equalStrings(outcome.Changes, tc.changes) {
				t.Fatalf("VerifyClean = %+v, %v; want code %d changes %v", outcome, err, tc.code, tc.changes)
			}
		})
	}

	withRevision := func(runner DiskMutationRunner) error {
		launch := applyLaunch()
		launch.Revision = "abc123"
		_, err := runner.VerifyClean(launch)
		return err
	}
	faultSweep(t, mutationScenario(defaultScript, nil, noMutation, withRevision), "Stat")
	faultSweep(t, mutationScenario(gitScript{index: defaultScript.index, diff: "a.txt\x00"}, nil, noMutation, withRevision), "Stat")

	refusals := map[string]struct {
		script gitScript
		mutate func(m *memFS, dir string) error
		revise bool
	}{
		"no revision":      {script: defaultScript, mutate: noMutation},
		"replay refused":   {script: gitScript{index: defaultScript.index, failAdd: true}, mutate: noMutation, revise: true},
		"mutator failed":   {script: defaultScript, mutate: func(*memFS, string) error { return execFailure() }, revise: true},
		"cleanup refused":  {script: gitScript{index: defaultScript.index, failRemove: true}, mutate: noMutation, revise: true},
		"replay diff lost": {script: gitScript{index: defaultScript.index, failDiff: true}, mutate: noMutation, revise: true},
	}
	for name, tc := range refusals {
		t.Run(name, func(t *testing.T) {
			m := mutationRepository(&callLog{})
			commands := &fakeCommands{log: &callLog{}, respond: gitResponder(m, tc.script, tc.mutate)}
			useDoubles(t, m, commands, fakeEnv{"PATH": "/usr/bin"})
			launch := applyLaunch()
			if tc.revise {
				launch.Revision = "abc123"
			}
			if _, err := (DiskMutationRunner{}).VerifyClean(launch); err == nil {
				t.Fatalf("VerifyClean(%s) succeeded, want a refusal", name)
			}
		})
	}
}

func TestDiskMutationResolvesTheMutatorOnThePath(t *testing.T) {
	m := mutationRepository(&callLog{})
	m.nodes["/usr/bin/plain"] = &memNode{perm: 0o644}
	m.nodes["/usr/bin/tool"] = &memNode{perm: 0o755}
	useDoubles(t, m, nil, fakeEnv{"PATH": "/usr/bin"})
	if got, err := resolveProgram("tool", "/repo", "/usr/bin", true); err != nil || got != "/usr/bin/tool" {
		t.Fatalf("resolveProgram(tool) = %q, %v", got, err)
	}
	if got, err := resolveProgram("./run", "/repo", "/usr/bin", true); err != nil || got != "./run" {
		t.Fatalf("resolveProgram(path) = %q, %v", got, err)
	}
	for _, name := range []string{"plain", "absent"} {
		if _, err := resolveProgram(name, "/repo", "/usr/bin", true); err == nil {
			t.Fatalf("resolveProgram(%s) found a tool that is not an executable file", name)
		}
	}
	if got, err := resolveProgram("tool", "/repo", "/usr/bin", false); err != nil || got != "/usr/bin/tool" {
		t.Fatalf("resolveProgram from the process PATH = %q, %v", got, err)
	}
	if got, err := resolveProgram("tool", "/repo", ":/usr/bin", true); err != nil || got != "/usr/bin/tool" {
		t.Fatalf("an empty PATH segment = %q, %v", got, err)
	}
}

func TestDiskMutationMutatorPathAndEnvironment(t *testing.T) {
	m := mutationRepository(&callLog{})
	_ = m.mkdirAll("/repo/node_modules/.bin", 0o755)
	useDoubles(t, m, nil, fakeEnv{"PATH": "/usr/bin:relative"})
	pathValue, normalized, err := mutatorPath("/repo", nil)
	if err != nil || !normalized || pathValue != "/repo/node_modules/.bin:/usr/bin:/repo/relative" {
		t.Fatalf("mutatorPath = %q, %v, %v", pathValue, normalized, err)
	}
	if pathValue, normalized, err = mutatorPath("/repo", map[string]string{"PATH": "/x"}); err != nil || pathValue != "/repo/node_modules/.bin:/x" || !normalized {
		t.Fatalf("mutatorPath with a projected PATH = %q, %v, %v", pathValue, normalized, err)
	}
	useDoubles(t, mutationRepository(&callLog{}), nil, fakeEnv{})
	if _, normalized, err := mutatorPath("/repo", nil); err != nil || normalized {
		t.Fatalf("mutatorPath with no PATH normalized = %v, %v", normalized, err)
	}
	if _, _, err := mutatorPath("/re:po", map[string]string{"PATH": "relative"}); err == nil {
		t.Fatal("a root that cannot be represented in PATH was accepted")
	}
	env := mutatorEnvironment("local", map[string]string{"TOOL": "1"}, "/a:/b", true)
	joined := strings.Join(env, "\n")
	for _, want := range []string{"RHINO_GATE_SURFACE=local", "TOOL=1", "PATH=/a:/b"} {
		if !strings.Contains(joined, want) {
			t.Errorf("mutatorEnvironment lacks %s", want)
		}
	}
}

func TestDiskMutationSelectedPathsAreNeverLinked(t *testing.T) {
	m := repository(&callLog{}, map[string]string{"snap/dir/x.txt": "x"})
	symlink(m, "snap/linked", "dir")
	symlink(m, "snap/dir/linkfile", "x.txt")
	useDoubles(t, m, nil, nil)
	if err := ensureSelectedNotLinked("/repo/snap", []string{"dir/x.txt"}); err != nil {
		t.Fatalf("a plain path = %v", err)
	}
	if err := ensureSelectedNotLinked("/repo/snap", []string{"absent/x"}); err != nil {
		t.Fatalf("a missing path = %v", err)
	}
	if err := ensureSelectedNotLinked("/repo/snap", []string{"linked/x"}); err == nil {
		t.Fatal("a path through a link was accepted")
	}
	if err := ensureSelectedNotLinked("/repo/snap", []string{"dir/linkfile"}); err == nil {
		t.Fatal("a selected link was accepted")
	}
	if err := ensureNoLinkedParents("/repo/snap", "linked/x"); err == nil {
		t.Fatal("a write through a link was accepted")
	}
	if err := ensureNoLinkedParents("/repo/snap", "dir/x.txt"); err != nil {
		t.Fatalf("a write into a plain directory = %v", err)
	}
	faultSweep(t, func(t *testing.T, log *callLog) func() error {
		useDoubles(t, repository(log, map[string]string{"snap/dir/x.txt": "x"}), nil, nil)
		return func() error { return ensureSelectedNotLinked("/repo/snap", []string{"dir/x.txt"}) }
	}, "Lstat")
}

func TestDiskMutationDigestsReportEveryDifference(t *testing.T) {
	m := repository(&callLog{}, map[string]string{"snap/a": "1", "snap/b": "2"})
	useDoubles(t, m, nil, nil)
	before, err := snapshotDigests("/repo/snap")
	if err != nil || len(before) != 2 {
		t.Fatalf("snapshotDigests = %v, %v", before, err)
	}
	if _, err := snapshotDigests("/repo/absent"); err == nil {
		t.Fatal("a missing snapshot was digested")
	}
	if got := changedDigests(before, map[string]fileDigest{}); len(got) != 2 {
		t.Fatalf("changedDigests after removal = %v", got)
	}
	if digestMatches(before, "snap/a", "/repo") {
		t.Fatal("a file that is not in the snapshot matched")
	}
	if _, present, err := digestOf("/repo/absent"); present || err != nil {
		t.Fatalf("digestOf(absent) = %v, %v; want absent, nil", present, err)
	}
	faultSweep(t, func(t *testing.T, log *callLog) func() error {
		useDoubles(t, repository(log, map[string]string{"snap/a": "1"}), nil, nil)
		return func() error {
			_, err := snapshotDigests("/repo/snap")
			return err
		}
	})
}

func TestDiskMutationUpdateIndexBlobNeedsAnIndexedPath(t *testing.T) {
	m := mutationRepository(&callLog{})
	commands := &fakeCommands{log: &callLog{}, respond: gitResponder(m, gitScript{}, noMutation)}
	useDoubles(t, m, commands, nil)
	if err := updateIndexBlob("/repo", "/repo/a.txt", "a.txt"); err == nil || !strings.Contains(err.Error(), "absent from the Git index") {
		t.Fatalf("updateIndexBlob of an unindexed path = %v", err)
	}
}
