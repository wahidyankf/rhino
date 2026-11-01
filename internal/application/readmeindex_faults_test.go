package application_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// readmeEveryConfig declares a docs tree whose every directory needs an index.
const readmeEveryConfig = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs\n          require-direct-children: every-directory\n"

// TestReadmeIndexRefusesAMalformedDeclaration proves each malformed readme-index
// shape refuses the run with the reason its decoder names.
func TestReadmeIndexRefusesAMalformedDeclaration(t *testing.T) {
	const head = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n"
	cases := []struct {
		name   string
		body   string
		reason string
	}{
		{
			name:   "trees that are not a list name no trees",
			body:   "      trees: x\n",
			reason: "readme-index.trees names no trees",
		},
		{
			name:   "a tree that is not a mapping names no trees",
			body:   "      trees:\n        - x\n",
			reason: "readme-index.trees names no trees",
		},
		{
			name:   "a tree with an unknown key refuses by name",
			body:   "      trees:\n        - path: docs\n          bogus: 1\n",
			reason: "readme-index.trees name unknown key `bogus`",
		},
		{
			name:   "a glob exclusion is not an exact path",
			body:   "      trees:\n        - path: docs\n          exclusions: [\"generated/*\"]\n",
			reason: "README index tree paths and exclusions must be exact paths relative to the repository",
		},
		{
			name:   "annotations that are not a list name no annotation",
			body:   "      trees:\n        - path: docs\n          annotations: x\n",
			reason: "readme-index.annotations name no annotation",
		},
		{
			name:   "an annotation that is not a mapping names no annotation",
			body:   "      trees:\n        - path: docs\n          annotations:\n            - x\n",
			reason: "readme-index.annotations name no annotation",
		},
		{
			name:   "exclusions that are not a list name no path",
			body:   "      trees:\n        - path: docs\n          exclusions: x\n",
			reason: "readme-index.exclusions names no path",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newMemoryTree("/root")
			tree.files["repo-config.yml"] = head + tc.body
			outcome := application.Execute(tree, args("md", "readme-index", "validate"), nil)
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
		})
	}
}

// TestReadmeIndexReadsOnlyWhatItsTreesDeclare proves an index policy with no
// trees checks nothing, and an exclusion keeps its subtree out of the check
// without hiding a sibling directory that is not excluded.
func TestReadmeIndexReadsOnlyWhatItsTreesDeclare(t *testing.T) {
	t.Run("an index policy with no trees checks nothing", func(t *testing.T) {
		tree := newMemoryTree("/root")
		tree.files["repo-config.yml"] = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index: {}\n"
		outcome := application.Execute(tree, args("md", "readme-index", "validate"), nil)
		if outcome.ExitCode != 0 {
			t.Fatalf("an empty index policy refused: %+v", outcome)
		}
	})
	t.Run("an exclusion does not hide a sibling directory without an index", func(t *testing.T) {
		config := readmeEveryConfig + "          exclusions:\n            - generated\n"
		tree := newMemoryTree("/root")
		tree.files["repo-config.yml"] = config
		tree.files["docs/README.md"] = "# Docs\n"
		tree.files["docs/other/a.md"] = "# Other\n"
		outcome := application.Execute(tree, args("md", "readme-index", "validate"), nil)
		if outcome.ExitCode != 1 || !strings.Contains(outcome.Stdout+outcome.Stderr, "docs/other") {
			t.Fatalf("sibling without an index not reported: %+v", outcome)
		}
	})
}

// TestReadmeIndexRefusesAnUnreadableFile proves an index, a child index, or an
// annotation the walk cannot read refuses the run, naming the file it could not
// read.
func TestReadmeIndexRefusesAnUnreadableFile(t *testing.T) {
	cases := []struct {
		name   string
		config string
		files  map[string]string
		broken string
	}{
		{
			name:   "an unreadable root index refuses",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs\n",
			files:  map[string]string{"docs/README.md": "# Docs\n"},
			broken: "docs/README.md",
		},
		{
			name:   "an unreadable child index refuses under every-directory",
			config: readmeEveryConfig,
			files:  map[string]string{"docs/README.md": "# Docs\n", "docs/sub/README.md": "# Sub\n"},
			broken: "docs/sub/README.md",
		},
		{
			name: "an unreadable annotation refuses",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs\n" +
				"          annotations:\n            - path: guide.md\n              text: canon-marker\n",
			files:  map[string]string{"docs/README.md": "# Docs\n", "docs/guide.md": "canon-marker\n"},
			broken: "docs/guide.md",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newMemoryTree("/root")
			tree.files["repo-config.yml"] = tc.config
			for path, text := range tc.files {
				tree.files[path] = text
			}
			tree.unreadable[tc.broken] = true
			outcome := application.Execute(tree, args("md", "readme-index", "validate"), nil)
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.broken) {
				t.Fatalf("unreadable %s not refused: %+v", tc.broken, outcome)
			}
		})
	}
}

// TestReadmeIndexLinkTargets proves a link is checked only when it names a
// repository path: an unclosed or empty destination and a path that climbs past
// the root are not targets, and a link that climbs one level resolves against
// its parent and is checked there.
func TestReadmeIndexLinkTargets(t *testing.T) {
	cases := []struct {
		name     string
		config   string
		files    map[string]string
		exitCode int
		contains []string
	}{
		{
			name:     "an unclosed link is not a target",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs\n",
			files:    map[string]string{"docs/README.md": "# Docs\n\n[a](unclosed\n"},
			exitCode: 0,
		},
		{
			name:     "a link with no destination is not a target",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs\n",
			files:    map[string]string{"docs/README.md": "# Docs\n\n[b]( )\n"},
			exitCode: 0,
		},
		{
			name:     "a link that climbs past the root is not a target",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    readme-index:\n      trees:\n        - path: docs\n",
			files:    map[string]string{"docs/README.md": "# Docs\n\n[out](../../outside.md)\n"},
			exitCode: 0,
		},
		{
			name:     "a link that climbs one level resolves against its parent",
			config:   readmeEveryConfig,
			files:    map[string]string{"docs/README.md": "# Docs\n\n[sub](sub/README.md)\n", "docs/sub/README.md": "# Sub\n\n[up](../nowhere.md)\n"},
			exitCode: 1,
			contains: []string{"a README index link resolves to nothing in the repository", "docs/sub/README.md"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newMemoryTree("/root")
			tree.files["repo-config.yml"] = tc.config
			for path, text := range tc.files {
				tree.files[path] = text
			}
			outcome := application.Execute(tree, args("md", "readme-index", "validate"), nil)
			if outcome.ExitCode != tc.exitCode {
				t.Fatalf("exit = %d, want %d: %+v", outcome.ExitCode, tc.exitCode, outcome)
			}
			for _, fragment := range tc.contains {
				if !strings.Contains(outcome.Stdout+outcome.Stderr, fragment) {
					t.Fatalf("output lacks %q: %+v", fragment, outcome)
				}
			}
		})
	}
}

// TestChildrenOfListsTheTopLevelEntriesOfTheRoot proves the root of the tree
// lists each file and each directory once, sorted, and a nested directory
// contributes only its own first segment.
func TestChildrenOfListsTheTopLevelEntriesOfTheRoot(t *testing.T) {
	files := []string{"README.md", "docs/a.md", "docs/sub/b.md"}
	want := []string{"README.md", "docs"}
	for _, root := range []string{"", "."} {
		if got := application.ChildrenOf(files, root); !reflect.DeepEqual(got, want) {
			t.Errorf("ChildrenOf(%q) = %v, want %v", root, got, want)
		}
	}
}
