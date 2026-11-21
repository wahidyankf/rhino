package runtime

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"testing"
	"time"
)

// errInjected is the failure a double returns for a call a test chose to fail.
var errInjected = errors.New("injected failure")

var (
	errNotDir   = errors.New("not a directory")
	errIsDir    = errors.New("is a directory")
	errInvalid  = errors.New("invalid argument")
	errNotEmpty = errors.New("directory not empty")
)

// callLog records every call the doubles of one scenario answer, in order. It
// fails the call numbered failAt (1-based; 0 fails none) and every call of an
// operation named in failOps. One log is shared by all doubles of a scenario,
// so a fault sweep can name any call the scenario makes.
type callLog struct {
	ops     []string
	failAt  int
	cause   error
	failOps map[string]bool
}

// step records one call and returns the injected failure when it is the one
// the log was told to fail.
func (c *callLog) step(op string) error {
	c.ops = append(c.ops, op)
	if c.failAt == len(c.ops) || c.failOps[op] {
		if c.cause != nil {
			return c.cause
		}
		return errInjected
	}
	return nil
}

// memNode is one entry of memFS: a directory, a symbolic link, or a file.
type memNode struct {
	dir  bool
	link string
	perm fs.FileMode
	data []byte
}

// memFS is an in-memory fileSystem double. Paths are slash paths, cleaned on
// entry, and "/" always exists. Every operation is one call on the log.
type memFS struct {
	log   *callLog
	cwd   string
	nodes map[string]*memNode
}

var _ fileSystem = (*memFS)(nil)

func newMemFS(log *callLog) *memFS {
	m := &memFS{log: log, cwd: "/repo", nodes: map[string]*memNode{"/": {dir: true, perm: 0o755}}}
	_ = m.mkdirAll("/repo", 0o755)
	_ = m.mkdirAll("/tmp", 0o777)
	return m
}

func (m *memFS) get(name string) *memNode { return m.nodes[path.Clean(name)] }

// resolve follows symbolic links to a node, a bounded number of hops away.
func (m *memFS) resolve(name string) (string, *memNode) {
	current := path.Clean(name)
	for hop := 0; hop < 8; hop++ {
		node := m.nodes[current]
		if node == nil || node.link == "" {
			return current, node
		}
		target := node.link
		if !path.IsAbs(target) {
			target = path.Join(path.Dir(current), target)
		}
		current = path.Clean(target)
	}
	return current, nil
}

func (m *memFS) parentIsDir(name string) error {
	parent := m.nodes[path.Dir(path.Clean(name))]
	switch {
	case parent == nil:
		return fs.ErrNotExist
	case !parent.dir:
		return errNotDir
	}
	return nil
}

func (m *memFS) mkdirAll(name string, perm fs.FileMode) error {
	var missing []string
	for cur := path.Clean(name); ; cur = path.Dir(cur) {
		if node := m.nodes[cur]; node != nil {
			if _, target := m.resolve(cur); target == nil || !target.dir {
				return errNotDir
			}
			break
		}
		missing = append(missing, cur)
		if cur == "/" || cur == "." {
			break
		}
	}
	for i := len(missing) - 1; i >= 0; i-- {
		m.nodes[missing[i]] = &memNode{dir: true, perm: perm}
	}
	return nil
}

