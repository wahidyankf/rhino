package bdd_test

import (
	"context"
	"testing"

	"github.com/cucumber/godog"
)

// behavioursCorpus is the Gherkin corpus the suite executes, relative to this
// package directory. Every .feature file beneath it runs.
const behavioursCorpus = "../../specs/behaviours"

// TestBehaviours runs every scenario in the corpus against the built binary.
// Strict mode turns an undefined, ambiguous, or pending step into a failure.
// The run also has to execute exactly as many scenarios as the parsed corpus
// holds, so a scenario cannot disappear without failing the suite.
func TestBehaviours(t *testing.T) {
	expected := corpusScenarioCount(t, behavioursCorpus)
	parent := t.TempDir()
	executed := 0

	initialize := func(scenarios *godog.ScenarioContext) {
		scenarios.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
			w, err := newWorld(parent, rhinoBinary)
			if err != nil {
				return ctx, err
			}
			executed++
			return withWorld(ctx, w), nil
		})
		scenarios.After(func(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
			w, err := worldFrom(ctx)
			if err != nil {
				return ctx, err
			}
			return ctx, w.close()
		})
		for _, binding := range bindings() {
			scenarios.Step(binding.pattern, binding.handler)
		}
	}

	status := godog.TestSuite{
		Name:                "rhino-behaviours",
		ScenarioInitializer: initialize,
		Options: &godog.Options{
			Format:   "progress",
			Paths:    []string{behavioursCorpus},
			Strict:   true,
			NoColors: true,
			TestingT: t,
		},
	}.Run()
	t.Logf("executed %d scenarios of %d in the corpus", executed, expected)
	if status != 0 {
		t.Fatalf("behaviour suite exited %d", status)
	}
	if executed != expected {
		t.Fatalf("executed %d scenarios; the corpus holds %d", executed, expected)
	}
}
