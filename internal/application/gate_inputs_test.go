package application_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/runtime"
	"github.com/wahidyankf/rhino/internal/application"
	"github.com/wahidyankf/rhino/internal/domain"
)

const commitMsgGateConfig = "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: msg-check\n      type: check\n      inputs:\n        message: { kind: commit-message }\n      command:\n        executable: ./check\n        args:\n          - { input: message.text, expand: single }\n        environment:\n          CHECK_MESSAGE: { input: message.text }\n      run-on:\n        commit-msg:\n          bind:\n            message: { source: hook-message-file }\n        pull-request:\n          bind:\n            message: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: exact\n"

const pushGateConfig = "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: range-check\n      type: check\n      inputs:\n        range: { kind: commit-range }\n      command:\n        executable: ./check\n        args:\n          - { input: range.base, expand: single }\n          - { input: range.head, expand: single }\n        environment:\n          NX_BASE: { input: range.base }\n          NX_HEAD: { input: range.head }\n      run-on:\n        pre-push:\n          bind:\n            range: { source: push-updates, fallback: refs/remotes/origin/main }\n        pull-request:\n          bind:\n            range: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: exact\n"

const explicitFilesGateConfig = "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: files-check\n      type: check\n      inputs:\n        files: { kind: files }\n      command:\n        executable: ./check\n        args:\n          - { input: files.paths, expand: repeat }\n      run-on:\n        pull-request:\n          reason: runs only in the disposable pull-request replay\n          bind:\n            files: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: at-least\n"

const indexGateConfig = "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: index-check\n      type: check\n      inputs:\n        staged: { kind: files }\n        state: { kind: repository-state }\n      command:\n        executable: ./check\n        args:\n          - { input: staged.paths, expand: repeat }\n          - { input: state.root, expand: single }\n      run-on:\n        pre-commit:\n          bind:\n            staged: { source: git-index }\n            state: { source: checkout }\n        pull-request:\n          bind:\n            staged: { source: explicit-range, range: explicit }\n            state: { source: checkout }\n  composition:\n    pull-request:\n      relation: exact\n"

// hookTree stages a commit-msg tree whose canonical message file holds text.
func hookTree(text string) *memoryTree {
	tree := gateTree(commitMsgGateConfig)
	tree.files["COMMIT_EDITMSG"] = text
	tree.setHookMessageFile("COMMIT_EDITMSG")
	return tree
}

// TestGateRunResolvesCommitMessageHookFile proves a commit-msg gate projects
// the hook message into argv and the environment, the scenario the rollout
// proved broken.
func TestGateRunResolvesCommitMessageHookFile(t *testing.T) {
	tree := hookTree("feat: checked message\n")
	launcher := &stubLauncher{}
	outcome := application.ExecuteWithLauncher(tree,
		args("gate", "run", "--surface", "commit-msg", "--message-file", "COMMIT_EDITMSG"), nil, launcher)
	if outcome.ExitCode != 0 || !strings.Contains(outcome.Stdout, "[gate] msg-check passed") {
		t.Fatalf("hook gate did not pass: %+v", outcome)
	}
	if len(launcher.seen) != 1 {
		t.Fatalf("child not launched: %+v", launcher.seen)
	}
	argv := launcher.seen[0]
	if len(argv) != 2 || argv[1] != "feat: checked message\n" {
		t.Fatalf("message not projected into argv: %q", argv)
	}
	if len(launcher.seenLaunch) != 1 || launcher.seenLaunch[0].Environment["CHECK_MESSAGE"] != "feat: checked message\n" {
		t.Fatalf("message not projected into environment: %+v", launcher.seenLaunch)
	}
}

// TestGateRunRefusesCommitMessageWithoutFlag proves the incomplete refusal
// names the missing message-file option before any child starts.
func TestGateRunRefusesCommitMessageWithoutFlag(t *testing.T) {
	tree := hookTree("feat: checked message\n")
	launcher := &stubLauncher{}
	outcome := application.ExecuteWithLauncher(tree, args("gate", "run", "--surface", "commit-msg"), nil, launcher)
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "requires `--message-file` for a commit-message input") {
		t.Fatalf("missing message file not refused: %+v", outcome)
	}
	if len(launcher.seen) != 0 {
		t.Fatalf("child started before refusal: %v", launcher.seen)
	}
	json := application.ExecuteWithLauncher(tree,
		args("gate", "run", "--surface", "commit-msg", "--output", "json"), nil, launcher)
	if json.ExitCode != 2 || !strings.Contains(json.Stderr, "\"code\":\"rhino.args.incomplete\"") {
		t.Fatalf("refusal code not reported: %+v", json)
	}
}

