// Package bootstrap composes concrete adapters for the command entry point.
package bootstrap

import (
	"context"
	"os"

	"github.com/wahidyankf/rhino/internal/adapters/cli"
	"github.com/wahidyankf/rhino/internal/adapters/runtime"
)

// Execute runs the composed production command boundary, carrying the
// process-boundary context to the gate-child launcher and the exit mapping.
func Execute(ctx context.Context, args []string) int {
	return cli.Application{
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		Launcher:    runtime.OSLauncher{Ctx: ctx},
		Environment: runtime.DiskEnvironmentStore{},
		Toolchains:  runtime.DiskToolchainRunner{},
		Adapters:    runtime.DiskAdapterStore{},
		Models:      runtime.DiskModelResolver{},
		Mutations:   runtime.DiskMutationRunner{},
	}.RunArgs(ctx, args)
}
