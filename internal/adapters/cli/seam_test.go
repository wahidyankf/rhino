package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// fakeSignals is a signalSource whose deliveries the test makes by hand.
type fakeSignals struct {
	channel chan<- os.Signal
	stopped bool
}

func (f *fakeSignals) Notify(channel chan<- os.Signal, _ ...os.Signal) { f.channel = channel }

func (f *fakeSignals) Stop(channel chan<- os.Signal) { f.stopped = channel == f.channel }

// otherSignal is an os.Signal that is not a syscall.Signal.
type otherSignal string

func (o otherSignal) Signal()        {}
func (o otherSignal) String() string { return string(o) }

func TestSignalContextCancelsWithTheDeliveredSignal(t *testing.T) {
	tests := []struct {
		name      string
		signal    os.Signal
		wantCause error
	}{
		{name: "interrupt", signal: syscall.SIGINT, wantCause: signalCause(syscall.SIGINT)},
		{name: "termination", signal: syscall.SIGTERM, wantCause: signalCause(syscall.SIGTERM)},
		{name: "other signal", signal: otherSignal("custom"), wantCause: context.Canceled},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := &fakeSignals{}
			ctx, stop := signalContext(context.Background(), source)
			source.channel <- test.signal
			<-ctx.Done()
			if cause := context.Cause(ctx); cause != test.wantCause {
				t.Fatalf("cause = %v, want %v", cause, test.wantCause)
			}
			stop()
			if !source.stopped || ctx.Err() == nil {
				t.Fatal("stop did not release the signal source and cancel the context")
			}
		})
	}
}

func TestSignalContextEndsWhenTheParentEnds(t *testing.T) {
	source := &fakeSignals{}
	parent, cancelParent := context.WithCancel(context.Background())
	ctx, stop := signalContext(parent, source)
	cancelParent()
	<-ctx.Done()
	if cause := context.Cause(ctx); cause != context.Canceled {
		t.Fatalf("cause = %v, want the parent's cancellation", cause)
	}
	stop()
	if !source.stopped {
		t.Fatal("stop did not release the signal source")
	}
}

func TestOrDefaultKeepsTheCallersValueOrTheFallback(t *testing.T) {
	var nilWriter io.Writer
	if got := orDefault(nilWriter, io.Writer(io.Discard)); got != io.Discard {
		t.Fatalf("orDefault(nil) = %v, want the fallback", got)
	}
	if got := orDefault[io.Writer](io.Discard, os.Stdout); got != io.Discard {
		t.Fatalf("orDefault(value) = %v, want the value", got)
	}
}

// refusingTree is a repository that cannot be established.
func refusingTree() (application.Tree, error) {
	return nil, errors.New("no repository here")
}

func TestRunArgsRefusesWhenTheRepositoryCannotBeEstablished(t *testing.T) {
	var stdout, stderr strings.Builder
	app := Application{Stdout: &stdout, Stderr: &stderr, currentDirectory: refusingTree}
	if code := app.RunArgs(context.Background(), []string{"version"}); code == 0 {
		t.Fatal("a run with no repository exited 0")
	}
	if !strings.Contains(stdout.String()+stderr.String(), "no repository here") {
		t.Fatalf("the refusal omitted the cause: stdout %q stderr %q", stdout.String(), stderr.String())
	}
}

// unreadable is a standard input whose every read fails.
type unreadable struct{}

func (unreadable) Read([]byte) (int, error) { return 0, errInjected }

var errInjected = errors.New("injected read failure")

func TestRunArgsRefusesAnUnreadableStandardInput(t *testing.T) {
	var stdout, stderr strings.Builder
	app := Application{Stdin: unreadable{}, Stdout: &stdout, Stderr: &stderr, currentDirectory: refusingTree}
	code := app.RunArgs(context.Background(), []string{"gate", "run", "--surface", "pre-push", "--push-updates-stdin"})
	if code == 0 || !strings.Contains(stdout.String()+stderr.String(), "standard input could not be read") {
		t.Fatalf("exit %d, stdout %q, stderr %q; want an unreadable-input refusal", code, stdout.String(), stderr.String())
	}
	fileCode := app.RunArgs(context.Background(), []string{"md", "mermaid", "validate", "--file", "-"})
	if fileCode == 0 || !strings.Contains(stdout.String()+stderr.String(), "standard input could not be read") {
		t.Fatalf("--file - exit %d; want an unreadable-input refusal", fileCode)
	}
}

func TestRunArgsReadsStandardInputBeforeTheRepository(t *testing.T) {
	var stdout, stderr strings.Builder
	app := Application{
		Stdin:            strings.NewReader("ref\n"),
		Stdout:           &stdout,
		Stderr:           &stderr,
		currentDirectory: refusingTree,
	}
	code := app.RunArgs(context.Background(), []string{"gate", "run", "--surface", "pre-push", "--push-updates-stdin"})
	if code == 0 || strings.Contains(stdout.String()+stderr.String(), "could not be read") {
		t.Fatalf("exit %d with output %q %q; want the repository refusal after a clean read", code, stdout.String(), stderr.String())
	}
}

func TestRunArgsUsesTheProcessStreamsWhenNoneAreGiven(t *testing.T) {
	code := Application{currentDirectory: refusingTree}.RunArgs(context.Background(), []string{"version"})
	if code == 0 {
		t.Fatal("a run with no repository and no stream exited 0")
	}
}

func TestRunArgsRecordsTheVersionAndCommit(t *testing.T) {
	savedVersion, savedCommit := application.Version, application.Commit
	t.Cleanup(func() {
		application.Version, application.Commit = savedVersion, savedCommit
	})
	app := Application{Version: "v9.9.9", Commit: "abc1234", Stdout: io.Discard, Stderr: io.Discard, currentDirectory: refusingTree}
	app.RunArgs(context.Background(), []string{"version"})
	if application.Version != "v9.9.9" || application.Commit != "abc1234" {
		t.Fatalf("version %q commit %q were not recorded", application.Version, application.Commit)
	}
}

func TestRunRunsWithTheProcessArguments(t *testing.T) {
	app := Application{Stdout: io.Discard, Stderr: io.Discard, currentDirectory: refusingTree}
	if got, want := app.Run(context.Background()), app.RunArgs(context.Background(), os.Args[1:]); got != want {
		t.Fatalf("Run = %d, RunArgs(os.Args) = %d", got, want)
	}
}

// failingWriter fails every write with err.
type failingWriter struct{ err error }

func (f failingWriter) Write([]byte) (int, error) { return 0, f.err }

func TestEmitMapsWriteFailuresToTheirStatuses(t *testing.T) {
	outcome := application.Outcome{ExitCode: 0, Stdout: "out\n", Stderr: "err\n"}
	tests := []struct {
		name   string
		stdout io.Writer
		stderr io.Writer
		want   int
	}{
		{name: "all written", stdout: io.Discard, stderr: io.Discard, want: 0},
		{name: "stdout closed", stdout: failingWriter{syscall.EPIPE}, stderr: io.Discard, want: 141},
		{name: "stdout broken", stdout: failingWriter{errInjected}, stderr: io.Discard, want: 2},
		{name: "stderr closed", stdout: io.Discard, stderr: failingWriter{syscall.EPIPE}, want: 141},
		{name: "stderr broken", stdout: io.Discard, stderr: failingWriter{errInjected}, want: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := emit(test.stdout, test.stderr, outcome); got != test.want {
				t.Fatalf("emit = %d, want %d", got, test.want)
			}
		})
	}
}
