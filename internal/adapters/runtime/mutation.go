// Mutation runner: the only process and write boundary a `type: mutation`
// gate reaches. It owns the snapshot, the declared mutator's working
// directory, every Git write, and the cleanup of both. Dispatch in the
// application layer decides what to run and how to report it; this adapter
// decides what may change.
//
// Local `apply-index` never lets the mutator touch the working tree. It
// checks the Git index out into a temporary snapshot, runs the mutator there,
// and copies back only the selected paths it changed. A path whose working
// tree held unstaged edits keeps them and is reported as a divergence.
//
// Pull-request `verify-clean` replays the candidate revision in a detached,
// disposable worktree and reports what the replay changed. The ordinary
// checkout is never written.
package runtime

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wahidyankf/rhino/internal/application"
)

// DiskMutationRunner is the process and write boundary for mutation gates.
type DiskMutationRunner struct{}

// The two reasons a snapshot path is a refusal before anything runs. They are
// the oracle's wordings, which a caller reads to learn which boundary it lacks.
const (
	errNoIndexBoundary   = "this entry point has no index mutation boundary"
	errNoReplayBoundary  = "a disposable pull-request replay requires an explicit candidate revision"
	errSelectedSymlink   = "selected index path is a symbolic link `%s`"
	errSelectedEscape    = "selected index path escapes its root `%s`"
	errOutsideSelection  = "mutator changed `%s` outside its selected index paths"
	errNoDeclaredCommand = "the gate declares no command"
)

// fileDigest is the content fingerprint a before-and-after comparison uses.
type fileDigest [sha256.Size]byte

// ApplyIndex runs the declared mutator against a snapshot of the Git index
// and writes back only the selected paths it changed.
func (DiskMutationRunner) ApplyIndex(launch application.MutationLaunch) (application.MutationOutcome, error) {
	root := launch.Directory
	if _, err := gitRun(root, "rev-parse", "--is-inside-work-tree"); err != nil {
		return application.MutationOutcome{}, errors.New(errNoIndexBoundary)
	}
	if len(launch.SelectedPaths) == 0 {
		return application.MutationOutcome{}, nil
	}
	for _, path := range launch.SelectedPaths {
		if !gitRepositoryPath(path) {
			return application.MutationOutcome{}, fmt.Errorf(errSelectedEscape, path)
		}
	}

	snapshot, err := os.MkdirTemp("", "rhino-index-")
	if err != nil {
		return application.MutationOutcome{}, fmt.Errorf("could not create index snapshot: %s", err.Error())
	}
	defer os.RemoveAll(snapshot)
	if _, err := gitRun(root, "checkout-index", "--prefix="+snapshot+string(filepath.Separator), "-a"); err != nil {
		return application.MutationOutcome{}, err
	}
	if err := ensureSelectedNotLinked(snapshot, launch.SelectedPaths); err != nil {
		return application.MutationOutcome{}, err
	}
	before, err := snapshotDigests(snapshot)
	if err != nil {
		return application.MutationOutcome{}, err
	}
	code, err := runMutator(snapshot, launch)
	if err != nil {
		return application.MutationOutcome{}, err
	}
	if code != 0 {
		return application.MutationOutcome{Code: code}, nil
	}
	after, err := snapshotDigests(snapshot)
	if err != nil {
		return application.MutationOutcome{}, err
	}
	changed := changedDigests(before, after)
	for _, path := range changed {
		if !containsPath(launch.SelectedPaths, path) {
			return application.MutationOutcome{}, fmt.Errorf(errOutsideSelection, path)
		}
	}

	var divergences []string
	for _, path := range changed {
		if _, present := after[path]; present {
			if err := updateIndexBlob(root, filepath.Join(snapshot, filepath.FromSlash(path)), path); err != nil {
				return application.MutationOutcome{}, err
			}
			if digestMatches(before, path, root) {
				if err := replaceWorkingFile(root, snapshot, path); err != nil {
					return application.MutationOutcome{}, err
				}
			} else {
				divergences = append(divergences, path)
			}
			continue
		}
		// --force-remove, not --remove: the working-tree file still exists
		// until the copy-back below, and --remove leaves a present file in the
		// index. The index must drop the path whatever the working tree holds.
		if _, err := gitRun(root, "update-index", "--force-remove", "--", path); err != nil {
			return application.MutationOutcome{}, err
		}
		if digestMatches(before, path, root) {
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(path))); err != nil {
				return application.MutationOutcome{}, fmt.Errorf("could not synchronize deleted `%s`: %s", path, err.Error())
			}
		} else {
			divergences = append(divergences, path)
		}
	}
	return application.MutationOutcome{Code: 0, Changes: changed, Divergences: divergences}, nil
}

