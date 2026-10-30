package runtime_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wahidyankf/rhino/internal/application"

	"github.com/wahidyankf/rhino/internal/adapters/runtime"
)

func writeGateChild(t *testing.T, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gate-child")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLaunchMapsNonExecutableChild(t *testing.T) {
	child := writeGateChild(t, 0o644)
	launched := runtime.OSLauncher{}.Launch([]string{child}, t.TempDir())
	if !launched.NotExecutable {
		t.Fatalf("non-executable child not mapped to 126: %+v", launched)
	}
	if launched.NotFound || launched.Refused != "" {
		t.Fatalf("wrong vocabulary for non-executable child: %+v", launched)
	}
}

func TestLaunchMapsMissingChild(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	launched := runtime.OSLauncher{}.Launch([]string{missing}, t.TempDir())
	if !launched.NotFound {
		t.Fatalf("missing child not mapped to 127: %+v", launched)
	}
}

func TestLaunchKillsChildOnCancellation(t *testing.T) {
	child := func() string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "gate-child")
		if err := os.WriteFile(path, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan application.Launched, 1)
	go func() {
		done <- runtime.OSLauncher{Ctx: ctx}.Launch([]string{child}, t.TempDir())
	}()
	time.Sleep(200 * time.Millisecond)
	cancel()
	select {
	case launched := <-done:
		if launched.Refused == "" || launched.NotFound || launched.NotExecutable {
			t.Fatalf("cancellation not reported as refusal: %+v", launched)
		}
	case <-time.After(25 * time.Second):
		t.Fatal("cancelled child did not return")
	}
}
