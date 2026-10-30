package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

func metadataTree(config string, files map[string]string) *memoryTree {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = config
	for path, content := range files {
		tree.files[path] = content
	}
	return tree
}

const metadataAgentConfig = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    metadata:\n      surfaces:\n        - glob: \"agents/*.md\"\n          schema: agent\n"

const metadataAgentDoc = "---\nname: example\ndescription: A complete portable agent description that is long enough.\nwhen_to_use: >-\n  Use this agent when its declared task is the current task.\ntier: execution\ncapabilities:\n  - repository-read\n---\n"

func TestExecuteMetadataCleanAcrossSchemas(t *testing.T) {
	cases := []struct {
		name   string
		config string
		files  map[string]string
	}{
		{
			name:   "valid agent passes",
			config: metadataAgentConfig,
			files:  map[string]string{"agents/example.md": metadataAgentDoc},
		},
		{
			name:   "valid governance passes without a name",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    metadata:\n      surfaces:\n        - glob: \"docs/*.md\"\n          schema: governance\n",
			files:  map[string]string{"docs/governance.md": "---\ndescription: A portable governance description that is long enough here.\nwhen_to_use: >-\n  Use this governance when its declared scope is the current scope.\n---\n"},
		},
		{
			name:   "valid workflow passes with its stem as its name",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    metadata:\n      surfaces:\n        - glob: \"workflows/*.md\"\n          schema: workflow\n",
			files:  map[string]string{"workflows/release.md": "---\nname: release\ndescription: A portable workflow description that is long enough to pass.\nwhen_to_use: >\n  Use this workflow when the declared delivery action is needed.\n---\n"},
		},
		{
			name:   "valid workflow readme takes its directory as its name",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    metadata:\n      surfaces:\n        - glob: \"workflows/*/README.md\"\n          schema: workflow\n",
			files:  map[string]string{"workflows/release/README.md": "---\nname: release\ndescription: A portable workflow description that is long enough to pass.\nwhen_to_use: >\n  Use this workflow when the declared delivery action is needed.\n---\n"},
		},
		{
			name:   "workflow readme match ignores case",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    metadata:\n      surfaces:\n        - glob: \"workflows/*/readme.md\"\n          schema: workflow\n",
			files:  map[string]string{"workflows/release/readme.md": "---\nname: release\ndescription: A portable workflow description that is long enough to pass.\nwhen_to_use: >\n  Use this workflow when the declared delivery action is needed.\n---\n"},
		},
		{
			name:   "valid skill passes with its directory as its name",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    metadata:\n      surfaces:\n        - glob: \"skills/*/SKILL.md\"\n          schema: skill\n",
			files:  map[string]string{"skills/example-skill/SKILL.md": "---\nname: example-skill\ndescription: A complete portable skill description that is long enough.\nwhen_to_use: >\n  Use this skill when its declared behaviour is needed.\ncompatibility: supported\n---\n"},
		},
		{
			name:   "last matching surface governs",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    metadata:\n      surfaces:\n        - glob: \"agents/*.md\"\n          schema: agent\n        - glob: \"agents/lead.md\"\n          schema: governance\n",
			files:  map[string]string{"agents/lead.md": "---\ndescription: A portable governance description that is long enough here.\nwhen_to_use: >-\n  Use this governance when its declared scope is the current scope.\n---\n"},
		},
		{
			name:   "files outside every surface are not inspected",
			config: metadataAgentConfig,
			files:  map[string]string{"docs/free.md": "---\n---\n"},
		},
		{
			name:   "non-markdown files are not inspected",
			config: metadataAgentConfig,
			files:  map[string]string{"agents/notes.txt": "---\n---\n"},
		},
		{
			name:   "quoted values and plus-marked folded scalars read as the same values",
			config: metadataAgentConfig,
			files:  map[string]string{"agents/example.md": "---\nname: example\ndescription: 'A complete portable agent description that is long enough.'\nwhen_to_use: >+\n  Use this agent when its declared task is the current task.\ntier: execution\ncapabilities:\n  - repository-read\n---\n"},
		},
		{
			name:   "compatibility holds no value rules on its skill schema",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    metadata:\n      surfaces:\n        - glob: \"skills/*/SKILL.md\"\n          schema: skill\n",
			files:  map[string]string{"skills/example-skill/SKILL.md": "---\nname: example-skill\ndescription: A complete portable skill description that is long enough.\nwhen_to_use: >\n  Use this skill when its declared behaviour is needed.\ncompatibility: |\n  kept verbatim\n---\n"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome := application.Execute(metadataTree(tc.config, tc.files),
				args("metadata", "validate"), nil)
			if outcome.ExitCode != 0 {
				t.Fatalf("expected clean: %+v", outcome)
			}
			if !strings.Contains(outcome.Stdout, "[metadata] checked") {
				t.Fatalf("summary missing: %q", outcome.Stdout)
			}
		})
	}
}

