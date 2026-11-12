package application

import (
	"errors"
	"strings"
	"testing"
)

// childStartToolchain declares one applied provision vector for linux, the
// platform the test host reports, with an executable the runner refuses to start.
const childStartToolchain = `schema: rhino/repo-config/v2
toolchains:
  entries:
    - id: installer
      executable: installer-tool
      probe: [--version]
      required: false
      provision: [{platform: linux, executable: ./installer, arguments: [install]}]
`

// childStartRunner refuses every launch as a child that could not be started,
// with the status its runner classified.
type childStartRunner struct {
	status int
}

func (r childStartRunner) Run(ToolchainLaunch) (ToolchainResult, error) {
	return ToolchainResult{}, &ChildStartError{Status: r.status, Err: errors.New("could not start declared toolchain: fixture")}
}

// unclassifiedStartRunner refuses every launch with a start failure the runner
// could not classify as 127 or 126.
type unclassifiedStartRunner struct{}

func (unclassifiedStartRunner) Run(ToolchainLaunch) (ToolchainResult, error) {
	return ToolchainResult{}, errors.New("could not start declared toolchain: fixture")
}

// TestProvisionExitsWithTheChildStatusWhenItsExecutableCannotStart proves an applied
// provision whose child is not found ends with 127, and one that cannot be executed
// ends with 126, both under the toolchain code.
func TestProvisionExitsWithTheChildStatusWhenItsExecutableCannotStart(t *testing.T) {
	document := parseProvisionFixture(t, childStartToolchain)
	for _, status := range []int{127, 126} {
		outcome := provisionToolchains(document, childStartRunner{status: status}, "linux", true, JSON)
		if outcome.ExitCode != status || !strings.Contains(outcome.Stderr, `"code":"rhino.toolchain.failed"`) {
			t.Fatalf("a child with status %d ended %+v, want exit %d under rhino.toolchain.failed", status, outcome, status)
		}
	}
}

// TestProvisionKeepsExitTwoForAStartFailureItCannotClassify proves a start failure
// with no 127 or 126 classification still refuses with exit 2.
func TestProvisionKeepsExitTwoForAStartFailureItCannotClassify(t *testing.T) {
	document := parseProvisionFixture(t, childStartToolchain)
	outcome := provisionToolchains(document, unclassifiedStartRunner{}, "linux", true, JSON)
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, `"code":"rhino.toolchain.failed"`) {
		t.Fatalf("an unclassified start failure ended %+v, want exit 2", outcome)
	}
}
