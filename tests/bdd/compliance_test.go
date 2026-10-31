package bdd_test

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/cucumber/godog"
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

// countFeatureFiles counts the .feature files on disk beneath path.
func countFeatureFiles(path string) (int, error) {
	count := 0
	err := filepath.WalkDir(path, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".feature") {
			count++
		}
		return nil
	})
	return count, err
}

// corpusProblems reports every way the corpus and the bindings disagree. A
// clean corpus returns nothing. The checks run before any scenario executes, so
// a broken corpus fails with its full list of problems.
func corpusProblems(parsed corpus, featureFiles int, bound []binding) []error {
	var problems []error
	if parsed.scenarios == 0 {
		problems = append(problems, errors.New("behaviour corpus has no scenarios"))
	}
	if parsed.features != featureFiles {
		problems = append(problems, fmt.Errorf("parsed %d of %d feature files", parsed.features, featureFiles))
	}

	patterns := make([]*regexp.Regexp, len(bound))
	used := make([]bool, len(bound))
	for i, b := range bound {
		pattern, err := regexp.Compile(b.pattern)
		if err != nil {
			problems = append(problems, fmt.Errorf("invalid binding %q: %w", b.pattern, err))
			continue
		}
		patterns[i] = pattern
	}

	for _, step := range parsed.steps {
		matches := 0
		for i, pattern := range patterns {
			if pattern != nil && pattern.MatchString(step) {
				matches++
				used[i] = true
			}
		}
		switch {
		case matches == 0:
			problems = append(problems, fmt.Errorf("undefined behaviour step %q", step))
		case matches > 1:
			problems = append(problems, fmt.Errorf("ambiguous behaviour step %q matches %d bindings", step, matches))
		}
	}

	for i, b := range bound {
		if !used[i] {
			problems = append(problems, fmt.Errorf("unused behaviour binding %q", b.pattern))
		}
	}
	return problems
}

// TestFeatureCompliance proves the corpus is complete and every step is bound
// exactly once. It never runs a scenario, so a failure here names the defect
// without a scenario failure hiding it.
func TestFeatureCompliance(t *testing.T) {
	parsed, err := loadCorpus(behavioursCorpus)
	if err != nil {
		t.Fatal(err)
	}
	featureFiles, err := countFeatureFiles(behavioursCorpus)
	if err != nil {
		t.Fatalf("walk behaviour corpus: %v", err)
	}
	for _, problem := range corpusProblems(parsed, featureFiles, bindings()) {
		t.Error(problem)
	}
}

// TestCorpusProblemsRejectDefects proves each rejection path fires, so the
// compliance check cannot pass by accident.
func TestCorpusProblemsRejectDefects(t *testing.T) {
	known := binding{pattern: `^known$`, handler: func() {}}
	other := binding{pattern: `^other$`, handler: func() {}}
	tests := []struct {
		name    string
		parsed  corpus
		files   int
		bound   []binding
		message string
	}{
		{"empty corpus", corpus{features: 1}, 1, []binding{known}, "no scenarios"},
		{"parse gap", corpus{features: 1, scenarios: 1, steps: []string{"known"}}, 2, []binding{known}, "parsed 1 of 2"},
		{"undefined step", corpus{features: 1, scenarios: 1, steps: []string{"unknown"}}, 1, []binding{known}, "undefined behaviour step"},
		{"ambiguous step", corpus{features: 1, scenarios: 1, steps: []string{"known"}}, 1, []binding{known, known}, "ambiguous behaviour step"},
		{"unused binding", corpus{features: 1, scenarios: 1, steps: []string{"known"}}, 1, []binding{known, other}, "unused behaviour binding"},
		{"invalid pattern", corpus{features: 1, scenarios: 1, steps: []string{"known"}}, 1, []binding{{pattern: "(", handler: func() {}}}, "invalid binding"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			problems := corpusProblems(test.parsed, test.files, test.bound)
			for _, problem := range problems {
				if strings.Contains(problem.Error(), test.message) {
					return
				}
			}
			t.Fatalf("expected a problem containing %q, got %v", test.message, problems)
		})
	}
}

// TestCorpusProblemsAcceptCleanCorpus proves a complete, bound corpus passes.
func TestCorpusProblemsAcceptCleanCorpus(t *testing.T) {
	clean := corpus{features: 2, scenarios: 2, steps: []string{"known", "other"}}
	bound := []binding{{pattern: `^known$`, handler: func() {}}, {pattern: `^other$`, handler: func() {}}}
	if problems := corpusProblems(clean, 2, bound); len(problems) != 0 {
		t.Fatalf("clean corpus reported %v", problems)
	}
}
