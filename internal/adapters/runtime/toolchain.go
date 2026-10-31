package runtime

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wahidyankf/rhino/internal/application"
)

// maxToolchainOutput caps one declared child's captured output, the way the
// oracle bounds it: a child that exceeds the bound yields no output at all.
const maxToolchainOutput = 64 * 1024

// DiskToolchainRunner starts declared toolchain probes and provision vectors
// as operating-system processes. Only toolchain validate and applied
// provision use it; tree validators never do.
//
// The child is started directly, never through a shell: a declared value
// holding a space or a metacharacter stays one argument. Standard input and
// diagnostics are discarded, and output past the bound, past the timeout, or
// outside text yields no output at all, only a reason.
type DiskToolchainRunner struct{}

// Run starts one declared vector and reports its status.
func (DiskToolchainRunner) Run(launch application.ToolchainLaunch) (application.ToolchainResult, error) {
	if strings.TrimSpace(launch.Executable) == "" || strings.HasPrefix(launch.Executable, "-") {
		return application.ToolchainResult{}, fmt.Errorf("the declared executable is invalid")
	}
	command := exec.Command(launch.Executable, launch.Arguments...)
	command.Stdin = nil
	command.Stderr = nil
	// Wait owns the copy from the child's output into the capture, so reaping
	// the child can never close a pipe under a reader. A grandchild that keeps
	// the output open gets one second after the child is reaped.
	command.WaitDelay = time.Second
	captured := boundedCapture{limit: maxToolchainOutput}
	command.Stdout = &captured
	if err := command.Start(); err != nil {
		return application.ToolchainResult{}, fmt.Errorf("could not start declared toolchain: %w", err)
	}
	waited := make(chan error, 1)
	go func() { waited <- command.Wait() }()
	var timeout <-chan time.Time
	if launch.TimeoutSeconds != nil {
		timer := time.NewTimer(time.Duration(*launch.TimeoutSeconds) * time.Second)
		defer timer.Stop()
		timeout = timer.C
	}
	select {
	case <-timeout:
		_ = command.Process.Kill()
		<-waited
		return application.ToolchainResult{}, fmt.Errorf("the declared toolchain exceeded its timeout")
	case err := <-waited:
		// The wait error is discarded on purpose: the exit below is read
		// from the reaped process state, which reports a signal death and a
		// nonzero status alike.
		_ = err
		return finishToolchain(command, &captured)
	}
}

func finishToolchain(command *exec.Cmd, captured *boundedCapture) (application.ToolchainResult, error) {
	if captured.total > maxToolchainOutput {
		return application.ToolchainResult{}, fmt.Errorf("the declared toolchain emitted too much output")
	}
	state := command.ProcessState
	if state == nil || state.ExitCode() == -1 {
		return application.ToolchainResult{}, fmt.Errorf("the declared toolchain ended by signal")
	}
	if !utf8.Valid(captured.kept) {
		return application.ToolchainResult{}, fmt.Errorf("the declared toolchain emitted non-text output")
	}
	return application.ToolchainResult{Code: state.ExitCode(), Stdout: string(captured.kept)}, nil
}
