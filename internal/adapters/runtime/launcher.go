package runtime

import (
	"context"
	"errors"
	"os"
	"os/exec"

	"github.com/wahidyankf/rhino/internal/application"
)

// OSLauncher starts declared gate children as operating-system processes.
// Only gate dispatch uses it; tree validators never do.
//
// Ctx carries cancellation from the process boundary (interrupt or
// termination) to the running child. It is a field rather than a Launch
// parameter so the application library stays signal-free the way the oracle's
// does: a signal death never appears in an outcome, and the CLI boundary
// translates a cancelled run to 128+N. A nil Ctx runs to completion.
type OSLauncher struct {
	Ctx context.Context
}

// Launch runs argv in directory and maps the result to the gate vocabulary:
// 127 not found, 126 not executable, other failures refused, nonzero passed
// through for the caller to report as findings.
func (launcher OSLauncher) Launch(argv []string, directory string) application.Launched {
	if len(argv) == 0 {
		return application.Launched{Refused: "declares no executable command"}
	}
	ctx := launcher.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	command.Dir = directory
	command.Stdin = nil
	command.Stdout = nil
	command.Stderr = nil
	command.Env = os.Environ()
	if err := command.Run(); err != nil {
		// Cancellation wins over every other classification: a child the
		// boundary stopped is a protocol failure, never a finding and never
		// a code the child chose.
		if ctx.Err() != nil {
			return application.Launched{Refused: "gate child interrupted"}
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return application.Launched{Code: exit.ExitCode()}
		}
		// LookPath reports a bare name through ErrNotFound, while an
		// absolute path surfaces the kernel's ENOENT directly; the oracle
		// maps both by kind to 127.
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			return application.Launched{NotFound: true}
		}
		// The oracle maps PermissionDenied to 126. Go surfaces a
		// non-executable child as EACCES, which is neither an ExitError nor
		// ErrNotFound, so without this arm the 126 path is unreachable.
		if os.IsPermission(err) {
			return application.Launched{NotExecutable: true}
		}
		return application.Launched{Refused: err.Error()}
	}
	return application.Launched{Code: 0}
}
