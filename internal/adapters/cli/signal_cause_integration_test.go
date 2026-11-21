// Integration level: this file reaches the real host, so the default build omits it (see .golangci.yml).

//go:build integration

package cli

import (
	"syscall"
	"testing"
)

// TestSignalCauseNamesTheSignalItCarries proves the cause reads as the signal
// that ended the run, the text an operator sees when a run is cancelled.
func TestSignalCauseNamesTheSignalItCarries(t *testing.T) {
	if got := signalCause(syscall.SIGTERM).Error(); got != "terminated signal received" {
		t.Fatalf("signalCause(SIGTERM).Error() = %q", got)
	}
}
