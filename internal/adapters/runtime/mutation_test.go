package runtime_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/runtime"
	"github.com/wahidyankf/rhino/internal/application"
)

// writeFormatter is a declared mutator that rewrites every path it is given.
// It runs from the snapshot, so its relative paths are repository paths.
var writeFormatter = []string{"/bin/sh", "-c", `for f in "$@"; do printf 'formatted\n' > "$f"; done`, "fmt"}

func mutationLaunch(argv []string, dir string, selected ...string) application.MutationLaunch {
	full := append(append([]string(nil), argv...), selected...)
	return application.MutationLaunch{
		Argv:          full,
		Directory:     dir,
		SelectedPaths: selected,
	}
}

// gitOutput runs one Git command in dir and returns its trimmed output.
func gitOutput(dir string, args ...string) (string, error) {
	output, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

// indexBlob reads one staged blob from the Git index.
func indexBlob(t *testing.T, dir, path string) string {
	t.Helper()
	output, err := gitOutput(dir, "show", ":"+path)
	if err != nil {
		t.Fatalf("cannot read index blob %s: %v: %s", path, err, output)
	}
	return output
}

// TestDiskMutationApplyIndexFormatsOnlyTheSelectedBlob proves a pre-commit
// mutation rewrites the staged blob and the matching working-tree file, and
// leaves an unselected file alone.
func TestDiskMutationApplyIndexFormatsOnlyTheSelectedBlob(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "a.md", "draft\n")
	write(t, dir, "b.md", "keep\n")
	run("add", "a.md", "b.md")
	run("commit", "-q", "-m", "base")
	write(t, dir, "a.md", "draft two\n")
	write(t, dir, "b.md", "keep edit\n")
	run("add", "a.md")

	outcome, err := (runtime.DiskMutationRunner{}).ApplyIndex(mutationLaunch(writeFormatter, dir, "a.md"))
	if err != nil || outcome.Code != 0 {
		t.Fatalf("apply-index failed: %+v %v", outcome, err)
	}
	if strings.Join(outcome.Changes, ",") != "a.md" || len(outcome.Divergences) != 0 {
		t.Fatalf("changes or divergences wrong: %+v", outcome)
	}
	if got := indexBlob(t, dir, "a.md"); got != "formatted" {
		t.Fatalf("staged blob not formatted: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "a.md")); string(got) != "formatted\n" {
		t.Fatalf("working-tree copy not formatted: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "b.md")); string(got) != "keep edit\n" {
		t.Fatalf("unselected file changed: %q", got)
	}
}

// TestDiskMutationApplyIndexPreservesUnstagedEdits proves a path whose working
// tree holds edits the index does not have is staged formatted but its
// working-tree copy is preserved and reported as a divergence.
func TestDiskMutationApplyIndexPreservesUnstagedEdits(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "a.md", "draft\n")
	run("add", "a.md")
	run("commit", "-q", "-m", "base")
	write(t, dir, "a.md", "staged\n")
	run("add", "a.md")
	write(t, dir, "a.md", "unstaged edit\n")

	outcome, err := (runtime.DiskMutationRunner{}).ApplyIndex(mutationLaunch(writeFormatter, dir, "a.md"))
	if err != nil || outcome.Code != 0 {
		t.Fatalf("apply-index failed: %+v %v", outcome, err)
	}
	if strings.Join(outcome.Divergences, ",") != "a.md" {
		t.Fatalf("unstaged edit not reported: %+v", outcome)
	}
	if got := indexBlob(t, dir, "a.md"); got != "formatted" {
		t.Fatalf("index not formatted: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "a.md")); string(got) != "unstaged edit\n" {
		t.Fatalf("unstaged edit overwritten: %q", got)
	}
}

// TestDiskMutationApplyIndexKeepsTheExecutableBit proves a formatted file keeps
// the mode the index records, so a script is not silently made non-executable.
func TestDiskMutationApplyIndexKeepsTheExecutableBit(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "run.sh", "old\n")
	if err := os.Chmod(filepath.Join(dir, "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	run("add", "run.sh")
	run("commit", "-q", "-m", "base")
	write(t, dir, "run.sh", "old two\n")
	if err := os.Chmod(filepath.Join(dir, "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	run("add", "run.sh")

	outcome, err := (runtime.DiskMutationRunner{}).ApplyIndex(mutationLaunch(writeFormatter, dir, "run.sh"))
	if err != nil || outcome.Code != 0 {
		t.Fatalf("apply-index failed: %+v %v", outcome, err)
	}
	info, err := os.Stat(filepath.Join(dir, "run.sh"))
	if err != nil || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("executable bit lost: %v %v", info, err)
	}
	if mode := run("ls-files", "--stage", "--", "run.sh"); !strings.HasPrefix(mode, "100755 ") {
		t.Fatalf("index mode changed: %q", mode)
	}
}

// TestDiskMutationApplyIndexHandlesAwkwardNames proves a selected name holding a
// comma is recorded exactly, not split as an update-index argument.
func TestDiskMutationApplyIndexHandlesAwkwardNames(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "x,y.md", "draft\n")
	run("add", "x,y.md")
	run("commit", "-q", "-m", "base")
	write(t, dir, "x,y.md", "draft two\n")
	run("add", "x,y.md")

	outcome, err := (runtime.DiskMutationRunner{}).ApplyIndex(mutationLaunch(writeFormatter, dir, "x,y.md"))
	if err != nil || outcome.Code != 0 {
		t.Fatalf("apply-index failed: %+v %v", outcome, err)
	}
	if got := indexBlob(t, dir, "x,y.md"); got != "formatted" {
		t.Fatalf("comma name not updated: %q", got)
	}
}

// TestDiskMutationApplyIndexRemovesADeletedPath proves a selected path the
// mutator deletes leaves the index and the working tree.
func TestDiskMutationApplyIndexRemovesADeletedPath(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "gone.md", "draft\n")
	run("add", "gone.md")
	run("commit", "-q", "-m", "base")
	remove := []string{"/bin/sh", "-c", `rm -f "$1"`, "rm"}

	outcome, err := (runtime.DiskMutationRunner{}).ApplyIndex(mutationLaunch(remove, dir, "gone.md"))
	if err != nil || outcome.Code != 0 || strings.Join(outcome.Changes, ",") != "gone.md" {
		t.Fatalf("deletion not reported: %+v %v", outcome, err)
	}
	if listed := run("ls-files", "--", "gone.md"); listed != "" {
		t.Fatalf("index still lists the deleted path: %q", listed)
	}
	if _, err := os.Stat(filepath.Join(dir, "gone.md")); !os.IsNotExist(err) {
		t.Fatalf("working-tree file not removed: %v", err)
	}
}

// TestDiskMutationApplyIndexRefusesASelectedSymlinkBeforeTheMutator proves a
// selected path that is a tracked link is refused before any child starts.
func TestDiskMutationApplyIndexRefusesASelectedSymlinkBeforeTheMutator(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "target.md", "target\n")
	if err := os.Symlink("target.md", filepath.Join(dir, "link.md")); err != nil {
		t.Skip("symlinks unavailable")
	}
	run("add", "target.md", "link.md")
	run("commit", "-q", "-m", "base")
	marker := filepath.Join(dir, "ran")
	touch := []string{"/bin/sh", "-c", `touch "$1"`, "touch", marker}

	_, err := (runtime.DiskMutationRunner{}).ApplyIndex(mutationLaunch(touch, dir, "link.md"))
	if err == nil || !strings.Contains(err.Error(), "selected index path is a symbolic link `link.md`") {
		t.Fatalf("selected symlink not refused: %v", err)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("mutator started before the refusal: %v", statErr)
	}
}

// TestDiskMutationApplyIndexRefusesEscapingSelections proves a selected path
// that leaves the repository is refused before anything is snapshotted.
func TestDiskMutationApplyIndexRefusesEscapingSelections(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "a.md", "a\n")
	run("add", "a.md")
	run("commit", "-q", "-m", "base")
	for _, selected := range []string{"../escape.md", "/absolute.md", "dir/../a.md", "a//b"} {
		_, err := (runtime.DiskMutationRunner{}).ApplyIndex(application.MutationLaunch{
			Argv: writeFormatter, Directory: dir, SelectedPaths: []string{selected},
		})
		if err == nil || !strings.Contains(err.Error(), "selected index path escapes its root `"+selected+"`") {
			t.Fatalf("%s: escaping selection not refused: %v", selected, err)
		}
	}
}

