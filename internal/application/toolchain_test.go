package application_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// recordedLaunch is one probe or provision vector the test double saw.
type recordedLaunch struct {
	executable string
	arguments  []string
}

// recordingToolchainRunner answers queued results in order and records every
// launch, so tests prove planning never starts a process and applying stops
// after the first failure.
type recordingToolchainRunner struct {
	launches []recordedLaunch
	outcomes []toolchainAnswer
}

type toolchainAnswer struct {
	code   int
	stdout string
	failed string
}

func okProbe(stdout string) toolchainAnswer { return toolchainAnswer{code: 0, stdout: stdout} }

func (r *recordingToolchainRunner) Run(launch application.ToolchainLaunch) (application.ToolchainResult, error) {
	r.launches = append(r.launches, recordedLaunch{executable: launch.Executable, arguments: append([]string(nil), launch.Arguments...)})
	if len(r.outcomes) == 0 {
		return application.ToolchainResult{}, errors.New("the synthetic runner had no declared result")
	}
	answer := r.outcomes[0]
	r.outcomes = r.outcomes[1:]
	if answer.failed != "" {
		return application.ToolchainResult{}, errors.New(answer.failed)
	}
	return application.ToolchainResult{Code: answer.code, Stdout: answer.stdout}, nil
}

func toolchainTree(body string) *memoryTree {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = "schema: rhino/repo-config/v2\n" + body
	return tree
}

func runToolchain(tree *memoryTree, runner *recordingToolchainRunner, words ...string) application.Outcome {
	var held application.ToolchainRunner
	if runner != nil {
		held = runner
	}
	return application.ExecuteWithToolchains(tree, args(words...), nil, nil, nil, held)
}

// vectorsForEveryPlatform declares the same vector for every platform name a
// host can report under the contract: the oracle's names for the Go GOOS values
// that have one. The host always finds its own vector whatever machine runs the
// suite. arguments is an inline YAML list, or "" for none.
func vectorsForEveryPlatform(executable, arguments string) string {
	platforms := []string{
		"aix", "android", "dragonfly", "freebsd", "illumos", "ios", "linux",
		"macos", "netbsd", "openbsd", "solaris", "windows",
	}
	vectors := make([]string, 0, len(platforms))
	for _, platform := range platforms {
		vector := "{platform: " + platform + ", executable: " + executable
		if arguments != "" {
			vector += ", arguments: " + arguments
		}
		vectors = append(vectors, vector+"}")
	}
	return "[" + strings.Join(vectors, ", ") + "]"
}

const toolchainEntry = `toolchains:
  entries:
    - id: formatter
      executable: formatter
      probe: [--version]
      parser: first-token
      version: 1.2.3
      required: true
`

func TestToolchainValidateRefusesAnUndeclaredSection(t *testing.T) {
	for _, format := range []struct {
		words   []string
		stream  string
		payload string
	}{
		{[]string{"toolchain", "validate"}, "rhino: toolchains section is not declared\n", ""},
		{[]string{"toolchain", "validate", "--output", "json"}, "", `"code":"rhino.config.undeclared","message":"toolchains section is not declared"`},
	} {
		tree := toolchainTree("policies: {}\n")
		outcome := runToolchain(tree, &recordingToolchainRunner{}, format.words...)
		if outcome.ExitCode != 2 {
			t.Fatalf("%v exited %d, want 2", format.words, outcome.ExitCode)
		}
		if format.stream != "" && outcome.Stderr != format.stream {
			t.Fatalf("%v stderr = %q, want %q", format.words, outcome.Stderr, format.stream)
		}
		if format.payload != "" && !strings.Contains(outcome.Stderr, format.payload) {
			t.Fatalf("%v stderr = %q, want it to contain %q", format.words, outcome.Stderr, format.payload)
		}
	}
}

