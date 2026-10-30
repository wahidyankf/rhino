package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

func linkTree(config string, files map[string]string) *memoryTree {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = config
	for path, content := range files {
		tree.files[path] = content
	}
	return tree
}

const linkConfig = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    internal-link:\n      exclude-sources: []\n"

func TestExecuteInternalLinkFindings(t *testing.T) {
	cases := []struct {
		name     string
		files    map[string]string
		links    []string
		exitCode int
		contains []string
		absent   []string
	}{
		{
			name:     "resolving links pass",
			files:    map[string]string{"docs/a.md": "[guide](guide.md)\n", "docs/guide.md": "# guide\n"},
			exitCode: 0,
		},
		{
			name:     "missing target is reported at its line",
			files:    map[string]string{"docs/a.md": "[good](guide.md) [missing](missing.md)\n", "docs/guide.md": "# guide\n"},
			exitCode: 1,
			contains: []string{"`missing.md` does not exist", "docs/a.md:1"},
		},
		{
			name:     "absolute and climbing targets resolve outside",
			files:    map[string]string{"docs/a.md": "[abs](/tmp/file) [up](../../outside)\n"},
			exitCode: 1,
			contains: []string{"resolves outside the repository"},
		},
		{
			name:     "nul byte is malformed",
			files:    map[string]string{"docs/a.md": "[bad](\x00)\n"},
			exitCode: 1,
			contains: []string{"`\\0` is not a usable path"},
		},
		{
			name:     "images fragments urls and queries are skipped",
			files:    map[string]string{"docs/a.md": "![image](image.png) [here](#part) [url](https://example.com/x.md) [query](guide.md?x=1#part)\n", "docs/guide.md": "# guide\n"},
			exitCode: 0,
		},
		{
			name:     "code spans and fenced blocks are quotations not links",
			files:    map[string]string{"docs/a.md": "` [quoted](no.md) `\n```md\n[fenced](nope.md)\n```\n"},
			exitCode: 0,
		},
		{
			name:     "reference definitions carry the target",
			files:    map[string]string{"docs/a.md": "[label]: refs/target.md\n", "docs/refs/target.md": "# target\n"},
			exitCode: 0,
		},
		{
			name:     "missing definition target is reported",
			files:    map[string]string{"docs/a.md": "[label]: refs/gone.md\n"},
			exitCode: 1,
			contains: []string{"`refs/gone.md` does not exist", "docs/a.md:1"},
		},
		{
			name:     "empty and query-only targets inspect nothing to report",
			files:    map[string]string{"docs/a.md": "[empty]() [query](?x=1)\n"},
			exitCode: 0,
		},
		{
			name:     "directory targets exist",
			files:    map[string]string{"docs/a.md": "[dir](sub)\n", "docs/sub/page.md": "# page\n"},
			exitCode: 0,
		},
		{
			name:     "targets behind a link resolve outside",
			files:    map[string]string{"docs/a.md": "[held](outside/held.md)\n", "docs/outside/held.md": "# held\n"},
			links:    []string{"docs/outside"},
			exitCode: 1,
			contains: []string{"resolves outside the repository"},
		},
		{
			name:     "parent segments resolve inside",
			files:    map[string]string{"docs/sub/a.md": "[up](../guide.md)\n", "docs/guide.md": "# guide\n"},
			exitCode: 0,
		},
		{
			name:     "inspection continues past a fault",
			files:    map[string]string{"docs/a.md": "[bad](\x00)\n", "docs/b.md": "[gone](gone.md)\n"},
			exitCode: 1,
			contains: []string{"not a usable path", "does not exist"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := linkTree(linkConfig, tc.files)
			for _, link := range tc.links {
				tree.links[link] = true
			}
			outcome := application.Execute(tree, args("md", "internal-link", "validate"), nil)
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

func TestExecuteInternalLinkCountsInspectedLinks(t *testing.T) {
	tree := linkTree(linkConfig, map[string]string{
		"docs/a.md":     "[good](guide.md) [missing](missing.md)\n",
		"docs/guide.md": "# guide\n",
		"plain.txt":     "[ignored](gone.md)\n",
	})
	outcome := application.Execute(tree, args("md", "internal-link", "validate"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	if !strings.Contains(outcome.Stdout, "[internal-link] checked 2 links, 1 finding") {
		t.Fatalf("inspected count wrong: %q", outcome.Stdout)
	}
	if strings.Contains(outcome.Stdout, "plain.txt") {
		t.Fatalf("non-markdown source was scanned: %q", outcome.Stdout)
	}
}

func TestExecuteInternalLinkExcludesSourcesButKeepsTargets(t *testing.T) {
	config := "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    internal-link:\n      exclude-sources:\n        - generated/**\n"
	tree := linkTree(config, map[string]string{
		"generated/a.md": "[ignored](missing.md)\n",
		"docs/a.md":      "[kept](missing.md)\n",
	})
	outcome := application.Execute(tree, args("md", "internal-link", "validate"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	if !strings.Contains(outcome.Stderr, "docs/a.md") {
		t.Fatalf("kept source missing: %+v", outcome)
	}
	if strings.Contains(outcome.Stderr, "generated/a.md") {
		t.Fatalf("excluded source was read: %+v", outcome)
	}
	if !strings.Contains(outcome.Stdout, "[internal-link] checked 1 link, 1 finding") {
		t.Fatalf("excluded source was inspected: %q", outcome.Stdout)
	}
}

func TestExecuteInternalLinkKindsInJSON(t *testing.T) {
	tree := linkTree(linkConfig, map[string]string{
		"docs/a.md": "[missing](missing.md) [outside](../../outside) [bad](\x00)\n",
	})
	outcome := application.Execute(tree, args("md", "internal-link", "validate", "--output", "json"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	for _, want := range []string{
		"\"kind\":\"internal-link-missing\"", "\"kind\":\"internal-link-outside-repository\"",
		"\"kind\":\"internal-link-malformed\"", "\"subject\":\"link\"", "\"inspected\":3",
	} {
		if !strings.Contains(outcome.Stdout, want) {
			t.Fatalf("stdout misses %q: %+v", want, outcome)
		}
	}
}

func TestExecuteInternalLinkUnreadableFiles(t *testing.T) {
	tree := linkTree(linkConfig, map[string]string{"docs/a.md": "[x](y.md)\n"})
	tree.unreadable["docs/a.md"] = true
	if outcome := application.Execute(tree, args("md", "internal-link", "validate"), nil); outcome.ExitCode != 2 {
		t.Fatalf("unreadable source did not refuse: %+v", outcome)
	}
	binary := linkTree(linkConfig, map[string]string{"docs/a.md": "bytes"})
	binary.binary["docs/a.md"] = true
	if outcome := application.Execute(binary, args("md", "internal-link", "validate"), nil); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "holds no text") {
		t.Fatalf("binary source did not refuse: %+v", outcome)
	}
}

func TestExecuteInternalLinkRefusals(t *testing.T) {
	cases := []struct {
		name   string
		config string
		stderr []string
	}{
		{
			name:   "undeclared policy is refused at its owner",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown: {}\n",
			stderr: []string{"policies.markdown.internal-link"},
		},
		{
			name:   "malformed exclude glob is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    internal-link:\n      exclude-sources: [\"[\"]\n",
			stderr: []string{"md-internal-link.exclude-sources"},
		},
		{
			name:   "absolute exclude glob is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    internal-link:\n      exclude-sources: [\"/abs/**\"]\n",
			stderr: []string{"md-internal-link.exclude-sources", "relative to the repository root"},
		},
		{
			name:   "unknown policy key is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    internal-link:\n      exclude-sources: []\n      surfaces: []\n",
			stderr: []string{"unknown key"},
		},
		{
			name:   "non-string exclusion is refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    internal-link:\n      exclude-sources: [42]\n",
			stderr: []string{"no glob"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome := application.Execute(linkTree(tc.config, nil),
				args("md", "internal-link", "validate"), nil)
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
