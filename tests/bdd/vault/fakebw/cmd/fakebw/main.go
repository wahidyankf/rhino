// Command fakebw is the built form of the fake Bitwarden CLI test double. A
// test installs a bw wrapper that runs it with the scenario's state and log.
package main

import (
	"os"

	"github.com/wahidyankf/rhino/tests/bdd/vault/fakebw"
)

func main() {
	os.Exit(fakebw.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Environ()))
}
