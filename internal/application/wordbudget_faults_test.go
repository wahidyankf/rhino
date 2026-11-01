package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// TestWordBudgetRefusesABrokenScanOrAnEscapingSurface proves a word-budget run
// refuses rather than reporting a clean count when its exclusion list is
// malformed, or when a declared surface reaches a link that leaves the root.
func TestWordBudgetRefusesABrokenScanOrAnEscapingSurface(t *testing.T) {
	cases := []struct {
		name   string
		config string
		links  []application.TreeLink
		reason string
	}{
		{
			name:   "a scan section that is not a mapping refuses the exclusion list",
			config: wordBudgetBase + "      count: letters-and-digits\n      surfaces:\n        - glob: \"docs/**/*.md\"\n          fail: 1\nscan: 3\n",
			reason: "scan.exclude-directories names no directory",
		},
		{
			name:   "a surface that reaches a link leaving the root refuses as an escape",
			config: wordBudgetBase + "      count: letters-and-digits\n      surfaces:\n        - glob: \"linked/**/*.md\"\n          fail: 1\n",
			links:  []application.TreeLink{{Path: "linked", Kind: application.LinkOutside}},
			reason: "linked",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newMemoryTree("/root")
			tree.files["repo-config.yml"] = tc.config
			tree.files["docs/a.md"] = "one two three\n"
			tree.treeLinks = tc.links
			outcome := application.Execute(tree, args("governance", "word-budget", "validate"), nil)
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
		})
	}
}
