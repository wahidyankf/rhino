package runtime_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/runtime"
)

// namesGit runs one Git command in dir, with the developer's Git configuration
// kept out of the result.
func namesGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	if output, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}

// TestDiskTreeNameQueriesRefuseANonRepository proves both name queries refuse a
// directory that is not a Git repository, and name the refusal.
func TestDiskTreeNameQueriesRefuseANonRepository(t *testing.T) {
	tree := &runtime.DiskTree{Dir: t.TempDir()}
	if _, err := tree.TrackedFiles(); err == nil || !strings.Contains(err.Error(), "Git refused the tracked name query") {
		t.Fatalf("TrackedFiles err = %v, want a Git refusal", err)
	}
	if _, err := tree.IgnoredFiles(); err == nil || !strings.Contains(err.Error(), "Git refused the ignored name query") {
		t.Fatalf("IgnoredFiles err = %v, want a Git refusal", err)
	}
}

// TestDiskTreeNameQueriesListSortedUniqueNames proves the tracked names are the
// index's paths, sorted, and the ignored names are the untracked paths Git
// ignores, sorted, with no file content read.
func TestDiskTreeNameQueriesListSortedUniqueNames(t *testing.T) {
	dir := t.TempDir()
	namesGit(t, dir, "init", "-q")
	write := func(path, content string) {
		t.Helper()
		full := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("b.txt", "b\n")
	write("a/x.env", "x\n")
	write(".gitignore", "build/\n")
	write("build/out.log", "log\n")
	namesGit(t, dir, "add", "--", "b.txt", "a/x.env", ".gitignore")

	tree := &runtime.DiskTree{Dir: dir}
	tracked, err := tree.TrackedFiles()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{".gitignore", "a/x.env", "b.txt"}; !reflect.DeepEqual(tracked, want) {
		t.Fatalf("tracked = %v, want %v", tracked, want)
	}
	ignored, err := tree.IgnoredFiles()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"build/out.log"}; !reflect.DeepEqual(ignored, want) {
		t.Fatalf("ignored = %v, want %v", ignored, want)
	}
}
