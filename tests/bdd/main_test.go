// Package bdd_test executes the Gherkin corpus under specs/behaviours against
// the real rhino binary. Every scenario is a command-line scenario, so one
// adapter drives the compiled executable in an isolated temporary repository.
package bdd_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// rhinoBinary is the compiled executable under test. TestMain assigns it once,
// before any test runs, and no test writes it afterwards.
var rhinoBinary string

// TestMain builds the rhino command once into a temporary directory and removes
// that directory after the run. Building in TestMain keeps the build out of
// every scenario's time budget and gives all scenarios the same binary.
func TestMain(m *testing.M) {
	os.Exit(runWithBinary(m))
}

func runWithBinary(m *testing.M) int {
	dir, err := os.MkdirTemp("", "rhino-bdd-binary-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create binary directory: %v\n", err)
		return 2
	}
	defer os.RemoveAll(dir)

	name := "rhino"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	rhinoBinary = filepath.Join(dir, name)
	if err := buildRhino(rhinoBinary); err != nil {
		fmt.Fprintf(os.Stderr, "build rhino for the behaviour suite: %v\n", err)
		return 2
	}
	return m.Run()
}

// buildRhino compiles ./cmd/rhino from the module root, two levels above this
// package. It passes argv directly, with no shell.
func buildRhino(output string) error {
	goTool, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("locate go toolchain: %w", err)
	}
	command := exec.Command(goTool, "build", "-o", output, "./cmd/rhino")
	command.Dir = filepath.Join("..", "..")
	if out, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("go build ./cmd/rhino: %w\n%s", err, out)
	}
	return nil
}
