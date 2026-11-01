package application_test

import (
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// TestMatchTrailingStarMatchesTheRemainingName proves a star that ends a
// pattern matches the empty rest of a name, so a glob reaches a name it
// spells out in full, and still refuses a literal it does not cover.
func TestMatchTrailingStarMatchesTheRemainingName(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"a*", "a", true},
		{"a**", "a", true},
		{"a*b", "a", false},
	}
	for _, tc := range cases {
		got, err := application.Match(tc.pattern, tc.path)
		if err != nil || got != tc.want {
			t.Fatalf("Match(%q,%q)=%v,%v want %v", tc.pattern, tc.path, got, err, tc.want)
		}
	}
}
