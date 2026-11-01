package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// regularRangeGateConfig binds one regular-files input to an explicit range on
// the pull-request replay.
const regularRangeGateConfig = "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: regular-check\n      type: check\n      inputs:\n        files: { kind: regular-files }\n      command:\n        executable: ./check\n        args:\n          - { input: files.paths, expand: repeat }\n      run-on:\n        pull-request:\n          reason: runs only in the disposable pull-request replay\n          bind:\n            files: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: at-least\n"

// literalGateConfig projects one literal argument into the argv.
const literalGateConfig = "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: literal-check\n      command:\n        executable: ./check\n        args:\n          - {literal: --strict}\n      run-on:\n        pre-commit: {}\n        pull-request: {}\n  composition:\n    pull-request:\n      relation: exact\n"

// checkoutTreeGateConfig binds a files input to the checked-out tree on both
// the local and the replay surface.
const checkoutTreeGateConfig = "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: tree-check\n      type: check\n      inputs:\n        tree: { kind: files }\n      command:\n        executable: ./check\n        args:\n          - { input: tree.paths, expand: repeat }\n      run-on:\n        pre-commit:\n          bind:\n            tree: { source: checkout }\n        pull-request:\n          bind:\n            tree: { source: checkout }\n  composition:\n    pull-request:\n      relation: exact\n"

// mutationCheckoutConfig binds the mutation's files to the checked-out tree, so
// argv sees every file but the write set must stay empty.
const mutationCheckoutConfig = "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: format-tree\n      type: mutation\n      inputs:\n        staged: { kind: files }\n      mutation:\n        local: apply-index\n        ci: verify-clean\n      command:\n        executable: ./format\n        args:\n          - { input: staged.paths, expand: repeat }\n      run-on:\n        pre-commit:\n          bind:\n            staged: { source: checkout }\n        pull-request:\n          bind:\n            staged: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: exact\n"

// mutationMessageConfig adds a commit-message input the write set must ignore.
const mutationMessageConfig = "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: format-staged\n      type: mutation\n      inputs:\n        staged: { kind: files }\n        messages: { kind: commit-message }\n      mutation:\n        local: apply-index\n        ci: verify-clean\n      command:\n        executable: ./format\n        args:\n          - { input: staged.paths, expand: repeat }\n          - { input: messages.text, expand: single }\n      run-on:\n        pre-commit:\n          bind:\n            staged: { source: git-index }\n            messages: { source: hook-message-file }\n        pull-request:\n          bind:\n            staged: { source: explicit-range, range: explicit }\n            messages: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: exact\n"

// TestGateRunRefusesAnIncompleteOrNonCommitRange proves each kind of input that
// reads an explicit range refuses before any child starts when the range is
// missing a side or names something that is not a commit ID.
func TestGateRunRefusesAnIncompleteOrNonCommitRange(t *testing.T) {
	cases := []struct {
		name   string
		config string
		flags  []string
		reason string
	}{
		{
			name:   "a file range without both sides refuses as incomplete",
			config: explicitFilesGateConfig,
			flags:  []string{"--surface", "pull-request"},
			reason: "requires both `--base` and `--head` for an explicit file range",
		},
		{
			name:   "a regular-file range without both sides refuses as incomplete",
			config: regularRangeGateConfig,
			flags:  []string{"--surface", "pull-request"},
			reason: "requires both `--base` and `--head` for an explicit regular-file range",
		},
		{
			name:   "a regular-file range over a non-commit base refuses as unrecognized",
			config: regularRangeGateConfig,
			flags:  []string{"--surface", "pull-request", "--base", "not-a-commit", "--head", "bbbbbbb"},
			reason: "requires hexadecimal commit IDs for `--base` and `--head`",
		},
		{
			name:   "a commit-message range without both sides refuses as incomplete",
			config: commitMsgGateConfig,
			flags:  []string{"--surface", "pull-request"},
			reason: "requires both `--base` and `--head` for an explicit commit range",
		},
		{
			name:   "a commit-message range over a non-commit head refuses as unrecognized",
			config: commitMsgGateConfig,
			flags:  []string{"--surface", "pull-request", "--base", "aaaaaaa", "--head", "not-a-commit"},
			reason: "requires hexadecimal commit IDs for `--base` and `--head`",
		},
		{
			name:   "a commit range without both sides refuses as incomplete",
			config: pushGateConfig,
			flags:  []string{"--surface", "pull-request"},
			reason: "requires both `--base` and `--head` for an explicit commit range",
		},
		{
			name:   "a commit range over a non-commit base refuses as unrecognized",
			config: pushGateConfig,
			flags:  []string{"--surface", "pull-request", "--base", "not-a-commit", "--head", "bbbbbbb"},
			reason: "requires hexadecimal commit IDs for `--base` and `--head`",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := gateTree(tc.config)
			launcher := &stubLauncher{}
			outcome := application.ExecuteWithLauncher(tree, args(append([]string{"gate", "run"}, tc.flags...)...), nil, launcher)
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
			if len(launcher.seen) != 0 {
				t.Fatalf("a refused range started a child: %v", launcher.seen)
			}
		})
	}
}

