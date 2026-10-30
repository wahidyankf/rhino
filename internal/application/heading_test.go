package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

func headingTree(config string, files map[string]string) *memoryTree {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = config
	for path, content := range files {
		tree.files[path] = content
	}
	return tree
}

const headingConfig = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    heading-hierarchy:\n      surfaces:\n        - glob: \"docs/*.md\"\n      single-h1: true\n      max-level-jump: 1\n"

func TestExecuteHeadingHierarchyFindings(t *testing.T) {
	cases := []struct {
		name     string
		files    map[string]string
		exitCode int
		contains []string
	}{
		{
			name:     "ordered headings pass",
			files:    map[string]string{"docs/a.md": "# Top\n\n## Next\n\n### Deep\n"},
			exitCode: 0,
		},
		{
			name:     "missing level-1 is reported at line one",
			files:    map[string]string{"docs/a.md": "## starts deep\n"},
			exitCode: 1,
			contains: []string{"declares no level-1 heading", "docs/a.md:1"},
		},
		{
			name:     "every extra level-1 is reported at its line",
			files:    map[string]string{"docs/a.md": "# first\n# second\n# third\n"},
			exitCode: 1,
			contains: []string{"declares a second level-1 heading", "docs/a.md:2", "docs/a.md:3"},
		},
		{
			name:     "excessive drop is reported with its jump",
			files:    map[string]string{"docs/a.md": "# top\n#### skip\n"},
			exitCode: 1,
			contains: []string{"drops from level 1 to level 4", "jump=3"},
		},
		{
			name:     "a drop within the allowance passes",
			files:    map[string]string{"docs/a.md": "# top\n## next\n"},
			exitCode: 0,
		},
		{
			name:     "climbing back up is never a drop",
			files:    map[string]string{"docs/a.md": "# top\n#### deep\n## back up\n"},
			exitCode: 1,
			contains: []string{"drops from level 1 to level 4"},
		},
		{
			name:     "a hash inside a fenced block is not a heading",
			files:    map[string]string{"docs/a.md": "# top\n```sh\n# fenced\n```\n"},
			exitCode: 0,
		},
		{
			name:     "hashtags and overlong runs are not headings",
			files:    map[string]string{"docs/a.md": "# top\n#hashtag\n####### too deep\n"},
			exitCode: 0,
		},
		{
			name:     "indented headings still count",
			files:    map[string]string{"docs/a.md": "# top\n  ## second\n"},
			exitCode: 0,
		},
		{
			name:     "files outside every surface are not inspected",
			files:    map[string]string{"outside/a.md": "### ignored\n"},
			exitCode: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome := application.Execute(headingTree(headingConfig, tc.files),
				args("md", "heading-hierarchy", "validate"), nil)
			if outcome.ExitCode != tc.exitCode {
				t.Fatalf("exit %d want %d: %+v", outcome.ExitCode, tc.exitCode, outcome)
			}
			for _, want := range tc.contains {
				if !strings.Contains(outcome.Stderr, want) {
					t.Fatalf("stderr misses %q: %+v", want, outcome)
				}
			}
		})
	}
}

func TestExecuteHeadingHierarchyJumpAllowance(t *testing.T) {
	config := "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    heading-hierarchy:\n      surfaces:\n        - glob: \"docs/*.md\"\n      single-h1: false\n      max-level-jump: 2\n"
	outcome := application.Execute(
		headingTree(config, map[string]string{"docs/a.md": "### deep start\n##### drop of two\n"}),
		args("md", "heading-hierarchy", "validate"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("allowed jump was reported: %+v", outcome)
	}
}

func TestExecuteHeadingHierarchyUnreadableFiles(t *testing.T) {
	tree := headingTree(headingConfig, map[string]string{"docs/a.md": "# top\n"})
	tree.unreadable["docs/a.md"] = true
	if outcome := application.Execute(tree, args("md", "heading-hierarchy", "validate"), nil); outcome.ExitCode != 2 {
		t.Fatalf("unreadable file did not refuse: %+v", outcome)
	}
	binary := headingTree(headingConfig, map[string]string{"docs/a.md": "bytes"})
	binary.binary["docs/a.md"] = true
	if outcome := application.Execute(binary, args("md", "heading-hierarchy", "validate"), nil); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "holds no text") {
		t.Fatalf("binary file did not refuse: %+v", outcome)
	}
}

