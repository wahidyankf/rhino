// Integration level: this file reaches the real host, so the default build omits it (see .golangci.yml).

//go:build integration

package cli

import (
	"context"
	"syscall"
	"testing"
)

// fakeSignals is a signalSource whose deliveries the test makes by hand.
func TestCancelledExitNamesTheSignalOrDefaultsToInterrupt(t *testing.T) {
	signalled, cancelSignalled := context.WithCancelCause(context.Background())
	cancelSignalled(signalCause(syscall.SIGTERM))
	if got := cancelledExit(signalled); got != 143 {
		t.Fatalf("cancelledExit(SIGTERM) = %d, want 143", got)
	}
	plain, cancelPlain := context.WithCancel(context.Background())
	cancelPlain()
	if got := cancelledExit(plain); got != interruptedExit {
		t.Fatalf("cancelledExit(plain) = %d, want %d", got, interruptedExit)
	}
}
