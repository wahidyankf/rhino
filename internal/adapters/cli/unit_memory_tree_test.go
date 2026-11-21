package cli

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/wahidyankf/rhino/internal/application"
)

// The in-memory repository the unit corpus runs against. It implements the
// application's Tree and the optional Git and mode capabilities the application
// asks for, so the CLI reads it exactly as it reads a disk checkout. Nothing here
// touches the operating system: every path lives in one map keyed by absolute
// path, and the Git state is a declared snapshot of commits and the index.

// memKind is the kind of one in-memory file-system entry.
type memKind int

const (
	memFile memKind = iota
	memDir
	memLink
)

// memEntry is one entry of the in-memory file system. A link keeps its target as
// the scenario wrote it, absolute or relative to the link's own directory.
type memEntry struct {
	kind       memKind
	content    string
	perm       fs.FileMode
	target     string
	unreadable bool
}

// memFS is the scenario's whole file system, keyed by clean absolute path. The
// repository, the directory outside it, and every other path share one namespace,
// so a link from the repository to outside it is an ordinary link.
type memFS struct {
	entries map[string]*memEntry
}

func newMemFS() *memFS {
	return &memFS{entries: map[string]*memEntry{}}
}

// mkdirAll creates a directory and every missing ancestor.
func (m *memFS) mkdirAll(p string) {
	p = path.Clean(p)
	var chain []string
	for current := p; current != "/" && current != "."; current = path.Dir(current) {
		chain = append(chain, current)
	}
	for i := len(chain) - 1; i >= 0; i-- {
		if _, ok := m.entries[chain[i]]; !ok {
			m.entries[chain[i]] = &memEntry{kind: memDir, perm: 0o755}
		}
	}
}

// writeFile stores a regular file, creating its parent directories.
func (m *memFS) writeFile(p, content string, perm fs.FileMode) {
	p = path.Clean(p)
	m.mkdirAll(path.Dir(p))
	m.entries[p] = &memEntry{kind: memFile, content: content, perm: perm}
}

// symlink stores a link at p to target, creating its parent directories.
func (m *memFS) symlink(p, target string) {
	p = path.Clean(p)
	m.mkdirAll(path.Dir(p))
	m.entries[p] = &memEntry{kind: memLink, target: target, perm: 0o777}
}

// removeAll deletes p and everything beneath it.
func (m *memFS) removeAll(p string) {
	p = path.Clean(p)
	for key := range m.entries {
		if key == p || strings.HasPrefix(key, p+"/") {
			delete(m.entries, key)
		}
	}
}

// lstat returns the entry at p without following a link.
func (m *memFS) lstat(p string) (*memEntry, bool) {
	entry, ok := m.entries[path.Clean(p)]
	return entry, ok
}

// resolve follows every link on the way to p, the last component included, the
// way filepath.EvalSymlinks does. It fails when a component is absent or the
// links loop.
func (m *memFS) resolve(p string) (string, bool) {
	return m.resolveHops(path.Clean(p), 0)
}

// maxLinkHops bounds link following, so a loop fails instead of recursing.
const maxLinkHops = 40

func (m *memFS) resolveHops(p string, hops int) (string, bool) {
	if hops > maxLinkHops {
		return "", false
	}
	parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
	current := "/"
	for i, part := range parts {
		if part == "" || part == "." {
			continue
		}
		next := path.Join(current, part)
		entry, ok := m.entries[next]
		if !ok {
			return "", false
		}
		if entry.kind == memLink {
			target := entry.target
			if !path.IsAbs(target) {
				target = path.Join(current, target)
			}
			rest := strings.Join(parts[i+1:], "/")
			return m.resolveHops(path.Join(target, rest), hops+1)
		}
		current = next
	}
	return current, true
}

// memBlob is one index or tree entry: its content and its Git mode.
type memBlob struct {
	content string
	mode    string
}

// memGit is the Git state of the repository: the commits the scenario declared,
// the index, and the names of the base and head commits. The base commit is the
// main branch, which HEAD names; the head commit is the explicit range's result.
type memGit struct {
	fixtures map[string]memBlob
	ranged   map[string]memBlob
	index    map[string]memBlob
	commits  map[string]map[string]memBlob
	base     string
	head     string
	counter  int
}

// newMemGit starts a repository whose only commit is empty, with HEAD at it.
func newMemGit() *memGit {
	g := &memGit{
		fixtures: map[string]memBlob{},
		ranged:   map[string]memBlob{},
		index:    map[string]memBlob{},
		commits:  map[string]map[string]memBlob{},
	}
	g.base = g.commit(map[string]memBlob{})
	g.head = g.base
	return g
}

// commit records a commit of tree and returns its name.
func (g *memGit) commit(tree map[string]memBlob) string {
	g.counter++
	name := fmt.Sprintf("%040x", g.counter)
	g.commits[name] = tree
	return name
}