// VerifyClean replays the candidate revision in a disposable worktree, runs the
// mutator there, and reports every path the replay changed. A clean replay
// exits 0; any change makes it a finding.
func (DiskMutationRunner) VerifyClean(launch application.MutationLaunch) (application.MutationOutcome, error) {
	if launch.Revision == "" {
		return application.MutationOutcome{}, errors.New(errNoReplayBoundary)
	}
	root := launch.Directory
	parent, err := os.MkdirTemp("", "rhino-pull-request-")
	if err != nil {
		return application.MutationOutcome{}, fmt.Errorf("could not create pull-request snapshot: %s", err.Error())
	}
	defer os.RemoveAll(parent)
	checkout := filepath.Join(parent, "checkout")
	if _, err := gitRun(root, "worktree", "add", "--detach", "--force", checkout, launch.Revision); err != nil {
		return application.MutationOutcome{}, err
	}
	outcome, runErr := replayInCheckout(checkout, launch)
	_, cleanupErr := gitRun(root, "worktree", "remove", "--force", checkout)
	switch {
	case cleanupErr != nil:
		return application.MutationOutcome{}, cleanupErr
	case runErr != nil:
		return application.MutationOutcome{}, runErr
	}
	return outcome, nil
}

// replayInCheckout runs the mutator in the candidate checkout and collects the
// paths it changed, tracked or new.
func replayInCheckout(checkout string, launch application.MutationLaunch) (application.MutationOutcome, error) {
	code, err := runMutator(checkout, launch)
	if err != nil {
		return application.MutationOutcome{}, err
	}
	if code != 0 {
		return application.MutationOutcome{Code: code}, nil
	}
	tracked, err := gitPaths(checkout, "diff", "--no-ext-diff", "--no-renames", "--name-only", "-z", "HEAD")
	if err != nil {
		return application.MutationOutcome{}, err
	}
	created, err := gitPaths(checkout, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return application.MutationOutcome{}, err
	}
	changes := mergeSorted(tracked, created)
	code = 0
	if len(changes) > 0 {
		code = 1
	}
	return application.MutationOutcome{Code: code, Changes: changes}, nil
}

// runMutator starts the declared command from dir with the projected
// environment. Its streams stay inside the boundary, stdin is closed, and a
// relative PATH segment keeps the meaning it had for the original repository.
func runMutator(dir string, launch application.MutationLaunch) (int, error) {
	if len(launch.Argv) == 0 {
		return 0, errors.New(errNoDeclaredCommand)
	}
	program, arguments := launch.Argv[0], launch.Argv[1:]
	pathValue, normalized, err := mutatorPath(launch.Directory, launch.Environment)
	if err != nil {
		return 0, err
	}
	resolved, err := resolveProgram(program, dir, pathValue, normalized)
	if err != nil {
		return 0, err
	}
	command := &exec.Cmd{
		Path: resolved,
		Args: append([]string{program}, arguments...),
		Dir:  dir,
		Env:  mutatorEnvironment(launch.Surface, launch.Environment, pathValue, normalized),
	}
	if err := command.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			if exit.ExitCode() == -1 {
				return 0, fmt.Errorf("`%s` was ended by a signal", program)
			}
			return exit.ExitCode(), nil
		}
		return 0, fmt.Errorf("`%s` could not be started: %s", program, err.Error())
	}
	return 0, nil
}

// mutatorEnvironment is the inherited environment, the selecting surface, the
// projected entries, and the normalized PATH when one applies. Projected
// entries overwrite inherited ones by name, as they do for a check child.
func mutatorEnvironment(surface string, projected map[string]string, pathValue string, normalized bool) []string {
	merged := make(map[string]string)
	for _, entry := range os.Environ() {
		name, value, _ := strings.Cut(entry, "=")
		merged[name] = value
	}
	merged[gateSurfaceVariable] = surface
	for name, value := range projected {
		merged[name] = value
	}
	if normalized {
		merged["PATH"] = pathValue
	}
	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)
	environment := make([]string, 0, len(names))
	for _, name := range names {
		environment = append(environment, name+"="+merged[name])
	}
	return environment
}

