package bdd_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gitState is the Git repository a scenario builds on demand. The base commit
// is empty and is the repository's only commit. The range head is a second
// commit whose tree lists the files the explicit range changes. Building it
// with a separate index file keeps the repository's real index untouched, so
// the index steps and the range steps stay independent.
type gitState struct {
	ready    bool
	base     string
	head     string
	fixtures []trackedFile
}

// trackedFile is a fixture file the scenario commits so Git sees it with its
// mode. A declared mutation child is one: a snapshot or a replay checkout only
// holds committed content, so an untracked child cannot start.
type trackedFile struct {
	path string
	mode string
	blob string
}

// gitRun runs git with argv only, inside the scenario repository. The optional
// extra variables come after the scenario environment and win over it.
func (w *world) gitRun(ctx context.Context, stdin string, extra []string, args ...string) (string, error) {
	gitTool, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("locate git: %w", err)
	}
	command := exec.CommandContext(ctx, gitTool, args...)
	command.Dir = w.repo
	command.Env = append(w.environment(), extra...)
	if stdin != "" {
		command.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// ensureGit initializes the repository on first use. HEAD names the main
// branch, and main points at the empty base commit, so index comparisons have a
// commit to compare against.
func (w *world) ensureGit(ctx context.Context) error {
	if w.git.ready {
		return nil
	}
	if _, err := w.gitRun(ctx, "", nil, "init", "-q", "."); err != nil {
		return err
	}
	if _, err := w.gitRun(ctx, "", nil, "symbolic-ref", "HEAD", "refs/heads/main"); err != nil {
		return err
	}
	base, err := w.buildBase(ctx)
	if err != nil {
		return err
	}
	if _, err := w.gitRun(ctx, "", nil, "update-ref", "refs/heads/main", base); err != nil {
		return err
	}
	w.git = gitState{ready: true, base: base, head: base}
	return nil
}

// writeTree returns the tree of the index named by extra.
func (w *world) writeTree(ctx context.Context, extra []string) (string, error) {
	out, err := w.gitRun(ctx, "", extra, "write-tree")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// commitTree creates a commit of tree; an empty parent makes a root commit.
func (w *world) commitTree(ctx context.Context, tree, parent string) (string, error) {
	args := []string{"commit-tree", tree, "-m", "rhino bdd fixture"}
	if parent != "" {
		args = append(args, "-p", parent)
	}
	out, err := w.gitRun(ctx, "", nil, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// blobFor stores content as a blob and returns its object name.
func (w *world) blobFor(ctx context.Context, content string) (string, error) {
	out, err := w.gitRun(ctx, content, nil, "hash-object", "-w", "--stdin")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// workingContent returns the working-tree bytes of a file, or nothing when the
// file does not exist, so an index entry for a new path starts empty.
func (w *world) workingContent(rel string) (string, error) {
	path, err := w.repoPath(rel)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read working tree file %s: %w", rel, err)
	}
	return string(data), nil
}

// indexSet stages a regular file with content into the real index.
func (w *world) indexSet(ctx context.Context, rel, content string) error {
	if err := w.ensureGit(ctx); err != nil {
		return err
	}
	blob, err := w.blobFor(ctx, content)
	if err != nil {
		return err
	}
	_, err = w.gitRun(ctx, "", nil, "update-index", "--add", "--cacheinfo", "100644,"+blob+","+rel)
	return err
}

// indexReset empties the real index, so it selects no regular files.
func (w *world) indexReset(ctx context.Context) error {
	if err := w.ensureGit(ctx); err != nil {
		return err
	}
	_, err := w.gitRun(ctx, "", nil, "read-tree", "--empty")
	return err
}

// rangeSet adds a regular file with content to the explicit range head and
// creates the head commit that results.
func (w *world) rangeSet(ctx context.Context, rel, content string) error {
	if err := w.ensureGit(ctx); err != nil {
		return err
	}
	blob, err := w.blobFor(ctx, content)
	if err != nil {
		return err
	}
	rangeIndex := []string{"GIT_INDEX_FILE=" + filepath.Join(w.dir, "range.index")}
	if _, err := w.gitRun(ctx, "", rangeIndex, "update-index", "--add", "--cacheinfo", "100644,"+blob+","+rel); err != nil {
		return err
	}
	return w.commitRange(ctx)
}

// addFixtures stages every tracked fixture into the index named by extra.
func (w *world) addFixtures(ctx context.Context, extra []string) error {
	for _, file := range w.git.fixtures {
		if _, err := w.gitRun(ctx, "", extra, "update-index", "--add", "--cacheinfo", file.mode+","+file.blob+","+file.path); err != nil {
			return err
		}
	}
	return nil
}

// buildBase commits the base: an empty root commit that holds every tracked
// fixture. A fixture in the base never counts as changed by an explicit range.
func (w *world) buildBase(ctx context.Context) (string, error) {
	baseIndex := []string{"GIT_INDEX_FILE=" + filepath.Join(w.dir, "base.index")}
	if err := w.addFixtures(ctx, baseIndex); err != nil {
		return "", err
	}
	tree, err := w.writeTree(ctx, baseIndex)
	if err != nil {
		return "", err
	}
	return w.commitTree(ctx, tree, "")
}

// commitRange rebuilds the range head from the range index, which also holds
// every tracked fixture, so the replay checkout can start the fixtures.
func (w *world) commitRange(ctx context.Context) error {
	rangeIndex := []string{"GIT_INDEX_FILE=" + filepath.Join(w.dir, "range.index")}
	if err := w.addFixtures(ctx, rangeIndex); err != nil {
		return err
	}
	tree, err := w.writeTree(ctx, rangeIndex)
	if err != nil {
		return err
	}
	head, err := w.commitTree(ctx, tree, w.git.base)
	if err != nil {
		return err
	}
	w.git.head = head
	return nil
}

// trackExecutable commits the working-tree file at rel as an executable. The
// base and the range head both hold it, so no explicit range reports it as
// changed. The real index holds it only when staged is true, because a snapshot
// of the index is the only place a pre-commit replay looks.
func (w *world) trackExecutable(ctx context.Context, rel string, staged bool) error {
	if err := w.ensureGit(ctx); err != nil {
		return err
	}
	content, err := w.workingContent(rel)
	if err != nil {
		return err
	}
	blob, err := w.blobFor(ctx, content)
	if err != nil {
		return err
	}
	w.git.fixtures = append(w.git.fixtures, trackedFile{path: rel, mode: "100755", blob: blob})
	previous := w.git.base
	base, err := w.buildBase(ctx)
	if err != nil {
		return err
	}
	if _, err := w.gitRun(ctx, "", nil, "update-ref", "refs/heads/main", base); err != nil {
		return err
	}
	w.git.base = base
	if w.git.head == previous {
		w.git.head = base
	} else if err := w.commitRange(ctx); err != nil {
		return err
	}
	if staged {
		_, err = w.gitRun(ctx, "", nil, "update-index", "--add", "--cacheinfo", "100755,"+blob+","+rel)
		return err
	}
	return nil
}

// resolveRange replaces {base} and {head} in one argument with the scenario's
// commit names. An argument with neither placeholder is returned unchanged.
func (w *world) resolveRange(ctx context.Context, arg string) (string, error) {
	if !strings.Contains(arg, "{base}") && !strings.Contains(arg, "{head}") {
		return arg, nil
	}
	if err := w.ensureGit(ctx); err != nil {
		return "", err
	}
	arg = strings.ReplaceAll(arg, "{base}", w.git.base)
	return strings.ReplaceAll(arg, "{head}", w.git.head), nil
}

// indexBlob returns the content the real index holds at a path.
func (w *world) indexBlob(ctx context.Context, rel string) (string, error) {
	if err := w.ensureGit(ctx); err != nil {
		return "", err
	}
	return w.gitRun(ctx, "", nil, "show", ":"+rel)
}
