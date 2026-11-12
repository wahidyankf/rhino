package bdd_test

import (
	"context"

	"github.com/wahidyankf/rhino/tests/bdd/vault"
)

// cliVaultDriver builds the environment vault scenarios in the scenario's own
// repository and runs the built rhino binary there, observing only argv, exit
// code, and streams.
type cliVaultDriver struct {
	world *world
}

func (d *cliVaultDriver) Declare(ctx context.Context, configuration string) error {
	if err := d.world.ensureGit(ctx); err != nil {
		return err
	}
	d.world.config = configuration
	return d.world.writeRepoFile("repo-config.yml", configuration)
}

func (d *cliVaultDriver) Track(ctx context.Context, path, content string) error {
	if err := d.world.writeRepoFile(path, content); err != nil {
		return err
	}
	return d.world.indexSet(ctx, path, content)
}

// Ignore writes the file and names it in the repository's .gitignore, so Git
// reports it as ignored and untracked.
func (d *cliVaultDriver) Ignore(ctx context.Context, path, content string) error {
	if err := d.world.ensureGit(ctx); err != nil {
		return err
	}
	if err := d.world.writeRepoFile(path, content); err != nil {
		return err
	}
	existing, err := d.world.workingContent(".gitignore")
	if err != nil {
		return err
	}
	return d.world.writeRepoFile(".gitignore", existing+"/"+path+"\n")
}

// Validate runs the built binary with PATH reduced to a git shim that logs each
// invocation, so the run can start git and no other program found by name.
func (d *cliVaultDriver) Validate(ctx context.Context, asJSON bool) (vault.Result, error) {
	arguments := "env|validate"
	if asJSON {
		arguments += "|--output|json"
	}
	trace, err := vault.NewGitTrace(d.world.dir)
	if err != nil {
		return vault.Result{}, err
	}
	d.world.pathOnly = trace.PATH()
	err = d.world.invoke(ctx, arguments)
	d.world.pathOnly = ""
	if err != nil {
		return vault.Result{}, err
	}
	processes, err := trace.Processes()
	if err != nil {
		return vault.Result{}, err
	}
	run, err := d.world.lastRun()
	if err != nil {
		return vault.Result{}, err
	}
	return vault.Result{ExitCode: run.exitCode, Stdout: run.stdout, Stderr: run.stderr, Processes: processes}, nil
}

// Close has nothing to remove: the scenario world the corpus creates owns the
// directory and removes it after the scenario.
func (d *cliVaultDriver) Close() error { return nil }
