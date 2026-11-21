// Integration level: this file reaches the real host, so the default build omits it (see .golangci.yml).

//go:build integration

package application_test

import (
	"strings"
	"testing"
)

// TestEnvironmentFindingsOrderByRuleThenPathThenKey proves findings from several
// rules and several files are listed in one fixed order: by rule first, then by
// path, then by key, so two runs produce identical output.
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