func (m *memFS) children(dir string) []string {
	var out []string
	for name := range m.nodes {
		if name != dir && path.Dir(name) == dir {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func (m *memFS) removeTree(name string) {
	for key := range m.nodes {
		if key == name || strings.HasPrefix(key, name+"/") {
			delete(m.nodes, key)
		}
	}
}

func (m *memFS) moveTree(from, to string) {
	m.removeTree(to)
	var keys []string
	for name := range m.nodes {
		if name == from || strings.HasPrefix(name, from+"/") {
			keys = append(keys, name)
		}
	}
	for _, name := range keys {
		m.nodes[to+name[len(from):]] = m.nodes[name]
		delete(m.nodes, name)
	}
}

// tempName picks an unused name in dir, the way os.MkdirTemp and os.CreateTemp
// do: the last * in pattern becomes a number.
func (m *memFS) tempName(dir, pattern string) (string, error) {
	if dir == "" {
		dir = "/tmp"
	}
	if parent := m.get(dir); parent == nil || !parent.dir {
		return "", pathError("createtemp", dir, fs.ErrNotExist)
	}
	for n := 1; ; n++ {
		base := pattern + fmt.Sprint(n)
		if i := strings.LastIndex(pattern, "*"); i >= 0 {
			base = pattern[:i] + fmt.Sprint(n) + pattern[i+1:]
		}
		if name := path.Join(dir, base); m.get(name) == nil {
			return name, nil
		}
	}
}

func pathError(op, name string, err error) error {
	return &fs.PathError{Op: op, Path: name, Err: err}
}

func (m *memFS) Getwd() (string, error) {
	if err := m.log.step("Getwd"); err != nil {
		return "", err
	}
	return m.cwd, nil
}

func (m *memFS) Lstat(name string) (fs.FileInfo, error) {
	if err := m.log.step("Lstat"); err != nil {
		return nil, pathError("lstat", name, err)
	}
	node := m.get(name)
	if node == nil {
		return nil, pathError("lstat", name, fs.ErrNotExist)
	}
	return memInfo{name: path.Base(path.Clean(name)), node: node}, nil
}

func (m *memFS) Stat(name string) (fs.FileInfo, error) {
	if err := m.log.step("Stat"); err != nil {
		return nil, pathError("stat", name, err)
	}
	_, node := m.resolve(name)
	if node == nil {
		return nil, pathError("stat", name, fs.ErrNotExist)
	}
	return memInfo{name: path.Base(path.Clean(name)), node: node}, nil
}

func (m *memFS) Readlink(name string) (string, error) {
	if err := m.log.step("Readlink"); err != nil {
		return "", pathError("readlink", name, err)
	}
	node := m.get(name)
	switch {
	case node == nil:
		return "", pathError("readlink", name, fs.ErrNotExist)
	case node.link == "":
		return "", pathError("readlink", name, errInvalid)
	}
	return node.link, nil
}

func (m *memFS) EvalSymlinks(name string) (string, error) {
	if err := m.log.step("EvalSymlinks"); err != nil {
		return "", err
	}
	resolved, node := m.resolve(name)
	if node == nil {
		return "", pathError("lstat", name, fs.ErrNotExist)
	}
	return resolved, nil
}

func (m *memFS) ReadFile(name string) ([]byte, error) {
	if err := m.log.step("ReadFile"); err != nil {
		return nil, pathError("open", name, err)
	}
	_, node := m.resolve(name)
	switch {
	case node == nil:
		return nil, pathError("open", name, fs.ErrNotExist)
	case node.dir:
		return nil, pathError("read", name, errIsDir)
	}
	return append([]byte(nil), node.data...), nil
}

func (m *memFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if err := m.log.step("ReadDir"); err != nil {
		return nil, pathError("readdirent", name, err)
	}
	node := m.get(name)
	switch {
	case node == nil:
		return nil, pathError("open", name, fs.ErrNotExist)
	case !node.dir:
		return nil, pathError("readdirent", name, errNotDir)
	}
	var entries []fs.DirEntry
	for _, child := range m.children(path.Clean(name)) {
		entries = append(entries, memDirEntry{memInfo{name: path.Base(child), node: m.nodes[child]}})
	}
	return entries, nil
}

func (m *memFS) WalkDir(root string, visit fs.WalkDirFunc) error {
	if err := m.log.step("WalkDir"); err != nil {
		return visit(root, nil, err)
	}
	if m.get(root) == nil {
		return visit(root, nil, fs.ErrNotExist)
	}
	err := m.walk(path.Clean(root), visit)
	if err == fs.SkipDir || err == fs.SkipAll {
		return nil
	}
	return err
}

func (m *memFS) walk(name string, visit fs.WalkDirFunc) error {
	node := m.nodes[name]
	err := visit(name, memDirEntry{memInfo{name: path.Base(name), node: node}}, nil)
	if err != nil || !node.dir {
		if err == fs.SkipDir && node.dir {
			return nil
		}
		return err
	}
	for _, child := range m.children(name) {
		if err := m.walk(child, visit); err != nil {
			if err == fs.SkipDir {
				break
			}
			return err
		}
	}
	return nil
}

func (m *memFS) Mkdir(name string, perm fs.FileMode) error {
	if err := m.log.step("Mkdir"); err != nil {
		return pathError("mkdir", name, err)
	}
	if m.get(name) != nil {
		return pathError("mkdir", name, fs.ErrExist)
	}
	if err := m.parentIsDir(name); err != nil {
		return pathError("mkdir", name, err)
	}
	m.nodes[path.Clean(name)] = &memNode{dir: true, perm: perm}
	return nil
}

func (m *memFS) MkdirAll(name string, perm fs.FileMode) error {
	if err := m.log.step("MkdirAll"); err != nil {
		return pathError("mkdir", name, err)
	}
	return m.mkdirAll(name, perm)
}

func (m *memFS) MkdirTemp(dir, pattern string) (string, error) {
	if err := m.log.step("MkdirTemp"); err != nil {
		return "", pathError("mkdirtemp", dir, err)
	}
	name, err := m.tempName(dir, pattern)
	if err != nil {
		return "", err
	}
	m.nodes[name] = &memNode{dir: true, perm: 0o700}
	return name, nil
}

func (m *memFS) CreateTemp(dir, pattern string) (stagedFile, error) {
	if err := m.log.step("CreateTemp"); err != nil {
		return nil, pathError("createtemp", dir, err)
	}
	name, err := m.tempName(dir, pattern)
	if err != nil {
		return nil, err
	}
	node := &memNode{perm: 0o600}
	m.nodes[name] = node
	return &memFile{owner: m, name: name, node: node}, nil
}

func (m *memFS) OpenFile(name string, flag int, perm fs.FileMode) (stagedFile, error) {
	if err := m.log.step("OpenFile"); err != nil {
		return nil, pathError("open", name, err)
	}
	node := m.get(name)
	switch {
	case node == nil && flag&os.O_CREATE == 0:
		return nil, pathError("open", name, fs.ErrNotExist)
	case node != nil && flag&os.O_CREATE != 0 && flag&os.O_EXCL != 0:
		return nil, pathError("open", name, fs.ErrExist)
	case node == nil:
		if err := m.parentIsDir(name); err != nil {
			return nil, pathError("open", name, err)
		}
		node = &memNode{perm: perm}
		m.nodes[path.Clean(name)] = node
	case node.dir:
		return nil, pathError("open", name, errIsDir)
	}
	if flag&os.O_TRUNC != 0 {
		node.data = nil
	}
	return &memFile{owner: m, name: path.Clean(name), node: node}, nil
}

func (m *memFS) Open(name string) (stagedFile, error) {
	if err := m.log.step("Open"); err != nil {
		return nil, pathError("open", name, err)
	}
	node := m.get(name)
	if node == nil {
		return nil, pathError("open", name, fs.ErrNotExist)
	}
	return &memFile{owner: m, name: path.Clean(name), node: node, readOnly: true}, nil
}

func (m *memFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	if err := m.log.step("WriteFile"); err != nil {
		return pathError("open", name, err)
	}
	node := m.get(name)
	switch {
	case node == nil:
		if err := m.parentIsDir(name); err != nil {
			return pathError("open", name, err)
		}
		node = &memNode{perm: perm}
		m.nodes[path.Clean(name)] = node
	case node.dir:
		return pathError("open", name, errIsDir)
	}
	node.data = append([]byte(nil), data...)
	return nil
}

func (m *memFS) Chmod(name string, mode fs.FileMode) error {
	if err := m.log.step("Chmod"); err != nil {
		return pathError("chmod", name, err)
	}
	node := m.get(name)
	if node == nil {
		return pathError("chmod", name, fs.ErrNotExist)
	}
	node.perm = mode.Perm()
	return nil
}

func (m *memFS) Rename(oldpath, newpath string) error {
	if err := m.log.step("Rename"); err != nil {
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: err}
	}
	from, to := path.Clean(oldpath), path.Clean(newpath)
	switch {
	case m.nodes[from] == nil:
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: fs.ErrNotExist}
	case m.parentIsDir(to) != nil:
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: m.parentIsDir(to)}
	case to == from || strings.HasPrefix(to, from+"/"):
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: errInvalid}
	}
	m.moveTree(from, to)
	return nil
}

