package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
	"github.com/wahidyankf/rhino/internal/domain"
)

type stubLauncher struct {
	code          int
	notFound      bool
	notExecutable bool
	refused       string
	seen          [][]string
	seenLaunch    []application.GateLaunch
}

func (s *stubLauncher) Launch(launch application.GateLaunch) application.Launched {
	s.seen = append(s.seen, launch.Argv)
	s.seenLaunch = append(s.seenLaunch, launch)
	return application.Launched{Code: s.code, NotFound: s.notFound, NotExecutable: s.notExecutable, Refused: s.refused}
}

func gateTree(text string) *memoryTree {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = text
	return tree
}

const gateConfig = "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      command:\n        executable: ./check\n      run-on:\n        pre-commit: {}\n        pull-request:\n          reason: replay check\n  composition:\n    pull-request:\n      relation: exact\n"

func TestGateRunPassesAndMapsFailures(t *testing.T) {
	tree := gateTree(gateConfig)
	launcher := &stubLauncher{}
	passed := application.ExecuteWithLauncher(tree, args("gate", "run", "--surface", "pre-commit"), nil, launcher)
	if passed.ExitCode != 0 || !strings.Contains(passed.Stdout, "[gate] check passed") {
		t.Fatalf("pass not reported: %+v", passed)
	}
	if len(launcher.seen) != 1 || launcher.seen[0][0] != "./check" {
		t.Fatalf("child not launched: %v", launcher.seen)
	}
	for _, tc := range []struct {
		name     string
		launcher *stubLauncher
		code     int
		want     string
	}{
		{"findings", &stubLauncher{code: 1}, 1, "[gate] check failed"},
		{"not found", &stubLauncher{notFound: true}, 127, "executable was not found"},
		{"not executable", &stubLauncher{notExecutable: true}, 126, "executable could not be executed"},
		{"refused", &stubLauncher{refused: "boom"}, 2, "gate `check`: boom"},
	} {
		outcome := application.ExecuteWithLauncher(tree, args("gate", "run", "--surface", "pre-commit"), nil, tc.launcher)
		if outcome.ExitCode != tc.code || !strings.Contains(outcome.Stdout+outcome.Stderr, "check") {
			t.Fatalf("%s: %+v", tc.name, outcome)
		}
		if tc.code != 1 && !strings.Contains(outcome.Stderr, tc.want) {
			t.Fatalf("%s stderr missing %q: %+v", tc.name, tc.want, outcome)
		}
	}
}

func TestGateRunJSONShapes(t *testing.T) {
	tree := gateTree(gateConfig)
	launcher := &stubLauncher{}
	outcome := application.ExecuteWithLauncher(tree, args("gate", "run", "--surface", "pre-commit", "--output", "json"), nil, launcher)
	if outcome.ExitCode != 0 || !strings.Contains(outcome.Stdout, "\"surface\":\"pre-commit\"") {
		t.Fatalf("gate run json: %+v", outcome)
	}
	failing := application.ExecuteWithLauncher(tree, args("gate", "run", "--surface", "pre-commit", "--output", "json"), nil, &stubLauncher{code: 3})
	if failing.ExitCode != 1 || !strings.Contains(failing.Stdout, "\"result\":\"findings\"") {
		t.Fatalf("gate findings json: %+v", failing)
	}
	missing := application.ExecuteWithLauncher(tree, args("gate", "run", "--surface", "pre-commit", "--output", "json"), nil, &stubLauncher{notFound: true})
	if missing.ExitCode != 127 || !strings.Contains(missing.Stderr, "rhino.gate.child-not-found") {
		t.Fatalf("gate 127 json: %+v", missing)
	}
	unexecutable := application.ExecuteWithLauncher(tree, args("gate", "run", "--surface", "pre-commit", "--output", "json"), nil, &stubLauncher{notExecutable: true})
	if unexecutable.ExitCode != 126 || !strings.Contains(unexecutable.Stderr, "\"schemaVersion\":1") || !strings.Contains(unexecutable.Stderr, "rhino.gate.child-not-executable") {
		t.Fatalf("gate 126 json: %+v", unexecutable)
	}
}

func TestGateRunRefusesBeforeDispatch(t *testing.T) {
	tree := gateTree(gateConfig)
	if outcome := application.Execute(tree, args("gate", "run"), nil); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "--surface") {
		t.Fatalf("missing surface: %+v", outcome)
	}
	if outcome := application.Execute(tree, args("gate", "run", "--surface", "nightly"), nil); outcome.ExitCode != 2 {
		t.Fatalf("unknown surface: %+v", outcome)
	}
	noCommand := gateTree("schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: nocmd\n      run-on:\n        pre-commit: {}\n        pull-request:\n          reason: x\n  composition:\n    pull-request:\n      relation: exact\n")
	if outcome := application.Execute(noCommand, args("gate", "run", "--surface", "pre-commit"), nil); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "declares no executable command") {
		t.Fatalf("missing command: %+v", outcome)
	}
	if outcome := application.Execute(tree, args("gate", "run", "--surface", "pre-commit"), nil); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "not wired") {
		t.Fatalf("nil launcher: %+v", outcome)
	}
}

