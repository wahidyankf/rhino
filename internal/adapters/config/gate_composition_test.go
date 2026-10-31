package config_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/config"
)

// TestGateCompositionRefusesPullRequestExtras proves an exact composition refuses
// a gate that runs only on pull-request, and an at-least composition accepts it
// only with its own reason.
func TestGateCompositionRefusesPullRequestExtras(t *testing.T) {
	const template = "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: local-check\n      type: check\n      run-on:\n        pre-commit: {}\n        pull-request: {}\n    - id: replay-only\n      type: check\n      run-on:\n        pull-request: %s\n  composition:\n    pull-request:\n      relation: %s\n"
	tests := []struct {
		name       string
		relation   string
		membership string
		want       string
	}{
		{"exact refuses a pull-request extra", "exact", "{}", "under the exact composition relation"},
		{"at-least refuses an extra without a reason", "at-least", "{}", "needs its own non-empty reason"},
		{"at-least accepts an extra with its reason", "at-least", "{reason: replay}", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			text := strings.Replace(strings.Replace(template, "%s", test.membership, 1), "%s", test.relation, 1)
			_, err := config.Parse(text)
			switch {
			case test.want == "" && err != nil:
				t.Fatalf("expected acceptance, got %v", err)
			case test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)):
				t.Fatalf("got %v, want refusal %q", err, test.want)
			}
		})
	}
}

// TestGateCompositionRequiresLocalSurfacesOnly proves the pull-request
// composition rule applies to the local surfaces the oracle names, pre-commit,
// commit-msg, and pre-push, and never to a gate that runs only on manual,
// main, or scheduled. The oracle's LOCAL set is exactly those three surfaces.
func TestGateCompositionRequiresLocalSurfacesOnly(t *testing.T) {
	const refusal = "must declare the pull-request relation when local or pull-request gates exist"
	tests := []struct {
		name    string
		surface string
		want    string
	}{
		{"manual-only gate needs no composition", "manual", ""},
		{"main-only gate needs no composition", "main", ""},
		{"scheduled-only gate needs no composition", "scheduled", ""},
		{"pre-commit gate needs composition", "pre-commit", refusal},
		{"commit-msg gate needs composition", "commit-msg", refusal},
		{"pre-push gate needs composition", "pre-push", refusal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			text := "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: only-here\n      type: check\n      run-on:\n        " + test.surface + ": {}\n"
			_, err := config.Parse(text)
			switch {
			case test.want == "" && err != nil:
				t.Fatalf("%s gate was refused without composition: %v", test.surface, err)
			case test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)):
				t.Fatalf("%s gate: got %v, want refusal %q", test.surface, err, test.want)
			}
		})
	}
}
