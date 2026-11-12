package vault

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// argumentSeparator ends each argument the shim logs. Unit separator control
// characters cannot occur in the arguments a validation run passes to Git.
const argumentSeparator = "\x1f"

// GitTrace is a directory holding a git shim that logs each invocation and
// then runs the real Git by its resolved absolute path. An adapter puts the
// directory alone on PATH for one validation run, so the run can start git and
// nothing else found by name.
type GitTrace struct {
	bin string
	log string
}

// NewGitTrace creates the shim inside dir. The real Git is resolved now, while
// PATH still reaches it, and the shim executes that path directly.
func NewGitTrace(dir string) (*GitTrace, error) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("find git for the trace: %w", err)
	}
	trace := &GitTrace{bin: filepath.Join(dir, "trace-bin"), log: filepath.Join(dir, "trace.log")}
	if err := os.MkdirAll(trace.bin, 0o755); err != nil {
		return nil, fmt.Errorf("create trace directory: %w", err)
	}
	logQuoted := shellQuote(trace.log)
	script := "#!/bin/sh\n" +
		"printf '%s" + argumentSeparator + "' \"$@\" >> " + logQuoted + "\n" +
		"printf '\\n' >> " + logQuoted + "\n" +
		"exec " + shellQuote(realGit) + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(trace.bin, "git"), []byte(script), 0o755); err != nil {
		return nil, fmt.Errorf("write git shim: %w", err)
	}
	if err := trace.Reset(); err != nil {
		return nil, err
	}
	return trace, nil
}

// PATH is the value to put on PATH for a traced run: the shim directory alone.
func (g *GitTrace) PATH() string {
	return g.bin
}

// Reset empties the log before a run, so Processes reports only that run.
func (g *GitTrace) Reset() error {
	if err := os.WriteFile(g.log, nil, 0o600); err != nil {
		return fmt.Errorf("reset git trace: %w", err)
	}
	return nil
}

// Processes returns the git invocations the shim logged since the last Reset,
// in the order they ran.
func (g *GitTrace) Processes() ([]Process, error) {
	content, err := os.ReadFile(g.log)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read git trace: %w", err)
	}
	var processes []Process
	for _, line := range strings.Split(string(content), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, argumentSeparator)
		// Each argument ends with the separator, so the last field is empty.
		fields = fields[:len(fields)-1]
		processes = append(processes, Process{Program: "git", Args: fields})
	}
	return processes, nil
}

// shellQuote quotes one value for a POSIX shell script.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
