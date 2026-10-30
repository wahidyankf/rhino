package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

func layersTree(config string, files map[string]string) *memoryTree {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = config
	for path, content := range files {
		tree.files[path] = content
	}
	return tree
}

const layersBase = "schema: rhino/repo-config/v2\npolicies:\n  governance:\n    layers:\n"

func TestExecuteLayersCleanAndFindings(t *testing.T) {
	cases := []struct {
		name     string
		config   string
		files    map[string]string
		exitCode int
		contains []string
		absent   []string
	}{
		{
			name:   "a layer tree matching its declared order and categories reports clean",
			config: layersBase + "      root: policy\n      order: [foundation, practice]\n      categories:\n        foundation: [shared]\n        practice: [guides]\n",
			files: map[string]string{
				"policy/foundation/shared/item.md": "# Item\n",
				"policy/practice/guides/item.md":   "# Item\n",
			},
			exitCode: 0,
			contains: []string{"[layers] checked 2 governance directories, no findings"},
		},
		{
			name:     "an empty layer policy over an empty root reports clean with zero inspected",
			config:   layersBase + "      root: governance\n      order: []\n",
			files:    map[string]string{"elsewhere/note.md": "note\n"},
			exitCode: 0,
			contains: []string{"[layers] checked 0 governance directories, no findings"},
		},
		{
			name:   "a declared layer with no directory is reported missing",
			config: layersBase + "      root: policy\n      order: [foundation, practice]\n      categories:\n        foundation: [shared]\n",
			files: map[string]string{
				"policy/foundation/shared/item.md": "# Item\n",
			},
			exitCode: 1,
			contains: []string{
				"[layers] checked 1 governance directory, 1 finding",
				"policy/practice: a declared governance layer is absent",
			},
		},
		{
			name:   "a declared category with no directory is reported missing",
			config: layersBase + "      root: policy\n      order: [foundation]\n      categories:\n        foundation: [shared]\n",
			files: map[string]string{
				"policy/foundation/README.md": "# Item\n",
			},
			exitCode: 1,
			contains: []string{
				"policy/foundation/shared: a declared governance category is absent",
			},
		},
		{
			name:   "a directory under the root outside the declared order is reported unexpected",
			config: layersBase + "      root: policy\n      order: [foundation]\n      categories:\n        foundation: [shared]\n",
			files: map[string]string{
				"policy/foundation/shared/item.md": "# Item\n",
				"policy/extra/local/item.md":       "# Item\n",
			},
			exitCode: 1,
			contains: []string{
				"[layers] checked 2 governance directories, 1 finding",
				"policy/extra: the governance layer is not declared",
			},
		},
		{
			name:   "a file directly under the root reads as a layer that is not declared",
			config: layersBase + "      root: policy\n      order: [foundation]\n",
			files: map[string]string{
				"policy/foundation/shared/item.md": "# Item\n",
				"policy/notes.md":                  "# Item\n",
			},
			exitCode: 1,
			contains: []string{
				"policy/notes.md: the governance layer is not declared",
			},
		},
		{
			name:   "a directory inside a layer outside its declared categories is reported unexpected",
			config: layersBase + "      root: governance\n      order: [principles]\n      categories:\n        principles: []\n",
			files: map[string]string{
				"governance/principles/custom/README.md": "synthetic\n",
			},
			exitCode: 1,
			contains: []string{
				"governance/principles/custom: the governance category is not declared for its layer",
			},
			absent: []string{"missing-governance"},
		},
		{
			name:   "a missing layer and an unexpected layer are reported together",
			config: layersBase + "      root: governance\n      order: [principles, workflows]\n      categories:\n        principles: [general]\n",
			files: map[string]string{
				"governance/principles/README.md": "present\n",
				"governance/extra/README.md":      "unexpected\n",
			},
			exitCode: 1,
			contains: []string{
				"a declared governance layer is absent",
				"a declared governance category is absent",
				"the governance layer is not declared",
			},
		},
		{
			name:   "a file directly inside a layer is not a category and stays silent",
			config: layersBase + "      root: policy\n      order: [foundation]\n      categories:\n        foundation: [shared]\n",
			files: map[string]string{
				"policy/foundation/notes.md":       "# Item\n",
				"policy/foundation/shared/item.md": "# Item\n",
			},
			exitCode: 0,
			contains: []string{"[layers] checked 1 governance directory, no findings"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := layersTree(tc.config, tc.files)
			outcome := application.Execute(tree, args("governance", "layers", "validate"), nil)
			if outcome.ExitCode != tc.exitCode {
				t.Fatalf("expected exit %d: %+v", tc.exitCode, outcome)
			}
			for _, want := range tc.contains {
				if !strings.Contains(outcome.Stdout+outcome.Stderr, want) {
					t.Fatalf("expected %q in %+v", want, outcome)
				}
			}
			for _, want := range tc.absent {
				if strings.Contains(outcome.Stdout+outcome.Stderr, want) {
					t.Fatalf("expected %q absent from %+v", want, outcome)
				}
			}
		})
	}
}

