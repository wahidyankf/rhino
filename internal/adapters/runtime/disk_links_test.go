package runtime_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/runtime"
	"github.com/wahidyankf/rhino/internal/application"
)

// TestReadRefusesPathBehindDirectoryLink proves a read never goes through a
// directory link, even when the file itself is an ordinary file outside the
// repository. The final component is not a link, so only the walk of every
// component catches it.
func TestReadRefusesPathBehindDirectoryLink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	write(t, outside, "main.rs", "fn main() {}\n")
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	tree := &runtime.DiskTree{Dir: root}
	if _, err := tree.Read("linked/main.rs"); err == nil || err.Kind != "escapes" {
		t.Fatalf("read through a directory link not refused as escapes: %+v", err)
	}
}

// TestLinksClassifiesEachTarget proves a link is listed with where its target
// lies: inside the root, outside it, or nowhere at all.
func TestLinksClassifiesEachTarget(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	write(t, root, "held/long.md", "one two\n")
	links := map[string]string{
		"inside":   "held",
		"outside":  outside,
		"dangling": "missing-target",
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Skipf("symbolic links unavailable: %v", err)
		}
	}
	tree := &runtime.DiskTree{Dir: root}
	got := map[string]application.TreeLink{}
	for _, link := range tree.Links() {
		got[link.Path] = link
	}
	want := map[string]application.TreeLink{
		"inside":   {Path: "inside", Target: "held", Kind: application.LinkWithin},
		"outside":  {Path: "outside", Kind: application.LinkOutside},
		"dangling": {Path: "dangling", Kind: application.LinkUnresolved},
	}
	if len(got) != len(want) {
		t.Fatalf("links listed %+v, want %+v", got, want)
	}
	for path, expected := range want {
		if got[path] != expected {
			t.Fatalf("link %s classified %+v, want %+v", path, got[path], expected)
		}
	}
}
