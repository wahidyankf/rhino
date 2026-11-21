package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
	"github.com/wahidyankf/rhino/internal/domain"
)

// wiringWorkflowWith is a pull-request workflow whose one job runs the given steps.
func wiringWorkflowWith(steps string) string {
	return "on:\n  pull_request:\njobs:\n  quality-gate:\n    runs-on: ubuntu-24.04\n    steps:\n" + steps
}

// wiringFilesWith returns the accepted tree with the given files replaced or added.
func wiringFilesWith(replace map[string]string) map[string]string {
	files := wiringFiles()
	for path, content := range replace {
		files[path] = content
	}
	return files
}

// TestGateWiringSuppressedCallsAreReported proves W05: each way of hiding a failed
// gate call is a finding at the line that hides it.
func TestGateWiringSuppressedCallsAreReported(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
	}{
		{name: "or-true on a hook call", files: wiringFilesWith(map[string]string{
			".husky/pre-commit": "exec go run ./cmd/rhino gate run --surface pre-commit || true\n"})},
		{name: "or-colon on a hook call", files: wiringFilesWith(map[string]string{
			".husky/pre-push": "exec go run ./cmd/rhino gate run --surface pre-push --push-updates-stdin || :\n"})},
		{name: "errexit off around a hook call", files: wiringFilesWith(map[string]string{
			".husky/pre-commit": "set +e\nexec go run ./cmd/rhino gate run --surface pre-commit\n"})},
		{name: "continue-on-error on a workflow step that calls gate run", files: wiringFilesWith(map[string]string{
			".github/workflows/pr-quality-gate.yml": wiringWorkflowWith(
				"      - run: go run ./cmd/rhino gate run --surface pull-request\n        continue-on-error: true\n")})},
		{name: "errexit off in a literal workflow step", files: wiringFilesWith(map[string]string{
			".github/workflows/pr-quality-gate.yml": wiringWorkflowWith(
				"      - run: |\n          set +e\n          go run ./cmd/rhino gate run --surface pull-request\n")})},
		{name: "or-true on a workflow call", files: wiringFilesWith(map[string]string{
			".github/workflows/pr-quality-gate.yml": wiringWorkflowWith(
				"      - run: go run ./cmd/rhino gate run --surface pull-request || true\n")})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := runGateWiring(t, wiringConfig, tc.files, "--output", "json")
			if result.exit != 1 || !wiringHasKind(result.kinds, "W05") {
				t.Fatalf("exit %d kinds %v; want exit 1 with W05\nstdout: %s", result.exit, result.kinds, result.stdout)
			}
		})
	}
}

// TestGateWiringReleaseLockRule proves W06: a release a workflow names must match
// the lock file's version, and a line that only reads the lock file is not a
// release statement.
func TestGateWiringReleaseLockRule(t *testing.T) {
	lock := map[string]string{"hippo.lock": "version=v0.13.4\n"}
	mismatch := runGateWiring(t, wiringConfig, wiringFilesWith(mergeFiles(lock, map[string]string{
		".github/workflows/pr-quality-gate.yml": wiringWorkflowWith(
			"      - run: go run ./cmd/rhino gate run --surface pull-request\n" +
				"      - run: hippo install v0.12.0\n"),
	})), "--output", "json")
	if mismatch.exit != 1 || !wiringHasKind(mismatch.kinds, "W06") {
		t.Fatalf("mismatched release: exit %d kinds %v; want exit 1 with W06", mismatch.exit, mismatch.kinds)
	}

	reading := runGateWiring(t, wiringConfig, wiringFilesWith(mergeFiles(lock, map[string]string{
		".github/workflows/pr-quality-gate.yml": wiringWorkflowWith(
			"      - run: go run ./cmd/rhino gate run --surface pull-request\n" +
				"      - run: cat hippo.lock\n"),
	})), "--output", "json")
	if reading.exit != 0 {
		t.Fatalf("a step that reads the lock file: exit %d; want 0", reading.exit)
	}

	unversioned := runGateWiring(t, wiringConfig, wiringFilesWith(mergeFiles(map[string]string{"hippo.lock": "name=hippo\n"}, nil)), "--output", "json")
	if unversioned.exit != 0 {
		t.Fatalf("a lock file with no version line: exit %d; want 0", unversioned.exit)
	}
}

// mergeFiles returns one map holding both sets of files; the second wins.
func mergeFiles(first, second map[string]string) map[string]string {
	merged := map[string]string{}
	for path, content := range first {
		merged[path] = content
	}
	for path, content := range second {
		merged[path] = content
	}
	return merged
}

