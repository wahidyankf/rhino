package application_test

import (
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// TestGateRunCarriesSurfaceAndEnvironmentToLauncher proves dispatch hands the
// selected surface and the projected environment to the launcher, so a
// script gate reading RHINO_GATE_SURFACE is served even with a recording
// launcher standing in for the process boundary.
func TestGateRunCarriesSurfaceAndEnvironmentToLauncher(t *testing.T) {
	tree := gateTree("schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      inputs:\n        repo: { kind: repository-state }\n      command:\n        executable: ./check\n        environment:\n          RHINO_CHECK_ROOT: { input: repo.root }\n      run-on:\n        pre-commit:\n          bind:\n            repo: { source: checkout }\n        pull-request:\n          reason: replay check\n          bind:\n            repo: { source: checkout }\n  composition:\n    pull-request:\n      relation: exact\n")
	launcher := &stubLauncher{}
	outcome := application.ExecuteWithLauncher(tree, args("gate", "run", "--surface", "pre-commit"), nil, launcher)
	if outcome.ExitCode != 0 {
		t.Fatalf("gate run failed: %+v", outcome)
	}
	if len(launcher.seenLaunch) != 1 {
		t.Fatalf("child not launched: %+v", launcher.seenLaunch)
	}
	launch := launcher.seenLaunch[0]
	if launch.Surface != "pre-commit" {
		t.Fatalf("surface not carried to launcher: %+v", launch)
	}
	if len(launch.Environment) != 1 || launch.Environment["RHINO_CHECK_ROOT"] == "" {
		t.Fatalf("environment not carried to launcher: %+v", launch)
	}
}
