package bootstrap_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// These tests hold the product's reach claims to the code. Each finder has a
// positive control: the same search must find a case that is certainly there,
// so a search that finds nothing is never mistaken for a broken search.

const modulePath = "github.com/wahidyankf/rhino"

// productCommand is the binary whose dependency graph is the whole product.
const productCommand = modulePath + "/cmd/rhino"

// validatorCores are the packages every tree validator runs through.
var validatorCores = []string{
	modulePath + "/internal/domain",
	modulePath + "/internal/application",
	modulePath + "/internal/adapters/config",
}

// productRoots hold every product source file, relative to this package.
var productRoots = []string{"../../cmd", "../../internal"}

// runtimeAdapter is the only directory allowed to start child processes.
const runtimeAdapter = "internal/adapters/runtime"

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

// productSources lists every non-test Go source file in the product roots.
func productSources(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, root := range productRoots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	return files
}

// dependencyGraph lists every package the named packages depend on, the
// standard library included, through the go tool's own resolution.
func dependencyGraph(t *testing.T, packages ...string) []string {
	t.Helper()
	args := append([]string{"list", "-deps", "-f", "{{.ImportPath}}"}, packages...)
	output, err := exec.Command("go", args...).Output()
	if err != nil {
		t.Fatalf("go list -deps %v: %v", packages, err)
	}
	return strings.Fields(string(output))
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

func TestValidatorCoresImportNoOperatingSystemProcessNetworkOrRawPackage(t *testing.T) {
	for _, core := range validatorCores {
		dir := filepath.Join("..", strings.TrimPrefix(core, modulePath+"/internal/"))
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read core %s: %v", dir, err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			if found := banned(importsOf(t, path, nil), isValidatorCoreForbidden); len(found) != 0 {
				t.Errorf("%s imports %v", path, found)
			}
		}
	}
}

func TestValidatorCoreDependencyGraphStartsNoProcessAndOpensNoSocket(t *testing.T) {
	graph := dependencyGraph(t, validatorCores...)
	if found := banned(graph, isProcess); len(found) != 0 {
		t.Errorf("validator cores depend on %v", found)
	}
	if found := banned(graph, isNetwork); len(found) != 0 {
		t.Errorf("validator cores depend on %v", found)
	}
	// Positive control: the product graph does start processes, so the same
	// finder must see os/exec there.
	product := dependencyGraph(t, productCommand)
	if found := banned(product, isProcess); len(found) != 1 || found[0] != "os/exec" {
		t.Fatalf("control: product graph process packages = %v, want [os/exec]", found)
	}
}

func TestProductDependencyGraphHasNoNetworkPackage(t *testing.T) {
	product := dependencyGraph(t, productCommand)
	if found := banned(product, isNetwork); len(found) != 0 {
		t.Errorf("the rhino binary depends on %v, so it can open a socket", found)
	}
}

func TestProductSourcesImportNoNetworkUnsafeOrCgo(t *testing.T) {
	for _, path := range productSources(t) {
		if found := banned(importsOf(t, path, nil), func(p string) bool {
			return isNetwork(p) || p == "unsafe" || p == "C" || p == "plugin"
		}); len(found) != 0 {
			t.Errorf("%s imports %v", path, found)
		}
	}
}

// TestSyscallIsConfinedToSignalAndPipeHandling pins the one product file that
// may import syscall: the CLI adapter, for signal numbers and EPIPE. Any other
// file importing it could make a raw socket call, so it must be reviewed here.
func TestSyscallIsConfinedToSignalAndPipeHandling(t *testing.T) {
	var importers []string
	for _, path := range productSources(t) {
		if len(banned(importsOf(t, path, nil), func(p string) bool { return p == "syscall" })) != 0 {
			importers = append(importers, filepath.ToSlash(path))
		}
	}
	want := []string{"../../internal/adapters/cli/app.go"}
	if strings.Join(importers, ",") != strings.Join(want, ",") {
		t.Errorf("product files importing syscall = %v, want %v", importers, want)
	}
}

func TestProcessSpawningStaysInTheRuntimeAdapter(t *testing.T) {
	var importers []string
	for _, path := range productSources(t) {
		if len(banned(importsOf(t, path, nil), func(p string) bool { return p == "os/exec" })) == 0 {
			continue
		}
		importers = append(importers, filepath.ToSlash(path))
	}
	// Positive control: the runtime adapter starts the gate, toolchain, and
	// adapter children, so the search must find it.
	if len(importers) == 0 {
		t.Fatal("control: no product source imports os/exec")
	}
	for _, path := range importers {
		if !strings.Contains(path, "/"+runtimeAdapter+"/") {
			t.Errorf("%s imports os/exec outside the runtime adapter", path)
		}
	}
}
