package application_test

import (
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
	"github.com/wahidyankf/rhino/internal/domain"
	"github.com/wahidyankf/rhino/tests/bdd/vault/fakebw"
)

// TestEnvironmentVaultRefusesALoginWithNoPassword proves a login the provider
// holds with a null password refuses the run before any write. No empty value is
// written as a KEY= line in its place.
func TestEnvironmentVaultRefusesALoginWithNoPassword(t *testing.T) {
	v, inner := fixtureVault()
	v.Create(fakebw.Item{Name: "example/fixture/web/local/alpha-key", Type: 1, Login: &fakebw.Login{}})
	addLogin(v, "example/fixture/web/local/beta-key", "marker-beta-vault")
	tree := transferTree(fixtureConfig(fixtureEnvEntry))
	store := &recordingStore{tree: tree}
	outcome := runTransfer(tree, inner, store, "env", "pull")
	expectRefusal(t, outcome, domain.VaultRefused)
	if len(store.writes) != 0 {
		t.Fatalf("a refused pull wrote %d files", len(store.writes))
	}
}

// TestEnvironmentVaultNeverPullsAnOlderGenerationForANewestOneThatWillNotRead
// proves a newest generation the provider cannot read refuses the pull, and the
// run never falls back to an older generation of the secret.
func TestEnvironmentVaultNeverPullsAnOlderGenerationForANewestOneThatWillNotRead(t *testing.T) {
	v, inner := fixtureVault()
	addNote(v, "example/fixture/api/local/service-config", map[string]string{
		"service.json.g0001": "marker-service-one",
		"service.json.g0002": "marker-service-two",
	})
	unreadable := &application.VaultFault{Code: domain.VaultUnavailable, Reason: "the attachment cannot be read"}
	tree := transferTree(fixtureConfig(fixtureFileEntry))
	store := &recordingStore{tree: tree}
	outcome := runTransfer(tree, newFaultyVault(inner, map[string]*application.VaultFault{"ReadAttachment": unreadable}),
		store, "env", "pull")
	expectRefusal(t, outcome, domain.VaultUnavailable)
	if len(store.writes) != 0 {
		t.Fatalf("a refused pull wrote %d files", len(store.writes))
	}
}
