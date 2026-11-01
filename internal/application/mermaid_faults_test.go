package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// mermaidDiagram wraps one classDef or style line in a Mermaid block.
func mermaidDiagram(line string) string {
	return "```mermaid\nflowchart LR\nA[x] --> B[y]\n" + line + "\n```\n"
}

// TestMermaidRefusesAMalformedPalette proves a palette that is not a list of
// colour names refuses the run, naming the declaration it cannot read.
func TestMermaidRefusesAMalformedPalette(t *testing.T) {
	cases := []struct {
		name   string
		config string
		reason string
	}{
		{
			name:   "edge colors that are not a list name no color",
			config: strings.Replace(mermaidRendered, "edge-colors: [\"#000000\", \"#123456\"]", "edge-colors: x", 1),
			reason: "md-mermaid.edge-colors names no color",
		},
		{
			name:   "text colors that are not a list name no color",
			config: strings.Replace(mermaidRendered, "text-colors: [\"#000000\", \"#FFFFFF\"]", "text-colors: x", 1),
			reason: "md-mermaid.text-colors names no color",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := mermaidTree(tc.config, map[string]string{"docs/a.md": mermaidDiagram("A --> B")})
			outcome := application.Execute(tree, args("md", "mermaid", "validate"), nil)
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
		})
	}
}

// TestMermaidFlagsAClassDefThatLeavesThePalette proves each colour a classDef
// sets must be a declared one, and a class that sets only a text colour is
// flagged as having no shape to make legible.
func TestMermaidFlagsAClassDefThatLeavesThePalette(t *testing.T) {
	cases := []struct {
		name     string
		line     string
		contains string
	}{
		{"a text color outside the declared list is flagged", "classDef good fill:#FFFFFF,stroke:#000000,color:#111111", "text color `#111111` is not a declared text color"},
		{"a stroke outside the declared list is flagged on an outline-only class", "classDef edge stroke:#222222", "stroke `#222222` is not a declared edge color"},
		{"a class that sets only a text color is flagged", "classDef label color:#000000", "a class that sets only text color has neither a fill nor a stroke to make it legible"},
		{"a color function outside a classDef is flagged", "style A fill:rgb(1,2,3)", "color is declared outside a classDef"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := mermaidTree(mermaidRendered, map[string]string{"docs/a.md": mermaidDiagram(tc.line)})
			outcome := application.Execute(tree, args("md", "mermaid", "validate"), nil)
			if outcome.ExitCode != 1 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.contains) {
				t.Fatalf("expected finding %q: %+v", tc.contains, outcome)
			}
		})
	}
}

// TestMermaidDoesNotMeasureContrastForAColorItCannotRead proves a declared
// colour that is not a six-digit hex value, or one whose digits do not parse, is
// flagged as not comparable and never measured for contrast.
func TestMermaidDoesNotMeasureContrastForAColorItCannotRead(t *testing.T) {
	config := strings.Replace(mermaidRendered,
		"fill-colors: [\"#FFFFFF\", \"#000000\"]",
		"fill-colors: [\"red\", \"#FFF\", \"#GGGGGG\", \"#00GG00\", \"#0000GG\"]", 1)
	lines := []string{
		"classDef a fill:red,stroke:#000000,color:#000000",
		"classDef b fill:#FFF,stroke:#000000,color:#000000",
		"classDef c fill:#GGGGGG,stroke:#000000,color:#000000",
		"classDef d fill:#00GG00,stroke:#000000,color:#000000",
		"classDef e fill:#0000GG,stroke:#000000,color:#000000",
	}
	tree := mermaidTree(config, map[string]string{"docs/a.md": mermaidDiagram(strings.Join(lines, "\n"))})
	outcome := application.Execute(tree, args("md", "mermaid", "validate"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings for the uncomparable fills: %+v", outcome)
	}
	for _, fill := range []string{"red", "#FFF", "#GGGGGG", "#00GG00", "#0000GG"} {
		if !strings.Contains(outcome.Stdout+outcome.Stderr, "`fill:"+fill+"` is not a six-digit hex color") {
			t.Fatalf("fill %s not flagged as not comparable: %+v", fill, outcome)
		}
	}
	if strings.Contains(outcome.Stdout+outcome.Stderr, "contrast") {
		t.Fatalf("an uncomparable fill was measured for contrast: %+v", outcome)
	}
}

// TestMermaidRefusesAnUnreadableDiagramFile proves a Markdown file the scan
// cannot read refuses the run, rather than being skipped as clean.
func TestMermaidRefusesAnUnreadableDiagramFile(t *testing.T) {
	tree := mermaidTree(mermaidRendered, map[string]string{"docs/a.md": mermaidDiagram("A --> B")})
	tree.unreadable["docs/a.md"] = true
	outcome := application.Execute(tree, args("md", "mermaid", "validate"), nil)
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, "docs/a.md: permission denied") {
		t.Fatalf("unreadable diagram file not refused: %+v", outcome)
	}
}
