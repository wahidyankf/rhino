package cli

import (
	"syscall"
	"testing"
)

// The cancellation cause names the signal that ended the run, so a report can
// say why the run stopped.
func TestSignalCauseNamesTheSignalThatEndedTheRun(t *testing.T) {
	if got := signalCause(syscall.SIGTERM).Error(); got != "terminated signal received" {
		t.Fatalf("Error() = %q, want the terminated signal named", got)
	}
}
