package runtime_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/runtime"
)

func gitRepo(t *testing.T) (string, func(...string) string) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", dir}, args...)...)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Skipf("git unavailable: %s", string(output))
		}
		return strings.TrimSpace(string(output))
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "gate@test")
	run("config", "user.name", "gate")
	return dir, run
}

// TestDiskTreeChangedFilesReadsOnlyTheRange proves an explicit range
// resolves to exactly the paths changed between its two commits.
func TestDiskTreeChangedFilesReadsOnlyTheRange(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "a.txt", "a\n")
	run("add", "a.txt")
	run("commit", "-q", "-m", "base")
	base := run("rev-parse", "HEAD")
	write(t, dir, "b.txt", "b\n")
	write(t, dir, "a.txt", "a2\n")
	run("add", "b.txt", "a.txt")
	run("commit", "-q", "-m", "head")
	head := run("rev-parse", "HEAD")
	tree := &runtime.DiskTree{Dir: dir}
	paths, err := tree.ChangedFiles(base, head)
	if err != nil || len(paths) != 2 || paths[0] != "a.txt" || paths[1] != "b.txt" {
		t.Fatalf("changed files not read: %v %v", paths, err)
	}
	if _, err := tree.ChangedFiles("not-a-commit!!", head); err == nil {
		t.Fatal("unreadable range was accepted")
	}
}

// TestDiskTreeRegularFilesSelectsOnlyRegularDestinations proves the raw
// record parser keeps regular modes while a staged symlink stays outside
// the selection.
func TestDiskTreeRegularFilesSelectsOnlyRegularDestinations(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "regular.txt", "r\n")
	write(t, dir, "target.txt", "t\n")
	if err := os.Symlink(filepath.Join(dir, "target.txt"), filepath.Join(dir, "linked.txt")); err != nil {
		t.Skip("symlinks unavailable")
	}
	run("add", "regular.txt", "linked.txt")
	tree := &runtime.DiskTree{Dir: dir}
	paths, err := tree.IndexedRegularFiles()
	if err != nil || len(paths) != 1 || paths[0] != "regular.txt" {
		t.Fatalf("regular index not selected: %v %v", paths, err)
	}
	run("commit", "-q", "-m", "base")
	write(t, dir, "regular.txt", "r2\n")
	run("add", "regular.txt")
	run("commit", "-q", "-m", "head")
	base := run("rev-parse", "HEAD~1")
	head := run("rev-parse", "HEAD")
	ranged, err := tree.ChangedRegularFiles(base, head)
	if err != nil || len(ranged) != 1 || ranged[0] != "regular.txt" {
		t.Fatalf("regular range not selected: %v %v", ranged, err)
	}
	plain := t.TempDir()
	bare := &runtime.DiskTree{Dir: plain}
	if _, err := bare.IndexedRegularFiles(); err == nil {
		t.Fatal("non-repository regular index did not refuse")
	}
}

// TestDiskTreeCommitMessagesReadsTheRange proves the message input reads the
// non-merge text between two commits.
func TestDiskTreeCommitMessagesReadsTheRange(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "a.txt", "a\n")
	run("add", "a.txt")
	run("commit", "-q", "-m", "base commit")
	base := run("rev-parse", "HEAD")
	write(t, dir, "b.txt", "b\n")
	run("add", "b.txt")
	run("commit", "-q", "-m", "head commit")
	head := run("rev-parse", "HEAD")
	tree := &runtime.DiskTree{Dir: dir}
	messages, err := tree.CommitMessages(base, head)
	if err != nil || !strings.Contains(messages, "head commit") {
		t.Fatalf("messages not read: %q %v", messages, err)
	}
}

// TestDiskTreeResolveGitRefResolvesTheFallback proves a declared fallback
// ref resolves to its commit while an invalid ref is refused.
func TestDiskTreeResolveGitRefResolvesTheFallback(t *testing.T) {
	dir, run := gitRepo(t)
	write(t, dir, "a.txt", "a\n")
	run("add", "a.txt")
	run("commit", "-q", "-m", "base")
	head := run("rev-parse", "HEAD")
	tree := &runtime.DiskTree{Dir: dir}
	resolved, err := tree.ResolveGitRef("main")
	if err != nil || resolved != head {
		t.Fatalf("fallback not resolved: %q %v", resolved, err)
	}
	if _, err := tree.ResolveGitRef("no-such-ref"); err == nil ||
		!strings.Contains(err.Error(), "does not resolve to a commit") {
		t.Fatalf("missing ref not refused: %v", err)
	}
	if _, err := tree.ResolveGitRef("-bad"); err == nil ||
		!strings.Contains(err.Error(), "fallback ref is invalid") {
		t.Fatalf("invalid ref not refused: %v", err)
	}
}

// TestDiskTreeHookMessageFileReadsOnlyTheCanonicalPath proves the hook file
// boundary reads Git's COMMIT_EDITMSG and refuses any other path.
func TestDiskTreeHookMessageFileReadsOnlyTheCanonicalPath(t *testing.T) {
	dir, run := gitRepo(t)
	canonical := run("rev-parse", "--path-format=absolute", "--git-path", "COMMIT_EDITMSG")
	if err := os.WriteFile(canonical, []byte("feat: disk hook\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tree := &runtime.DiskTree{Dir: dir}
	message, readErr := tree.HookMessageFile(canonical)
	if readErr != nil || message != "feat: disk hook\n" {
		t.Fatalf("hook file not read: %q %v", message, readErr)
	}
	if _, readErr := tree.HookMessageFile("elsewhere.txt"); readErr == nil ||
		!strings.Contains(readErr.Reason, "not Git's canonical COMMIT_EDITMSG") {
		t.Fatalf("foreign path not refused: %v", readErr)
	}
	missing, readErr := tree.HookMessageFile(filepath.Join(dir, "COMMIT_EDITMSG"))
	if readErr == nil {
		t.Fatalf("wrong canonical path accepted: %q", missing)
	}
}

// TestDiskTreeHookMessageFileRefusesOutsideGit proves a missing message file
// and a repository without Git map to their own refusals.
func TestDiskTreeHookMessageFileRefusesOutsideGit(t *testing.T) {
	dir, run := gitRepo(t)
	canonical := run("rev-parse", "--path-format=absolute", "--git-path", "COMMIT_EDITMSG")
	tree := &runtime.DiskTree{Dir: dir}
	if _, readErr := tree.HookMessageFile(canonical); readErr == nil || readErr.Kind != "missing" {
		t.Fatalf("absent hook file not missing: %v", readErr)
	}
	plain := &runtime.DiskTree{Dir: t.TempDir()}
	if _, readErr := plain.HookMessageFile("COMMIT_EDITMSG"); readErr == nil {
		t.Fatal("hook file outside git was read")
	}
	if _, err := plain.ResolveGitRef("main"); err == nil {
		t.Fatal("ref outside git resolved")
	}
	if _, err := plain.CommitMessages("aaaaaaa", "bbbbbbb"); err == nil {
		t.Fatal("messages outside git were read")
	}
}
