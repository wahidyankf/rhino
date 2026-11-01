package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// TestQualityGatesRefusesAMalformedPropagationHeadingList proves a propagation
// heading list that is not a list of names refuses the run, naming the field.
func TestQualityGatesRefusesAMalformedPropagationHeadingList(t *testing.T) {
	tree := conformingQualityTree()
	tree.files["repo-config.yml"] = mutateQualityConfig("      propagation-headings: [Contract, Scope]\n", "      propagation-headings: x\n")
	outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, "quality-gates.propagation-headings names no propagation headings") {
		t.Fatalf("malformed propagation headings not refused: %+v", outcome)
	}
}

// TestQualityGatesRefusesAMaxCyclesOutsideItsRange proves a run default whose
// cycle ceiling is above three refuses, because a gate never runs more than three
// cycles.
func TestQualityGatesRefusesAMaxCyclesOutsideItsRange(t *testing.T) {
	tree := conformingQualityTree()
	tree.files["repo-config.yml"] = mutateQualityConfig("      gates: [{family: plan}]\n",
		"      defaults: {max-cycles: 4}\n      gates: [{family: plan}]\n")
	outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, "defaults.max-cycles: must be 1, 2, or 3") {
		t.Fatalf("out-of-range max-cycles not refused: %+v", outcome)
	}
}

// TestQualityGatesRefusesAWordBudgetSurfaceWithoutAGlob proves a budget surface
// that names no glob string refuses the run rather than governing a file by
// accident.
func TestQualityGatesRefusesAWordBudgetSurfaceWithoutAGlob(t *testing.T) {
	tree := conformingQualityTree()
	tree.files["repo-config.yml"] = qualityGateBase + "    word-budget:\n      count: whitespace-separated\n      surfaces: [{glob: 1, fail: 3}]\n"
	outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, "word-budget.surfaces names no budget surfaces") {
		t.Fatalf("a budget surface without a glob was not refused: %+v", outcome)
	}
}

// TestQualityGatesWithNoDeclaredFamilyFlagsEveryGateFile proves an omitted gates
// list declares no family, so a gate file in the gate group is reported as one
// no declaration names.
func TestQualityGatesWithNoDeclaredFamilyFlagsEveryGateFile(t *testing.T) {
	tree := conformingQualityTree()
	tree.files["repo-config.yml"] = mutateQualityConfig("      gates: [{family: plan}]\n", "")
	outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
	if outcome.ExitCode != 1 || !strings.Contains(outcome.Stdout+outcome.Stderr, "flows/quality/plan-quality-gate.md: the gate family is not declared") {
		t.Fatalf("an undeclared gate file was not flagged: %+v", outcome)
	}
}

// TestQualityGatesChecksEachDeclaredFamilyInOrder proves two declared families,
// one of which has lost its propagation file, report that file by name, so every
// family is checked, not only the first.
func TestQualityGatesChecksEachDeclaredFamilyInOrder(t *testing.T) {
	tree := conformingQualityTree()
	tree.files["repo-config.yml"] = mutateQualityConfig("      gates: [{family: plan}]\n", "      gates: [{family: plan}, {family: alpha}]\n")
	tree.files["flows/quality/alpha-quality-gate.md"] = qualityGateFile
	tree.files["agents/alpha-checker.md"] = "# Checker\n"
	tree.files["agents/alpha-fixer.md"] = "# Fixer\n"
	outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
	if outcome.ExitCode != 1 || !strings.Contains(outcome.Stdout+outcome.Stderr, "flows/quality/alpha-propagation.md: a declared gate family has no file here") {
		t.Fatalf("a second family's missing propagation file was not reported: %+v", outcome)
	}
}

// TestQualityGatesGateWithoutAVerdictHeadingIsReportedOnlyForItsHeading proves a
// gate file with no verdict heading has its absence reported as a missing gate
// heading, and no verdict is read from it.
func TestQualityGatesGateWithoutAVerdictHeadingIsReportedOnlyForItsHeading(t *testing.T) {
	tree := conformingQualityTree()
	tree.files["flows/quality/plan-quality-gate.md"] = strings.Replace(qualityGateFile, "## Verdict\n\n`PASS`\n", "", 1)
	outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
	if outcome.ExitCode != 1 || !strings.Contains(outcome.Stdout+outcome.Stderr, "flows/quality/plan-quality-gate.md: a required gate heading is absent (heading=Verdict") {
		t.Fatalf("a gate with no verdict heading was not reported: %+v", outcome)
	}
}

// TestQualityGatesReportsAModuleFileThatCannotBeRead proves a workflow module,
// read once the budget decides whether the workflow should split, refuses the run
// when it cannot be read.
func TestQualityGatesReportsAModuleFileThatCannotBeRead(t *testing.T) {
	tree := conformingQualityTree()
	tree.files["flows/plan/run.md"] = "# Run\n"
	tree.files["flows/plan/run/README.md"] = "- [One](001-one.md)\n"
	tree.files["flows/plan/run/001-one.md"] = "# One\n"
	tree.unreadable["flows/plan/run/001-one.md"] = true
	withBudgetTree(tree, "flows/plan/run.md", 1)
	outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, "flows/plan/run/001-one.md") {
		t.Fatalf("an unreadable module was not refused: %+v", outcome)
	}
}
