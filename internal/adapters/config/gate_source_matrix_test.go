package config_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/config"
)

// TestGateSourceMatrixMatchesTheOracle proves every input kind accepts exactly
// the sources the oracle's source table names, and refuses the rest as not a
// compatible source. Each pair is one declaration with its own projection.
func TestGateSourceMatrixMatchesTheOracle(t *testing.T) {
	type kindSpec struct {
		kind, field, expand string
	}
	kinds := []kindSpec{
		{"files", "paths", "repeat"},
		{"regular-files", "paths", "repeat"},
		{"commit-message", "text", "single"},
		{"commit-range", "base", "single"},
		{"repository-state", "root", "single"},
	}
	sources := []string{"git-index", "hook-message-file", "push-updates", "explicit-range", "checkout"}
	accepts := map[string][]string{
		"files":            {"git-index", "explicit-range", "checkout"},
		"regular-files":    {"git-index", "explicit-range"},
		"commit-message":   {"hook-message-file", "explicit-range"},
		"commit-range":     {"push-updates", "explicit-range"},
		"repository-state": {"checkout"},
	}
	for _, spec := range kinds {
		for _, source := range sources {
			t.Run(spec.kind+"/"+source, func(t *testing.T) {
				binding := "source: " + source
				if source == "explicit-range" {
					binding += ", range: explicit"
				}
				if source == "push-updates" {
					binding += ", fallback: refs/heads/main"
				}
				text := fmt.Sprintf("schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: typed-gate\n      type: check\n"+
					"      inputs:\n        item: { kind: %s }\n      command:\n        executable: ./rhino\n"+
					"        args:\n          - { input: item.%s, expand: %s }\n"+
					"      run-on:\n        manual:\n          bind:\n            item: { %s }\n",
					spec.kind, spec.field, spec.expand, binding)
				_, err := config.Parse(text)
				want := contains(accepts[spec.kind], source)
				switch {
				case want && err != nil:
					t.Fatalf("oracle accepts %s from %s, refused: %v", spec.kind, source, err)
				case !want && (err == nil || !strings.Contains(err.Error(), "not a compatible source")):
					t.Fatalf("oracle refuses %s from %s as not a compatible source, got %v", spec.kind, source, err)
				}
			})
		}
	}
}

// contains reports whether a list holds a value.
func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
