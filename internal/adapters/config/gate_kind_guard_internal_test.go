package config

import "testing"

// Config validation checks kinds against the known list before these helpers run,
// so an unknown kind is exercised on the helpers directly: it accepts no source and
// names no field.
func TestUnknownInputKindAcceptsNothingAndNamesNoField(t *testing.T) {
	if sourceAccepts("no-such-kind", "checkout") {
		t.Fatal("an unknown input kind accepted a source")
	}
	if inputHasField("no-such-kind", "paths") {
		t.Fatal("an unknown input kind named a field")
	}
}
