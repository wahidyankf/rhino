package e2e_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestVersionJourney asks the built binary for its version as JSON, and reads
// one schema-versioned object from standard output.
func TestVersionJourney(t *testing.T) {
	status, stdout, stderr := runRhino(t, t.TempDir(), "version", "--json")
	if status != 0 {
		t.Fatalf("exit %d; want 0; stderr: %s", status, stderr)
	}
	var document struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal([]byte(stdout), &document); err != nil {
		t.Fatalf("standard output is not one JSON object: %v\n%s", err, stdout)
	}
	if document.SchemaVersion != 1 {
		t.Fatalf("schemaVersion %d; want 1", document.SchemaVersion)
	}
}

// TestEmojiFindingJourney runs the emoji check in a repository whose JSON file
// holds a prohibited emoji, and expects the finding exit status.
func TestEmojiFindingJourney(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "repo-config.yml"),
		"schema: rhino/repo-config/v2\npolicies:\n  conventions:\n    emoji:\n      prohibited:\n        - glob: \"**/*.json\"\n")
	writeFile(t, filepath.Join(repo, "config", "a.json"), "{\"emoji\": \"\U0001F98F\"}\n")

	status, _, stderr := runRhino(t, repo, "convention", "emoji", "validate")
	if status != 1 {
		t.Fatalf("exit %d; want 1 for one prohibited emoji; stderr: %s", status, stderr)
	}
}

// runRhino starts the built binary in dir with args, and returns its exit
// status and both output streams. A start failure stops the test.
func runRhino(t *testing.T, dir string, args ...string) (int, string, string) {
	t.Helper()
	command := exec.Command(rhinoBinary, args...)
	command.Dir = dir
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err == nil {
		return 0, stdout.String(), stderr.String()
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("start rhino %v: %v", args, err)
	}
	return exit.ExitCode(), stdout.String(), stderr.String()
}

// writeFile creates path with content, and creates its parent directories.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