// wiringTriggerConfig declares one entry on each triggered surface, so every
// trigger shape a workflow may write is read against a surface that needs it.
const wiringTriggerConfig = "schema: rhino/repo-config/v2\n" +
	"gates:\n" +
	"  entries:\n" +
	"    - id: main-check\n" +
	"      command:\n" +
	"        executable: checker\n" +
	"      run-on:\n" +
	"        main: {}\n" +
	"    - id: nightly-check\n" +
	"      command:\n" +
	"        executable: checker\n" +
	"      run-on:\n" +
	"        scheduled: {}\n" +
	"    - id: manual-check\n" +
	"      command:\n" +
	"        executable: checker\n" +
	"      run-on:\n" +
	"        manual: {}\n" +
	"  wiring:\n" +
	"    hooks:\n" +
	"      directory: .husky\n" +
	"    ci:\n" +
	"      directory: .github/workflows\n"

// wiringTriggerSteps calls each triggered surface from one workflow step.
const wiringTriggerSteps = "      - run: go run ./cmd/rhino gate run --surface main\n" +
	"      - run: go run ./cmd/rhino gate run --surface scheduled\n" +
	"      - run: go run ./cmd/rhino gate run --surface manual\n"

// TestGateWiringTriggerShapes proves W01 reads every `on:` shape: a mapping with
// push branches, a schedule, and a manual dispatch passes, while a trigger that is
// missing or names another branch leaves its surface uncalled.
func TestGateWiringTriggerShapes(t *testing.T) {
	cases := []struct {
		name string
		on   string
		exit int
		kind string
	}{
		{name: "a mapping with push branches main, schedule, and dispatch", exit: 0,
			on: "on:\n  push:\n    branches: [main]\n  schedule:\n    - cron: \"0 0 * * *\"\n  workflow_dispatch:\n"},
		{name: "a scalar trigger leaves main uncalled", exit: 1, kind: "W01",
			on: "on: workflow_dispatch\n"},
		{name: "a sequence of triggers without push branches leaves main uncalled", exit: 1, kind: "W01",
			on: "on: [push, schedule]\n"},
		{name: "a push on another branch leaves main uncalled", exit: 1, kind: "W01",
			on: "on:\n  push:\n    branches: [dev]\n  schedule:\n  workflow_dispatch:\n"},
		{name: "a push with no branch list leaves main uncalled", exit: 1, kind: "W01",
			on: "on:\n  push:\n  workflow_dispatch:\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workflow := tc.on + "jobs:\n  run:\n    steps:\n" + wiringTriggerSteps
			files := map[string]string{".github/workflows/triggers.yml": workflow}
			result := runGateWiring(t, wiringTriggerConfig, files, "--output", "json")
			if result.exit != tc.exit {
				t.Fatalf("exit %d; want %d\nstdout: %s\nstderr: %s", result.exit, tc.exit, result.stdout, result.stderr)
			}
			if tc.kind != "" && !wiringHasKind(result.kinds, tc.kind) {
				t.Fatalf("kinds %v do not include %s", result.kinds, tc.kind)
			}
		})
	}
}

// TestGateWiringRefusesUnreadableFiles proves a hook, a workflow, or a lock file
// the rules cannot read refuses the run with its closed code.
func TestGateWiringRefusesUnreadableFiles(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*memoryTree)
		message string
	}{
		{name: "an unreadable hook", message: ".husky/pre-push", mutate: func(tree *memoryTree) {
			tree.unreadable[".husky/pre-push"] = true
		}},
		{name: "a hook that holds no text", message: ".husky/pre-push: holds no text", mutate: func(tree *memoryTree) {
			tree.binary[".husky/pre-push"] = true
		}},
		{name: "an unreadable lock file", message: "hippo.lock", mutate: func(tree *memoryTree) {
			tree.files["hippo.lock"] = "version=v0.13.4\n"
			tree.unreadable["hippo.lock"] = true
		}},
		{name: "an unreadable workflow", message: ".github/workflows/pr-quality-gate.yml", mutate: func(tree *memoryTree) {
			tree.unreadable[".github/workflows/pr-quality-gate.yml"] = true
		}},
		{name: "a workflow that is not YAML", message: ".github/workflows/pr-quality-gate.yml: holds no readable YAML",
			mutate: func(tree *memoryTree) {
				tree.files[".github/workflows/pr-quality-gate.yml"] = "on: [push\n"
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newMemoryTree("/root")
			tree.files["repo-config.yml"] = wiringConfig
			for path, content := range wiringFiles() {
				tree.files[path] = content
			}
			tc.mutate(tree)
			result := runOnTree(t, tree, []string{"gate", "wiring", "validate"}, "--output", "json")
			if result.exit != 2 || !strings.Contains(result.message, tc.message) {
				t.Fatalf("exit %d message %q; want exit 2 naming %q", result.exit, result.message, tc.message)
			}
			if result.code != domain.FileUnreadable.String() {
				t.Fatalf("code %q; want %q", result.code, domain.FileUnreadable.String())
			}
		})
	}
}

