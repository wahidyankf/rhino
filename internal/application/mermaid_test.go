package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

func mermaidTree(config string, files map[string]string) *memoryTree {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = config
	for path, content := range files {
		tree.files[path] = content
	}
	return tree
}

const mermaidBase = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    mermaid:\n" +
	"      node-label-graphemes: 4\n" +
	"      edge-label-graphemes: 3\n" +
	"      fill-colors: [\"#FFFFFF\", \"#000000\"]\n" +
	"      edge-colors: [\"#000000\", \"#123456\"]\n" +
	"      text-colors: [\"#000000\", \"#FFFFFF\"]\n"

const mermaidRendered = mermaidBase + "      authoring-rule: rendered\n"

func TestExecuteMermaidCleanRenderedDiagram(t *testing.T) {
	tree := mermaidTree(mermaidRendered, map[string]string{
		"docs/a.md": "```mermaid\nflowchart LR\naccTitle: short\naccDescr: short\nA[x] --> B[y]\nclassDef good fill:#FFFFFF,stroke:#000000,color:#000000\n```\n",
	})
	outcome := application.Execute(tree, args("md", "mermaid", "validate"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("expected clean: %+v", outcome)
	}
	if !strings.Contains(outcome.Stdout, "[mermaid] checked 1 diagram, no findings") {
		t.Fatalf("summary wrong: %q", outcome.Stdout)
	}
}

func TestExecuteMermaidFindings(t *testing.T) {
	cases := []struct {
		name     string
		config   string
		files    map[string]string
		exitCode int
		contains []string
		absent   []string
	}{
		{
			name:   "plain text repositories refuse every diagram before reading it",
			config: mermaidBase + "      authoring-rule: plain-text\n",
			files: map[string]string{
				"docs/a.md": "```mermaid\nsequenceDiagram\nA->>B: hi\n```\n",
			},
			exitCode: 1,
			contains: []string{"authors conceptual diagrams in plain text", "docs/a.md:2"},
		},
		{
			name:   "undeclared types are refused when the allowlist is declared",
			config: mermaidRendered + "      allowed-types: [flowchart]\n",
			files: map[string]string{
				"docs/a.md": "```mermaid\nmindmap\n  root\n```\n",
			},
			exitCode: 1,
			contains: []string{"`mindmap` is not in the declared allowed types"},
		},
		{
			name:   "unknown syntax is skipped when no allowlist is declared",
			config: mermaidRendered,
			files: map[string]string{
				"docs/a.md": "```mermaid\nsequenceDiagram\nA->>B: hi\n```\n",
			},
			exitCode: 0,
		},
		{
			name:   "rendered diagrams without an accessible title and description are refused",
			config: mermaidRendered,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[x] --> B[y]\n```\n",
			},
			exitCode: 1,
			contains: []string{"declares no accessible title", "declares no accessible description"},
		},
		{
			name:   "a colon and a block both count as an accessible description",
			config: mermaidRendered,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\naccTitle: t\naccDescr {\nlong description here\n}\nA[x]\n```\n",
			},
			exitCode: 0,
		},
		{
			name:   "colour outside a classDef is refused wherever it appears",
			config: mermaidBase,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[x] --> B[y]\nstyle A fill:#fff\n```\n",
			},
			exitCode: 1,
			contains: []string{"color is declared outside a classDef"},
		},
		{
			name:   "a pull request number is prose once types are declared",
			config: mermaidBase + "      allowed-types: [flowchart]\n",
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA->>B: fixes PR #123\n```\n",
			},
			exitCode: 0,
			absent:   []string{"outside a classDef"},
		},
		{
			name:   "undeclared palette colours are refused by role",
			config: mermaidBase,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[x]\nclassDef bad fill:#ABCDEF,stroke:#123456,color:#000000\n```\n",
			},
			exitCode: 1,
			contains: []string{"fill `#ABCDEF` is not a declared fill color"},
		},
		{
			name:   "a filled class without stroke or text colour is incomplete",
			config: mermaidBase,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[x]\nclassDef thin fill:#FFFFFF\n```\n",
			},
			exitCode: 1,
			contains: []string{"has no stroke", "declares no text color"},
		},
		{
			name:   "a non-hex role cannot be compared with the palette",
			config: mermaidBase,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[x]\nclassDef vague fill:blue,stroke:#123456,color:#000000\n```\n",
			},
			exitCode: 1,
			contains: []string{"`fill:blue` is not a six-digit hex color"},
		},
		{
			name:   "low contrast text on a permitted pairing is measured",
			config: mermaidRendered + "      canvas-colors: []\n",
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\naccTitle: t\naccDescr: d\nA[x]\nclassDef dim fill:#FFFFFF,stroke:#000000,color:#FFFFFF\n```\n",
			},
			exitCode: 1,
			contains: []string{"below the normal-text contrast threshold", "ratio=", "threshold="},
		},
		{
			name:   "a missing default class is demanded where the renderer applies one",
			config: mermaidBase + "      require-default-class: true\n",
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[x] --> B[y]\n```\n",
			},
			exitCode: 1,
			contains: []string{"declares no `classDef default`"},
		},
		{
			name:   "a partial default class names the missing role",
			config: mermaidBase + "      require-default-class: true\n",
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[x]\nclassDef default fill:#FFFFFF,stroke:#000000\n```\n",
			},
			exitCode: 1,
			contains: []string{"sets no `color`"},
		},
		{
			name:   "a state diagram never needs a default class",
			config: mermaidBase + "      require-default-class: true\n",
			files: map[string]string{
				"docs/a.md": "```mermaid\nstateDiagram-v2\n[*] --> A\n```\n",
			},
			exitCode: 0,
		},
		{
			name:   "theme overrides are refused on their own lines",
			config: mermaidBase + "      forbid-theme-overrides: true\n",
			files: map[string]string{
				"docs/a.md": "```mermaid\n---\nconfig:\n  theme: forest\n---\n%%{init: {\"theme\": \"dark\"}}%%\nflowchart LR\nA[x]\n```\n",
			},
			exitCode: 1,
			contains: []string{"a theme override replaces the renderer"},
		},
		{
			name:   "long node labels are reported with segment and limit",
			config: mermaidBase,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[long node] --> B[y]\n```\n",
			},
			exitCode: 1,
			contains: []string{"a node label is longer than the declared limit", "segment=node", "measured=9", "limit=4"},
		},
		{
			name:   "long edge labels are reported against the edge limit",
			config: mermaidBase,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[x] -->|long edge| B[y]\n```\n",
			},
			exitCode: 1,
			contains: []string{"a edge label is longer than the declared limit", "segment=edge"},
		},
		{
			name:   "a break splits one label into two measured segments",
			config: mermaidBase,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[ab<br/>cdefgh]\n```\n",
			},
			exitCode: 1,
			contains: []string{"measured=6"},
		},
		{
			name:   "a state transition ending in a semicolon is drawn",
			config: mermaidBase,
			files: map[string]string{
				"docs/a.md": "```mermaid\nstateDiagram-v2\nA --> B;\n```\n",
			},
			exitCode: 1,
			contains: []string{"ends in a semicolon"},
		},
		{
			name:   "the accessible description is prose and never measured",
			config: mermaidRendered,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\naccTitle: t\naccDescr: a very long description here\nA[x]\n```\n",
			},
			exitCode: 0,
		},
		{
			name:   "class lines apply a class in flowcharts and name a node in class diagrams",
			config: mermaidBase,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[x]\nclass A important\n```\n\n```mermaid\nclassDiagram\nclass VeryLongClass {\n```\n",
			},
			exitCode: 1,
			contains: []string{"a node label is longer than the declared limit"},
		},
		{
			name:   "excluded sources are skipped but stay valid link targets",
			config: mermaidBase + "      exclude: [\"generated/**\"]\n",
			files: map[string]string{
				"generated/a.md": "```mermaid\nflowchart LR\nA[long node]\n```\n",
				"docs/a.md":      "```mermaid\nflowchart LR\nA[x]\n```\n",
			},
			exitCode: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := mermaidTree(tc.config, tc.files)
			outcome := application.Execute(tree, args("md", "mermaid", "validate"), nil)
			if outcome.ExitCode != tc.exitCode {
				t.Fatalf("exit %d want %d: %+v", outcome.ExitCode, tc.exitCode, outcome)
			}
			for _, want := range tc.contains {
				if !strings.Contains(outcome.Stderr, want) {
					t.Fatalf("stderr misses %q: %+v", want, outcome)
				}
			}
			for _, miss := range tc.absent {
				if strings.Contains(outcome.Stderr, miss) || strings.Contains(outcome.Stdout, miss) {
					t.Fatalf("output holds %q: %+v", miss, outcome)
				}
			}
		})
	}
}

func TestExecuteMermaidParserHelpers(t *testing.T) {
	flow := "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    mermaid:\n" +
		"      node-label-graphemes: 4\n" +
		"      edge-label-graphemes: 3\n" +
		"      fill-colors: []\n" +
		"      edge-colors: []\n" +
		"      text-colors: []\n" +
		"      allowed-types: [flowchart, graph, classDiagram, classDiagram-v2, stateDiagram, stateDiagram-v2, erDiagram, requirementDiagram, block, block-beta]\n"
	cases := []struct {
		name     string
		config   string
		files    map[string]string
		exitCode int
		contains []string
		absent   []string
	}{
		{
			name:   "every supported declaration is read with its own label syntax",
			config: strings.Replace(flow, "node-label-graphemes: 4", "node-label-graphemes: 80", 1),
			files: map[string]string{
				"docs/a.md": "```mermaid\ngraph LR\nA[x]\n```\n\n" +
					"```mermaid\nclassDiagram-v2\nclass Ok {\n```\n\n" +
					"```mermaid\nstateDiagram\nA --> B\nstate \"oops\n```\n\n" +
					"```mermaid\nblock-beta\nA[x]\n```\n",
			},
			exitCode: 0,
		},
		{
			name:   "entity and requirement kinds read their names as labels",
			config: flow,
			files: map[string]string{
				"docs/a.md": "```mermaid\nerDiagram\nVERY_LONG_ENTITY ||--|| OTHER_ENTITY : relates\n```\n\n" +
					"```mermaid\nrequirementDiagram\nrequirement VeryLongRequirement {\n```\n",
			},
			exitCode: 1,
			contains: []string{"a node label is longer than the declared limit"},
		},
		{
			name:   "a word after an assignment is a colour outside a classDef",
			config: flow,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[x]\nstroke: red,fill: blue\n```\n",
			},
			exitCode: 1,
			contains: []string{"outside a classDef"},
		},
		{
			name:   "an assigned hex is a colour but a bare word is not",
			config: flow,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[x]\nstyle A fill:#fff\n%%{init: {'border': '#fff'}}%%\nbkg: #123\nbackground: #123\nno colon #123 here\n```\n",
			},
			exitCode: 1,
			contains: []string{"outside a classDef"},
		},
		{
			name:   "a named box colour is refused once types are declared",
			config: flow,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[x]\nbox purple Group\nbox Group\n```\n",
			},
			exitCode: 1,
			contains: []string{"outside a classDef"},
		},
		{
			name:   "a text edge writes its label between the dashes",
			config: flow,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[one] -->|two| B(three) -- four --> C{five}\nD -- long text --> E[y]\nF |unclosed rest\n```\n",
			},
			exitCode: 1,
			contains: []string{"measured=5", "segment=edge"},
		},
		{
			name:   "nested delimiters and quotes measure the inner text",
			config: mermaidBase,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[outer [inner]]\nB[\"quoted\"]\n```\n",
			},
			exitCode: 1,
			contains: []string{"measured=13", "measured=6"},
		},
		{
			name:   "markup is stripped and entities decode before measuring",
			config: mermaidBase,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[<b>AB</b>]\nB[C&#x44;]\nC[E&#69;]\nD[&lt;&gt;&quot;&apos;&amp;&nbsp;X]\n```\n",
			},
			exitCode: 1,
			contains: []string{"measured=7"},
		},
		{
			name:   "unknown and broken entities stay literal",
			config: mermaidBase,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[&unknown; &broken]\nB[&#xZZ;]\nC[&#65;]\n```\n",
			},
			exitCode: 1,
			contains: []string{"measured=17", "measured=6"},
		},
		{
			name:   "stray markup characters read as text",
			config: mermaidBase,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[a > b]\nB[a < b]\n```\n",
			},
			exitCode: 1,
			contains: []string{"measured=5"},
		},
		{
			name:   "outline and empty classDefs declare nothing to compare",
			config: mermaidBase,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[x]\nclassDef outline stroke:#123456\nclassDef empty\nclassDef incomplete missing-colon\nclassDef x:y\n```\n",
			},
			exitCode: 0,
		},
		{
			name:   "a mixed stroke and text class is judged on the stroke alone",
			config: mermaidBase,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[x]\nclassDef mixed stroke:#badbad,color:#000000\nclassDef short stroke:#xyz,color:#000000\n```\n",
			},
			exitCode: 1,
			contains: []string{"stroke `#badbad` is not a declared edge color", "`stroke:#xyz` is not a six-digit hex color"},
		},
		{
			name:   "identical fill and stroke leave no visible boundary",
			config: mermaidBase,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[x]\nclassDef same fill:#FFFFFF,stroke:#FFFFFF,color:#000000\n```\n",
			},
			exitCode: 1,
			contains: []string{"no visible boundary"},
		},
		{
			name:   "non-hex roles of the right length are still not colours",
			config: mermaidBase,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[x]\nclassDef badhex fill:#zzzzzz,stroke:#000000,color:#000000\n```\n",
			},
			exitCode: 1,
			contains: []string{"not a six-digit hex color"},
		},
		{
			name:   "a complete default class satisfies the renderer",
			config: mermaidBase + "      require-default-class: true\n",
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[x]\nclassDef default fill:#FFFFFF,stroke:#000000,color:#000000\n```\n",
			},
			exitCode: 0,
		},
		{
			name:   "a warm class stays visible on a dark canvas",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    mermaid:\n      node-label-graphemes: 4\n      edge-label-graphemes: 3\n      fill-colors: [\"#DE8F05\"]\n      edge-colors: [\"#000000\"]\n      text-colors: [\"#000000\"]\n      canvas-colors: [\"#FFFFFF\", \"#0D1117\"]\n",
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\nA[x]\nclassDef warm fill:#DE8F05,stroke:#000000,color:#000000\n```\n",
			},
			exitCode: 0,
			absent:   []string{"non-text contrast"},
		},
		{
			name:   "an opening brace without a closer starts a description block",
			config: mermaidBase,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\naccTitle { blah\nlong description here\n}\nA[x]\n```\n",
			},
			exitCode: 0,
		},
		{
			name:   "an inline braced title is a label like any other",
			config: mermaidBase,
			files: map[string]string{
				"docs/a.md": "```mermaid\nflowchart LR\naccTitle { blah }\nA[x]\n```\n",
			},
			exitCode: 1,
			contains: []string{"a node label is longer than the declared limit"},
		},
		{
			name:   "a narrowed non-markdown file is still inspected",
			config: mermaidBase,
			files: map[string]string{
				"plain.txt": "```mermaid\nflowchart LR\nA[long node]\n```\n",
			},
			exitCode: 1,
			contains: []string{"plain.txt"},
		},
		{
			name:   "a declaration sharing a keyword prefix reads its own remainder",
			config: flow,
			files: map[string]string{
				"docs/a.md": "```mermaid\nclassDiagram\nclass\n```\n\n```mermaid\nrequirementDiagram\nrequirement\n```\n",
			},
			exitCode: 1,
			contains: []string{"measured=7"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fileArgs := args("md", "mermaid", "validate")
			if tc.name == "a narrowed non-markdown file is still inspected" {
				fileArgs = args("md", "mermaid", "validate", "--file", "plain.txt")
			}
			tree := mermaidTree(tc.config, tc.files)
			outcome := application.Execute(tree, fileArgs, nil)
			if outcome.ExitCode != tc.exitCode {
				t.Fatalf("exit %d want %d: %+v", outcome.ExitCode, tc.exitCode, outcome)
			}
			for _, want := range tc.contains {
				if !strings.Contains(outcome.Stderr, want) {
					t.Fatalf("stderr misses %q: %+v", want, outcome)
				}
			}
			for _, miss := range tc.absent {
				if strings.Contains(outcome.Stderr, miss) || strings.Contains(outcome.Stdout, miss) {
					t.Fatalf("output holds %q: %+v", miss, outcome)
				}
			}
		})
	}
}

func TestExecuteMermaidDecodeRefusals(t *testing.T) {
	base := func(body string) string {
		return "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    mermaid:\n" + body
	}
	cases := []struct {
		name   string
		config string
		stderr []string
	}{
		{"non-string rule is refused", base("      authoring-rule: 42\n      node-label-graphemes: 4\n      edge-label-graphemes: 3\n      fill-colors: []\n      edge-colors: []\n      text-colors: []\n"), []string{"authoring-rule"}},
		{"non-integer node limit is refused", base("      node-label-graphemes: wide\n      edge-label-graphemes: 3\n      fill-colors: []\n      edge-colors: []\n      text-colors: []\n"), []string{"node-label-graphemes"}},
		{"negative edge limit is refused", base("      node-label-graphemes: 4\n      edge-label-graphemes: -1\n      fill-colors: []\n      edge-colors: []\n      text-colors: []\n"), []string{"edge-label-graphemes"}},
		{"non-list fills are refused", base("      node-label-graphemes: 4\n      edge-label-graphemes: 3\n      fill-colors: none\n      edge-colors: []\n      text-colors: []\n"), []string{"fill-colors"}},
		{"non-string fill is refused", base("      node-label-graphemes: 4\n      edge-label-graphemes: 3\n      fill-colors: [42]\n      edge-colors: []\n      text-colors: []\n"), []string{"fill-colors"}},
		{"non-list exclude is refused", base("      node-label-graphemes: 4\n      edge-label-graphemes: 3\n      fill-colors: []\n      edge-colors: []\n      text-colors: []\n      exclude: docs\n"), []string{"md-mermaid.exclude"}},
		{"non-string exclusion is refused", base("      node-label-graphemes: 4\n      edge-label-graphemes: 3\n      fill-colors: []\n      edge-colors: []\n      text-colors: []\n      exclude: [42]\n"), []string{"md-mermaid.exclude"}},
		{"non-boolean default class is refused", base("      node-label-graphemes: 4\n      edge-label-graphemes: 3\n      fill-colors: []\n      edge-colors: []\n      text-colors: []\n      require-default-class: 42\n"), []string{"require-default-class"}},
		{"non-list allowlist is refused", base("      node-label-graphemes: 4\n      edge-label-graphemes: 3\n      fill-colors: []\n      edge-colors: []\n      text-colors: []\n      allowed-types: flowchart\n"), []string{"allowed-types"}},
		{"non-string allowed type is refused", base("      node-label-graphemes: 4\n      edge-label-graphemes: 3\n      fill-colors: []\n      edge-colors: []\n      text-colors: []\n      allowed-types: [42]\n"), []string{"allowed-types"}},
		{"non-boolean theme forbid is refused", base("      node-label-graphemes: 4\n      edge-label-graphemes: 3\n      fill-colors: []\n      edge-colors: []\n      text-colors: []\n      forbid-theme-overrides: 42\n"), []string{"forbid-theme-overrides"}},
		{"non-list canvas is refused", base("      node-label-graphemes: 4\n      edge-label-graphemes: 3\n      fill-colors: []\n      edge-colors: []\n      text-colors: []\n      canvas-colors: white\n"), []string{"canvas-colors"}},
		{"non-string canvas is refused", base("      node-label-graphemes: 4\n      edge-label-graphemes: 3\n      fill-colors: []\n      edge-colors: []\n      text-colors: []\n      canvas-colors: [42]\n"), []string{"canvas-colors"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome := application.Execute(mermaidTree(tc.config, nil),
				args("md", "mermaid", "validate"), nil)
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

func TestExecuteMermaidPlainTextCountsUnknown(t *testing.T) {
	tree := mermaidTree(mermaidBase+"      authoring-rule: plain-text\n", map[string]string{
		"docs/a.md": "```mermaid\nsequenceDiagram\nA->>B: hi\n```\n\n```mermaid\nmindmap\n  root\n```\n",
	})
	outcome := application.Execute(tree, args("md", "mermaid", "validate"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	if !strings.Contains(outcome.Stdout, "[mermaid] checked 2 diagrams, 2 findings") {
		t.Fatalf("inspected count wrong: %q", outcome.Stdout)
	}
}

func TestExecuteMermaidCountsDiagramsNotFiles(t *testing.T) {
	tree := mermaidTree(mermaidBase, map[string]string{
		"docs/a.md": "```mermaid\nflowchart LR\nA[x]\n```\n\n```mermaid\nflowchart LR\nB[y]\n```\n",
		"plain.txt": "```mermaid\nflowchart LR\nC[long node]\n```\n",
		"docs/b.md": "# prose\n",
	})
	outcome := application.Execute(tree, args("md", "mermaid", "validate"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("expected clean: %+v", outcome)
	}
	if !strings.Contains(outcome.Stdout, "[mermaid] checked 2 diagrams, no findings") {
		t.Fatalf("inspected count wrong: %q", outcome.Stdout)
	}
	if strings.Contains(outcome.Stdout, "plain.txt") {
		t.Fatalf("non-markdown source was scanned: %q", outcome.Stdout)
	}
}

func TestExecuteMermaidKindsInJSON(t *testing.T) {
	tree := mermaidTree(mermaidRendered, map[string]string{
		"docs/a.md": "```mermaid\nflowchart LR\nA[long node] --> B[y]\n```\n",
	})
	outcome := application.Execute(tree, args("md", "mermaid", "validate", "--output", "json"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	for _, want := range []string{
		"\"kind\":\"mermaid-legibility\"", "\"kind\":\"mermaid-accessibility\"",
		"\"subject\":\"diagram\"", "\"inspected\":1",
		"\"segment\":\"node\"", "\"measured\":9", "\"limit\":4",
	} {
		if !strings.Contains(outcome.Stdout, want) {
			t.Fatalf("stdout misses %q: %+v", want, outcome)
		}
	}
	plain := application.Execute(mermaidTree(mermaidBase+"      authoring-rule: plain-text\n", map[string]string{
		"docs/a.md": "```mermaid\nflowchart LR\nA[x]\n```\n",
	}), args("md", "mermaid", "validate", "--output", "json"), nil)
	if plain.ExitCode != 1 || !strings.Contains(plain.Stdout, "\"kind\":\"diagram-authoring-rule\"") {
		t.Fatalf("authoring kind missing: %+v", plain)
	}
}

func TestExecuteMermaidUnreadableFiles(t *testing.T) {
	tree := mermaidTree(mermaidBase, map[string]string{"docs/a.md": "```mermaid\nflowchart LR\nA[x]\n```\n"})
	tree.unreadable["docs/a.md"] = true
	if outcome := application.Execute(tree, args("md", "mermaid", "validate"), nil); outcome.ExitCode != 2 {
		t.Fatalf("unreadable source did not refuse: %+v", outcome)
	}
	binary := mermaidTree(mermaidBase, map[string]string{"docs/a.md": "bytes"})
	binary.binary["docs/a.md"] = true
	if outcome := application.Execute(binary, args("md", "mermaid", "validate"), nil); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "holds no text") {
		t.Fatalf("binary source did not refuse: %+v", outcome)
	}
}

func TestExecuteMermaidSelection(t *testing.T) {
	files := map[string]string{
		"docs/a.md": "```mermaid\nflowchart LR\nA[long node]\n```\n",
		"docs/b.md": "```mermaid\nflowchart LR\nB[x]\n```\n",
	}
	narrowed := application.Execute(mermaidTree(mermaidBase, files),
		args("md", "mermaid", "validate", "--file", "docs/a.md"), nil)
	if narrowed.ExitCode != 1 || !strings.Contains(narrowed.Stderr, "docs/a.md") {
		t.Fatalf("selection was not narrowed: %+v", narrowed)
	}
	if strings.Contains(narrowed.Stderr, "docs/b.md") {
		t.Fatalf("unselected file was read: %+v", narrowed)
	}
	skipped := application.Execute(mermaidTree(mermaidBase+"      exclude: [\"docs/a.md\"]\n", files),
		args("md", "mermaid", "validate", "--file", "docs/a.md"), nil)
	if skipped.ExitCode != 0 {
		t.Fatalf("excluded selection did not skip: %+v", skipped)
	}
	stdin := "```mermaid\nflowchart LR\nA[long node]\n```\n"
	typed := application.Execute(mermaidTree(mermaidBase, nil),
		args("md", "mermaid", "validate", "--file", "-"), &stdin)
	if typed.ExitCode != 1 || !strings.Contains(typed.Stderr, "-:3") {
		t.Fatalf("stdin diagram was not inspected: %+v", typed)
	}
	missing := application.Execute(mermaidTree(mermaidBase, files),
		args("md", "mermaid", "validate", "--file", "docs/gone.md"), nil)
	if missing.ExitCode != 2 || !strings.Contains(missing.Stderr, "does not exist") {
		t.Fatalf("missing selection did not refuse: %+v", missing)
	}
	linked := mermaidTree(mermaidBase, files)
	linked.links["docs/a.md"] = true
	if outcome := application.Execute(linked,
		args("md", "mermaid", "validate", "--file", "docs/a.md"), nil); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "symbolic link") {
		t.Fatalf("linked selection did not refuse: %+v", outcome)
	}
	if outcome := application.Execute(mermaidTree(mermaidBase, files),
		args("md", "mermaid", "validate", "--file", "-"), nil); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "cannot be read") {
		t.Fatalf("missing stdin did not refuse: %+v", outcome)
	}
}

func TestExecuteMermaidRefusals(t *testing.T) {
	cases := []struct {
		name   string
		config string
		stderr []string
	}{
		{
			name:   "undeclared policy is refused at its owner",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown: {}\n",
			stderr: []string{"policies.markdown.mermaid"},
		},
		{
			name:   "unknown policy key is refused",
			config: mermaidBase + "      surfaces: []\n",
			stderr: []string{"unknown key"},
		},
		{
			name:   "missing limits are refused",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    mermaid:\n      fill-colors: []\n      edge-colors: []\n      text-colors: []\n",
			stderr: []string{"node-label-graphemes"},
		},
		{
			name:   "unknown authoring rule is refused",
			config: mermaidBase + "      authoring-rule: ascii\n",
			stderr: []string{"authoring-rule"},
		},
		{
			name:   "malformed exclude glob is refused",
			config: mermaidBase + "      exclude: [\"[\"]\n",
			stderr: []string{"md-mermaid.exclude"},
		},
		{
			name:   "absolute exclude glob is refused",
			config: mermaidBase + "      exclude: [\"/abs/**\"]\n",
			stderr: []string{"md-mermaid.exclude", "relative to the repository root"},
		},
		{
			name:   "non-hex canvas is refused",
			config: mermaidBase + "      canvas-colors: [white]\n",
			stderr: []string{"md-mermaid.canvas-colors", "six-digit hex"},
		},
		{
			name:   "invisible class on a declared canvas reports ratio and canvas",
			config: mermaidRendered + "      canvas-colors: [\"#FFFFFF\", \"#0D1117\"]\n",
			stderr: []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "invisible class on a declared canvas reports ratio and canvas" {
				tree := mermaidTree(tc.config, map[string]string{
					"docs/a.md": "```mermaid\nflowchart LR\naccTitle: t\naccDescr: d\nA[x]\nclassDef shy fill:#000000,stroke:#000000,color:#FFFFFF\n```\n",
				})
				outcome := application.Execute(tree, args("md", "mermaid", "validate"), nil)
				if outcome.ExitCode != 1 {
					t.Fatalf("expected findings: %+v", outcome)
				}
				for _, want := range []string{"`#0D1117`", "ratio=", "canvas="} {
					if !strings.Contains(outcome.Stderr, want) {
						t.Fatalf("stderr misses %q: %+v", want, outcome)
					}
				}
				return
			}
			outcome := application.Execute(mermaidTree(tc.config, nil),
				args("md", "mermaid", "validate"), nil)
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
