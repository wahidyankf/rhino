package runtime

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// The rollback of an adapter replacement reports every step it cannot undo: a
// removal of an installed family or file, and a restore from a backup that
// fails. Each step fails here, and the report keeps all of them.
func TestAdapterRollbackReportsEveryStepItCannotUndo(t *testing.T) {
	log := &callLog{failOps: map[string]bool{"RemoveAll": true, "Remove": true, "Rename": true}}
	useDoubles(t, repository(log, map[string]string{"fam/x": "x", "fam.bak": "b", "file": "f", "file.bak": "b"}), nil, nil)
	moved := []movedAdapter{
		{target: "/repo/fam", backup: "/repo/fam.bak", installed: true, kind: managedFamily},
		{target: "/repo/file", backup: "/repo/file.bak", installed: true, kind: managedFile},
	}
	if err := restoreAdapterEntries(moved); err == nil || !errors.Is(err, errInjected) {
		t.Fatalf("restoreAdapterEntries = %v, want every failed step reported", err)
	}
}

// A path under a link, a link itself, and a missing path answer from the link
// alone, and a file that holds no text is refused as not text.
func TestTreePredicatesAnswerFromTheLinkItself(t *testing.T) {
	m := repository(&callLog{}, map[string]string{"sub/x": "x", "bin.dat": "\xff"})
	m.nodes["/repo/link"] = &memNode{link: "sub"}
	useDoubles(t, m, nil, nil)
	tree := &DiskTree{Dir: "/repo"}
	if !tree.PassesThroughLink("link/x") {
		t.Fatal("PassesThroughLink missed a link among the parents")
	}
	if tree.Exists("missing") {
		t.Fatal("Exists reported a missing path")
	}
	if !tree.Exists("sub") {
		t.Fatal("Exists missed a directory")
	}
	if _, fault := tree.Read("bin.dat"); fault == nil || fault.Kind != "not-text" {
		t.Fatalf("Read of a file with no text = %+v, want a not-text refusal", fault)
	}
}

// Git's repository paths never hold a current or parent segment.
func TestGitRepositoryPathRefusesDotSegments(t *testing.T) {
	for _, path := range []string{"a/../b", "./a", "a/./b"} {
		if gitRepositoryPath(path) {
			t.Fatalf("gitRepositoryPath(%q) accepted a dot segment", path)
		}
	}
}

// A linked directory is skipped rather than entered, and the git metadata
// directory is skipped too. Both are answered by the walk callback's result.
func TestTreeListingsSkipLinkedDirectoriesAndGitMetadata(t *testing.T) {
	linkedDir := memDirEntry{info: memInfo{name: "linkdir", node: &memNode{dir: true, link: "elsewhere", perm: 0o755}}}
	double := walkingFS{memFS: repository(&callLog{}, nil), walk: func(visit fs.WalkDirFunc) error {
		if err := visit("/repo/linkdir", linkedDir, nil); err != fs.SkipDir {
			t.Errorf("a linked directory returned %v, want SkipDir", err)
		}
		if err := visit("/repo/.git", listedEntry(".git", true), nil); err != fs.SkipDir {
			t.Errorf("the git metadata directory returned %v, want SkipDir", err)
		}
		return nil
	}}
	useDoubles(t, double, nil, nil)
	if got := (&DiskTree{Dir: "/repo"}).Files(); len(got) != 0 {
		t.Fatalf("Files = %v, want nothing from a skipped directory", got)
	}
}

// A link that the walk reports under a relative path cannot be placed under an
// absolute root, so the listing fails loudly instead of dropping it.
func TestLinkListingFailsLoudlyOnAPathItCannotPlace(t *testing.T) {
	double := walkingFS{memFS: repository(&callLog{}, map[string]string{"a": "x"}), walk: func(visit fs.WalkDirFunc) error {
		return visit("elsewhere/link", memDirEntry{info: memInfo{name: "link", node: &memNode{link: "x", perm: 0o777}}}, nil)
	}}
	useDoubles(t, double, nil, nil)
	expectPanic(t, "listing the links broke", func() { (&DiskTree{Dir: "/repo"}).Links() })
}

// A root with a colon cannot be placed on a PATH list, so the mutator is refused
// before it runs.
func TestMutatorPathRefusesARootThatCannotBeRepresented(t *testing.T) {
	m := mutationRepository(&callLog{})
	_ = m.mkdirAll("/re:po/node_modules/.bin", 0o755)
	respond := gitResponder(m, defaultScript, noMutation)
	useDoubles(t, m, &fakeCommands{log: &callLog{}, respond: respond}, fakeEnv{"PATH": "/usr/bin"})
	_, err := runMutator("/re:po", application.MutationLaunch{Argv: []string{"mutator"}, Directory: "/re:po"})
	if err == nil || !strings.Contains(err.Error(), "cannot be represented safely") {
		t.Fatalf("runMutator with a colon in its root = %v, want a refusal", err)
	}
}

// An item without a login gets one when its password is set, and the edit
// succeeds.
func TestBitwardenUpdatesAnItemWithoutALogin(t *testing.T) {
	m := repository(&callLog{}, nil)
	answers := bwOverride(m, "unlocked", "get item", []byte(`{"id":"1","name":"app","type":2}`), nil)
	useDoubles(t, m, &fakeCommands{log: &callLog{}, respond: answers}, fakeEnv{"BW_SESSION": "s", "PATH": "/usr/bin"})
	if fault := (BitwardenVault{}).UpdateLogin(application.VaultItem{ID: "1"}, "new"); fault != nil {
		t.Fatalf("UpdateLogin of an item without a login = %+v, want success", fault)
	}
}
