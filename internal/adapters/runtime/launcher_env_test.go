package runtime_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/runtime"
	"github.com/wahidyankf/rhino/internal/application"
)

// TestLaunchSetsGateSurfaceAndProjectedEnvironment spawns a real gate child
// and proves it observes the oracle contract: the inherited environment plus
// RHINO_GATE_SURFACE and every projected entry.
func TestLaunchSetsGateSurfaceAndProjectedEnvironment(t *testing.T) {
	script := "#!/bin/sh\n" +
		"test \"$RHINO_GATE_SURFACE\" = \"pre-commit\" || exit 3\n" +
		"test \"$RHINO_CHECK_MESSAGE\" = \"hello\" || exit 4\n" +
		"test \"$RHINO_TEST_INHERITED\" = \"kept\" || exit 5\n" +
		"exit 0\n"
	path := filepath.Join(t.TempDir(), "gate-child")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RHINO_TEST_INHERITED", "kept")
	launched := runtime.OSLauncher{}.Launch(application.GateLaunch{
		Argv:        []string{path},
		Directory:   t.TempDir(),
		Surface:     "pre-commit",
		Environment: map[string]string{"RHINO_CHECK_MESSAGE": "hello"},
	})
	if launched != (application.Launched{Code: 0}) {
		t.Fatalf("gate child did not observe the gate environment: %+v", launched)
	}
}

// TestLaunchGateSurfaceOverridesInheritedValue proves a forged inherited
// RHINO_GATE_SURFACE never reaches the child, mirroring the oracle's
// `.env("RHINO_GATE_SURFACE", ...)` overwrite.
func TestLaunchGateSurfaceOverridesInheritedValue(t *testing.T) {
	script := "#!/bin/sh\n" +
		"test \"$RHINO_GATE_SURFACE\" = \"pre-push\" || exit 3\n" +
		"exit 0\n"
	path := filepath.Join(t.TempDir(), "gate-child")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RHINO_GATE_SURFACE", "forged")
	launched := runtime.OSLauncher{}.Launch(application.GateLaunch{
		Argv:      []string{path},
		Directory: t.TempDir(),
		Surface:   "pre-push",
	})
	if launched != (application.Launched{Code: 0}) {
		t.Fatalf("forged surface reached the gate child: %+v", launched)
	}
}
