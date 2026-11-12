package vault_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/tests/bdd/vault"
)

// corpus is the environment vault feature, relative to this package.
const corpus = "../../../specs/behaviours/environment-vault-validation.feature"

// TestCorpusIsBoundByTheSharedRegistry is the compliance check: every step of
// every scenario in the corpus has exactly one binding, and every binding is
// used. A failure names each unbound scenario.
func TestCorpusIsBoundByTheSharedRegistry(t *testing.T) {
	problems, err := vault.Unbound(corpus, vault.Bindings())
	if err != nil {
		t.Fatal(err)
	}
	for _, problem := range problems {
		t.Error(problem)
	}
}

// writeFeature writes a feature file into a fresh directory and returns its path.
func writeFeature(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "defects.feature")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const twoScenarios = `Feature: Compliance fixture

  Scenario: Bound scenario
    When I run environment validation
    Then the command exits 0

  Scenario: Unbound scenario
    When I run environment validation
    Then the output names "mystery"
`

// boundOnly is the subset of the registry the fixtures need: the run step and
// the exit-code assertion. The output-naming assertion stays unbound, so the
// fixture that uses it has exactly one gap to report.
func boundOnly() []vault.Binding {
	needed := map[string]bool{
		`^I run environment validation$`: true,
		`^the command exits (\d+)$`:      true,
	}
	var subset []vault.Binding
	for _, binding := range vault.Bindings() {
		if needed[binding.Pattern] {
			subset = append(subset, binding)
		}
	}
	return subset
}

// TestUnboundNamesEachUnboundScenario proves the check fires on a scenario
// step no binding matches, and names that scenario.
func TestUnboundNamesEachUnboundScenario(t *testing.T) {
	problems, err := vault.Unbound(writeFeature(t, twoScenarios), boundOnly())
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], `scenario "Unbound scenario" has no binding`) {
		t.Fatalf("problems = %v, want one naming the unbound scenario", problems)
	}
}

// TestUnboundRejectsAnUnusedBinding proves a binding no scenario uses is named.
func TestUnboundRejectsAnUnusedBinding(t *testing.T) {
	bindings := append(boundOnly(), vault.Binding{Pattern: `^never used$`, Handler: func() {}})
	feature := "Feature: Fixture\n\n  Scenario: Bound\n    When I run environment validation\n    Then the command exits 0\n"
	problems, err := vault.Unbound(writeFeature(t, feature), bindings)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "used by no scenario") {
		t.Fatalf("problems = %v, want one naming the unused binding", problems)
	}
}

// TestUnboundRejectsAnAmbiguousStep proves a step matching two bindings is named.
func TestUnboundRejectsAnAmbiguousStep(t *testing.T) {
	bindings := append(boundOnly(), vault.Binding{Pattern: `^I run environment validation$`, Handler: func() {}})
	feature := "Feature: Fixture\n\n  Scenario: Ambiguous\n    When I run environment validation\n    Then the command exits 0\n"
	problems, err := vault.Unbound(writeFeature(t, feature), bindings)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, problem := range problems {
		found = found || strings.Contains(problem, "ambiguous step")
	}
	if !found {
		t.Fatalf("problems = %v, want one naming the ambiguous step", problems)
	}
}

// TestUnboundRefusesACorpusThatDoesNotParse proves a corpus that fails to parse
// is an error, never an empty problem list.
func TestUnboundRefusesACorpusThatDoesNotParse(t *testing.T) {
	if _, err := vault.Unbound(writeFeature(t, "this line is not Gherkin at all\n"), vault.Bindings()); err == nil {
		t.Fatal("a corpus that does not parse was accepted")
	}
}

// TestUnboundRefusesAnInvalidPattern proves a binding whose pattern does not
// compile is an error.
func TestUnboundRefusesAnInvalidPattern(t *testing.T) {
	bindings := []vault.Binding{{Pattern: "(", Handler: func() {}}}
	if _, err := vault.Unbound(corpus, bindings); err == nil {
		t.Fatal("an invalid binding pattern was accepted")
	}
}

// TestUnboundAcceptsACompleteCorpus proves a corpus whose every step is bound
// reports nothing.
func TestUnboundAcceptsACompleteCorpus(t *testing.T) {
	clean := "Feature: Fixture\n\n  Scenario: Bound\n    When I run environment validation\n    Then the command exits 0\n"
	problems, err := vault.Unbound(writeFeature(t, clean), boundOnly())
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("a complete corpus reported %v", problems)
	}
}
