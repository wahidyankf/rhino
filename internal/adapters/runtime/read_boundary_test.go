package runtime_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The tree adapter is the read side every validator uses. These tests hold
// its reach to the code: which functions in disk.go may write, and which Git
// verbs it may start. Each finder has a positive control.

// osWriteFunctions are the os calls that create, change, or remove a file.
var osWriteFunctions = map[string]bool{
	"Chmod": true, "Chown": true, "Chtimes": true, "Create": true, "CreateTemp": true,
	"Lchown": true, "Link": true, "Mkdir": true, "MkdirAll": true, "MkdirTemp": true,
	"OpenFile": true, "Remove": true, "RemoveAll": true, "Rename": true, "Symlink": true,
	"Truncate": true, "WriteFile": true,
}

// readOnlyGitVerbs are the only Git subcommands the tree adapter may start.
var readOnlyGitVerbs = map[string]bool{"diff": true, "log": true, "rev-parse": true}

// adapterScan records, per function, the os write calls it makes and the Git
// verb each exec.Command call starts. A verb that is not a literal is "?".
type adapterScan struct {
	writes   map[string][]string
	gitVerbs map[string][]string
}

func functionName(decl *ast.FuncDecl) string {
	if decl.Recv == nil || len(decl.Recv.List) == 0 {
		return decl.Name.Name
	}
	receiver := decl.Recv.List[0].Type
	if star, ok := receiver.(*ast.StarExpr); ok {
		receiver = star.X
	}
	if ident, ok := receiver.(*ast.Ident); ok {
		return ident.Name + "." + decl.Name.Name
	}
	return decl.Name.Name
}

// gitVerb reads the subcommand of exec.Command("git", "-C", dir, verb, ...).
func gitVerb(args []ast.Expr) string {
	if len(args) < 4 {
		return "?"
	}
	program, ok := args[0].(*ast.BasicLit)
	if !ok || program.Value != `"git"` {
		return "?"
	}
	verb, ok := args[3].(*ast.BasicLit)
	if !ok {
		return "?"
	}
	text, err := strconv.Unquote(verb.Value)
	if err != nil {
		return "?"
	}
	return text
}

func scanAdapter(t *testing.T, name string, source any) adapterScan {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), name, source, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	scan := adapterScan{writes: map[string][]string{}, gitVerbs: map[string][]string{}}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		owner := functionName(fn)
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			switch {
			case pkg.Name == "os" && osWriteFunctions[selector.Sel.Name]:
				scan.writes[owner] = append(scan.writes[owner], selector.Sel.Name)
			case pkg.Name == "exec" && selector.Sel.Name == "Command":
				scan.gitVerbs[owner] = append(scan.gitVerbs[owner], gitVerb(call.Args))
			}
			return true
		})
	}
	return scan
}

func namesOf(values map[string][]string) []string {
	var names []string
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func TestTreeAdapterScannerFindsWritesAndGitVerbsInASyntheticFile(t *testing.T) {
	source := `package p

import (
	"os"
	"os/exec"
)

type DiskTree struct{ Dir string }

func (t *DiskTree) Bad() {
	_ = os.WriteFile("x", nil, 0o600)
	_ = exec.Command("git", "-C", t.Dir, "push").Run()
}
`
	scan := scanAdapter(t, "synthetic.go", source)
	if got := scan.writes["DiskTree.Bad"]; len(got) != 1 || got[0] != "WriteFile" {
		t.Fatalf("control writes = %v, want [WriteFile] under DiskTree.Bad", scan.writes)
	}
	if got := scan.gitVerbs["DiskTree.Bad"]; len(got) != 1 || got[0] != "push" {
		t.Fatalf("control git verbs = %v, want [push] under DiskTree.Bad", scan.gitVerbs)
	}
}

// TestTreeAdapterWritesOnlyInTheEnvironmentTransaction pins every function in
// disk.go that can change the disk. Only the environment store's transaction
// writes; the tree reads. A new writer must be added here deliberately.
func TestTreeAdapterWritesOnlyInTheEnvironmentTransaction(t *testing.T) {
	scan := scanAdapter(t, "disk.go", nil)
	// Positive control: the environment transaction really writes.
	if len(scan.writes) == 0 {
		t.Fatal("control: disk.go shows no write call at all")
	}
	want := []string{
		"installEnvironmentFiles",
		"removeEnvironmentFiles",
		"restoreEnvironmentFiles",
		"stageEnvironmentFile",
	}
	got := namesOf(scan.writes)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("functions writing to disk = %v, want %v", got, want)
	}
	for owner := range scan.writes {
		if strings.HasPrefix(owner, "DiskTree.") {
			t.Errorf("%s writes to disk: %v", owner, scan.writes[owner])
		}
	}
}

// TestTreeAdapterStartsOnlyReadOnlyGitQueries pins the child processes disk.go
// starts: Git queries that read the index, a range, a ref, or a message.
func TestTreeAdapterStartsOnlyReadOnlyGitQueries(t *testing.T) {
	scan := scanAdapter(t, "disk.go", nil)
	// Positive control: the tree adapter does start Git queries.
	if len(scan.gitVerbs) == 0 {
		t.Fatal("control: disk.go starts no Git query at all")
	}
	for owner, verbs := range scan.gitVerbs {
		for _, verb := range verbs {
			if !readOnlyGitVerbs[verb] {
				t.Errorf("%s starts git %q, which is not a read-only query", owner, verb)
			}
		}
	}
}
