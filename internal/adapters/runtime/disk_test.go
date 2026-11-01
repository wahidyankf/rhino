package runtime_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/runtime"
	"github.com/wahidyankf/rhino/internal/application"
)

func write(t *testing.T, dir, name, text string) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiskTreeReadsAndRefuses(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "docs/a.md", "hello\n")
	write(t, dir, "bin/x", "\xff\xfe-not-utf8")
	tree := &runtime.DiskTree{Dir: dir}

	text, readErr := tree.Read("docs/a.md")
	if readErr != nil || text != "hello\n" {
		t.Fatalf("read failed: %q %+v", text, readErr)
	}
	if _, readErr := tree.Read("missing.md"); readErr == nil || readErr.Kind != "missing" {
		t.Fatalf("missing file not reported: %+v", readErr)
	}
	if _, readErr := tree.Read("bin/x"); readErr == nil || readErr.Kind != "not-text" {
		t.Fatalf("binary file not reported: %+v", readErr)
	}
	if _, readErr := tree.Read("../outside"); readErr == nil {
		t.Fatal("escaping path was read")
	}
	if _, err := tree.RootedAt("missing-dir-xyz"); err == nil {
		t.Fatal("bad root was accepted")
	}
	if _, err := tree.RootedAt("../missing-outside"); err == nil {
		t.Fatal("missing root outside the working directory was accepted")
	}
	found := false
	for _, file := range tree.Files() {
		if file == "docs/a.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("walk missed file: %v", tree.Files())
	}
}

func TestDiskTreeExistsCoversFilesAndDirectories(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "docs/a.md", "hello\n")
	write(t, dir, "docs/sub/page.md", "page\n")
	tree := &runtime.DiskTree{Dir: dir}

	for _, path := range []string{"docs/a.md", "docs", "docs/sub"} {
		if !tree.Exists(path) {
			t.Fatalf("%s should exist: %v", path, tree.Files())
		}
	}
	for _, path := range []string{"docs/missing.md", "nope", "../outside", ""} {
		if tree.Exists(path) {
			t.Fatalf("%s should not exist", path)
		}
	}
}

func TestDiskTreePassesThroughLink(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "real/a.md", "x\n")
	write(t, dir, "docs/a.md", "doc\n")
	if err := os.Symlink(filepath.Join(dir, "real"), filepath.Join(dir, "linked")); err != nil {
		t.Skip("symlinks unavailable")
	}
	tree := &runtime.DiskTree{Dir: dir}

	for _, path := range []string{"linked", "linked/a.md", "docs/../linked/a.md"} {
		if !tree.PassesThroughLink(path) {
			t.Fatalf("%s should pass through a link", path)
		}
	}
	for _, path := range []string{"real/a.md", "docs/a.md", "missing.md", "", "."} {
		if tree.PassesThroughLink(path) {
			t.Fatalf("%s should not pass through a link", path)
		}
	}
}

func TestDiskEnvironmentStoreCreatesOnlyAbsentTargets(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, ".env.example", "EXAMPLE_KEY=placeholder\n")
	store := runtime.DiskEnvironmentStore{}

	if err := store.Create(dir, nil); err != nil {
		t.Fatalf("empty create refused: %v", err)
	}
	if err := store.Create(dir, []application.EnvironmentFile{{Path: ".env.generated", Contents: "EXAMPLE_KEY=placeholder\n"}}); err != nil {
		t.Fatalf("create refused: %v", err)
	}
	created, err := os.ReadFile(filepath.Join(dir, ".env.generated"))
	if err != nil || string(created) != "EXAMPLE_KEY=placeholder\n" {
		t.Fatalf("create wrote wrong bytes: %q %v", created, err)
	}
	if err := store.Create(dir, []application.EnvironmentFile{{Path: ".env.generated", Contents: "other\n"}}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("repeat create did not refuse: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".rhino-environment-transaction-") {
			t.Fatalf("stage leaked: %s", entry.Name())
		}
	}
}

func TestDiskEnvironmentStoreRestoresOnlyWithForceOnConflict(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "saved/.env.fixture", "SYNTHETIC=restored\n")
	write(t, dir, ".env.fixture", "SYNTHETIC=current\n")
	store := runtime.DiskEnvironmentStore{}

	conflict := []application.EnvironmentRestoreFile{{Path: ".env.fixture", Contents: "SYNTHETIC=restored\n"}}
	if err := store.Restore(dir, conflict); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("conflict without replace did not refuse: %v", err)
	}
	current, _ := os.ReadFile(filepath.Join(dir, ".env.fixture"))
	if string(current) != "SYNTHETIC=current\n" {
		t.Fatalf("refused restore overwrote its target: %q", current)
	}
	replacement := []application.EnvironmentRestoreFile{{Path: ".env.fixture", Contents: "SYNTHETIC=restored\n", Replace: true}}
	if err := store.Restore(dir, replacement); err != nil {
		t.Fatalf("forced restore refused: %v", err)
	}
	restored, _ := os.ReadFile(filepath.Join(dir, ".env.fixture"))
	if string(restored) != "SYNTHETIC=restored\n" {
		t.Fatalf("restore wrote wrong bytes: %q", restored)
	}
	write(t, dir, "blocked", "a file, not a directory\n")
	blocked := []application.EnvironmentRestoreFile{{Path: "blocked/.env.fixture", Contents: "x\n", Replace: true}}
	if err := store.Restore(dir, blocked); err == nil {
		t.Fatal("restore through a file parent was accepted")
	}
	if content, _ := os.ReadFile(filepath.Join(dir, "blocked")); string(content) != "a file, not a directory\n" {
		t.Fatalf("refused restore touched its blocker: %q", content)
	}
}