func (m *memFS) Remove(name string) error {
	if err := m.log.step("Remove"); err != nil {
		return pathError("remove", name, err)
	}
	node := m.get(name)
	switch {
	case node == nil:
		return pathError("remove", name, fs.ErrNotExist)
	case node.dir && len(m.children(path.Clean(name))) > 0:
		return pathError("remove", name, errNotEmpty)
	}
	delete(m.nodes, path.Clean(name))
	return nil
}

func (m *memFS) RemoveAll(name string) error {
	if err := m.log.step("RemoveAll"); err != nil {
		return pathError("removeall", name, err)
	}
	m.removeTree(path.Clean(name))
	return nil
}

// memInfo is the fs.FileInfo of one memFS node.
type memInfo struct {
	name string
	node *memNode
}

func (i memInfo) Name() string { return i.name }
func (i memInfo) Size() int64  { return int64(len(i.node.data)) }
func (i memInfo) IsDir() bool  { return i.node.dir }
func (i memInfo) Sys() any     { return nil }
func (i memInfo) ModTime() time.Time {
	return time.Time{}
}

func (i memInfo) Mode() fs.FileMode {
	mode := i.node.perm
	if i.node.dir {
		mode |= fs.ModeDir
	}
	if i.node.link != "" {
		mode |= fs.ModeSymlink
	}
	return mode
}

