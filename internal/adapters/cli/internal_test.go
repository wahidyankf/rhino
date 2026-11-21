package cli

import (
	"context"
	"testing"
)

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
