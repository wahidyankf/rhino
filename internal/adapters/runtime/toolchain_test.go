package runtime_test

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

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

// hostTool resolves a standard utility on this host's PATH. A fixed /bin path is
// not portable: macOS has no /bin/true or /bin/false.
func hostTool(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Fatalf("%s is not on PATH: %v", name, err)
	}
	return path
}

func TestDiskToolchainRunnerRunsDeclaredVectorsDirectly(t *testing.T) {
	trueTool, falseTool := hostTool(t, "true"), hostTool(t, "false")
	result, err := runVector(t, trueTool)
	if err != nil || result.Code != 0 {
		t.Fatalf("true failed: %+v %v", result, err)
	}
	result, err = runVector(t, "/bin/echo", "1.2.3")
	if err != nil || result.Code != 0 || strings.TrimSpace(result.Stdout) != "1.2.3" {
		t.Fatalf("echo failed: %+v %v", result, err)
	}
	result, err = runVector(t, falseTool)
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

// A child that exits at once closes its end of the pipe while the runner is
// still collecting output. Reaping it must never close the read side under a
// reader mid-read, so every run, even under parallel load, keeps its exit code
// and its output.
func TestDiskToolchainRunnerKeepsOutputOfFastExitingChildren(t *testing.T) {
	const workers, runsPerWorker = 16, 40
	trueTool := hostTool(t, "true")
	var guard sync.Mutex
	failed, first := 0, ""
	record := func(label string, result application.ToolchainResult, err error) {
		guard.Lock()
		defer guard.Unlock()
		if failed == 0 {
			first = fmt.Sprintf("%s: %+v %v", label, result, err)
		}
		failed++
	}
	var group sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for run := 0; run < runsPerWorker; run++ {
				result, err := runVector(t, "/bin/echo", "fast")
				if err != nil || result.Code != 0 || result.Stdout != "fast\n" {
					record("echo", result, err)
				}
				result, err = runVector(t, trueTool)
				if err != nil || result.Code != 0 || result.Stdout != "" {
					record("true", result, err)
				}
			}
		}()
	}
	group.Wait()
	if failed > 0 {
		t.Fatalf("%d of %d fast runs failed, first: %s", failed, workers*runsPerWorker*2, first)
	}
}

func TestDiskToolchainRunnerBoundsOutputWithoutBlockingTheChild(t *testing.T) {
	// Exactly the bound is kept whole.
	result, err := runVector(t, "/bin/sh", "-c", "head -c 65536 /dev/zero | tr '\\0' a")
	if err != nil || result.Code != 0 || len(result.Stdout) != 64*1024 {
		t.Fatalf("output at the bound was refused: %d bytes, %v", len(result.Stdout), err)
	}
	// One byte past the bound, and far more than a pipe holds, yield no output.
	for _, size := range []string{"65537", "4194304"} {
		result, err = runVector(t, "/bin/sh", "-c", "head -c "+size+" /dev/zero | tr '\\0' a")
		if err == nil || !strings.Contains(err.Error(), "emitted too much output") || result.Stdout != "" {
			t.Fatalf("%s bytes were not refused as too much output: %+v %v", size, result, err)
		}
	}
}

func TestDiskToolchainRunnerRefusesNonTextAndSignalDeath(t *testing.T) {
	if _, err := runVector(t, "/bin/sh", "-c", "printf '\\377\\376'"); err == nil ||
		!strings.Contains(err.Error(), "emitted non-text output") {
		t.Fatalf("non-UTF-8 output was not refused: %v", err)
	}
	if _, err := runVector(t, "/bin/sh", "-c", "kill -9 $$"); err == nil ||
		!strings.Contains(err.Error(), "ended by signal") {
		t.Fatalf("signal death was not reported: %v", err)
	}
}

func TestDiskToolchainRunnerKillsAChildPastItsTimeout(t *testing.T) {
	timeout := uint64(1)
	started := time.Now()
	_, err := runtime.DiskToolchainRunner{}.Run(application.ToolchainLaunch{
		Executable:     "/bin/sleep",
		Arguments:      []string{"30"},
		TimeoutSeconds: &timeout,
	})
	if err == nil || !strings.Contains(err.Error(), "exceeded its timeout") {
		t.Fatalf("timeout was not reported: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("the child was not killed at its timeout: %s", elapsed)
	}
}

func TestDiskToolchainRunnerKeepsArgumentsLiteralAndDiscardsDiagnostics(t *testing.T) {
	result, err := runVector(t, "/bin/echo", "a b;c $HOME")
	if err != nil || result.Stdout != "a b;c $HOME\n" {
		t.Fatalf("an argument was interpreted: %+v %v", result, err)
	}
	result, err = runVector(t, "/bin/sh", "-c", "echo noise >&2; echo kept; cat")
	if err != nil || result.Code != 0 || result.Stdout != "kept\n" {
		t.Fatalf("diagnostics or standard input leaked: %+v %v", result, err)
	}
}
