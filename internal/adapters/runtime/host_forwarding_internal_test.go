package runtime

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"testing"
)

// errForward is the answer every recorder gives. A wrapper that dropped its
// result would return nil and fail the check.
var errForward = errors.New("forwarded")

// swapCall replaces one operating-system call with a recorder and returns the
// function that restores the call.
func swapCall[T any](target *T, recorder T) (restore func()) {
	previous := *target
	*target = recorder
	return func() { *target = previous }
}

// TestOSSeamsForwardEachCallUnchanged replaces every operating-system call the
// seams forward to with a recorder, then calls each wrapper with a probe. Each
// recorder checks what it received, so a wrapper that dropped, reordered, or
// altered an argument or result fails here. No call reaches the host.
func TestOSSeamsForwardEachCallUnchanged(t *testing.T) {
	const probe = "probe"
	var seen string
	system := osFileSystem{}

	defer swapCall(&sysGetwd, func() (string, error) { return probe, errForward })()
	if dir, err := system.Getwd(); dir != probe || err != errForward {
		t.Fatalf("Getwd forwarded %q, %v", dir, err)
	}

	defer swapCall(&sysLstat, func(name string) (fs.FileInfo, error) { seen = name; return nil, errForward })()
	if _, err := system.Lstat(probe); err != errForward || seen != probe {
		t.Fatalf("Lstat forwarded %q, %v", seen, err)
	}

	defer swapCall(&sysStat, func(name string) (fs.FileInfo, error) { seen = name; return nil, errForward })()
	if _, err := system.Stat(probe); err != errForward || seen != probe {
		t.Fatalf("Stat forwarded %q, %v", seen, err)
	}

	defer swapCall(&sysReadlink, func(name string) (string, error) { seen = name; return probe, errForward })()
	if got, err := system.Readlink(probe); got != probe || err != errForward || seen != probe {
		t.Fatalf("Readlink forwarded %q, %q, %v", seen, got, err)
	}

	defer swapCall(&sysEvalSymlinks, func(name string) (string, error) { seen = name; return probe, errForward })()
	if got, err := system.EvalSymlinks(probe); got != probe || err != errForward || seen != probe {
		t.Fatalf("EvalSymlinks forwarded %q, %q, %v", seen, got, err)
	}

	defer swapCall(&sysReadFile, func(name string) ([]byte, error) { seen = name; return []byte(probe), errForward })()
	if got, err := system.ReadFile(probe); string(got) != probe || err != errForward || seen != probe {
		t.Fatalf("ReadFile forwarded %q, %q, %v", seen, got, err)
	}

	defer swapCall(&sysReadDir, func(name string) ([]fs.DirEntry, error) { seen = name; return nil, errForward })()
	if _, err := system.ReadDir(probe); err != errForward || seen != probe {
		t.Fatalf("ReadDir forwarded %q, %v", seen, err)
	}

	defer swapCall(&sysWalkDir, func(root string, visit fs.WalkDirFunc) error { seen = root; return visit(root, nil, errForward) })()
	visited := false
	err := system.WalkDir(probe, func(string, fs.DirEntry, error) error { visited = true; return errForward })
	if err != errForward || seen != probe || !visited {
		t.Fatalf("WalkDir forwarded %q, %v, visited %v", seen, err, visited)
	}

	defer swapCall(&sysMkdir, func(name string, perm fs.FileMode) error { seen = name; return errForward })()
	if err := system.Mkdir(probe, 0o755); err != errForward || seen != probe {
		t.Fatalf("Mkdir forwarded %q, %v", seen, err)
	}

	defer swapCall(&sysMkdirAll, func(name string, perm fs.FileMode) error { seen = name; return errForward })()
	if err := system.MkdirAll(probe, 0o755); err != errForward || seen != probe {
		t.Fatalf("MkdirAll forwarded %q, %v", seen, err)
	}

	defer swapCall(&sysMkdirTemp, func(dir, pattern string) (string, error) { seen = dir + pattern; return probe, errForward })()
	if got, err := system.MkdirTemp("d", "p"); got != probe || err != errForward || seen != "dp" {
		t.Fatalf("MkdirTemp forwarded %q, %q, %v", seen, got, err)
	}

	defer swapCall(&sysCreateTemp, func(dir, pattern string) (*os.File, error) { seen = dir + pattern; return nil, errForward })()
	if _, err := system.CreateTemp("d", "p"); err != errForward || seen != "dp" {
		t.Fatalf("CreateTemp forwarded %q, %v", seen, err)
	}

	defer swapCall(&sysOpenFile, func(name string, flag int, perm fs.FileMode) (*os.File, error) {
		seen = name
		return nil, errForward
	})()
	if _, err := system.OpenFile(probe, os.O_RDONLY, 0o600); err != errForward || seen != probe {
		t.Fatalf("OpenFile forwarded %q, %v", seen, err)
	}

	defer swapCall(&sysOpen, func(name string) (*os.File, error) { seen = name; return nil, errForward })()
	if _, err := system.Open(probe); err != errForward || seen != probe {
		t.Fatalf("Open forwarded %q, %v", seen, err)
	}

	defer swapCall(&sysWriteFile, func(name string, data []byte, perm fs.FileMode) error { seen = name + string(data); return errForward })()
	if err := system.WriteFile("n", []byte("d"), 0o600); err != errForward || seen != "nd" {
		t.Fatalf("WriteFile forwarded %q, %v", seen, err)
	}

	defer swapCall(&sysChmod, func(name string, mode fs.FileMode) error { seen = name; return errForward })()
	if err := system.Chmod(probe, 0o600); err != errForward || seen != probe {
		t.Fatalf("Chmod forwarded %q, %v", seen, err)
	}

	defer swapCall(&sysRename, func(oldpath, newpath string) error { seen = oldpath + newpath; return errForward })()
	if err := system.Rename("a", "b"); err != errForward || seen != "ab" {
		t.Fatalf("Rename forwarded %q, %v", seen, err)
	}

	defer swapCall(&sysRemove, func(name string) error { seen = name; return errForward })()
	if err := system.Remove(probe); err != errForward || seen != probe {
		t.Fatalf("Remove forwarded %q, %v", seen, err)
	}

	defer swapCall(&sysRemoveAll, func(name string) error { seen = name; return errForward })()
	if err := system.RemoveAll(probe); err != errForward || seen != probe {
		t.Fatalf("RemoveAll forwarded %q, %v", seen, err)
	}

	defer swapCall(&sysCommand, func(name string, arguments ...string) *hostCommand {
		seen = name + arguments[0]
		return &hostCommand{Path: probe}
	})()
	if cmd := (osCommands{}).Command(probe, "x"); cmd.Path != probe || seen != probe+"x" {
		t.Fatalf("Command forwarded %q, %q", seen, cmd.Path)
	}

	defer swapCall(&sysCommandContext, func(ctx context.Context, name string, arguments ...string) *hostCommand {
		seen = name
		return &hostCommand{Path: probe}
	})()
	if cmd := (osCommands{}).CommandContext(context.Background(), probe); cmd.Path != probe || seen != probe {
		t.Fatalf("CommandContext forwarded %q, %q", seen, cmd.Path)
	}

	var ran *hostCommand
	defer swapCall(&sysOutput, func(c *hostCommand) ([]byte, error) { ran = c; return []byte(probe), errForward })()
	if got, err := (osCommands{}).Output(&hostCommand{Path: probe}); string(got) != probe || err != errForward || ran.Path != probe {
		t.Fatalf("Output forwarded %v, %q", err, got)
	}

	defer swapCall(&sysRun, func(c *hostCommand) error { ran = c; return errForward })()
	if err := (osCommands{}).Run(&hostCommand{Path: probe}); err != errForward || ran.Path != probe {
		t.Fatalf("Run forwarded %v", err)
	}

	defer swapCall(&sysStart, func(c *hostCommand) error { ran = c; return errForward })()
	if err := (osCommands{}).Start(&hostCommand{Path: probe}); err != errForward || ran.Path != probe {
		t.Fatalf("Start forwarded %v", err)
	}

	defer swapCall(&sysLookPath, func(file string) (string, error) { seen = file; return probe, errForward })()
	if got, err := (osCommands{}).LookPath(probe); got != probe || err != errForward || seen != probe {
		t.Fatalf("LookPath forwarded %q, %q, %v", seen, got, err)
	}

	defer swapCall(&sysGetenv, func(key string) string { seen = key; return probe })()
	if got := (osEnvironment{}).Getenv(probe); got != probe || seen != probe {
		t.Fatalf("Getenv forwarded %q, %q", seen, got)
	}

	defer swapCall(&sysLookupEnv, func(key string) (string, bool) { seen = key; return probe, true })()
	if got, ok := (osEnvironment{}).LookupEnv(probe); got != probe || !ok || seen != probe {
		t.Fatalf("LookupEnv forwarded %q, %q, %v", seen, got, ok)
	}

	defer swapCall(&sysEnviron, func() []string { return []string{probe} })()
	if got := (osEnvironment{}).Environ(); len(got) != 1 || got[0] != probe {
		t.Fatalf("Environ forwarded %v", got)
	}

	defer swapCall(&sysGetpid, func() int { return 7 })()
	if got := (osEnvironment{}).Getpid(); got != 7 {
		t.Fatalf("Getpid forwarded %d", got)
	}
}