// TestGateRunRefusesUnreadableMessageFile proves each hook-file fault maps to
// its own code and wording.
func TestGateRunRefusesUnreadableMessageFile(t *testing.T) {
	run := func(tree *memoryTree, messageFile string) application.Outcome {
		return application.ExecuteWithLauncher(tree,
			args("gate", "run", "--surface", "commit-msg", "--message-file", messageFile), nil, &stubLauncher{})
	}
	missing := hookTree("x")
	delete(missing.files, "COMMIT_EDITMSG")
	if outcome := run(missing, "COMMIT_EDITMSG"); outcome.ExitCode != 2 ||
		!strings.Contains(outcome.Stderr, "gate `msg-check`: message file `COMMIT_EDITMSG` does not exist") {
		t.Fatalf("missing message file not refused: %+v", outcome)
	}
	json := application.ExecuteWithLauncher(missing,
		args("gate", "run", "--surface", "commit-msg", "--message-file", "COMMIT_EDITMSG", "--output", "json"), nil, &stubLauncher{})
	if json.ExitCode != 2 || !strings.Contains(json.Stderr, "\"code\":\"rhino.file.missing\"") {
		t.Fatalf("missing message file code not reported: %+v", json)
	}
	unreadable := hookTree("x")
	unreadable.markUnreadable("COMMIT_EDITMSG")
	if outcome := run(unreadable, "COMMIT_EDITMSG"); outcome.ExitCode != 2 ||
		!strings.Contains(outcome.Stderr, "message file `COMMIT_EDITMSG` cannot be read: permission denied") {
		t.Fatalf("unreadable message file not refused: %+v", outcome)
	}
	binary := hookTree("x")
	binary.markBinary("COMMIT_EDITMSG")
	if outcome := run(binary, "COMMIT_EDITMSG"); outcome.ExitCode != 2 ||
		!strings.Contains(outcome.Stderr, "message file `COMMIT_EDITMSG` is not text") {
		t.Fatalf("binary message file not refused: %+v", outcome)
	}
	foreign := hookTree("x")
	if outcome := run(foreign, "other.txt"); outcome.ExitCode != 2 ||
		!strings.Contains(outcome.Stderr, "the supplied path is not Git's canonical COMMIT_EDITMSG") {
		t.Fatalf("foreign message file not refused: %+v", outcome)
	}
	noBoundary := gateTree(commitMsgGateConfig)
	noBoundary.files["COMMIT_EDITMSG"] = "x"
	if outcome := run(noBoundary, "COMMIT_EDITMSG"); outcome.ExitCode != 2 ||
		!strings.Contains(outcome.Stderr, "the Git hook message-file boundary is unavailable") {
		t.Fatalf("missing boundary not refused: %+v", outcome)
	}
}

// TestGateRunResolvesExplicitRangeFiles proves a pull-request gate receives
// only the changed paths for the immutable base..head pair.
func TestGateRunResolvesExplicitRangeFiles(t *testing.T) {
	tree := gateTree(explicitFilesGateConfig)
	tree.setChangedFiles("aaaaaaa", "bbbbbbb", "docs/b.md", "docs/a.md")
	launcher := &stubLauncher{}
	outcome := application.ExecuteWithLauncher(tree,
		args("gate", "run", "--surface", "pull-request", "--base", "aaaaaaa", "--head", "bbbbbbb"), nil, launcher)
	if outcome.ExitCode != 0 {
		t.Fatalf("range gate did not pass: %+v", outcome)
	}
	if len(launcher.seen) != 1 {
		t.Fatalf("child not launched: %v", launcher.seen)
	}
	argv := launcher.seen[0]
	if len(argv) != 3 || argv[1] != "docs/a.md" || argv[2] != "docs/b.md" {
		t.Fatalf("changed paths not projected: %q", argv)
	}
}

// TestGateRunRefusesNonCommitRange proves a non-hexadecimal base is refused
// as unrecognized before any Git answer is read.
func TestGateRunRefusesNonCommitRange(t *testing.T) {
	tree := gateTree(explicitFilesGateConfig)
	outcome := application.ExecuteWithLauncher(tree,
		args("gate", "run", "--surface", "pull-request", "--base", "not-a-commit", "--head", "bbbbbbb"), nil, &stubLauncher{})
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "requires hexadecimal commit IDs for `--base` and `--head`") {
		t.Fatalf("non-commit range not refused: %+v", outcome)
	}
	if !strings.Contains(outcome.Stderr, "rhino.args.unrecognized") && outcome.ExitCode != 2 {
		t.Fatalf("wrong code: %+v", outcome)
	}
}

