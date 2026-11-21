package bdd

import (
	"context"
	"fmt"
	"testing"

	"github.com/cucumber/godog"
	"github.com/wahidyankf/rhino/tests/bdd/vault"
)

// corpus is the parsed form of a Gherkin corpus: how many feature files parsed,
// how many scenarios the pickles hold, and every step text they use.
type corpus struct {
	features  int
	scenarios int
	steps     []string
}

// loadCorpus parses every feature file beneath path, expanding outlines into
// one pickle per example row, so the scenario count is the count that runs.
func loadCorpus(path string) (corpus, error) {
	features, err := (godog.TestSuite{Options: &godog.Options{Paths: []string{path}, Strict: true}}).RetrieveFeatures()
	if err != nil {
		return corpus{}, fmt.Errorf("parse behaviour corpus: %w", err)
	}
	result := corpus{features: len(features)}
	for _, feature := range features {
		for _, pickle := range feature.Pickles {
			result.scenarios++
			for _, step := range pickle.Steps {
				result.steps = append(result.steps, step.Text)
			}
		}
	}
	return result, nil
}

// corpusScenarioCount returns how many scenarios the corpus holds, failing the
// test when the corpus does not parse.
func corpusScenarioCount(t *testing.T, path string) int {
	t.Helper()
	parsed, err := loadCorpus(path)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.scenarios
}

// RunCorpus executes every scenario of the Gherkin corpus at path in strict
// mode. Each scenario gets its own world under a temporary directory, and its
// steps drive the rhino executable at binary as a child process, with the fake
// bw at fakeBW for vault scenarios. The run fails on any undefined, ambiguous,
// or pending step, and when the executed count differs from the corpus count,
// so a scenario cannot disappear without failing the run.
//
// The integration suite and the end-to-end suite both call RunCorpus. They
// differ in which test file holds the call and in the binary they build.
func RunCorpus(t *testing.T, path, binary, fakeBW string) {
	t.Helper()
	expected := corpusScenarioCount(t, path)
	parent := t.TempDir()
	executed := 0

	initialize := func(scenarios *godog.ScenarioContext) {
		scenarios.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
			w, err := newWorld(parent, binary, fakeBW)
			if err != nil {
				return ctx, err
			}
			executed++
			return vault.WithDriver(withWorld(ctx, w), &cliVaultDriver{world: w}), nil
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
			Paths:    []string{path},
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
