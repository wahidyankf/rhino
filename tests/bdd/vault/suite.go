package vault

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/cucumber/godog"
)

// Unbound reports every step of the corpus at path that no binding matches, or
// that more than one binding matches. Each problem names its scenario and step.
// The check parses the corpus and never runs a scenario. A binding that no
// scenario of this corpus uses is not a problem here, because the registry
// serves every corpus; Unused checks that across all of them.
func Unbound(path string, bindings []Binding) ([]string, error) {
	pickles, err := parse(path)
	if err != nil {
		return nil, err
	}
	var problems []string
	patterns := make([]*regexp.Regexp, len(bindings))
	for i, binding := range bindings {
		pattern, err := regexp.Compile(binding.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid binding %q: %w", binding.Pattern, err)
		}
		patterns[i] = pattern
	}
	for _, pickle := range pickles {
		for _, step := range pickle.steps {
			matches := 0
			for _, pattern := range patterns {
				if pattern.MatchString(step) {
					matches++
				}
			}
			switch {
			case matches == 0:
				problems = append(problems, fmt.Sprintf("scenario %q has no binding for step %q", pickle.name, step))
			case matches > 1:
				problems = append(problems, fmt.Sprintf("scenario %q has an ambiguous step %q", pickle.name, step))
			}
		}
	}
	return problems, nil
}

// Unused returns every binding that no step of any corpus in paths uses. A
// binding no scenario uses reports nothing, which looks exactly like passing.
func Unused(paths []string, bindings []Binding) ([]string, error) {
	used := make([]bool, len(bindings))
	for _, path := range paths {
		pickles, err := parse(path)
		if err != nil {
			return nil, err
		}
		for _, pickle := range pickles {
			for _, step := range pickle.steps {
				for i, binding := range bindings {
					if regexp.MustCompile(binding.Pattern).MatchString(step) {
						used[i] = true
					}
				}
			}
		}
	}
	var problems []string
	for i, binding := range bindings {
		if !used[i] {
			problems = append(problems, fmt.Sprintf("binding %q is used by no scenario", binding.Pattern))
		}
	}
	return problems, nil
}

// RunSuite is the layer runner an adapter calls from its own test. It first
// fails on every scenario the shared bindings leave unbound, naming it, and
// then runs each scenario under strict mode against the driver that open
// builds for it. A skipped or missing scenario fails the run.
func RunSuite(t *testing.T, path string, open func() (Driver, error)) {
	t.Helper()
	problems, err := Unbound(path, Bindings())
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) > 0 {
		for _, problem := range problems {
			t.Error(problem)
		}
		return
	}
	pickles, err := parse(path)
	if err != nil {
		t.Fatal(err)
	}
	expected := len(pickles)
	executed := 0
	status := godog.TestSuite{
		Name: "environment-vault-validation",
		ScenarioInitializer: func(scenarios *godog.ScenarioContext) {
			scenarios.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
				driver, err := open()
				if err != nil {
					return ctx, err
				}
				executed++
				return WithDriver(ctx, driver), nil
			})
			scenarios.After(func(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
				current, err := scenarioFrom(ctx)
				if err != nil {
					return ctx, err
				}
				return ctx, current.driver.Close()
			})
			for _, binding := range Bindings() {
				scenarios.Step(binding.Pattern, binding.Handler)
			}
		},
		Options: &godog.Options{
			Format:   "progress",
			Paths:    []string{path},
			Strict:   true,
			NoColors: true,
			TestingT: t,
		},
	}.Run()
	if status != 0 {
		t.Fatalf("vault suite exited %d", status)
	}
	if executed != expected {
		t.Fatalf("executed %d scenarios; the corpus holds %d", executed, expected)
	}
}

// pickle is one executable scenario of the corpus: its name and its step texts,
// with every outline example expanded.
type pickle struct {
	name  string
	steps []string
}

// parse reads the feature file at path without running it, and flattens its
// scenarios into pickles.
func parse(path string) ([]pickle, error) {
	features, err := (godog.TestSuite{Options: &godog.Options{Paths: []string{path}, Strict: true}}).RetrieveFeatures()
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	var pickles []pickle
	for _, feature := range features {
		for _, scenario := range feature.Pickles {
			current := pickle{name: scenario.Name}
			for _, step := range scenario.Steps {
				current.steps = append(current.steps, step.Text)
			}
			pickles = append(pickles, current)
		}
	}
	return pickles, nil
}
