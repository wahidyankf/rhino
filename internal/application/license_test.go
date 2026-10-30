package application_test

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

func licenseTree(config string, files map[string]string) *memoryTree {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = config
	for path, content := range files {
		tree.files[path] = content
	}
	return tree
}

func licenseDigest(text string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(text)))
}

const licenseBase = "schema: rhino/repo-config/v2\npolicies:\n  conventions:\n    license:\n"

func TestExecuteLicenseCleanAndFindings(t *testing.T) {
	held := "SPDX: MIT\n"
	cases := []struct {
		name     string
		config   string
		files    map[string]string
		exitCode int
		contains []string
		absent   []string
	}{
		{
			name:     "a held identifier with a matching digest reports clean",
			config:   licenseBase + "      paths: [{path: LICENSE, identifiers: [MIT], sha256: \"" + licenseDigest(held) + "\"}]\n",
			files:    map[string]string{"LICENSE": held},
			exitCode: 0,
			contains: []string{"[license] checked 1 license file, no findings"},
		},
		{
			name:     "a path with no checks reports clean when the file is held",
			config:   licenseBase + "      paths: [{path: LICENSE}]\n",
			files:    map[string]string{"LICENSE": "anything\n"},
			exitCode: 0,
		},
		{
			name:     "an absent file is a missing-license finding rather than a refusal",
			config:   licenseBase + "      paths: [{path: missing}]\n",
			files:    map[string]string{},
			exitCode: 1,
			contains: []string{
				"missing: the configured license file is absent",
			},
		},
		{
			name:     "an absent identifier is reported with the identifier it missed",
			config:   licenseBase + "      paths: [{path: LICENSE, identifiers: [Apache]}]\n",
			files:    map[string]string{"LICENSE": "SPDX: MIT\n"},
			exitCode: 1,
			contains: []string{
				"the configured license identifier is absent",
				"identifier=Apache",
			},
		},
		{
			name:     "a digest that does not match is a digest-mismatch finding",
			config:   licenseBase + "      paths: [{path: LICENSE, sha256: \"" + strings.Repeat("0", 64) + "\"}]\n",
			files:    map[string]string{"LICENSE": "SPDX: MIT\n"},
			exitCode: 1,
			contains: []string{
				"the configured license digest does not match",
			},
		},
		{
			name: "one line reports every mismatch kind together in path order",
			config: licenseBase + "      paths: [{path: missing, identifiers: [MIT]}, {path: LICENSE, identifiers: [Apache], sha256: \"" +
				strings.Repeat("0", 64) + "\"}]\n",
			files:    map[string]string{"LICENSE": "SPDX: MIT\n"},
			exitCode: 1,
			contains: []string{
				"the configured license file is absent",
				"the configured license identifier is absent",
				"the configured license digest does not match",
			},
		},
		{
			name:     "an excluded path is never read and never accused",
			config:   licenseBase + "      paths: [{path: ignored, identifiers: [unread], sha256: not-a-digest}]\n      exclusions: [ignored]\n",
			files:    map[string]string{},
			exitCode: 0,
			absent:   []string{"ignored"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := licenseTree(tc.config, tc.files)
			outcome := application.Execute(tree, args("convention", "license", "validate"), nil)
			if outcome.ExitCode != tc.exitCode {
				t.Fatalf("expected exit %d: %+v", tc.exitCode, outcome)
			}
			for _, want := range tc.contains {
				if !strings.Contains(outcome.Stdout+outcome.Stderr, want) {
					t.Fatalf("expected %q in %+v", want, outcome)
				}
			}
			for _, want := range tc.absent {
				if strings.Contains(outcome.Stdout+outcome.Stderr, want) {
					t.Fatalf("expected %q absent from %+v", want, outcome)
				}
			}
		})
	}
}

