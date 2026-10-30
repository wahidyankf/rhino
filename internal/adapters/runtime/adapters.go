package runtime

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/wahidyankf/rhino/internal/application"
)

// maxModelOutput caps one declared model-listing child's captured output,
// the way the oracle bounds it: a child that exceeds the bound yields no
// output at all.
const maxModelOutput = 8 * 1024 * 1024

// adapterTransactionOrdinal numbers staged adapter transactions inside this
// process, so two generations never share a stage directory.
var adapterTransactionOrdinal atomic.Uint64

// DiskAdapterStore is the narrow write port only canonical adapter
// generation holds. It stages every byte below the selected repository
// root, rejects a link at any managed path component, and restores moved
// families and files when a later rename fails.
type DiskAdapterStore struct{}

// Replace installs one planned adapter transaction atomically per family:
// every desired byte is staged first, then each family and exact file is
// moved into place with its predecessor preserved for rollback.
func (DiskAdapterStore) Replace(root string, transaction application.AdapterTransaction) error {
	info, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("cannot inspect adapter root: %s", err.Error())
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("the repository root is not a non-link directory")
	}
	roots, err := adapterRoots(transaction.Roots)
	if err != nil {
		return err
	}
	exactPaths, err := adapterExactPaths(roots, transaction.ExactPaths)
	if err != nil {
		return err
	}
	desired, err := adapterFiles(roots, exactPaths, transaction.Files)
	if err != nil {
		return err
	}
	for _, managed := range roots {
		if err := ensureNoAdapterLinks(root, managed); err != nil {
			return err
		}
	}
	for _, path := range exactPaths {
		if err := ensureNoAdapterLinks(root, path); err != nil {
			return err
		}
	}
	ordinal := adapterTransactionOrdinal.Add(1) - 1
	stage := filepath.Join(root, fmt.Sprintf(".rhino-adapter-transaction-%d-%d", os.Getpid(), ordinal))
	if err := os.Mkdir(stage, 0o755); err != nil {
		return fmt.Errorf("cannot create adapter transaction: %s", err.Error())
	}
	result := replaceAdapterEntries(root, stage, roots, exactPaths, desired)
	_ = os.RemoveAll(stage)
	return result
}

type managedAdapterKind int

const (
	managedFamily managedAdapterKind = iota
	managedFile
)

type movedAdapter struct {
	target    string
	backup    string
	installed bool
	kind      managedAdapterKind
}

