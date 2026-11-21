package e2e_test

import (
	"testing"

	"github.com/wahidyankf/rhino/tests/bdd"
)

// The end-to-end level executes every feature file of the corpus against the
// built binary, so each file's scenarios are covered at this level by the
// whole-file references below.
//
// @scenario specs/behaviours/behaviour-coverage.feature: *
// @scenario specs/behaviours/cli-surface.feature: *
// @scenario specs/behaviours/environment-vault-validation.feature: *
// @scenario specs/behaviours/environment-vault.feature: *
// @scenario specs/behaviours/quality-gates.feature: *
// @scenario specs/behaviours/regular-files.feature: *
// @scenario specs/behaviours/v0-4-contract.feature: *
// @scenario specs/behaviours/wiring.feature: *

// behaviourCorpus is the Gherkin corpus the end-to-end level executes, relative
// to this package directory.
const behaviourCorpus = "../../specs/behaviours"

// TestBehaviourCorpus runs every scenario of the corpus against the built rhino
// binary, one child process per rhino run, through the shared corpus runner.
func TestBehaviourCorpus(t *testing.T) {
	bdd.RunCorpus(t, behaviourCorpus, rhinoBinary, fakeBWBinary)
}
