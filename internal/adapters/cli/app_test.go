package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/cli"
	"github.com/wahidyankf/rhino/internal/adapters/runtime"
	"github.com/wahidyankf/rhino/internal/application"
)

func wiredCliEnvironment() application.EnvironmentStore { return runtime.DiskEnvironmentStore{} }

func writeFile(t *testing.T, name, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runArgs(t *testing.T, dir string, args []string) (int, string, string) {
	t.Helper()
	t.Chdir(dir)
	app := cli.Application{Stdin: strings.NewReader(""), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	code := app.RunArgs(context.Background(), args)
	return code, app.Stdout.(*bytes.Buffer).String(), app.Stderr.(*bytes.Buffer).String()
}

func TestRunArgsCoversHelpVersionAndFindings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "repo-config.yml"), "schema: rhino/repo-config/v2\npolicies:\n  conventions:\n    emoji:\n      prohibited:\n        - glob: \"**/*.json\"\n")
	writeFile(t, filepath.Join(dir, "config", "a.json"), "held \U0001F98F\n")

	if code, stdout, _ := runArgs(t, dir, []string{"--help"}); code != 0 || !strings.Contains(stdout, "Repository Hygiene") {
		t.Fatalf("help failed: %d %q", code, stdout)
	}
	if code, stdout, _ := runArgs(t, dir, []string{"version", "--json"}); code != 0 || !strings.Contains(stdout, "\"schemaVersion\":1") {
		t.Fatalf("version --json failed: %d %q", code, stdout)
	}
	if code, _, stderr := runArgs(t, dir, []string{"convention", "emoji", "validate"}); code != 1 || !strings.Contains(stderr, "U+1F98F") {
		t.Fatalf("emoji run failed: %d %q", code, stderr)
	}
	if code, _, stderr := runArgs(t, dir, []string{"nope"}); code != 2 || !strings.Contains(stderr, "unrecognized command") {
		t.Fatalf("unknown command failed: %d %q", code, stderr)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errBoom }

var errBoom = errorString("boom")

type errorString string

func (e errorString) Error() string { return string(e) }

func TestRunArgsReadsStdinOnlyWhenSelected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/repo-config.yml", "schema: rhino/repo-config/v2\n")
	t.Chdir(dir)
	app := cli.Application{Stdin: strings.NewReader("hello"), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	if code := app.RunArgs(context.Background(), []string{"md", "mermaid", "validate", "--file", "-"}); code != 2 {
		t.Fatalf("mermaid with stdin failed: %d", code)
	}
	bad := cli.Application{Stdin: failingReader{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	if code := bad.RunArgs(context.Background(), []string{"md", "mermaid", "validate", "--file", "-"}); code != 2 {
		t.Fatalf("unreadable stdin failed: %d", code)
	} else if !strings.Contains(bad.Stderr.(*bytes.Buffer).String(), "standard input could not be read") {
		t.Fatalf("stdin reason missing: %q", bad.Stderr.(*bytes.Buffer).String())
	}
}

func TestRunArgsRefusesMissingConfig(t *testing.T) {
	dir := t.TempDir()
	if code, _, stderr := runArgs(t, dir, []string{"repo-config", "validate"}); code != 2 || stderr == "" {
		t.Fatalf("missing config failed: %d %q", code, stderr)
	}
}

func TestRunArgsWritesDeclaredEnvironmentTargets(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "repo-config.yml"), "schema: rhino/repo-config/v2\nenvironment:\n  examples:\n    - source: shared.example\n      target: local.generated\n")
	writeFile(t, filepath.Join(dir, "shared.example"), "EXAMPLE_KEY=placeholder\n")
	t.Chdir(dir)
	run := func(args []string, wired bool) (int, string, string) {
		out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
		app := cli.Application{Stdin: strings.NewReader(""), Stdout: out, Stderr: errOut}
		if wired {
			app.Environment = wiredCliEnvironment()
		}
		return app.RunArgs(context.Background(), args), out.String(), errOut.String()
	}
	if code, stdout, _ := run([]string{"env", "init"}, false); code != 0 || !strings.Contains(stdout, "pass --apply to write") {
		t.Fatalf("init plan failed: %d %q", code, stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "local.generated")); err == nil {
		t.Fatal("plan wrote its target")
	}
	if code, _, stderr := run([]string{"env", "init", "--apply"}, false); code != 2 || !strings.Contains(stderr, "no environment write boundary") {
		t.Fatalf("unwired init failed: %d %q", code, stderr)
	}
	if code, stdout, _ := run([]string{"env", "init", "--apply"}, true); code != 0 || !strings.Contains(stdout, "created declared targets") {
		t.Fatalf("init apply failed: %d %q", code, stdout)
	}
	created, err := os.ReadFile(filepath.Join(dir, "local.generated"))
	if err != nil || string(created) != "EXAMPLE_KEY=placeholder\n" {
		t.Fatalf("init wrote wrong bytes: %q %v", created, err)
	}
	if code, stdout, _ := run([]string{"env", "validate"}, false); code != 0 || !strings.Contains(stdout, "no findings") {
		t.Fatalf("validate failed: %d %q", code, stdout)
	}
}

// `--root` names the repository, so a sibling directory outside the working
// directory is accepted and read as that repository. A selection inside the
// chosen root still refuses a path whose `..` climbs out of it.
func TestRunArgsRootNamesAnyExistingDirectory(t *testing.T) {
	parent := t.TempDir()
	working := filepath.Join(parent, "repo")
	writeFile(t, filepath.Join(working, "README.md"), "# Repo\n")
	writeFile(t, filepath.Join(parent, "other-repo", "repo-config.yml"), "schema: rhino/repo-config/v2\n")

	if code, _, stderr := runArgs(t, working, []string{"--root", "../other-repo", "repo-config", "validate"}); code != 0 {
		t.Fatalf("root outside the working directory was refused: %d %q", code, stderr)
	}
	if code, _, stderr := runArgs(t, working, []string{"repo-config", "validate", "--root", "../other-repo"}); code != 0 {
		t.Fatalf("trailing root outside the working directory was refused: %d %q", code, stderr)
	}
	if code, _, stderr := runArgs(t, working, []string{"--root", "../missing", "repo-config", "validate"}); code != 2 || !strings.Contains(stderr, "names nothing usable") {
		t.Fatalf("missing root was not refused as unusable: %d %q", code, stderr)
	}
	if code, _, stderr := runArgs(t, working, []string{"--root", "../other-repo", "md", "mermaid", "validate", "--file", "../escape.md"}); code != 2 || !strings.Contains(stderr, "escapes the repository root") {
		t.Fatalf("selection escaping the chosen root was not refused: %d %q", code, stderr)
	}
}

func TestRunArgsReportsCancelledRunAsInterrupted(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	app := cli.Application{Stdin: strings.NewReader(""), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	if code := app.RunArgs(cancelled, []string{"version"}); code != 130 {
		t.Fatalf("cancelled run exited %d, want 130", code)
	}
}