func TestExecuteMetadataAgentReportsEachStructuralAndValueFault(t *testing.T) {
	tree := metadataTree(metadataAgentConfig, map[string]string{
		"agents/wrong-name.md": "---\nwhen_to_use: plain routing text. It has too many sentences. One. Two. Three.\ndescription: |\n  short\ndescription: duplicate description that deliberately has sufficient length\nname: Wrong_Name\ntier: imaginary\ncapabilities:\n  - shell\n  - repository-read\n  - unknown\n  - unknown\nskills: []\nconstraints: scalar\nunknown: null\norphaned value\n---\n",
	})
	outcome := application.Execute(tree, args("metadata", "validate"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	for _, want := range []string{
		"metadata-frontmatter-malformed",
		"metadata-duplicate-key",
		"metadata-unknown-key",
		"metadata-key-order",
		"metadata-description-form",
		"metadata-description-length",
		"metadata-folded-scalar-required",
		"metadata-when-to-use-sentences",
		"metadata-name-format",
		"metadata-tier-unknown",
		"metadata-array-order",
		"metadata-capability-unknown",
		"metadata-array-duplicate",
		"metadata-array-empty",
		"metadata-array-required",
		"metadata-null-value",
	} {
		if !strings.Contains(outcome.Stderr, want) {
			t.Fatalf("stderr misses %q: %q", want, outcome.Stderr)
		}
	}
	// Structural findings carry line and column 1 with the field they are about.
	if !strings.Contains(outcome.Stderr, "agents/wrong-name.md:2:1 metadata-folded-scalar-required when_to_use") {
		t.Fatalf("structural position or field wrong: %q", outcome.Stderr)
	}
	if !strings.Contains(outcome.Stderr, "(first at line 3)") {
		t.Fatalf("duplicate omits its first line: %q", outcome.Stderr)
	}
}

func TestExecuteMetadataOrderMissingAndBlocks(t *testing.T) {
	cases := []struct {
		name     string
		files    map[string]string
		exitCode int
		contains []string
		absent   []string
	}{
		{
			name:     "transposed keys report one order finding",
			files:    map[string]string{"agents/example.md": "---\ndescription: A complete portable agent description that is long enough.\nname: example\nwhen_to_use: >-\n  Use this agent when its declared task is the current task.\ntier: execution\ncapabilities:\n  - repository-read\n---\n"},
			exitCode: 1,
			contains: []string{"metadata-key-order", "agents/example.md:3:1"},
			absent:   []string{"metadata-required-key-missing"},
		},
		{
			name:     "absent block misses every required key at line one",
			files:    map[string]string{"docs/governance.md": "# prose\n"},
			exitCode: 1,
			contains: []string{"metadata-required-key-missing", "docs/governance.md:1:1"},
		},
		{
			name:     "a fence further down is prose not front matter",
			files:    map[string]string{"agents/example.md": "prose\n---\nname: example\n---\n"},
			exitCode: 1,
			contains: []string{"metadata-required-key-missing", "agents/example.md:1:1"},
		},
		{
			name:     "unterminated block reports the fence not the keys",
			files:    map[string]string{"agents/example.md": "---\nname: example"},
			exitCode: 1,
			contains: []string{"metadata-frontmatter-unterminated", "agents/example.md:1:1"},
			absent:   []string{"metadata-required-key-missing"},
		},
		{
			name:     "missing keys report at the block line below leading blanks",
			files:    map[string]string{"agents/example.md": "\n---\nname: example\n---\n"},
			exitCode: 1,
			contains: []string{"agents/example.md:2:1 metadata-required-key-missing description"},
		},
		{
			name:     "orphaned continuations and keyless lines are malformed",
			files:    map[string]string{"agents/example.md": "---\nname: example\ndescription: A complete portable agent description that is long enough.\nwhen_to_use: >-\n  Use this agent when its declared task is the current task.\ntier: execution\ncapabilities:\n  - repository-read\n  orphaned\n- dashed\n---\n"},
			exitCode: 1,
			contains: []string{"metadata-frontmatter-malformed", "agents/example.md:9:1", "agents/example.md:10:1"},
		},
		{
			name:     "dispatch list after constraints passes in canonical order",
			files:    map[string]string{"agents/lead.md": "---\nname: lead\ndescription: Leads the work by dispatching each part to a teammate.\nwhen_to_use: >-\n  Use when a goal spans several agents and needs one coordinator.\ntier: plan\ncapabilities:\n  - repository-read\nconstraints:\n  - inline-result-only\ndispatches:\n  - writer\n  - tester\n---\n"},
			exitCode: 0,
		},
		{
			name:     "dispatch list before constraints breaks canonical order",
			files:    map[string]string{"agents/lead.md": "---\nname: lead\ndescription: Leads the work by dispatching each part to a teammate.\nwhen_to_use: >-\n  Use when a goal spans several agents and needs one coordinator.\ntier: plan\ncapabilities:\n  - repository-read\ndispatches:\n  - writer\nconstraints:\n  - inline-result-only\n---\n"},
			exitCode: 1,
			contains: []string{"metadata-key-order"},
		},
		{
			name:     "scalar dispatch list is refused as an array",
			files:    map[string]string{"agents/lead.md": "---\nname: lead\ndescription: Leads the work by dispatching each part to a teammate.\nwhen_to_use: >-\n  Use when a goal spans several agents and needs one coordinator.\ntier: plan\ncapabilities:\n  - repository-read\ndispatches: writer\n---\n"},
			exitCode: 1,
			contains: []string{"metadata-array-required"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := metadataAgentConfig
			if _, ok := tc.files["docs/governance.md"]; ok {
				config = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    metadata:\n      surfaces:\n        - glob: \"docs/*.md\"\n          schema: governance\n"
			}
			outcome := application.Execute(metadataTree(config, tc.files),
				args("metadata", "validate"), nil)
			if outcome.ExitCode != tc.exitCode {
				t.Fatalf("exit %d want %d: %+v", outcome.ExitCode, tc.exitCode, outcome)
			}
			for _, want := range tc.contains {
				if !strings.Contains(outcome.Stderr, want) {
					t.Fatalf("stderr misses %q: %q", want, outcome.Stderr)
				}
			}
			for _, miss := range tc.absent {
				if strings.Contains(outcome.Stderr, miss) {
					t.Fatalf("stderr holds %q: %q", miss, outcome.Stderr)
				}
			}
		})
	}
}

func TestExecuteMetadataValueRules(t *testing.T) {
	agentDoc := func(body string) map[string]string {
		return map[string]string{"agents/example.md": "---\n" + body + "---\n"}
	}
	valid := "name: example\ndescription: A complete portable agent description that is long enough.\nwhen_to_use: >-\n  Use this agent when its declared task is the current task.\ntier: execution\ncapabilities:\n  - repository-read\n"
	cases := []struct {
		name     string
		body     string
		exitCode int
		contains []string
		absent   []string
	}{
		{name: "canonical agent passes", body: valid, exitCode: 0},
		{
			name:     "short description is rejected with its count",
			body:     "name: example\ndescription: short\nwhen_to_use: >-\n  Use this agent when its declared task is the current task.\ntier: execution\ncapabilities:\n  - repository-read\n",
			exitCode: 1,
			contains: []string{"metadata-description-length", "is 5 characters; this field holds 20 through 300"},
		},
		{
			name:     "plain trigger is rejected as a folded scalar",
			body:     "name: example\ndescription: A complete portable agent description that is long enough.\nwhen_to_use: Use this agent when its declared task is the current task.\ntier: execution\ncapabilities:\n  - repository-read\n",
			exitCode: 1,
			contains: []string{"metadata-folded-scalar-required"},
		},
		{
			name:     "fourth sentence is rejected with its count",
			body:     "name: example\ndescription: A complete portable agent description that is long enough.\nwhen_to_use: >-\n  Use this agent! Is it needed? Yes. Now.\ntier: execution\ncapabilities:\n  - repository-read\n",
			exitCode: 1,
			contains: []string{"metadata-when-to-use-sentences", "carries 4 sentences; a routing trigger states at most 3"},
		},
		{
			name:     "trigger repeating the description adds no routing",
			body:     "name: example\ndescription: A portable description that is long enough to satisfy the shared bound.\nwhen_to_use: >\n  A portable description that is long enough to satisfy the shared bound.\ntier: execution\ncapabilities:\n  - repository-read\n",
			exitCode: 1,
			contains: []string{"metadata-routing-not-distinct"},
		},
		{
			name:     "uppercase name is rejected as a format",
			body:     "name: Wrong_Name\ndescription: A complete portable agent description that is long enough.\nwhen_to_use: >-\n  Use this agent when its declared task is the current task.\ntier: execution\ncapabilities:\n  - repository-read\n",
			exitCode: 1,
			contains: []string{"metadata-name-format"},
			absent:   []string{"metadata-name-path-mismatch"},
		},
		{
			name:     "well-formed name from another path is rejected as a mismatch",
			body:     "name: other\ndescription: A complete portable agent description that is long enough.\nwhen_to_use: >-\n  Use this agent when its declared task is the current task.\ntier: execution\ncapabilities:\n  - repository-read\n",
			exitCode: 1,
			contains: []string{"metadata-name-path-mismatch"},
		},
		{
			name:     "unknown tier is rejected",
			body:     "name: example\ndescription: A complete portable agent description that is long enough.\nwhen_to_use: >-\n  Use this agent when its declared task is the current task.\ntier: imaginary\ncapabilities:\n  - repository-read\n",
			exitCode: 1,
			contains: []string{"metadata-tier-unknown"},
		},
		{
			name:     "list description is rejected as a scalar",
			body:     "name: example\ndescription:\n  - one\nwhen_to_use: >-\n  Use this agent when its declared task is the current task.\ntier: execution\ncapabilities:\n  - repository-read\n",
			exitCode: 1,
			contains: []string{"metadata-scalar-required"},
		},
		{
			name:     "scalar capabilities are rejected as an array",
			body:     "name: example\ndescription: A complete portable agent description that is long enough.\nwhen_to_use: >-\n  Use this agent when its declared task is the current task.\ntier: execution\ncapabilities: shell\n",
			exitCode: 1,
			contains: []string{"metadata-array-required"},
		},
		{
			name:     "empty capabilities are rejected as an absent answer",
			body:     "name: example\ndescription: A complete portable agent description that is long enough.\nwhen_to_use: >-\n  Use this agent when its declared task is the current task.\ntier: execution\ncapabilities: []\n",
			exitCode: 1,
			contains: []string{"metadata-array-empty"},
		},
		{
			name:     "duplicate skill members are rejected once each",
			body:     "name: example\ndescription: A complete portable agent description that is long enough.\nwhen_to_use: >-\n  Use this agent when its declared task is the current task.\ntier: execution\ncapabilities:\n  - repository-read\nskills:\n  - one\n  - one\n",
			exitCode: 1,
			contains: []string{"metadata-array-duplicate", "lists `one` twice"},
		},
		{
			name:     "scalar constraints are rejected as an array",
			body:     valid + "constraints: scalar\n",
			exitCode: 1,
			contains: []string{"metadata-array-required"},
		},
		{
			name:     "tilde null is rejected as no answer",
			body:     valid + "unknown: ~\n",
			exitCode: 1,
			contains: []string{"metadata-null-value", "metadata-unknown-key"},
		},
		{
			name:     "quoted empty is rejected as no value",
			body:     valid + "constraints: \"\"\n",
			exitCode: 1,
			contains: []string{"metadata-empty-value"},
		},
		{
			name:     "bare empty is rejected as no value",
			body:     "name: example\ndescription:\nwhen_to_use: >-\n  Use this agent when its declared task is the current task.\ntier: execution\ncapabilities:\n  - repository-read\n",
			exitCode: 1,
			contains: []string{"metadata-empty-value"},
		},
		{
			name:     "short trigger is rejected with its count",
			body:     "name: example\ndescription: A complete portable agent description that is long enough.\nwhen_to_use: >-\n  Too short.\ntier: execution\ncapabilities:\n  - repository-read\n",
			exitCode: 1,
			contains: []string{"metadata-when-to-use-length", "this field holds 20 through 240"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome := application.Execute(metadataTree(metadataAgentConfig, agentDoc(tc.body)),
				args("metadata", "validate"), nil)
			if outcome.ExitCode != tc.exitCode {
				t.Fatalf("exit %d want %d: %+v", outcome.ExitCode, tc.exitCode, outcome)
			}
			for _, want := range tc.contains {
				if !strings.Contains(outcome.Stderr, want) {
					t.Fatalf("stderr misses %q: %q", want, outcome.Stderr)
				}
			}
			for _, miss := range tc.absent {
				if strings.Contains(outcome.Stderr, miss) {
					t.Fatalf("stderr holds %q: %q", miss, outcome.Stderr)
				}
			}
		})
	}
}

func TestExecuteMetadataSurfacesRefusalsAndReads(t *testing.T) {
	cases := []struct {
		name     string
		config   string
		contains []string
	}{
		{
			name:     "undeclared policy is refused at its owner",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown: {}\n",
			contains: []string{"policies.markdown.metadata"},
		},
		{
			name:     "unknown policy key is refused",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    metadata:\n      surfaces: []\n      extra: 1\n",
			contains: []string{"unknown key"},
		},
		{
			name:     "missing surfaces are refused",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    metadata: {}\n",
			contains: []string{"metadata.surfaces"},
		},
		{
			name:     "surface without a mapping is refused",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    metadata:\n      surfaces: [\"agents/*.md\"]\n",
			contains: []string{"no surface"},
		},
		{
			name:     "unknown surface key is refused",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    metadata:\n      surfaces:\n        - glob: \"agents/*.md\"\n          schema: agent\n          style: kebab-case\n",
			contains: []string{"unknown key"},
		},
		{
			name:     "empty surface glob is refused",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    metadata:\n      surfaces:\n        - glob: \"\"\n          schema: agent\n",
			contains: []string{"no glob"},
		},
		{
			name:     "missing schema is refused",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    metadata:\n      surfaces:\n        - glob: \"agents/*.md\"\n",
			contains: []string{"no schema"},
		},
		{
			name:     "unknown schema is refused",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    metadata:\n      surfaces:\n        - glob: \"agents/*.md\"\n          schema: playbook\n",
			contains: []string{"unknown schema"},
		},
		{
			name:     "malformed surface glob is refused",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    metadata:\n      surfaces:\n        - glob: \"[\"\n          schema: agent\n",
			contains: []string{"metadata.surfaces"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome := application.Execute(metadataTree(tc.config, nil),
				args("metadata", "validate"), nil)
			if outcome.ExitCode != 2 {
				t.Fatalf("expected refusal: %+v", outcome)
			}
			if outcome.Stdout != "" {
				t.Fatalf("refusal writes no stdout: %+v", outcome)
			}
			for _, want := range tc.contains {
				if !strings.Contains(outcome.Stderr, want) {
					t.Fatalf("stderr misses %q: %+v", want, outcome)
				}
			}
		})
	}

	t.Run("binary file refuses the run", func(t *testing.T) {
		tree := metadataTree(metadataAgentConfig, map[string]string{"agents/example.md": "bytes"})
		tree.binary["agents/example.md"] = true
		if outcome := application.Execute(tree, args("metadata", "validate"), nil); outcome.ExitCode != 2 ||
			!strings.Contains(outcome.Stderr, "holds no text") {
			t.Fatalf("binary file did not refuse: %+v", outcome)
		}
	})

	t.Run("unreadable file refuses the run", func(t *testing.T) {
		tree := metadataTree(metadataAgentConfig, map[string]string{"agents/example.md": metadataAgentDoc})
		tree.unreadable["agents/example.md"] = true
		if outcome := application.Execute(tree, args("metadata", "validate"), nil); outcome.ExitCode != 2 {
			t.Fatalf("unreadable file did not refuse: %+v", outcome)
		}
	})

	t.Run("inspected count names governed files", func(t *testing.T) {
		tree := metadataTree(metadataAgentConfig, map[string]string{
			"agents/example.md": metadataAgentDoc,
			"plain.txt":         "---\n---\n",
			"outside.md":        "---\n---\n",
		})
		outcome := application.Execute(tree, args("metadata", "validate"), nil)
		if outcome.ExitCode != 0 {
			t.Fatalf("expected clean: %+v", outcome)
		}
		if !strings.Contains(outcome.Stdout, "[metadata] checked 1 file, no findings") {
			t.Fatalf("inspected count wrong: %q", outcome.Stdout)
		}
		if !strings.Contains(outcome.Stdout, "[metadata] scanned agents/example.md") {
			t.Fatalf("scanned file missing: %q", outcome.Stdout)
		}
	})
}

func TestExecuteMetadataJSONShapes(t *testing.T) {
	tree := metadataTree(metadataAgentConfig, map[string]string{
		"agents/example.md": "---\nname: example\ndescription: short\nwhen_to_use: plain trigger text that is long enough to pass the shared bound.\ntier: execution\ncapabilities:\n  - repository-read\n---\n",
	})
	outcome := application.Execute(tree, args("metadata", "validate", "--output", "json"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	for _, want := range []string{
		`"kind":"metadata-description-length"`, `"line":3`, `"column":1`, `"field":"description"`,
		`"kind":"metadata-folded-scalar-required"`, `"subject":"file"`, `"scanned":["agents/example.md"]`,
	} {
		if !strings.Contains(outcome.Stdout, want) {
			t.Fatalf("stdout misses %q: %q", want, outcome.Stdout)
		}
	}

	clean := application.Execute(
		metadataTree(metadataAgentConfig, map[string]string{"agents/example.md": metadataAgentDoc}),
		args("metadata", "validate", "--output", "json"), nil)
	if clean.ExitCode != 0 || !strings.Contains(clean.Stdout, `"exitCode":0`) {
		t.Fatalf("clean json wrong: %+v", clean)
	}

	refused := application.Execute(metadataTree("schema: rhino/repo-config/v2\n", nil),
		args("metadata", "validate", "--output", "json"), nil)
	if refused.ExitCode != 2 || !strings.Contains(refused.Stderr, "rhino.config.undeclared") {
		t.Fatalf("refusal json wrong: %+v", refused)
	}
}