// commitRange rebuilds the head from the fixtures and the explicit range changes.
// The fixtures win where both name a path, as the replay index does.
func (g *memGit) commitRange() {
	tree := map[string]memBlob{}
	for p, blob := range g.ranged {
		tree[p] = blob
	}
	for p, blob := range g.fixtures {
		tree[p] = blob
	}
	g.head = g.commit(tree)
}

// setIndex stages a regular file with content into the index.
func (g *memGit) setIndex(p, content string) {
	g.index[path.Clean(p)] = memBlob{content: content, mode: "100644"}
}

// resetIndex empties the index, so it selects no paths.
func (g *memGit) resetIndex() {
	g.index = map[string]memBlob{}
}

// setRange adds a regular file with content to the explicit range and commits it.
func (g *memGit) setRange(p, content string) {
	g.ranged[path.Clean(p)] = memBlob{content: content, mode: "100644"}
	g.commitRange()
}

// trackExecutable commits an executable fixture into the base and the range head.
// The index holds it only when staged is true.
func (g *memGit) trackExecutable(p, content string, staged bool) {
	p = path.Clean(p)
	g.fixtures[p] = memBlob{content: content, mode: "100755"}
	previous := g.base
	base := map[string]memBlob{}
	for fixture, blob := range g.fixtures {
		base[fixture] = blob
	}
	g.base = g.commit(base)
	if g.head == previous {
		g.head = g.base
	} else {
		g.commitRange()
	}
	if staged {
		g.index[p] = memBlob{content: content, mode: "100755"}
	}
}

// indexAgainstBase lists the paths whose index entry differs from HEAD, deleted
// paths included, which is what `git diff --cached` names.
func (g *memGit) indexAgainstBase() map[string]bool {
	head := g.commits[g.base]
	changed := map[string]bool{}
	for p, blob := range g.index {
		if old, ok := head[p]; !ok || old != blob {
			changed[p] = true
		}
	}
	for p := range head {
		if _, ok := g.index[p]; !ok {
			changed[p] = true
		}
	}
	return changed
}

