package application_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// mutationGateConfig declares one mutation gate that formats the staged
// files on pre-commit and replays the pull-request range on the CI surface.
const mutationGateConfig = "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: format-staged\n      type: mutation\n      inputs:\n        staged: { kind: files }\n      mutation:\n        local: apply-index\n        ci: verify-clean\n      command:\n        executable: ./format\n        args:\n          - { input: staged.paths, expand: repeat }\n      run-on:\n        pre-commit:\n          bind:\n            staged: { source: git-index }\n        pull-request:\n          bind:\n            staged: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: exact\n"

// recordingMutations is the fake mutation boundary: it records each call and
// answers with the outcome a test prepares.
type recordingMutations struct {
	applyCalls  []application.MutationLaunch
	verifyCalls []application.MutationLaunch
	applied     application.MutationOutcome
	verified    application.MutationOutcome
	err         error
}

func (r *recordingMutations) ApplyIndex(launch application.MutationLaunch) (application.MutationOutcome, error) {
	r.applyCalls = append(r.applyCalls, launch)
	return r.applied, r.err
}

func (r *recordingMutations) VerifyClean(launch application.MutationLaunch) (application.MutationOutcome, error) {
	r.verifyCalls = append(r.verifyCalls, launch)
	return r.verified, r.err
}

// mutationTree stages the index and the changed range the gate reads.
func mutationTree() *memoryTree {
	tree := gateTree(mutationGateConfig)
	tree.setIndexedFiles("a.md", "b.md")
	tree.setChangedFiles("aaaaaaa", "bbbbbbb", "c.md")
	return tree
}

func runMutationGate(tree *memoryTree, mutations application.MutationRunner, launcher *stubLauncher, flags ...string) application.Outcome {
	return application.ExecuteWithMutations(tree, args(append([]string{"gate", "run"}, flags...)...), nil,
		launcher, nil, nil, nil, nil, mutations)
}

// TestMutationGateAppliesToTheIndexOnPreCommit proves a pre-commit mutation
// hands the Git index selection to the apply-index boundary, never the child
// launcher, and reports each changed path.
func TestMutationGateAppliesToTheIndexOnPreCommit(t *testing.T) {
	tree := mutationTree()
	launcher := &stubLauncher{}
	mutations := &recordingMutations{applied: application.MutationOutcome{Changes: []string{"a.md"}}}
	outcome := runMutationGate(tree, mutations, launcher, "--surface", "pre-commit")
	if outcome.ExitCode != 0 {
		t.Fatalf("mutation did not pass: %+v", outcome)
	}
	if !strings.Contains(outcome.Stdout, "[gate] format-staged changed a.md\n") ||
		!strings.Contains(outcome.Stdout, "[gate] format-staged passed\n") {
		t.Fatalf("changes not reported: %q", outcome.Stdout)
	}
	if len(launcher.seen) != 0 {
		t.Fatalf("a mutation started its child through the ordinary launcher: %v", launcher.seen)
	}
	if len(mutations.verifyCalls) != 0 || len(mutations.applyCalls) != 1 {
		t.Fatalf("wrong boundary called: apply=%d verify=%d", len(mutations.applyCalls), len(mutations.verifyCalls))
	}
	call := mutations.applyCalls[0]
	if strings.Join(call.Argv, " ") != "./format a.md b.md" {
		t.Fatalf("argv not projected: %q", call.Argv)
	}
	if strings.Join(call.SelectedPaths, ",") != "a.md,b.md" {
		t.Fatalf("selection not the index selection: %q", call.SelectedPaths)
	}
	if call.Directory != "/root" || call.Revision != "" {
		t.Fatalf("directory or revision wrong: %+v", call)
	}
}

// TestMutationGateVerifiesTheRevisionOnPullRequest proves the pull-request
// surface replays the candidate revision through verify-clean with the
// reviewed changed paths, never the apply-index boundary.
func TestMutationGateVerifiesTheRevisionOnPullRequest(t *testing.T) {
	tree := mutationTree()
	mutations := &recordingMutations{}
	outcome := runMutationGate(tree, mutations, &stubLauncher{},
		"--surface", "pull-request", "--base", "aaaaaaa", "--head", "bbbbbbb")
	if outcome.ExitCode != 0 || !strings.Contains(outcome.Stdout, "[gate] format-staged passed\n") {
		t.Fatalf("replay did not pass: %+v", outcome)
	}
	if len(mutations.applyCalls) != 0 || len(mutations.verifyCalls) != 1 {
		t.Fatalf("wrong boundary called: apply=%d verify=%d", len(mutations.applyCalls), len(mutations.verifyCalls))
	}
	call := mutations.verifyCalls[0]
	if call.Revision != "bbbbbbb" || strings.Join(call.SelectedPaths, ",") != "c.md" {
		t.Fatalf("revision or selection wrong: %+v", call)
	}
}

