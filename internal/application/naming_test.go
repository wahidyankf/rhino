package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

func namingTree(config string, files map[string]string) *memoryTree {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = config
	for path, content := range files {
		tree.files[path] = content
	}
	return tree
}

const namingConfig = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces:\n        - glob: \"docs/**/*.md\"\n          style: kebab-case\n      exempt: []\n"

func TestExecuteNamingFindingsAndClean(t *testing.T) {
	cases := []struct {
		name     string
		files    map[string]string
		exitCode int
		contains []string
		absent   []string
	}{
		{
			name:     "kebab-case tree passes",
			files:    map[string]string{"docs/guide.md": "# Guide\n", "docs/topics/page.md": "# Page\n"},
			exitCode: 0,
		},
		{
			name:     "non-kebab name is reported",
			files:    map[string]string{"docs/Bad_Name.md": "# file\n"},
			exitCode: 1,
			contains: []string{"kebab-case style", "docs/Bad_Name.md"},
		},
		{
			name:     "fragment part number is reported",
			files:    map[string]string{"docs/guide-part-2.md": "# fragment\n"},
			exitCode: 1,
			contains: []string{"named as a fragment", "docs/guide-part-2.md"},
		},
		{
			name:     "fragment continuation number is reported",
			files:    map[string]string{"docs/guide-continuation-3.md": "# fragment\n"},
			exitCode: 1,
			contains: []string{"named as a fragment"},
		},
		{
			name:     "fragment continued ending is reported",
			files:    map[string]string{"docs/guide-continued.md": "# fragment\n"},
			exitCode: 1,
			contains: []string{"named as a fragment"},
		},
		{
			name:     "parted and part-two endings are ordinary names",
			files:    map[string]string{"docs/guide-parted.md": "# file\n", "docs/guide-part-two.md": "# file\n"},
			exitCode: 0,
		},
		{
			name:     "non-markdown files are not inspected",
			files:    map[string]string{"docs/Bad_Name.txt": "bytes\n"},
			exitCode: 0,
		},
		{
			name:     "uppercase markdown extension is inspected",
			files:    map[string]string{"docs/Bad_Name.MD": "# file\n"},
			exitCode: 1,
			contains: []string{"kebab-case style"},
		},
		{
			name:     "uncovered files are not inspected",
			files:    map[string]string{"elsewhere/Bad_Name.md": "# file\n"},
			exitCode: 0,
		},
		{
			name:     "extensionless files are not markdown",
			files:    map[string]string{"docs/Makefile": "all:\n"},
			exitCode: 0,
		},
		{
			name:     "empty stems are not kebab-case",
			files:    map[string]string{"docs/.md": "# file\n"},
			exitCode: 1,
			contains: []string{"kebab-case style"},
		},
		{
			name:     "double hyphens are not kebab-case",
			files:    map[string]string{"docs/bad--name.md": "# file\n"},
			exitCode: 1,
			contains: []string{"kebab-case style"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome := application.Execute(namingTree(namingConfig, tc.files),
				args("md", "naming", "validate"), nil)
			if outcome.ExitCode != tc.exitCode {
				t.Fatalf("exit %d want %d: %+v", outcome.ExitCode, tc.exitCode, outcome)
			}
			for _, want := range tc.contains {
				if !strings.Contains(outcome.Stderr, want) {
					t.Fatalf("stderr misses %q: %+v", want, outcome)
				}
			}
			for _, miss := range tc.absent {
				if strings.Contains(outcome.Stderr, miss) {
					t.Fatalf("stderr holds %q: %+v", miss, outcome)
				}
			}
		})
	}
}

