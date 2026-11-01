package application_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// walkCountingTree charges one filesystem walk to every query the disk adapter
// answers by walking the whole repository: the file list, a directory's
// children, the directory test, and an existence check. It lets a test prove
// how often a validator makes the adapter walk without timing anything.
type walkCountingTree struct {
	*memoryTree
	walks int
}

func (t *walkCountingTree) Files() []string {
	t.walks++
	return t.memoryTree.Files()
}

func (t *walkCountingTree) Children(directory string) []string {
	t.walks++
	return t.memoryTree.Children(directory)
}

func (t *walkCountingTree) IsDirectory(path string) bool {
	t.walks++
	return t.memoryTree.IsDirectory(path)
}

func (t *walkCountingTree) Exists(path string) bool {
	t.walks++
	return t.memoryTree.Exists(path)
}

// wideReadmeTree builds a complete-index tree of areas directories, each with
// an index linking its own pages and the next area, so every check the
// validator makes has work to do.
func wideReadmeTree(areas, pages int) *walkCountingTree {
	tree := &walkCountingTree{memoryTree: newMemoryTree("/root")}
	tree.files["repo-config.yml"] = readmeCompleteConfig
	root := &strings.Builder{}
	root.WriteString("# Docs\n\n")
	for area := 0; area < areas; area++ {
		fmt.Fprintf(root, "[area%d](area%d)\n", area, area)
		index := &strings.Builder{}
		fmt.Fprintf(index, "# Area %d\n\n", area)
		for page := 0; page < pages; page++ {
			fmt.Fprintf(index, "[page%d](page%d.md)\n", page, page)
			tree.files[fmt.Sprintf("docs/area%d/page%d.md", area, page)] = "# Page\n"
		}
		tree.files[fmt.Sprintf("docs/area%d/README.md", area)] = index.String()
	}
	tree.files["docs/README.md"] = root.String()
	return tree
}

// TestReadmeIndexWalksTheRepositoryOnceNotPerQuery proves a complete-index run
// asks the tree for its shape once. The disk adapter walks the whole
// repository on every shape query, so a validator that asks per directory and
// per link costs the number of queries times the size of the repository.
func TestReadmeIndexWalksTheRepositoryOnceNotPerQuery(t *testing.T) {
	tree := wideReadmeTree(400, 3)
	outcome := application.Execute(tree, []string{"md", "readme-index", "validate"}, nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0\n%s%s", outcome.ExitCode, outcome.Stdout, outcome.Stderr)
	}
	if !strings.Contains(outcome.Stdout, "checked 401 directories, no findings") {
		t.Fatalf("unexpected report:\n%s", outcome.Stdout)
	}
	if tree.walks > 1 {
		t.Errorf("the run walked the repository %d times, want at most 1", tree.walks)
	}
}

// TestReadmeIndexDirectChildrenWalksTheRepositoryOnce proves the direct-child
// mode shares the single snapshot too.
func TestReadmeIndexDirectChildrenWalksTheRepositoryOnce(t *testing.T) {
	tree := &walkCountingTree{memoryTree: newMemoryTree("/root")}
	tree.files["repo-config.yml"] = readmeStrictConfig
	index := &strings.Builder{}
	for page := 0; page < 300; page++ {
		fmt.Fprintf(index, "[page%d](page%d.md)\n", page, page)
		tree.files[fmt.Sprintf("docs/page%d.md", page)] = "# Page\n"
	}
	tree.files["docs/README.md"] = index.String()
	outcome := application.Execute(tree, []string{"md", "readme-index", "validate"}, nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0\n%s%s", outcome.ExitCode, outcome.Stdout, outcome.Stderr)
	}
	if tree.walks > 1 {
		t.Errorf("the run walked the repository %d times, want at most 1", tree.walks)
	}
}
