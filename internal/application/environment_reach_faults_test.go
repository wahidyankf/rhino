package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// unindexedTree exposes only the Tree contract, so it has no index snapshot
// boundary to read staged paths from.
type unindexedTree struct{ application.Tree }

// TestEnvironmentStagingNeedsAnIndexSnapshotBoundary proves the staging guard
// refuses a tree with no index boundary, rather than reading the working copy as
// if it were what is staged.
func TestEnvironmentStagingNeedsAnIndexSnapshotBoundary(t *testing.T) {
	inner := newMemoryTree("/root")
	inner.files["repo-config.yml"] = "schema: rhino/repo-config/v2\nenvironment:\n  staged:\n    forbidden: [\".env*\"]\n"
	outcome := application.Execute(unindexedTree{inner}, args("env", "validate"), nil)
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "environment staging guard refused: this tree has no Git index snapshot boundary") {
		t.Fatalf("an unindexed tree was staged-checked: %+v", outcome)
	}
}

// TestEnvironmentStagedAllowlistMustBeAList proves a staged allowlist that is not
// a list of paths refuses the declaration.
func TestEnvironmentStagedAllowlistMustBeAList(t *testing.T) {
	tree := envConfig("environment:\n  staged:\n    allowed: {}\n")
	outcome := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "validate")
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "names no paths") {
		t.Fatalf("a non-list staged allowlist was not refused: %+v", outcome)
	}
}

// TestEnvironmentDetectorPathsMustBeAList proves a detector whose paths are not a
// list refuses the declaration rather than reading no source.
func TestEnvironmentDetectorPathsMustBeAList(t *testing.T) {
	tree := envConfig("environment:\n  detectors:\n    - language: rust\n      paths: x\n")
	outcome := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "validate")
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "names no paths") {
		t.Fatalf("a non-list detector path was not refused: %+v", outcome)
	}
}

// TestEnvironmentInitAppliesAndReportsJSON proves a first apply writes the
// declared target and reports it as completed in the JSON form.
func TestEnvironmentInitAppliesAndReportsJSON(t *testing.T) {
	tree := envConfig("environment:\n  examples:\n    - source: .env.example\n      target: .env.generated\n")
	tree.files[".env.example"] = "EXAMPLE_KEY=placeholder\n"
	outcome := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "init", "--apply", "--output", "json")
	want := "{\"command\":\"environment-init\",\"count\":1,\"schemaVersion\":1,\"status\":\"completed\",\"targets\":[\".env.generated\"]}\n"
	if outcome.ExitCode != 0 || outcome.Stdout != want {
		t.Fatalf("applied init json differs: %+v", outcome)
	}
}

// TestEnvironmentBackupToTheRootDirectoryWritesEachTargetWhereItIs proves a
// backup whose destination is the repository root writes each file at its own
// declared path. That path already holds the file being backed up, so the write
// refuses rather than overwrite it.
func TestEnvironmentBackupToTheRootDirectoryWritesEachTargetWhereItIs(t *testing.T) {
	tree := envConfig("environment:\n  examples:\n    - source: .env.fixture\n      target: .env.fixture\n")
	tree.files[".env.fixture"] = "FIXTURE=1\n"
	outcome := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "backup", "--dir", ".")
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "environment target `.env.fixture` already exists") {
		t.Fatalf("root backup did not refuse the existing target: %+v", outcome)
	}
	if tree.files[".env.fixture"] != "FIXTURE=1\n" {
		t.Fatalf("root backup overwrote the file it backs up: %q", tree.files[".env.fixture"])
	}
}

// TestEnvironmentRestoreRefusesAnEscapingTarget proves a restore whose declared
// target leaves the repository refuses, as a backup of the same policy does.
func TestEnvironmentRestoreRefusesAnEscapingTarget(t *testing.T) {
	tree := envConfig("environment:\n  examples:\n    - source: ex\n      target: ../outside\n")
	tree.files["saved/ex"] = "x\n"
	outcome := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "restore", "--dir", "saved", "--force")
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "environment example leaves the repository root") {
		t.Fatalf("escaping restore target not refused: %+v", outcome)
	}
}
