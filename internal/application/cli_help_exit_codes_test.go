package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// TestHelpNamesEveryKindOfExitOne pins the exit-1 line of the help text to the
// three things that may report 1, the vault commands among them.
func TestHelpNamesEveryKindOfExitOne(t *testing.T) {
	text, ok := application.HelpFor(nil)
	if !ok {
		t.Fatal("the top-level help is missing")
	}
	want := "1    a declared policy was violated, a vault command reported a finding, or a gate child failed"
	if !strings.Contains(text, want) {
		t.Fatalf("the exit-code help does not carry the exit-1 line %q; got:\n%s", want, text)
	}
}
