package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
	"github.com/wahidyankf/rhino/internal/domain"
)

func readmeTree(config string, files map[string]string) *memoryTree {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = config
	for path, content := range files {
		tree.files[path] = content
	}
	return tree
}

const readmeRootConfig = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs\n"

const readmeStrictConfig = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs\n          require-direct-children: true\n"

const readmeCompleteConfig = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs\n          require-direct-children: every-directory\n"

func TestExecuteReadmeIndexFindings(t *testing.T) {
	cases := []struct {
		name     string
		config   string
		files    map[string]string
		exitCode int
		contains []string
		absent   []string
	}{
		{
			name:     "an index with no child requirement passes",
			config:   readmeRootConfig,
			files:    map[string]string{"docs/README.md": "# Docs\n"},
			exitCode: 0,
		},
		{
			name:     "a declared directory without an index is reported",
			config:   readmeRootConfig,
			files:    map[string]string{"docs/guide.md": "# Guide\n"},
			exitCode: 1,
			contains: []string{"the declared directory has no README index", "docs/README.md", "checked 1 directory, 1 finding"},
		},
		{
			name:     "an unlinked direct child is reported against the index",
			config:   readmeStrictConfig,
			files:    map[string]string{"docs/README.md": "# Docs\n", "docs/guide.md": "# Guide\n"},
			exitCode: 1,
			contains: []string{"the declared README index omits a direct child", "child=docs/guide.md"},
		},
		{
			name:     "a linked direct child passes",
			config:   readmeStrictConfig,
			files:    map[string]string{"docs/README.md": "# Docs\n\n[guide](guide.md)\n", "docs/guide.md": "# Guide\n"},
			exitCode: 0,
		},
		{
			name:     "a linked child through a fragment passes",
			config:   readmeStrictConfig,
			files:    map[string]string{"docs/README.md": "# Docs\n\n[guide](guide.md#part)\n", "docs/guide.md": "# Guide\n"},
			exitCode: 0,
		},
		{
			name:     "a non-markdown child needs no link",
			config:   readmeCompleteConfig,
			files:    map[string]string{"docs/README.md": "# Docs\n", "docs/logo.png": "bytes"},
			exitCode: 0,
		},
		{
			name:   "a complete tree with linked children passes",
			config: readmeCompleteConfig,
			files: map[string]string{
				"docs/README.md":     "# Docs\n\n[sub](sub)\n",
				"docs/sub/a.md":      "# A\n",
				"docs/sub/README.md": "# Sub\n\n[a](a.md)\n",
			},
			exitCode: 0,
			contains: []string{"checked 2 directories, no findings"},
		},
		{
			name:   "a nested directory without an index is reported",
			config: readmeCompleteConfig,
			files: map[string]string{
				"docs/README.md": "# Docs\n\n[sub](sub)\n",
				"docs/sub/a.md":  "# A\n",
			},
			exitCode: 1,
			contains: []string{"a directory in the declared tree has no README index", "docs/sub/README.md"},
		},
		{
			name:   "a nested index omitting its child is reported",
			config: readmeCompleteConfig,
			files: map[string]string{
				"docs/README.md":     "# Docs\n\n[sub](sub)\n",
				"docs/sub/README.md": "# Sub\n",
				"docs/sub/a.md":      "# A\n",
			},
			exitCode: 1,
			contains: []string{"a README index in the declared tree omits a direct child", "child=docs/sub/a.md"},
		},
		{
			name:   "a dangling index link is reported with its target",
			config: readmeCompleteConfig,
			files: map[string]string{
				"docs/README.md": "# Docs\n\n[gone](gone.md)\n",
			},
			exitCode: 1,
			contains: []string{"a README index link resolves to nothing in the repository", "target=docs/gone.md"},
		},
		{
			name:   "an absolute index link is not a repository target",
			config: readmeCompleteConfig,
			files: map[string]string{
				"docs/README.md": "# Docs\n\n[root](/other.md)\n",
			},
			exitCode: 0,
		},
		{
			name:   "an external index link is not a repository target",
			config: readmeCompleteConfig,
			files: map[string]string{
				"docs/README.md": "# Docs\n\n[site](https://example.com)\n",
			},
			exitCode: 0,
		},
		{
			name:   "a subdirectory linked through its index passes",
			config: readmeCompleteConfig,
			files: map[string]string{
				"docs/README.md":     "# Docs\n\n[sub](sub/README.md)\n",
				"docs/sub/README.md": "# Sub\n",
			},
			exitCode: 0,
		},
		{
			name:   "a subdirectory with a present sibling document passes",
			config: readmeCompleteConfig,
			files: map[string]string{
				"docs/README.md":     "# Docs\n\n[sub-doc](sub.md)\n",
				"docs/sub.md":        "# Sub\n",
				"docs/sub/README.md": "# Sub index\n",
			},
			exitCode: 0,
		},
		{
			name:     "a declared annotation present in its readme passes",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs\n          annotations:\n            - path: README.md\n              text: canon-marker\n",
			files:    map[string]string{"docs/README.md": "# Docs\n\ncanon-marker\n"},
			exitCode: 0,
		},
		{
			name:     "a declared annotation missing from its readme is reported",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs\n          annotations:\n            - path: README.md\n              text: canon-marker\n",
			files:    map[string]string{"docs/README.md": "# Docs\n"},
			exitCode: 1,
			contains: []string{"the configured README annotation is absent", "docs/README.md"},
		},
		{
			name:     "a declared annotation file that is absent is reported",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs\n          annotations:\n            - path: guide.md\n              text: canon-marker\n",
			files:    map[string]string{"docs/README.md": "# Docs\n"},
			exitCode: 1,
			contains: []string{"the configured annotation file is absent", "docs/guide.md"},
		},
		{
			name:     "a missing index skips its annotations",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs\n          annotations:\n            - path: README.md\n              text: canon-marker\n",
			files:    map[string]string{},
			exitCode: 1,
			contains: []string{"the declared directory has no README index"},
			absent:   []string{"annotation"},
		},
		{
			name:   "an excluded subtree needs no index and no child link",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs\n          require-direct-children: every-directory\n          exclusions:\n            - generated\n",
			files: map[string]string{
				"docs/README.md":      "# Docs\n",
				"docs/generated/a.md": "# Generated\n",
			},
			exitCode: 0,
		},
		{
			name:   "a scan-excluded directory needs no index",
			config: "schema: rhino/repo-config/v2\nscan:\n  exclude-directories:\n    - generated\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs\n          require-direct-children: every-directory\n",
			files: map[string]string{
				"docs/README.md":      "# Docs\n",
				"docs/generated/a.md": "# Generated\n",
			},
			exitCode: 0,
		},
		{
			name:     "a glob tree path is refused",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs/*\n",
			files:    map[string]string{"docs/README.md": "# Docs\n"},
			exitCode: 2,
			contains: []string{"README index tree paths and exclusions must be exact paths relative to the repository"},
		},
		{
			name:     "an annotation outside the tree shape is refused",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs\n          annotations:\n            - path: ../outside.md\n              text: canon-marker\n",
			files:    map[string]string{"docs/README.md": "# Docs\n"},
			exitCode: 2,
			contains: []string{"README annotations must name exact paths relative to their declared tree"},
		},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			tree := readmeTree(row.config, row.files)
			outcome := application.Execute(tree, args("md", "readme-index", "validate"), nil)
			if outcome.ExitCode != row.exitCode {
				t.Fatalf("exit %d, want %d: stdout %q stderr %q", outcome.ExitCode, row.exitCode, outcome.Stdout, outcome.Stderr)
			}
			for _, want := range row.contains {
				if !strings.Contains(outcome.Stdout+outcome.Stderr, want) {
					t.Fatalf("missing %q in stdout %q stderr %q", want, outcome.Stdout, outcome.Stderr)
				}
			}
			for _, missing := range row.absent {
				if strings.Contains(outcome.Stdout+outcome.Stderr, missing) {
					t.Fatalf("unexpected %q in stdout %q stderr %q", missing, outcome.Stdout, outcome.Stderr)
				}
			}
		})
	}
}

func TestExecuteReadmeIndexDecodeRefusals(t *testing.T) {
	cases := []struct {
		name     string
		config   string
		contains string
	}{
		{
			name:     "an unknown policy key is refused",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      unknown: []\n",
			contains: "names unknown key `unknown`",
		},
		{
			name:     "a non-list trees is refused",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees: docs\n",
			contains: "readme-index.trees names no trees",
		},
		{
			name:     "an unknown tree key is refused",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs\n          unknown: 1\n",
			contains: "readme-index.trees name unknown key `unknown`",
		},
		{
			name:     "an unknown child mode is refused",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs\n          require-direct-children: sometimes\n",
			contains: "must be a boolean or `every-directory`",
		},
		{
			name:     "an annotation without text is refused",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs\n          annotations:\n            - path: README.md\n",
			contains: "readme-index.annotations name no text",
		},
		{
			name:     "an annotation with an unknown key is refused",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs\n          annotations:\n            - path: README.md\n              text: x\n              unknown: y\n",
			contains: "readme-index.annotations name unknown key `unknown`",
		},
		{
			name:     "a non-string exclusion is refused",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs\n          exclusions:\n            - 7\n",
			contains: "readme-index.exclusions names no path",
		},
		{
			name:     "a malformed scan section is refused",
			config:   "schema: rhino/repo-config/v2\nscan: generated\npolicies:\n  markdown:\n    readme-index:\n      trees: []\n",
			contains: "scan.exclude-directories names no directory",
		},
		{
			name:     "an unknown scan key is refused",
			config:   "schema: rhino/repo-config/v2\nscan:\n  unknown: []\npolicies:\n  markdown:\n    readme-index:\n      trees: []\n",
			contains: "scan names unknown key `unknown`",
		},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			tree := readmeTree(row.config, map[string]string{})
			outcome := application.Execute(tree, args("md", "readme-index", "validate"), nil)
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, row.contains) {
				t.Fatalf("not refused with %q: %+v", row.contains, outcome)
			}
		})
	}
}

func TestExecuteReadmeIndexRefusals(t *testing.T) {
	undeclared := newMemoryTree("/root")
	undeclared.files["repo-config.yml"] = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces: []\n"
	outcome := application.Execute(undeclared, args("md", "readme-index", "validate"), nil)
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "policies.markdown.readme-index is not declared") {
		t.Fatalf("undeclared policy did not fail closed: %+v", outcome)
	}

	jsonOutcome := application.Execute(readmeTree(readmeRootConfig, map[string]string{}), args("md", "readme-index", "validate", "--output", "json"), nil)
	if jsonOutcome.ExitCode != 1 || !strings.Contains(jsonOutcome.Stdout, `"command":"readme-index"`) {
		t.Fatalf("json shape broken: %+v", jsonOutcome)
	}
	if !strings.Contains(jsonOutcome.Stdout, `"subject":"directory"`) {
		t.Fatalf("json subject broken: %+v", jsonOutcome)
	}

	binary := readmeTree(readmeRootConfig, map[string]string{"docs/README.md": "# Docs\n"})
	binary.binary["docs/README.md"] = true
	if outcome := application.Execute(binary, args("md", "readme-index", "validate"), nil); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "holds no text") {
		t.Fatalf("binary index did not fail closed: %+v", outcome)
	}

	unreadable := readmeTree(readmeRootConfig, map[string]string{"docs/README.md": "# Docs\n"})
	unreadable.unreadable["docs/README.md"] = true
	if outcome := application.Execute(unreadable, args("md", "readme-index", "validate"), nil); outcome.ExitCode != 2 {
		t.Fatalf("unreadable index did not fail closed: %+v", outcome)
	}

	if outcome := application.Execute(readmeTree(readmeRootConfig, map[string]string{"docs/README.md": "# Docs\n"}), args("md", "readme-index", "validate", "--file", "docs/README.md"), nil); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "`--file` is not accepted") {
		t.Fatalf("leaf flag was not refused: %+v", outcome)
	}

	_ = domain.ConfigUnusable
}