func replaceAdapterEntries(root, stage string, roots, exactPaths []string, desired map[string]string) error {
	staged := filepath.Join(stage, "desired")
	if err := os.Mkdir(staged, 0o755); err != nil {
		return fmt.Errorf("cannot stage adapter bytes: %s", err.Error())
	}
	ordered := make([]string, 0, len(desired))
	for path := range desired {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	for _, path := range ordered {
		target := filepath.Join(staged, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("cannot create adapter stage path: %s", err.Error())
		}
		file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
		if err != nil {
			return fmt.Errorf("cannot stage adapter file: %s", err.Error())
		}
		if _, err := file.WriteString(desired[path]); err != nil {
			_ = file.Close()
			return fmt.Errorf("cannot write adapter stage file: %s", err.Error())
		}
		if err := file.Sync(); err != nil {
			_ = file.Close()
			return fmt.Errorf("cannot sync adapter stage file: %s", err.Error())
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("cannot sync adapter stage file: %s", err.Error())
		}
	}
	backups := filepath.Join(stage, "backups")
	if err := os.Mkdir(backups, 0o755); err != nil {
		return fmt.Errorf("cannot prepare adapter rollback: %s", err.Error())
	}
	var moved []movedAdapter
	restore := func() { restoreAdapterEntries(moved) }
	for index, managed := range roots {
		target := filepath.Join(root, filepath.FromSlash(managed))
		stagedFamily := filepath.Join(staged, filepath.FromSlash(managed))
		if info, err := os.Stat(stagedFamily); err != nil || !info.IsDir() {
			restore()
			return fmt.Errorf("adapter transaction has no generated family for `%s`", managed)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			restore()
			return fmt.Errorf("cannot create parent for adapter family `%s`: %s", managed, err.Error())
		}
		backup := filepath.Join(backups, fmt.Sprintf("%d", index))
		if _, err := os.Stat(target); err == nil {
			metadata, err := os.Lstat(target)
			if err != nil {
				restore()
				return fmt.Errorf("cannot inspect current adapter family `%s`: %s", managed, err.Error())
			}
			if metadata.Mode()&os.ModeSymlink != 0 || !metadata.IsDir() {
				restore()
				return fmt.Errorf("adapter family `%s` is not a non-link directory", managed)
			}
			if err := os.Rename(target, backup); err != nil {
				restore()
				return fmt.Errorf("cannot move current adapter family `%s` aside: %s", managed, err.Error())
			}
		}
		moved = append(moved, movedAdapter{target: target, backup: backup, kind: managedFamily})
		if err := os.Rename(stagedFamily, target); err != nil {
			restore()
			return fmt.Errorf("cannot install adapter family `%s`: %s", managed, err.Error())
		}
		moved[len(moved)-1].installed = true
	}
	for index, path := range exactPaths {
		target := filepath.Join(root, filepath.FromSlash(path))
		stagedFile := filepath.Join(staged, filepath.FromSlash(path))
		if info, err := os.Stat(stagedFile); err != nil || info.IsDir() {
			restore()
			return fmt.Errorf("adapter transaction has no generated file for `%s`", path)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			restore()
			return fmt.Errorf("cannot create parent for exact adapter `%s`: %s", path, err.Error())
		}
		backup := filepath.Join(backups, fmt.Sprintf("exact-%d", index))
		if _, err := os.Stat(target); err == nil {
			metadata, err := os.Lstat(target)
			if err != nil {
				restore()
				return fmt.Errorf("cannot inspect current exact adapter `%s`: %s", path, err.Error())
			}
			if metadata.Mode()&os.ModeSymlink != 0 || !metadata.Mode().IsRegular() {
				restore()
				return fmt.Errorf("exact adapter `%s` is not a regular non-link file", path)
			}
			if err := os.Rename(target, backup); err != nil {
				restore()
				return fmt.Errorf("cannot move current exact adapter `%s` aside: %s", path, err.Error())
			}
		}
		moved = append(moved, movedAdapter{target: target, backup: backup, kind: managedFile})
		if err := os.Rename(stagedFile, target); err != nil {
			restore()
			return fmt.Errorf("cannot install exact adapter `%s`: %s", path, err.Error())
		}
		moved[len(moved)-1].installed = true
	}
	return nil
}

func restoreAdapterEntries(moved []movedAdapter) {
	for i := len(moved) - 1; i >= 0; i-- {
		entry := moved[i]
		if entry.installed {
			if _, err := os.Stat(entry.target); err == nil {
				switch entry.kind {
				case managedFamily:
					_ = os.RemoveAll(entry.target)
				case managedFile:
					_ = os.Remove(entry.target)
				}
			}
		}
		if _, err := os.Stat(entry.backup); err == nil {
			_ = os.Rename(entry.backup, entry.target)
		}
	}
}

// ensureNoAdapterLinks refuses a managed path crossing a filesystem link,
// checked component by component from the link itself, never by following
// it.
func ensureNoAdapterLinks(root, relative string) error {
	current := root
	for _, segment := range strings.Split(relative, "/") {
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				break
			}
			return fmt.Errorf("cannot inspect adapter path `%s`: %s", relative, err.Error())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("adapter path `%s` crosses a symbolic link", relative)
		}
	}
	return nil
}

func normalAdapterPath(path string) (string, bool) {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return "", false
	}
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == "" || segment == "." || segment == ".." || strings.ContainsRune(segment, '\\') {
			return "", false
		}
	}
	return trimmed, true
}

func underAdapterRoot(path, root string) bool {
	return path == root || (strings.HasPrefix(path, root) && len(path) > len(root) && path[len(root)] == '/')
}

func adapterRoots(roots []string) ([]string, error) {
	normalized := make([]string, 0, len(roots))
	for _, root := range roots {
		path, ok := normalAdapterPath(root)
		if !ok {
			return nil, fmt.Errorf("invalid adapter root `%s`", root)
		}
		normalized = append(normalized, path)
	}
	sort.Strings(normalized)
	for i := 1; i < len(normalized); i++ {
		if normalized[i] == normalized[i-1] {
			return nil, fmt.Errorf("adapter roots must be distinct, non-overlapping repository paths")
		}
	}
	for i := 1; i < len(normalized); i++ {
		if underAdapterRoot(normalized[i], normalized[i-1]) {
			return nil, fmt.Errorf("adapter roots must be distinct, non-overlapping repository paths")
		}
	}
	return normalized, nil
}

