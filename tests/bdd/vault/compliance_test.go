package vault_test

import (
	"testing"

	"github.com/wahidyankf/rhino/tests/bdd/vault"
)

// corpora are the feature files the shared registry serves, relative to this
// package. Every binding must be used by a scenario of at least one of them.
var corpora = []string{
	"../../../specs/behaviours/environment-vault-validation.feature",
	"../../../specs/behaviours/environment-vault.feature",
}

// TestEveryBindingServesACorpus fails when the shared registry holds a step no
// scenario of either corpus uses, naming it.
func TestEveryBindingServesACorpus(t *testing.T) {
	problems, err := vault.Unused(corpora, vault.Bindings())
	if err != nil {
		t.Fatal(err)
	}
	for _, problem := range problems {
		t.Error(problem)
	}
}

// TestUnusedRejectsABindingNoCorpusUses proves the check fires, using a
// registry with one step no corpus uses.
func TestUnusedRejectsABindingNoCorpusUses(t *testing.T) {
	bindings := []vault.Binding{
		{Pattern: `^the command exits (\d+)$`, Handler: nil},
		{Pattern: `^a step no scenario writes$`, Handler: nil},
	}
	problems, err := vault.Unused(corpora, bindings)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 {
		t.Fatalf("problems = %v, want one unused binding", problems)
	}
}
