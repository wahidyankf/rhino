package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// TestLayersRefusesAMalformedPolicy proves each malformed layer declaration
// refuses the run with the reason its decoder names, before any file is read.
func TestLayersRefusesAMalformedPolicy(t *testing.T) {
	cases := []struct {
		name   string
		fields string
		reason string
	}{
		{
			name:   "an unknown layer key refuses rather than being ignored",
			fields: "      bogus: x\n",
			reason: "policies.governance.layers names unknown key `bogus`",
		},
		{
			name:   "a root that is not a string names no layer root",
			fields: "      root: [1]\n",
			reason: "layers.root names no layer root",
		},
		{
			name:   "an order that is not a list names no layer order",
			fields: "      order: x\n",
			reason: "layers.order names no layer order",
		},
		{
			name:   "an order item that is not a string names no layer",
			fields: "      order: [1]\n",
			reason: "layers.order names no layer",
		},
		{
			name:   "categories that are not a mapping name no categories",
			fields: "      categories: x\n",
			reason: "layers.categories names no categories",
		},
		{
			name:   "a layer whose categories are not a list names no categories",
			fields: "      categories:\n        domain: x\n",
			reason: "layers.categories names no categories",
		},
		{
			name:   "a category that is not a string names no category",
			fields: "      categories:\n        domain: [1]\n",
			reason: "layers.categories names no category",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newMemoryTree("/root")
			tree.files["repo-config.yml"] = "schema: rhino/repo-config/v2\npolicies:\n  governance:\n    layers:\n" + tc.fields
			outcome := application.Execute(tree, args("governance", "layers", "validate"), nil)
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
		})
	}
}

// TestInternalLinkWithoutExcludedSourcesStillChecksEveryLink proves an
// internal-link policy that declares no exclusion list excludes nothing, so a
// broken link is still reported.
func TestInternalLinkWithoutExcludedSourcesStillChecksEveryLink(t *testing.T) {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    internal-link: {}\n"
	tree.files["docs/a.md"] = "# A\n\nSee [Missing](missing.md).\n"
	outcome := application.Execute(tree, args("md", "internal-link", "validate"), nil)
	if outcome.ExitCode != 1 || !strings.Contains(outcome.Stdout+outcome.Stderr, "docs/a.md") {
		t.Fatalf("broken link not reported with no exclusions declared: %+v", outcome)
	}
}

// TestInternalLinkRefusesAMalformedExclusionList proves an exclusion list that
// is not a list refuses rather than excluding nothing silently.
func TestInternalLinkRefusesAMalformedExclusionList(t *testing.T) {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    internal-link:\n      exclude-sources: x\n"
	outcome := application.Execute(tree, args("md", "internal-link", "validate"), nil)
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, "md-internal-link.exclude-sources names no glob") {
		t.Fatalf("malformed exclusion list not refused: %+v", outcome)
	}
}

// TestInternalLinkLinkShapes proves a link destination is judged as a relative
// path unless it carries a URI scheme, and a code span left open does not hide
// the link that follows it from the check.
func TestInternalLinkLinkShapes(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		exitCode int
		contains string
	}{
		{
			name:     "a destination whose scheme holds an underscore is a relative path, and is broken",
			text:     "# A\n\nSee [Under](a_b:c.md).\n",
			exitCode: 1,
			contains: "a_b:c.md",
		},
		{
			name:     "a destination whose scheme starts with a digit is a relative path, and is broken",
			text:     "# A\n\nSee [Digit](1abc:c.md).\n",
			exitCode: 1,
			contains: "1abc:c.md",
		},
		{
			name:     "a destination with an empty scheme is a relative path, and is broken",
			text:     "# A\n\nSee [Empty](:c.md).\n",
			exitCode: 1,
			contains: ":c.md",
		},
		{
			name:     "a link after an unclosed code span is still checked",
			text:     "# A\n\n`open [Missing](missing.md)\n",
			exitCode: 1,
			contains: "missing.md",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newMemoryTree("/root")
			tree.files["repo-config.yml"] = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    internal-link: {}\n"
			tree.files["docs/a.md"] = tc.text
			outcome := application.Execute(tree, args("md", "internal-link", "validate"), nil)
			if outcome.ExitCode != tc.exitCode || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.contains) {
				t.Fatalf("expected exit %d naming %q: %+v", tc.exitCode, tc.contains, outcome)
			}
		})
	}
}

// TestInternalLinkSkipsAFileThatVanishedAfterListing proves a markdown file the
// walk listed and the read found gone is skipped, neither accused nor refused.
func TestInternalLinkSkipsAFileThatVanishedAfterListing(t *testing.T) {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    internal-link: {}\n"
	tree.files["docs/gone.md"] = "See [Missing](missing.md).\n"
	tree.vanished["docs/gone.md"] = true
	outcome := application.Execute(tree, args("md", "internal-link", "validate"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("a vanished file refused or was accused: %+v", outcome)
	}
}