func TestToolchainValidateRefusesInvalidPolicyBeforeAnyProbe(t *testing.T) {
	rows := []struct {
		name  string
		entry string
		cause string
	}{
		{"empty identifier", "id: ''\n      executable: formatter\n      probe: [--version]", "identifiers must be unique and non-empty"},
		{"leading-dash executable", "id: formatter\n      executable: -invalid\n      probe: [--version]", "has an invalid executable"},
		{"empty probe", "id: formatter\n      executable: formatter\n      probe: []", "declares no probe argv"},
		{"parser without version", "id: formatter\n      executable: formatter\n      probe: [--version]\n      parser: first-token", "must declare both parser and version, or neither"},
		{"version without parser", "id: formatter\n      executable: formatter\n      probe: [--version]\n      version: 1.2.3", "must declare both parser and version, or neither"},
		{"blank version", "id: formatter\n      executable: formatter\n      probe: [--version]\n      parser: first-token\n      version: '   '", "has an empty version constraint"},
		{"zero timeout", "id: formatter\n      executable: formatter\n      probe: [--version]\n      timeout-seconds: 0", "has a zero timeout"},
		{"duplicate provision platform", "id: formatter\n      executable: formatter\n      probe: [--version]\n      provision: [{platform: linux, executable: installer}, {platform: linux, executable: installer}]", "has a duplicate or invalid provision declaration"},
		{"uppercase provision platform", "id: formatter\n      executable: formatter\n      probe: [--version]\n      provision: [{platform: macOS, executable: installer}]", "has a duplicate or invalid provision declaration"},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			runner := &recordingToolchainRunner{}
			tree := toolchainTree("toolchains:\n  entries:\n    - " + row.entry + "\n")
			outcome := runToolchain(tree, runner, "toolchain", "validate")
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "toolchain policy:") || !strings.Contains(outcome.Stderr, row.cause) {
				t.Fatalf("stderr = %q (exit %d), want policy refusal naming %q", outcome.Stderr, outcome.ExitCode, row.cause)
			}
			if len(runner.launches) != 0 {
				t.Fatalf("policy refusal started %d processes, want none", len(runner.launches))
			}
		})
	}
}

func TestToolchainValidateReportsFindingsWithoutRepeatingToolOutput(t *testing.T) {
	rows := []struct {
		name     string
		answers  []toolchainAnswer
		required string
		exit     int
		rule     string
	}{
		{"unavailable probe", []toolchainAnswer{{failed: "offline"}}, "      required: true\n", 1, "unavailable"},
		{"failed probe", []toolchainAnswer{{code: 9, stdout: "SYNTHETIC=value"}}, "      required: true\n", 1, "probe-failed"},
		{"mismatched version", []toolchainAnswer{okProbe("9.9.9")}, "      required: true\n", 1, "version-mismatch"},
		{"optional failure stays clean", []toolchainAnswer{{failed: "offline"}}, "      required: false\n", 0, ""},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			runner := &recordingToolchainRunner{outcomes: row.answers}
			tree := toolchainTree("toolchains:\n  entries:\n    - id: formatter\n      executable: formatter\n      probe: [--version]\n      parser: first-token\n      version: 1.2.3\n" + row.required)
			outcome := runToolchain(tree, runner, "toolchain", "validate")
			if outcome.ExitCode != row.exit {
				t.Fatalf("exited %d, want %d (stdout %q stderr %q)", outcome.ExitCode, row.exit, outcome.Stdout, outcome.Stderr)
			}
			if row.exit == 1 {
				if !strings.Contains(outcome.Stderr, "[toolchain-validate] formatter: "+row.rule+"\n") {
					t.Fatalf("stderr = %q, want finding %q", outcome.Stderr, row.rule)
				}
			}
			if strings.Contains(outcome.Stdout, "SYNTHETIC") || strings.Contains(outcome.Stderr, "SYNTHETIC") {
				t.Fatalf("tool output leaked into the report: stdout %q stderr %q", outcome.Stdout, outcome.Stderr)
			}
		})
	}
}

func TestToolchainValidateRendersCleanAndFindingShapes(t *testing.T) {
	clean := toolchainTree(toolchainEntry)
	runner := &recordingToolchainRunner{outcomes: []toolchainAnswer{okProbe("1.2.3 extra")}}
	if outcome := runToolchain(clean, runner, "toolchain", "validate"); outcome.ExitCode != 0 ||
		outcome.Stdout != "[toolchain-validate] checked 1 declared toolchains, no findings\n" {
		t.Fatalf("clean text = %+v", outcome)
	}
	runner = &recordingToolchainRunner{outcomes: []toolchainAnswer{okProbe("1.2.3 extra")}}
	if outcome := runToolchain(clean, runner, "toolchain", "validate", "--output", "json"); outcome.ExitCode != 0 ||
		outcome.Stdout != "{\"command\":\"toolchain-validate\",\"exitCode\":0,\"inspected\":1,\"schemaVersion\":1}\n" {
		t.Fatalf("clean json = %+v", outcome)
	}
	runner = &recordingToolchainRunner{outcomes: []toolchainAnswer{{code: 9}}}
	if outcome := runToolchain(clean, runner, "toolchain", "validate", "--output", "json"); outcome.ExitCode != 1 ||
		outcome.Stdout != "{\"command\":\"toolchain-validate\",\"exitCode\":1,\"findings\":[{\"rule\":\"probe-failed\",\"toolchain\":\"formatter\"}],\"schemaVersion\":1}\n" {
		t.Fatalf("finding json = %+v", outcome)
	}
}

