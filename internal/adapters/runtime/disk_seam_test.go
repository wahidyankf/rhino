package runtime

import (
	"path"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// repository builds an in-memory repository at /repo holding the given files.
func repository(log *callLog, files map[string]string) *memFS {
	m := newMemFS(log)
	for name, contents := range files {
		_ = m.mkdirAll(path.Dir("/repo/"+name), 0o755)
		m.nodes["/repo/"+name] = &memNode{perm: 0o644, data: []byte(contents)}
	}
	return m
}

// symlink adds a symbolic link at /repo/name that points at target.
func symlink(m *memFS, name, target string) {
	m.nodes["/repo/"+name] = &memNode{link: target, perm: 0o777}
}

// linkedRepository is the fixture the tree queries read: regular files, a
// directory, a Git directory, and links inside, outside, dangling, and to the root.
func linkedRepository(log *callLog) *memFS {
	m := repository(log, map[string]string{
		"a.txt":       "A",
		"dir/b.txt":   "B",
		".git/config": "[core]",
		"outside/x":   "X",
	})
	m.nodes["/tmp/outside.txt"] = &memNode{perm: 0o644, data: []byte("O")}
	symlink(m, "in", "a.txt")
	symlink(m, "abs", "/repo/dir")
	symlink(m, "out", "/tmp/outside.txt")
	symlink(m, "dangling", "missing.txt")
	symlink(m, "rootlink", ".")
	symlink(m, "dirlink", "dir")
	return m
}

// answering is a commandRunner response that prints out and ends with err.
func answering(out []byte, err error) func(*hostCommand) ([]byte, error) {
	return func(*hostCommand) ([]byte, error) { return out, err }
}

func TestDiskTreeAtCurrentDirectoryReadsTheWorkingDirectory(t *testing.T) {
	faultSweep(t, func(t *testing.T, log *callLog) func() error {
		useDoubles(t, newMemFS(log), nil, nil)
		return func() error {
			tree, err := AtCurrentDirectory()
			if err == nil && tree.Dir != "/repo" {
				t.Errorf("the working directory is %q, want /repo", tree.Dir)
			}
			return err
		}
	})
}

func TestDiskTreeRootedAtAcceptsOnlyAUsableDirectory(t *testing.T) {
	cases := []struct {
		name  string
		path  string
		want  string
		fails bool
	}{
		{name: "blank", path: "  ", fails: true},
		{name: "missing", path: "missing", fails: true},
		{name: "a file", path: "a.txt", fails: true},
		{name: "relative directory", path: "dir", want: "/repo/dir"},
		{name: "absolute directory", path: "/repo/dir", want: "/repo/dir"},
		{name: "link to a directory", path: "dirlink", want: "/repo/dir"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useDoubles(t, linkedRepository(&callLog{}), nil, nil)
			tree := DiskTree{Dir: "/repo"}
			root, err := tree.RootedAt(tc.path)
			if tc.fails {
				if err == nil {
					t.Fatalf("RootedAt(%q) returned a root, want a refusal", tc.path)
				}
				return
			}
			if err != nil || root.Root() != tc.want {
				t.Fatalf("RootedAt(%q) = %v, %v; want %q", tc.path, root, err, tc.want)
			}
		})
	}
	faultSweep(t, func(t *testing.T, log *callLog) func() error {
		useDoubles(t, linkedRepository(log), nil, nil)
		return func() error {
			_, err := (&DiskTree{Dir: "/repo"}).RootedAt("dirlink")
			return err
		}
	})
}

func TestDiskTreeFilesListsRegularFilesOnly(t *testing.T) {
	useDoubles(t, linkedRepository(&callLog{}), nil, nil)
	got := (&DiskTree{Dir: "/repo"}).Files()
	if want := []string{"a.txt", "dir/b.txt", "outside/x"}; !equalStrings(got, want) {
		t.Fatalf("Files() = %v, want %v", got, want)
	}
	failed := &callLog{failAt: 1, cause: errInjected}
	useDoubles(t, linkedRepository(failed), nil, nil)
	if files := (&DiskTree{Dir: "/repo"}).Files(); len(files) != 0 {
		t.Fatalf("a failed walk listed %v", files)
	}
}

