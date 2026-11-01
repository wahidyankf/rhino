package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// TestMermaidSelectionSkipsAFileThatVanishedAfterListing proves a Markdown file
// the walk listed and the read found gone is skipped, while a file that is still
// present is still judged.
func TestMermaidSelectionSkipsAFileThatVanishedAfterListing(t *testing.T) {
	tree := mermaidTree(mermaidBase, map[string]string{
		"docs/gone.md":  mermaidDiagram("classDef x fill:#123456"),
		"docs/clean.md": mermaidDiagram("A --> B"),
	})
	tree.vanished["docs/gone.md"] = true
	outcome := application.Execute(tree, args("md", "mermaid", "validate"), nil)
	if outcome.ExitCode != 0 || strings.Contains(outcome.Stdout+outcome.Stderr, "gone.md") {
		t.Fatalf("a vanished file was judged or refused: %+v", outcome)
	}
}

// TestMermaidFileArgumentThatCannotBeReadRefuses proves a file named on the
// command line that exists but cannot be read refuses the run by name.
func TestMermaidFileArgumentThatCannotBeReadRefuses(t *testing.T) {
	tree := mermaidTree(mermaidRendered, map[string]string{"docs/a.md": mermaidDiagram("A --> B")})
	tree.unreadable["docs/a.md"] = true
	outcome := application.Execute(tree, args("md", "mermaid", "validate", "--file", "docs/a.md"), nil)
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, "docs/a.md: cannot be read") {
		t.Fatalf("unreadable named file not refused: %+v", outcome)
	}
}

