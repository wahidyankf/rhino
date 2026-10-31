// Package cli adapts operating-system streams and signals to the application.
package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/wahidyankf/rhino/internal/adapters/runtime"
	"github.com/wahidyankf/rhino/internal/application"
	"github.com/wahidyankf/rhino/internal/domain"
)

// SignalContext returns a context cancelled by interrupt or termination. Its
// cancellation cause names the signal, so the CLI boundary reports the status
// for the signal that ended the run. The caller owns the stop function and must
// defer it.
func SignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancelCause(parent)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case received := <-signals:
			if sig, ok := received.(syscall.Signal); ok {
				cancel(signalCause(sig))
			} else {
				cancel(context.Canceled)
			}
		case <-ctx.Done():
		}
	}()
	return ctx, func() {
		signal.Stop(signals)
		cancel(nil)
	}
}

// signalCause is the cancellation cause of a run ended by a signal.
type signalCause syscall.Signal

func (c signalCause) Error() string { return syscall.Signal(c).String() + " signal received" }

// Application wires one invocation to its streams.
type Application struct {
	Version string
	Commit  string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	// Launcher starts declared gate children; nil refuses dispatch honestly.
	Launcher application.Launcher
	// Environment holds the write boundary only init, backup, and restore
	// reach; nil fails those operations closed, never the reads.
	Environment application.EnvironmentStore
	// Toolchains holds the process boundary only toolchain validate and
	// applied provision reach; nil fails closed through the same findings
	// and refusals an unwired entry point reports.
	Toolchains application.ToolchainRunner
	// Adapters holds the write boundary only adapter generation reaches;
	// nil fails closed, never the reads validation performs.
	Adapters application.AdapterStore
	// Models holds the process boundary only adapter generation reaches;
	// nil resolves through fallbacks with warnings, the way an unwired
	// entry point reports.
	Models application.ModelResolver
	// Mutations holds the process and write boundary only gate dispatch of a
	// `type: mutation` gate reaches; nil fails every mutation closed.
	Mutations application.MutationRunner
}

// Run executes args and returns the process exit code.
func (app Application) Run(ctx context.Context) int {
	return app.RunArgs(ctx, os.Args[1:])
}

// interruptedExit is 128+SIGINT. It is the status for a run cancelled by a
// context that names no signal.
const interruptedExit = 130

// cancelledExit is the status for a run a signal ended: 128 plus the signal
// number, so an interrupt is 130 and a termination is 143.
func cancelledExit(ctx context.Context) int {
	if cause, ok := context.Cause(ctx).(signalCause); ok {
		return 128 + int(cause)
	}
	return interruptedExit
}

// RunArgs executes explicit args and returns the process exit code.
func (app Application) RunArgs(ctx context.Context, args []string) int {
	if ctx == nil {
		ctx = context.Background()
	}
	stdin := app.Stdin
	stdout := app.Stdout
	stderr := app.Stderr
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	if stdin == nil {
		stdin = os.Stdin
	}
	if app.Version != "" {
		application.Version = app.Version
	}
	if app.Commit != "" {
		application.Commit = app.Commit
	}

	parsed, refusal := application.Parse(args)
	var stdinText *string
	if refusal == nil && parsed.Run != nil {
		invocation := parsed.Run
		if invocation.PushUpdatesStdin || hasStdinFile(invocation.Files) {
			text, err := io.ReadAll(stdin)
			if err != nil {
				outcome := application.RefuseWith(invocation.Format,
					domain.InputUnreadable, "standard input could not be read: "+err.Error())
				return emit(stdout, stderr, outcome)
			}
			value := string(text)
			stdinText = &value
		}
	}

	tree, err := runtime.AtCurrentDirectory()
	if err != nil {
		format := application.Text
		if refusal == nil && parsed.Run != nil {
			format = parsed.Run.Format
		}
		return emit(stdout, stderr, application.RefuseWith(format,
			domain.RepositoryUnusable, err.Error()))
	}
	outcome := application.ExecuteWithMutations(tree, args, stdinText, app.Launcher, app.Environment, app.Toolchains, app.Adapters, app.Models, app.Mutations)
	if ctx.Err() != nil {
		return cancelledExit(ctx)
	}
	return emit(stdout, stderr, outcome)
}

func hasStdinFile(files []string) bool {
	for _, file := range files {
		if file == "-" {
			return true
		}
	}
	return false
}

// emit writes both streams and maps a closed reader to 141, the status a
// shell reports for SIGPIPE, rather than to a finding or a crash.
func emit(stdout, stderr io.Writer, outcome application.Outcome) int {
	if _, err := io.WriteString(stdout, outcome.Stdout); err != nil && isClosedPipe(err) {
		return 141
	} else if err != nil {
		return 2
	}
	if _, err := io.WriteString(stderr, outcome.Stderr); err != nil && isClosedPipe(err) {
		return 141
	} else if err != nil {
		return 2
	}
	return outcome.ExitCode
}

func isClosedPipe(err error) bool {
	return errors.Is(err, syscall.EPIPE)
}