func TestDiskTreeReadClassifiesEachAnswer(t *testing.T) {
	cases := []struct {
		name string
		path string
		kind string
		text string
	}{
		{name: "text", path: "a.txt", text: "A"},
		{name: "escape", path: "../x", kind: "escapes"},
		{name: "through a link", path: "in", kind: "escapes"},
		{name: "missing", path: "nothing.txt", kind: "missing"},
		{name: "directory", path: "dir", kind: "unreadable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useDoubles(t, linkedRepository(&callLog{}), nil, nil)
			text, readErr := (&DiskTree{Dir: "/repo"}).Read(tc.path)
			if tc.kind == "" {
				if readErr != nil || text != tc.text {
					t.Fatalf("Read(%q) = %q, %v; want %q", tc.path, text, readErr, tc.text)
				}
				return
			}
			if readErr == nil || readErr.Kind != tc.kind {
				t.Fatalf("Read(%q) kind = %v, want %s", tc.path, readErr, tc.kind)
			}
		})
	}
	m := repository(&callLog{}, map[string]string{"bin": "\xff\xfe"})
	useDoubles(t, m, nil, nil)
	if _, readErr := (&DiskTree{Dir: "/repo"}).Read("bin"); readErr == nil || readErr.Kind != "not-text" {
		t.Fatalf("Read of binary = %v, want not-text", readErr)
	}
	faultSweep(t, func(t *testing.T, log *callLog) func() error {
		useDoubles(t, linkedRepository(log), nil, nil)
		return func() error {
			_, readErr := (&DiskTree{Dir: "/repo"}).Read("dir/b.txt")
			if readErr != nil {
				return readErr
			}
			return nil
		}
	}, "Lstat")
}

func TestDiskTreeFileModeReportsPermissionBits(t *testing.T) {
	useDoubles(t, linkedRepository(&callLog{}), nil, nil)
	if mode, err := (&DiskTree{Dir: "/repo"}).FileMode("a.txt"); err != nil || mode != 0o644 {
		t.Fatalf("FileMode(a.txt) = %o, %v; want 644", mode, err)
	}
	if _, err := (&DiskTree{Dir: "/repo"}).FileMode("in/x"); err == nil || err.Kind != "escapes" {
		t.Fatalf("FileMode through a link = %v, want escapes", err)
	}
	if _, err := (&DiskTree{Dir: "/repo"}).FileMode("nothing"); err == nil || err.Kind != "missing" {
		t.Fatalf("FileMode of a missing path = %v, want missing", err)
	}
	faultSweep(t, func(t *testing.T, log *callLog) func() error {
		useDoubles(t, linkedRepository(log), nil, nil)
		return func() error {
			_, err := (&DiskTree{Dir: "/repo"}).FileMode("a.txt")
			if err != nil {
				return err
			}
			return nil
		}
	}, "Lstat")
}

