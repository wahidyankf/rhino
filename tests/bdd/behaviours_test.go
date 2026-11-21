package bdd

import "testing"

// The suite executes every feature file of the corpus in strict mode, so each
// file's scenarios are covered at the integration level by the whole-file
// references below.
//
// @scenario specs/behaviours/behaviour-coverage.feature: *
// @scenario specs/behaviours/cli-surface.feature: *
// @scenario specs/behaviours/environment-vault-validation.feature: *
// @scenario specs/behaviours/environment-vault.feature: *
// @scenario specs/behaviours/quality-gates.feature: *
// @scenario specs/behaviours/regular-files.feature: *
// @scenario specs/behaviours/v0-4-contract.feature: *
// @scenario specs/behaviours/wiring.feature: *

// behavioursCorpus is the Gherkin corpus the suite executes, relative to this
// package directory. Every .feature file beneath it runs.
const behavioursCorpus = "../../specs/behaviours"

// TestBehaviours runs every scenario in the corpus against the built binary,
// through RunCorpus. Strict mode and the count check are RunCorpus's.
func TestBehaviours(t *testing.T) {
	RunCorpus(t, behavioursCorpus, rhinoBinary, fakeBWBinary)
}
