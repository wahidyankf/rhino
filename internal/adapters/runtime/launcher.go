package runtime

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/wahidyankf/rhino/internal/application"
)

// gateSurfaceVariable is the one environment variable RHINO sets on every
// gate child, and the only thing a leaf may branch on. It mirrors the
// oracle's `.env("RHINO_GATE_SURFACE", launch.surface)`.
const gateSurfaceVariable = "RHINO_GATE_SURFACE"

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

// Launch runs the declared vector in its directory and maps the result to
// the gate vocabulary: 127 not found, 126 not executable, other failures
// refused, nonzero passed through for the caller to report as findings.
//
// The child inherits the process environment plus the selecting surface and
// every projected entry, mirroring the oracle's inherited environment plus
// `.env("RHINO_GATE_SURFACE", ...)` plus `.envs(...)`: a projected entry
// overwrites an inherited value rather than duplicating it.
func (launcher OSLauncher) Launch(launch application.GateLaunch) application.Launched {
	if len(launch.Argv) == 0 {
		return application.Launched{Refused: "declares no executable command"}
	}
	ctx := launcher.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	command := exec.CommandContext(ctx, launch.Argv[0], launch.Argv[1:]...)
	command.Dir = launch.Directory
	// A push surface hands its update records to every child unmodified;
	// any other surface leaves the child with empty standard input.
	if launch.Stdin != nil {
		command.Stdin = strings.NewReader(*launch.Stdin)
	}
	command.Stdout = nil
	command.Stderr = nil
	command.Env = gateEnvironment(launch)
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

// gateEnvironment merges the inherited process environment with the gate's
// own variables. Projected entries overwrite inherited ones by name, and the
// selecting surface always wins, so a forged inherited RHINO_GATE_SURFACE
// never reaches the child. The result is sorted for deterministic dispatch.
func gateEnvironment(launch application.GateLaunch) []string {
	merged := make(map[string]string)
	for _, entry := range os.Environ() {
		name, value, _ := strings.Cut(entry, "=")
		merged[name] = value
	}
	merged[gateSurfaceVariable] = launch.Surface
	for name, value := range launch.Environment {
		merged[name] = value
	}
	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)
	environment := make([]string, 0, len(names))
	for _, name := range names {
		environment = append(environment, name+"="+merged[name])
	}
	return environment
}
