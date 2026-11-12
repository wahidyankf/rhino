package fakebw

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Package is the import path of the fake's command, built once per test run.
const Package = "github.com/wahidyankf/rhino/tests/bdd/vault/fakebw/cmd/fakebw"

// Build compiles the fake command into dir and returns the executable's path.
// It runs from the calling test's directory, which lies inside the module, so
// the import path resolves.
func Build(dir string) (string, error) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("locate go toolchain: %w", err)
	}
	output := filepath.Join(dir, "fakebw")
	command := exec.Command(goTool, "build", "-o", output, Package)
	if out, err := command.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build %s: %w\n%s", Package, err, out)
	}
	return output, nil
}

// InstallWrapper writes an executable named bw into binDir. The wrapper runs
// the built fake with this scenario's state and log, so the adapter, which
// starts bw by name on PATH, reaches the fake and nothing else.
func InstallWrapper(binDir, binary, statePath, logPath string) error {
	script := "#!/bin/sh\n" +
		"exec " + shellQuote(binary) +
		" --state " + shellQuote(statePath) +
		" --log " + shellQuote(logPath) +
		" -- \"$@\"\n"
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("create bw directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "bw"), []byte(script), 0o755); err != nil {
		return fmt.Errorf("write bw wrapper: %w", err)
	}
	return os.Chmod(filepath.Join(binDir, "bw"), 0o755)
}

// shellQuote quotes one word for a POSIX shell script.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