// TestGateRunResolvesPushRange proves a pre-push gate projects the pushed
// base and head from the update records on standard input.
func TestGateRunResolvesPushRange(t *testing.T) {
	tree := gateTree(pushGateConfig)
	launcher := &stubLauncher{}
	stdin := "refs/heads/main aaaaaaa refs/heads/main bbbbbbb\n"
	outcome := application.ExecuteWithLauncher(tree,
		args("gate", "run", "--surface", "pre-push", "--push-updates-stdin"), &stdin, launcher)
	if outcome.ExitCode != 0 || !strings.Contains(outcome.Stdout, "[gate] range-check passed") {
		t.Fatalf("push gate did not pass: %+v", outcome)
	}
	if len(launcher.seen) != 1 {
		t.Fatalf("child not launched: %v", launcher.seen)
	}
	argv := launcher.seen[0]
	if len(argv) != 3 || argv[1] != "bbbbbbb" || argv[2] != "aaaaaaa" {
		t.Fatalf("push range not projected: %q", argv)
	}
	environment := launcher.seenLaunch[0].Environment
	if environment["NX_BASE"] != "bbbbbbb" || environment["NX_HEAD"] != "aaaaaaa" {
		t.Fatalf("push range not in environment: %v", environment)
	}
}

// TestGateRunSkipsDeletedPushRange proves a deleted ref skips the range gate
// cleanly, because there is no head to inspect.
func TestGateRunSkipsDeletedPushRange(t *testing.T) {
	tree := gateTree(pushGateConfig)
	launcher := &stubLauncher{}
	stdin := "refs/heads/gone 0000000 refs/heads/gone aaaaaaa\n"
	outcome := application.ExecuteWithLauncher(tree,
		args("gate", "run", "--surface", "pre-push", "--push-updates-stdin"), &stdin, launcher)
	if outcome.ExitCode != 0 {
		t.Fatalf("deleted range did not skip clean: %+v", outcome)
	}
	if len(launcher.seen) != 0 {
		t.Fatalf("deleted range started a child: %v", launcher.seen)
	}
}

// TestGateRunRefusesPushWithoutFlag proves the stdin flag is required for a
// push-update range.
func TestGateRunRefusesPushWithoutFlag(t *testing.T) {
	tree := gateTree(pushGateConfig)
	launcher := &stubLauncher{}
	outcome := application.ExecuteWithLauncher(tree, args("gate", "run", "--surface", "pre-push"), nil, launcher)
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "requires `--push-updates-stdin` for a push-update range") {
		t.Fatalf("push range without flag not refused: %+v", outcome)
	}
	if len(launcher.seen) != 0 {
		t.Fatalf("child started before refusal: %v", launcher.seen)
	}
}

// TestGateRunRefusesBadPushRecords proves each malformed update record maps
// to its input-unreadable wording.
func TestGateRunRefusesBadPushRecords(t *testing.T) {
	cases := []struct {
		name   string
		record string
		want   string
	}{
		{"empty", "", "received no pre-push update record"},
		{"lone newline", "\n", "received an invalid pre-push update record"},
		{"multiple", "a aaaaaaa b bbbbbbb\nc ccccccc d ddddddd\n", "received multiple pre-push update records"},
		{"short", "only two\n", "received an invalid pre-push update record"},
		{"bad local", "a nope b bbbbbbb\n", "received a push update without a hexadecimal local commit ID"},
		{"bad remote", "a aaaaaaa b nope\n", "received a push update without a hexadecimal remote commit ID"},
	}
	for _, tc := range cases {
		config := pushGateConfig
		tree := gateTree(config)
		stdin := tc.record
		outcome := application.ExecuteWithLauncher(tree,
			args("gate", "run", "--surface", "pre-push", "--push-updates-stdin"), &stdin, &stubLauncher{})
		if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, tc.want) {
			t.Fatalf("%s: %+v", tc.name, outcome)
		}
		jsonTree := gateTree(config)
		json := application.ExecuteWithLauncher(jsonTree,
			args("gate", "run", "--surface", "pre-push", "--push-updates-stdin", "--output", "json"), &stdin, &stubLauncher{})
		if json.ExitCode != 2 || !strings.Contains(json.Stderr, "\"code\":\"rhino.input.unreadable\"") {
			t.Fatalf("%s code: %+v", tc.name, json)
		}
	}
}

// TestGateRunProjectsNewRefViaFallback proves a new ref resolves its base
// through the declared fallback ref.
func TestGateRunProjectsNewRefViaFallback(t *testing.T) {
	tree := gateTree(pushGateConfig)
	tree.setGitRef("refs/remotes/origin/main", "ccccccc")
	launcher := &stubLauncher{}
	stdin := "refs/heads/new aaaaaaa refs/heads/new 0000000\n"
	outcome := application.ExecuteWithLauncher(tree,
		args("gate", "run", "--surface", "pre-push", "--push-updates-stdin"), &stdin, launcher)
	if outcome.ExitCode != 0 {
		t.Fatalf("new ref did not resolve: %+v", outcome)
	}
	argv := launcher.seen[0]
	if len(argv) != 3 || argv[1] != "ccccccc" || argv[2] != "aaaaaaa" {
		t.Fatalf("fallback base not projected: %q", argv)
	}
}

