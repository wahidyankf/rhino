package cli

import (
	"context"
	"errors"
	"io"
	"syscall"
	"testing"
)

func TestIsClosedPipe(t *testing.T) {
	if !isClosedPipe(syscall.EPIPE) {
		t.Fatal("EPIPE not detected")
	}
	if isClosedPipe(io.EOF) {
		t.Fatal("EOF detected as closed pipe")
	}
	if isClosedPipe(errors.New("boom")) {
		t.Fatal("generic error detected as closed pipe")
	}
}

func TestSignalContextCancels(t *testing.T) {
	ctx, stop := SignalContext(context.Background())
	if ctx == nil {
		t.Fatal("no context")
	}
	stop()
	select {
	case <-ctx.Done():
	default:
		t.Fatal("stop did not cancel")
	}
}
