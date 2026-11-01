package application

import (
	"reflect"
	"testing"
)

// listedTree is a tree that knows only its file list, which is all a shaped
// tree reads from the tree it wraps.
type listedTree struct {
	Tree
	files []string
}

func (t listedTree) Files() []string { return t.files }

// TestShapedTreeAnswersAsTheFileListHelpersDo proves the one-pass index gives
// the answers ChildrenOf and IsDirectoryOf give for every spelling a caller
// can use: the root, nested and missing directories, a file, a sibling that
// shares a prefix, a trailing slash, and an unsorted listing.
func TestShapedTreeAnswersAsTheFileListHelpersDo(t *testing.T) {
	listings := map[string][]string{
		"unsorted with siblings sharing a prefix": {"b/c.md", "a.md", "a/z.md", "a/b/c.md", "a/b/d.md", "README.md", "a/y.md"},
		"a trailing slash and an empty segment":   {"a/", "x//y"},
		"empty":                                   nil,
	}
	directories := []string{"", ".", "/", "a", "a/", "/a/", "a/b", "a/b/c.md", "a.md", "b", "x", "x/", "x//y", "missing", "a/missing"}
	for name, files := range listings {
		t.Run(name, func(t *testing.T) {
			shaped := newShapedTree(listedTree{files: files})
			if !reflect.DeepEqual(shaped.Files(), files) {
				t.Errorf("Files() = %v, want %v", shaped.Files(), files)
			}
			for _, directory := range directories {
				if got, want := shaped.Children(directory), ChildrenOf(files, directory); !reflect.DeepEqual(got, want) {
					t.Errorf("Children(%q) = %v, want %v", directory, got, want)
				}
				if got, want := shaped.IsDirectory(directory), IsDirectoryOf(files, directory); got != want {
					t.Errorf("IsDirectory(%q) = %v, want %v", directory, got, want)
				}
			}
		})
	}
}

// TestShapedTreeChildrenAreACopy proves a caller sorting or editing the
// answer cannot change a later answer.
func TestShapedTreeChildrenAreACopy(t *testing.T) {
	shaped := newShapedTree(listedTree{files: []string{"a/x.md", "a/y.md"}})
	first := shaped.Children("a")
	first[0] = "changed"
	if got := shaped.Children("a"); got[0] != "a/x.md" {
		t.Errorf("a caller's edit reached the index: %v", got)
	}
}
