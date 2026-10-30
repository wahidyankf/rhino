package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

func wordBudgetTree(config string, files map[string]string) *memoryTree {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = config
	for path, content := range files {
		tree.files[path] = content
	}
	return tree
}

const wordBudgetBase = "schema: rhino/repo-config/v2\npolicies:\n  governance:\n    word-budget:\n"

func TestExecuteWordBudgetFindings(t *testing.T) {
	cases := []struct {
		name     string
		config   string
		files    map[string]string
		exitCode int
		contains []string
		absent   []string
	}{
		{
			name:   "a repository under budget reports clean and lists what it read",
			config: wordBudgetBase + "      count: letters-and-digits\n      surfaces:\n        - glob: \"**/*.md\"\n          fail: 3\n",
			files: map[string]string{
				"docs/a.md": "one two\n",
				"docs/b.md": "one two three\n",
			},
			exitCode: 0,
			contains: []string{
				"[word-budget] checked 2 files, no findings",
				"[word-budget] scanned docs/a.md",
				"[word-budget] scanned docs/b.md",
			},
		},
		{
			name:   "a file exactly at its limit passes",
			config: wordBudgetBase + "      count: letters-and-digits\n      surfaces:\n        - glob: \"**/*.md\"\n          fail: 3\n",
			files: map[string]string{
				"docs/a.md": "one two three\n",
			},
			exitCode: 0,
			contains: []string{"[word-budget] checked 1 file, no findings"},
		},
		{
			name:   "a file over its limit is reported with its count and limit",
			config: wordBudgetBase + "      count: letters-and-digits\n      surfaces:\n        - glob: \"**/*.md\"\n          fail: 3\n",
			files: map[string]string{
				"docs/a.md": "one two three four\n",
			},
			exitCode: 1,
			contains: []string{
				"[word-budget] checked 1 file, 1 finding",
				"[word-budget] scanned docs/a.md",
				"docs/a.md: is longer than its declared word budget (words=4, limit=3)",
			},
		},
		{
			name: "the last matching surface governs the file",
			config: wordBudgetBase + "      count: letters-and-digits\n      surfaces:\n" +
				"        - glob: \"**/*.md\"\n          fail: 2\n" +
				"        - glob: \"docs/*.md\"\n          fail: 3\n",
			files: map[string]string{
				"docs/a.md":  "one two three\n",
				"notes/a.md": "one two three\n",
			},
			exitCode: 1,
			contains: []string{
				"[word-budget] checked 2 files, 1 finding",
				"notes/a.md: is longer than its declared word budget (words=3, limit=2)",
			},
			absent: []string{"docs/a.md:"},
		},
		{
			name:   "joined words count once while markdown syntax and urls count as their parts",
			config: wordBudgetBase + "      count: letters-and-digits\n      surfaces:\n        - glob: \"**/*.md\"\n          fail: 1\n",
			files: map[string]string{
				"a.md": "can't-stop\n",
				"b.md": "one -- two\n",
				"c.md": "**bold** https://example.com/a/b\n",
			},
			exitCode: 1,
			contains: []string{
				"b.md: is longer than its declared word budget (words=2, limit=1)",
				"c.md: is longer than its declared word budget (words=6, limit=1)",
			},
			absent: []string{"a.md:"},
		},
		{
			name:   "whitespace-separated counting keeps bold and urls whole",
			config: wordBudgetBase + "      count: whitespace-separated\n      surfaces:\n        - glob: \"**/*.md\"\n          fail: 2\n",
			files: map[string]string{
				"a.md": "can't-stop déjà_vu 123\n",
				"b.md": "**bold** https://example.com/a/b\n",
				"c.md": "one\n two\tthree\n",
			},
			exitCode: 1,
			contains: []string{
				"a.md: is longer than its declared word budget (words=3, limit=2)",
				"c.md: is longer than its declared word budget (words=3, limit=2)",
			},
			absent: []string{"b.md:"},
		},
		{
			name:   "files no surface reaches are skipped while the listing stays sorted",
			config: wordBudgetBase + "      count: letters-and-digits\n      surfaces:\n        - glob: \"docs/*.md\"\n          fail: 1\n",
			files: map[string]string{
				"notes/a.md": "one two three\n",
				"docs/b.md":  "one two\n",
				"docs/a.md":  "one\n",
			},
			exitCode: 1,
			contains: []string{
				"[word-budget] checked 2 files, 1 finding",
				"[word-budget] scanned docs/a.md",
				"[word-budget] scanned docs/b.md",
				"docs/b.md: is longer than its declared word budget (words=2, limit=1)",
			},
			absent: []string{"notes/a.md"},
		},
		{
			name:   "non-markdown files are never counted even when a surface reaches them",
			config: wordBudgetBase + "      count: letters-and-digits\n      surfaces:\n        - glob: \"**/*\"\n          fail: 1\n",
			files: map[string]string{
				"a.txt": "one two three\n",
				"b.md":  "one two three\n",
			},
			exitCode: 1,
			contains: []string{
				"[word-budget] checked 1 file, 1 finding",
				"[word-budget] scanned b.md",
			},
			absent: []string{"a.txt"},
		},
		{
			name:   "an uppercase markdown extension is still a markdown file",
			config: wordBudgetBase + "      count: letters-and-digits\n      surfaces:\n        - glob: \"**/*.md\"\n          fail: 1\n",
			files: map[string]string{
				"F.MD": "README MD test\n",
			},
			exitCode: 1,
			contains: []string{
				"F.MD: is longer than its declared word budget (words=3, limit=1)",
			},
		},
		{
			name:   "an empty file is listed as read and stays clean",
			config: wordBudgetBase + "      count: letters-and-digits\n      surfaces:\n        - glob: \"**/*.md\"\n          fail: 1\n",
			files: map[string]string{
				"a.md": "",
			},
			exitCode: 0,
			contains: []string{
				"[word-budget] checked 1 file, no findings",
				"[word-budget] scanned a.md",
			},
		},
		{
			name:   "a scan-excluded directory is never read and never accused",
			config: "schema: rhino/repo-config/v2\nscan:\n  exclude-directories: [generated]\npolicies:\n  governance:\n    word-budget:\n      count: letters-and-digits\n      surfaces:\n        - glob: \"**/*.md\"\n          fail: 1\n",
			files: map[string]string{
				"generated/a.md": "one two three\n",
				"docs/a.md":      "one two three\n",
			},
			exitCode: 1,
			contains: []string{
				"[word-budget] checked 1 file, 1 finding",
				"[word-budget] scanned docs/a.md",
				"docs/a.md: is longer than its declared word budget (words=3, limit=1)",
			},
			absent: []string{"generated/a.md"},
		},
		{
			name:   "a warn and target alongside the limit change nothing about the finding",
			config: wordBudgetBase + "      count: letters-and-digits\n      surfaces:\n        - glob: \"**/*.md\"\n          fail: 1\n          warn: 0\n          target: 0\n",
			files: map[string]string{
				"a.md": "one two three\n",
			},
			exitCode: 1,
			contains: []string{
				"a.md: is longer than its declared word budget (words=3, limit=1)",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome := application.Execute(wordBudgetTree(tc.config, tc.files),
				args("governance", "word-budget", "validate"), nil)
			if outcome.ExitCode != tc.exitCode {
				t.Fatalf("exit code %d, want %d: %+v", outcome.ExitCode, tc.exitCode, outcome)
			}
			for _, want := range tc.contains {
				if !strings.Contains(outcome.Stdout+outcome.Stderr, want) {
					t.Fatalf("output misses %q: %+v", want, outcome)
				}
			}
			for _, missing := range tc.absent {
				if strings.Contains(outcome.Stdout+outcome.Stderr, missing) {
					t.Fatalf("output unexpectedly contains %q: %+v", missing, outcome)
				}
			}
		})
	}
}

