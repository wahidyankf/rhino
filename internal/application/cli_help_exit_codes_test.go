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

// TestHelpNamesEveryRequiredChildStatus pins the 126 and 127 lines of the help
// text to the three kinds of child RHINO starts and requires: a gate child, the
// Bitwarden CLI an env command starts, and a toolchain provision child.
func TestHelpNamesEveryRequiredChildStatus(t *testing.T) {
	text, ok := application.HelpFor(nil)
	if !ok {
		t.Fatal("the top-level help is missing")
	}
	for _, want := range []string{
		"126  a required child was found and could not be executed: a gate child, the Bitwarden CLI, or a toolchain provision child",
		"127  a required child was not found: a gate child, the Bitwarden CLI, or a toolchain provision child",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("the exit-code help does not carry the line %q; got:\n%s", want, text)
		}
	}
}
