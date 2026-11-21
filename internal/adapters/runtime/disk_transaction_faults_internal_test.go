package runtime

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// The environment transaction reports what it cannot undo: a staging directory
// it could not remove, and a rollback that did not finish. Each case fails the
// operation that carries the failure and checks that the report names it.
func TestEnvironmentTransactionReportsWhatItCannotUndo(t *testing.T) {
	t.Run("a staging directory that cannot be removed", func(t *testing.T) {
		log := &callLog{failOps: map[string]bool{"RemoveAll": true}}
		useDoubles(t, repository(log, map[string]string{}), nil, nil)
		err := (DiskEnvironmentStore{}).Create("/repo", []application.EnvironmentFile{envTarget("config/.env", "X=1")})
		if err == nil || !strings.Contains(err.Error(), "cannot remove staging directory") {
			t.Fatalf("Create = %v, want the staging removal reported", err)
		}
	})

	t.Run("a create rollback that cannot remove what it installed", func(t *testing.T) {
		log := &callLog{failOps: map[string]bool{"Remove": true}}
		useDoubles(t, repository(log, map[string]string{"keep": "old"}), nil, nil)
		err := (DiskEnvironmentStore{}).Create("/repo", []application.EnvironmentFile{
			envTarget("a", "1"), envTarget("keep", "new"),
		})
		if err == nil || !strings.Contains(err.Error(), "rollback incomplete") {
			t.Fatalf("Create = %v, want the rollback failure reported", err)
		}
	})

	t.Run("a restore rollback that cannot remove what it restored", func(t *testing.T) {
		log := &callLog{failOps: map[string]bool{"Remove": true}}
		useDoubles(t, repository(log, map[string]string{"keep": "old"}), nil, nil)
		err := (DiskEnvironmentStore{}).Restore("/repo", []application.EnvironmentRestoreFile{
			{Path: "a", Contents: "A"}, {Path: "keep", Contents: "new", Replace: false},
		})
		if err == nil || !strings.Contains(err.Error(), "rollback incomplete") {
			t.Fatalf("Restore = %v, want the rollback failure reported", err)
		}
	})
}

// The adapter replacement reports a rollback that did not finish, and a staging
// directory it could not remove, instead of dropping either.
func TestAdapterTransactionReportsWhatItCannotUndo(t *testing.T) {
	t.Run("a staging directory that cannot be removed", func(t *testing.T) {
		log := &callLog{failOps: map[string]bool{"RemoveAll": true}}
		useDoubles(t, repository(log, map[string]string{"README.md": "r"}), nil, nil)
		err := (DiskAdapterStore{}).Replace("/repo", adapterTransaction())
		if err == nil || !strings.Contains(err.Error(), "cannot remove staging directory") {
			t.Fatalf("Replace = %v, want the staging removal reported", err)
		}
	})

	t.Run("a rollback that cannot remove what it installed", func(t *testing.T) {
		prior := func(log *callLog) *memFS {
			return repository(log, map[string]string{"config/agents/old.md": "old", "AGENTS.md": "prior"})
		}
		clean := &callLog{}
		useDoubles(t, prior(clean), nil, nil)
		if err := (DiskAdapterStore{}).Replace("/repo", adapterTransaction()); err != nil {
			t.Fatalf("the clean run failed: %v", err)
		}
		// Fail the last install, after the family install has landed, and make
		// every removal in the rollback fail too.
		last := 0
		for k, op := range clean.ops {
			if op == "Rename" {
				last = k + 1
			}
		}
		log := &callLog{failAt: last, cause: errInjected, failOps: map[string]bool{"Remove": true, "RemoveAll": true}}
		useDoubles(t, prior(log), nil, nil)
		err := (DiskAdapterStore{}).Replace("/repo", adapterTransaction())
		if err == nil || !strings.Contains(err.Error(), "rollback incomplete") {
			t.Fatalf("Replace = %v, want the rollback failure reported", err)
		}
	})
}
