package application

import (
	"errors"
	"testing"
)

// A child that cannot start reports its cause through Unwrap, and an
// unknown exemption tag is refused with a message naming the valid tags. Both
// answer from the values alone.
func TestChildStartErrorUnwrapsItsCause(t *testing.T) {
	cause := errors.New("cause")
	if got := (&ChildStartError{Status: 127, Err: cause}).Unwrap(); got != cause {
		t.Fatalf("Unwrap = %v, want the cause", got)
	}
}

func TestExemptionProblemRefusesAnUnknownExemptionTag(t *testing.T) {
	level, problem := exemptionProblem(featureFile{}, featureTag{Name: "@bogus-exempt"})
	if level != "" || problem == "" {
		t.Fatalf("exemptionProblem = (%q, %q), want no level and a problem", level, problem)
	}
}
