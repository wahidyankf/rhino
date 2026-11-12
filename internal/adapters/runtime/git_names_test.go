package runtime

import (
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// TestGitNamePathsSortsAndDeduplicates proves the name list is sorted and holds
// each path once, however often Git repeats it.
func TestGitNamePathsSortsAndDeduplicates(t *testing.T) {
	got, err := gitNamePaths("tracked", []byte("b.txt\x00a/x.env\x00b.txt\x00"))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a/x.env", "b.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
}

// TestGitNamePathsEmptyOutputIsNoNames proves an empty repository lists no names
// and is not a refusal.
func TestGitNamePathsEmptyOutputIsNoNames(t *testing.T) {
	got, err := gitNamePaths("ignored", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("names = %v, want none", got)
	}
}

// TestGitNamePathsRefusesANonUTF8Name proves a name Git returns as invalid UTF-8
// refuses the whole query, rather than being passed on as a path.
func TestGitNamePathsRefusesANonUTF8Name(t *testing.T) {
	_, err := gitNamePaths("ignored", []byte("ok.txt\x00\xff\xfe\x00"))
	if err == nil || !strings.Contains(err.Error(), "non-UTF-8 path") {
		t.Fatalf("err = %v, want a refusal for a non-UTF-8 name", err)
	}
}

// TestGitNamePathsRefusesAnEscapingName proves a name that leaves the repository
// refuses the whole query, for both relative and absolute escapes.
func TestGitNamePathsRefusesAnEscapingName(t *testing.T) {
	for _, raw := range []string{"../outside.env", "/etc/passwd"} {
		_, err := gitNamePaths("tracked", []byte("ok.txt\x00"+raw+"\x00"))
		if err == nil || !strings.Contains(err.Error(), "escaping path") {
			t.Fatalf("%q: err = %v, want a refusal for an escaping name", raw, err)
		}
	}
}

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
	if want := "Git refused the tracked name query with status"; !strings.Contains(got, want) {
		t.Fatalf("message = %q, want it to contain %q", got, want)
	}
}

// TestGitNameQueryErrorNamesAFailureToStart proves a failure that is not a Git
// exit status is reported with its cause.
func TestGitNameQueryErrorNamesAFailureToStart(t *testing.T) {
	got := gitNameQueryError("ignored", errors.New("boom")).Error()
	if want := "could not read the ignored names from Git: boom"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}
