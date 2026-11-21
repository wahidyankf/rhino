package runtime

import (
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// vaultFile is one declared local file a pull writes.
func vaultFile(name, contents string, replace bool) application.VaultFile {
	return application.VaultFile{Path: name, Contents: []byte(contents), Mode: 0o600, Replace: replace}
}

func TestDiskVaultFileStoreWritesThroughATemporaryFile(t *testing.T) {
	m := repository(&callLog{}, map[string]string{"keep": "old", "dir/x": "x"})
	useDoubles(t, m, nil, nil)
	store := DiskVaultFileStore{}
	if err := store.Write("/repo", vaultFile("config/.env", "A=1", false)); err != nil {
		t.Fatalf("Write of a new file = %v", err)
	}
	if got := m.get("/repo/config/.env"); got == nil || string(got.data) != "A=1" || got.perm != 0o600 {
		t.Fatalf("written file = %+v", got)
	}
	if err := store.Write("/repo", vaultFile("keep", "new", true)); err != nil {
		t.Fatalf("Write with replace = %v", err)
	}
	if got := string(m.get("/repo/keep").data); got != "new" {
		t.Fatalf("replaced file = %q, want new", got)
	}
	for name := range m.nodes {
		if len(name) > len("/repo/.rhino-vault-") && name[:len("/repo/.rhino-vault-")] == "/repo/.rhino-vault-" {
			t.Fatalf("temporary file %s was left behind", name)
		}
	}
}

func TestDiskVaultFileStoreRefusesEachUnsafeWrite(t *testing.T) {
	cases := map[string]struct {
		root   string
		file   application.VaultFile
		before func(m *memFS)
	}{
		"root is a file":   {root: "/repo/README.md", file: vaultFile("a", "a", false)},
		"invalid target":   {root: "/repo", file: vaultFile("../x", "x", false)},
		"crosses a link":   {root: "/repo", file: vaultFile("linkdir/x", "x", false), before: func(m *memFS) { symlink(m, "linkdir", "dir") }},
		"target is a dir":  {root: "/repo", file: vaultFile("dir", "x", true), before: func(m *memFS) { _ = m.mkdirAll("/repo/dir", 0o755) }},
		"target exists":    {root: "/repo", file: vaultFile("keep", "x", false), before: func(m *memFS) { m.nodes["/repo/keep"] = &memNode{perm: 0o644} }},
		"parent is a file": {root: "/repo", file: vaultFile("keep/x", "x", false), before: func(m *memFS) { m.nodes["/repo/keep"] = &memNode{perm: 0o644} }},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := repository(&callLog{}, map[string]string{"README.md": "r"})
			if tc.before != nil {
				tc.before(m)
			}
			useDoubles(t, m, nil, nil)
			if err := (DiskVaultFileStore{}).Write(tc.root, tc.file); err == nil {
				t.Fatalf("Write(%s) succeeded, want a refusal", name)
			}
		})
	}
}

func TestDiskVaultFileStoreLeavesTheTargetWhenAnyStepFails(t *testing.T) {
	steps := []string{stepStage, stepWrite, stepChmod, stepSync, stepClose, stepRename, stepSyncDirectory}
	for _, step := range steps {
		t.Run(step, func(t *testing.T) {
			m := repository(&callLog{}, map[string]string{"keep": "old"})
			useDoubles(t, m, nil, nil)
			store := DiskVaultFileStore{fail: func(name string) error {
				if name == step {
					return errInjected
				}
				return nil
			}}
			if err := store.Write("/repo", vaultFile("keep", "new", true)); err == nil {
				t.Fatalf("a failed %s step returned no error", step)
			}
			if step == stepSyncDirectory {
				// The rename has already installed the new bytes; only the directory sync failed.
				return
			}
			if got := string(m.get("/repo/keep").data); got != "old" {
				t.Fatalf("a failed %s step changed the target to %q", step, got)
			}
		})
	}
}

func TestDiskVaultFileStoreSweepsEveryCall(t *testing.T) {
	faultSweep(t, func(t *testing.T, log *callLog) func() error {
		useDoubles(t, repository(log, map[string]string{"keep": "old"}), nil, nil)
		return func() error { return (DiskVaultFileStore{}).Write("/repo", vaultFile("keep", "new", true)) }
	}, "Lstat")
}