func TestDiskEnvironmentStoreRefusesUnsafeTransactions(t *testing.T) {
	store := runtime.DiskEnvironmentStore{}

	regular := t.TempDir()
	write(t, regular, "root-file", "x\n")
	if err := store.Create(filepath.Join(regular, "root-file"), nil); err == nil || !strings.Contains(err.Error(), "not a non-link directory") {
		t.Fatalf("file root not refused: %v", err)
	}
	if err := store.Create(regular, []application.EnvironmentFile{{Path: "a/../outside", Contents: "x"}}); err == nil || !strings.Contains(err.Error(), "invalid environment target") {
		t.Fatalf("climbing target not refused: %v", err)
	}
	dup := []application.EnvironmentFile{{Path: "a", Contents: "x"}, {Path: "a", Contents: "y"}}
	if err := store.Create(regular, dup); err == nil || !strings.Contains(err.Error(), "duplicate environment target") {
		t.Fatalf("duplicate create not refused: %v", err)
	}
	write(t, regular, "blocked", "a file, not a directory\n")
	blocked := []application.EnvironmentFile{{Path: "blocked/x", Contents: "x"}}
	if err := store.Create(regular, blocked); err == nil || !strings.Contains(err.Error(), "cannot create environment parent") {
		t.Fatalf("blocked parent not refused: %v", err)
	}
	partial := []application.EnvironmentFile{{Path: "good", Contents: "x"}, {Path: "blocked/x", Contents: "x"}}
	if err := store.Create(regular, partial); err == nil {
		t.Fatal("partial transaction was accepted")
	}
	if _, statErr := os.Lstat(filepath.Join(regular, "good")); statErr == nil {
		t.Fatal("partial transaction left its first install behind")
	}

	if err := os.Mkdir(filepath.Join(regular, "adir"), 0o755); err != nil {
		t.Fatal(err)
	}
	dirTarget := []application.EnvironmentRestoreFile{{Path: "adir", Contents: "x", Replace: true}}
	if err := store.Restore(regular, dirTarget); err == nil || !strings.Contains(err.Error(), "not a regular non-link file") {
		t.Fatalf("directory target not refused: %v", err)
	}
	if err := store.Restore(regular, []application.EnvironmentRestoreFile{{Path: "", Contents: "x", Replace: true}}); err == nil || !strings.Contains(err.Error(), "invalid environment target") {
		t.Fatalf("empty restore not refused: %v", err)
	}
	dupRestore := []application.EnvironmentRestoreFile{{Path: "a", Contents: "x", Replace: true}, {Path: "a", Contents: "y", Replace: true}}
	if err := store.Restore(regular, dupRestore); err == nil || !strings.Contains(err.Error(), "duplicate environment target") {
		t.Fatalf("duplicate restore not refused: %v", err)
	}
	rollback := []application.EnvironmentRestoreFile{{Path: "rolled", Contents: "x", Replace: true}, {Path: "blocked/x", Contents: "x", Replace: true}}
	if err := store.Restore(regular, rollback); err == nil {
		t.Fatal("blocked restore was accepted")
	}
	if _, statErr := os.Lstat(filepath.Join(regular, "rolled")); statErr == nil {
		t.Fatal("rolled-back restore left its first install behind")
	}

	linked := t.TempDir()
	write(t, linked, "real/x", "x\n")
	if err := os.Symlink(filepath.Join(linked, "real"), filepath.Join(linked, "linked")); err != nil {
		t.Skip("symlinks unavailable")
	}
	if err := store.Create(linked, []application.EnvironmentFile{{Path: "linked/x", Contents: "x"}}); err == nil || !strings.Contains(err.Error(), "crosses a symbolic link") {
		t.Fatalf("linked target not refused: %v", err)
	}
}

func TestDiskTreeIndexedFilesReadsOnlyTheGitIndex(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Skip("git unavailable")
	}
	write(t, dir, "staged.md", "staged\n")
	write(t, dir, "unstaged.md", "unstaged\n")
	if err := exec.Command("git", "-C", dir, "add", "staged.md").Run(); err != nil {
		t.Skip("git staging unavailable")
	}
	tree := &runtime.DiskTree{Dir: dir}

	staged, err := tree.IndexedFiles()
	if err != nil || len(staged) != 1 || staged[0] != "staged.md" {
		t.Fatalf("index not read: %v %v", staged, err)
	}

	plain := t.TempDir()
	bare := &runtime.DiskTree{Dir: plain}
	if _, err := bare.IndexedFiles(); err == nil || !strings.Contains(err.Error(), "Git refused") {
		t.Fatalf("non-repository index did not refuse: %v", err)
	}
}

func TestDiskTreeSkipsSymlinks(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "real/a.md", "x\n")
	if err := os.Symlink(filepath.Join(dir, "real"), filepath.Join(dir, "linked")); err != nil {
		t.Skip("symlinks unavailable")
	}
	tree := &runtime.DiskTree{Dir: dir}
	for _, file := range tree.Files() {
		if len(file) >= 6 && file[:6] == "linked" {
			t.Fatalf("walk followed a link: %v", tree.Files())
		}
	}
	if _, readErr := tree.Read("linked"); readErr == nil {
		t.Fatal("link was read")
	}
}