func TestToolchainValidateMatchesDeclaredVersionShapes(t *testing.T) {
	rows := []struct {
		name   string
		parser string
		stdout string
		clean  bool
	}{
		{"first line matches", "parser: first-line\nversion: 1.2.3", "1.2.3\nsecond", true},
		{"first line mismatches", "parser: first-line\nversion: 1.2.3", "9.9.9\nsecond", false},
		{"first token ignores trailing words", "parser: first-token\nversion: 1.2.3", "1.2.3 extra", true},
		{"empty probe output mismatches", "parser: first-token\nversion: 1.2.3", "", false},
		{"unconstrained probe always matches", "", "anything at all", true},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			body := "toolchains:\n  entries:\n    - id: formatter\n      executable: formatter\n      probe: [--version]\n      required: true\n"
			if row.parser != "" {
				body += "      " + strings.ReplaceAll(row.parser, "\n", "\n      ") + "\n"
			}
			runner := &recordingToolchainRunner{outcomes: []toolchainAnswer{okProbe(row.stdout)}}
			outcome := runToolchain(toolchainTree(body), runner, "toolchain", "validate")
			if row.clean && (outcome.ExitCode != 0 || !strings.Contains(outcome.Stdout, "no findings")) {
				t.Fatalf("stdout = %q (exit %d), want clean", outcome.Stdout, outcome.ExitCode)
			}
			if !row.clean && (outcome.ExitCode != 1 || !strings.Contains(outcome.Stderr, "version-mismatch")) {
				t.Fatalf("stderr = %q (exit %d), want version-mismatch", outcome.Stderr, outcome.ExitCode)
			}
		})
	}
}

func TestToolchainRefusesMalformedDeclarations(t *testing.T) {
	rows := []struct {
		name   string
		body   string
		reason string
	}{
		{"unknown group key", "toolchains:\n  surprises: true\n", "toolchains names unknown key"},
		{"entries are not a list", "toolchains:\n  entries: nope\n", "toolchains.entries names no entries"},
		{"unknown entry key", "toolchains:\n  entries:\n    - id: formatter\n      executable: formatter\n      probe: [--version]\n      surprises: true\n", "toolchains.entries names unknown key"},
		{"unknown provision key", "toolchains:\n  entries:\n    - id: formatter\n      executable: formatter\n      probe: [--version]\n      provision: [{platform: linux, executable: installer, surprises: true}]\n", "toolchains.provision names unknown key"},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			runner := &recordingToolchainRunner{}
			outcome := runToolchain(toolchainTree(row.body), runner, "toolchain", "validate")
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, row.reason) {
				t.Fatalf("stderr = %q (exit %d), want %q", outcome.Stderr, outcome.ExitCode, row.reason)
			}
			if len(runner.launches) != 0 {
				t.Fatalf("malformed policy started %d processes", len(runner.launches))
			}
		})
	}
}

func TestToolchainProvisionPlansEveryDeclaredVectorBeforeApplying(t *testing.T) {
	config := `toolchains:
  entries:
    - id: first
      executable: probe-tool
      probe: [--version]
      provision: ` + vectorsForEveryPlatform("installer-one", "[--literal=$VALUE]") + `
      required: true
    - id: skipped
      executable: probe-tool
      probe: [--version]
      required: true
    - id: second
      executable: probe-tool
      probe: [--version]
      provision: ` + vectorsForEveryPlatform("installer-two", "[--next]") + `
      required: true
`
	runner := &recordingToolchainRunner{}
	if outcome := runToolchain(toolchainTree(config), runner, "toolchain", "provision"); outcome.ExitCode != 0 ||
		outcome.Stdout != "[toolchain-provision] planned 2 declared toolchains; pass --apply to run\n" {
		t.Fatalf("plan text = %+v", outcome)
	}
	if len(runner.launches) != 0 {
		t.Fatalf("planning started %d processes, want none", len(runner.launches))
	}
	runner = &recordingToolchainRunner{}
	if outcome := runToolchain(toolchainTree(config), runner, "toolchain", "provision", "--output", "json"); outcome.ExitCode != 0 ||
		outcome.Stdout != "{\"command\":\"toolchain-provision\",\"count\":2,\"schemaVersion\":1,\"status\":\"planned\",\"toolchains\":[\"first\",\"second\"]}\n" {
		t.Fatalf("plan json = %+v", outcome)
	}
	if len(runner.launches) != 0 {
		t.Fatalf("json planning started %d processes, want none", len(runner.launches))
	}
	empty := &recordingToolchainRunner{}
	if outcome := runToolchain(toolchainTree("toolchains: {}\n"), empty, "toolchain", "provision"); outcome.ExitCode != 0 ||
		!strings.Contains(outcome.Stdout, "planned 0") {
		t.Fatalf("empty plan = %+v", outcome)
	}
	if outcome := runToolchain(toolchainTree("policies: {}\n"), empty, "toolchain", "provision", "--apply"); outcome.ExitCode != 2 ||
		!strings.Contains(outcome.Stderr, "toolchains section is not declared") {
		t.Fatalf("undeclared provision = %+v", outcome)
	}
}