// TestGateRunResolvesGitIndexCheckoutAndState proves index and checkout
// sources resolve on the pre-commit surface.
func TestGateRunResolvesGitIndexCheckoutAndState(t *testing.T) {
	tree := gateTree(indexGateConfig)
	tree.setIndexedFiles("docs/staged.md")
	launcher := &stubLauncher{}
	outcome := application.ExecuteWithLauncher(tree, args("gate", "run", "--surface", "pre-commit"), nil, launcher)
	if outcome.ExitCode != 0 {
		t.Fatalf("index gate did not pass: %+v", outcome)
	}
	argv := launcher.seen[0]
	if len(argv) != 3 || argv[1] != "docs/staged.md" || argv[2] != "/root" {
		t.Fatalf("index and state not projected: %q", argv)
	}
}

// TestGateRunResolvesRegularFiles proves the opt-in regular-files input
// resolves through both of its sources.
func TestGateRunResolvesRegularFiles(t *testing.T) {
	config := "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: regular-check\n      type: check\n      inputs:\n        files: { kind: regular-files }\n      command:\n        executable: ./check\n        args:\n          - { input: files.paths, expand: repeat }\n      run-on:\n        pre-commit:\n          bind:\n            files: { source: git-index }\n        pull-request:\n          reason: runs only in the disposable pull-request replay\n          bind:\n            files: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: at-least\n"
	pre := gateTree(config)
	pre.setIndexedRegularFiles("src/main.go")
	launcher := &stubLauncher{}
	if outcome := application.ExecuteWithLauncher(pre, args("gate", "run", "--surface", "pre-commit"), nil, launcher); outcome.ExitCode != 0 {
		t.Fatalf("regular index gate did not pass: %+v", outcome)
	}
	if argv := launcher.seen[0]; len(argv) != 2 || argv[1] != "src/main.go" {
		t.Fatalf("regular index paths not projected: %q", argv)
	}
	pr := gateTree(config)
	pr.setChangedRegularFiles("aaaaaaa", "bbbbbbb", "src/a.go")
	launcher = &stubLauncher{}
	if outcome := application.ExecuteWithLauncher(pr,
		args("gate", "run", "--surface", "pull-request", "--base", "aaaaaaa", "--head", "bbbbbbb"), nil, launcher); outcome.ExitCode != 0 {
		t.Fatalf("regular range gate did not pass: %+v", outcome)
	}
	if argv := launcher.seen[0]; len(argv) != 2 || argv[1] != "src/a.go" {
		t.Fatalf("regular range paths not projected: %q", argv)
	}
}

// TestGateRunRefusesSingleProjectionOfSequence proves `expand: single`
// refuses a multi-value input instead of silently narrowing it.
func TestGateRunRefusesSingleProjectionOfSequence(t *testing.T) {
	config := strings.Replace(explicitFilesGateConfig, "expand: repeat", "expand: single", 1)
	tree := gateTree(config)
	tree.setChangedFiles("aaaaaaa", "bbbbbbb", "a.md", "b.md")
	outcome := application.ExecuteWithLauncher(tree,
		args("gate", "run", "--surface", "pull-request", "--base", "aaaaaaa", "--head", "bbbbbbb"), nil, &stubLauncher{})
	if outcome.ExitCode != 2 ||
		!strings.Contains(outcome.Stderr, "`files.paths` is not one value for `expand: single`") {
		t.Fatalf("sequence into single not refused: %+v", outcome)
	}
}

// TestGateRunRefusesSequenceIntoEnvironment proves a file sequence cannot
// collapse into one environment variable.
func TestGateRunRefusesSequenceIntoEnvironment(t *testing.T) {
	config := "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: files-check\n      type: check\n      inputs:\n        files: { kind: files }\n      command:\n        executable: ./check\n        environment:\n          FILES: { input: files.paths }\n      run-on:\n        pull-request:\n          reason: runs only in the disposable pull-request replay\n          bind:\n            files: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: at-least\n"
	tree := gateTree(config)
	tree.setChangedFiles("aaaaaaa", "bbbbbbb", "a.md", "b.md")
	outcome := application.ExecuteWithLauncher(tree,
		args("gate", "run", "--surface", "pull-request", "--base", "aaaaaaa", "--head", "bbbbbbb"), nil, &stubLauncher{})
	if outcome.ExitCode != 2 ||
		!strings.Contains(outcome.Stderr, "`files.paths` cannot project a sequence into environment variable `FILES`") {
		t.Fatalf("sequence into env not refused: %+v", outcome)
	}
}

