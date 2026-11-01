package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// TestTraceabilityRefusesAnUnshapedEntry proves an artifact or relationship
// that is not a mapping, or that names a key the decoder does not read, refuses
// the run with the reason for its own list, so no partial graph is checked.
func TestTraceabilityRefusesAnUnshapedEntry(t *testing.T) {
	cases := []struct {
		name   string
		config string
		reason string
	}{
		{
			name:   "an artifact that is a bare scalar fails the artifact check",
			config: traceabilityBase + "      artifacts:\n        - just-a-string\n",
			reason: "traceability artifacts require unique simple identifiers and exact repository-relative paths",
		},
		{
			name:   "an artifact with an unknown key refuses by name",
			config: traceabilityBase + "      artifacts:\n        - id: source\n          path: docs/a.md\n          bogus: 1\n",
			reason: "traceability.artifacts name unknown key `bogus`",
		},
		{
			name: "a relationship that is a bare scalar names no declared artifact",
			config: traceabilityBase + "      artifacts:\n        - id: source\n          path: docs/a.md\n" +
				"      relationships:\n        - just-a-string\n",
			reason: "every traceability relationship must name declared artifacts",
		},
		{
			name:   "a relationship with an unknown key refuses by name",
			config: traceabilityLinked + "        - from: source\n          to: target\n          bogus: 1\n",
			reason: "traceability.relationships name unknown key `bogus`",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newMemoryTree("/root")
			tree.files["repo-config.yml"] = tc.config
			tree.files["docs/a.md"] = "# Source\n"
			tree.files["docs/b.md"] = "# Target\n"
			outcome := application.Execute(tree, args("governance", "traceability", "validate"), nil)
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
		})
	}
}
