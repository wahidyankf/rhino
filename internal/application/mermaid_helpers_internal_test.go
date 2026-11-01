package application

import "testing"

// These helpers guard inputs their callers never pass, so each is exercised on
// its own with that input. Each assertion names the value the guard returns.

// TestDiagramFirstLineDefaultsToTheFirstLine proves a diagram with no recorded
// lines reports line 1, the place a reader starts, rather than indexing nothing.
func TestDiagramFirstLineDefaultsToTheFirstLine(t *testing.T) {
	if got := diagramFirstLine(&diagram{}); got != 1 {
		t.Fatalf("diagramFirstLine of an empty diagram = %d, want 1", got)
	}
}

// TestColorsABoxNeedsANameAfterTheKeyword proves a sequence-diagram box keyword
// with nothing after it names no colour.
func TestColorsABoxNeedsANameAfterTheKeyword(t *testing.T) {
	if colorsABox("box ") {
		t.Fatal("a box with no name was read as naming a colour")
	}
	if !colorsABox("box red") {
		t.Fatal("a box naming a colour was not read as one")
	}
}

// TestCanvasFindingsSkipsACanvasItCannotRead proves a canvas colour that is not a
// six-digit hex value is skipped, so no contrast is measured against it.
func TestCanvasFindingsSkipsACanvasItCannotRead(t *testing.T) {
	fill := "#000000"
	rules := mermaidRules{canvasColors: []string{"red"}}
	if findings := canvasFindings(3, &fill, nil, rules); len(findings) != 0 {
		t.Fatalf("an unreadable canvas produced findings: %+v", findings)
	}
}
