package bootstrap_test

import (
	"context"
	"testing"

	"github.com/wahidyankf/rhino/internal/bootstrap"
)

func TestExecuteVersionExitsClean(t *testing.T) {
	if code := bootstrap.Execute(context.Background(), []string{"version"}); code != 0 {
		t.Fatalf("version exited %d", code)
	}
}

func TestExecuteUnknownCommandIsRefused(t *testing.T) {
	if code := bootstrap.Execute(context.Background(), []string{"nope", "validate"}); code != 2 {
		t.Fatalf("unknown command exited %d", code)
	}
}

func TestExecuteCancelledContextReturns130(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if code := bootstrap.Execute(cancelled, []string{"version"}); code != 130 {
		t.Fatalf("cancelled bootstrap run exited %d, want 130", code)
	}
}
