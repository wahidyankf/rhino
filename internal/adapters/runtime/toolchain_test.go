package runtime_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/runtime"
	"github.com/wahidyankf/rhino/internal/application"
)

func runVector(t *testing.T, executable string, arguments ...string) (application.ToolchainResult, error) {
	t.Helper()
	return runtime.DiskToolchainRunner{}.Run(application.ToolchainLaunch{
		Executable: executable,
		Arguments:  arguments,
	})
}

func TestDiskToolchainRunnerRunsDeclaredVectorsDirectly(t *testing.T) {
	result, err := runVector(t, "/bin/true")
	if err != nil || result.Code != 0 {
		t.Fatalf("true failed: %+v %v", result, err)
	}
	result, err = runVector(t, "/bin/echo", "1.2.3")
	if err != nil || result.Code != 0 || strings.TrimSpace(result.Stdout) != "1.2.3" {
		t.Fatalf("echo failed: %+v %v", result, err)
	}
	result, err = runVector(t, "/bin/false")
	if err != nil || result.Code == 0 {
		t.Fatalf("false looked clean: %+v %v", result, err)
	}
	if _, err := runVector(t, "rhino-no-such-tool-xyz"); err == nil ||
		!strings.Contains(err.Error(), "could not start declared toolchain") {
		t.Fatalf("missing tool was not reported as unstartable: %v", err)
	}
	for _, executable := range []string{"", "   ", "-invalid"} {
		if _, err := runVector(t, executable); err == nil ||
			!strings.Contains(err.Error(), "the declared executable is invalid") {
			t.Fatalf("%q was not refused as invalid: %v", executable, err)
		}
	}
}