func TestJSONReports(t *testing.T) {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = "schema: rhino/repo-config/v2\npolicies:\n  conventions:\n    emoji:\n      prohibited:\n        - glob: \"**/*.json\"\n"
	tree.files["c/a.json"] = "hi \u2713\n"
	outcome := application.Execute(tree, args("convention", "emoji", "validate", "--output", "json"), nil)
	if outcome.ExitCode != 1 || !strings.Contains(outcome.Stdout, "\"kind\":\"emoji-in-prohibited-file\"") || !strings.Contains(outcome.Stdout, "\"codePoint\":\"U+2713\"") {
		t.Fatalf("emoji json: %+v", outcome)
	}
	clean := application.Execute(tree, args("repo-config", "validate", "--output", "json"), nil)
	if clean.ExitCode != 0 || !strings.Contains(clean.Stdout, "\"inspected\":1") {
		t.Fatalf("repo-config json: %+v", clean)
	}
	list := application.Execute(gateTree(gateConfig), args("gate", "list", "--output", "json"), nil)
	if list.ExitCode != 0 || !strings.Contains(list.Stdout, "\"command\":[\"gate\",\"list\"]") {
		t.Fatalf("gate list json: %+v", list)
	}
	refused := application.Execute(tree, args("md", "naming", "validate", "--output", "json"), nil)
	if refused.ExitCode != 2 || !strings.Contains(refused.Stderr, "rhino.config.undeclared") {
		t.Fatalf("refusal json: %+v", refused)
	}
}

func TestMatchCharacterClasses(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"[abc].md", "a.md", true},
		{"[abc].md", "d.md", false},
		{"[a-z].md", "m.md", true},
		{"[!a-z].md", "5.md", true},
		{"[!a-z].md", "m.md", false},
		{"file?.md", "file1.md", true},
		{"*.md", "a.md", true},
		{"**", "a/b/c", true},
	}
	for _, tc := range cases {
		got, err := application.Match(tc.pattern, tc.path)
		if err != nil || got != tc.want {
			t.Fatalf("Match(%q,%q)=%v,%v want %v", tc.pattern, tc.path, got, err, tc.want)
		}
	}
	if _, err := application.Match("[abc", "a"); err == nil {
		t.Fatal("unterminated class was not an error")
	}
	selected, err := application.Select([]string{"b.md", "a.md"}, "k", []string{"*.md"})
	if err != nil || len(selected) != 2 || selected[0] != "a.md" {
		t.Fatalf("select not sorted: %v %v", selected, err)
	}
}

func TestStructuralFindingsSortByPosition(t *testing.T) {
	report := application.NewReport("metadata", "file")
	line2, line10 := 2, 10
	column := 3
	field := "title"
	report.Findings = append(report.Findings,
		application.Finding{Kind: "b", Path: "a.md", Line: &line10, Column: &column, Field: &field, Message: "second"},
		application.Finding{Kind: "a", Path: "a.md", Line: &line2, Column: &column, Field: &field, Message: "first"},
	)
	outcome := report.Render(application.Text)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	first := strings.Index(outcome.Stderr, "a.md:2:3")
	second := strings.Index(outcome.Stderr, "a.md:10:3")
	if first < 0 || second < 0 || first > second {
		t.Fatalf("structural order wrong: %q", outcome.Stderr)
	}
	if !strings.Contains(outcome.Stdout, "2 findings") {
		t.Fatalf("summary missing: %q", outcome.Stdout)
	}
	json := report.Render(application.JSON)
	if !strings.Contains(json.Stdout, "\"column\":3") || !strings.Contains(json.Stdout, "\"field\":\"title\"") {
		t.Fatalf("structural json: %q", json.Stdout)
	}
}

func TestStructuralFindingsBreakColumnAndFieldTies(t *testing.T) {
	report := application.NewReport("metadata", "file")
	line := 2
	column2, column10 := 2, 10
	fieldA, fieldB := "a", "b"
	report.Findings = append(report.Findings,
		application.Finding{Kind: "b", Path: "a.md", Line: &line, Column: &column10, Message: "wide"},
		application.Finding{Kind: "b", Path: "a.md", Line: &line, Column: &column2, Message: "narrow"},
		application.Finding{Kind: "b", Path: "a.md", Line: &line, Column: &column2, Field: &fieldB, Message: "second"},
		application.Finding{Kind: "b", Path: "a.md", Line: &line, Column: &column2, Field: &fieldA, Message: "first"},
	)
	outcome := report.Render(application.Text)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	// Numeric columns, not string order: column 2 reads before column 10,
	// and an absent field reads before any present one.
	first := strings.Index(outcome.Stderr, "first")
	second := strings.Index(outcome.Stderr, "second")
	narrow := strings.Index(outcome.Stderr, "narrow")
	wide := strings.Index(outcome.Stderr, "wide")
	if first < 0 || second < 0 || narrow < 0 || wide < 0 {
		t.Fatalf("findings missing: %q", outcome.Stderr)
	}
	if !(narrow < first && first < second && second < wide) {
		t.Fatalf("column and field ties out of order: %q", outcome.Stderr)
	}
	json := report.Render(application.JSON)
	jsonNarrow := strings.Index(json.Stdout, "narrow")
	jsonWide := strings.Index(json.Stdout, "wide")
	if jsonNarrow < 0 || jsonWide < 0 || jsonNarrow > jsonWide {
		t.Fatalf("json order wrong: %q", json.Stdout)
	}
}

func TestErrorCodesCovered(t *testing.T) {
	codes := []domain.ErrorCode{
		domain.ArgsUnrecognized, domain.ArgsIncomplete, domain.InputUnreadable,
		domain.RepositoryUnusable, domain.ConfigUnusable, domain.ConfigUndeclared,
		domain.PathEscapesRoot, domain.FileMissing, domain.FileUnreadable,
		domain.FileExists, domain.FileUnwritable, domain.GateChildNotFound,
		domain.GateChildNotExecutable, domain.GateChildRefused, domain.GateUndeclared,
		domain.ToolchainFailed, domain.ToolchainUndeclared, domain.HarnessRefused,
	}
	for _, code := range codes {
		if !strings.HasPrefix(code.String(), "rhino.") {
			t.Fatalf("code not namespaced: %v", code)
		}
	}
}
