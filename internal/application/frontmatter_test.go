package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

func frontmatterTree(config string, files map[string]string) *memoryTree {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = config
	for path, content := range files {
		tree.files[path] = content
	}
	return tree
}

const frontmatterConfig = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    frontmatter:\n      surfaces:\n        - glob: \"notes/*.md\"\n          require: [title, status]\n          enum:\n            status: [draft, published]\n          iso-date: [date]\n          forbid: [legacy]\n"

func TestExecuteFrontmatterFindings(t *testing.T) {
	cases := []struct {
		name     string
		files    map[string]string
		exitCode int
		contains []string
		absent   []string
	}{
		{
			name:     "valid block passes",
			files:    map[string]string{"notes/a.md": "---\ntitle: Entry\nstatus: draft\ndate: 2024-02-29\n---\n# body\n"},
			exitCode: 0,
		},
		{
			name:     "quoted values read as the same value",
			files:    map[string]string{"notes/a.md": "---\ntitle: 'Entry'\nstatus: \"draft\"\n---\n"},
			exitCode: 0,
		},
		{
			name:     "century leap day passes only on multiples of 400",
			files:    map[string]string{"notes/a.md": "---\ntitle: Entry\nstatus: draft\ndate: 2000-02-29\n---\n"},
			exitCode: 0,
		},
		{
			name:     "century non-leap day is rejected",
			files:    map[string]string{"notes/a.md": "---\ntitle: Entry\nstatus: draft\ndate: 1900-02-29\n---\n"},
			exitCode: 1,
			contains: []string{"does not hold an ISO calendar date"},
		},
		{
			name:     "malformed date shapes are rejected",
			files:    map[string]string{"notes/a.md": "---\ntitle: Entry\nstatus: draft\ndate: 2026-2-01\n---\n"},
			exitCode: 1,
			contains: []string{"does not hold an ISO calendar date"},
		},
		{
			name:     "month outside the calendar is rejected",
			files:    map[string]string{"notes/a.md": "---\ntitle: Entry\nstatus: draft\ndate: 2026-13-01\n---\n"},
			exitCode: 1,
			contains: []string{"does not hold an ISO calendar date"},
		},
		{
			name:     "day zero is rejected",
			files:    map[string]string{"notes/a.md": "---\ntitle: Entry\nstatus: draft\ndate: 2026-01-00\n---\n"},
			exitCode: 1,
			contains: []string{"does not hold an ISO calendar date"},
		},
		{
			name:     "non-declarations inside the block are ignored",
			files:    map[string]string{"notes/a.md": "---\ntitle: Entry\nstatus: draft\njust words\n: nameless\n---\n"},
			exitCode: 0,
		},
		{
			name:     "missing required key is reported at line one",
			files:    map[string]string{"notes/a.md": "---\ntitle: Entry\n---\n"},
			exitCode: 1,
			contains: []string{"front matter declares no `status`", "notes/a.md:1"},
		},
		{
			name:     "empty block misses every required key",
			files:    map[string]string{"notes/a.md": "---\n---\n"},
			exitCode: 1,
			contains: []string{"front matter declares no `title`", "front matter declares no `status`"},
		},
		{
			name:     "no block misses every required key",
			files:    map[string]string{"notes/a.md": "# just prose\n"},
			exitCode: 1,
			contains: []string{"front matter declares no `title`"},
		},
		{
			name:     "fence further down is prose not front matter",
			files:    map[string]string{"notes/a.md": "prose\n---\ntitle: x\n---\n"},
			exitCode: 1,
			contains: []string{"front matter declares no `title`"},
		},
		{
			name:     "forbidden key is reported at its line",
			files:    map[string]string{"notes/a.md": "---\ntitle: Entry\nstatus: draft\nlegacy: yes\n---\n"},
			exitCode: 1,
			contains: []string{"`legacy` is a key this surface forbids", "notes/a.md:4"},
		},
		{
			name:     "value outside its declared set is reported",
			files:    map[string]string{"notes/a.md": "---\ntitle: Entry\nstatus: wrong\n---\n"},
			exitCode: 1,
			contains: []string{"`status` holds a value outside its declared set", "value=wrong"},
		},
		{
			name:     "impossible calendar date is reported",
			files:    map[string]string{"notes/a.md": "---\ntitle: Entry\nstatus: draft\ndate: 2026-02-30\n---\n"},
			exitCode: 1,
			contains: []string{"`date` does not hold an ISO calendar date", "value=2026-02-30"},
		},
		{
			name:     "leap day passes only in a leap year",
			files:    map[string]string{"notes/a.md": "---\ntitle: Entry\nstatus: draft\ndate: 2023-02-29\n---\n"},
			exitCode: 1,
			contains: []string{"does not hold an ISO calendar date"},
		},
		{
			name:     "unterminated block is reported not parsed",
			files:    map[string]string{"notes/a.md": "---\ntitle: broken"},
			exitCode: 1,
			contains: []string{"front matter opens and never closes", "notes/a.md:1"},
		},
		{
			name:     "indented and nested declarations are not top-level keys",
			files:    map[string]string{"notes/a.md": "---\n  title: nested\n# comment\nstatus: draft\ntitle: Entry\n---\n"},
			exitCode: 0,
		},
		{
			name:     "files outside every surface are not inspected",
			files:    map[string]string{"outside/a.md": "---\n---\n"},
			exitCode: 0,
		},
		{
			name:     "non-markdown files are not inspected",
			files:    map[string]string{"notes/a.txt": "---\n---\n"},
			exitCode: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome := application.Execute(frontmatterTree(frontmatterConfig, tc.files),
				args("md", "frontmatter", "validate"), nil)
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

func TestExecuteFrontmatterUnreadableFiles(t *testing.T) {
	tree := frontmatterTree(frontmatterConfig, map[string]string{"notes/a.md": "---\n---\n"})
	tree.unreadable["notes/a.md"] = true
	if outcome := application.Execute(tree, args("md", "frontmatter", "validate"), nil); outcome.ExitCode != 2 {
		t.Fatalf("unreadable file did not refuse: %+v", outcome)
	}
	binary := frontmatterTree(frontmatterConfig, map[string]string{"notes/a.md": "bytes"})
	binary.binary["notes/a.md"] = true
	if outcome := application.Execute(binary, args("md", "frontmatter", "validate"), nil); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "holds no text") {
		t.Fatalf("binary file did not refuse: %+v", outcome)
	}
}

func TestExecuteFrontmatterKindsInJSON(t *testing.T) {
	tree := frontmatterTree(frontmatterConfig, map[string]string{
		"notes/a.md": "---\nstatus: wrong\ndate: 2026-02-30\nlegacy: yes\n---\n",
	})
	outcome := application.Execute(tree, args("md", "frontmatter", "validate", "--output", "json"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	for _, kind := range []string{"\"kind\":\"missing-frontmatter-key\"", "\"kind\":\"invalid-frontmatter-value\"", "\"kind\":\"forbidden-frontmatter-key\""} {
		if !strings.Contains(outcome.Stdout, kind) {
			t.Fatalf("kind %s missing: %+v", kind, outcome)
		}
	}
	if !strings.Contains(outcome.Stdout, "\"value\":\"wrong\"") {
		t.Fatalf("value detail missing: %+v", outcome)
	}
	// Violations read in file order: line 2 (status) before line 3 (date)
	// before line 4 (legacy), whatever the kinds alphabetize to.
	status := strings.Index(outcome.Stdout, "outside its declared set")
	date := strings.Index(outcome.Stdout, "ISO calendar date")
	legacy := strings.Index(outcome.Stdout, "this surface forbids")
	if status < 0 || date < 0 || legacy < 0 || !(status < date && date < legacy) {
		t.Fatalf("violations out of file order: %+v", outcome)
	}
}

func TestExecuteFrontmatterRefusals(t *testing.T) {
	cases := []struct {
		name   string
		config string
		stderr []string
	}{
		{
			name:   "undeclared policy is refused at its owner",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown: {}\n",
			stderr: []string{"policies.markdown.frontmatter"},
		},
		{
			name:   "malformed surface glob is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    frontmatter:\n      surfaces:\n        - glob: \"[\"\n          require: []\n",
			stderr: []string{"md-frontmatter.surfaces"},
		},
		{
			name:   "missing surfaces are refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    frontmatter: {}\n",
			stderr: []string{"surfaces"},
		},
		{
			name:   "missing require is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    frontmatter:\n      surfaces:\n        - glob: \"notes/*.md\"\n",
			stderr: []string{"require"},
		},
		{
			name:   "unknown surface key is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    frontmatter:\n      surfaces:\n        - glob: \"notes/*.md\"\n          require: []\n          schema: {}\n",
			stderr: []string{"unknown key"},
		},
		{
			name:   "empty surface glob is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    frontmatter:\n      surfaces:\n        - glob: \"\"\n          require: []\n",
			stderr: []string{"no glob"},
		},
		{
			name:   "surface without a mapping is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    frontmatter:\n      surfaces: [\"notes/*.md\"]\n",
			stderr: []string{"no surface"},
		},
		{
			name:   "unknown policy key is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    frontmatter:\n      surfaces: []\n      require: []\n",
			stderr: []string{"unknown key"},
		},
		{
			name:   "non-string require entry is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    frontmatter:\n      surfaces:\n        - glob: \"notes/*.md\"\n          require: [42]\n",
			stderr: []string{"no require"},
		},
		{
			name:   "enum outside a mapping is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    frontmatter:\n      surfaces:\n        - glob: \"notes/*.md\"\n          require: []\n          enum: [draft]\n",
			stderr: []string{"no enum"},
		},
		{
			name:   "enum with non-string values is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    frontmatter:\n      surfaces:\n        - glob: \"notes/*.md\"\n          require: []\n          enum:\n            status: [42]\n",
			stderr: []string{"no enum"},
		},
		{
			name:   "iso-date outside a list is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    frontmatter:\n      surfaces:\n        - glob: \"notes/*.md\"\n          require: []\n          iso-date: yesterday\n",
			stderr: []string{"no iso-date"},
		},
		{
			name:   "forbid with non-string entries is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    frontmatter:\n      surfaces:\n        - glob: \"notes/*.md\"\n          require: []\n          forbid: [42]\n",
			stderr: []string{"no forbid"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome := application.Execute(frontmatterTree(tc.config, nil),
				args("md", "frontmatter", "validate"), nil)
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