// TestGateWiringReadsEveryWorkflowShape proves the workflow and hook reader accepts
// the shapes a repository writes without a finding: empty files, jobs and steps
// of the wrong kind, a step with no run text, and a hook whose gate call is only
// in a comment, which makes no call at all.
func TestGateWiringReadsEveryWorkflowShape(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		exit  int
		kind  string
	}{
		{name: "an empty workflow makes no call", exit: 1, kind: "W03",
			files: wiringFilesWith(map[string]string{".github/workflows/pr-quality-gate.yml": ""})},
		{name: "jobs that are not a mapping make no call", exit: 1, kind: "W03",
			files: wiringFilesWith(map[string]string{".github/workflows/pr-quality-gate.yml": "on:\n  pull_request:\njobs: [1]\n"})},
		{name: "steps that are not a sequence make no call", exit: 1, kind: "W03",
			files: wiringFilesWith(map[string]string{".github/workflows/pr-quality-gate.yml": "on:\n  pull_request:\njobs:\n  q:\n    steps: x\n"})},
		{name: "a step whose run is a mapping is skipped", exit: 1, kind: "W03",
			files: wiringFilesWith(map[string]string{".github/workflows/pr-quality-gate.yml": wiringWorkflowWith(
				"      - run: {name: not-a-command}\n")})},
		{name: "a commented gate call is no call", exit: 1, kind: "W01",
			files: wiringFilesWith(map[string]string{".husky/pre-commit": "# exec go run ./cmd/rhino gate run --surface pre-commit\n"})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := runGateWiring(t, wiringConfig, tc.files, "--output", "json")
			if result.exit != tc.exit || !wiringHasKind(result.kinds, tc.kind) {
				t.Fatalf("exit %d kinds %v; want exit %d with %s\nstdout: %s\nstderr: %s",
					result.exit, result.kinds, tc.exit, tc.kind, result.stdout, result.stderr)
			}
		})
	}
}

// TestGateWiringHookCallIsNotAWorkflowCall proves W03 counts only a workflow step as
// the pull-request caller: a hook that calls the surface does not satisfy it.
func TestGateWiringHookCallIsNotAWorkflowCall(t *testing.T) {
	files := wiringFilesWith(map[string]string{
		".husky/pre-commit":                     "exec go run ./cmd/rhino gate run --surface pull-request\n",
		".github/workflows/pr-quality-gate.yml": wiringWorkflowWith("      - run: echo no gate here\n"),
	})
	result := runGateWiring(t, wiringConfig, files, "--output", "json")
	if result.exit != 1 || !wiringHasKind(result.kinds, "W03") {
		t.Fatalf("exit %d kinds %v; want exit 1 with W03", result.exit, result.kinds)
	}
}

// TestGateWiringTextSortsFindings proves the text rendering names every finding on
// its own line, in the contract's order, when a run holds more than one.
func TestGateWiringTextSortsFindings(t *testing.T) {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = wiringConfig
	for path, content := range wiringFilesWith(map[string]string{
		".husky/pre-push":   "exec echo nothing\n",
		".husky/pre-rebase": "exec go run ./cmd/rhino gate run --surface main\n",
	}) {
		tree.files[path] = content
	}
	outcome := application.Execute(tree, []string{"gate", "wiring", "validate"}, nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("exit %d; want 1\nstderr: %s", outcome.ExitCode, outcome.Stderr)
	}
	lines := strings.Split(strings.TrimSpace(outcome.Stderr), "\n")
	if len(lines) != 2 || !strings.Contains(outcome.Stderr, "W01") || !strings.Contains(outcome.Stderr, "W02") {
		t.Fatalf("want one W01 and one W02 line, got:\n%s", outcome.Stderr)
	}
}