func TestDiskTreeLinksClassifiesEachLink(t *testing.T) {
	useDoubles(t, linkedRepository(&callLog{}), nil, nil)
	got := (&DiskTree{Dir: "/repo"}).Links()
	want := []application.TreeLink{
		{Path: "abs", Target: "dir", Kind: application.LinkWithin},
		{Path: "dangling", Kind: application.LinkUnresolved},
		{Path: "dirlink", Target: "dir", Kind: application.LinkWithin},
		{Path: "in", Target: "a.txt", Kind: application.LinkWithin},
		{Path: "out", Kind: application.LinkOutside},
		{Path: "rootlink", Target: "", Kind: application.LinkWithin},
	}
	if len(got) != len(want) {
		t.Fatalf("Links() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Links()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
	faultSweep(t, func(t *testing.T, log *callLog) func() error {
		useDoubles(t, linkedRepository(log), nil, nil)
		return func() error {
			(&DiskTree{Dir: "/repo"}).Links()
			return nil
		}
	}, "Lstat", "Readlink", "EvalSymlinks", "WalkDir")
}

func TestDiskTreeExistsAndLinkPassage(t *testing.T) {
	useDoubles(t, linkedRepository(&callLog{}), nil, nil)
	tree := DiskTree{Dir: "/repo"}
	for path, want := range map[string]bool{
		"a.txt": true, "dir": true, "nothing": false, "../x": false, "in": false, "dangling": false,
	} {
		if got := tree.Exists(path); got != want {
			t.Errorf("Exists(%q) = %v, want %v", path, got, want)
		}
	}
	for path, want := range map[string]bool{
		"in/x": true, "in": true, "a.txt": false, "missing/x": false, "": false, ".": false,
	} {
		if got := tree.PassesThroughLink(path); got != want {
			t.Errorf("PassesThroughLink(%q) = %v, want %v", path, got, want)
		}
	}
	faultSweep(t, func(t *testing.T, log *callLog) func() error {
		useDoubles(t, linkedRepository(log), nil, nil)
		return func() error {
			if !(&DiskTree{Dir: "/repo"}).Exists("a.txt") {
				return errInjected
			}
			return nil
		}
	}, "Lstat")
}

// gitRepository is the fixture a Git query runs in: the tree plus a commit message file.
func gitRepository(log *callLog) *memFS {
	m := linkedRepository(log)
	m.nodes["/repo/.git/COMMIT_EDITMSG"] = &memNode{perm: 0o644, data: []byte("subject\n")}
	return m
}

func TestDiskTreeGitQueriesRefuseEachFailure(t *testing.T) {
	queries := []struct {
		name string
		good []byte
		run  func(tree DiskTree) error
	}{
		{"IndexedFiles", []byte("b.txt\x00a/x.md\x00b.txt\x00"), func(tree DiskTree) error {
			paths, err := tree.IndexedFiles()
			if err == nil && !equalStrings(paths, []string{"a/x.md", "b.txt"}) {
				return errInjected
			}
			return err
		}},
		{"TrackedFiles", []byte("a.txt\x00"), func(tree DiskTree) error {
			_, err := tree.TrackedFiles()
			return err
		}},
		{"IgnoredFiles", []byte("build/out\x00"), func(tree DiskTree) error {
			_, err := tree.IgnoredFiles()
			return err
		}},
		{"IndexedRegularFiles", rawRecord("100644", "a.txt"), func(tree DiskTree) error {
			_, err := tree.IndexedRegularFiles()
			return err
		}},
		{"ChangedFiles", []byte("x.md\x00"), func(tree DiskTree) error {
			_, err := tree.ChangedFiles("base", "head")
			return err
		}},
		{"ChangedRegularFiles", rawRecord("100755", "run.sh"), func(tree DiskTree) error {
			_, err := tree.ChangedRegularFiles("base", "head")
			return err
		}},
		{"ResolveGitRef", []byte("0123456789abcdef0123456789abcdef01234567\n"), func(tree DiskTree) error {
			_, err := tree.ResolveGitRef("main")
			return err
		}},
		{"CommitMessages", []byte("subject\n\nbody\n"), func(tree DiskTree) error {
			_, err := tree.CommitMessages("base", "head")
			return err
		}},
	}
	for _, query := range queries {
		t.Run(query.name, func(t *testing.T) {
			faultSweep(t, func(t *testing.T, log *callLog) func() error {
				commands := &fakeCommands{log: log, respond: answering(query.good, nil)}
				useDoubles(t, gitRepository(log), commands, nil)
				return func() error { return query.run(DiskTree{Dir: "/repo"}) }
			})
		})
	}
}

// rawRecord is one raw Git change record for a regular file with the given destination mode.
func rawRecord(mode, name string) []byte {
	return []byte(":000000 " + mode + " 0000000000000000000000000000000000000000 abcdef0 A\x00" + name + "\x00")
}

func TestDiskTreeGitQueriesClassifyFailuresAndBadOutput(t *testing.T) {
	type call func(tree DiskTree) error
	cases := []struct {
		name    string
		output  []byte
		err     error
		run     call
		wantErr bool
	}{
		{"index refused", nil, execFailure(), func(tree DiskTree) error { _, err := tree.IndexedFiles(); return err }, true},
		{"index not UTF-8", []byte("\xff"), nil, func(tree DiskTree) error { _, err := tree.IndexedFiles(); return err }, true},
		{"index escapes", []byte("../x\x00"), nil, func(tree DiskTree) error { _, err := tree.IndexedFiles(); return err }, true},
		{"tracked refused", nil, execFailure(), func(tree DiskTree) error { _, err := tree.TrackedFiles(); return err }, true},
		{"tracked unreadable", nil, errInjected, func(tree DiskTree) error { _, err := tree.TrackedFiles(); return err }, true},
		{"tracked not UTF-8", []byte("\xff"), nil, func(tree DiskTree) error { _, err := tree.TrackedFiles(); return err }, true},
		{"ignored refused", nil, execFailure(), func(tree DiskTree) error { _, err := tree.IgnoredFiles(); return err }, true},
		{"ignored escapes", []byte("/abs\x00"), nil, func(tree DiskTree) error { _, err := tree.IgnoredFiles(); return err }, true},
		{"raw refused", nil, execFailure(), func(tree DiskTree) error { _, err := tree.IndexedRegularFiles(); return err }, true},
		{"raw unreadable", nil, errInjected, func(tree DiskTree) error { _, err := tree.IndexedRegularFiles(); return err }, true},
		{"raw symlink skipped", rawRecord("120000", "link"), nil, func(tree DiskTree) error {
			paths, err := tree.IndexedRegularFiles()
			if err == nil && len(paths) != 0 {
				return errInjected
			}
			return err
		}, false},
		{"changed refused", nil, execFailure(), func(tree DiskTree) error { _, err := tree.ChangedFiles("a", "b"); return err }, true},
		{"changed unreadable", nil, errInjected, func(tree DiskTree) error { _, err := tree.ChangedFiles("a", "b"); return err }, true},
		{"changed not UTF-8", []byte("\xff"), nil, func(tree DiskTree) error { _, err := tree.ChangedFiles("a", "b"); return err }, true},
		{"changed escapes", []byte("../x\x00"), nil, func(tree DiskTree) error { _, err := tree.ChangedFiles("a", "b"); return err }, true},
		{"changed regular refused", nil, execFailure(), func(tree DiskTree) error { _, err := tree.ChangedRegularFiles("a", "b"); return err }, true},
		{"changed regular unreadable", nil, errInjected, func(tree DiskTree) error { _, err := tree.ChangedRegularFiles("a", "b"); return err }, true},
		{"ref refused", nil, execFailure(), func(tree DiskTree) error { _, err := tree.ResolveGitRef("main"); return err }, true},
		{"ref unreadable", nil, errInjected, func(tree DiskTree) error { _, err := tree.ResolveGitRef("main"); return err }, true},
		{"ref not UTF-8", []byte("\xff"), nil, func(tree DiskTree) error { _, err := tree.ResolveGitRef("main"); return err }, true},
		{"ref not hex", []byte("zz\n"), nil, func(tree DiskTree) error { _, err := tree.ResolveGitRef("main"); return err }, true},
		{"ref empty", nil, nil, func(tree DiskTree) error { _, err := tree.ResolveGitRef(""); return err }, true},
		{"ref option", nil, nil, func(tree DiskTree) error { _, err := tree.ResolveGitRef("-x"); return err }, true},
		{"ref spaced", nil, nil, func(tree DiskTree) error { _, err := tree.ResolveGitRef("a b"); return err }, true},
		{"ref tab", nil, nil, func(tree DiskTree) error { _, err := tree.ResolveGitRef("a\tb"); return err }, true},
		{"ref newline", nil, nil, func(tree DiskTree) error { _, err := tree.ResolveGitRef("a\nb"); return err }, true},
		{"ref carriage", nil, nil, func(tree DiskTree) error { _, err := tree.ResolveGitRef("a\rb"); return err }, true},
		{"ref vertical", nil, nil, func(tree DiskTree) error { _, err := tree.ResolveGitRef("a\vb"); return err }, true},
		{"ref form feed", nil, nil, func(tree DiskTree) error { _, err := tree.ResolveGitRef("a\fb"); return err }, true},
		{"messages refused", nil, execFailure(), func(tree DiskTree) error { _, err := tree.CommitMessages("a", "b"); return err }, true},
		{"messages unreadable", nil, errInjected, func(tree DiskTree) error { _, err := tree.CommitMessages("a", "b"); return err }, true},
		{"messages not UTF-8", []byte("\xff"), nil, func(tree DiskTree) error { _, err := tree.CommitMessages("a", "b"); return err }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			commands := &fakeCommands{log: &callLog{}, respond: answering(tc.output, tc.err)}
			useDoubles(t, gitRepository(&callLog{}), commands, nil)
			err := tc.run(DiskTree{Dir: "/repo"})
			if tc.wantErr != (err != nil) {
				t.Fatalf("%s: error = %v, want error %v", tc.name, err, tc.wantErr)
			}
		})
	}
}

func TestDiskTreeHookMessageFileReadsOnlyGitsCanonicalFile(t *testing.T) {
	const canonical = "/repo/.git/COMMIT_EDITMSG\n"
	rev := func(out []byte, err error) *fakeCommands {
		return &fakeCommands{log: &callLog{}, respond: answering(out, err)}
	}
	cases := []struct {
		name     string
		commands *fakeCommands
		file     string
		text     string
		kind     string
	}{
		{name: "canonical absolute", commands: rev([]byte(canonical), nil), file: "/repo/.git/COMMIT_EDITMSG", text: "subject\n"},
		{name: "canonical relative", commands: rev([]byte(canonical), nil), file: ".git/COMMIT_EDITMSG", text: "subject\n"},
		{name: "refused query", commands: rev(nil, execFailure()), file: ".git/COMMIT_EDITMSG", kind: "unreadable"},
		{name: "unreadable query", commands: rev(nil, errInjected), file: ".git/COMMIT_EDITMSG", kind: "unreadable"},
		{name: "not UTF-8 path", commands: rev([]byte("\xff"), nil), file: ".git/COMMIT_EDITMSG", kind: "unreadable"},
		{name: "relative git path", commands: rev([]byte("COMMIT_EDITMSG\n"), nil), file: ".git/COMMIT_EDITMSG", kind: "unreadable"},
		{name: "other path", commands: rev([]byte(canonical), nil), file: ".git/other", kind: "unreadable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useDoubles(t, gitRepository(&callLog{}), tc.commands, nil)
			text, err := (&DiskTree{Dir: "/repo"}).HookMessageFile(tc.file)
			if tc.kind == "" {
				if err != nil || text != tc.text {
					t.Fatalf("HookMessageFile = %q, %v; want %q", text, err, tc.text)
				}
				return
			}
			if err == nil || err.Kind != tc.kind {
				t.Fatalf("HookMessageFile kind = %v, want %s", err, tc.kind)
			}
		})
	}
	faultSweep(t, func(t *testing.T, log *callLog) func() error {
		useDoubles(t, gitRepository(log), &fakeCommands{log: log, respond: answering([]byte(canonical), nil)}, nil)
		return func() error {
			_, err := (&DiskTree{Dir: "/repo"}).HookMessageFile(".git/COMMIT_EDITMSG")
			if err != nil {
				return err
			}
			return nil
		}
	})
}

