package runtime

import (
	"fmt"
	"io"
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

// toolchainCaptured is what the output reader kept: at most one byte past
// the bound, so an over-long child is still detected after it is reaped.
type toolchainCaptured struct {
	output []byte
	failed error
}

// Run starts one declared vector and reports its status.
func (DiskToolchainRunner) Run(launch application.ToolchainLaunch) (application.ToolchainResult, error) {
	if strings.TrimSpace(launch.Executable) == "" || strings.HasPrefix(launch.Executable, "-") {
		return application.ToolchainResult{}, fmt.Errorf("the declared executable is invalid")
	}
	command := exec.Command(launch.Executable, launch.Arguments...)
	command.Stdin = nil
	command.Stderr = nil
	stream, err := command.StdoutPipe()
	if err != nil {
		return application.ToolchainResult{}, fmt.Errorf("could not start declared toolchain: %w", err)
	}
	if err := command.Start(); err != nil {
		return application.ToolchainResult{}, fmt.Errorf("could not start declared toolchain: %w", err)
	}
	read := make(chan toolchainCaptured, 1)
	go func() {
		output, err := io.ReadAll(io.LimitReader(stream, maxToolchainOutput+1))
		if err != nil {
			read <- toolchainCaptured{failed: fmt.Errorf("could not read toolchain output: %w", err)}
			return
		}
		read <- toolchainCaptured{output: output}
	}()
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
		<-read
		return application.ToolchainResult{}, fmt.Errorf("the declared toolchain exceeded its timeout")
	case err := <-waited:
		// The wait error is discarded on purpose: the exit below is read
		// from the reaped process state, which reports a signal death and a
		// nonzero status alike.
		_ = err
		captured := <-read
		return finishToolchain(command, captured)
	}
}

func finishToolchain(command *exec.Cmd, captured toolchainCaptured) (application.ToolchainResult, error) {
	if captured.failed != nil {
		return application.ToolchainResult{}, captured.failed
	}
	if int64(len(captured.output)) > maxToolchainOutput {
		return application.ToolchainResult{}, fmt.Errorf("the declared toolchain emitted too much output")
	}
	state := command.ProcessState
	if state == nil || state.ExitCode() == -1 {
		return application.ToolchainResult{}, fmt.Errorf("the declared toolchain ended by signal")
	}
	if !utf8.Valid(captured.output) {
		return application.ToolchainResult{}, fmt.Errorf("the declared toolchain emitted non-text output")
	}
	return application.ToolchainResult{Code: state.ExitCode(), Stdout: string(captured.output)}, nil
}
