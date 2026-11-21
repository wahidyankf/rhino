package runtime

import "testing"

// TestDiskTreeChildrenAndDirectoryQueriesReadTheTreeFiles proves the directory
// queries answer from the tree's own files: a directory holding a file is a
// child and a directory, and a file is neither.
func TestDiskTreeChildrenAndDirectoryQueriesReadTheTreeFiles(t *testing.T) {
	useDoubles(t, repository(&callLog{}, map[string]string{"a.txt": "a", "dir/b.txt": "b"}), nil, nil)
	tree := DiskTree{Dir: "/repo"}
	if got := tree.Children("dir"); !equalStrings(got, []string{"dir/b.txt"}) {
		t.Fatalf("Children(dir) = %v, want [dir/b.txt]", got)
	}
	if !tree.IsDirectory("dir") {
		t.Fatal("IsDirectory(dir) = false, want true")
	}
	if tree.IsDirectory("a.txt") || tree.IsDirectory("missing") {
		t.Fatal("IsDirectory reported a file or an absent path as a directory")
	}
}
