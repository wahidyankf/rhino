package cli

import (
	"context"
	"io"
	"syscall"
	"testing"
	"time"
)

// TestRunArgsReportsASignalAs128PlusN proves a run cancelled by a signal
// reports 128 plus the signal number, the status the exit-status convention
// names for a death by signal: 130 for an interrupt, 143 for a termination.
func TestRunArgsReportsASignalAs128PlusN(t *testing.T) {
	tests := []struct {
		name   string
		signal syscall.Signal
		want   int
	}{
		{name: "interrupt", signal: syscall.SIGINT, want: 130},
		{name: "termination", signal: syscall.SIGTERM, want: 143},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancelCause(context.Background())
			cancel(signalCause(test.signal))
			app := Application{Stdout: io.Discard, Stderr: io.Discard}
			if code := app.RunArgs(ctx, []string{"version"}); code != test.want {
				t.Fatalf("run cancelled by %v exited %d, want %d", test.signal, code, test.want)
			}
		})
	}
}

// TestSignalContextCarriesTheSignal proves a delivered termination signal
// cancels the context with that signal as its cause, so the CLI boundary can
// report the status for the signal that actually ended the run.
func TestSignalContextCarriesTheSignal(t *testing.T) {
	ctx, stop := SignalContext(context.Background())
	defer stop()
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("termination signal did not cancel the context")
	}
	if cause := context.Cause(ctx); cause != error(signalCause(syscall.SIGTERM)) {
		t.Fatalf("cause = %v, want SIGTERM", cause)
	}
}