func TestExecuteLicenseJSONCarriesKinds(t *testing.T) {
	held := "SPDX: MIT\n"
	tree := licenseTree(
		licenseBase+"      paths: [{path: missing}, {path: LICENSE, identifiers: [Apache], sha256: \""+strings.Repeat("0", 64)+"\"}]\n",
		map[string]string{"LICENSE": held},
	)
	outcome := application.Execute(tree, args("convention", "license", "validate", "--output", "json"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	for _, want := range []string{
		`"kind":"missing-license"`,
		`"kind":"license-identifier-mismatch"`,
		`"kind":"license-digest-mismatch"`,
		`"identifier":"Apache"`,
		`"command":"license"`,
		`"exitCode":1`,
	} {
		if !strings.Contains(outcome.Stdout, want) {
			t.Fatalf("expected %q in %q", want, outcome.Stdout)
		}
	}
	if outcome.Stderr != "" {
		t.Fatalf("json findings must leave stderr empty: %+v", outcome)
	}
}

func TestExecuteLicenseRefusals(t *testing.T) {
	held := "SPDX: MIT\n"
	cases := []struct {
		name     string
		config   string
		files    map[string]string
		unread   []string
		binary   []string
		links    []string
		exitCode int
		contains []string
	}{
		{
			name:     "an undeclared license section refuses rather than defaulting",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  conventions:\n    emoji:\n      prohibited: []\n",
			exitCode: 2,
			contains: []string{"policies.conventions.license is not declared"},
		},
		{
			name:     "an exclusion that climbs refuses the whole run",
			config:   licenseBase + "      paths: []\n      exclusions: [../outside]\n",
			exitCode: 2,
			contains: []string{"license exclusions must be exact repository-relative paths"},
		},
		{
			name:     "an absolute license path refuses the whole run",
			config:   licenseBase + "      paths: [{path: /LICENSE}]\n",
			exitCode: 2,
			contains: []string{"license paths must be exact repository-relative paths"},
		},
		{
			name:     "a glob license path refuses the whole run",
			config:   licenseBase + "      paths: [{path: LICENSE*.txt}]\n",
			exitCode: 2,
			contains: []string{"license paths must be exact repository-relative paths"},
		},
		{
			name:     "a repeated license path refuses the whole run",
			config:   licenseBase + "      paths: [{path: LICENSE}, {path: LICENSE}]\n",
			files:    map[string]string{"LICENSE": held},
			exitCode: 2,
			contains: []string{"license paths must not repeat"},
		},
		{
			name:     "a malformed digest refuses before any file is read",
			config:   licenseBase + "      paths: [{path: LICENSE, sha256: not-a-digest}]\n",
			files:    map[string]string{"LICENSE": held},
			exitCode: 2,
			contains: []string{"a license sha256 must be 64 lowercase hexadecimal characters"},
		},
		{
			name:     "an unreadable license file fails closed with its path",
			config:   licenseBase + "      paths: [{path: LICENSE}]\n",
			files:    map[string]string{"LICENSE": held},
			unread:   []string{"LICENSE"},
			exitCode: 2,
			contains: []string{"LICENSE"},
		},
		{
			name:     "a binary license file fails closed as holding no text",
			config:   licenseBase + "      paths: [{path: LICENSE}]\n",
			files:    map[string]string{"LICENSE": "bytes"},
			binary:   []string{"LICENSE"},
			exitCode: 2,
			contains: []string{"holds no text"},
		},
		{
			name:     "a license file behind a link is never followed",
			config:   licenseBase + "      paths: [{path: LICENSE}]\n",
			files:    map[string]string{"LICENSE": held},
			links:    []string{"LICENSE"},
			exitCode: 2,
			contains: []string{"passes through a symbolic link, which RHINO never follows"},
		},
		{
			name:     "a license run reports its refusal as JSON when asked",
			config:   "schema: rhino/repo-config/v2\n",
			exitCode: 2,
			contains: []string{"rhino.config.undeclared"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := licenseTree(tc.config, tc.files)
			for _, path := range tc.unread {
				tree.unreadable[path] = true
			}
			for _, path := range tc.binary {
				tree.binary[path] = true
			}
			for _, path := range tc.links {
				tree.links[path] = true
			}
			format := args("convention", "license", "validate")
			if strings.Contains(tc.name, "as JSON") {
				format = args("convention", "license", "validate", "--output", "json")
			}
			outcome := application.Execute(tree, format, nil)
			if outcome.ExitCode != tc.exitCode {
				t.Fatalf("expected exit %d: %+v", tc.exitCode, outcome)
			}
			for _, want := range tc.contains {
				if !strings.Contains(outcome.Stdout+outcome.Stderr, want) {
					t.Fatalf("expected %q in %+v", want, outcome)
				}
			}
		})
	}
}
