package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// TestNamingRefusesABrokenPolicyOrAnEscapingSurface proves a naming run refuses
// rather than reporting a clean walk when its scan section is malformed, a
// surface is not a mapping or a glob that cannot compile, or a surface reaches
// a link that leaves the root.
func TestNamingRefusesABrokenPolicyOrAnEscapingSurface(t *testing.T) {
	cases := []struct {
		name   string
		config string
		links  []application.TreeLink
		reason string
	}{
		{
			name:   "a scan section that is not a mapping refuses the exclusion list",
			config: namingConfig + "scan: 3\n",
			reason: "scan.exclude-directories names no directory",
		},
		{
			name:   "a surface that is not a mapping names no surface",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces:\n        - docs/**/*.md\n      exempt: []\n",
			reason: "policies.markdown.naming.surfaces names no surface",
		},
		{
			name:   "a surface glob that cannot compile refuses the run",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces:\n        - glob: \"[\"\n          style: kebab-case\n      exempt: []\n",
			reason: "unterminated character class",
		},
		{
			name:   "a surface that reaches a link leaving the root refuses as an escape",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces:\n        - glob: \"linked/**/*.md\"\n          style: kebab-case\n      exempt: []\n",
			links:  []application.TreeLink{{Path: "linked", Kind: application.LinkOutside}},
			reason: "linked",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newMemoryTree("/root")
			tree.files["repo-config.yml"] = tc.config
			tree.files["docs/good-name.md"] = "# Good\n"
			tree.treeLinks = tc.links
			outcome := application.Execute(tree, args("md", "naming", "validate"), nil)
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
		})
	}
}

// TestNamingGovernsAFileAtTheRepositoryRoot proves a surface glob that names a
// root-level file governs it, and a name that breaks the style is a finding.
func TestNamingGovernsAFileAtTheRepositoryRoot(t *testing.T) {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces:\n        - glob: \"*.md\"\n          style: kebab-case\n      exempt: []\n"
	tree.files["Bad_Name.md"] = "# Bad\n"
	outcome := application.Execute(tree, args("md", "naming", "validate"), nil)
	if outcome.ExitCode != 1 || !strings.Contains(outcome.Stdout+outcome.Stderr, "Bad_Name.md") {
		t.Fatalf("root-level name not governed: %+v", outcome)
	}
}

// TestHeadingHierarchyRefusesABrokenPolicyOrAnEscapingSurface proves a heading
// run refuses when its scan section is malformed or a surface reaches a link
// that leaves the root, and skips a surface file that vanished before its read.
func TestHeadingHierarchyRefusesABrokenPolicyOrAnEscapingSurface(t *testing.T) {
	const linkedConfig = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    heading-hierarchy:\n      surfaces:\n        - glob: \"linked/*.md\"\n      single-h1: true\n      max-level-jump: 1\n"
	cases := []struct {
		name   string
		config string
		links  []application.TreeLink
		reason string
	}{
		{
			name:   "a scan section that is not a mapping refuses the exclusion list",
			config: headingConfig + "scan: 3\n",
			reason: "scan.exclude-directories names no directory",
		},
		{
			name:   "a surface that reaches a link leaving the root refuses as an escape",
			config: linkedConfig,
			links:  []application.TreeLink{{Path: "linked", Kind: application.LinkOutside}},
			reason: "linked",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newMemoryTree("/root")
			tree.files["repo-config.yml"] = tc.config
			tree.files["docs/a.md"] = "# A\n"
			tree.treeLinks = tc.links
			outcome := application.Execute(tree, args("md", "heading-hierarchy", "validate"), nil)
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
		})
	}
}

// TestHeadingHierarchySkipsAFileThatVanishedAfterListing proves a surface file
// listed by the walk and gone by the read is skipped, not refused or accused.
func TestHeadingHierarchySkipsAFileThatVanishedAfterListing(t *testing.T) {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = headingConfig
	tree.files["docs/gone.md"] = "# Gone\n\n### Skipped level\n"
	tree.vanished["docs/gone.md"] = true
	outcome := application.Execute(tree, args("md", "heading-hierarchy", "validate"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("a vanished file refused or was accused: %+v", outcome)
	}
}
