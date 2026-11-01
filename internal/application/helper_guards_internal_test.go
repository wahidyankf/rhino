package application

import (
	"strings"
	"testing"
)

// These helpers guard inputs their callers reach only through a shape the
// front end refuses first, so each one is exercised with the input it guards.

// TestCheckQualityDefaultsRefusesACycleCeilingAboveThree proves the run-default
// check refuses a cycle ceiling above three, the bound a gate never exceeds.
func TestCheckQualityDefaultsRefusesACycleCeilingAboveThree(t *testing.T) {
	if err := checkQualityDefaults(map[string]any{"max-cycles": 4}); err == nil {
		t.Fatal("a cycle ceiling above three was accepted")
	}
	if err := checkQualityDefaults(map[string]any{"max-cycles": 3}); err != nil {
		t.Fatalf("a cycle ceiling of three was refused: %v", err)
	}
}

// TestScopeNoteIsEmptyWhenEveryMatchingCommandAcceptsTheFlag proves the note
// names only the commands that accept a flag when some do not, and is empty when
// all of them do.
func TestScopeNoteIsEmptyWhenEveryMatchingCommandAcceptsTheFlag(t *testing.T) {
	matching := []*Leaf{
		{Path: []string{"gate", "run"}, Accepts: []Accepts{AcceptSurface}},
		{Path: []string{"gate", "list"}, Accepts: []Accepts{AcceptSurface}},
	}
	if note := scopeNote(matching, AcceptSurface); note != "" {
		t.Fatalf("a flag every command accepts produced a note: %q", note)
	}
	matching[1].Accepts = nil
	if note := scopeNote(matching, AcceptSurface); !strings.Contains(note, "`gate run`") || strings.Contains(note, "`gate list`") {
		t.Fatalf("a partial flag did not name only the accepting command: %q", note)
	}
}

// TestClassEndFindsNoClosingBracketInAnUnclosedClass proves a character class
// with no closing bracket reports no end, and a closed one reports its bracket.
func TestClassEndFindsNoClosingBracketInAnUnclosedClass(t *testing.T) {
	if got := classEnd([]byte("[abc")); got != -1 {
		t.Fatalf("an unclosed class reported end %d", got)
	}
	if got := classEnd([]byte("[a]")); got != 2 {
		t.Fatalf("a closed class reported end %d, want 2", got)
	}
}

// TestIsPortableNameRefusesAnEmptyNameOrWord proves a portable name is not
// empty and has no empty word between its hyphens.
func TestIsPortableNameRefusesAnEmptyNameOrWord(t *testing.T) {
	for _, value := range []string{"", "a--b", "-a"} {
		if isPortableName(value) {
			t.Fatalf("isPortableName(%q) accepted an unportable name", value)
		}
	}
	if !isPortableName("a-b") {
		t.Fatal("a portable name was refused")
	}
}

// TestMetadataSentencesCountsAProseFragmentAsOne proves text with content but no
// sentence end still counts as one sentence.
func TestMetadataSentencesCountsAProseFragmentAsOne(t *testing.T) {
	if got := metadataSentences("..."); got != 1 {
		t.Fatalf("a fragment of punctuation counted %d sentences, want 1", got)
	}
}

func TestStartsWithSpaceIsFalseForAnEmptyValue(t *testing.T) {
	if startsWithSpace("") {
		t.Fatal("an empty value started with a space")
	}
}

// TestMetadataBlockLineDefaultsToTheFirstLine proves a document with no closing
// delimiter reports the first line, where a missing key is reported.
func TestMetadataBlockLineDefaultsToTheFirstLine(t *testing.T) {
	if got := metadataBlockLine("name: x"); got != 1 {
		t.Fatalf("a block with no delimiter reported line %d, want 1", got)
	}
}

// TestMetadataUnquoteKeepsAnOpenQuoteAsWritten proves a value that opens a double
// quote and never closes it is read as written, not stripped of its quote.
func TestMetadataUnquoteKeepsAnOpenQuoteAsWritten(t *testing.T) {
	if got := metadataUnquote(`"abc`); got != `"abc` {
		t.Fatalf("an open quote was altered to %q", got)
	}
}

// TestReadmeExcludedIgnoresAPathOutsideTheRoot proves an index exclusion applies
// only to paths under the index's root.
func TestReadmeExcludedIgnoresAPathOutsideTheRoot(t *testing.T) {
	if readmeExcluded("docs/a.md", "other", []string{"docs"}) {
		t.Fatal("a path outside the index root was excluded")
	}
}

// TestReadmeTargetsSkipsAnUnterminatedOrBlankLink proves a link with no closing
// parenthesis, and one whose target is blank, names no target.
func TestReadmeTargetsSkipsAnUnterminatedOrBlankLink(t *testing.T) {
	if got := readmeTargets("docs/README.md", "[a](b"); len(got) != 0 {
		t.Fatalf("an unterminated link named targets %v", got)
	}
	if got := readmeTargets("docs/README.md", "[a]( ) x"); len(got) != 0 {
		t.Fatalf("a blank link named targets %v", got)
	}
}

// TestReadmeResolveRefusesATargetThatLeavesOrNamesNothing proves a target that
// climbs past the top of the repository, or resolves to no path, is not a target.
func TestReadmeResolveRefusesATargetThatLeavesOrNamesNothing(t *testing.T) {
	if _, ok := readmeResolve("README.md", "../x"); ok {
		t.Fatal("a target climbing past the root resolved")
	}
	if _, ok := readmeResolve("README.md", "."); ok {
		t.Fatal("a target naming the root itself resolved")
	}
}

// TestIsDirectoryOfTheRootMeansAnyFile proves the repository root counts as a
// directory exactly when the listing holds at least one file.
func TestIsDirectoryOfTheRootMeansAnyFile(t *testing.T) {
	if !IsDirectoryOf([]string{"a"}, ".") {
		t.Fatal("the root of a non-empty listing was not a directory")
	}
	if IsDirectoryOf(nil, "") {
		t.Fatal("the root of an empty listing was a directory")
	}
}
