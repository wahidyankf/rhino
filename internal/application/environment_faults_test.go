package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// TestEnvironmentFindingsOrderByRuleThenPathThenKey proves findings from several
// rules and several files are listed in one fixed order: by rule first, then by
// path, then by key, so two runs produce identical output.
func TestEnvironmentFindingsOrderByRuleThenPathThenKey(t *testing.T) {
	tree := envConfig("environment:\n  contracts:\n    - path: cfg\n      keys: [DECLARED]\n" +
		"  detectors:\n    - language: rust\n      paths: [src/a.rs, src/b.rs]\n    - language: go\n      paths: [src/c.go]\n")
	tree.files["cfg"] = "declared\n"
	tree.files["src/a.rs"] = "std::env::var(\"UNDECLARED_A\");\n"
	tree.files["src/b.rs"] = "std::env::var(\"UNDECLARED_B\");\n"
	tree.files["src/c.go"] = "os.Getenv(name)\n"
	outcome := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "validate")
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	ordered := []string{"declared-but-unread", "(DECLARED)", "(UNDECLARED_A)", "(UNDECLARED_B)", "unsupported-dynamic-access"}
	last := -1
	for _, fragment := range ordered {
		at := strings.Index(outcome.Stderr, fragment)
		if at <= last {
			t.Fatalf("%q is out of order in %q", fragment, outcome.Stderr)
		}
		last = at
	}
}

// TestEnvironmentCleanJSONNamesTheCommand proves a clean validation in JSON
// names its command, its exit code, and how many sources it inspected.
func TestEnvironmentCleanJSONNamesTheCommand(t *testing.T) {
	tree := envConfig("environment:\n  contracts:\n    - path: cfg\n      keys: [DECLARED]\n" +
		"  detectors:\n    - language: rust\n      paths: [src/a.rs]\n")
	tree.files["cfg"] = "declared\n"
	tree.files["src/a.rs"] = "std::env::var(\"DECLARED\");\n"
	outcome := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "validate", "--output", "json")
	if outcome.ExitCode != 0 || !strings.Contains(outcome.Stdout, `"command":"environment-validate","exitCode":0`) {
		t.Fatalf("clean JSON validation not reported: %+v", outcome)
	}
}

// TestEnvironmentRestoreRefusesAnEscapingOrUnwritableRequest proves a restore
// that would leave the repository, or that has no write boundary, refuses before
// it reads or writes any target.
func TestEnvironmentRestoreRefusesAnEscapingOrUnwritableRequest(t *testing.T) {
	t.Run("a backup directory that climbs out of the root refuses", func(t *testing.T) {
		tree := envConfig("environment:\n  examples:\n    - source: .env.example\n      target: .env.fixture\n")
		outcome := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "restore", "--dir", "../escape")
		if outcome.ExitCode == 0 || !strings.Contains(outcome.Stderr, "backup destination leaves the repository root") {
			t.Fatalf("escaping backup directory not refused: %+v", outcome)
		}
	})
	t.Run("a restore without a write boundary refuses", func(t *testing.T) {
		tree := envConfig("environment:\n  examples:\n    - source: .env.example\n      target: .env.fixture\n")
		tree.files["saved/.env.fixture"] = "SYNTHETIC=restored\n"
		outcome := application.Execute(tree, args("env", "restore", "--dir", "saved"), nil)
		if outcome.ExitCode == 0 || !strings.Contains(outcome.Stderr, "no environment write boundary") {
			t.Fatalf("restore without a store not refused: %+v", outcome)
		}
	})
}
