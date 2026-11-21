// Integration level: this file reaches the real host, so the default build omits it (see .golangci.yml).

//go:build integration

package bootstrap_test

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests hold the product's reach claims to the code. Each finder has a
// positive control: the same search must find a case that is certainly there,
// so a search that finds nothing is never mistaken for a broken search.

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