func TestExecuteWordBudgetJSON(t *testing.T) {
	config := wordBudgetBase + "      count: letters-and-digits\n      surfaces:\n        - glob: \"**/*.md\"\n          fail: 3\n"
	outcome := application.Execute(
		wordBudgetTree(config, map[string]string{"a.md": "one two three four\n"}),
		args("governance", "word-budget", "validate", "--output", "json"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	for _, want := range []string{
		`"command":"word-budget"`,
		`"exitCode":1`,
		`"subject":"file"`,
		`"inspected":1`,
		`"scanned":["a.md"]`,
		`"kind":"word-limit-exceeded"`,
		`"path":"a.md"`,
		`"message":"is longer than its declared word budget"`,
		`"words":4`,
		`"limit":3`,
	} {
		if !strings.Contains(outcome.Stdout, want) {
			t.Fatalf("json misses %q: %+v", want, outcome)
		}
	}
	if outcome.Stderr != "" {
		t.Fatalf("unexpected stderr: %q", outcome.Stderr)
	}

	clean := application.Execute(
		wordBudgetTree(config, map[string]string{"a.md": "one two\n"}),
		args("governance", "word-budget", "validate", "--output", "json"), nil)
	if clean.ExitCode != 0 || !strings.Contains(clean.Stdout, `"exitCode":0`) ||
		!strings.Contains(clean.Stdout, `"scanned":["a.md"]`) || clean.Stderr != "" {
		t.Fatalf("unexpected clean json: %+v", clean)
	}
}

func TestExecuteWordBudgetRefusals(t *testing.T) {
	cases := []struct {
		name   string
		config string
		stderr []string
	}{
		{
			name:   "undeclared policy is refused at its owner",
			config: "schema: rhino/repo-config/v2\npolicies:\n  governance: {}\n",
			stderr: []string{"policies.governance.word-budget"},
		},
		{
			name:   "malformed surface glob is refused",
			config: wordBudgetBase + "      count: letters-and-digits\n      surfaces:\n        - glob: \"[\"\n          fail: 1\n",
			stderr: []string{"governance-word-budget.surfaces"},
		},
		{
			name:   "missing surfaces are refused",
			config: wordBudgetBase + "      count: letters-and-digits\n",
			stderr: []string{"word-budget.surfaces"},
		},
		{
			name:   "an unknown word rule is refused",
			config: wordBudgetBase + "      count: characters\n      surfaces:\n        - glob: \"**/*.md\"\n          fail: 1\n",
			stderr: []string{"word-budget.count"},
		},
		{
			name:   "an unknown policy key is refused",
			config: wordBudgetBase + "      count: letters-and-digits\n      surfaces: []\n      extra: 1\n",
			stderr: []string{"unknown key"},
		},
		{
			name:   "a surface without a mapping is refused",
			config: wordBudgetBase + "      count: letters-and-digits\n      surfaces: [\"**/*.md\"]\n",
			stderr: []string{"word-budget.surfaces"},
		},
		{
			name:   "a surface without a limit is refused",
			config: wordBudgetBase + "      count: letters-and-digits\n      surfaces:\n        - glob: \"**/*.md\"\n",
			stderr: []string{"word-budget.surfaces"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome := application.Execute(wordBudgetTree(tc.config, map[string]string{"a.md": "one\n"}),
				args("governance", "word-budget", "validate"), nil)
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

func TestExecuteWordBudgetUnreadableAndBinary(t *testing.T) {
	tree := wordBudgetTree(wordBudgetBase+"      count: letters-and-digits\n      surfaces:\n        - glob: \"**/*.md\"\n          fail: 100\n", map[string]string{"docs/a.md": "one two\n"})
	tree.unreadable["docs/a.md"] = true
	if outcome := application.Execute(tree,
		args("governance", "word-budget", "validate"), nil); outcome.ExitCode != 2 ||
		!strings.Contains(outcome.Stderr, "docs/a.md") {
		t.Fatalf("unreadable file did not refuse: %+v", outcome)
	}

	binary := wordBudgetTree(wordBudgetBase+"      count: letters-and-digits\n      surfaces:\n        - glob: \"**/*.md\"\n          fail: 100\n", map[string]string{"docs/a.md": "one two\n"})
	binary.binary["docs/a.md"] = true
	if outcome := application.Execute(binary,
		args("governance", "word-budget", "validate"), nil); outcome.ExitCode != 2 ||
		!strings.Contains(outcome.Stderr, "docs/a.md: holds no text") {
		t.Fatalf("binary file did not refuse: %+v", outcome)
	}

	vanished := wordBudgetTree(wordBudgetBase+"      count: letters-and-digits\n      surfaces:\n        - glob: \"**/*.md\"\n          fail: 100\n", map[string]string{"docs/a.md": "one two\n"})
	vanished.vanished["docs/a.md"] = true
	if outcome := application.Execute(vanished,
		args("governance", "word-budget", "validate"), nil); outcome.ExitCode != 0 ||
		!strings.Contains(outcome.Stdout, "checked 0 files") {
		t.Fatalf("vanished file was not skipped: %+v", outcome)
	}
}