func TestExecuteNamingExemptionAndLastSurfaceWins(t *testing.T) {
	config := "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces:\n        - glob: \"docs/**/*.md\"\n          style: kebab-case\n        - glob: \"docs/loose/**\"\n          style: kebab-case\n      exempt:\n        - docs/exempt-name.md\n"
	tree := namingTree(config, map[string]string{
		"docs/Bad_Name.md":       "# file\n",
		"docs/exempt-name.md":    "# file\n",
		"docs/loose/Also_Bad.md": "# file\n",
	})
	outcome := application.Execute(tree, args("md", "naming", "validate"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	if !strings.Contains(outcome.Stderr, "docs/Bad_Name.md") {
		t.Fatalf("governing name not reported: %+v", outcome)
	}
	if strings.Contains(outcome.Stderr, "exempt-name.md") {
		t.Fatalf("exempt file was inspected: %+v", outcome)
	}

	fragmentExempt := "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces:\n        - glob: \"docs/**/*.md\"\n          style: kebab-case\n      exempt:\n        - docs/guide-part-2.md\n"
	fragmented := application.Execute(
		namingTree(fragmentExempt, map[string]string{"docs/guide-part-2.md": "# fragment\n"}),
		args("md", "naming", "validate"), nil)
	if fragmented.ExitCode != 1 || !strings.Contains(fragmented.Stderr, "named as a fragment") {
		t.Fatalf("exempt fragment was not still reported: %+v", fragmented)
	}
}

func TestExecuteNamingKindsInJSON(t *testing.T) {
	tree := namingTree(namingConfig, map[string]string{
		"docs/Bad_Name.md":     "# file\n",
		"docs/guide-part-2.md": "# fragment\n",
	})
	outcome := application.Execute(tree, args("md", "naming", "validate", "--output", "json"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	if !strings.Contains(outcome.Stdout, "\"kind\":\"invalid-md-name\"") {
		t.Fatalf("kebab violation kind missing: %+v", outcome)
	}
	if !strings.Contains(outcome.Stdout, "\"kind\":\"fragmented-md-name\"") {
		t.Fatalf("fragment violation kind missing: %+v", outcome)
	}
}

func TestExecuteNamingPathPrefixed(t *testing.T) {
	config := "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces:\n        - glob: \"areas/**/*.md\"\n          style: path-prefixed\n          separator: \"--\"\n      exempt: []\n"
	valid := application.Execute(namingTree(config, map[string]string{
		"areas/f-sharp/guides/f_sh-gu--topic.md": "# valid\n",
		"areas/guide.md":                         "# root file\n",
		"areas/x/x_--solo.md":                    "# one-character directory stays visible\n",
		"areas/a--b/a_b_--topic.md":              "# empty words encode to nothing\n",
	}), args("md", "naming", "validate"), nil)
	if valid.ExitCode != 0 {
		t.Fatalf("valid prefixed names refused: %+v", valid)
	}
	upperConfig := "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces:\n        - glob: \"AREAS/**/*.md\"\n          style: path-prefixed\n          separator: \"--\"\n      exempt: []\n"
	upper := application.Execute(namingTree(upperConfig, map[string]string{
		"areas/x/y.md": "# file\n",
	}), args("md", "naming", "validate"), nil)
	if upper.ExitCode != 1 || !strings.Contains(upper.Stderr, "path-prefixed style") {
		t.Fatalf("case-insensitive root did not fall back to the full directory: %+v", upper)
	}
	invalid := application.Execute(namingTree(config, map[string]string{
		"areas/f-sharp/guides/wrong--Topic.md": "# invalid\n",
	}), args("md", "naming", "validate", "--output", "json"), nil)
	if invalid.ExitCode != 1 || !strings.Contains(invalid.Stdout, "\"kind\":\"invalid-md-name\"") {
		t.Fatalf("prefixed violation missing: %+v", invalid)
	}
	if !strings.Contains(invalid.Stdout, "\"prefix\":\"f_sh-gu\"") {
		t.Fatalf("prefixed finding misses expected prefix: %+v", invalid)
	}
}

func TestExecuteNamingRefusals(t *testing.T) {
	cases := []struct {
		name   string
		config string
		stderr []string
	}{
		{
			name:   "undeclared policy is refused at its owner",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown: {}\n",
			stderr: []string{"policies.markdown.naming"},
		},
		{
			name:   "malformed surface glob is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces:\n        - glob: \"[\"\n          style: kebab-case\n      exempt: []\n",
			stderr: []string{"md-naming.surfaces"},
		},
		{
			name:   "malformed exempt glob is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces:\n        - glob: \"docs/**/*.md\"\n          style: kebab-case\n      exempt: [\"[\"]\n",
			stderr: []string{"md-naming.exempt"},
		},
		{
			name:   "path-prefixed surface without separator is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces:\n        - glob: \"areas/**/*.md\"\n          style: path-prefixed\n      exempt: []\n",
			stderr: []string{"separator"},
		},
		{
			name:   "kebab-case surface with separator is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces:\n        - glob: \"docs/**/*.md\"\n          style: kebab-case\n          separator: \"--\"\n      exempt: []\n",
			stderr: []string{"separator"},
		},
		{
			name:   "missing surfaces are refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming: {}\n",
			stderr: []string{"surfaces"},
		},
		{
			name:   "missing exempt is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces:\n        - glob: \"docs/**/*.md\"\n          style: kebab-case\n",
			stderr: []string{"exempt"},
		},
		{
			name:   "unknown policy key is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces: []\n      exempt: []\n      styles: []\n",
			stderr: []string{"unknown key"},
		},
		{
			name:   "unknown surface key is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces:\n        - glob: \"docs/**/*.md\"\n          style: kebab-case\n          root: docs\n      exempt: []\n",
			stderr: []string{"unknown key"},
		},
		{
			name:   "empty surface glob is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces:\n        - glob: \"\"\n          style: kebab-case\n      exempt: []\n",
			stderr: []string{"no glob"},
		},
		{
			name:   "empty exempt glob is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces:\n        - glob: \"docs/**/*.md\"\n          style: kebab-case\n      exempt: [\"\"]\n",
			stderr: []string{"no glob"},
		},
		{
			name:   "blank separator is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces:\n        - glob: \"areas/**/*.md\"\n          style: path-prefixed\n          separator: \"\"\n      exempt: []\n",
			stderr: []string{"separator"},
		},
		{
			name:   "unknown style is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces:\n        - glob: \"docs/**/*.md\"\n          style: scream-case\n      exempt: []\n",
			stderr: []string{"style"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome := application.Execute(namingTree(tc.config, nil),
				args("md", "naming", "validate"), nil)
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
