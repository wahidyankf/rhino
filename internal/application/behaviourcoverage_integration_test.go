// Integration level: this file reaches the real host, so the default build omits it (see .golangci.yml).

//go:build integration

package application_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// behaviourConfig declares the corpus, the fixtures, and the three levels, each
// with a glob that matches one file of its own.
// TestBehaviourCoverageIsStatic proves the behaviour validator is static: its
// sources import no process, syscall, or network package and call no process
// or network function. A test must assert that, because the spec makes it the
// one property a reviewer cannot see in the output.
func TestBehaviourCoverageIsStatic(t *testing.T) {
	forbiddenImports := map[string]bool{
		"os/exec": true, "syscall": true, "net": true, "net/http": true, "plugin": true,
	}
	forbiddenSelectors := map[string]bool{"exec": true, "syscall": true, "net": true, "http": true}
	var sources []string
	for _, pattern := range []string{"behaviourcoverage*.go", "gherkin*.go"} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range matches {
			if !strings.HasSuffix(match, "_test.go") {
				sources = append(sources, match)
			}
		}
	}
	if len(sources) == 0 {
		t.Fatal("no behaviour coverage sources found to check")
	}
	for _, source := range sources {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, source, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("%s: %v", source, err)
		}
		for _, imported := range file.Imports {
			path, _ := strconv.Unquote(imported.Path.Value)
			if forbiddenImports[path] {
				t.Errorf("%s imports %q; the behaviour validator must start no process and reach no network", source, path)
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if ident, ok := selector.X.(*ast.Ident); ok && forbiddenSelectors[ident.Name] {
				t.Errorf("%s calls %s.%s; the behaviour validator must start no process and reach no network",
					source, ident.Name, selector.Sel.Name)
			}
			return true
		})
	}
}
