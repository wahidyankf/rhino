package config_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/config"
)

// TestLicenseDigestKeepsItsWrittenText proves an all-digit digest reads as the
// text the file wrote. YAML would read it as a number and lose the zeros, which
// the oracle's string field never does.
func TestLicenseDigestKeepsItsWrittenText(t *testing.T) {
	digest := strings.Repeat("0", 64)
	document, err := config.Parse("schema: rhino/repo-config/v2\npolicies:\n  conventions:\n    license:\n      paths:\n        - path: LICENSE\n          sha256: " + digest + "\n")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	license, ok := document.PolicyGroup("conventions", "license")
	if !ok {
		t.Fatal("license policy not found")
	}
	paths, _ := license["paths"].([]any)
	if len(paths) != 1 {
		t.Fatalf("expected one license path, got %v", license["paths"])
	}
	entry, _ := paths[0].(map[string]any)
	if got, _ := entry["sha256"].(string); got != digest {
		t.Fatalf("digest read as %#v, want the written text", entry["sha256"])
	}
}
