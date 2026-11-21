package main

import (
	"context"
	"testing"
)

func TestRunGivesExecuteTheSignalContextAndArguments(t *testing.T) {
	var (
		seenParent context.Context
		seenCtx    context.Context
		seenArgs   []string
		stopped    bool
		signalled  context.Context
		signalStop = func() { stopped = true }
		wantCode   = 7
	)
	signals := func(parent context.Context) (context.Context, context.CancelFunc) {
		seenParent = parent
		ctx, cancel := context.WithCancel(parent)
		signalled = ctx
		return ctx, func() {
			signalStop()
			cancel()
		}
	}
	execute := func(ctx context.Context, args []string) int {
		seenCtx = ctx
		seenArgs = args
		if stopped {
			t.Fatal("the signal context was stopped before execute returned")
		}
		return wantCode
	}
	if code := run([]string{"gate", "run"}, signals, execute); code != wantCode {
		t.Fatalf("run = %d, want %d", code, wantCode)
	}
	if seenParent == nil || seenCtx != signalled {
		t.Fatal("execute did not receive the signal context")
	}
	if len(seenArgs) != 2 || seenArgs[0] != "gate" || seenArgs[1] != "run" {
		t.Fatalf("execute received args %v", seenArgs)
	}
	if !stopped {
		t.Fatal("the signal context was not stopped after execute")
	}
}
