// Package runtime walks the repository read-only, never following links.
package runtime

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/wahidyankf/rhino/internal/application"
)

// TreeError aliases the consumer's read error so this adapter stays
// structurally compatible with the boundary it serves.
type TreeError = application.ReadError

// DiskTree reads the repository rooted at Dir.
type DiskTree struct {
	Dir string
}

// AtCurrentDirectory opens the working directory.
func AtCurrentDirectory() (*DiskTree, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("repository root could not be established: %w", err)
	}
	return &DiskTree{Dir: dir}, nil
}

// Root reports the root.
func (t *DiskTree) Root() string { return t.Dir }

// RootedAt re-roots at a directory. `--root` names the repository rather than a
// selection inside it, so any existing directory is accepted: one reached
// through `..`, one outside the working directory, or one named by an absolute
// path. A path that names nothing, or names a file, is refused.
func (t *DiskTree) RootedAt(path string) (application.Tree, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("`--root` names nothing")
	}
	candidate := path
	if !filepath.IsAbs(candidate) {
		// Not cleaned, unlike filepath.Join: a `..` below a working directory
		// reached through a link climbs from the link's target, as the operating
		// system resolves it, and a lexical fold would climb from the spelling.
		candidate = t.Dir + string(filepath.Separator) + path
	}
	info, err := os.Stat(candidate)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("`--root %s` names nothing usable", path)
	}
	// The resolved directory is the repository the run reads, so every later
	// path is relative to it, whatever link or spelling named it.
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return nil, fmt.Errorf("`--root %s` names nothing usable", path)
	}
	return &DiskTree{Dir: resolved}, nil
}

// Files lists repository-relative regular files, excluding symlinks.
func (t *DiskTree) Files() []string {
	var out []string
	_ = filepath.WalkDir(t.Dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(t.Dir, path)
		if err != nil {
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(out)
	return out
}

// Read returns one file as text. Links are never followed.
func (t *DiskTree) Read(path string) (string, *TreeError) {
	if leavesRoot(path) {
		return "", &TreeError{Kind: "escapes", Reason: "`" + path + "` escapes the repository root"}
	}
	// Every component is checked, the last included, before the file is
	// opened: the operating system would otherwise follow a directory link in
	// the middle of the path and read outside the repository.
	if t.PassesThroughLink(path) {
		return "", &TreeError{Kind: "escapes", Reason: "`" + path + "` passes through a filesystem link"}
	}
	full := filepath.Join(t.Dir, filepath.FromSlash(path))
	if _, err := os.Lstat(full); err != nil {
		if os.IsNotExist(err) {
			return "", &TreeError{Kind: "missing", Reason: path + ": no such file"}
		}
		// The path names something the repository cannot open: a blocked
		// parent, a denied component. Only an absent path is missing.
		return "", &TreeError{Kind: "unreadable", Reason: path + ": " + err.Error()}
	}
	data, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			return "", &TreeError{Kind: "missing", Reason: path + ": no such file"}
		}
		return "", &TreeError{Kind: "unreadable", Reason: path + ": " + err.Error()}
	}
	if !utf8.Valid(data) {
		return "", &TreeError{Kind: "not-text", Reason: path + ": holds no text"}
	}
	return string(data), nil
}

// Links lists every filesystem link under the root, with where its target
// lies. The walk finds links by their type and never follows one.
func (t *DiskTree) Links() []application.TreeLink {
	realRoot, err := filepath.EvalSymlinks(t.Dir)
	if err != nil {
		return nil
	}
	var out []application.TreeLink
	_ = filepath.WalkDir(t.Dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			rel, relErr := filepath.Rel(t.Dir, path)
			if relErr == nil {
				out = append(out, classifyLink(filepath.ToSlash(rel), path, realRoot))
			}
			return nil
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		return nil
	})
	sort.Slice(out, func(left, right int) bool { return out[left].Path < out[right].Path })
	return out
}

// classifyLink resolves one link's target and names where it lies: inside the
// root with its repository-relative path, outside it, or nowhere.
func classifyLink(rel, full, realRoot string) application.TreeLink {
	target, err := os.Readlink(full)
	if err != nil {
		return application.TreeLink{Path: rel, Kind: application.LinkUnresolved}
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(full), target)
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return application.TreeLink{Path: rel, Kind: application.LinkUnresolved}
	}
	inside, err := filepath.Rel(realRoot, resolved)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return application.TreeLink{Path: rel, Kind: application.LinkOutside}
	}
	if inside == "." {
		inside = ""
	}
	return application.TreeLink{Path: rel, Target: filepath.ToSlash(inside), Kind: application.LinkWithin}
}

