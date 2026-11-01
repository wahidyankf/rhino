package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// TestDirectoryMapRefusesMalformedTreeDeclarations proves each malformed tree
// list refuses the whole run, so a declared tree is never silently skipped.
func TestDirectoryMapRefusesMalformedTreeDeclarations(t *testing.T) {
	cases := []struct {
		name   string
		config string
		reason string
	}{
		{
			name:   "a scan section that is not a mapping refuses the exclusion list",
			config: "schema: rhino/repo-config/v2\nscan: 3\npolicies:\n  governance:\n    directory-map:\n      trees:\n        - path: docs\n",
			reason: "scan.exclude-directories names no directory",
		},
		{
			name:   "a tree item that is a bare scalar names no tree",
			config: directoryMapBase + "      trees:\n        - docs\n",
			reason: "directory-map.trees names no trees",
		},
		{
			name:   "a tree entry with an unknown key refuses rather than being ignored",
			config: directoryMapBase + "      trees:\n        - path: docs\n          bogus: x\n",
			reason: "directory-map.trees name unknown key `bogus`",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := directoryMapTree(tc.config, map[string]string{
				"docs/README.md": "# Docs\n\n## Directory Map\n",
			})
			outcome := application.Execute(tree, directoryMapArgs(""), nil)
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
		})
	}
}

// TestDirectoryMapWithNoTreesInspectsNothing proves a present but null tree
// list is no declaration of trees: the run inspects zero directories and is clean.
func TestDirectoryMapWithNoTreesInspectsNothing(t *testing.T) {
	tree := directoryMapTree(directoryMapBase+"      trees:\n", map[string]string{
		"docs/README.md": "# Docs\n\n## Directory Map\n",
	})
	outcome := application.Execute(tree, directoryMapArgs(""), nil)
	if outcome.ExitCode != 0 || !strings.Contains(outcome.Stdout, "checked 0 directories, no findings") {
		t.Fatalf("null tree list was not a clean empty walk: %+v", outcome)
	}
}

// TestDirectoryMapMapEntryParsing proves the map reader takes only the one
// entry form, stops at the next top-level heading, and rejects every target
// that is not a direct sibling. Each malformed entry is reported as itself, or
// ignored, and never resolves to a sibling.
func TestDirectoryMapMapEntryParsing(t *testing.T) {
	cases := []struct {
		name     string
		readme   string
		exitCode int
		contains []string
		absent   []string
	}{
		{
			name:     "a plain list item without a link is ignored, so its sibling is missing",
			readme:   "# Docs\n\n## Directory Map\n- guide.md\n",
			exitCode: 1,
			contains: []string{"missing map entry for `docs/guide.md`"},
			absent:   []string{"invalid map entry"},
		},
		{
			name:     "a link missing its closing parenthesis is ignored, so its sibling is missing",
			readme:   "# Docs\n\n## Directory Map\n- [Guide](guide.md\n",
			exitCode: 1,
			contains: []string{"missing map entry for `docs/guide.md`"},
			absent:   []string{"invalid map entry"},
		},
		{
			name:     "a heading of the first level ends the map before its entries",
			readme:   "# Docs\n\n## Directory Map\n- [Guide](guide.md)\n# Later\n- [Ghost](ghost.md)\n",
			exitCode: 0,
			contains: []string{"checked 1 directory, no findings"},
			absent:   []string{"ghost.md"},
		},
		{
			name:     "an absolute target is an invalid entry",
			readme:   "# Docs\n\n## Directory Map\n- [Abs](/abs.md)\n- [Guide](guide.md)\n",
			exitCode: 1,
			contains: []string{"invalid map entry `/abs.md`"},
		},
		{
			name:     "an empty target is an invalid entry",
			readme:   "# Docs\n\n## Directory Map\n- [Empty]()\n- [Guide](guide.md)\n",
			exitCode: 1,
			contains: []string{"invalid map entry ``"},
		},
		{
			name:     "a URI target is an invalid entry",
			readme:   "# Docs\n\n## Directory Map\n- [Web](https://example.com)\n- [Guide](guide.md)\n",
			exitCode: 1,
			contains: []string{"invalid map entry `https://example.com`"},
		},
		{
			name:     "a target that starts with a colon is an invalid entry",
			readme:   "# Docs\n\n## Directory Map\n- [Colon](:x)\n- [Guide](guide.md)\n",
			exitCode: 1,
			contains: []string{"invalid map entry `:x`"},
		},
		{
			name:     "a target whose scheme starts with a digit is a relative path, and is invalid",
			readme:   "# Docs\n\n## Directory Map\n- [Digit](1abc:y)\n- [Guide](guide.md)\n",
			exitCode: 1,
			contains: []string{"invalid map entry `1abc:y`"},
		},
		{
			name:     "a target whose scheme holds a space is a relative path, and is invalid",
			readme:   "# Docs\n\n## Directory Map\n- [Space](a b:c)\n- [Guide](guide.md)\n",
			exitCode: 1,
			contains: []string{"invalid map entry `a b:c`"},
		},
		{
			name:     "a target that is only current-directory segments names nothing",
			readme:   "# Docs\n\n## Directory Map\n- [Dot](./)\n- [Guide](guide.md)\n",
			exitCode: 1,
			contains: []string{"invalid map entry `./`"},
		},
		{
			name:     "a target two levels below a sibling is invalid, not resolved through it",
			readme:   "# Docs\n\n## Directory Map\n- [Deep](guide.md/extra.md)\n- [Guide](guide.md)\n",
			exitCode: 1,
			contains: []string{"invalid map entry `guide.md/extra.md`"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := directoryMapTree(directoryMapBase+"      trees:\n        - path: docs\n", map[string]string{
				"docs/README.md": tc.readme,
				"docs/guide.md":  "# Guide\n",
			})
			outcome := application.Execute(tree, directoryMapArgs(""), nil)
			if outcome.ExitCode != tc.exitCode {
				t.Fatalf("expected exit %d: %+v", tc.exitCode, outcome)
			}
			for _, want := range tc.contains {
				if !strings.Contains(outcome.Stdout+outcome.Stderr, want) {
					t.Fatalf("expected %q in %+v", want, outcome)
				}
			}
			for _, unwanted := range tc.absent {
				if strings.Contains(outcome.Stdout+outcome.Stderr, unwanted) {
					t.Fatalf("did not expect %q in %+v", unwanted, outcome)
				}
			}
		})
	}
}