// memDirEntry is the fs.DirEntry of one memFS node.
type memDirEntry struct{ info memInfo }

func (e memDirEntry) Name() string               { return e.info.name }
func (e memDirEntry) IsDir() bool                { return e.info.node.dir }
func (e memDirEntry) Type() fs.FileMode          { return e.info.Mode().Type() }
func (e memDirEntry) Info() (fs.FileInfo, error) { return e.info, nil }

// memFile is an open memFS file. Each write, chmod, sync, and close is a call.
type memFile struct {
	owner    *memFS
	name     string
	node     *memNode
	readOnly bool
}

var _ stagedFile = (*memFile)(nil)

func (f *memFile) Name() string { return f.name }

func (f *memFile) Write(p []byte) (int, error) {
	if err := f.owner.log.step("Write"); err != nil {
		return 0, err
	}
	f.node.data = append(f.node.data, p...)
	return len(p), nil
}

func (f *memFile) WriteString(s string) (int, error) { return f.Write([]byte(s)) }

func (f *memFile) Chmod(mode fs.FileMode) error {
	if err := f.owner.log.step("FileChmod"); err != nil {
		return err
	}
	f.node.perm = mode.Perm()
	return nil
}

func (f *memFile) Sync() error  { return f.owner.log.step("Sync") }
func (f *memFile) Close() error { return f.owner.log.step("Close") }

// fakeCommands is a recording commandRunner. respond answers each child with
// its standard output and status; lookPath answers each PATH search.
type fakeCommands struct {
	log      *callLog
	respond  func(command *hostCommand) ([]byte, error)
	lookPath func(file string) (string, error)
}

var _ commandRunner = (*fakeCommands)(nil)

