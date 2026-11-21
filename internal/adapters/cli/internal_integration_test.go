// Integration level: this file reaches the real host, so the default build omits it (see .golangci.yml).

//go:build integration

package cli

import (
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