func TestExecuteHeadingHierarchyKindsInJSON(t *testing.T) {
	tree := headingTree(headingConfig, map[string]string{"docs/a.md": "# first\n# second\n#### skip\n"})
	outcome := application.Execute(tree, args("md", "heading-hierarchy", "validate", "--output", "json"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	for _, want := range []string{"\"kind\":\"multiple-h1\"", "\"kind\":\"heading-level-jump\"", "\"jump\":3"} {
		if !strings.Contains(outcome.Stdout, want) {
			t.Fatalf("stdout misses %q: %+v", want, outcome)
		}
	}
}

func TestExecuteHeadingHierarchyRefusals(t *testing.T) {
	cases := []struct {
		name   string
		config string
		stderr []string
	}{
		{
			name:   "undeclared policy is refused at its owner",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown: {}\n",
			stderr: []string{"policies.markdown.heading-hierarchy"},
		},
		{
			name:   "malformed surface glob is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    heading-hierarchy:\n      surfaces:\n        - glob: \"[\"\n      single-h1: true\n      max-level-jump: 1\n",
			stderr: []string{"md-heading-hierarchy.surfaces"},
		},
		{
			name:   "missing surfaces are refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    heading-hierarchy:\n      single-h1: true\n      max-level-jump: 1\n",
			stderr: []string{"surfaces"},
		},
		{
			name:   "missing single-h1 is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    heading-hierarchy:\n      surfaces:\n        - glob: \"docs/*.md\"\n      max-level-jump: 1\n",
			stderr: []string{"single-h1"},
		},
		{
			name:   "missing max-level-jump is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    heading-hierarchy:\n      surfaces:\n        - glob: \"docs/*.md\"\n      single-h1: true\n",
			stderr: []string{"max-level-jump"},
		},
		{
			name:   "unknown surface key is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    heading-hierarchy:\n      surfaces:\n        - glob: \"docs/*.md\"\n          style: kebab-case\n      single-h1: true\n      max-level-jump: 1\n",
			stderr: []string{"unknown key"},
		},
		{
			name:   "empty surface glob is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    heading-hierarchy:\n      surfaces:\n        - glob: \"\"\n      single-h1: true\n      max-level-jump: 1\n",
			stderr: []string{"no glob"},
		},
		{
			name:   "surface without a mapping is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    heading-hierarchy:\n      surfaces: [\"docs/*.md\"]\n      single-h1: true\n      max-level-jump: 1\n",
			stderr: []string{"no surface"},
		},
		{
			name:   "unknown policy key is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    heading-hierarchy:\n      surfaces:\n        - glob: \"docs/*.md\"\n      single-h1: true\n      max-level-jump: 1\n      min-level: 1\n",
			stderr: []string{"unknown key"},
		},
		{
			name:   "non-boolean single-h1 is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    heading-hierarchy:\n      surfaces:\n        - glob: \"docs/*.md\"\n      single-h1: sometimes\n      max-level-jump: 1\n",
			stderr: []string{"single-h1"},
		},
		{
			name:   "non-numeric max-level-jump is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    heading-hierarchy:\n      surfaces:\n        - glob: \"docs/*.md\"\n      single-h1: true\n      max-level-jump: far\n",
			stderr: []string{"max-level-jump"},
		},
		{
			name:   "negative max-level-jump is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    heading-hierarchy:\n      surfaces:\n        - glob: \"docs/*.md\"\n      single-h1: true\n      max-level-jump: -1\n",
			stderr: []string{"max-level-jump"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome := application.Execute(headingTree(tc.config, nil),
				args("md", "heading-hierarchy", "validate"), nil)
			if outcome.ExitCode != 2 {
				t.Fatalf("expected refusal: %+v", outcome)
			}
			for _, want := range tc.stderr {
				if !strings.Contains(outcome.Stderr, want) {
					t.Fatalf("stderr misses %q: %+v", want, outcome)
				}
			}
		})
	}
}