// Children lists the direct children of a directory, derived from the file
// list: the files directly inside it and the subdirectories holding a file.
// A subdirectory holding no file is not a child.
func (t *DiskTree) Children(directory string) []string {
	return application.ChildrenOf(t.Files(), directory)
}

// IsDirectory reports whether path names a directory holding a file.
func (t *DiskTree) IsDirectory(path string) bool {
	return application.IsDirectoryOf(t.Files(), path)
}

// Exists reports whether path names a file the tree can read or a directory
// the walk lists. An unreadable file reads as absent here, the way the
// oracle's existence check answers from a read: a target that cannot be
// opened is missing, not refused, at the resolving site.
func (t *DiskTree) Exists(path string) bool {
	if _, readErr := t.Read(path); readErr == nil {
		return true
	}
	if leavesRoot(path) {
		return false
	}
	full := filepath.Join(t.Dir, filepath.FromSlash(path))
	info, err := os.Lstat(full)
	if err != nil || !info.IsDir() {
		return false
	}
	prefix := strings.TrimSuffix(path, "/") + "/"
	for _, file := range t.Files() {
		if strings.HasPrefix(file, prefix) {
			return true
		}
	}
	return false
}

// PassesThroughLink reports whether any component of path, the last
// included, is a filesystem link. Answered from the link itself, never by
// following it.
func (t *DiskTree) PassesThroughLink(path string) bool {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" || trimmed == "." {
		return false
	}
	current := t.Dir
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == "" || segment == "." {
			continue
		}
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if err != nil {
			return false
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return true
		}
	}
	return false
}

// IndexedFiles lists the staged paths from the Git index, never the mutable
// working tree. A tree with no Git snapshot capability must refuse rather
// than fall back to the file walk.
func (t *DiskTree) IndexedFiles() ([]string, error) {
	output, err := exec.Command("git", "-C", t.Dir, "diff", "--cached", "--name-only", "-z", "--").Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil, fmt.Errorf("Git refused the index query with status %d", exit.ExitCode())
		}
		return nil, fmt.Errorf("could not read the Git index: %s", err.Error())
	}
	if !utf8.Valid(output) {
		return nil, fmt.Errorf("the Git index contains a non-UTF-8 path")
	}
	var paths []string
	for _, raw := range strings.Split(string(output), "\x00") {
		if raw == "" {
			continue
		}
		if !isRepositoryPath(raw) {
			return nil, fmt.Errorf("the Git index returned an escaping path `%s`", raw)
		}
		paths = append(paths, raw)
	}
	sort.Strings(paths)
	out := paths[:0]
	for _, path := range paths {
		if len(out) == 0 || out[len(out)-1] != path {
			out = append(out, path)
		}
	}
	return out, nil
}

// IndexedRegularFiles selects regular-file destinations from the Git index
// snapshot. Raw Git metadata decides: destination modes 100644 and 100755
// are included, while symlinks, gitlinks, and deletions are excluded.
func (t *DiskTree) IndexedRegularFiles() ([]string, error) {
	output, err := exec.Command("git", "-C", t.Dir, "diff", "--cached", "--raw",
		"-z", "--no-renames", "--no-ext-diff", "--no-textconv", "--").Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil, fmt.Errorf("Git refused the regular-file index query with status %d", exit.ExitCode())
		}
		return nil, fmt.Errorf("could not read regular files from the Git index: %s", err.Error())
	}
	return regularFilePaths(output)
}

// ChangedFiles lists the repository-relative paths changed between two
// already validated immutable commits. A pull-request gate receives only
// that reviewed selection, never the complete checkout.
func (t *DiskTree) ChangedFiles(base, head string) ([]string, error) {
	output, err := exec.Command("git", "-C", t.Dir, "diff", "--name-only", "-z",
		"--no-renames", "--end-of-options", base+".."+head).Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("Git refused the declared changed-file range")
		}
		return nil, fmt.Errorf("could not read changed files for the declared range: %s", err.Error())
	}
	if !utf8.Valid(output) {
		return nil, fmt.Errorf("Git returned a non-UTF-8 changed path")
	}
	var paths []string
	for _, raw := range strings.Split(string(output), "\x00") {
		if raw == "" {
			continue
		}
		if !gitRepositoryPath(raw) {
			return nil, fmt.Errorf("Git returned an escaping changed path `%s`", raw)
		}
		paths = append(paths, raw)
	}
	sort.Strings(paths)
	out := paths[:0]
	for _, path := range paths {
		if len(out) == 0 || out[len(out)-1] != path {
			out = append(out, path)
		}
	}
	return out, nil
}