// TestGateRunRefusesBadReferences proves each malformed reference is refused
// with its declaration wording, as the oracle words it, before any child starts.
func TestGateRunRefusesBadReferences(t *testing.T) {
	build := func(arg, env string) string {
		return "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: files-check\n      type: check\n      inputs:\n        files: { kind: files }\n      command:\n        executable: ./check\n        args:\n          - " + arg + "\n" + env + "      run-on:\n" +
			"        pull-request:\n          reason: runs only in the disposable pull-request replay\n          bind:\n            files: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: at-least\n"
	}
	cases := []struct {
		name   string
		config string
		want   string
	}{
		{"no dot", build("{ input: filespaths, expand: repeat }", ""), "filespaths: is not an `input.field` reference"},
		{"unknown input", build("{ input: unknown.paths, expand: repeat }", ""), "unknown.paths: names an input this gate does not declare"},
		{"wrong field", build("{ input: files.root, expand: single }", ""), "files.root: does not name a field available on a files input"},
		{"malformed argv", build("{ literal: labelled, expand: single }", ""), "args: adds `expand` to a literal argv item"},
	}
	for _, tc := range cases {
		tree := gateTree(tc.config)
		tree.setChangedFiles("aaaaaaa", "bbbbbbb", "a.md")
		outcome := application.ExecuteWithLauncher(tree,
			args("gate", "run", "--surface", "pull-request", "--base", "aaaaaaa", "--head", "bbbbbbb"), nil, &stubLauncher{})
		if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, tc.want) {
			t.Fatalf("%s: %+v", tc.name, outcome)
		}
	}
	envConfig := build("{ input: files.paths, expand: single }", "        environment:\n          ONE: { input: files.paths }\n")
	tree := gateTree(envConfig)
	tree.setChangedFiles("aaaaaaa", "bbbbbbb", "only.md")
	launcher := &stubLauncher{}
	if outcome := application.ExecuteWithLauncher(tree,
		args("gate", "run", "--surface", "pull-request", "--base", "aaaaaaa", "--head", "bbbbbbb"), nil, launcher); outcome.ExitCode != 0 {
		t.Fatalf("single file into env refused: %+v", outcome)
	} else if launcher.seenLaunch[0].Environment["ONE"] != "only.md" {
		t.Fatalf("env not projected: %v", launcher.seenLaunch[0].Environment)
	}
}

// TestGateRunRefusesInvalidBinding proves a kind/source pair the vocabulary
// never allows refuses as unusable configuration.
func TestGateRunRefusesInvalidBinding(t *testing.T) {
	config := strings.Replace(explicitFilesGateConfig, "explicit-range, range: explicit", "push-updates, fallback: refs/remotes/origin/main", 1)
	tree := gateTree(config)
	outcome := application.ExecuteWithLauncher(tree,
		args("gate", "run", "--surface", "pull-request", "--base", "aaaaaaa", "--head", "bbbbbbb"), nil, &stubLauncher{})
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "files: declares `files` but push-updates is not a compatible source") {
		t.Fatalf("invalid binding not refused: %+v", outcome)
	}
	json := application.ExecuteWithLauncher(gateTree(config),
		args("gate", "run", "--surface", "pull-request", "--base", "aaaaaaa", "--head", "bbbbbbb", "--output", "json"), nil, &stubLauncher{})
	if json.ExitCode != 2 || !strings.Contains(json.Stderr, "\"code\":\"rhino.config.unusable\"") {
		t.Fatalf("wrong code: %+v", json)
	}
}

// TestGateRunResolveRefusalDropsProgress proves a resolve refusal carries no
// progress on either stream, mirroring the oracle: a caller parsing the
// result never receives a document describing a run that did not happen.
func TestGateRunResolveRefusalDropsProgress(t *testing.T) {
	config := "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: ok\n      type: check\n      command:\n        executable: ./check\n      run-on:\n        pull-request:\n          reason: replay\n    - id: files-check\n      type: check\n      inputs:\n        files: { kind: files }\n      command:\n        executable: ./check\n        args:\n          - { input: files.paths, expand: repeat }\n      run-on:\n        pull-request:\n          reason: replay\n          bind:\n            files: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: at-least\n"
	tree := gateTree(config)
	outcome := application.ExecuteWithLauncher(tree,
		args("gate", "run", "--surface", "pull-request", "--base", "aaaaaaa", "--head", "bbbbbbb"), nil, &stubLauncher{code: 0})
	if outcome.ExitCode != 2 {
		t.Fatalf("expected refusal: %+v", outcome)
	}
	if outcome.Stdout != "" {
		t.Fatalf("progress leaked onto stdout: %q", outcome.Stdout)
	}
	if strings.Contains(outcome.Stderr, "[gate] ok ") {
		t.Fatalf("progress leaked onto stderr: %q", outcome.Stderr)
	}
	if !strings.Contains(outcome.Stderr, "files-check") {
		t.Fatalf("missing refusal: %+v", outcome)
	}
}

