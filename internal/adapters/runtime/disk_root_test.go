package runtime_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/runtime"
)

// A repository reached through a symbolic link, such as a temporary directory
// under /tmp on macOS, must still root its own subdirectories. The working
// directory a shell reports keeps the link, so containment compares the resolved
// candidate with the resolved root and not with the link's spelling.
func TestDiskTreeRootedAtAcceptsARootReachedThroughALink(t *testing.T) {
	target := t.TempDir()
	if err := os.Mkdir(filepath.Join(target, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(filepath.Join(target, "sub"))
	if err != nil {
		t.Fatal(err)
	}
	tree := &runtime.DiskTree{Dir: link}
	rooted, err := tree.RootedAt("sub")
	if err != nil {
		t.Fatalf("a subdirectory of a linked root was refused: %v", err)
	}
	if rooted.Root() != want {
		t.Fatalf("root = %q, want %q", rooted.Root(), want)
	}
	if _, err := tree.RootedAt("../outside"); err == nil {
		t.Fatal("an escaping root was accepted through a link")
	}
}
