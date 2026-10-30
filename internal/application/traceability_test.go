package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

func traceabilityTree(config string, files map[string]string) *memoryTree {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = config
	for path, content := range files {
		tree.files[path] = content
	}
	return tree
}

const traceabilityBase = "schema: rhino/repo-config/v2\npolicies:\n  governance:\n    traceability:\n"

const traceabilityLinked = traceabilityBase +
	"      artifacts:\n" +
	"        - id: source\n" +
	"          path: docs/a.md\n" +
	"        - id: target\n" +
	"          path: docs/b.md\n" +
	"      relationships:\n" +
	"        - from: source\n" +
	"          to: target\n"

func TestExecuteTraceabilityCleanAndFindings(t *testing.T) {
	cases := []struct {
		name     string
		config   string
		files    map[string]string
		exitCode int
		contains []string
		absent   []string
	}{
		{
			name:   "linked artifacts report clean and list every artifact scanned",
			config: traceabilityLinked,
			files: map[string]string{
				"docs/a.md": "# Source\n\nSee [target](b.md).\n",
				"docs/b.md": "# Target\n",
			},
			exitCode: 0,
			contains: []string{
				"[traceability] checked 2 artifacts, no findings",
				"[traceability] scanned docs/a.md",
				"[traceability] scanned docs/b.md",
			},
		},
		{
			name:     "an empty traceability policy checks no artifacts and reports clean",
			config:   traceabilityBase + "      artifacts: []\n",
			files:    map[string]string{"docs/a.md": "# Source\n"},
			exitCode: 0,
			contains: []string{"[traceability] checked 0 artifacts, no findings"},
		},
		{
			name:   "a missing target is reported absent and its relationship unlinked",
			config: traceabilityLinked,
			files: map[string]string{
				"docs/a.md": "# Source\n\nNo links here.\n",
			},
			exitCode: 1,
			contains: []string{
				"[traceability] checked 1 artifact, 2 findings",
				"docs/b.md: the declared traceability artifact is absent",
				"the declared artifact relationship has no matching local link",
			},
		},
		{
			name:   "a source linking its missing target reports only the absence",
			config: traceabilityLinked,
			files: map[string]string{
				"docs/a.md": "# Source\n\nSee [target](b.md).\n",
			},
			exitCode: 1,
			contains: []string{
				"[traceability] checked 1 artifact, 1 finding",
				"docs/b.md: the declared traceability artifact is absent",
			},
			absent: []string{"missing-traceability-relationship"},
		},
		{
			name:   "a missing source is reported absent without an unlinked relationship",
			config: traceabilityLinked,
			files: map[string]string{
				"docs/b.md": "# Target\n",
			},
			exitCode: 1,
			contains: []string{
				"docs/a.md: the declared traceability artifact is absent",
			},
			absent: []string{"missing-traceability-relationship"},
		},
		{
			name:   "an unlinked source is reported with the target it does not link",
			config: traceabilityLinked,
			files: map[string]string{
				"docs/a.md": "# Source\n\nNo links here.\n",
				"docs/b.md": "# Target\n",
			},
			exitCode: 1,
			contains: []string{
				"[traceability] checked 2 artifacts, 1 finding",
				"docs/a.md: the declared artifact relationship has no matching local link (target=docs/b.md)",
			},
		},
		{
			name:   "a link through a fragment still counts as a local link",
			config: traceabilityLinked,
			files: map[string]string{
				"docs/a.md": "# Source\n\nSee [target](b.md#section).\n",
				"docs/b.md": "# Target\n",
			},
			exitCode: 0,
			contains: []string{"[traceability] checked 2 artifacts, no findings"},
		},
		{
			name:   "an absolute link does not satisfy a declared relationship",
			config: traceabilityLinked,
			files: map[string]string{
				"docs/a.md": "# Source\n\nSee [target](/docs/b.md).\n",
				"docs/b.md": "# Target\n",
			},
			exitCode: 1,
			contains: []string{
				"the declared artifact relationship has no matching local link",
			},
		},
		{
			name:   "an external link does not satisfy a declared relationship",
			config: traceabilityLinked,
			files: map[string]string{
				"docs/a.md": "# Source\n\nSee [target](https://example.com/b.md).\n",
				"docs/b.md": "# Target\n",
			},
			exitCode: 1,
			contains: []string{
				"the declared artifact relationship has no matching local link",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := traceabilityTree(tc.config, tc.files)
			outcome := application.Execute(tree, args("governance", "traceability", "validate"), nil)
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

func TestExecuteTraceabilityFindingsJSON(t *testing.T) {
	tree := traceabilityTree(traceabilityLinked, map[string]string{
		"docs/a.md": "# Source\n\nNo links here.\n",
		"docs/b.md": "# Target\n",
	})
	outcome := application.Execute(tree, args("governance", "traceability", "validate", "--output", "json"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	for _, want := range []string{
		`"command":"traceability"`,
		`"subject":"artifact"`,
		`"exitCode":1`,
		`"inspected":2`,
		`"kind":"missing-traceability-relationship"`,
		`"path":"docs/a.md"`,
		`"message":"the declared artifact relationship has no matching local link"`,
		`"target":"docs/b.md"`,
	} {
		if !strings.Contains(outcome.Stdout, want) {
			t.Fatalf("expected %q in %q", want, outcome.Stdout)
		}
	}
	if outcome.Stderr != "" {
		t.Fatalf("json findings must leave stderr empty: %+v", outcome)
	}
}

func TestExecuteTraceabilityCleanJSON(t *testing.T) {
	tree := traceabilityTree(traceabilityLinked, map[string]string{
		"docs/a.md": "# Source\n\nSee [target](b.md).\n",
		"docs/b.md": "# Target\n",
	})
	outcome := application.Execute(tree, args("governance", "traceability", "validate", "--output", "json"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("expected clean: %+v", outcome)
	}
	for _, want := range []string{
		`"command":"traceability"`,
		`"exitCode":0`,
		`"inspected":2`,
		`"violations":[]`,
	} {
		if !strings.Contains(outcome.Stdout, want) {
			t.Fatalf("expected %q in %q", want, outcome.Stdout)
		}
	}
}

func TestExecuteTraceabilityRefusals(t *testing.T) {
	cases := []struct {
		name     string
		config   string
		files    map[string]string
		unread   []string
		binary   []string
		exitCode int
		contains []string
	}{
		{
			name:     "an undeclared traceability section refuses rather than defaulting",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  governance: {}\n",
			exitCode: 2,
			contains: []string{"policies.governance.traceability is not declared, and RHINO holds no default for it"},
		},
		{
			name: "a non-simple artifact identifier refuses the whole run",
			config: traceabilityBase + "      artifacts:\n" +
				"        - id: Bad_Id\n" +
				"          path: docs/a.md\n",
			exitCode: 2,
			contains: []string{"traceability artifacts require unique simple identifiers and exact repository-relative paths"},
		},
		{
			name: "a climbing artifact path refuses the whole run",
			config: traceabilityBase + "      artifacts:\n" +
				"        - id: source\n" +
				"          path: ../outside.md\n",
			exitCode: 2,
			contains: []string{"traceability artifacts require unique simple identifiers and exact repository-relative paths"},
		},
		{
			name: "an absolute artifact path refuses the whole run",
			config: traceabilityBase + "      artifacts:\n" +
				"        - id: source\n" +
				"          path: /docs/a.md\n",
			exitCode: 2,
			contains: []string{"traceability artifacts require unique simple identifiers and exact repository-relative paths"},
		},
		{
			name: "a glob artifact path refuses the whole run",
			config: traceabilityBase + "      artifacts:\n" +
				"        - id: source\n" +
				"          path: docs/*.md\n",
			exitCode: 2,
			contains: []string{"traceability artifacts require unique simple identifiers and exact repository-relative paths"},
		},
		{
			name: "a repeated artifact identifier refuses the whole run",
			config: traceabilityBase + "      artifacts:\n" +
				"        - id: source\n" +
				"          path: docs/a.md\n" +
				"        - id: source\n" +
				"          path: docs/b.md\n",
			exitCode: 2,
			contains: []string{"traceability artifact identifiers must not repeat"},
		},
		{
			name: "a relationship from an undeclared artifact refuses the whole run",
			config: traceabilityBase + "      artifacts:\n" +
				"        - id: source\n" +
				"          path: docs/a.md\n" +
				"      relationships:\n" +
				"        - from: ghost\n" +
				"          to: source\n",
			exitCode: 2,
			contains: []string{"every traceability relationship must name declared artifacts"},
		},
		{
			name: "a relationship to an undeclared artifact refuses the whole run",
			config: traceabilityBase + "      artifacts:\n" +
				"        - id: source\n" +
				"          path: docs/a.md\n" +
				"      relationships:\n" +
				"        - from: source\n" +
				"          to: ghost\n",
			exitCode: 2,
			contains: []string{"every traceability relationship must name declared artifacts"},
		},
		{
			name:     "an unreadable artifact fails closed with its path",
			config:   traceabilityLinked,
			files:    map[string]string{"docs/a.md": "# Source\n"},
			unread:   []string{"docs/a.md"},
			exitCode: 2,
			contains: []string{"docs/a.md"},
		},
		{
			name:     "a binary artifact fails closed as holding no text",
			config:   traceabilityLinked,
			files:    map[string]string{"docs/a.md": "bytes"},
			binary:   []string{"docs/a.md"},
			exitCode: 2,
			contains: []string{"docs/a.md: holds no text"},
		},
		{
			name:     "an unknown traceability key refuses rather than being ignored",
			config:   traceabilityBase + "      artifacts: []\n      bogus: true\n",
			exitCode: 2,
			contains: []string{"policies.governance.traceability names unknown key `bogus`"},
		},
		{
			name:     "a scalar artifacts list refuses rather than reading as one artifact",
			config:   traceabilityBase + "      artifacts: oops\n",
			exitCode: 2,
			contains: []string{"traceability.artifacts names no artifacts"},
		},
		{
			name:     "a scalar relationships list refuses rather than reading as one relationship",
			config:   traceabilityBase + "      relationships: oops\n",
			exitCode: 2,
			contains: []string{"traceability.relationships names no relationships"},
		},
		{
			name:     "an undeclared traceability section reports as JSON when asked",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  governance: {}\n",
			exitCode: 2,
			contains: []string{`"code":"rhino.config.undeclared"`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := traceabilityTree(tc.config, tc.files)
			for _, path := range tc.unread {
				tree.unreadable[path] = true
			}
			for _, path := range tc.binary {
				tree.binary[path] = true
			}
			format := args("governance", "traceability", "validate")
			if strings.Contains(tc.name, "as JSON") {
				format = args("governance", "traceability", "validate", "--output", "json")
			}
			outcome := application.Execute(tree, format, nil)
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
