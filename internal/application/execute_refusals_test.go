package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// TestExecuteRefusesAConfigurationItCannotRead proves that each way the
// configuration file can fail to read refuses the whole run and names the file.
func TestExecuteRefusesAConfigurationItCannotRead(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(tree *memoryTree)
		reason  string
	}{
		{
			name: "a binary configuration file refuses as holding no text",
			prepare: func(tree *memoryTree) {
				tree.binary["repo-config.yml"] = true
			},
			reason: "repo-config.yml: holds no text",
		},
		{
			name: "an unreadable configuration file refuses with the read reason",
			prepare: func(tree *memoryTree) {
				tree.unreadable["repo-config.yml"] = true
			},
			reason: "repo-config.yml: cannot be read at line 1: repo-config.yml: permission denied",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newMemoryTree("/root")
			tree.files["repo-config.yml"] = "schema: rhino/repo-config/v2\n"
			tc.prepare(tree)
			outcome := application.Execute(tree, args("repo-config", "validate"), nil)
			if outcome.ExitCode != 2 {
				t.Fatalf("expected refusal: %+v", outcome)
			}
			if !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected %q in %+v", tc.reason, outcome)
			}
		})
	}
}

// TestGateRunRefusesAnUndeclaredGatesSection proves a gate run with no gates
// section refuses before any child starts, because RHINO holds no default.
func TestGateRunRefusesAnUndeclaredGatesSection(t *testing.T) {
	tree := gateTree("schema: rhino/repo-config/v2\n")
	launcher := &stubLauncher{}
	outcome := application.ExecuteWithLauncher(tree, args("gate", "run", "--surface", "pre-commit"), nil, launcher)
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "gates section is not declared") {
		t.Fatalf("undeclared gates not refused: %+v", outcome)
	}
	if len(launcher.seen) != 0 {
		t.Fatalf("a refused run started a child: %v", launcher.seen)
	}
}

// TestEmojiValidationRefusesMalformedPolicy proves each malformed emoji policy
// or scan section refuses the run rather than being read as a clean walk.
func TestEmojiValidationRefusesMalformedPolicy(t *testing.T) {
	cases := []struct {
		name   string
		config string
		reason string
	}{
		{
			name:   "an undeclared emoji policy refuses rather than defaulting",
			config: "schema: rhino/repo-config/v2\npolicies:\n  conventions: {}\n",
			reason: "policies.conventions.emoji is not declared",
		},
		{
			name:   "a prohibited item that is not a mapping names no glob",
			config: "schema: rhino/repo-config/v2\npolicies:\n  conventions:\n    emoji:\n      prohibited:\n        - just-a-string\n",
			reason: "convention-emoji.prohibited names no glob",
		},
		{
			name:   "a scan section that is not a mapping refuses the exclusion list",
			config: "schema: rhino/repo-config/v2\nscan: 3\npolicies:\n  conventions:\n    emoji:\n      prohibited:\n        - glob: \"**/*.json\"\n",
			reason: "scan.exclude-directories names no directory",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newMemoryTree("/root")
			tree.files["repo-config.yml"] = tc.config
			tree.files["config/a.json"] = "plain\n"
			outcome := application.Execute(tree, args("convention", "emoji", "validate"), nil)
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
		})
	}
}

// TestEmojiValidationSkipsAFileThatVanishedAfterListing proves a prohibited
// file listed by the walk but gone by the read is skipped, not accused and not
// refused: the emoji it once held cannot be inspected any more.
func TestEmojiValidationSkipsAFileThatVanishedAfterListing(t *testing.T) {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = "schema: rhino/repo-config/v2\npolicies:\n  conventions:\n    emoji:\n      prohibited:\n        - glob: \"**/*.json\"\n"
	tree.files["config/gone.json"] = "held \U0001F98F\n"
	tree.vanished["config/gone.json"] = true
	outcome := application.Execute(tree, args("convention", "emoji", "validate"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("a vanished file refused or was accused: %+v", outcome)
	}
	if strings.Contains(outcome.Stdout+outcome.Stderr, "U+1F98F") {
		t.Fatalf("a vanished file was still inspected: %+v", outcome)
	}
}