// TestDiskMutationApplyIndexRefusesAnUnselectedChange proves a mutator that
// changes a file outside the selection is refused, and nothing is written.
func TestDiskMutationApplyIndexRefusesAnUnselectedChange(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "a.md", "draft\n")
	write(t, dir, "b.md", "other\n")
	run("add", "a.md", "b.md")
	run("commit", "-q", "-m", "base")
	write(t, dir, "a.md", "draft two\n")
	run("add", "a.md")
	touchB := []string{"/bin/sh", "-c", `printf 'changed\n' > b.md; printf 'formatted\n' > a.md`, "sh"}

	_, err := (runtime.DiskMutationRunner{}).ApplyIndex(mutationLaunch(touchB, dir, "a.md"))
	if err == nil || !strings.Contains(err.Error(), "mutator changed `b.md` outside its selected index paths") {
		t.Fatalf("unselected change not refused: %v", err)
	}
	if got := indexBlob(t, dir, "a.md"); got != "draft two" {
		t.Fatalf("index written after refusal: %q", got)
	}
}

// TestDiskMutationApplyIndexReportsTheMutatorStatus proves a mutator that exits
// nonzero is reported by its status and writes nothing.
func TestDiskMutationApplyIndexReportsTheMutatorStatus(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "a.md", "draft\n")
	run("add", "a.md")
	run("commit", "-q", "-m", "base")
	write(t, dir, "a.md", "draft two\n")
	run("add", "a.md")
	failing := []string{"/bin/sh", "-c", `exit 3`, "sh"}

	outcome, err := (runtime.DiskMutationRunner{}).ApplyIndex(mutationLaunch(failing, dir, "a.md"))
	if err != nil || outcome.Code != 3 || len(outcome.Changes) != 0 {
		t.Fatalf("status not reported: %+v %v", outcome, err)
	}
	if got := indexBlob(t, dir, "a.md"); got != "draft two" {
		t.Fatalf("index written by a failing mutator: %q", got)
	}
}