// TestGateRunProjectsLiteralArguments proves a literal argument travels into
// argv untouched, the one argument form no input reference supplies.
func TestGateRunProjectsLiteralArguments(t *testing.T) {
	tree := gateTree(literalGateConfig)
	launcher := &stubLauncher{}
	outcome := application.ExecuteWithLauncher(tree, args("gate", "run", "--surface", "pre-commit"), nil, launcher)
	if outcome.ExitCode != 0 {
		t.Fatalf("literal gate did not pass: %+v", outcome)
	}
	if len(launcher.seen) != 1 || strings.Join(launcher.seen[0], " ") != "./check --strict" {
		t.Fatalf("literal argument not projected: %v", launcher.seen)
	}
}

// TestGateRunProjectsEveryFileFromACheckoutBinding proves a files input bound
// to the checkout expands to every file the tree holds, sorted.
func TestGateRunProjectsEveryFileFromACheckoutBinding(t *testing.T) {
	tree := gateTree(checkoutTreeGateConfig)
	tree.files["docs/a.md"] = "# A\n"
	launcher := &stubLauncher{}
	outcome := application.ExecuteWithLauncher(tree, args("gate", "run", "--surface", "pre-commit"), nil, launcher)
	if outcome.ExitCode != 0 {
		t.Fatalf("checkout gate did not pass: %+v", outcome)
	}
	if len(launcher.seen) != 1 || strings.Join(launcher.seen[0], " ") != "./check docs/a.md repo-config.yml" {
		t.Fatalf("checkout files not projected: %v", launcher.seen)
	}
}

// TestMutationWriteSetIgnoresCheckoutBindings proves a mutation whose files
// come from the checkout cannot widen its write set: argv may name the tree,
// but the boundary is handed an empty selection to write.
func TestMutationWriteSetIgnoresCheckoutBindings(t *testing.T) {
	tree := gateTree(mutationCheckoutConfig)
	tree.files["a.md"] = "# A\n"
	mutations := &recordingMutations{}
	outcome := runMutationGate(tree, mutations, &stubLauncher{}, "--surface", "pre-commit")
	if outcome.ExitCode != 0 {
		t.Fatalf("checkout mutation did not pass: %+v", outcome)
	}
	if len(mutations.applyCalls) != 1 {
		t.Fatalf("apply-index not called once: %d", len(mutations.applyCalls))
	}
	if len(mutations.applyCalls[0].SelectedPaths) != 0 {
		t.Fatalf("checkout binding widened the write set: %q", mutations.applyCalls[0].SelectedPaths)
	}
}

// TestMutationWriteSetIgnoresCommitMessageBindings proves a commit-message input
// bound to an explicit range contributes no paths to the replay's write set,
// while the file range it sits beside still does.
func TestMutationWriteSetIgnoresCommitMessageBindings(t *testing.T) {
	tree := gateTree(mutationMessageConfig)
	tree.setChangedFiles("aaaaaaa", "bbbbbbb", "c.md")
	tree.setCommitMessages("aaaaaaa", "bbbbbbb", "feat: a message\n")
	mutations := &recordingMutations{}
	outcome := runMutationGate(tree, mutations, &stubLauncher{},
		"--surface", "pull-request", "--base", "aaaaaaa", "--head", "bbbbbbb")
	if outcome.ExitCode != 0 {
		t.Fatalf("message mutation did not pass: %+v", outcome)
	}
	if len(mutations.verifyCalls) != 1 {
		t.Fatalf("verify-clean not called once: %d", len(mutations.verifyCalls))
	}
	if strings.Join(mutations.verifyCalls[0].SelectedPaths, ",") != "c.md" {
		t.Fatalf("write set not the file range alone: %q", mutations.verifyCalls[0].SelectedPaths)
	}
}