// Command builds a child without searching the PATH, so a unit test reads no
// environment and no file.
func (f *fakeCommands) Command(name string, arguments ...string) *hostCommand {
	return &hostCommand{Path: name, Args: append([]string{name}, arguments...)}
}

// CommandContext is Command for a child a context may cancel.
func (f *fakeCommands) CommandContext(_ context.Context, name string, arguments ...string) *hostCommand {
	return f.Command(name, arguments...)
}

func (f *fakeCommands) answer(op string, command *hostCommand) ([]byte, error) {
	if err := f.log.step(op); err != nil {
		return nil, err
	}
	if f.respond == nil {
		return nil, nil
	}
	return f.respond(command)
}

func (f *fakeCommands) Output(command *hostCommand) ([]byte, error) {
	return f.answer("Output", command)
}

func (f *fakeCommands) Run(command *hostCommand) error {
	out, err := f.answer("Run", command)
	if command.Stdout != nil {
		_, _ = command.Stdout.Write(out)
	}
	return err
}

func (f *fakeCommands) Start(command *hostCommand) error {
	_, err := f.answer("Start", command)
	return err
}

func (f *fakeCommands) LookPath(file string) (string, error) {
	if err := f.log.step("LookPath"); err != nil {
		return "", err
	}
	if f.lookPath == nil {
		return "/usr/bin/" + file, nil
	}
	return f.lookPath(file)
}

// fakeEnv is a fixed process environment.
type fakeEnv map[string]string

var _ environment = fakeEnv(nil)

func (e fakeEnv) Getenv(key string) string { return e[key] }

func (e fakeEnv) LookupEnv(key string) (string, bool) {
	value, ok := e[key]
	return value, ok
}

// Getpid is a fixed process identifier, so staged names are stable under test.
func (e fakeEnv) Getpid() int { return 4242 }

func (e fakeEnv) Environ() []string {
	var out []string
	for key, value := range e {
		out = append(out, key+"="+value)
	}
	sort.Strings(out)
	return out
}

// execFailure is the status-carrying error a child that exited nonzero yields.
// Its status reads -1, because a double cannot make a process state.
func execFailure() error { return &hostExitError{} }

// useDoubles installs the doubles as the package seams until the test ends. A
// nil argument keeps the seam it already has.
func useDoubles(t *testing.T, files fileSystem, commands commandRunner, env environment) {
	t.Helper()
	prevFiles, prevCommands, prevEnv := hostFiles, hostCommands, hostEnv
	t.Cleanup(func() { hostFiles, hostCommands, hostEnv = prevFiles, prevCommands, prevEnv })
	if files != nil {
		hostFiles = files
	}
	if commands != nil {
		hostCommands = commands
	}
	if env != nil {
		hostEnv = env
	}
}

// faultSweep runs a scenario cleanly, then once per call the clean run made,
// failing exactly that call with errInjected and then with fs.ErrNotExist. The
// scenario installs its doubles on the log it is given and returns the operation
// to run; its clean run must succeed. For errInjected, the operation must return
// an error unless the failed call belongs to an operation named in tolerated.
// For fs.ErrNotExist the run only has to finish, since a missing path is a
// normal answer for some calls.
func faultSweep(t *testing.T, scenario func(t *testing.T, log *callLog) func() error, tolerated ...string) {
	t.Helper()
	clean := &callLog{}
	if err := scenario(t, clean)(); err != nil {
		t.Fatalf("the clean run failed: %v", err)
	}
	for k := 1; k <= len(clean.ops); k++ {
		log := &callLog{failAt: k, cause: errInjected}
		err := scenario(t, log)()
		if op := log.ops[k-1]; err == nil && !isTolerated(op, tolerated) {
			t.Errorf("call %d (%s) failed, but the operation returned no error", k, op)
		}
		missing := &callLog{failAt: k, cause: fs.ErrNotExist}
		_ = scenario(t, missing)()
	}
}

func isTolerated(op string, tolerated []string) bool {
	for _, name := range tolerated {
		if name == op {
			return true
		}
	}
	return false
}