func TestExecuteLayersJSONCarriesKinds(t *testing.T) {
	tree := layersTree(
		layersBase+"      root: policy\n      order: [foundation]\n      categories:\n        foundation: [shared]\n",
		map[string]string{
			"policy/foundation/shared/item.md": "# Item\n",
			"policy/extra/local/item.md":       "# Item\n",
		},
	)
	outcome := application.Execute(tree, args("governance", "layers", "validate", "--output", "json"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	for _, want := range []string{
		`"command":"layers"`,
		`"subject":"governance directory"`,
		`"exitCode":1`,
		`"inspected":2`,
		`"kind":"unexpected-governance-layer"`,
		`"path":"policy/extra"`,
		`"message":"the governance layer is not declared"`,
	} {
		if !strings.Contains(outcome.Stdout, want) {
			t.Fatalf("expected %q in %q", want, outcome.Stdout)
		}
	}
	if outcome.Stderr != "" {
		t.Fatalf("json findings must leave stderr empty: %+v", outcome)
	}
}

func TestExecuteLayersMissingCategoryJSON(t *testing.T) {
	tree := layersTree(
		layersBase+"      root: policy\n      order: [foundation]\n      categories:\n        foundation: [shared]\n",
		map[string]string{"policy/foundation/README.md": "# Item\n"},
	)
	outcome := application.Execute(tree, args("governance", "layers", "validate", "--output", "json"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	for _, want := range []string{
		`"kind":"missing-governance-category"`,
		`"path":"policy/foundation/shared"`,
		`"message":"a declared governance category is absent"`,
	} {
		if !strings.Contains(outcome.Stdout, want) {
			t.Fatalf("expected %q in %q", want, outcome.Stdout)
		}
	}
}

func TestExecuteLayersCleanJSON(t *testing.T) {
	tree := layersTree(
		layersBase+"      root: policy\n      order: [foundation, practice]\n      categories:\n        foundation: [shared]\n        practice: [guides]\n",
		map[string]string{
			"policy/foundation/shared/item.md": "# Item\n",
			"policy/practice/guides/item.md":   "# Item\n",
		},
	)
	outcome := application.Execute(tree, args("governance", "layers", "validate", "--output", "json"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("expected clean: %+v", outcome)
	}
	for _, want := range []string{
		`"command":"layers"`,
		`"exitCode":0`,
		`"inspected":2`,
		`"violations":[]`,
	} {
		if !strings.Contains(outcome.Stdout, want) {
			t.Fatalf("expected %q in %q", want, outcome.Stdout)
		}
	}
}

func TestExecuteLayersRefusals(t *testing.T) {
	cases := []struct {
		name     string
		config   string
		files    map[string]string
		links    []string
		exitCode int
		contains []string
	}{
		{
			name:     "an undeclared layers section refuses rather than defaulting",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  governance: {}\n",
			exitCode: 2,
			contains: []string{"policies.governance.layers is not declared"},
		},
		{
			name:     "a climbing layer root refuses the whole run",
			config:   layersBase + "      root: ../governance\n      order: []\n",
			exitCode: 2,
			contains: []string{"a layer root and every layer name must be exact repository-relative names"},
		},
		{
			name:     "an absolute layer root refuses the whole run",
			config:   layersBase + "      root: /governance\n      order: []\n",
			exitCode: 2,
			contains: []string{"a layer root and every layer name must be exact repository-relative names"},
		},
		{
			name:     "a glob layer root refuses the whole run",
			config:   layersBase + "      root: \"governance/*\"\n      order: []\n",
			exitCode: 2,
			contains: []string{"a layer root and every layer name must be exact repository-relative names"},
		},
		{
			name:     "a non-simple layer name refuses the whole run",
			config:   layersBase + "      root: governance\n      order: [Not-simple, Not-simple]\n",
			exitCode: 2,
			contains: []string{"a layer root and every layer name must be exact repository-relative names"},
		},
		{
			name:     "a repeated layer name refuses the whole run",
			config:   layersBase + "      root: governance\n      order: [principles, principles]\n",
			exitCode: 2,
			contains: []string{"layer order and categories must use unique exact simple names"},
		},
		{
			name: "a category outside the declared order refuses the whole run",
			config: layersBase + "      root: governance\n      order: [principles]\n" +
				"      categories:\n        missing: [general]\n",
			exitCode: 2,
			contains: []string{"layer order and categories must use unique exact simple names"},
		},
		{
			name: "a non-simple category name refuses the whole run",
			config: layersBase + "      root: governance\n      order: [principles]\n" +
				"      categories:\n        principles: [../outside]\n",
			exitCode: 2,
			contains: []string{"layer order and categories must use unique exact simple names"},
		},
		{
			name: "an underscore category name refuses the whole run",
			config: layersBase + "      root: governance\n      order: [principles]\n" +
				"      categories:\n        principles: [bad_name]\n",
			exitCode: 2,
			contains: []string{"layer order and categories must use unique exact simple names"},
		},
		{
			name:     "a layer root behind a symbolic link refuses rather than being skipped",
			config:   layersBase + "      root: governance\n      order: []\n",
			files:    map[string]string{"governance/held.md": "held\n"},
			links:    []string{"governance"},
			exitCode: 2,
			contains: []string{"governance: passes through a symbolic link, which RHINO never follows"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := layersTree(tc.config, tc.files)
			for _, link := range tc.links {
				tree.links[link] = true
			}
			outcome := application.Execute(tree, args("governance", "layers", "validate"), nil)
			if outcome.ExitCode != tc.exitCode {
				t.Fatalf("expected exit %d: %+v", tc.exitCode, outcome)
			}
			for _, want := range tc.contains {
				if !strings.Contains(outcome.Stdout+outcome.Stderr, want) {
					t.Fatalf("expected %q in %+v", want, outcome)
				}
			}
		})
	}
}
