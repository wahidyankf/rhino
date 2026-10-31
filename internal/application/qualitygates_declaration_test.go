package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// TestRepoConfigValidateRefusesQualityGateDeclarations proves repo-config
// validate applies the declaration rules a quality-gate structure check needs,
// and a valid declaration passes.
func TestRepoConfigValidateRefusesQualityGateDeclarations(t *testing.T) {
	declaration := func(root string) string {
		return "schema: rhino/repo-config/v2\npolicies:\n  governance:\n    quality-gates:\n      root: " + root +
			"\n      agents: agents\n      groups: [plan, quality]\n      gate-group: quality\n" +
			"      gate-headings: [Entry, Verdict]\n      verdict-heading: Verdict\n      verdicts: [PASS]\n" +
			"      propagation-headings: [Contract]\n      gates: [{family: plan}]\n"
	}
	refused := application.ExecuteWithLauncher(gateTree(declaration("../workflows")), args("repo-config", "validate"), nil, &stubLauncher{})
	if refused.ExitCode != 2 || !strings.Contains(refused.Stderr, "exact repository-relative paths") {
		t.Fatalf("escaping root not refused by repo-config validate: %+v", refused)
	}
	accepted := application.ExecuteWithLauncher(gateTree(declaration("workflows")), args("repo-config", "validate"), nil, &stubLauncher{})
	if accepted.ExitCode != 0 {
		t.Fatalf("valid declaration refused: %+v", accepted)
	}
}
