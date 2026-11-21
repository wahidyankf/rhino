package bootstrap_test

import (
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"testing"
)

// These tests hold the product's reach claims to the code. Each finder has a
// positive control: the same search must find a case that is certainly there,
// so a search that finds nothing is never mistaken for a broken search.

func isNetwork(path string) bool {
	return path == "net" || strings.HasPrefix(path, "net/")
}

func isProcess(path string) bool {
	return path == "os/exec" || path == "plugin"
}

// isRawAccess names packages that reach past Go's safe, portable surface:
// raw system calls, unchecked memory, cgo, and dynamic plugins.
func isRawAccess(path string) bool {
	return path == "syscall" || path == "unsafe" || path == "C" || path == "plugin"
}

// isValidatorCoreForbidden names what a validator core must never import
// directly: the operating system, processes, sockets, and raw access. A core
// without "os" cannot open, write, or remove a file on its own.
func isValidatorCoreForbidden(path string) bool {
	return path == "os" || isNetwork(path) || isProcess(path) || isRawAccess(path)
}

// banned returns the paths that the rule rejects, sorted and de-duplicated.
func banned(paths []string, rule func(string) bool) []string {
	seen := map[string]bool{}
	var found []string
	for _, path := range paths {
		if rule(path) && !seen[path] {
			seen[path] = true
			found = append(found, path)
		}
	}
	sort.Strings(found)
	return found
}

// importsOf lists the import paths of one Go source file.
func importsOf(t *testing.T, name string, source any) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), name, source, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse imports of %s: %v", name, err)
	}
	var paths []string
	for _, spec := range file.Imports {
		paths = append(paths, strings.Trim(spec.Path.Value, `"`))
	}
	return paths
}

func TestImportRulesClassifyEachPathAsDeclared(t *testing.T) {
	// Positive control for every rule the tests below apply.
	cases := []struct {
		path string
		rule func(string) bool
		want bool
	}{
		{"net", isNetwork, true},
		{"net/http", isNetwork, true},
		{"network", isNetwork, false},
		{"os/exec", isProcess, true},
		{"plugin", isProcess, true},
		{"unsafe", isRawAccess, true},
		{"syscall", isRawAccess, true},
		{"os", isValidatorCoreForbidden, true},
		{"strings", isValidatorCoreForbidden, false},
		{"io/fs", isValidatorCoreForbidden, false},
	}
	for _, c := range cases {
		if got := c.rule(c.path); got != c.want {
			t.Errorf("rule(%q) = %t, want %t", c.path, got, c.want)
		}
	}
}

func TestSourceFinderFindsBannedImportsInASyntheticFile(t *testing.T) {
	source := "package p\n\nimport (\n\t\"net/http\"\n\t\"os/exec\"\n\t\"strings\"\n)\n"
	got := banned(importsOf(t, "synthetic.go", source), isNetwork)
	if len(got) != 1 || got[0] != "net/http" {
		t.Fatalf("control found %v, want [net/http]", got)
	}
}
