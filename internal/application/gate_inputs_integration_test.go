// Integration level: this file reaches the real host, so the default build omits it (see .golangci.yml).

//go:build integration

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
)

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
		t.Fatalf("git unavailable: %s", string(output))
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
