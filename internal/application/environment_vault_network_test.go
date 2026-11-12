package application_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validationSources are the directories whose non-test sources run when
// env validate runs: the application, its config and runtime adapters, the
// domain, and the command. Validation must reach no network package through any
// of them, so a validation run cannot make a network call.
var validationSources = []string{".", "../adapters/config", "../adapters/runtime", "../domain", "../../cmd/rhino"}

// networkImports names every import in one Go source that reaches the network.
func networkImports(t *testing.T, name string, source string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), name, source, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	var found []string
	for _, spec := range file.Imports {
		path := strings.Trim(spec.Path.Value, `"`)
		if path == "net" || strings.HasPrefix(path, "net/") || path == "crypto/tls" {
			found = append(found, path)
		}
	}
	return found
}

// TestValidationSourcesImportNoNetworkPackage fails when a non-test source on
// the validation path imports a network package.
func TestValidationSourcesImportNoNetworkPackage(t *testing.T) {
	for _, dir := range validationSources {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			source, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range networkImports(t, file, string(source)) {
				t.Errorf("%s imports %q, a network package on the validation path", file, path)
			}
		}
	}
}

// TestNetworkImportCheckRejectsANetworkPackage proves the check finds a network
// import, so an empty result above means no such import exists.
func TestNetworkImportCheckRejectsANetworkPackage(t *testing.T) {
	found := networkImports(t, "probe.go", "package probe\n\nimport (\n\t\"net/http\"\n\t\"crypto/tls\"\n)\n")
	if len(found) != 2 {
		t.Fatalf("found %v, want net/http and crypto/tls", found)
	}
}
