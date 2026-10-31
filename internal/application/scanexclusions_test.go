package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

func scanExclusionTree(config string, files map[string]string) *memoryTree {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = config
	for path, content := range files {
		tree.files[path] = content
	}
	return tree
}

const scanExcluded = "schema: rhino/repo-config/v2\nscan:\n  exclude-directories: [generated]\n"

func TestScanExcludedDirectoryIsNotScanned(t *testing.T) {
	cleanInternalLink := scanExcluded + "policies:\n  markdown:\n    internal-link:\n      exclude-sources: []\n"
	broken := map[string]string{"generated/broken.md": "[missing](missing.md)\n"}
	if outcome := application.Execute(
		scanExclusionTree(cleanInternalLink, broken),
		args("md", "internal-link", "validate"), nil,
	); outcome.ExitCode != 0 {
		t.Fatalf("shared exclusion did not silence internal-link: %+v", outcome)
	}

	plainTextMermaid := scanExcluded + "policies:\n  markdown:\n    mermaid:\n      node-label-graphemes: 4\n      edge-label-graphemes: 3\n      fill-colors: [\"#FFFFFF\"]\n      edge-colors: [\"#000000\"]\n      text-colors: [\"#000000\"]\n      authoring-rule: plain-text\n"
	if outcome := application.Execute(
		scanExclusionTree(plainTextMermaid, map[string]string{
			"generated/diagram.md": "```mermaid\nflowchart LR\nA --> B\n```\n",
		}),
		args("md", "mermaid", "validate"), nil,
	); outcome.ExitCode != 0 {
		t.Fatalf("shared exclusion did not silence mermaid: %+v", outcome)
	}

	strictFrontmatter := scanExcluded + "policies:\n  markdown:\n    frontmatter:\n      surfaces:\n        - glob: \"**/*.md\"\n          require: [title]\n"
	if outcome := application.Execute(
		scanExclusionTree(strictFrontmatter, map[string]string{
			"generated/page.md": "# no front matter\n",
		}),
		args("md", "frontmatter", "validate"), nil,
	); outcome.ExitCode != 0 {
		t.Fatalf("shared exclusion did not silence frontmatter: %+v", outcome)
	}

	strictHeading := scanExcluded + "policies:\n  markdown:\n    heading-hierarchy:\n      surfaces:\n        - glob: \"**/*.md\"\n      single-h1: true\n      max-level-jump: 1\n"
	if outcome := application.Execute(
		scanExclusionTree(strictHeading, map[string]string{
			"generated/page.md": "## no level one\n",
		}),
		args("md", "heading-hierarchy", "validate"), nil,
	); outcome.ExitCode != 0 {
		t.Fatalf("shared exclusion did not silence heading-hierarchy: %+v", outcome)
	}

	strictMetadata := scanExcluded + "policies:\n  markdown:\n    metadata:\n      surfaces:\n        - glob: \"**/*.md\"\n          schema: governance\n"
	if outcome := application.Execute(
		scanExclusionTree(strictMetadata, map[string]string{
			"generated/page.md": "# no front matter\n",
		}),
		args("metadata", "validate"), nil,
	); outcome.ExitCode != 0 {
		t.Fatalf("shared exclusion did not silence metadata: %+v", outcome)
	}

	strictNaming := scanExcluded + "policies:\n  markdown:\n    naming:\n      surfaces:\n        - glob: \"**/*.md\"\n          style: kebab-case\n      exempt: []\n"
	if outcome := application.Execute(
		scanExclusionTree(strictNaming, map[string]string{
			"generated/BADNAME.md": "# shouty\n",
		}),
		args("md", "naming", "validate"), nil,
	); outcome.ExitCode != 0 {
		t.Fatalf("shared exclusion did not silence naming: %+v", outcome)
	}

	strictEmoji := scanExcluded + "policies:\n  conventions:\n    emoji:\n      prohibited:\n        - glob: \"**/*.md\"\n"
	if outcome := application.Execute(
		scanExclusionTree(strictEmoji, map[string]string{
			"generated/mood.md": "a 😀 moment\n",
		}),
		args("convention", "emoji", "validate"), nil,
	); outcome.ExitCode != 0 {
		t.Fatalf("shared exclusion did not silence emoji: %+v", outcome)
	}
}

func TestScanExcludedDirectoryIsNeverRead(t *testing.T) {
	config := scanExcluded + "policies:\n  markdown:\n    internal-link:\n      exclude-sources: []\n"
	tree := scanExclusionTree(config, map[string]string{
		"docs/a.md":         "# clean\n",
		"generated/dark.md": "[missing](missing.md)\n",
	})
	tree.unreadable["generated/dark.md"] = true
	if outcome := application.Execute(tree, args("md", "internal-link", "validate"), nil); outcome.ExitCode != 0 {
		t.Fatalf("unreadable shared-excluded file refused the run: %+v", outcome)
	}
}

func TestScanExclusionMatchesDirectoryNamesOnly(t *testing.T) {
	config := scanExcluded + "policies:\n  markdown:\n    internal-link:\n      exclude-sources: []\n"
	// A file merely named like the directory, a sibling with a shared
	// prefix, and a differently-cased directory are all still scanned:
	// the oracle matches exact directory names at any depth.
	tree := scanExclusionTree(config, map[string]string{
		"generated":                     "[missing](missing.md)\n",
		"generated-docs/a.md":           "[missing](missing.md)\n",
		"Generated/case.md":             "[missing](missing.md)\n",
		"docs/nested/generated/deep.md": "[missing](missing.md)\n",
	})
	outcome := application.Execute(tree, args("md", "internal-link", "validate"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings outside the excluded directory: %+v", outcome)
	}
	for _, want := range []string{"generated-docs/a.md", "Generated/case.md"} {
		if !strings.Contains(outcome.Stderr, want) {
			t.Fatalf("stderr misses still-scanned %q: %+v", want, outcome)
		}
	}
	if strings.Contains(outcome.Stderr, "nested/generated/deep.md") {
		t.Fatalf("nested excluded directory was scanned: %+v", outcome)
	}
}

func TestMermaidDeclaredExcludeReplacesSharedExclusions(t *testing.T) {
	config := scanExcluded + "policies:\n  markdown:\n    mermaid:\n      node-label-graphemes: 4\n      edge-label-graphemes: 3\n      fill-colors: [\"#FFFFFF\"]\n      edge-colors: [\"#000000\"]\n      text-colors: [\"#000000\"]\n      authoring-rule: plain-text\n      exclude: [\"other/**\"]\n"
	// The validator list replaces the shared one: a diagram under the
	// shared-excluded directory is inspected again, while the declared
	// exclusion stays silent.
	tree := scanExclusionTree(config, map[string]string{
		"generated/diagram.md": "```mermaid\nflowchart LR\nA --> B\n```\n",
		"other/skipped.md":     "```mermaid\nflowchart LR\nA --> B\n```\n",
	})
	outcome := application.Execute(tree, args("md", "mermaid", "validate"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("declared mermaid exclude did not replace the shared one: %+v", outcome)
	}
	if !strings.Contains(outcome.Stderr, "generated/diagram.md") {
		t.Fatalf("shared-excluded file was not re-inspected: %+v", outcome)
	}
	if strings.Contains(outcome.Stderr, "other/skipped.md") {
		t.Fatalf("declared mermaid exclusion was not honoured: %+v", outcome)
	}
}
