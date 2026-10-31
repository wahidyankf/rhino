package bdd_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// invocationTimeout bounds one rhino process. A scenario that hangs fails
// here instead of stalling the whole suite.
const invocationTimeout = 2 * time.Minute

// worldKey carries the scenario state through the godog context.
type worldKey struct{}

func withWorld(ctx context.Context, w *world) context.Context {
	return context.WithValue(ctx, worldKey{}, w)
}

func worldFrom(ctx context.Context) (*world, error) {
	w, ok := ctx.Value(worldKey{}).(*world)
	if !ok || w == nil {
		return nil, errors.New("no scenario world in context")
	}
	return w, nil
}

// world is the state of one scenario. Each scenario owns one directory, which
// holds the repository, a sibling outside the repository for escape cases, a
// bin directory placed first on PATH for bare child names, and the logs that
// fake children write. Nothing outlives the scenario.
type world struct {
	dir     string
	repo    string
	outside string
	bin     string
	logs    string
	binary  string

	config     string
	stdin      *string
	runs       []invocation
	git        gitState
	snapshotOn bool
}

// invocation records one rhino process and the repository state either side.
type invocation struct {
	args     []string
	stdout   string
	stderr   string
	exitCode int
	before   snapshot
	after    snapshot
}

// newWorld creates an empty scenario directory under parent.
func newWorld(parent, binary string) (*world, error) {
	dir, err := os.MkdirTemp(parent, "scenario-")
	if err != nil {
		return nil, fmt.Errorf("create scenario directory: %w", err)
	}
	w := &world{
		dir:     dir,
		repo:    filepath.Join(dir, "repo"),
		outside: filepath.Join(dir, "outside"),
		bin:     filepath.Join(dir, "bin"),
		logs:    filepath.Join(dir, "logs"),
		binary:  binary,
	}
	for _, path := range []string{w.repo, w.outside, w.bin, w.logs} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			_ = os.RemoveAll(dir)
			return nil, fmt.Errorf("create %s: %w", path, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "gitconfig"), nil, 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("write git configuration: %w", err)
	}
	return w, nil
}

// close removes the scenario directory. Permissions a scenario removed from a
// file do not block removal, because deleting needs only the directory.
func (w *world) close() error {
	return os.RemoveAll(w.dir)
}

// repoPath resolves a repository-relative path and refuses one that leaves the
// repository, so a scenario cannot write outside its own directory.
func (w *world) repoPath(rel string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("scenario path %q leaves the repository", rel)
	}
	return filepath.Join(w.repo, clean), nil
}

// writeRepoFile writes content at a repository-relative path, creating parent
// directories. A file whose content starts with a shebang is made executable,
// because the scenarios use such files as gate children.
func (w *world) writeRepoFile(rel, content string) error {
	path, err := w.repoPath(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", rel, err)
	}
	mode := os.FileMode(0o644)
	if strings.HasPrefix(content, "#!") {
		mode = 0o755
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		return fmt.Errorf("write %s: %w", rel, err)
	}
	return os.Chmod(path, mode)
}

// writeScript writes an executable fixture at an absolute path.
func writeScript(path, body string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		return fmt.Errorf("write script %s: %w", path, err)
	}
	return os.Chmod(path, 0o755)
}

// shellQuote quotes one word for a POSIX shell fixture. The fixtures are
// written as files and run with argv, so this quoting only has to be right for
// the paths and values the scenarios name.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// environment returns the variables every rhino process and fixture child sees.
// Inherited RHINO_ and GIT_ variables are dropped, so a developer's shell cannot
// change a scenario. PATH gains the scenario bin directory first, and Git reads
// only the scenario's empty configuration.
func (w *world) environment() []string {
	var env []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if name == "PATH" || strings.HasPrefix(name, "RHINO_") || strings.HasPrefix(name, "GIT_") {
			continue
		}
		env = append(env, entry)
	}
	env = append(env, "PATH="+w.bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return append(env, w.gitEnvironment()...)
}

// gitEnvironment isolates Git from the developer's global and system
// configuration and fixes the identity used for commits the suite creates.
func (w *world) gitEnvironment() []string {
	return []string{
		"GIT_CONFIG_GLOBAL=" + filepath.Join(w.dir, "gitconfig"),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=rhino-bdd",
		"GIT_AUTHOR_EMAIL=rhino-bdd@example.invalid",
		"GIT_COMMITTER_NAME=rhino-bdd",
		"GIT_COMMITTER_EMAIL=rhino-bdd@example.invalid",
	}
}

