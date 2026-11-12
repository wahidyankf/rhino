package runtime_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/runtime"
	"github.com/wahidyankf/rhino/internal/application"
)

// fileRoot is a repository directory for the file store tests.
func fileRoot(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// TestVaultFileStoreWritesTheDeclaredModeAndParents proves a new file is written
// with its declared mode, its missing parents are created 0700, and no
// temporary file is left beside it.
func TestVaultFileStoreWritesTheDeclaredModeAndParents(t *testing.T) {
	root := fileRoot(t)
	store := runtime.DiskVaultFileStore{}
	if err := store.Write(root, application.VaultFile{Path: "config/app.env", Contents: []byte("A=1\n"), Mode: 0o600}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(root, "config", "app.env"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 0600", info.Mode().Perm())
	}
	parent, err := os.Stat(filepath.Join(root, "config"))
	if err != nil {
		t.Fatal(err)
	}
	if parent.Mode().Perm() != 0o700 {
		t.Fatalf("parent mode = %o, want 0700", parent.Mode().Perm())
	}
	entries, err := os.ReadDir(filepath.Join(root, "config"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("config holds %d entries, want only the target", len(entries))
	}
}

// TestVaultFileStoreReplacesOnlyWhenAuthorized proves an existing target is
// refused unless the write replaces it, and a replacement takes the new bytes and mode.
func TestVaultFileStoreReplacesOnlyWhenAuthorized(t *testing.T) {
	root := fileRoot(t)
	target := filepath.Join(root, "app.env")
	if err := os.WriteFile(target, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := runtime.DiskVaultFileStore{}
	if err := store.Write(root, application.VaultFile{Path: "app.env", Contents: []byte("new\n"), Mode: 0o600}); err == nil {
		t.Fatal("an existing target was replaced without authorization")
	}
	if data, _ := os.ReadFile(target); string(data) != "old\n" {
		t.Fatalf("refused write changed the target to %q", data)
	}
	if err := store.Write(root, application.VaultFile{Path: "app.env", Contents: []byte("new\n"), Mode: 0o600, Replace: true}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(target)
	info, _ := os.Stat(target)
	if string(data) != "new\n" || info.Mode().Perm() != 0o600 {
		t.Fatalf("replacement = %q mode %o", data, info.Mode().Perm())
	}
}

// TestVaultFileStoreRefusesLinksOnThePath proves a symbolic link at the target,
// or at any parent, refuses the write, and the file the link names is untouched.
func TestVaultFileStoreRefusesLinksOnThePath(t *testing.T) {
	outside := fileRoot(t)
	outsideFile := filepath.Join(outside, "outside.env")
	if err := os.WriteFile(outsideFile, []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := runtime.DiskVaultFileStore{}

	t.Run("a link at the target", func(t *testing.T) {
		root := fileRoot(t)
		if err := os.Symlink(outsideFile, filepath.Join(root, "app.env")); err != nil {
			t.Fatal(err)
		}
		err := store.Write(root, application.VaultFile{Path: "app.env", Contents: []byte("x"), Mode: 0o600, Replace: true})
		if err == nil {
			t.Fatal("a symbolic link target was written through")
		}
		if data, _ := os.ReadFile(outsideFile); string(data) != "outside\n" {
			t.Fatalf("the linked file changed to %q", data)
		}
	})
	t.Run("a link at a parent", func(t *testing.T) {
		root := fileRoot(t)
		if err := os.Symlink(outside, filepath.Join(root, "config")); err != nil {
			t.Fatal(err)
		}
		if err := store.Write(root, application.VaultFile{Path: "config/app.env", Contents: []byte("x"), Mode: 0o600}); err == nil {
			t.Fatal("a write crossed a parent link")
		}
		if _, err := os.Stat(filepath.Join(outside, "app.env")); err == nil {
			t.Fatal("a write landed outside the repository")
		}
	})
}

// TestVaultFileStoreRefusesWhatItCannotInstall proves a directory target, a
// parent that is a file, an invalid path, and a root that is not a directory
// each refuse the write, and no file is left behind.
func TestVaultFileStoreRefusesWhatItCannotInstall(t *testing.T) {
	store := runtime.DiskVaultFileStore{}
	t.Run("a directory at the target", func(t *testing.T) {
		root := fileRoot(t)
		if err := os.Mkdir(filepath.Join(root, "app.env"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := store.Write(root, application.VaultFile{Path: "app.env", Contents: []byte("x"), Mode: 0o600, Replace: true}); err == nil {
			t.Fatal("a directory target was replaced")
		}
	})
	t.Run("a file where a parent must be", func(t *testing.T) {
		root := fileRoot(t)
		if err := os.WriteFile(filepath.Join(root, "config"), []byte("file\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := store.Write(root, application.VaultFile{Path: "config/app.env", Contents: []byte("x"), Mode: 0o600}); err == nil {
			t.Fatal("a parent that is a file was written under")
		}
	})
	t.Run("a path that leaves the repository", func(t *testing.T) {
		root := fileRoot(t)
		if err := store.Write(root, application.VaultFile{Path: "../app.env", Contents: []byte("x"), Mode: 0o600}); err == nil {
			t.Fatal("a path that leaves the root was accepted")
		}
	})
	t.Run("a root that is not a directory", func(t *testing.T) {
		file := filepath.Join(fileRoot(t), "not-a-directory")
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := store.Write(file, application.VaultFile{Path: "app.env", Contents: []byte("x"), Mode: 0o600}); err == nil {
			t.Fatal("a root that is not a directory was written under")
		}
	})
}