func adapterExactPaths(roots, exact []string) ([]string, error) {
	normalized := make([]string, 0, len(exact))
	for _, path := range exact {
		clean, ok := normalAdapterPath(path)
		if !ok {
			return nil, fmt.Errorf("invalid exact adapter path `%s`", path)
		}
		normalized = append(normalized, clean)
	}
	sort.Strings(normalized)
	for i := 1; i < len(normalized); i++ {
		if normalized[i] == normalized[i-1] {
			return nil, fmt.Errorf("exact adapter paths must be distinct and outside every adapter root")
		}
	}
	for _, path := range normalized {
		for _, root := range roots {
			if underAdapterRoot(path, root) {
				return nil, fmt.Errorf("exact adapter paths must be distinct and outside every adapter root")
			}
		}
	}
	return normalized, nil
}

func adapterFiles(roots, exactPaths []string, files []application.AdapterFile) (map[string]string, error) {
	desired := make(map[string]string, len(files))
	for _, file := range files {
		path, ok := normalAdapterPath(file.Path)
		if !ok {
			return nil, fmt.Errorf("invalid generated adapter path `%s`", file.Path)
		}
		under := false
		for _, root := range roots {
			if underAdapterRoot(path, root) {
				under = true
				break
			}
		}
		if !under && !containsAdapterPath(exactPaths, path) {
			return nil, fmt.Errorf("generated adapter path `%s` is outside every declared adapter family or exact path", path)
		}
		if _, present := desired[path]; present {
			return nil, fmt.Errorf("duplicate generated adapter path `%s`", path)
		}
		desired[path] = file.Contents
	}
	for _, path := range exactPaths {
		if _, present := desired[path]; !present {
			return nil, fmt.Errorf("every exact adapter path must have generated bytes")
		}
	}
	return desired, nil
}

func containsAdapterPath(paths []string, path string) bool {
	for _, held := range paths {
		if held == path {
			return true
		}
	}
	return false
}

// modelCaptured is what the output reader kept: at most one byte past the
// bound, so an over-long child is still detected after it is reaped.
type modelCaptured struct {
	output []byte
	failed error
}

// DiskModelResolver is the no-shell process boundary for a declared
// model-listing command. Its output is read only for model names and never
// enters a report; a bound keeps a hostile child from retaining unbounded
// output.
type DiskModelResolver struct{}

// Resolve starts one declared model vector and reports its standard output.
func (DiskModelResolver) Resolve(launch application.ModelLaunch) (string, error) {
	if len(launch.Arguments) == 0 {
		return "", fmt.Errorf("the model command declares no executable")
	}
	program := launch.Arguments[0]
	if strings.TrimSpace(program) == "" || strings.HasPrefix(program, "-") {
		return "", fmt.Errorf("the model command's executable is invalid")
	}
	command := exec.Command(program, launch.Arguments[1:]...)
	command.Dir = launch.Directory
	stream, err := command.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("could not start model command: %s", err.Error())
	}
	if err := command.Start(); err != nil {
		return "", fmt.Errorf("could not start model command: %s", err.Error())
	}
	read := make(chan modelCaptured, 1)
	go func() {
		output, err := io.ReadAll(io.LimitReader(stream, maxModelOutput+1))
		if err != nil {
			read <- modelCaptured{failed: fmt.Errorf("could not read model command output: %s", err.Error())}
			return
		}
		read <- modelCaptured{output: output}
	}()
	waited := make(chan error, 1)
	go func() { waited <- command.Wait() }()
	timer := time.NewTimer(time.Duration(launch.TimeoutSeconds) * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		_ = command.Process.Kill()
		<-waited
		<-read
		return "", fmt.Errorf("the model command exceeded its timeout")
	case err := <-waited:
		// The wait error is discarded on purpose: the exit below is read
		// from the reaped process state, which reports a signal death and a
		// nonzero status alike.
		_ = err
		captured := <-read
		return finishModel(command, captured)
	}
}

func finishModel(command *exec.Cmd, captured modelCaptured) (string, error) {
	if captured.failed != nil {
		return "", captured.failed
	}
	if int64(len(captured.output)) > maxModelOutput {
		return "", fmt.Errorf("the model command emitted too much output")
	}
	state := command.ProcessState
	if state == nil || state.ExitCode() == -1 {
		return "", fmt.Errorf("the model command ended by signal")
	}
	if state.ExitCode() != 0 {
		return "", fmt.Errorf("the model command exited with status %d", state.ExitCode())
	}
	if !utf8.Valid(captured.output) {
		return "", fmt.Errorf("the model command emitted non-text output")
	}
	return string(captured.output), nil
}