// diffTrees lists the paths that differ between two commit trees. A deleted path
// is named only when withDeletions is true.
func (g *memGit) diffTrees(base, head string, withDeletions bool) ([]string, error) {
	before, okBefore := g.commits[base]
	after, okAfter := g.commits[head]
	if !okBefore || !okAfter {
		return nil, fmt.Errorf("git refused the declared changed-file range")
	}
	var out []string
	for p, blob := range after {
		if old, ok := before[p]; !ok || old != blob {
			out = append(out, p)
		}
	}
	if withDeletions {
		for p := range before {
			if _, ok := after[p]; !ok {
				out = append(out, p)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// sortedKeys returns the keys of a set, sorted.
func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// memTree is a repository rooted at dir. A tree that is not the repository root
// holds no Git state, so every Git query refuses there, as Git does outside a
// repository.
type memTree struct {
	files *memFS
	dir   string
	git   *memGit
}

// newMemTree opens the root of the in-memory repository.
func newMemTree(files *memFS, dir string, git *memGit) *memTree {
	return &memTree{files: files, dir: path.Clean(dir), git: git}
}

// inside returns the repository-relative path of an absolute path under the root,
// and reports whether every directory between the root and it is a real directory
// (a walk never descends through a link) and no component is Git's own.
func (t *memTree) inside(p string) (string, bool) {
	if !strings.HasPrefix(p, t.dir+"/") {
		return "", false
	}
	rel := strings.TrimPrefix(p, t.dir+"/")
	segments := strings.Split(rel, "/")
	for _, segment := range segments {
		if segment == ".git" {
			return "", false
		}
	}
	for ancestor := path.Dir(p); ancestor != t.dir; ancestor = path.Dir(ancestor) {
		entry, ok := t.files.lstat(ancestor)
		if !ok || entry.kind != memDir {
			return "", false
		}
	}
	return rel, true
}

// Root reports the repository root.
func (t *memTree) Root() string { return t.dir }

// Files lists the repository-relative regular files, excluding links.
func (t *memTree) Files() []string {
	var out []string
	for p, entry := range t.files.entries {
		if entry.kind != memFile {
			continue
		}
		if rel, ok := t.inside(p); ok {
			out = append(out, rel)
		}
	}
	sort.Strings(out)
	return out
}

// RootedAt re-roots at a directory the tree can reach, following links as the
// operating system does. A path that names nothing, or names a file, is refused.
func (t *memTree) RootedAt(p string) (application.Tree, error) {
	if strings.TrimSpace(p) == "" {
		return nil, fmt.Errorf("`--root` names nothing")
	}
	candidate := p
	if !path.IsAbs(candidate) {
		candidate = t.dir + "/" + p
	}
	candidate = path.Clean(candidate)
	resolved, ok := t.files.resolve(candidate)
	if !ok {
		return nil, fmt.Errorf("`--root %s` names nothing usable", p)
	}
	entry, ok := t.files.lstat(resolved)
	if !ok || entry.kind != memDir {
		return nil, fmt.Errorf("`--root %s` names nothing usable", p)
	}
	var git *memGit
	if resolved == t.dir {
		git = t.git
	}
	return newMemTree(t.files, resolved, git), nil
}

// Read returns one file as text. Links are never followed.
func (t *memTree) Read(p string) (string, *application.ReadError) {
	if memLeavesRoot(p) {
		return "", &application.ReadError{Kind: "escapes", Reason: "`" + p + "` escapes the repository root"}
	}
	if t.PassesThroughLink(p) {
		return "", &application.ReadError{Kind: "escapes", Reason: "`" + p + "` passes through a filesystem link"}
	}
	full := path.Join(t.dir, p)
	entry, ok := t.files.lstat(full)
	if !ok {
		return "", &application.ReadError{Kind: "missing", Reason: p + ": no such file"}
	}
	if entry.kind == memDir {
		return "", &application.ReadError{Kind: "unreadable", Reason: p + ": is a directory"}
	}
	if entry.unreadable {
		return "", &application.ReadError{Kind: "unreadable", Reason: p + ": open " + full + ": permission denied"}
	}
	if !utf8.ValidString(entry.content) {
		return "", &application.ReadError{Kind: "not-text", Reason: p + ": holds no text"}
	}
	return entry.content, nil
}

// FileMode returns the permission bits of the entry a path names, refusing a
// path that passes through a link.
func (t *memTree) FileMode(p string) (uint32, *application.ReadError) {
	if memLeavesRoot(p) || t.PassesThroughLink(p) {
		return 0, &application.ReadError{Kind: "escapes", Reason: "`" + p + "` passes through a filesystem link"}
	}
	entry, ok := t.files.lstat(path.Join(t.dir, p))
	if !ok {
		return 0, &application.ReadError{Kind: "missing", Reason: p + ": no such file"}
	}
	return uint32(entry.perm.Perm()), nil
}

// Links lists every link under the root with where its target lies.
func (t *memTree) Links() []application.TreeLink {
	realRoot, ok := t.files.resolve(t.dir)
	if !ok {
		return nil
	}
	var out []application.TreeLink
	for p, entry := range t.files.entries {
		if entry.kind != memLink {
			continue
		}
		if rel, ok := t.inside(p); ok {
			out = append(out, t.classifyLink(rel, p, realRoot))
		}
	}
	sort.Slice(out, func(left, right int) bool { return out[left].Path < out[right].Path })
	return out
}

// classifyLink resolves one link's target and names where it lies.
func (t *memTree) classifyLink(rel, full, realRoot string) application.TreeLink {
	target := t.files.entries[full].target
	if !path.IsAbs(target) {
		target = path.Join(path.Dir(full), target)
	}
	resolved, ok := t.files.resolve(target)
	if !ok {
		return application.TreeLink{Path: rel, Kind: application.LinkUnresolved}
	}
	if !strings.HasPrefix(resolved, realRoot+"/") && resolved != realRoot {
		return application.TreeLink{Path: rel, Kind: application.LinkOutside}
	}
	inside := strings.TrimPrefix(strings.TrimPrefix(resolved, realRoot), "/")
	return application.TreeLink{Path: rel, Target: inside, Kind: application.LinkWithin}
}

// Children lists the direct children of a directory, derived from the file list.
func (t *memTree) Children(directory string) []string {
	return application.ChildrenOf(t.Files(), directory)
}

// IsDirectory reports whether path names a directory holding a file.
func (t *memTree) IsDirectory(p string) bool {
	return application.IsDirectoryOf(t.Files(), p)
}

// Exists reports whether path names a readable file or a directory the walk lists.
func (t *memTree) Exists(p string) bool {
	if _, readErr := t.Read(p); readErr == nil {
		return true
	}
	if memLeavesRoot(p) {
		return false
	}
	entry, ok := t.files.lstat(path.Join(t.dir, p))
	if !ok || entry.kind != memDir {
		return false
	}
	prefix := strings.TrimSuffix(p, "/") + "/"
	for _, file := range t.Files() {
		if strings.HasPrefix(file, prefix) {
			return true
		}
	}
	return false
}

// PassesThroughLink reports whether any component of path, the last included, is
// a link. The answer comes from the link itself, never by following it.
func (t *memTree) PassesThroughLink(p string) bool {
	trimmed := strings.Trim(p, "/")
	if trimmed == "" || trimmed == "." {
		return false
	}
	current := t.dir
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == "" || segment == "." {
			continue
		}
		current = path.Join(current, segment)
		entry, ok := t.files.lstat(current)
		if !ok {
			return false
		}
		if entry.kind == memLink {
			return true
		}
	}
	return false
}

// gitRefused is the failure a Git query reports outside a repository or for an
// unknown commit, the way the disk tree reports a refused Git invocation.
func (t *memTree) gitRefused(what string) error {
	return fmt.Errorf("git refused the %s query with status 128", what)
}

// IndexedFiles lists the paths the index changes against HEAD.
func (t *memTree) IndexedFiles() ([]string, error) {
	if t.git == nil {
		return nil, t.gitRefused("index")
	}
	return sortedKeys(t.git.indexAgainstBase()), nil
}

// TrackedFiles lists every path the index tracks.
func (t *memTree) TrackedFiles() ([]string, error) {
	if t.git == nil {
		return nil, t.gitRefused("tracked")
	}
	tracked := map[string]bool{}
	for p := range t.git.index {
		tracked[p] = true
	}
	return sortedKeys(tracked), nil
}

// IgnoredFiles lists the ignored paths. The unit repository declares no ignore rules.
func (t *memTree) IgnoredFiles() ([]string, error) {
	if t.git == nil {
		return nil, t.gitRefused("ignored")
	}
	return nil, nil
}

// IndexedRegularFiles lists the index entries that are new or modified against HEAD.
func (t *memTree) IndexedRegularFiles() ([]string, error) {
	if t.git == nil {
		return nil, t.gitRefused("regular-file index")
	}
	head := t.git.commits[t.git.base]
	var out []string
	for p, blob := range t.git.index {
		if old, ok := head[p]; !ok || old != blob {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out, nil
}

// ChangedFiles lists the paths changed between two declared commits.
func (t *memTree) ChangedFiles(base, head string) ([]string, error) {
	if t.git == nil {
		return nil, fmt.Errorf("git refused the declared changed-file range")
	}
	return t.git.diffTrees(base, head, true)
}

// ChangedRegularFiles lists the regular files added or modified between two commits.
func (t *memTree) ChangedRegularFiles(base, head string) ([]string, error) {
	if t.git == nil {
		return nil, fmt.Errorf("git refused the declared regular-file range")
	}
	return t.git.diffTrees(base, head, false)
}

// ResolveGitRef resolves a declared comparison ref to a commit the scenario made.
func (t *memTree) ResolveGitRef(reference string) (string, error) {
	if t.git == nil {
		return "", fmt.Errorf("the declared fallback ref does not resolve to a commit")
	}
	if _, ok := t.git.commits[reference]; !ok {
		return "", fmt.Errorf("the declared fallback ref does not resolve to a commit")
	}
	return reference, nil
}

// CommitMessages reads the commit messages of a declared range: none when the
// range is empty, and the fixture message for each commit the head adds.
func (t *memTree) CommitMessages(base, head string) (string, error) {
	if t.git == nil {
		return "", fmt.Errorf("git refused the declared commit-message range")
	}
	if _, ok := t.git.commits[base]; !ok {
		return "", fmt.Errorf("git refused the declared commit-message range")
	}
	if _, ok := t.git.commits[head]; !ok {
		return "", fmt.Errorf("git refused the declared commit-message range")
	}
	if base == head {
		return "", nil
	}
	return "rhino bdd fixture\n\n", nil
}

// HookMessageFile reads the commit message Git keeps for the current commit-msg
// hook. Only Git's canonical file is read, whatever path the caller supplies.
func (t *memTree) HookMessageFile(p string) (string, *application.ReadError) {
	if t.git == nil {
		return "", &application.ReadError{Kind: "unreadable", Reason: "the Git hook message-file boundary is unavailable"}
	}
	canonical := path.Join(t.dir, ".git", "COMMIT_EDITMSG")
	supplied := p
	if !path.IsAbs(supplied) {
		supplied = path.Join(t.dir, p)
	}
	if supplied != canonical {
		return "", &application.ReadError{Kind: "unreadable", Reason: "the supplied path is not Git's canonical COMMIT_EDITMSG"}
	}
	entry, ok := t.files.lstat(canonical)
	if !ok {
		return "", &application.ReadError{Kind: "missing", Reason: p + ": no such file"}
	}
	if !utf8.ValidString(entry.content) {
		return "", &application.ReadError{Kind: "not-text", Reason: p + ": holds no text"}
	}
	return entry.content, nil
}

// memLeavesRoot reports whether a repository path is absolute, empty, or climbs
// out of the root by its own segments.
func memLeavesRoot(p string) bool {
	if strings.HasPrefix(p, "/") || strings.TrimSpace(p) == "" {
		return true
	}
	depth := 0
	for _, segment := range strings.Split(p, "/") {
		switch segment {
		case "..":
			depth--
		case ".", "":
		default:
			depth++
		}
		if depth < 0 {
			return true
		}
	}
	return false
}