// TestMutationGateFindingExitsOne proves a mutator that exits nonzero is a
// finding, not a refusal: the run exits 1 and names the surface.
func TestMutationGateFindingExitsOne(t *testing.T) {
	tree := mutationTree()
	mutations := &recordingMutations{applied: application.MutationOutcome{Code: 3}}
	outcome := runMutationGate(tree, mutations, &stubLauncher{}, "--surface", "pre-commit")
	if outcome.ExitCode != 1 {
		t.Fatalf("finding did not exit 1: %+v", outcome)
	}
	if !strings.Contains(outcome.Stdout, "[gate] format-staged failed\n") ||
		!strings.Contains(outcome.Stderr, "[gate] format-staged reported a finding at pre-commit\n") {
		t.Fatalf("finding not reported: %+v", outcome)
	}
}

// TestMutationGateReportsPreservedUnstagedEdits proves a path whose working
// tree held unstaged edits is named as preserved, and the gate still passes.
func TestMutationGateReportsPreservedUnstagedEdits(t *testing.T) {
	tree := mutationTree()
	mutations := &recordingMutations{applied: application.MutationOutcome{
		Changes:     []string{"a.md"},
		Divergences: []string{"b.md"},
	}}
	outcome := runMutationGate(tree, mutations, &stubLauncher{}, "--surface", "pre-commit")
	if outcome.ExitCode != 0 || !strings.Contains(outcome.Stdout, "[gate] format-staged preserved unstaged b.md\n") {
		t.Fatalf("divergence not reported: %+v", outcome)
	}
}

// TestMutationGateRunnerRefusalKeepsProgress proves a boundary refusal exits 2
// with the reason, and keeps completed progress on stderr in text mode only.
func TestMutationGateRunnerRefusalKeepsProgress(t *testing.T) {
	tree := mutationTree()
	mutations := &recordingMutations{err: errors.New("mutator changed `x.md` outside its selected index paths")}
	outcome := runMutationGate(tree, mutations, &stubLauncher{}, "--surface", "pre-commit")
	if outcome.ExitCode != 2 || outcome.Stdout != "" {
		t.Fatalf("refusal not exit 2 with empty stdout: %+v", outcome)
	}
	if !strings.Contains(outcome.Stderr, "[gate] format-staged could not run\n") ||
		!strings.Contains(outcome.Stderr, "gate `format-staged`: mutator changed `x.md` outside its selected index paths") {
		t.Fatalf("refusal reason missing: %+v", outcome)
	}
	json := runMutationGate(tree, mutations, &stubLauncher{}, "--surface", "pre-commit", "--output", "json")
	if json.ExitCode != 2 || strings.Contains(json.Stderr, "[gate]") || !strings.Contains(json.Stderr, "rhino.gate.child-refused") {
		t.Fatalf("json refusal carried progress or no code: %+v", json)
	}
}

// TestMutationGateSkipsAnEmptyRegularFileSelection proves a regular-files
// mutation with nothing selected passes without calling any boundary.
func TestMutationGateSkipsAnEmptyRegularFileSelection(t *testing.T) {
	tree := gateTree(strings.Replace(mutationGateConfig, "kind: files", "kind: regular-files", 1))
	tree.setIndexedRegularFiles()
	mutations := &recordingMutations{}
	outcome := runMutationGate(tree, mutations, &stubLauncher{}, "--surface", "pre-commit")
	if outcome.ExitCode != 0 || !strings.Contains(outcome.Stdout, "[gate] format-staged passed\n") {
		t.Fatalf("empty selection did not pass: %+v", outcome)
	}
	if len(mutations.applyCalls) != 0 {
		t.Fatalf("empty selection reached the boundary: %d", len(mutations.applyCalls))
	}
}