// resolveProgram finds the declared executable. A name with a separator is a
// path relative to the child's directory; a bare name is searched in the PATH
// the child will see, so a tool in the original repository is found.
func resolveProgram(program, dir, pathValue string, normalized bool) (string, error) {
	if strings.Contains(program, "/") {
		return program, nil
	}
	search := pathValue
	if !normalized {
		search = os.Getenv("PATH")
	}
	for _, entry := range strings.Split(search, ":") {
		if entry == "" {
			entry = dir
		}
		candidate := filepath.Join(entry, program)
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("`%s` could not be started: No such file or directory", program)
}

// mutatorPath builds the PATH a mutator child sees. A snapshot holds no
// dependency tree, so the original repository's node_modules/.bin comes first
// when it exists, and every relative segment of the caller's PATH is resolved
// from the original repository root. The bool reports whether an override
// applies at all.
func mutatorPath(root string, environment map[string]string) (string, bool, error) {
	path, present := environment["PATH"]
	if !present {
		path, present = os.LookupEnv("PATH")
	}
	var entries []string
	tools := filepath.Join(root, "node_modules", ".bin")
	if info, err := os.Stat(tools); err == nil && info.IsDir() {
		entries = append(entries, tools)
	}
	if present {
		for _, entry := range strings.Split(path, ":") {
			if !filepath.IsAbs(entry) {
				entry = filepath.Join(root, entry)
			}
			entries = append(entries, entry)
		}
	}
	if len(entries) == 0 {
		return "", false, nil
	}
	for _, entry := range entries {
		if strings.Contains(entry, ":") {
			return "", false, errors.New("the mutator PATH cannot be represented safely")
		}
	}
	return strings.Join(entries, ":"), true, nil
}

// ensureSelectedNotLinked refuses a selected path whose final component, or any
// directory above it, is a link in the snapshot, before a mutator can follow it.
func ensureSelectedNotLinked(snapshot string, selected []string) error {
	for _, path := range selected {
		parts := strings.Split(path, "/")
		current := snapshot
		for index, part := range parts {
			current = filepath.Join(current, part)
			info, err := os.Lstat(current)
			if err != nil {
				if os.IsNotExist(err) {
					break
				}
				return fmt.Errorf("could not inspect selected index path `%s`: %s", path, err.Error())
			}
			if info.Mode()&fs.ModeSymlink != 0 {
				if index == len(parts)-1 {
					return fmt.Errorf(errSelectedSymlink, path)
				}
				return fmt.Errorf("selected index path `%s` crosses a symbolic link", path)
			}
		}
	}
	return nil
}

// snapshotDigests fingerprints every regular file under a snapshot, keyed by
// its repository path. Links stay opaque: resolving one could read or write
// outside the snapshot.
func snapshotDigests(root string) (map[string]fileDigest, error) {
	digests := make(map[string]fileDigest)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("could not read snapshot: %s", err.Error())
		}
		if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("snapshot path leaves its root")
		}
		relative = filepath.ToSlash(relative)
		if !gitRepositoryPath(relative) {
			return fmt.Errorf("snapshot contains an escaping path `%s`", relative)
		}
		digest, present, err := digestOf(path)
		if err != nil {
			return err
		}
		if !present {
			return fmt.Errorf("snapshot file `%s` disappeared while it was inspected", relative)
		}
		digests[relative] = digest
		return nil
	})
	return digests, err
}

// digestOf fingerprints one file. A missing file is absent, not an error.
func digestOf(path string) (fileDigest, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fileDigest{}, false, nil
		}
		return fileDigest{}, false, fmt.Errorf("could not read `%s`: %s", path, err.Error())
	}
	return sha256.Sum256(data), true, nil
}

// digestMatches reports whether the working-tree file at path still holds the
// bytes the snapshot started from, so a formatter's change may replace it.
func digestMatches(before map[string]fileDigest, path, root string) bool {
	want, wantPresent := before[path]
	got, gotPresent, err := digestOf(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return false
	}
	return wantPresent == gotPresent && want == got
}