// ChangedRegularFiles selects regular-file destinations changed between two
// already validated immutable commits, by the same recorded-mode rule the
// index snapshot uses.
func (t *DiskTree) ChangedRegularFiles(base, head string) ([]string, error) {
	output, err := exec.Command("git", "-C", t.Dir, "diff", "--raw", "-z",
		"--no-renames", "--no-ext-diff", "--no-textconv",
		"--end-of-options", base+".."+head, "--").Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("Git refused the declared regular-file range")
		}
		return nil, fmt.Errorf("could not read regular files for the declared Git range: %s", err.Error())
	}
	return regularFilePaths(output)
}

// ResolveGitRef resolves a repository-declared comparison ref to one
// immutable commit, for a pre-push update that creates a new ref and
// supplies no remote base object.
func (t *DiskTree) ResolveGitRef(reference string) (string, error) {
	if reference == "" || strings.HasPrefix(reference, "-") || strings.ContainsFunc(reference, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f'
	}) {
		return "", fmt.Errorf("the declared fallback ref is invalid")
	}
	output, err := exec.Command("git", "-C", t.Dir, "rev-parse", "--verify",
		"--end-of-options", reference+"^{commit}").Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("the declared fallback ref does not resolve to a commit")
		}
		return "", fmt.Errorf("could not resolve the declared fallback ref: %s", err.Error())
	}
	if !utf8.Valid(output) {
		return "", fmt.Errorf("Git returned a non-UTF-8 fallback commit")
	}
	commit := strings.TrimSpace(string(output))
	for i := 0; i < len(commit); i++ {
		character := commit[i]
		if character >= '0' && character <= '9' || character >= 'a' && character <= 'f' || character >= 'A' && character <= 'F' {
			continue
		}
		return "", fmt.Errorf("Git returned an invalid fallback commit")
	}
	return commit, nil
}

// CommitMessages reads the non-merge commit-message text for one already
// validated immutable range. A pull-request gate consumes the same semantic
// message input as the commit-msg hook, from the reviewed range instead.
func (t *DiskTree) CommitMessages(base, head string) (string, error) {
	output, err := exec.Command("git", "-C", t.Dir, "log", "--format=%B",
		"--no-merges", "--end-of-options", base+".."+head).Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("Git refused the declared commit-message range")
		}
		return "", fmt.Errorf("could not read commit messages for the declared range: %s", err.Error())
	}
	if !utf8.Valid(output) {
		return "", fmt.Errorf("Git returned non-UTF-8 commit-message text")
	}
	return string(output), nil
}

// HookMessageFile reads the one message file Git assigned to the current
// commit-msg hook. The supplied path must be Git's canonical
// COMMIT_EDITMSG; any other path is refused, never followed elsewhere.
func (t *DiskTree) HookMessageFile(path string) (string, *TreeError) {
	output, err := exec.Command("git", "-C", t.Dir, "rev-parse",
		"--path-format=absolute", "--git-path", "COMMIT_EDITMSG").Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return "", &TreeError{Kind: "unreadable", Reason: "the Git hook message-file boundary is unavailable"}
		}
		return "", &TreeError{Kind: "unreadable", Reason: "the Git hook message-file boundary is unavailable: " + err.Error()}
	}
	if !utf8.Valid(output) {
		return "", &TreeError{Kind: "unreadable", Reason: "the Git hook message-file boundary returned a non-UTF-8 path"}
	}
	canonical := strings.TrimSpace(string(output))
	if !filepath.IsAbs(canonical) {
		return "", &TreeError{Kind: "unreadable", Reason: "the Git hook message-file boundary returned a relative path"}
	}
	supplied := path
	if !filepath.IsAbs(supplied) {
		supplied = filepath.Join(t.Dir, filepath.FromSlash(path))
	}
	if supplied != canonical {
		return "", &TreeError{Kind: "unreadable", Reason: "the supplied path is not Git's canonical COMMIT_EDITMSG"}
	}
	data, err := os.ReadFile(canonical)
	if err != nil {
		if os.IsNotExist(err) {
			return "", &TreeError{Kind: "missing", Reason: path + ": no such file"}
		}
		return "", &TreeError{Kind: "unreadable", Reason: path + ": " + err.Error()}
	}
	if !utf8.Valid(data) {
		return "", &TreeError{Kind: "not-text", Reason: path + ": holds no text"}
	}
	return string(data), nil
}

