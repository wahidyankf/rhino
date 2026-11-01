package runtime_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/runtime"
)

// A repository reached through a symbolic link, such as a temporary directory
// under /tmp on macOS, must still root its own subdirectories. The working
// directory a shell reports keeps the link, so the root is resolved before it
// is returned and never compared with the link's spelling.
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
}

// `--root` names the repository rather than a selection inside it, so a
// directory outside the working directory is accepted, whether it is reached
// through `..` or named by an absolute path, and the rerooted tree reads that
// repository's own files.
func TestDiskTreeRootedAtAcceptsADirectoryOutsideTheWorkingDirectory(t *testing.T) {
	parent := t.TempDir()
	working := filepath.Join(parent, "repo")
	other := filepath.Join(parent, "other-repo")
	write(t, working, "README.md", "# Repo\n")
	write(t, other, "repo-config.yml", "schema: rhino/repo-config/v2\n")
	want, err := filepath.EvalSymlinks(other)
	if err != nil {
		t.Fatal(err)
	}
	tree := &runtime.DiskTree{Dir: working}
	for _, path := range []string{
		"../other-repo",
		"../other-repo/",
		"../other-repo/../other-repo",
		filepath.Join("..", "other-repo"),
		other,
	} {
		rooted, err := tree.RootedAt(path)
		if err != nil {
			t.Fatalf("root %q was refused: %v", path, err)
		}
		if rooted.Root() != want {
			t.Fatalf("root %q = %q, want %q", path, rooted.Root(), want)
		}
		if text, readErr := rooted.Read("repo-config.yml"); readErr != nil || !strings.Contains(text, "rhino/repo-config/v2") {
			t.Fatalf("root %q did not read its own configuration: %q %+v", path, text, readErr)
		}
	}
}

// A `..` root is the working directory's parent, the directory the operating
// system reaches by naming it.
func TestDiskTreeRootedAtAcceptsTheParentDirectory(t *testing.T) {
	parent := t.TempDir()
	working := filepath.Join(parent, "repo")
	write(t, working, "README.md", "# Repo\n")
	want, err := filepath.EvalSymlinks(parent)
	if err != nil {
		t.Fatal(err)
	}
	rooted, err := (&runtime.DiskTree{Dir: working}).RootedAt("..")
	if err != nil {
		t.Fatalf("the parent directory was refused: %v", err)
	}
	if rooted.Root() != want {
		t.Fatalf("root = %q, want %q", rooted.Root(), want)
	}
}

// A `..` segment below a linked working directory climbs from the link's
// target, as the operating system resolves it, and not from the link's
// spelling.
func TestDiskTreeRootedAtClimbsFromTheLinkTargetNotTheLinkSpelling(t *testing.T) {
	parent := t.TempDir()
	working := filepath.Join(parent, "repo")
	write(t, working, "README.md", "# Repo\n")
	write(t, filepath.Join(parent, "other-repo"), "repo-config.yml", "schema: rhino/repo-config/v2\n")
	spelled := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(working, spelled); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	want, err := filepath.EvalSymlinks(filepath.Join(parent, "other-repo"))
	if err != nil {
		t.Fatal(err)
	}
	rooted, err := (&runtime.DiskTree{Dir: spelled}).RootedAt("../other-repo")
	if err != nil {
		t.Fatalf("a parent-relative root below a link was refused: %v", err)
	}
	if rooted.Root() != want {
		t.Fatalf("root = %q, want %q", rooted.Root(), want)
	}
}

// A root that names nothing, a missing directory, or a file is refused with
// the same reason wherever it lies, inside the working directory or outside it.
func TestDiskTreeRootedAtRefusesMissingAndNonDirectoryRoots(t *testing.T) {
	parent := t.TempDir()
	working := filepath.Join(parent, "repo")
	write(t, working, "README.md", "# Repo\n")
	write(t, parent, "notes.md", "# Notes\n")
	tree := &runtime.DiskTree{Dir: working}
	for _, path := range []string{"", "   ", "../missing", "missing-dir-xyz", "README.md", "../notes.md"} {
		_, err := tree.RootedAt(path)
		if err == nil {
			t.Fatalf("root %q was accepted", path)
		}
		if !strings.Contains(err.Error(), "names nothing") {
			t.Fatalf("root %q refused with %q, want a names-nothing refusal", path, err.Error())
		}
	}
}
