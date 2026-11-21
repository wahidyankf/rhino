package cli

import (
	"context"
	"fmt"
	"testing"

	"github.com/cucumber/godog"
)

// The unit corpus runner executes every scenario of one feature file in process,
// strictly, against the in-memory world. Each scenario gets its own world, so no
// scenario sees another's repository, Git state, or child fixtures. The run fails
// on any undefined, ambiguous, or pending step, and when the executed count differs
// from the corpus count, so no scenario can disappear unnoticed.
//
// The feature text is the one file this level reads, as the vault unit runners
// already read theirs: the corpus is the statement being proved, and nothing the
// scenario acts on is read from disk.

// runUnitFeature runs every scenario of the feature file at feature.
func runUnitFeature(t *testing.T, feature string) {
	t.Helper()
	expected, err := unitScenarioCount(feature)
	if err != nil {
		t.Fatal(err)
	}
	executed := 0
	initialize := func(scenarios *godog.ScenarioContext) {
		scenarios.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
			executed++
			return context.WithValue(ctx, unitWorldKey{}, newUnitWorld()), nil
		})
		for _, binding := range unitBindings() {
			scenarios.Step(binding.pattern, binding.handler)
		}
	}
	status := godog.TestSuite{
		Name:                "rhino-unit-behaviours",
		ScenarioInitializer: initialize,
		Options: &godog.Options{
			Format:   "progress",
			Paths:    []string{feature},
			Strict:   true,
			NoColors: true,
			TestingT: t,
		},
	}.Run()
	t.Logf("executed %d scenarios of %d in the corpus", executed, expected)
	if status != 0 {
		t.Fatalf("unit behaviour suite exited %d", status)
	}
	if executed != expected {
		t.Fatalf("executed %d scenarios; the corpus holds %d", executed, expected)
	}
}

// unitScenarioCount parses one feature file and counts its pickles, one per
// outline row, which is the count the run executes.
func unitScenarioCount(feature string) (int, error) {
	features, err := (godog.TestSuite{Options: &godog.Options{Paths: []string{feature}, Strict: true}}).RetrieveFeatures()
	if err != nil {
		return 0, fmt.Errorf("parse behaviour corpus: %w", err)
	}
	count := 0
	for _, parsed := range features {
		count += len(parsed.Pickles)
	}
	return count, nil
}

// TestRegularFilesAtUnitLevel runs every scenario of the regular-files feature in
// process against the in-memory repository and Git state.
//
// @scenario specs/behaviours/regular-files.feature: *
func TestRegularFilesAtUnitLevel(t *testing.T) {
	runUnitFeature(t, "../../../specs/behaviours/regular-files.feature")
}

// TestWiringAtUnitLevel runs every scenario of the wiring feature in process.
//
// @scenario specs/behaviours/wiring.feature: *
func TestWiringAtUnitLevel(t *testing.T) {
	runUnitFeature(t, "../../../specs/behaviours/wiring.feature")
}

// TestQualityGatesAtUnitLevel runs every scenario of the quality-gates feature in
// process.
//
// @scenario specs/behaviours/quality-gates.feature: *
func TestQualityGatesAtUnitLevel(t *testing.T) {
	runUnitFeature(t, "../../../specs/behaviours/quality-gates.feature")
}

// TestCLISurfaceAtUnitLevel runs every scenario of the cli-surface feature in
// process.
//
// @scenario specs/behaviours/cli-surface.feature: *
func TestCLISurfaceAtUnitLevel(t *testing.T) {
	runUnitFeature(t, "../../../specs/behaviours/cli-surface.feature")
}

// TestV04ContractAtUnitLevel runs every scenario of the v0-4-contract feature in
// process.
//
// @scenario specs/behaviours/v0-4-contract.feature: *
func TestV04ContractAtUnitLevel(t *testing.T) {
	runUnitFeature(t, "../../../specs/behaviours/v0-4-contract.feature")
}

// TestBehaviourCoverageAtUnitLevel runs every scenario of the behaviour-coverage
// feature in process.
//
// @scenario specs/behaviours/behaviour-coverage.feature: *
func TestBehaviourCoverageAtUnitLevel(t *testing.T) {
	runUnitFeature(t, "../../../specs/behaviours/behaviour-coverage.feature")
}
