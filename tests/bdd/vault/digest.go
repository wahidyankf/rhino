package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// DigestTree fingerprints every entry under root: its relative path, its
// permission bits, and for a regular file its bytes. Two equal digests mean
// the run changed no byte and no mode, so a scenario can prove that a refused
// command wrote nothing. Links are described by their target, never followed.
func DigestTree(root string) (string, error) {
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, _ fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(hash, "%s %s\n", filepath.ToSlash(rel), info.Mode())
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			fmt.Fprintf(hash, "link %s\n", target)
		case info.Mode().IsRegular():
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			hash.Write(data)
			hash.Write([]byte{'\n'})
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("digest %s: %w", root, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