// changedDigests lists every path present in only one snapshot, or present in
// both with different bytes, sorted.
func changedDigests(before, after map[string]fileDigest) []string {
	seen := make(map[string]bool)
	var changed []string
	for path := range before {
		seen[path] = true
	}
	for path := range after {
		seen[path] = true
	}
	for path := range seen {
		want, wantPresent := before[path]
		got, gotPresent := after[path]
		if wantPresent != gotPresent || want != got {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	return changed
}

func containsPath(paths []string, path string) bool {
	for _, held := range paths {
		if held == path {
			return true
		}
	}
	return false
}

// updateIndexBlob records the snapshot's bytes as the staged blob for path,
// keeping the mode the index already holds.
func updateIndexBlob(root, source, path string) error {
	listing, err := gitRun(root, "ls-files", "--stage", "--", path)
	if err != nil {
		return err
	}
	fields := strings.Fields(string(listing))
	if len(fields) == 0 {
		return fmt.Errorf("selected path `%s` is absent from the Git index", path)
	}
	mode := fields[0]
	hash, err := gitRun(root, "hash-object", "-w", "--", source)
	if err != nil {
		return err
	}
	object := strings.TrimSpace(string(hash))
	_, err = gitRun(root, "update-index", "--cacheinfo", mode+","+object+","+path)
	return err
}

// replaceWorkingFile installs the snapshot's bytes at the working-tree path
// through a temporary file in the same directory, so a reader never sees a
// partial file. The snapshot's mode is kept.
func replaceWorkingFile(root, snapshot, path string) error {
	target := filepath.Join(root, filepath.FromSlash(path))
	if err := ensureNoLinkedParents(root, path); err != nil {
		return err
	}
	source := filepath.Join(snapshot, filepath.FromSlash(path))
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("could not read snapshot replacement: %s", err.Error())
	}
	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("could not read snapshot replacement: %s", err.Error())
	}
	temporary, err := os.CreateTemp(filepath.Dir(target), ".rhino-index-*")
	if err != nil {
		return fmt.Errorf("could not create atomic replacement: %s", err.Error())
	}
	name := temporary.Name()
	installed := false
	defer func() {
		if !installed {
			_ = os.Remove(name)
		}
	}()
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("could not write atomic replacement: %s", err.Error())
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("could not flush atomic replacement: %s", err.Error())
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("could not flush atomic replacement: %s", err.Error())
	}
	if err := os.Chmod(name, info.Mode().Perm()); err != nil {
		return fmt.Errorf("could not set atomic replacement mode: %s", err.Error())
	}
	if err := os.Rename(name, target); err != nil {
		return fmt.Errorf("could not replace working-tree file: %s", err.Error())
	}
	installed = true
	return nil
}

// ensureNoLinkedParents refuses a working-tree write whose directories hold a
// link, checked component by component without following it.
func ensureNoLinkedParents(root, path string) error {
	current := root
	parts := strings.Split(path, "/")
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("could not inspect `%s`: %s", path, err.Error())
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("working-tree path `%s` crosses a symbolic link", path)
		}
	}
	return nil
}

// gitRun runs one Git command in root and returns its standard output. A
// nonzero status is a refusal naming only the status, never the child's text.
func gitRun(root string, arguments ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	output, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil, fmt.Errorf("Git command failed with status %d", exit.ExitCode())
		}
		return nil, fmt.Errorf("Git could not start: %s", err.Error())
	}
	return output, nil
}

// gitPaths runs a Git command that prints NUL-separated repository paths.
func gitPaths(root string, arguments ...string) ([]string, error) {
	output, err := gitRun(root, arguments...)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, record := range strings.Split(string(output), "\x00") {
		if record == "" {
			continue
		}
		if !gitRepositoryPath(record) {
			return nil, fmt.Errorf("Git returned an escaping changed path `%s`", record)
		}
		paths = append(paths, record)
	}
	return mergeSorted(paths), nil
}

// mergeSorted returns the sorted, de-duplicated union of its inputs.
func mergeSorted(groups ...[]string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, group := range groups {
		for _, path := range group {
			if !seen[path] {
				seen[path] = true
				out = append(out, path)
			}
		}
	}
	sort.Strings(out)
	return out
}
