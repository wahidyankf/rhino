package config_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/config"
)

const typedGateConfig = "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: range-check\n      type: check\n      inputs:\n        range: { kind: commit-range }\n      command:\n        executable: npm\n        args:\n          - { literal: exec }\n          - { literal: -- }\n          - { input: range.base, expand: single }\n          - { input: range.head, expand: single }\n        environment:\n          NX_BASE: { input: range.base }\n          NX_HEAD: { input: range.head }\n      run-on:\n        pre-push:\n          bind:\n            range: { source: push-updates, fallback: refs/remotes/origin/main }\n        pull-request:\n          bind:\n            range: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: exact\n"

// TestGateDecodeReadsTypedInputsCommandAndBinds proves the declaration keeps
// every typed gate field the run resolves: the kind, the input kinds, the
// argv items, the environment projections, and the per-surface binds.
func TestGateDecodeReadsTypedInputsCommandAndBinds(t *testing.T) {
	document, err := config.Parse(typedGateConfig)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	gates := document.GatesOn("pre-push")
	if len(gates) != 1 {
		t.Fatalf("expected one gate, got %d", len(gates))
	}
	gate := gates[0]
	if gate.Kind != "check" {
		t.Fatalf("unexpected gate kind %q", gate.Kind)
	}
	input, ok := gate.Inputs["range"]
	if !ok || input.Kind != "commit-range" {
		t.Fatalf("unexpected inputs %v", gate.Inputs)
	}
	if gate.Executable != "npm" {
		t.Fatalf("unexpected executable %q", gate.Executable)
	}
	if len(gate.Args) != 4 {
		t.Fatalf("unexpected args %v", gate.Args)
	}
	if !gate.Args[0].HasLiteral || gate.Args[0].Literal != "exec" {
		t.Fatalf("first arg not a literal: %+v", gate.Args[0])
	}
	if gate.Args[2].Input != "range.base" || gate.Args[2].Expand != "single" {
		t.Fatalf("third arg not a single projection: %+v", gate.Args[2])
	}
	base, ok := gate.Environment["NX_BASE"]
	if !ok || base.Input != "range.base" {
		t.Fatalf("unexpected environment %v", gate.Environment)
	}
	bind, ok := gate.RunOn["pre-push"].Bind["range"]
	if !ok || bind.Source != "push-updates" {
		t.Fatalf("unexpected pre-push bind %v", gate.RunOn["pre-push"].Bind)
	}
	if bind.Fallback == nil || *bind.Fallback != "refs/remotes/origin/main" {
		t.Fatalf("fallback not kept: %+v", bind)
	}
	pullBind, ok := gate.RunOn["pull-request"].Bind["range"]
	if !ok || pullBind.Source != "explicit-range" || pullBind.Range != "explicit" {
		t.Fatalf("unexpected pull-request bind %v", gate.RunOn["pull-request"].Bind)
	}
}

// TestGateDecodeRefusesPlainArgs proves a plain-string argv item is refused
// exactly like the v0.15.0 oracle, which accepts only `{literal}` mappings.
func TestGateDecodeRefusesPlainArgs(t *testing.T) {
	_, err := config.Parse("schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      type: check\n      command:\n        executable: ./rhino\n        args:\n          - repo-config\n          - {literal: validate}\n      run-on:\n        pre-commit: {}\n        pull-request:\n          reason: replay check\n  composition:\n    pull-request:\n      relation: exact\n")
	if err == nil || !strings.Contains(err.Error(), "gates.entries[0].command.args[0]: invalid type: string \"repo-config\", expected struct Argument at line 9 column 13") {
		t.Fatalf("plain arg not refused like the oracle: %v", err)
	}
}

// TestGateDecodeRefusesLiteralEnvironment proves a plain-string environment
// value is refused exactly like the oracle.
func TestGateDecodeRefusesLiteralEnvironment(t *testing.T) {
	_, err := config.Parse("schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      type: check\n      command:\n        executable: ./rhino\n        args:\n          - {literal: validate}\n        environment:\n          RHINO_CHECK_MESSAGE: hello\n      run-on:\n        pre-commit: {}\n        pull-request:\n          reason: replay check\n  composition:\n    pull-request:\n      relation: exact\n")
	if err == nil || !strings.Contains(err.Error(), "gates.entries[0].command.environment.RHINO_CHECK_MESSAGE: invalid type: string \"hello\", expected struct EnvironmentProjection at line 11 column 32") {
		t.Fatalf("literal env not refused like the oracle: %v", err)
	}
}

// TestGateDecodeRefusesPushWithoutFallback proves a push-updates bind
// without its declared fallback is refused exactly like the v0.15.0 oracle.
func TestGateDecodeRefusesPushWithoutFallback(t *testing.T) {
	_, err := config.Parse("schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: range-check\n      type: check\n      inputs:\n        range: { kind: commit-range }\n      command:\n        executable: ./check\n        args:\n          - { input: range.base, expand: single }\n      run-on:\n        pre-push:\n          bind:\n            range: { source: push-updates }\n        pull-request:\n          bind:\n            range: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: exact\n")
	if err == nil || !strings.Contains(err.Error(), "line 1: range: uses `push-updates` without a declared fallback ref") {
		t.Fatalf("missing fallback not refused like the oracle: %v", err)
	}
}

// TestGateDecodeRefusesInvalidPushFallback proves a push-updates bind with
// a leading-dash fallback is refused exactly like the oracle.
func TestGateDecodeRefusesInvalidPushFallback(t *testing.T) {
	_, err := config.Parse("schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: range-check\n      type: check\n      inputs:\n        range: { kind: commit-range }\n      command:\n        executable: ./check\n        args:\n          - { input: range.base, expand: single }\n      run-on:\n        pre-push:\n          bind:\n            range: { source: push-updates, fallback: \"-bad\" }\n        pull-request:\n          bind:\n            range: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: exact\n")
	if err == nil || !strings.Contains(err.Error(), "line 1: range: declares an invalid push-update fallback ref") {
		t.Fatalf("invalid fallback not refused like the oracle: %v", err)
	}
}

// TestGateDecodeRefusesFallbackOnWrongSource proves a fallback on a
// non-push-updates source is refused exactly like the oracle.
func TestGateDecodeRefusesFallbackOnWrongSource(t *testing.T) {
	_, err := config.Parse("schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: files-check\n      type: check\n      inputs:\n        files: { kind: files }\n      command:\n        executable: ./check\n        args:\n          - { input: files.paths, expand: repeat }\n      run-on:\n        pre-commit:\n          bind:\n            files: { source: git-index, fallback: refs/main }\n        pull-request:\n          bind:\n            files: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: exact\n")
	if err == nil || !strings.Contains(err.Error(), "line 1: files: uses `fallback` with a source other than `push-updates`") {
		t.Fatalf("misplaced fallback not refused like the oracle: %v", err)
	}
}

// TestGateDecodeRefusesLiteralArgvWithExpand proves a literal argv item that
// also names an expansion is refused at parse time, as the oracle refuses it,
// before any gate can dispatch.
func TestGateDecodeRefusesLiteralArgvWithExpand(t *testing.T) {
	_, err := config.Parse("schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      type: check\n      command:\n        executable: ./check\n        args:\n          - {literal: labelled, expand: single}\n      run-on:\n        pre-commit: {}\n        pull-request:\n          reason: replay check\n  composition:\n    pull-request:\n      relation: exact\n")
	if err == nil || !strings.Contains(err.Error(), "line 1: args: adds `expand` to a literal argv item") {
		t.Fatalf("literal argv with expand not refused like the oracle: %v", err)
	}
}
