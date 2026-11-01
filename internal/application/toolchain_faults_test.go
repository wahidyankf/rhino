package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// TestToolchainRefusesAMalformedEntryOrProvision proves each malformed toolchain
// entry or provision refuses the run with the reason its decoder names, before
// any tool is probed.
func TestToolchainRefusesAMalformedEntryOrProvision(t *testing.T) {
	cases := []struct {
		name   string
		entry  string
		reason string
	}{
		{"an entry with no id", "executable: formatter\n      probe: [--version]", "toolchains.entries names no id"},
		{"an entry with no executable", "id: formatter\n      probe: [--version]", "toolchains.entries names no executable"},
		{"a probe that is not a list", "id: formatter\n      executable: formatter\n      probe: x", "toolchains.entries names no probe"},
		{"a probe item that is not a scalar", "id: formatter\n      executable: formatter\n      probe: [[a]]", "toolchains.entries names no probe"},
		{"a parser that is not a scalar", "id: formatter\n      executable: formatter\n      probe: [--version]\n      parser: [x]\n      version: 1.2.3", "toolchains.entries names no parser"},
		{"a version that is not a scalar", "id: formatter\n      executable: formatter\n      probe: [--version]\n      parser: first-token\n      version: [x]", "toolchains.entries names no version"},
		{"a timeout that is not an integer", "id: formatter\n      executable: formatter\n      probe: [--version]\n      timeout-seconds: soon", "toolchains.entries names no timeout"},
		{"a required value that is not a boolean", "id: formatter\n      executable: formatter\n      probe: [--version]\n      required: yes-please", "toolchains.entries names no required value"},
		{"a provision that is not a list", "id: formatter\n      executable: formatter\n      probe: [--version]\n      provision: x", "toolchains.provision names no provision"},
		{"a provision item that is not a mapping", "id: formatter\n      executable: formatter\n      probe: [--version]\n      provision: [x]", "toolchains.provision names no provision"},
		{"a provision with an unknown key", "id: formatter\n      executable: formatter\n      probe: [--version]\n      provision: [{platform: linux, executable: installer, bogus: 1}]", "toolchains.provision names unknown key `bogus`"},
		{"a provision with no platform", "id: formatter\n      executable: formatter\n      probe: [--version]\n      provision: [{executable: installer}]", "toolchains.provision names no platform"},
		{"a provision with no executable", "id: formatter\n      executable: formatter\n      probe: [--version]\n      provision: [{platform: linux}]", "toolchains.provision names no executable"},
		{"a provision whose arguments are not a list", "id: formatter\n      executable: formatter\n      probe: [--version]\n      provision: [{platform: linux, executable: installer, arguments: x}]", "toolchains.provision names no arguments"},
		{"a provision with an empty platform is not a platform", "id: formatter\n      executable: formatter\n      probe: [--version]\n      provision: [{platform: \"\", executable: installer}]", "has a duplicate or invalid provision declaration"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := &recordingToolchainRunner{}
			tree := toolchainTree("toolchains:\n  entries:\n    - " + tc.entry + "\n")
			outcome := runToolchain(tree, runner, "toolchain", "validate")
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
			if len(runner.launches) != 0 {
				t.Fatalf("a malformed entry probed %d tools, want none", len(runner.launches))
			}
		})
	}
}

// TestToolchainAcceptsAHyphenatedProvisionPlatform proves a provision platform
// may hold hyphens and digits as well as lowercase letters, so a clean entry
// validates.
func TestToolchainAcceptsAHyphenatedProvisionPlatform(t *testing.T) {
	runner := &recordingToolchainRunner{outcomes: []toolchainAnswer{okProbe("1.2.3")}}
	body := "toolchains:\n  entries:\n    - id: formatter\n      executable: formatter\n      probe: [--version]\n" +
		"      parser: first-token\n      version: 1.2.3\n      provision: [{platform: linux-x64, executable: installer}]\n"
	outcome := runToolchain(toolchainTree(body), runner, "toolchain", "validate")
	if outcome.ExitCode != 0 {
		t.Fatalf("hyphenated platform refused: %+v", outcome)
	}
}

// TestToolchainFirstLineVersionReadsTheWholeOutput proves a first-line parser
// reads an output with no line break as one line, so a version printed alone
// still matches.
func TestToolchainFirstLineVersionReadsTheWholeOutput(t *testing.T) {
	runner := &recordingToolchainRunner{outcomes: []toolchainAnswer{{stdout: "1.2.3"}}}
	body := "toolchains:\n  entries:\n    - id: formatter\n      executable: formatter\n      probe: [--version]\n" +
		"      parser: first-line\n      version: 1.2.3\n      required: true\n"
	outcome := runToolchain(toolchainTree(body), runner, "toolchain", "validate")
	if outcome.ExitCode != 0 {
		t.Fatalf("a single-line version did not match: %+v", outcome)
	}
}

// TestToolchainValidateWithoutARunnerProbesNothing proves a validation entered
// without a runner treats every tool as unavailable, so a required tool is a
// finding rather than a silent pass.
func TestToolchainValidateWithoutARunnerProbesNothing(t *testing.T) {
	tree := toolchainTree(toolchainEntry)
	outcome := application.Execute(tree, args("toolchain", "validate"), nil)
	if outcome.ExitCode != 1 || !strings.Contains(outcome.Stderr, "formatter: unavailable") {
		t.Fatalf("missing runner did not report the required tool unavailable: %+v", outcome)
	}
}
