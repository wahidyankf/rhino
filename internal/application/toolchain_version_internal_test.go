package application

import "testing"

// Config decoding refuses a version without a parser, so this guard is checked
// directly: a declared version with no parser never matches, and no declared
// version matches anything.
func TestToolchainVersionMatchesNeedsAParserForAVersion(t *testing.T) {
	version := "1.2.3"
	if toolchainVersionMatches(toolchain{version: &version}, "1.2.3") {
		t.Fatal("a version with no parser matched")
	}
	if !toolchainVersionMatches(toolchain{}, "anything at all") {
		t.Fatal("an undeclared version did not match")
	}
}
