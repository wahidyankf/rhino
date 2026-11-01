package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// licenseFaultBase opens a license policy; each row appends the one field it tests.
const licenseFaultBase = "schema: rhino/repo-config/v2\npolicies:\n  conventions:\n    license:\n"

// TestLicenseRefusesAMalformedPolicy proves each malformed license declaration
// refuses the run with the reason its decoder names, or with the exact-path or
// digest rule it breaks, before any license file is read.
func TestLicenseRefusesAMalformedPolicy(t *testing.T) {
	cases := []struct {
		name   string
		fields string
		links  []string
		reason string
	}{
		{
			name:   "an unknown license key refuses rather than being ignored",
			fields: "      bogus: x\n",
			reason: "policies.conventions.license names unknown key `bogus`",
		},
		{
			name:   "a paths declaration that is not a list names no paths",
			fields: "      paths: x\n",
			reason: "policies.conventions.license.paths names no license paths",
		},
		{
			name:   "an exclusions declaration that is not a list names no exclusions",
			fields: "      exclusions: x\n",
			reason: "policies.conventions.license.exclusions names no exclusions",
		},
		{
			name:   "a blank exclusion names no exclusion",
			fields: "      exclusions: [\"\"]\n",
			reason: "policies.conventions.license.exclusions names no exclusion",
		},
		{
			name:   "a path entry that is a bare scalar names no license path",
			fields: "      paths:\n        - LICENSE\n",
			reason: "policies.conventions.license.paths names no license path",
		},
		{
			name:   "a path entry with an unknown key refuses",
			fields: "      paths:\n        - path: LICENSE\n          bogus: 1\n",
			reason: "policies.conventions.license.paths names unknown key `bogus`",
		},
		{
			name:   "a blank path names no license path",
			fields: "      paths:\n        - path: \" \"\n",
			reason: "policies.conventions.license.paths names no license path",
		},
		{
			name:   "identifiers that are not a list name no identifiers",
			fields: "      paths:\n        - path: LICENSE\n          identifiers: x\n",
			reason: "policies.conventions.license.paths names no license identifiers",
		},
		{
			name:   "a blank identifier names no identifier",
			fields: "      paths:\n        - path: LICENSE\n          identifiers: [\"\"]\n",
			reason: "policies.conventions.license.paths names no license identifier",
		},
		{
			name:   "a digest that is not a string names no digest",
			fields: "      paths:\n        - path: LICENSE\n          sha256: [1]\n",
			reason: "policies.conventions.license.paths names no license digest",
		},
		{
			name:   "a path with a glob character is not one exact file",
			fields: "      paths:\n        - path: docs/*.md\n",
			reason: "license paths must be exact repository-relative paths",
		},
		{
			name:   "an exclusion with a glob character is not one exact file",
			fields: "      exclusions: [\"docs/*.md\"]\n",
			reason: "license exclusions must be exact repository-relative paths",
		},
		{
			name:   "an uppercase digest is not a SHA-256 shape",
			fields: "      paths:\n        - path: LICENSE\n          sha256: \"" + strings.Repeat("A", 64) + "\"\n",
			reason: "a license sha256 must be 64 lowercase hexadecimal characters",
		},
		{
			name:   "a digest of the wrong length is not a SHA-256 shape",
			fields: "      paths:\n        - path: LICENSE\n          sha256: \"abc\"\n",
			reason: "a license sha256 must be 64 lowercase hexadecimal characters",
		},
		{
			name:   "a path with a backslash is not one exact file",
			fields: "      paths:\n        - path: docs\\LICENSE\n",
			reason: "license paths must be exact repository-relative paths",
		},
		{
			name:   "a license file reached through a link is never read",
			fields: "      paths:\n        - path: linked/LICENSE\n",
			links:  []string{"linked"},
			reason: "passes through a symbolic link, which RHINO never follows",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newMemoryTree("/root")
			tree.files["repo-config.yml"] = licenseFaultBase + tc.fields
			tree.files["linked/LICENSE"] = "text\n"
			for _, link := range tc.links {
				tree.links[link] = true
			}
			outcome := application.Execute(tree, args("convention", "license", "validate"), nil)
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
		})
	}
}

