package runtime

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

// This file holds the seams the adapters reach the operating system through.
// Production wiring keeps the operating-system implementations below, which
// forward each call unchanged. Unit tests replace the package variables with
// doubles, so the adapter bodies run without touching the host.

// fileSystem is the file-system boundary every adapter reaches.
type fileSystem interface {
	Getwd() (string, error)
	Lstat(name string) (fs.FileInfo, error)
	Stat(name string) (fs.FileInfo, error)
	Readlink(name string) (string, error)
	EvalSymlinks(name string) (string, error)
	ReadFile(name string) ([]byte, error)
	ReadDir(name string) ([]fs.DirEntry, error)
	WalkDir(root string, visit fs.WalkDirFunc) error
	Mkdir(name string, perm fs.FileMode) error
	MkdirAll(name string, perm fs.FileMode) error
	MkdirTemp(dir, pattern string) (string, error)
	CreateTemp(dir, pattern string) (stagedFile, error)
	OpenFile(name string, flag int, perm fs.FileMode) (stagedFile, error)
	Open(name string) (stagedFile, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
	Chmod(name string, mode fs.FileMode) error
	Rename(oldpath, newpath string) error
	Remove(name string) error
	RemoveAll(name string) error
}

// stagedFile is an open file an adapter writes, syncs, or closes.
type stagedFile interface {
	Name() string
	Write(p []byte) (int, error)
	WriteString(s string) (int, error)
	Chmod(mode fs.FileMode) error
	Sync() error
	Close() error
}

// commandRunner is the process boundary every adapter reaches. Command and
// CommandContext build a child without starting it; Output, Run, and Start run it.
type commandRunner interface {
	Command(name string, arguments ...string) *exec.Cmd
	CommandContext(ctx context.Context, name string, arguments ...string) *exec.Cmd
	Output(command *exec.Cmd) ([]byte, error)
	Run(command *exec.Cmd) error
	Start(command *exec.Cmd) error
	LookPath(file string) (string, error)
}

// environment is the process environment the adapters read.
type environment interface {
	Getenv(key string) string
	LookupEnv(key string) (string, bool)
	Environ() []string
	Getpid() int
}

// The operating-system calls the wrappers below forward to. Each is a variable,
// so a unit test can forward a wrapper to a recorder and check that the wrapper
// passes its arguments and results through unchanged. Production keeps the real
// calls, and no wrapper adds behaviour of its own.
var (
	sysGetwd          = os.Getwd
	sysLstat          = os.Lstat
	sysStat           = os.Stat
	sysReadlink       = os.Readlink
	sysEvalSymlinks   = filepath.EvalSymlinks
	sysReadFile       = os.ReadFile
	sysReadDir        = os.ReadDir
	sysWalkDir        = filepath.WalkDir
	sysMkdir          = os.Mkdir
	sysMkdirAll       = os.MkdirAll
	sysMkdirTemp      = os.MkdirTemp
	sysCreateTemp     = os.CreateTemp
	sysOpenFile       = os.OpenFile
	sysOpen           = os.Open
	sysWriteFile      = os.WriteFile
	sysChmod          = os.Chmod
	sysRename         = os.Rename
	sysRemove         = os.Remove
	sysRemoveAll      = os.RemoveAll
	sysCommand        = exec.Command
	sysCommandContext = exec.CommandContext
	sysOutput         = (*exec.Cmd).Output
	sysRun            = (*exec.Cmd).Run
	sysStart          = (*exec.Cmd).Start
	sysLookPath       = exec.LookPath
	sysGetenv         = os.Getenv
	sysLookupEnv      = os.LookupEnv
	sysEnviron        = os.Environ
	sysGetpid         = os.Getpid
)

// osFileSystem forwards each file-system call to the operating system.
type osFileSystem struct{}

func (osFileSystem) Getwd() (string, error)                     { return sysGetwd() }
func (osFileSystem) Lstat(name string) (fs.FileInfo, error)     { return sysLstat(name) }
func (osFileSystem) Stat(name string) (fs.FileInfo, error)      { return sysStat(name) }
func (osFileSystem) Readlink(name string) (string, error)       { return sysReadlink(name) }
func (osFileSystem) EvalSymlinks(name string) (string, error)   { return sysEvalSymlinks(name) }
func (osFileSystem) ReadFile(name string) ([]byte, error)       { return sysReadFile(name) }
func (osFileSystem) ReadDir(name string) ([]fs.DirEntry, error) { return sysReadDir(name) }
func (osFileSystem) WalkDir(root string, visit fs.WalkDirFunc) error {
	return sysWalkDir(root, visit)
}
func (osFileSystem) Mkdir(name string, perm fs.FileMode) error    { return sysMkdir(name, perm) }
func (osFileSystem) MkdirAll(name string, perm fs.FileMode) error { return sysMkdirAll(name, perm) }
func (osFileSystem) MkdirTemp(dir, pattern string) (string, error) {
	return sysMkdirTemp(dir, pattern)
}
func (osFileSystem) CreateTemp(dir, pattern string) (stagedFile, error) {
	return sysCreateTemp(dir, pattern)
}
func (osFileSystem) OpenFile(name string, flag int, perm fs.FileMode) (stagedFile, error) {
	return sysOpenFile(name, flag, perm)
}
func (osFileSystem) Open(name string) (stagedFile, error) { return sysOpen(name) }
func (osFileSystem) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return sysWriteFile(name, data, perm)
}
func (osFileSystem) Chmod(name string, mode fs.FileMode) error { return sysChmod(name, mode) }
func (osFileSystem) Rename(oldpath, newpath string) error      { return sysRename(oldpath, newpath) }
func (osFileSystem) Remove(name string) error                  { return sysRemove(name) }
func (osFileSystem) RemoveAll(name string) error               { return sysRemoveAll(name) }

// osCommands forwards each process call to the operating system.
type osCommands struct{}

func (osCommands) Command(name string, arguments ...string) *exec.Cmd {
	return sysCommand(name, arguments...)
}
func (osCommands) CommandContext(ctx context.Context, name string, arguments ...string) *exec.Cmd {
	return sysCommandContext(ctx, name, arguments...)
}
func (osCommands) Output(command *exec.Cmd) ([]byte, error) { return sysOutput(command) }
func (osCommands) Run(command *exec.Cmd) error              { return sysRun(command) }
func (osCommands) Start(command *exec.Cmd) error            { return sysStart(command) }
func (osCommands) LookPath(file string) (string, error)     { return sysLookPath(file) }

// osEnvironment forwards each environment read to the operating system.
type osEnvironment struct{}

func (osEnvironment) Getenv(key string) string            { return sysGetenv(key) }
func (osEnvironment) LookupEnv(key string) (string, bool) { return sysLookupEnv(key) }
func (osEnvironment) Environ() []string                   { return sysEnviron() }
func (osEnvironment) Getpid() int                         { return sysGetpid() }

// The seams production code reaches. Tests replace them with doubles.
var (
	hostFiles    fileSystem    = osFileSystem{}
	hostCommands commandRunner = osCommands{}
	hostEnv      environment   = osEnvironment{}
)

// The process values the command seam carries. Unit-test doubles name them
// through these aliases, so a unit test builds a child description without
// importing os/exec: it starts no child, and the hermetic rule holds (C21).
type (
	hostCommand   = exec.Cmd
	hostExitError = exec.ExitError
)

// hostErrNotFound is the failure a runner reports for an absent executable.
var hostErrNotFound = exec.ErrNotFound
