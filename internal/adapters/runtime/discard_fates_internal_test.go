package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// A close that fails after the write or sync already failed is joined to that
// failure, so neither is dropped. Each case checks that both errors are kept.
func twoJoined(err error) bool {
	joined, ok := err.(interface{ Unwrap() []error })
	return ok && len(joined.Unwrap()) == 2
}

func TestReplacementKeepsACloseFailureWithTheWriteFailure(t *testing.T) {
	log := &callLog{failOps: map[string]bool{"Write": true, "Close": true}}
	m := repository(log, map[string]string{"a.txt": "old"})
	_ = m.mkdirAll("/snapshot", 0o755)
	m.nodes["/snapshot/a.txt"] = &memNode{perm: 0o644, data: []byte("new")}
	useDoubles(t, m, nil, nil)
	err := replaceWorkingFile("/repo", "/snapshot", "a.txt")
	if err == nil || !strings.Contains(err.Error(), "could not write atomic replacement") || !twoJoined(err) {
		t.Fatalf("replaceWorkingFile = %v, want the write failure and the close failure both kept", err)
	}
}

func TestSyncDirectoryKeepsACloseFailureWithTheSyncFailure(t *testing.T) {
	log := &callLog{failOps: map[string]bool{"Sync": true, "Close": true}}
	useDoubles(t, repository(log, nil), nil, nil)
	err := syncDirectory("/repo")
	if err == nil || !strings.Contains(err.Error(), "cannot sync repo") || !twoJoined(err) {
		t.Fatalf("syncDirectory = %v, want the sync failure and the close failure both kept", err)
	}
}

func TestVaultStageKeepsACloseFailureWithTheRefusal(t *testing.T) {
	log := &callLog{failOps: map[string]bool{"Close": true}}
	useDoubles(t, repository(log, nil), nil, nil)
	store := DiskVaultFileStore{fail: func(step string) error {
		if step == stepClose {
			return errors.New("stage close refused")
		}
		return nil
	}}
	err := store.Write("/repo", application.VaultFile{Path: "config/.env", Contents: []byte("X=1"), Mode: 0o600})
	if err == nil || !strings.Contains(err.Error(), "cannot close `config/.env`") || !twoJoined(err) {
		t.Fatalf("Write = %v, want the refusal and the close failure both kept", err)
	}
}
