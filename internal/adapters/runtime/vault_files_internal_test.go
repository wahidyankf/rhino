package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// errSeam is the failure a test seam returns for the step it fails.
var errSeam = errors.New("seam failure")

// failAt returns a seam that fails exactly the named step.
func failAt(step string) func(string) error {
	return func(name string) error {
		if name == step {
			return errSeam
		}
		return nil
	}
}

// assertTarget reads the target and its mode and compares both with the wanted ones.
func assertTarget(t *testing.T, target, content string, mode os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(data) != content {
		t.Fatalf("target holds %q, want %q", data, content)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != mode {
		t.Fatalf("target mode %o, want %o", info.Mode().Perm(), mode)
	}
}

// assertNoStaged fails when a temporary file of a write is left beside the target.
func assertNoStaged(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".rhino-vault-") {
			t.Fatalf("a failed write left %s beside the target", entry.Name())
		}
	}
}

// TestVaultFileWriteFailureBeforeTheRenameKeepsTheOldTarget proves a write that
// fails at any step before its rename leaves an existing target with its old
// bytes and mode, and removes its temporary file, so nothing is left behind.
func TestVaultFileWriteFailureBeforeTheRenameKeepsTheOldTarget(t *testing.T) {
	for _, step := range []string{stepStage, stepWrite, stepChmod, stepSync, stepClose, stepRename} {
		t.Run(step, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "config")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(dir, "app.env")
			if err := os.WriteFile(target, []byte("old\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			store := DiskVaultFileStore{fail: failAt(step)}
			err := store.Write(root, application.VaultFile{Path: "config/app.env", Contents: []byte("new\n"), Mode: 0o640, Replace: true})
			if err == nil || !strings.Contains(err.Error(), errSeam.Error()) {
				t.Fatalf("write error = %v, want the seam failure at %s", err, step)
			}
			assertTarget(t, target, "old\n", 0o600)
			assertNoStaged(t, dir)
		})
	}
}

// TestVaultFileWriteFailureBeforeTheRenameLeavesNoNewTarget proves a write of a
// new file that fails before its rename creates no target and no temporary file.
func TestVaultFileWriteFailureBeforeTheRenameLeavesNoNewTarget(t *testing.T) {
	for _, step := range []string{stepWrite, stepChmod, stepSync, stepClose, stepRename} {
		t.Run(step, func(t *testing.T) {
			root := t.TempDir()
			store := DiskVaultFileStore{fail: failAt(step)}
			err := store.Write(root, application.VaultFile{Path: "config/app.env", Contents: []byte("new\n"), Mode: 0o600})
			if err == nil {
				t.Fatalf("write succeeded with %s failing", step)
			}
			if _, statErr := os.Lstat(filepath.Join(root, "config", "app.env")); !os.IsNotExist(statErr) {
				t.Fatalf("a failed write created the target at %s", step)
			}
			assertNoStaged(t, filepath.Join(root, "config"))
		})
	}
}

// TestVaultFileWriteFailureAfterTheRenameLeavesTheNewTargetWhole proves a
// failure after the rename, while the directory is synced, leaves the target
// fully new with its declared mode, never a mix, and still removes its temporary file.
func TestVaultFileWriteFailureAfterTheRenameLeavesTheNewTargetWhole(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "config")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "app.env")
	if err := os.WriteFile(target, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := DiskVaultFileStore{fail: failAt(stepSyncDirectory)}
	err := store.Write(root, application.VaultFile{Path: "config/app.env", Contents: []byte("new\n"), Mode: 0o640, Replace: true})
	if err == nil || !strings.Contains(err.Error(), errSeam.Error()) {
		t.Fatalf("write error = %v, want the seam failure at the directory sync", err)
	}
	assertTarget(t, target, "new\n", 0o640)
	assertNoStaged(t, dir)
}

// TestDiskTreeFileModeNeverFollowsALink proves the mode query refuses a path that
// passes through a link, the way a read does, and reads the mode of a regular file.
func TestDiskTreeFileModeNeverFollowsALink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "app.env"), []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "config")); err != nil {
		t.Skipf("symbolic links are not available here: %v", err)
	}
	tree := &DiskTree{Dir: root}
	if _, err := tree.FileMode("config/app.env"); err == nil || err.Kind != "escapes" {
		t.Fatalf("a mode through a link = %+v, want escapes", err)
	}
	local := filepath.Join(root, "local.env")
	if err := os.WriteFile(local, []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(local, 0o640); err != nil {
		t.Fatal(err)
	}
	if mode, err := tree.FileMode("local.env"); err != nil || mode != 0o640 {
		t.Fatalf("mode = %o, %+v; want 0640", mode, err)
	}
}
