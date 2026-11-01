package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// TestQualityGatesRefusesAMalformedField proves each declared quality-gate field
// of the wrong shape refuses the run with the reason its decoder names, so no
// audit runs against a partial policy.
func TestQualityGatesRefusesAMalformedField(t *testing.T) {
	cases := []struct {
		name   string
		config string
		reason string
	}{
		{"a root that is not a string", mutateQualityConfig("      root: flows\n", "      root: [1]\n"), "quality-gates.root names no workflow root"},
		{"an agent directory that is not a string", mutateQualityConfig("      agents: agents\n", "      agents: [1]\n"), "quality-gates.agents names no agent directory"},
		{"groups that are not a list", mutateQualityConfig("      groups: [plan, quality]\n", "      groups: x\n"), "quality-gates.groups names no workflow groups"},
		{"a gate group that is not a string", mutateQualityConfig("      gate-group: quality\n", "      gate-group: [1]\n"), "quality-gates.gate-group names no gate group"},
		{"gate headings that are not a list", mutateQualityConfig("      gate-headings: [Entry, Inputs, Verdict]\n", "      gate-headings: x\n"), "quality-gates.gate-headings names no gate headings"},
		{"a verdict heading that is not a string", mutateQualityConfig("      verdict-heading: Verdict\n", "      verdict-heading: [1]\n"), "quality-gates.verdict-heading names no verdict heading"},
		{"verdicts that are not a list", mutateQualityConfig("      verdicts: [PASS]\n", "      verdicts: x\n"), "quality-gates.verdicts names no gate verdicts"},
		{"retired inputs that are not a list", mutateQualityConfig("      retired-inputs: [max-audits]\n", "      retired-inputs: x\n"), "quality-gates.retired-inputs names no retired inputs"},
		{"a retired input that is not a string", mutateQualityConfig("      retired-inputs: [max-audits]\n", "      retired-inputs: [1]\n"), "quality-gates.retired-inputs names no retired inputs"},
		{"gate families that are not a list", mutateQualityConfig("      gates: [{family: plan}]\n", "      gates: x\n"), "quality-gates.gates names no gate families"},
		{"run defaults that are not a mapping", mutateQualityConfig("      gates: [{family: plan}]\n", "      defaults: x\n      gates: [{family: plan}]\n"), "quality-gates.defaults names no run defaults"},
		{"a run mode that names no gate mode", mutateQualityConfig("      gates: [{family: plan}]\n", "      defaults: {mode: nope}\n      gates: [{family: plan}]\n"), "quality-gates.defaults.mode names no gate mode"},
		{"a run default with an unknown key", mutateQualityConfig("      gates: [{family: plan}]\n", "      defaults: {bogus: 1}\n      gates: [{family: plan}]\n"), "quality-gates.defaults names unknown key `bogus`"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := conformingQualityTree()
			tree.files["repo-config.yml"] = tc.config
			outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
		})
	}
}

// TestQualityGatesAcceptsDeclaredRunDefaults proves a run default in the allowed
// mode and cycle range is read without refusing the otherwise conforming policy.
func TestQualityGatesAcceptsDeclaredRunDefaults(t *testing.T) {
	tree := conformingQualityTree()
	tree.files["repo-config.yml"] = mutateQualityConfig("      gates: [{family: plan}]\n",
		"      defaults: {mode: strict, max-cycles: 2}\n      gates: [{family: plan}]\n")
	outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("declared run defaults refused the conforming policy: %+v", outcome)
	}
}

// TestQualityGatesWordBudgetWithoutSurfacesGovernsNoFile proves the quality-gates
// audit reads a word budget that declares its count rule and no surfaces, and
// that such a budget governs no file, so the conforming policy stays clean.
func TestQualityGatesWordBudgetWithoutSurfacesGovernsNoFile(t *testing.T) {
	tree := conformingQualityTree()
	tree.files["repo-config.yml"] = qualityGateBase + "    word-budget:\n      count: whitespace-separated\n"
	outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("a budget with no surfaces refused the audit: %+v", outcome)
	}
}
