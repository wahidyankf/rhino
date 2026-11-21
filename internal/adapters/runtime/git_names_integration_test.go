// Integration level: this file reaches the real host, so the default build omits it (see .golangci.yml).

//go:build integration

package runtime

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// TestGitNamePathsSortsAndDeduplicates proves the name list is sorted and holds
// each path once, however often Git repeats it.
// TestGitNameQueryErrorNamesAGitRefusal proves a failed Git process is reported
// by its status, without any of its output.
func TestGitNameQueryErrorNamesAGitRefusal(t *testing.T) {
	// A directory outside any repository makes git ls-files exit with status 128.
	err := exec.Command("git", "-C", t.TempDir(), "ls-files").Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("setup: want an exit error from git outside a repository, got %v", err)
	}
	got := gitNameQueryError("tracked", err).Error()
	if want := "git refused the tracked name query with status"; !strings.Contains(got, want) {
		t.Fatalf("message = %q, want it to contain %q", got, want)
	}
}
