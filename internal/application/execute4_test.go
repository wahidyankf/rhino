package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
	"github.com/wahidyankf/rhino/internal/domain"
)

func TestParseValueAndScopeEdges(t *testing.T) {
	for _, cmd := range [][]string{
		{"--root"},
		{"--output"},
		{"--file"},
		{"--dir"},
		{"--surface"},
		{"--message-file"},
		{"--base", "a", "gate", "run"},
		{"--head", "b", "gate", "run"},
	} {
		if _, refusal := application.Parse(args(cmd...)); refusal == nil || refusal.Code != domain.ArgsIncomplete {
			t.Fatalf("%v: expected incomplete, got %+v", cmd, refusal)
		}
	}
	if _, refusal := application.Parse(args("--output", "yaml", "version")); refusal == nil {
		t.Fatal("bad output format was not refused")
	}
	if parsed, refusal := application.Parse(args("--quiet", "--verbose", "--no-color", "version")); refusal != nil || parsed.Run == nil {
		t.Fatalf("presentation flags refused: %+v", refusal)
	}
	if parsed, refusal := application.Parse(args("--", "version")); refusal != nil || parsed.Run.Category != "version" {
		t.Fatalf("separator form failed: %+v %+v", parsed.Run, refusal)
	}
	if parsed, refusal := application.Parse(args("-V")); refusal != nil || parsed.Run.Category != "version" {
		t.Fatalf("-V failed: %+v", refusal)
	}
	if parsed, refusal := application.Parse(args("gate", "list", "--directory", "x")); refusal == nil || !strings.Contains(refusal.Message, "not accepted") {
		t.Fatalf("directory on gate list: %+v %+v", parsed, refusal)
	}
	for _, cmd := range [][]string{
		{"governance", "directory-map", "validate", "--directory", "/abs"},
		{"governance", "directory-map", "validate", "--directory", "../up"},
		{"governance", "directory-map", "validate", "--directory", ""},
		{"md", "mermaid", "validate", "--file", "/abs"},
		{"md", "mermaid", "validate", "--file", "../up"},
	} {
		if _, refusal := application.Parse(args(cmd...)); refusal == nil || refusal.Code != domain.PathEscapesRoot {
			t.Fatalf("%v: expected escapes-root, got %+v", cmd, refusal)
		}
	}
	if _, refusal := application.Parse(args("gate", "run", "--surface", "pre-commit", "--message-file", "m")); refusal == nil {
		t.Fatal("message-file on pre-commit was not refused")
	}
	if _, refusal := application.Parse(args("gate", "run", "--surface", "commit-msg", "--push-updates-stdin")); refusal == nil {
		t.Fatal("push-updates on commit-msg was not refused")
	}
	if _, refusal := application.Parse(args("gate", "run", "--surface", "pre-commit", "--base", "a", "--head", "b")); refusal == nil {
		t.Fatal("range on pre-commit was not refused")
	}
	if _, refusal := application.Parse(args("nope", "--help")); refusal == nil || !strings.Contains(refusal.Message, "unrecognized command") {
		t.Fatalf("unknown help prefix: %+v", refusal)
	}
	if _, refusal := application.Parse(args("a", "b", "c", "d", "--help")); refusal == nil {
		t.Fatal("overlong help prefix was not refused")
	}
	if ok, err := application.Match("*[0-9]", "a5"); err != nil || !ok {
		t.Fatalf("star class failed: %v %v", ok, err)
	}
	if ok, err := application.Match("*[0-9]", "ab"); err != nil || ok {
		t.Fatalf("star class backtrack failed: %v %v", ok, err)
	}
	if parsed, refusal := application.Parse(args("md", "--help")); refusal != nil || parsed.Help == nil || !strings.Contains(parsed.Help.Text, "md frontmatter") {
		t.Fatalf("prefix help failed: %+v %+v", parsed.Help, refusal)
	}
}

func TestExecuteRootSurfaceAndJSONValidate(t *testing.T) {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = "schema: rhino/repo-config/v2\ngates: {}\n"
	if outcome := application.Execute(tree, args("gate", "list", "--root", "missing"), nil); outcome.ExitCode != 2 {
		t.Fatalf("bad root passed: %+v", outcome)
	}
	versioned := application.Execute(tree, args("version"), nil)
	if versioned.ExitCode != 0 || !strings.HasPrefix(versioned.Stdout, "v") {
		t.Fatalf("version text: %+v", versioned)
	}
	validateJSON := application.Execute(tree, args("gate", "validate", "--output", "json"), nil)
	if validateJSON.ExitCode != 0 || !strings.Contains(validateJSON.Stdout, "gate\",\"validate") {
		t.Fatalf("gate validate json: %+v", validateJSON)
	}
	withStdin := "records"
	if outcome := application.ExecuteWithLauncher(tree, args("gate", "list"), &withStdin, &stubLauncher{}); outcome.ExitCode != 0 {
		t.Fatalf("stdin-passing list failed: %+v", outcome)
	}
	if _, refusal := application.Parse(args()); refusal == nil || !strings.Contains(refusal.Message, "no command given") {
		t.Fatalf("empty invocation: %+v", refusal)
	}
	if outcome := application.Execute(tree, args("gate", "validate"), nil); outcome.ExitCode != 0 {
		t.Fatalf("gate validate text: %+v", outcome)
	}
	empty := newMemoryTree("/root")
	empty.files["repo-config.yml"] = "schema: rhino/repo-config/v2\n"
	if outcome := application.Execute(empty, args("gate", "validate"), nil); outcome.ExitCode != 2 {
		t.Fatalf("undeclared gate validate passed: %+v", outcome)
	}
	for _, text := range []string{
		"schema: rhino/repo-config/v2\npolicies:\n  conventions:\n    emoji:\n      prohibited: {}\n",
		"schema: rhino/repo-config/v2\npolicies:\n  conventions:\n    emoji:\n      prohibited:\n        - glob: \"\"\n",
		"schema: rhino/repo-config/v2\npolicies:\n  conventions:\n    emoji:\n      prohibited:\n        - noglob: x\n",
	} {
		broken := newMemoryTree("/root")
		broken.files["repo-config.yml"] = text
		if outcome := application.Execute(broken, args("convention", "emoji", "validate"), nil); outcome.ExitCode != 2 {
			t.Fatalf("malformed emoji policy passed: %+v", outcome)
		}
	}
	missingFormat := application.Execute(tree, args("--output", "json", "nope"), nil)
	if missingFormat.ExitCode != 2 || !strings.Contains(missingFormat.Stderr, "schemaVersion") {
		t.Fatalf("pre-format refusal not json: %+v", missingFormat)
	}
}
