package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// TestFrontmatterRefusesABrokenSurfaceDeclaration proves a malformed surface
// refuses the run, and a surface glob that reaches an escaping link refuses
// too, rather than governing a partial set of files.
func TestFrontmatterRefusesABrokenSurfaceDeclaration(t *testing.T) {
	cases := []struct {
		name   string
		config string
		links  []application.TreeLink
		reason string
	}{
		{
			name:   "an enum whose permitted values are not a list names no enum",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    frontmatter:\n      surfaces:\n        - glob: \"notes/*.md\"\n          require: [title]\n          enum:\n            status: x\n",
			reason: "md-frontmatter.surfaces names no enum",
		},
		{
			name:   "a surface glob that cannot compile refuses the run",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    frontmatter:\n      surfaces:\n        - glob: \"[\"\n          require: [title]\n",
			reason: "md-frontmatter.surfaces: `[`",
		},
		{
			name:   "a surface that reaches a link leaving the root refuses as an escape",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    frontmatter:\n      surfaces:\n        - glob: \"linked/**/*.md\"\n          require: [title]\n",
			links:  []application.TreeLink{{Path: "linked", Kind: application.LinkOutside}},
			reason: "linked",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newMemoryTree("/root")
			tree.files["repo-config.yml"] = tc.config
			tree.files["notes/a.md"] = "---\ntitle: a\n---\n"
			tree.treeLinks = tc.links
			outcome := application.Execute(tree, args("md", "frontmatter", "validate"), nil)
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
		})
	}
}

// TestFrontmatterSkipsAFileThatVanishedAfterListing proves a surface file the
// walk listed and the read found gone is skipped, neither accused nor refused.
func TestFrontmatterSkipsAFileThatVanishedAfterListing(t *testing.T) {
	tree := frontmatterTree(frontmatterConfig, map[string]string{"notes/gone.md": "---\n---\n"})
	tree.vanished["notes/gone.md"] = true
	outcome := application.Execute(tree, args("md", "frontmatter", "validate"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("a vanished file refused or was accused: %+v", outcome)
	}
}

// TestFrontmatterIsoDatesFollowTheCalendar proves an iso-date key is accepted
// only when it names a day the calendar has, including every month length and
// the leap-year rule, and that a blank line inside the block is ignored.
func TestFrontmatterIsoDatesFollowTheCalendar(t *testing.T) {
	cases := []struct {
		name     string
		date     string
		exitCode int
	}{
		{"a leap day in a leap year is a date", "2024-02-29", 0},
		{"a 30-day month's last day is a date", "2026-04-30", 0},
		{"a 28-day February past its last day is not a date", "2026-02-29", 1},
		{"a 30-day month past its last day is not a date", "2026-04-31", 1},
		{"a month above twelve is not a date", "2026-13-01", 1},
		{"a month of zero is not a date", "2026-00-01", 1},
		{"a day of zero is not a date", "2026-01-00", 1},
		{"a short month field is not a date", "2026-1-01", 1},
		{"a date with the wrong separators is not a date", "2026/01/01", 1},
		{"a date with a letter in it is not a date", "20x6-01-01", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text := "---\ntitle: a\nstatus: draft\n\ndate: " + tc.date + "\n---\n"
			tree := frontmatterTree(frontmatterConfig, map[string]string{"notes/a.md": text})
			outcome := application.Execute(tree, args("md", "frontmatter", "validate"), nil)
			if outcome.ExitCode != tc.exitCode {
				t.Fatalf("date %q: expected exit %d: %+v", tc.date, tc.exitCode, outcome)
			}
		})
	}
}