// TestGateRunLaunchRefusalKeepsProgressOnStderr proves a child that never
// started keeps the completed progress on stderr in text mode, with stdout
// empty.
func TestGateRunLaunchRefusalKeepsProgressOnStderr(t *testing.T) {
	config := "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: ok\n      type: check\n      command:\n        executable: ./check\n      run-on:\n        pre-commit: {}\n        pull-request:\n          reason: replay\n    - id: missing\n      type: check\n      command:\n        executable: ./missing\n      run-on:\n        pre-commit: {}\n        pull-request:\n          reason: replay\n  composition:\n    pull-request:\n      relation: exact\n"
	tree := gateTree(config)
	passing := &stubLauncher{}
	// First gate passes, second is not found: drive both with one launcher
	// that fails the second launch.
	calls := 0
	launcher := &recordingLauncher{launch: func(launch application.GateLaunch) application.Launched {
		calls++
		if calls == 1 {
			passing.Launch(launch)
			return application.Launched{Code: 0}
		}
		return application.Launched{NotFound: true}
	}}
	outcome := application.ExecuteWithLauncher(tree, args("gate", "run", "--surface", "pre-commit"), nil, launcher)
	if outcome.ExitCode != 127 {
		t.Fatalf("expected 127: %+v", outcome)
	}
	if outcome.Stdout != "" {
		t.Fatalf("stdout not empty: %q", outcome.Stdout)
	}
	if !strings.Contains(outcome.Stderr, "[gate] ok passed") ||
		!strings.Contains(outcome.Stderr, "executable was not found") {
		t.Fatalf("progress or reason missing: %+v", outcome)
	}
}

type recordingLauncher struct {
	launch func(application.GateLaunch) application.Launched
}

func (r *recordingLauncher) Launch(launch application.GateLaunch) application.Launched {
	return r.launch(launch)
}

// noGitOnlyTree carries the ordinary tree boundary without any Git input
// boundary, proving the dispatch refuses with the oracle's default
// no-boundary wordings instead of widening a boundary the caller never
// declared.
type noGitOnlyTree struct {
	application.Tree
}

