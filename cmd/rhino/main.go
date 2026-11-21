// Command rhino validates repository hygiene from repo-config.yml.
package main

import (
	"context"
	"os"

	"github.com/wahidyankf/rhino/internal/adapters/cli"
	"github.com/wahidyankf/rhino/internal/bootstrap"
)

// run executes args under the context signals returns, and returns the exit
// status. The context is stopped once execute returns.
func run(args []string, signals func(context.Context) (context.Context, context.CancelFunc), execute func(context.Context, []string) int) int {
	ctx, stop := signals(context.Background())
	defer stop()
	return execute(ctx, args)
}

func main() {
	os.Exit(run(os.Args[1:], cli.SignalContext, bootstrap.Execute))
}