func TestToolchainProvisionAppliesVectorsInOrderAndStopsAtFirstFailure(t *testing.T) {
	config := `toolchains:
  entries:
    - id: first
      executable: probe-tool
      probe: [--version]
      provision: ` + vectorsForEveryPlatform("installer-one", "") + `
      required: true
    - id: second
      executable: probe-tool
      probe: [--version]
      provision: ` + vectorsForEveryPlatform("installer-two", "") + `
      required: true
`
	runner := &recordingToolchainRunner{outcomes: []toolchainAnswer{okProbe(""), okProbe("")}}
	if outcome := runToolchain(toolchainTree(config), runner, "toolchain", "provision", "--apply"); outcome.ExitCode != 0 ||
		outcome.Stdout != "[toolchain-provision] completed 2 declared toolchains\n" {
		t.Fatalf("completed text = %+v", outcome)
	}
	if len(runner.launches) != 2 || runner.launches[0].executable != "installer-one" || runner.launches[1].executable != "installer-two" {
		t.Fatalf("launches = %+v, want installer-one then installer-two", runner.launches)
	}
	runner = &recordingToolchainRunner{outcomes: []toolchainAnswer{{code: 1}}}
	if outcome := runToolchain(toolchainTree(config), runner, "toolchain", "provision", "--apply"); outcome.ExitCode != 2 ||
		!strings.Contains(outcome.Stderr, "provision failed with status 1") {
		t.Fatalf("status failure = %+v", outcome)
	}
	if len(runner.launches) != 1 || runner.launches[0].executable != "installer-one" {
		t.Fatalf("failing apply launched %+v, want only installer-one", runner.launches)
	}
	single := toolchainTree(`toolchains:
  entries:
    - id: declared
      executable: probe-tool
      probe: [--version]
      provision: ` + vectorsForEveryPlatform("installer", "") + `
      required: true
`)
	runner = &recordingToolchainRunner{outcomes: []toolchainAnswer{okProbe("")}}
	if outcome := runToolchain(single, runner, "toolchain", "provision", "--apply", "--output", "json"); outcome.ExitCode != 0 ||
		outcome.Stdout != "{\"command\":\"toolchain-provision\",\"count\":1,\"schemaVersion\":1,\"status\":\"completed\"}\n" {
		t.Fatalf("completed json = %+v", outcome)
	}
}

func TestToolchainProvisionFailsClosedWithoutAProcessBoundary(t *testing.T) {
	tree := toolchainTree(`toolchains:
  entries:
    - id: declared
      executable: probe-tool
      probe: [--version]
      provision: ` + vectorsForEveryPlatform("installer", "") + `
      required: true
`)
	if outcome := application.Execute(tree, args("toolchain", "provision", "--apply"), nil); outcome.ExitCode != 2 ||
		!strings.Contains(outcome.Stderr, "toolchain `declared` could not start: this entry point has no toolchain process boundary") {
		t.Fatalf("unwired provision = %+v", outcome)
	}
}

func TestToolchainProvisionRefusesUnsupportedPlatformAndFailedVectors(t *testing.T) {
	unsupported := toolchainTree(`toolchains:
  entries:
    - id: unsupported
      executable: probe-tool
      probe: [--version]
      provision: [{platform: unavailable-platform, executable: installer}]
      required: true
`)
	if outcome := runToolchain(unsupported, &recordingToolchainRunner{}, "toolchain", "provision", "--apply"); outcome.ExitCode != 2 ||
		!strings.Contains(outcome.Stderr, "has no provision declaration for") {
		t.Fatalf("unsupported platform = %+v", outcome)
	}
	launchFailure := toolchainTree(`toolchains:
  entries:
    - id: declared
      executable: probe-tool
      probe: [--version]
      provision: ` + vectorsForEveryPlatform("installer", "[install]") + `
      required: true
`)
	runner := &recordingToolchainRunner{outcomes: []toolchainAnswer{{failed: "not found"}}}
	if outcome := runToolchain(launchFailure, runner, "toolchain", "provision", "--apply"); outcome.ExitCode != 2 ||
		!strings.Contains(outcome.Stderr, "could not start") {
		t.Fatalf("launch failure = %+v", outcome)
	}
	statusFailure := &recordingToolchainRunner{outcomes: []toolchainAnswer{{code: 1, stdout: "SYNTHETIC=value"}}}
	if outcome := runToolchain(launchFailure, statusFailure, "toolchain", "provision", "--apply"); outcome.ExitCode != 2 ||
		!strings.Contains(outcome.Stderr, "provision failed with status 1") ||
		strings.Contains(outcome.Stdout, "SYNTHETIC") || strings.Contains(outcome.Stderr, "SYNTHETIC") {
		t.Fatalf("status failure = %+v", outcome)
	}
}