// gitRepositoryPath reports whether a Git-returned path stays inside the
// repository: non-empty, never absolute, and no empty, current, or parent
// segment. A record Git cannot place inside the root is refused rather than
// projected into a child.
func gitRepositoryPath(path string) bool {
	if path == "" || strings.HasPrefix(path, "/") {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

// DiskEnvironmentStore is the only write boundary environment init, backup,
// and restore hold. It stages every byte before installing, and preserves an
// overwritten target until its replacement is durable.
type DiskEnvironmentStore struct{}

// Create installs declared targets that do not exist yet.
func (DiskEnvironmentStore) Create(root string, files []application.EnvironmentFile) error {
	seen := make(map[string]bool, len(files))
	contents := make(map[string]string, len(files))
	for _, file := range files {
		if seen[file.Path] {
			return fmt.Errorf("duplicate environment target `%s`", file.Path)
		}
		seen[file.Path] = true
		contents[file.Path] = file.Contents
	}
	return installEnvironmentFiles(root, contents)
}

// Restore installs declared targets, replacing existing ones only when each
// file was planned with replacement authorized.
func (DiskEnvironmentStore) Restore(root string, files []application.EnvironmentRestoreFile) error {
	desired := make(map[string]restoreDesired, len(files))
	for _, file := range files {
		if _, ok := desired[file.Path]; ok {
			return fmt.Errorf("duplicate environment target `%s`", file.Path)
		}
		desired[file.Path] = restoreDesired{contents: file.Contents, replace: file.Replace}
	}
	return restoreEnvironmentFiles(root, desired)
}

// restoreDesired is one staged restore: the backup bytes and whether an
// existing target may be replaced by them.
type restoreDesired struct {
	contents string
	replace  bool
}

func environmentRoot(root string) (string, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return "", fmt.Errorf("cannot inspect environment root: %s", err.Error())
	}
	if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
		return "", fmt.Errorf("the repository root is not a non-link directory")
	}
	return root, nil
}

func installEnvironmentFiles(root string, files map[string]string) error {
	if _, err := environmentRoot(root); err != nil {
		return err
	}
	ordered := make([]string, 0, len(files))
	for path := range files {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	for _, path := range ordered {
		if !isNormalPath(path) {
			return fmt.Errorf("invalid environment target `%s`", path)
		}
	}
	stage, err := os.MkdirTemp(root, ".rhino-environment-transaction-*")
	if err != nil {
		return fmt.Errorf("cannot stage environment transaction: %s", err.Error())
	}
	defer os.RemoveAll(stage)
	for _, path := range ordered {
		if err := stageEnvironmentFile(stage, path, files[path]); err != nil {
			return err
		}
	}
	var installed []string
	for _, path := range ordered {
		target := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			removeEnvironmentFiles(root, installed)
			return fmt.Errorf("cannot create environment parent `%s`: %s", path, err.Error())
		}
		if err := ensureEnvironmentPathIsSafe(root, path); err != nil {
			removeEnvironmentFiles(root, installed)
			return err
		}
		if _, err := os.Lstat(target); err == nil {
			removeEnvironmentFiles(root, installed)
			return fmt.Errorf("environment target `%s` already exists", path)
		}
		if err := os.Rename(filepath.Join(stage, filepath.FromSlash(path)), target); err != nil {
			removeEnvironmentFiles(root, installed)
			return fmt.Errorf("cannot install environment target `%s`: %s", path, err.Error())
		}
		installed = append(installed, path)
	}
	return nil
}