// TestDiskMutationApplyIndexRunsNothingForAnEmptySelection proves an empty
// selection reports clean without starting the mutator.
func TestDiskMutationApplyIndexRunsNothingForAnEmptySelection(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "a.md", "a\n")
	run("add", "a.md")
	run("commit", "-q", "-m", "base")
	marker := filepath.Join(dir, "ran")
	touch := []string{"/bin/sh", "-c", `touch "$1"`, "touch", marker}

	outcome, err := (runtime.DiskMutationRunner{}).ApplyIndex(mutationLaunch(touch, dir))
	if err != nil || outcome.Code != 0 || len(outcome.Changes) != 0 {
		t.Fatalf("empty selection not clean: %+v %v", outcome, err)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("mutator ran for an empty selection: %v", statErr)
	}
}

// TestDiskMutationApplyIndexRefusesOutsideGit proves the local boundary refuses
// a repository without Git, in the oracle's words.
func TestDiskMutationApplyIndexRefusesOutsideGit(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.md", "a\n")
	_, err := (runtime.DiskMutationRunner{}).ApplyIndex(mutationLaunch(writeFormatter, dir, "a.md"))
	if err == nil || !strings.Contains(err.Error(), "this entry point has no index mutation boundary") {
		t.Fatalf("outside git not refused: %v", err)
	}
}

