package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
	"github.com/wahidyankf/rhino/internal/domain"
)

// TestParseRefusesAValueFlagWithNoValue proves a value-taking flag at the end
// of the argument list refuses as incomplete, naming the flag, rather than
// reading the next argument or a missing one.
func TestParseRefusesAValueFlagWithNoValue(t *testing.T) {
	cases := []struct {
		name  string
		words []string
		flag  string
	}{
		{"an output flag with no value", []string{"repo-config", "validate", "--output"}, "--output"},
		{"a base flag with no value", []string{"gate", "run", "--surface", "pull-request", "--base"}, "--base"},
		{"a head flag with no value", []string{"gate", "run", "--surface", "pull-request", "--base", "aaaaaaa", "--head"}, "--head"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, refusal := application.Parse(args(tc.words...))
			if refusal == nil || refusal.Code != domain.ArgsIncomplete || !strings.Contains(refusal.Message, "`"+tc.flag+"` needs a value") {
				t.Fatalf("expected incomplete refusal for %s: %+v", tc.flag, refusal)
			}
		})
	}
}