// invoke runs the rhino binary with the given pipe-separated arguments in the
// repository. Every invocation records stdout, stderr, the exit code, and the
// repository snapshot before and after, which the state assertions read.
// Placeholders {base} and {head} resolve to the scenario's Git range.
func (w *world) invoke(ctx context.Context, joined string) error {
	var args []string
	if joined != "" {
		args = strings.Split(joined, "|")
	}
	for i, arg := range args {
		resolved, err := w.resolveRange(ctx, arg)
		if err != nil {
			return err
		}
		args[i] = resolved
	}
	before, err := takeSnapshot(w.repo)
	if err != nil {
		return fmt.Errorf("snapshot repository before run: %w", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, invocationTimeout)
	defer cancel()
	command := exec.CommandContext(runCtx, w.binary, args...)
	command.Dir = w.repo
	command.Env = w.environment()
	if w.stdin != nil {
		command.Stdin = strings.NewReader(*w.stdin)
	}
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	runErr := command.Run()
	w.stdin = nil
	exitCode := 0
	var exit *exec.ExitError
	switch {
	case runErr == nil:
	case errors.As(runErr, &exit):
		exitCode = exit.ExitCode()
	default:
		return fmt.Errorf("start rhino %q: %w", joined, runErr)
	}
	if runCtx.Err() != nil {
		return fmt.Errorf("rhino %q exceeded %s", joined, invocationTimeout)
	}

	after, err := takeSnapshot(w.repo)
	if err != nil {
		return fmt.Errorf("snapshot repository after run: %w", err)
	}
	w.runs = append(w.runs, invocation{
		args:     args,
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		exitCode: exitCode,
		before:   before,
		after:    after,
	})
	return nil
}

// lastRun returns the most recent invocation, or an error when none ran.
func (w *world) lastRun() (invocation, error) {
	if len(w.runs) == 0 {
		return invocation{}, errors.New("no CLI invocation has run in this scenario")
	}
	return w.runs[len(w.runs)-1], nil
}

// snapshot maps every repository path to a description of its kind, mode,
// content digest, or link target. Two equal snapshots mean no byte changed.
type snapshot map[string]string

// takeSnapshot walks the tree without following links. A file the scenario made
// unreadable is recorded as unreadable, which is still a stable description.
func takeSnapshot(root string) (snapshot, error) {
	result := snapshot{}
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
		mode := info.Mode()
		switch {
		case mode&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			result[rel] = "link " + target
		case mode.IsDir():
			result[rel] = "dir " + mode.Perm().String()
		case mode.IsRegular():
			data, err := os.ReadFile(path)
			if err != nil {
				result[rel] = "unreadable " + mode.Perm().String()
				return nil
			}
			result[rel] = fmt.Sprintf("file %s %x", mode.Perm(), sha256.Sum256(data))
		default:
			result[rel] = "other " + mode.String()
		}
		return nil
	})
	return result, err
}

// sameSnapshot reports whether two snapshots describe identical trees.
func sameSnapshot(before, after snapshot) bool {
	return maps.Equal(before, after)
}

// declaredGate is the subset of a gate entry the fixtures need.
type declaredGate struct {
	ID      string `yaml:"id"`
	Type    string `yaml:"type"`
	Command struct {
		Executable string `yaml:"executable"`
	} `yaml:"command"`
}

// declaredGates reads gates.entries from the scenario configuration.
func (w *world) declaredGates() ([]declaredGate, error) {
	var document struct {
		Gates struct {
			Entries []declaredGate `yaml:"entries"`
		} `yaml:"gates"`
	}
	if err := yaml.Unmarshal([]byte(w.config), &document); err != nil {
		return nil, fmt.Errorf("read scenario configuration: %w", err)
	}
	return document.Gates.Entries, nil
}

// gateByID returns the declared gate with the given identifier.
func (w *world) gateByID(id string) (declaredGate, error) {
	gates, err := w.declaredGates()
	if err != nil {
		return declaredGate{}, err
	}
	for _, gate := range gates {
		if gate.ID == id {
			return gate, nil
		}
	}
	return declaredGate{}, fmt.Errorf("the configuration declares no gate %q", id)
}

// mutationGate returns the first declared mutation gate.
func (w *world) mutationGate() (declaredGate, error) {
	gates, err := w.declaredGates()
	if err != nil {
		return declaredGate{}, err
	}
	for _, gate := range gates {
		if gate.Type == "mutation" {
			return gate, nil
		}
	}
	return declaredGate{}, errors.New("the configuration declares no mutation gate")
}

// placeExecutable puts a fixture where the configured command finds it. A
// command with a slash is a repository-relative path. A bare name is found on
// PATH, so the fixture goes into the scenario bin directory.
func (w *world) placeExecutable(command, body string) error {
	if strings.Contains(command, "/") {
		path, err := w.repoPath(command)
		if err != nil {
			return err
		}
		return writeScript(path, body)
	}
	return writeScript(filepath.Join(w.bin, command), body)
}

// childLog returns the prefix of the files a fixture child writes for a gate.
func (w *world) childLog(id string) string {
	return filepath.Join(w.logs, "child-"+id)
}

// readChildLog reads one of the files a fixture child wrote.
func (w *world) readChildLog(id, suffix string) (string, error) {
	data, err := os.ReadFile(w.childLog(id) + suffix)
	if err != nil {
		return "", fmt.Errorf("gate %q wrote no %s record; it did not run: %w", id, suffix, err)
	}
	return string(data), nil
}

// readRepoFile reads a repository-relative file for an assertion.
func (w *world) readRepoFile(rel string) (string, error) {
	path, err := w.repoPath(rel)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", rel, err)
	}
	return string(data), nil
}