// vendorFaultBase opens a vendor policy; each row appends the one field it tests.
const vendorFaultBase = "schema: rhino/repo-config/v2\npolicies:\n  governance:\n    vendor:\n"

// TestVendorRefusesAMalformedVocabulary proves each malformed vendor declaration
// refuses the run with the reason its decoder names, so the vocabulary is never
// checked against a partial reading of the policy.
func TestVendorRefusesAMalformedVocabulary(t *testing.T) {
	cases := []struct {
		name   string
		fields string
		reason string
	}{
		{
			name:   "roots that are not a list name no roots",
			fields: "      roots: x\n",
			reason: "vendor.roots names no roots",
		},
		{
			name:   "excluded binding roots that are not a list name no roots",
			fields: "      excluded-binding-roots: x\n",
			reason: "vendor.excluded-binding-roots names no roots",
		},
		{
			name:   "a root that is not a string names no roots",
			fields: "      roots: [1]\n",
			reason: "vendor.roots names no roots",
		},
		{
			name:   "forbidden terms that are not a list name no terms",
			fields: "      forbidden-terms: x\n",
			reason: "vendor.forbidden-terms names no terms",
		},
		{
			name:   "vocabulary exceptions that are not a list name no exceptions",
			fields: "      vocabulary-exceptions: x\n",
			reason: "vendor.vocabulary-exceptions names no exceptions",
		},
		{
			name:   "an exception that is not a mapping names no exceptions",
			fields: "      vocabulary-exceptions:\n        - x\n",
			reason: "vendor.vocabulary-exceptions names no exceptions",
		},
		{
			name:   "an exception with an unknown key refuses",
			fields: "      vocabulary-exceptions:\n        - term: t\n          bogus: 1\n",
			reason: "vendor.vocabulary-exceptions name unknown key `bogus`",
		},
		{
			name:   "an exception without a term names no term",
			fields: "      vocabulary-exceptions:\n        - paths: [a]\n",
			reason: "vendor.vocabulary-exceptions names no term",
		},
		{
			name:   "an exception whose paths are not a list names no paths",
			fields: "      vocabulary-exceptions:\n        - term: t\n          paths: x\n",
			reason: "vendor.vocabulary-exceptions names no paths",
		},
		{
			name:   "an exception whose path is not a string names no paths",
			fields: "      vocabulary-exceptions:\n        - term: t\n          paths: [1]\n",
			reason: "vendor.vocabulary-exceptions names no paths",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newMemoryTree("/root")
			tree.files["repo-config.yml"] = vendorFaultBase + tc.fields
			outcome := application.Execute(tree, args("governance", "vendor", "validate"), nil)
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
		})
	}
}

// TestLicenseRefusesAFileThatEscapesThroughALink proves a license file the read
// reports as escaping the tree through a link refuses the run, and is not read.
func TestLicenseRefusesAFileThatEscapesThroughALink(t *testing.T) {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = licenseFaultBase + "      paths:\n        - path: LICENSE\n"
	tree.files["LICENSE"] = "text\n"
	tree.escapes["LICENSE"] = true
	outcome := application.Execute(tree, args("convention", "license", "validate"), nil)
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, "passes through a symbolic link, which RHINO never follows") {
		t.Fatalf("escaping license file not refused: %+v", outcome)
	}
}

// TestVendorSkipsAFileThatVanishedAfterListing proves a vendor-walked file
// listed but gone by the read is skipped, not accused and not refused.
func TestVendorSkipsAFileThatVanishedAfterListing(t *testing.T) {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = vendorFaultBase + "      roots:\n        - docs\n      forbidden-terms:\n        - Vendorname\n"
	tree.files["docs/gone.md"] = "Vendorname appears here\n"
	tree.vanished["docs/gone.md"] = true
	outcome := application.Execute(tree, args("governance", "vendor", "validate"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("a vanished file refused or was accused: %+v", outcome)
	}
}