// TestGateRunRefusesWithoutGitBoundary proves every Git-bound source maps a
// tree without that boundary to the oracle's default refusal.
func TestGateRunRefusesWithoutGitBoundary(t *testing.T) {
	bound := func(inputs, bind, arg string) string {
		return "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      type: check\n      inputs:\n        data: { kind: " + inputs + " }\n      command:\n        executable: ./check\n        args:\n          - " + arg + "\n      run-on:\n        pre-commit:\n          bind:\n            data: { source: " + bind + " }\n        pull-request:\n          bind:\n            data: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: exact\n"
	}
	cases := []struct {
		name   string
		config string
		flags  []string
		stdin  *string
		want   string
	}{
		{"index", bound("files", "git-index", "{ input: data.paths, expand: repeat }"),
			args("gate", "run", "--surface", "pre-commit"), nil,
			"cannot resolve the Git index: this tree has no Git index snapshot boundary"},
		{"regular index", bound("regular-files", "git-index", "{ input: data.paths, expand: repeat }"),
			args("gate", "run", "--surface", "pre-commit"), nil,
			"cannot resolve regular files from the Git index: this tree has no regular-file Git index snapshot boundary"},
		{"hook file", "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      type: check\n      inputs:\n        data: { kind: commit-message }\n      command:\n        executable: ./check\n        args:\n          - { input: data.text, expand: single }\n      run-on:\n        commit-msg:\n          bind:\n            data: { source: hook-message-file }\n        pull-request:\n          reason: replay\n          bind:\n            data: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: at-least\n",
			args("gate", "run", "--surface", "commit-msg", "--message-file", "msg.txt"), nil,
			"message file `msg.txt` cannot be read: the Git hook message-file boundary is unavailable"},
	}
	for _, tc := range cases {
		tree := gateTree(tc.config)
		bare := &noGitOnlyTree{tree}
		outcome := application.ExecuteWithLauncher(bare, tc.flags, tc.stdin, &stubLauncher{})
		if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, tc.want) {
			t.Fatalf("%s: %+v", tc.name, outcome)
		}
	}
	ranged := []struct {
		name   string
		config string
		flags  []string
		stdin  *string
		want   string
	}{
		{"changed files", bound("files", "git-index", "{ input: data.paths, expand: repeat }"),
			args("gate", "run", "--surface", "pre-commit"), nil,
			"cannot resolve the Git index: this tree has no Git index snapshot boundary"},
		{"explicit files", "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      type: check\n      inputs:\n        data: { kind: files }\n      command:\n        executable: ./check\n        args:\n          - { input: data.paths, expand: repeat }\n      run-on:\n        pull-request:\n          reason: replay\n          bind:\n            data: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: at-least\n",
			args("gate", "run", "--surface", "pull-request", "--base", "aaaaaaa", "--head", "bbbbbbb"), nil,
			"cannot read changed files for the explicit range: this tree has no changed-file range boundary"},
		{"explicit regular", "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      type: check\n      inputs:\n        data: { kind: regular-files }\n      command:\n        executable: ./check\n        args:\n          - { input: data.paths, expand: repeat }\n      run-on:\n        pull-request:\n          reason: replay\n          bind:\n            data: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: at-least\n",
			args("gate", "run", "--surface", "pull-request", "--base", "aaaaaaa", "--head", "bbbbbbb"), nil,
			"cannot read regular files for the explicit range: this tree has no regular-file changed-file range boundary"},
		{"explicit message", "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      type: check\n      inputs:\n        data: { kind: commit-message }\n      command:\n        executable: ./check\n        args:\n          - { input: data.text, expand: single }\n      run-on:\n        pull-request:\n          reason: replay\n          bind:\n            data: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: at-least\n",
			args("gate", "run", "--surface", "pull-request", "--base", "aaaaaaa", "--head", "bbbbbbb"), nil,
			"cannot read commit messages for the explicit range: this tree has no commit-message range boundary"},
	}
	for _, tc := range ranged {
		tree := gateTree(tc.config)
		bare := &noGitOnlyTree{tree}
		outcome := application.ExecuteWithLauncher(bare, tc.flags, tc.stdin, &stubLauncher{})
		if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, tc.want) {
			t.Fatalf("%s: %+v", tc.name, outcome)
		}
	}
	fallback := "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      type: check\n      inputs:\n        data: { kind: commit-range }\n      command:\n        executable: ./check\n        args:\n          - { input: data.base, expand: single }\n      run-on:\n        pre-push:\n          bind:\n            data: { source: push-updates, fallback: refs/heads/main }\n        pull-request:\n          bind:\n            data: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: exact\n"
	records := "refs/heads/new aaaaaaa refs/heads/new 0000000\n"
	bare := &noGitOnlyTree{gateTree(fallback)}
	outcome := application.ExecuteWithLauncher(bare,
		args("gate", "run", "--surface", "pre-push", "--push-updates-stdin"), &records, &stubLauncher{})
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr,
		"cannot resolve push fallback `refs/heads/main`: this tree has no Git ref-resolution boundary") {
		t.Fatalf("fallback without boundary: %+v", outcome)
	}
}

// TestGateRunCommitMessageExplicitRange proves the pull-request surface reads
// the same commit-message input from the immutable range.
func TestGateRunCommitMessageExplicitRange(t *testing.T) {
	tree := gateTree(commitMsgGateConfig)
	tree.setCommitMessages("aaaaaaa", "bbbbbbb", "feat: ranged message\n")
	launcher := &stubLauncher{}
	outcome := application.ExecuteWithLauncher(tree,
		args("gate", "run", "--surface", "pull-request", "--base", "aaaaaaa", "--head", "bbbbbbb"), nil, launcher)
	if outcome.ExitCode != 0 {
		t.Fatalf("ranged message gate did not pass: %+v", outcome)
	}
	if argv := launcher.seen[0]; len(argv) != 2 || argv[1] != "feat: ranged message\n" {
		t.Fatalf("ranged message not projected: %q", argv)
	}
	if launcher.seenLaunch[0].Environment["CHECK_MESSAGE"] != "feat: ranged message\n" {
		t.Fatalf("ranged message not in env: %v", launcher.seenLaunch[0].Environment)
	}
}

// TestGateRunCommitRangeExplicit proves a commit-range input projects its
// base and head for an explicit range.
func TestGateRunCommitRangeExplicit(t *testing.T) {
	tree := gateTree(pushGateConfig)
	launcher := &stubLauncher{}
	outcome := application.ExecuteWithLauncher(tree,
		args("gate", "run", "--surface", "pull-request", "--base", "aaaaaaa", "--head", "bbbbbbb"), nil, launcher)
	if outcome.ExitCode != 0 {
		t.Fatalf("explicit range gate did not pass: %+v", outcome)
	}
	argv := launcher.seen[0]
	if len(argv) != 3 || argv[1] != "aaaaaaa" || argv[2] != "bbbbbbb" {
		t.Fatalf("explicit range not projected: %q", argv)
	}
}

