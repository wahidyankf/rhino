package application_test

import (
	"context"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
	"github.com/wahidyankf/rhino/tests/bdd/vault"
)

// environmentVaultCorpus is the vault validation feature, relative to this package.
const environmentVaultCorpus = "../../specs/behaviours/environment-vault-validation.feature"

// vaultMemoryTree is the in-memory repository a unit scenario runs against. Its
// two Git name queries answer from the tracked and ignored paths the scenario
// declares, so no process starts and no file is read from disk.
type vaultMemoryTree struct {
	*memoryTree
	tracked []string
	ignored []string
}

// TrackedFiles answers the tracked-name query from the declared paths.
func (m *vaultMemoryTree) TrackedFiles() ([]string, error) { return m.tracked, nil }

// IgnoredFiles answers the ignored-name query from the declared paths.
func (m *vaultMemoryTree) IgnoredFiles() ([]string, error) { return m.ignored, nil }

// unitVaultDriver builds one scenario in memory.
type unitVaultDriver struct {
	tree *vaultMemoryTree
}

func newUnitVaultDriver() *unitVaultDriver {
	return &unitVaultDriver{tree: &vaultMemoryTree{memoryTree: newMemoryTree("/repository")}}
}

func (d *unitVaultDriver) Declare(_ context.Context, configuration string) error {
	d.tree.files["repo-config.yml"] = configuration
	d.tree.tracked = append(d.tree.tracked, "repo-config.yml")
	return nil
}

func (d *unitVaultDriver) Track(_ context.Context, path, content string) error {
	d.tree.files[path] = content
	d.tree.tracked = append(d.tree.tracked, path)
	return nil
}

func (d *unitVaultDriver) Ignore(_ context.Context, path, content string) error {
	d.tree.files[path] = content
	d.tree.ignored = append(d.tree.ignored, path)
	return nil
}

func (d *unitVaultDriver) Validate(_ context.Context, asJSON bool) (vault.Result, error) {
	args := []string{"env", "validate"}
	if asJSON {
		args = append(args, "--output", "json")
	}
	outcome := application.Execute(d.tree, args, nil)
	return vault.Result{ExitCode: outcome.ExitCode, Stdout: outcome.Stdout, Stderr: outcome.Stderr}, nil
}

func (d *unitVaultDriver) Close() error { return nil }

// TestEnvironmentVaultValidationAtUnitAdapter runs every scenario of the vault
// validation corpus in process, against the in-memory tree.
func TestEnvironmentVaultValidationAtUnitAdapter(t *testing.T) {
	vault.RunSuite(t, environmentVaultCorpus, func() (vault.Driver, error) {
		return newUnitVaultDriver(), nil
	})
}