// TestDiskMutationVerifyCleanReportsTheReplayAgainstItsRevision proves the
// pull-request replay runs in a disposable checkout of the candidate revision
// and reports what that checkout changed.
func TestDiskMutationVerifyCleanReportsTheReplayAgainstItsRevision(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "a.md", "draft\n")
	run("add", "a.md")
	run("commit", "-q", "-m", "base")
	revision := run("rev-parse", "HEAD")

	outcome, err := (runtime.DiskMutationRunner{}).VerifyClean(application.MutationLaunch{
		Argv: mutationLaunch(writeFormatter, dir, "a.md").Argv, Directory: dir,
		SelectedPaths: []string{"a.md"}, Revision: revision,
	})
	if err != nil || outcome.Code != 1 || strings.Join(outcome.Changes, ",") != "a.md" {
		t.Fatalf("replay did not report the change: %+v %v", outcome, err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "a.md")); string(got) != "draft\n" {
		t.Fatalf("replay wrote to the ordinary checkout: %q", got)
	}
	if worktrees := run("worktree", "list", "--porcelain"); strings.Count(worktrees, "worktree ") != 1 {
		t.Fatalf("disposable checkout was not removed: %q", worktrees)
	}
}

// TestDiskMutationVerifyCleanPassesACleanReplay proves a mutator that changes
// nothing in the candidate revision passes, even when the ordinary working tree
// is dirty: the replay reads the revision, never the working tree.
func TestDiskMutationVerifyCleanPassesACleanReplay(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "a.md", "clean\n")
	run("add", "a.md")
	run("commit", "-q", "-m", "base")
	revision := run("rev-parse", "HEAD")
	write(t, dir, "a.md", "dirty working tree\n")
	noop := []string{"/bin/sh", "-c", "true", "true"}

	outcome, err := (runtime.DiskMutationRunner{}).VerifyClean(application.MutationLaunch{
		Argv: noop, Directory: dir, Revision: revision,
	})
	if err != nil || outcome.Code != 0 || len(outcome.Changes) != 0 {
		t.Fatalf("clean replay failed: %+v %v", outcome, err)
	}
}

// TestDiskMutationVerifyCleanCountsNewFiles proves a path the mutator creates
// in the replay is a change, not invisible because it is untracked.
func TestDiskMutationVerifyCleanCountsNewFiles(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "a.md", "a\n")
	run("add", "a.md")
	run("commit", "-q", "-m", "base")
	revision := run("rev-parse", "HEAD")
	create := []string{"/bin/sh", "-c", `printf 'new\n' > created.md`, "sh"}

	outcome, err := (runtime.DiskMutationRunner{}).VerifyClean(application.MutationLaunch{
		Argv: create, Directory: dir, Revision: revision,
	})
	if err != nil || outcome.Code != 1 || strings.Join(outcome.Changes, ",") != "created.md" {
		t.Fatalf("new file not counted: %+v %v", outcome, err)
	}
}

// TestDiskMutationVerifyCleanRequiresARevision proves a replay without a
// candidate commit refuses before it creates anything.
func TestDiskMutationVerifyCleanRequiresARevision(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "a.md", "a\n")
	run("add", "a.md")
	run("commit", "-q", "-m", "base")
	_, err := (runtime.DiskMutationRunner{}).VerifyClean(application.MutationLaunch{Argv: writeFormatter, Directory: dir})
	if err == nil || !strings.Contains(err.Error(), "requires an explicit candidate revision") {
		t.Fatalf("missing revision not refused: %v", err)
	}
}

