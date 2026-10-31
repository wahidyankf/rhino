package application_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/runtime"
	"github.com/wahidyankf/rhino/internal/application"
)

// TestGateRunForwardsSurfaceAndEnvironmentEndToEnd runs a real script gate
// through gate dispatch with the operating-system launcher and proves the
// child observes RHINO_GATE_SURFACE and the projected variable. The script
// exits nonzero naming the missing variable, so any dropped forwarding fails
// the run instead of passing silently.
func TestGateRunForwardsSurfaceAndEnvironmentEndToEnd(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "gate-child")
	script := "#!/bin/sh\n" +
		"test \"$RHINO_GATE_SURFACE\" = \"pre-commit\" || exit 3\n" +
		"test -n \"$RHINO_CHECK_ROOT\" || exit 4\n" +
		"exit 0\n"
	if err := os.WriteFile(child, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	config := "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      inputs:\n        repo: { kind: repository-state }\n      command:\n        executable: ./gate-child\n        environment:\n          RHINO_CHECK_ROOT: { input: repo.root }\n      run-on:\n        pre-commit:\n          bind:\n            repo: { source: checkout }\n        pull-request:\n          reason: replay check\n  composition:\n    pull-request:\n      relation: exact\n"
	if err := os.WriteFile(filepath.Join(root, "repo-config.yml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	tree := &runtime.DiskTree{Dir: root}
	outcome := application.ExecuteWithHarness(
		tree, args("gate", "run", "--surface", "pre-commit"), nil,
		runtime.OSLauncher{}, nil, nil, nil, nil,
	)
	if outcome.ExitCode != 0 || !strings.Contains(outcome.Stdout, "[gate] check passed") {
		t.Fatalf("script gate did not pass with the gate environment: %+v", outcome)
	}
}

// TestGateRunCarriesSurfaceAndEnvironmentToLauncher proves dispatch hands the
// selected surface and the projected environment to the launcher, so a
// script gate reading RHINO_GATE_SURFACE is served even with a recording
// launcher standing in for the process boundary.
func TestGateRunCarriesSurfaceAndEnvironmentToLauncher(t *testing.T) {
	tree := gateTree("schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      inputs:\n        repo: { kind: repository-state }\n      command:\n        executable: ./check\n        environment:\n          RHINO_CHECK_ROOT: { input: repo.root }\n      run-on:\n        pre-commit:\n          bind:\n            repo: { source: checkout }\n        pull-request:\n          reason: replay check\n  composition:\n    pull-request:\n      relation: exact\n")
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
