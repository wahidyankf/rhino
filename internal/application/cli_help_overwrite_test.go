package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// TestHelpOverwriteNamesOnlyFileGenerations pins the env push --overwrite help
// to the one case that keeps a previous generation: a file entry. A login value
// keeps none, so the help must not say every vault value keeps one.
func TestHelpOverwriteNamesOnlyFileGenerations(t *testing.T) {
	text, ok := application.HelpFor([]string{"env", "push"})
	if !ok {
		t.Fatal("the env push help is missing")
	}
	want := "  --overwrite            Replace a differing vault value. Only a file entry keeps\n" +
		"                         its previous generation.\n"
	if !strings.Contains(text, want) {
		t.Fatalf("the --overwrite help does not name only file entries as keeping a generation; want %q; got:\n%s", want, text)
	}
}