func restoreEnvironmentFiles(root string, files map[string]restoreDesired) error {
	if _, err := environmentRoot(root); err != nil {
		return err
	}
	ordered := make([]string, 0, len(files))
	for path := range files {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	for _, path := range ordered {
		if !isNormalPath(path) {
			return fmt.Errorf("invalid environment target `%s`", path)
		}
	}
	stage, err := os.MkdirTemp(root, ".rhino-environment-transaction-*")
	if err != nil {
		return fmt.Errorf("cannot stage environment transaction: %s", err.Error())
	}
	defer os.RemoveAll(stage)
	desired := filepath.Join(stage, "desired")
	if err := os.Mkdir(desired, 0o755); err != nil {
		return fmt.Errorf("cannot stage environment restore: %s", err.Error())
	}
	for _, path := range ordered {
		if err := stageEnvironmentFile(desired, path, files[path].contents); err != nil {
			return err
		}
	}
	backups := filepath.Join(stage, "backups")
	if err := os.Mkdir(backups, 0o755); err != nil {
		return fmt.Errorf("cannot prepare environment rollback: %s", err.Error())
	}
	type installed struct {
		target string
		backup string
	}
	var done []installed
	rollback := func() {
		for i := len(done) - 1; i >= 0; i-- {
			os.Remove(done[i].target)
			if done[i].backup != "" {
				os.Rename(done[i].backup, done[i].target)
			}
		}
	}
	for index, path := range ordered {
		target := filepath.Join(root, filepath.FromSlash(path))
		parent := filepath.Dir(target)
		if err := os.MkdirAll(parent, 0o755); err != nil {
			rollback()
			return fmt.Errorf("cannot create environment parent `%s`: %s", path, err.Error())
		}
		if err := ensureEnvironmentPathIsSafe(root, path); err != nil {
			rollback()
			return err
		}
		backup := ""
		if _, err := os.Lstat(target); err == nil {
			info, err := os.Lstat(target)
			if err == nil && (info.Mode()&fs.ModeSymlink != 0 || info.IsDir()) {
				rollback()
				return fmt.Errorf("environment target `%s` is not a regular non-link file", path)
			}
			if !files[path].replace {
				rollback()
				return fmt.Errorf("environment target `%s` already exists", path)
			}
			backup = filepath.Join(backups, fmt.Sprintf("%d", index))
			if err := os.Rename(target, backup); err != nil {
				rollback()
				return fmt.Errorf("cannot preserve environment target `%s`: %s", path, err.Error())
			}
		}
		if err := os.Rename(filepath.Join(desired, filepath.FromSlash(path)), target); err != nil {
			if backup != "" {
				os.Rename(backup, target)
			}
			rollback()
			return fmt.Errorf("cannot restore environment target `%s`: %s", path, err.Error())
		}
		done = append(done, installed{target: target, backup: backup})
	}
	return nil
}

func stageEnvironmentFile(stage, path, contents string) error {
	staged := filepath.Join(stage, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(staged), 0o755); err != nil {
		return fmt.Errorf("cannot stage environment target `%s`: %s", path, err.Error())
	}
	file, err := os.OpenFile(staged, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("cannot create environment stage `%s`: %s", path, err.Error())
	}
	if _, err := file.WriteString(contents); err != nil {
		file.Close()
		return fmt.Errorf("cannot write environment stage `%s`: %s", path, err.Error())
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("cannot sync environment stage `%s`: %s", path, err.Error())
	}
	return file.Close()
}

func removeEnvironmentFiles(root string, installed []string) {
	for i := len(installed) - 1; i >= 0; i-- {
		os.Remove(filepath.Join(root, filepath.FromSlash(installed[i])))
	}
}

// ensureEnvironmentPathIsSafe refuses a target crossing a filesystem link,
// checked component by component from the link itself, never by following it.
func ensureEnvironmentPathIsSafe(root, relative string) error {
	current := root
	for _, segment := range strings.Split(relative, "/") {
		current = filepath.Join(current, segment)
		if info, err := os.Lstat(current); err == nil && info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("environment target `%s` crosses a symbolic link", relative)
		}
	}
	return nil
}

func isNormalPath(path string) bool {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return false
	}
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == "" || segment == "." || segment == ".." || strings.ContainsRune(segment, '\\') {
			return false
		}
	}
	return true
}

func isRepositoryPath(path string) bool {
	if strings.HasPrefix(path, "/") || strings.TrimSpace(path) == "" {
		return false
	}
	depth := 0
	for _, segment := range strings.Split(path, "/") {
		switch segment {
		case "..":
			depth--
		case ".", "":
		default:
			depth++
		}
		if depth < 0 {
			return false
		}
	}
	return true
}

func leavesRoot(path string) bool {
	if strings.HasPrefix(path, "/") || strings.TrimSpace(path) == "" {
		return true
	}
	depth := 0
	for _, segment := range strings.Split(path, "/") {
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