func TestGateRunInputCodesAreNamespaced(t *testing.T) {
	for _, code := range []domain.ErrorCode{domain.ArgsIncomplete, domain.ArgsUnrecognized, domain.InputUnreadable, domain.RepositoryUnusable, domain.ConfigUnusable, domain.FileMissing, domain.FileUnreadable} {
		if !strings.HasPrefix(code.String(), "rhino.") {
			t.Fatalf("code not namespaced: %v", code)
		}
	}
}

func writeGateFile(t *testing.T, dir, name, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitSh(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Skipf("git unavailable: %s", string(output))
	}
	return strings.TrimSpace(string(output))
}

func initGateRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitSh(t, dir, "init", "-q", "-b", "main")
	gitSh(t, dir, "config", "user.email", "gate@test")
	gitSh(t, dir, "config", "user.name", "gate")
	return dir
}

func commitGateFile(t *testing.T, dir, name, contents, message string) string {
	t.Helper()
	writeGateFile(t, dir, name, contents)
	gitSh(t, dir, "add", name)
	gitSh(t, dir, "commit", "-q", "-m", message)
	return gitSh(t, dir, "rev-parse", "HEAD")
}

// TestGateRunCommitMsgEndToEnd runs a real commit-msg gate through dispatch
// with the operating-system launcher and proves the script child observes the
// projected hook message in argv and the environment.
func TestGateRunCommitMsgEndToEnd(t *testing.T) {
	dir := initGateRepo(t)
	message := "feat: hooked message\n"
	canonical := gitSh(t, dir, "rev-parse", "--path-format=absolute", "--git-path", "COMMIT_EDITMSG")
	if err := os.WriteFile(canonical, []byte(message), 0o644); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(dir, "gate-child")
	script := "#!/bin/sh\n" +
		"actual=$(printf '%s' \"$1\")\n" +
		"test \"$actual\" = \"feat: hooked message\" || exit 3\n" +
		"observed=$(printf '%s' \"$CHECK_MESSAGE\")\n" +
		"test \"$observed\" = \"feat: hooked message\" || exit 4\n" +
		"test \"$RHINO_GATE_SURFACE\" = \"commit-msg\" || exit 5\n" +
		"exit 0\n"
	if err := os.WriteFile(child, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	config := strings.Replace(commitMsgGateConfig, "./check", "./gate-child", 1)
	writeGateFile(t, dir, "repo-config.yml", config)
	tree := &runtime.DiskTree{Dir: dir}
	outcome := application.ExecuteWithHarness(tree,
		args("gate", "run", "--surface", "commit-msg", "--message-file", canonical), nil, runtime.OSLauncher{}, nil, nil, nil, nil)
	if outcome.ExitCode != 0 || !strings.Contains(outcome.Stdout, "[gate] msg-check passed") {
		t.Fatalf("commit-msg gate did not pass end to end: %+v", outcome)
	}
}

// TestGateRunPrePushEndToEnd runs a real pre-push gate through dispatch and
// proves the script child observes the projected base and head plus the
// unmodified update records on standard input.
func TestGateRunPrePushEndToEnd(t *testing.T) {
	dir := initGateRepo(t)
	base := commitGateFile(t, dir, "a.txt", "a\n", "base commit")
	head := commitGateFile(t, dir, "b.txt", "b\n", "head commit")
	child := filepath.Join(dir, "gate-child")
	script := fmt.Sprintf("#!/bin/sh\n"+
		"test \"$1\" = \"%s\" || exit 3\n"+
		"test \"$2\" = \"%s\" || exit 4\n"+
		"test \"$NX_BASE\" = \"%s\" || exit 5\n"+
		"test \"$NX_HEAD\" = \"%s\" || exit 6\n"+
		"test \"$RHINO_GATE_SURFACE\" = \"pre-push\" || exit 7\n"+
		"cat > observed-stdin.txt\n"+
		"exit 0\n", base, head, base, head)
	if err := os.WriteFile(child, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	config := strings.Replace(pushGateConfig, "./check", "./gate-child", 1)
	writeGateFile(t, dir, "repo-config.yml", config)
	records := fmt.Sprintf("refs/heads/main %s refs/heads/main %s\n", head, base)
	tree := &runtime.DiskTree{Dir: dir}
	outcome := application.ExecuteWithHarness(tree,
		args("gate", "run", "--surface", "pre-push", "--push-updates-stdin"), &records, runtime.OSLauncher{}, nil, nil, nil, nil)
	if outcome.ExitCode != 0 || !strings.Contains(outcome.Stdout, "[gate] range-check passed") {
		t.Fatalf("pre-push gate did not pass end to end: %+v", outcome)
	}
	observed, err := os.ReadFile(filepath.Join(dir, "observed-stdin.txt"))
	if err != nil || string(observed) != records {
		t.Fatalf("child did not observe the update records: %q %v", string(observed), err)
	}
}
