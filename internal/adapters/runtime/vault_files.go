package runtime

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/wahidyankf/rhino/internal/application"
)

// DiskVaultFileStore is the write boundary only env pull holds. It writes one
// declared file through a temporary file beside the target: the temporary file
// takes the declared mode, is synced, and is renamed over the target, so the
// target is either the old bytes or the new ones, never a mix. A link on the
// path refuses the write, checked component by component and never followed.
// A failed write removes its temporary file and leaves the target as it was.
type DiskVaultFileStore struct {
	// fail, when set, is asked before each step of a write, and a non-nil answer
	// fails that step as if the operating system had. It is a test seam only; the
	// product never sets it.
	fail func(step string) error
}

// The steps of one write, in order. A test names one to fail it.
const (
	stepStage         = "stage"
	stepWrite         = "write"
	stepChmod         = "chmod"
	stepSync          = "sync"
	stepClose         = "close"
	stepRename        = "rename"
	stepSyncDirectory = "sync-directory"
)

// Write installs file under root. It refuses an existing target unless
// file.Replace is set, and a target that is a directory always.
func (s DiskVaultFileStore) Write(root string, file application.VaultFile) error {
	if _, err := environmentRoot(root); err != nil {
		return err
	}
	if !isNormalPath(file.Path) {
		return fmt.Errorf("invalid target `%s`", file.Path)
	}
	if err := ensureEnvironmentPathIsSafe(root, file.Path); err != nil {
		return err
	}
	target := filepath.Join(root, filepath.FromSlash(file.Path))
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return fmt.Errorf("cannot create the parent of `%s`: %s", file.Path, err.Error())
	}
	if info, err := os.Lstat(target); err == nil {
		if info.IsDir() {
			return fmt.Errorf("`%s` is a directory", file.Path)
		}
		if !file.Replace {
			return fmt.Errorf("`%s` already exists", file.Path)
		}
	}
	if err := s.step(stepStage); err != nil {
		return fmt.Errorf("cannot stage `%s`: %s", file.Path, err.Error())
	}
	staged, err := os.CreateTemp(parent, ".rhino-vault-*")
	if err != nil {
		return fmt.Errorf("cannot stage `%s`: %s", file.Path, err.Error())
	}
	stagedName := staged.Name()
	installed := false
	defer func() {
		if !installed {
			_ = os.Remove(stagedName)
		}
	}()
	if err := s.stageContents(staged, file); err != nil {
		_ = staged.Close()
		return err
	}
	if err := s.step(stepClose); err != nil {
		_ = staged.Close()
		return fmt.Errorf("cannot close `%s`: %s", file.Path, err.Error())
	}
	if err := staged.Close(); err != nil {
		return fmt.Errorf("cannot close `%s`: %s", file.Path, err.Error())
	}
	if err := s.step(stepRename); err != nil {
		return fmt.Errorf("cannot install `%s`: %s", file.Path, err.Error())
	}
	if err := os.Rename(stagedName, target); err != nil {
		return fmt.Errorf("cannot install `%s`: %s", file.Path, err.Error())
	}
	installed = true
	if err := s.step(stepSyncDirectory); err != nil {
		return fmt.Errorf("cannot sync %s: %s", filepath.Base(parent), err.Error())
	}
	return syncDirectory(parent)
}

// stageContents writes the bytes, sets the mode, and syncs the temporary file.
func (s DiskVaultFileStore) stageContents(staged *os.File, file application.VaultFile) error {
	if err := s.step(stepWrite); err != nil {
		return fmt.Errorf("cannot write `%s`: %s", file.Path, err.Error())
	}
	if _, err := staged.Write(file.Contents); err != nil {
		return fmt.Errorf("cannot write `%s`: %s", file.Path, err.Error())
	}
	if err := s.step(stepChmod); err != nil {
		return fmt.Errorf("cannot set the mode of `%s`: %s", file.Path, err.Error())
	}
	if err := staged.Chmod(fs.FileMode(file.Mode)); err != nil {
		return fmt.Errorf("cannot set the mode of `%s`: %s", file.Path, err.Error())
	}
	if err := s.step(stepSync); err != nil {
		return fmt.Errorf("cannot sync `%s`: %s", file.Path, err.Error())
	}
	if err := staged.Sync(); err != nil {
		return fmt.Errorf("cannot sync `%s`: %s", file.Path, err.Error())
	}
	return nil
}

// step asks the test seam, when one is set, whether this step fails.
func (s DiskVaultFileStore) step(name string) error {
	if s.fail == nil {
		return nil
	}
	return s.fail(name)
}

// syncDirectory makes the rename durable by syncing the directory that holds it.
func syncDirectory(directory string) error {
	handle, err := os.Open(directory)
	if err != nil {
		return fmt.Errorf("cannot open %s to sync it: %s", filepath.Base(directory), err.Error())
	}
	if err := handle.Sync(); err != nil {
		_ = handle.Close()
		return fmt.Errorf("cannot sync %s: %s", filepath.Base(directory), err.Error())
	}
	return handle.Close()
}