// envTarget is one declared environment target for the store tests.
func envTarget(name, contents string) application.EnvironmentFile {
	return application.EnvironmentFile{Path: name, Contents: contents}
}

func TestDiskEnvironmentStoreCreateInstallsNewTargetsOnly(t *testing.T) {
	m := repository(&callLog{}, map[string]string{"keep": "old"})
	useDoubles(t, m, nil, nil)
	if err := (DiskEnvironmentStore{}).Create("/repo", []application.EnvironmentFile{envTarget("config/.env", "X=1")}); err != nil {
		t.Fatalf("Create = %v", err)
	}
	if got := string(m.get("/repo/config/.env").data); got != "X=1" {
		t.Fatalf("installed contents = %q", got)
	}
	cases := map[string]struct {
		root  string
		files []application.EnvironmentFile
	}{
		"duplicate":        {"/repo", []application.EnvironmentFile{envTarget("a", "1"), envTarget("a", "2")}},
		"escaping target":  {"/repo", []application.EnvironmentFile{envTarget("../x", "1")}},
		"missing root":     {"/repo/absent", []application.EnvironmentFile{envTarget("a", "1")}},
		"existing target":  {"/repo", []application.EnvironmentFile{envTarget("keep", "new")}},
		"linked directory": {"/repo", []application.EnvironmentFile{envTarget("linkdir/x", "1")}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fixture := repository(&callLog{}, map[string]string{"keep": "old", "dir/keep2": "k"})
			symlink(fixture, "linkdir", "dir")
			useDoubles(t, fixture, nil, nil)
			if err := (DiskEnvironmentStore{}).Create(tc.root, tc.files); err == nil {
				t.Fatalf("Create(%s) succeeded, want a refusal", name)
			}
		})
	}
	faultSweep(t, func(t *testing.T, log *callLog) func() error {
		useDoubles(t, repository(log, map[string]string{"keep": "old"}), nil, nil)
		return func() error {
			return (DiskEnvironmentStore{}).Create("/repo", []application.EnvironmentFile{envTarget("config/.env", "X=1")})
		}
	}, "Lstat")
}

