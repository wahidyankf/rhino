package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// emojiPolicy declares the emoji convention the scan-section rows read through.
const emojiPolicy = "policies:\n  conventions:\n    emoji:\n      prohibited:\n        - glob: \"**/*.json\"\n"

// TestConfigurationRefusesAMalformedDeclaration proves each malformed
// repository configuration refuses the run with its own reason, so no command
// reads a partial policy.
func TestConfigurationRefusesAMalformedDeclaration(t *testing.T) {
	cases := []struct {
		name   string
		config string
		words  []string
		reason string
	}{
		{
			name:   "text that is not YAML refuses at the configuration file",
			config: "schema: [\n",
			words:  []string{"repo-config", "validate"},
			reason: "repo-config.yml:",
		},
		{
			name:   "a gate entry that is not a mapping names no id",
			config: "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - just-a-string\n",
			words:  []string{"gate", "validate"},
			reason: "gate entry names no id",
		},
		{
			name:   "a gate entry without an id names no id",
			config: "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - command:\n        executable: ./check\n",
			words:  []string{"gate", "validate"},
			reason: "gate entry names no id",
		},
		{
			name: "a pull-request composition relation that is neither exact nor at-least refuses",
			config: "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: local-check\n      run-on:\n        pre-commit: {}\n" +
				"  composition:\n    pull-request:\n      relation: loose\n",
			words:  []string{"gate", "validate"},
			reason: "pull-request relation `loose` is not exact or at-least",
		},
		{
			name: "a mutation contract that is not a mapping refuses its shape",
			config: "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: format-staged\n      type: mutation\n" +
				"      mutation: apply\n      run-on:\n        pre-commit: {}\n        pull-request: {}\n",
			words:  []string{"gate", "validate"},
			reason: "expected struct MutationContract",
		},
		{
			name:   "a scan exclusion list that is not a list names no directory",
			config: "schema: rhino/repo-config/v2\nscan:\n  exclude-directories: x\n" + emojiPolicy,
			words:  []string{"convention", "emoji", "validate"},
			reason: "scan.exclude-directories names no directory",
		},
		{
			name:   "a scan exclusion that is not a string names no directory",
			config: "schema: rhino/repo-config/v2\nscan:\n  exclude-directories: [1]\n" + emojiPolicy,
			words:  []string{"convention", "emoji", "validate"},
			reason: "scan.exclude-directories names no directory",
		},
		{
			name:   "an internal-link check with a malformed scan list refuses",
			config: "schema: rhino/repo-config/v2\nscan:\n  exclude-directories: x\npolicies:\n  markdown:\n    internal-link: {}\n",
			words:  []string{"md", "internal-link", "validate"},
			reason: "scan.exclude-directories names no directory",
		},
		{
			name: "a frontmatter check with a malformed scan list refuses",
			config: "schema: rhino/repo-config/v2\nscan:\n  exclude-directories: x\npolicies:\n  markdown:\n    frontmatter:\n" +
				"      surfaces:\n        - glob: \"notes/*.md\"\n          require: [title]\n",
			words:  []string{"md", "frontmatter", "validate"},
			reason: "scan.exclude-directories names no directory",
		},
		{
			name:   "an environment section that is not a mapping is not declared",
			config: "schema: rhino/repo-config/v2\nenvironment: x\n",
			words:  []string{"env", "validate"},
			reason: "environment section is not declared",
		},
		{
			name:   "a toolchains section that is not a mapping is not declared",
			config: "schema: rhino/repo-config/v2\ntoolchains: x\n",
			words:  []string{"toolchain", "validate"},
			reason: "toolchains section is not declared",
		},
		{
			name:   "a harness section that is not a mapping declares no adapter profiles",
			config: "schema: rhino/repo-config/v2\nharness: x\n",
			words:  []string{"harness", "adapters", "validate"},
			reason: "no adapter profiles are declared",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newMemoryTree("/root")
			tree.files["repo-config.yml"] = tc.config
			outcome := application.Execute(tree, args(tc.words...), nil)
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
		})
	}
}

// TestScanWithNoExclusionsExcludesNothing proves a scan section that declares
// an empty mapping, or none, excludes no directory, so a prohibited file under
// any directory is still found.
func TestScanWithNoExclusionsExcludesNothing(t *testing.T) {
	for _, config := range []string{
		"schema: rhino/repo-config/v2\nscan: {}\npolicies:\n  conventions:\n    emoji:\n      prohibited:\n        - glob: \"**/*.json\"\n",
		"schema: rhino/repo-config/v2\npolicies:\n  conventions:\n    emoji:\n      prohibited:\n        - glob: \"**/*.json\"\n",
	} {
		tree := newMemoryTree("/root")
		tree.files["repo-config.yml"] = config
		tree.files["generated/a.json"] = "held \U0001F98F\n"
		outcome := application.Execute(tree, args("convention", "emoji", "validate"), nil)
		if outcome.ExitCode != 1 || !strings.Contains(outcome.Stderr, "U+1F98F") {
			t.Fatalf("an unexcluded directory was not inspected: %+v", outcome)
		}
	}
}