// TestMutationGateWithoutABoundaryRefusesBeforeAnyWrite proves an entry point
// that wires no mutation boundary refuses in the oracle's words, and starts no
// child.
func TestMutationGateWithoutABoundaryRefusesBeforeAnyWrite(t *testing.T) {
	tree := mutationTree()
	launcher := &stubLauncher{}
	outcome := application.ExecuteWithLauncher(tree, args("gate", "run", "--surface", "pre-commit"), nil, launcher)
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "no index mutation boundary") {
		t.Fatalf("unwired boundary not refused: %+v", outcome)
	}
	if len(launcher.seen) != 0 {
		t.Fatalf("child started without a boundary: %v", launcher.seen)
	}
	replay := application.ExecuteWithLauncher(tree,
		args("gate", "run", "--surface", "pull-request", "--base", "aaaaaaa", "--head", "bbbbbbb"), nil, launcher)
	if replay.ExitCode != 2 || !strings.Contains(replay.Stderr, "no disposable pull-request replay boundary") {
		t.Fatalf("unwired replay not refused: %+v", replay)
	}
}

// TestMutationGateProjectsItsEnvironment proves the declared environment
// projection reaches the boundary, so a mutator sees the same contract a
// check child sees.
func TestMutationGateProjectsItsEnvironment(t *testing.T) {
	config := strings.Replace(mutationGateConfig, "        args:\n", "        environment:\n          FORMAT_MODE: { input: staged.paths }\n        args:\n", 1)
	tree := gateTree(config)
	tree.setIndexedFiles("only.md")
	mutations := &recordingMutations{}
	outcome := runMutationGate(tree, mutations, &stubLauncher{}, "--surface", "pre-commit")
	if outcome.ExitCode != 0 || len(mutations.applyCalls) != 1 {
		t.Fatalf("mutation did not reach its boundary: %+v calls=%d", outcome, len(mutations.applyCalls))
	}
	if mutations.applyCalls[0].Environment["FORMAT_MODE"] != "only.md" {
		t.Fatalf("environment not projected: %v", mutations.applyCalls[0].Environment)
	}
}

// TestMutationGateJSONCarriesChanges proves a clean mutation run names the
// changed paths in the JSON document, under the documented `changes` field.
func TestMutationGateJSONCarriesChanges(t *testing.T) {
	tree := mutationTree()
	mutations := &recordingMutations{applied: application.MutationOutcome{Changes: []string{"a.md"}}}
	outcome := runMutationGate(tree, mutations, &stubLauncher{}, "--surface", "pre-commit", "--output", "json")
	want := "{\"changes\":[\"a.md\"],\"command\":[\"gate\",\"run\"],\"completed\":[\"format-staged\"],\"findings\":[],\"metadata\":{\"gates\":[\"format-staged\"]},\"result\":\"clean\",\"schemaVersion\":1,\"summary\":{\"changed\":1,\"findings\":0,\"scanned\":1},\"surface\":\"pre-commit\"}\n"
	if outcome.ExitCode != 0 || outcome.Stdout != want {
		t.Fatalf("clean document not the documented shape:\n got %q\nwant %q", outcome.Stdout, want)
	}
}

// TestMutationGateJSONFindingNamesTheGateOnce proves a finding in JSON mode
// reports one document and one stderr line that names the gate once, as the
// JSON reference documents.
func TestMutationGateJSONFindingNamesTheGateOnce(t *testing.T) {
	tree := mutationTree()
	mutations := &recordingMutations{applied: application.MutationOutcome{Code: 3, Changes: []string{"a.md"}}}
	outcome := runMutationGate(tree, mutations, &stubLauncher{}, "--surface", "pre-commit", "--output", "json")
	wantStdout := "{\"changes\":[\"a.md\"],\"command\":[\"gate\",\"run\"],\"completed\":[],\"findings\":[],\"metadata\":{\"gates\":[]},\"result\":\"findings\",\"schemaVersion\":1,\"summary\":{\"changed\":1,\"findings\":1,\"scanned\":0},\"surface\":\"pre-commit\"}\n"
	if outcome.ExitCode != 1 || outcome.Stdout != wantStdout {
		t.Fatalf("finding document wrong: %+v", outcome)
	}
	if outcome.Stderr != "[gate] format-staged reported a finding at pre-commit\n" {
		t.Fatalf("finding line wrong: %q", outcome.Stderr)
	}
}