// TestDiskMutationRelativePathSegmentsResolveFromTheRepository proves a tool
// that lives in the original repository's node_modules, or under a relative
// PATH segment, is found by the mutator from its disposable directory.
func TestDiskMutationRelativePathSegmentsResolveFromTheRepository(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "a.md", "a\n")
	run("add", "a.md")
	run("commit", "-q", "-m", "base")
	revision := run("rev-parse", "HEAD")
	tool := "#!/bin/sh\nexit 0\n"
	write(t, dir, "node_modules/.bin/fakefmt", tool)
	write(t, dir, "tools/fakefmt", tool)
	for _, path := range []string{"node_modules/.bin/fakefmt", "tools/fakefmt"} {
		if err := os.Chmod(filepath.Join(dir, path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	probe := []string{"/bin/sh", "-c", "command -v fakefmt >/dev/null", "probe"}

	local, err := (runtime.DiskMutationRunner{}).ApplyIndex(application.MutationLaunch{
		Argv: probe, Directory: dir, SelectedPaths: []string{"a.md"},
	})
	if err != nil || local.Code != 0 {
		t.Fatalf("node_modules tool not found on apply-index: %+v %v", local, err)
	}
	replay, err := (runtime.DiskMutationRunner{}).VerifyClean(application.MutationLaunch{
		Argv: probe, Directory: dir, Revision: revision,
	})
	if err != nil || replay.Code != 0 {
		t.Fatalf("node_modules tool not found on verify-clean: %+v %v", replay, err)
	}
}

// TestDiskMutationRelativePathSegmentResolvesFromTheRepository proves a relative
// PATH segment is read from the repository that declared the gate, not from the
// disposable directory the mutator runs in. The repository has no node_modules,
// so the only way the tool is found is through the relative segment.
func TestDiskMutationRelativePathSegmentResolvesFromTheRepository(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "a.md", "a\n")
	run("add", "a.md")
	run("commit", "-q", "-m", "base")
	write(t, dir, "tools/fakefmt", "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filepath.Join(dir, "tools/fakefmt"), 0o755); err != nil {
		t.Fatal(err)
	}
	probe := []string{"/bin/sh", "-c", "command -v fakefmt >/dev/null", "probe"}
	segment := application.MutationLaunch{
		Argv: probe, Directory: dir, SelectedPaths: []string{"a.md"},
		Environment: map[string]string{"PATH": "tools:/bin:/usr/bin"},
	}
	found, err := (runtime.DiskMutationRunner{}).ApplyIndex(segment)
	if err != nil || found.Code != 0 {
		t.Fatalf("relative PATH segment not resolved from the repository: %+v %v", found, err)
	}
}

// TestDiskMutationWithoutAToolReportsItsStatus is the control for the PATH
// test: with no tool anywhere, the probe fails and the status says so.
func TestDiskMutationWithoutAToolReportsItsStatus(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "a.md", "a\n")
	run("add", "a.md")
	run("commit", "-q", "-m", "base")
	probe := []string{"/bin/sh", "-c", "command -v definitely-not-a-tool-rhino >/dev/null", "probe"}

	outcome, err := (runtime.DiskMutationRunner{}).ApplyIndex(application.MutationLaunch{
		Argv: probe, Directory: dir, SelectedPaths: []string{"a.md"},
	})
	if err != nil || outcome.Code == 0 {
		t.Fatalf("missing tool reported clean: %+v %v", outcome, err)
	}
}

// TestDiskMutationProjectsItsEnvironment proves the declared environment is
// visible to the mutator, as it is to a check child.
func TestDiskMutationProjectsItsEnvironment(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "a.md", "a\n")
	run("add", "a.md")
	run("commit", "-q", "-m", "base")
	check := []string{"/bin/sh", "-c", `test "$FORMAT_MODE" = on`, "check"}

	outcome, err := (runtime.DiskMutationRunner{}).ApplyIndex(application.MutationLaunch{
		Argv: check, Directory: dir, SelectedPaths: []string{"a.md"},
		Environment: map[string]string{"FORMAT_MODE": "on"},
	})
	if err != nil || outcome.Code != 0 {
		t.Fatalf("projected environment not visible: %+v %v", outcome, err)
	}
}

// TestDiskMutationExposesTheSelectingSurface proves a mutator sees the same
// RHINO_GATE_SURFACE a check child sees, so one script can serve both kinds.
func TestDiskMutationExposesTheSelectingSurface(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "a.md", "a\n")
	run("add", "a.md")
	run("commit", "-q", "-m", "base")
	surface := []string{"/bin/sh", "-c", `test "$RHINO_GATE_SURFACE" = pre-commit`, "check"}

	outcome, err := (runtime.DiskMutationRunner{}).ApplyIndex(application.MutationLaunch{
		Argv: surface, Directory: dir, SelectedPaths: []string{"a.md"}, Surface: "pre-commit",
	})
	if err != nil || outcome.Code != 0 {
		t.Fatalf("selecting surface not visible to the mutator: %+v %v", outcome, err)
	}
}
