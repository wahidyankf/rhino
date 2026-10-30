// Command rhino validates repository hygiene from repo-config.yml.
package main

import (
	"context"
	"os"

	"github.com/wahidyankf/rhino/internal/adapters/cli"
	"github.com/wahidyankf/rhino/internal/bootstrap"
)

func run() int {
	ctx, stop := cli.SignalContext(context.Background())
	defer stop()
	return bootstrap.Execute(ctx, os.Args[1:])
}

func main() {
	os.Exit(run())
}