// TestMermaidIgnoresAFencedBlockThatIsNotMermaid proves a fenced block of any
// other language is never judged as a diagram, even when its text would be a
// finding if it were.
func TestMermaidIgnoresAFencedBlockThatIsNotMermaid(t *testing.T) {
	document := "```text\nclassDef x fill:#123456\n```\n" + mermaidDiagram("A --> B")
	tree := mermaidTree(mermaidBase, map[string]string{"docs/a.md": document})
	outcome := application.Execute(tree, args("md", "mermaid", "validate"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("a non-mermaid fence was judged: %+v", outcome)
	}
}

// TestMermaidBlockWithOnlyCommentsDeclaresNoDiagram proves a mermaid block with
// no declaration is declined, not read as a diagram whose colours can be judged.
func TestMermaidBlockWithOnlyCommentsDeclaresNoDiagram(t *testing.T) {
	document := "```mermaid\n%% fill:#123456 is only a note\n```\n"
	tree := mermaidTree(mermaidRendered, map[string]string{"docs/a.md": document})
	outcome := application.Execute(tree, args("md", "mermaid", "validate"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("a block with no declaration was judged: %+v", outcome)
	}
}

// TestMermaidPlainCommentIsNotAColorFinding proves a plain comment naming a
// colour is prose about the palette, never a finding.
func TestMermaidPlainCommentIsNotAColorFinding(t *testing.T) {
	document := "```mermaid\nflowchart LR\nA[x] --> B[y]\n%% fill:#123456 is a note about the palette\n```\n"
	tree := mermaidTree(mermaidBase, map[string]string{"docs/a.md": document})
	outcome := application.Execute(tree, args("md", "mermaid", "validate"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("a plain comment was flagged as a color: %+v", outcome)
	}
}

// TestMermaidStateLabelLongerThanItsLimitIsFlagged proves a quoted state label
// is read as a node label and measured against its limit.
func TestMermaidStateLabelLongerThanItsLimitIsFlagged(t *testing.T) {
	document := "```mermaid\nstateDiagram-v2\n    state \"a long state label\" as S1\n```\n"
	tree := mermaidTree(mermaidBase, map[string]string{"docs/a.md": document})
	outcome := application.Execute(tree, args("md", "mermaid", "validate"), nil)
	if outcome.ExitCode != 1 || !strings.Contains(outcome.Stdout+outcome.Stderr, "a node label is longer than the declared limit") {
		t.Fatalf("long state label not flagged: %+v", outcome)
	}
}

// TestMermaidClassLineThatNamesNoClassHasNoLabel proves a class-diagram relation
// line, which names no class after the keyword, carries no label to measure. The
// limit is 7 so the declaration line, which reads as a class named `Diagram`,
// is not what decides the result.
func TestMermaidClassLineThatNamesNoClassHasNoLabel(t *testing.T) {
	document := "```mermaid\nclassDiagram\n    Animal <|-- LongerNameDog\n```\n"
	config := strings.Replace(mermaidBase, "node-label-graphemes: 4", "node-label-graphemes: 7", 1)
	tree := mermaidTree(config, map[string]string{"docs/a.md": document})
	outcome := application.Execute(tree, args("md", "mermaid", "validate"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("a relation line was measured as a label: %+v", outcome)
	}
}

// TestMermaidUndeclaredKindOfADeclaredTypeHasNoLabels proves a diagram of a
// declared type this build cannot grammar is read, and its text is not measured
// as labels, since no grammar says what a label is.
func TestMermaidUndeclaredKindOfADeclaredTypeHasNoLabels(t *testing.T) {
	document := "```mermaid\npie title a title that is far longer than four graphemes\n\"Dogs\" : 386\n```\n"
	tree := mermaidTree(mermaidBase+"      allowed-types: [pie]\n", map[string]string{"docs/a.md": document})
	outcome := application.Execute(tree, args("md", "mermaid", "validate"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("a grammarless diagram had its text measured as labels: %+v", outcome)
	}
}

// TestMermaidEntitiesDecodeBeforeLabelsAreMeasured proves a numeric character
// reference decodes to the character it names when the label is measured, and a
// reference that does not decode, such as a surrogate or a signed number, is
// measured as the literal text it is.
func TestMermaidEntitiesDecodeBeforeLabelsAreMeasured(t *testing.T) {
	cases := []struct {
		name     string
		label    string
		exitCode int
	}{
		{"an uppercase hex reference decodes to one character", "&#X41;", 0},
		{"a surrogate reference is not decoded and is measured as text", "&#xD800;", 1},
		{"a signed reference is not decoded and is measured as text", "&#+65;", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document := "```mermaid\nflowchart LR\nA[\"" + tc.label + "\"] --> B[y]\n```\n"
			tree := mermaidTree(mermaidBase, map[string]string{"docs/a.md": document})
			outcome := application.Execute(tree, args("md", "mermaid", "validate"), nil)
			if outcome.ExitCode != tc.exitCode {
				t.Fatalf("label %s: expected exit %d: %+v", tc.label, tc.exitCode, outcome)
			}
		})
	}
}

// TestMermaidStrictColorIgnoresAHashThatAssignsNoColor proves that under a
// declared type list a `#` digit run is read as a colour only when a colour
// property assigns it, so a note that ends in a colon and a hash is prose.
func TestMermaidStrictColorIgnoresAHashThatAssignsNoColor(t *testing.T) {
	document := "```mermaid\nflowchart LR\nA[x] --> B[y]\nnote:#123456\n```\n"
	tree := mermaidTree(mermaidBase+"      allowed-types: [flowchart]\n", map[string]string{"docs/a.md": document})
	outcome := application.Execute(tree, args("md", "mermaid", "validate"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("a non-property hash was read as a color: %+v", outcome)
	}
}

// TestMermaidScanExclusionsThatAreMalformedRefuseTheSelection proves a mermaid
// run that falls back to the shared scan exclusions refuses a malformed scan
// section instead of reading it as an empty list.
func TestMermaidScanExclusionsThatAreMalformedRefuseTheSelection(t *testing.T) {
	config := mermaidRendered + "scan:\n  exclude-directories: x\n"
	tree := mermaidTree(config, map[string]string{"docs/a.md": mermaidDiagram("A --> B")})
	outcome := application.Execute(tree, args("md", "mermaid", "validate"), nil)
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, "scan.exclude-directories names no directory") {
		t.Fatalf("malformed scan exclusions not refused: %+v", outcome)
	}
}