func TestDiskEnvironmentStoreRestoreReplacesOnlyWhenAuthorized(t *testing.T) {
	restore := func(replace bool) []application.EnvironmentRestoreFile {
		return []application.EnvironmentRestoreFile{{Path: "keep", Contents: "new", Replace: replace}, {Path: "fresh/x", Contents: "F"}}
	}
	m := repository(&callLog{}, map[string]string{"keep": "old"})
	useDoubles(t, m, nil, nil)
	if err := (DiskEnvironmentStore{}).Restore("/repo", restore(true)); err != nil {
		t.Fatalf("Restore with replace = %v", err)
	}
	if got := string(m.get("/repo/keep").data); got != "new" {
		t.Fatalf("replaced contents = %q, want new", got)
	}
	refused := repository(&callLog{}, map[string]string{"keep": "old"})
	useDoubles(t, refused, nil, nil)
	if err := (DiskEnvironmentStore{}).Restore("/repo", restore(false)); err == nil {
		t.Fatal("Restore without replace succeeded over an existing target")
	}
	if got := string(refused.get("/repo/keep").data); got != "old" {
		t.Fatalf("refused restore changed the target to %q", got)
	}
	bad := map[string]func(m *memFS){
		"a directory target":  func(m *memFS) { m.nodes["/repo/keep"] = &memNode{dir: true, perm: 0o755} },
		"a linked target":     func(m *memFS) { symlink(m, "keep", "elsewhere") },
		"a duplicate":         nil,
		"an escaping restore": nil,
	}
	for name, shape := range bad {
		t.Run(name, func(t *testing.T) {
			fixture := repository(&callLog{}, map[string]string{"keep": "old"})
			if shape != nil {
				shape(fixture)
			}
			useDoubles(t, fixture, nil, nil)
			files := restore(true)
			switch name {
			case "a duplicate":
				files = append(files, application.EnvironmentRestoreFile{Path: "keep", Contents: "x", Replace: true})
			case "an escaping restore":
				files = []application.EnvironmentRestoreFile{{Path: "../x", Contents: "x"}}
			}
			if err := (DiskEnvironmentStore{}).Restore("/repo", files); err == nil {
				t.Fatalf("Restore(%s) succeeded, want a refusal", name)
			}
		})
	}
	faultSweep(t, func(t *testing.T, log *callLog) func() error {
		useDoubles(t, repository(log, map[string]string{"keep": "old"}), nil, nil)
		return func() error { return (DiskEnvironmentStore{}).Restore("/repo", restore(true)) }
	}, "Lstat")
}

// equalStrings reports whether two string slices hold the same values in order.
func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
