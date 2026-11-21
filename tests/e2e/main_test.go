// Package e2e_test drives the built rhino binary as a child process, the way a
// consumer runs it. Each journey observes only the arguments it passes, the exit
// status, and the two output streams.
package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/wahidyankf/rhino/tests/bdd/vault/fakebw"
)

// rhinoBinary is the compiled executable under test, and fakeBWBinary the
// compiled fake of the Bitwarden CLI that vault scenarios install as bw.
// TestMain builds both once, before any test runs, and no test writes them
// afterwards.
var (
	rhinoBinary  string
	fakeBWBinary string
)

// TestMain builds ./cmd/rhino and the fake bw into a temporary directory and
// removes that directory after the run.
func TestMain(m *testing.M) {
	os.Exit(runWithBinary(m))
}

func runWithBinary(m *testing.M) int {
	dir, err := os.MkdirTemp("", "rhino-e2e-binary-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create binary directory: %v\n", err)
		return 2
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintf(os.Stderr, "remove binary directory: %v\n", err)
		}
	}()

	name := "rhino"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	rhinoBinary = filepath.Join(dir, name)
	if err := buildRhino(rhinoBinary); err != nil {
		fmt.Fprintf(os.Stderr, "build rhino for the end-to-end suite: %v\n", err)
		return 2
	}
	fake, err := fakebw.Build(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build the fake bw for the end-to-end suite: %v\n", err)
		return 2
	}
	fakeBWBinary = fake
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
