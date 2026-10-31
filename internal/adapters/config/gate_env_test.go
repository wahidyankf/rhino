package config_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/config"
)

// TestGateEnvironRefusesLiteralEnvironment proves a plain-string
// environment value is refused exactly like the v0.15.0 oracle, which maps
// every environment variable to one declared input field.
func TestGateEnvironRefusesLiteralEnvironment(t *testing.T) {
	_, err := config.Parse("schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      command:\n        executable: ./check\n        environment:\n          RHINO_CHECK_MESSAGE: hello\n      run-on:\n        pre-commit: {}\n        pull-request:\n          reason: replay check\n  composition:\n    pull-request:\n      relation: exact\n")
	if err == nil || !strings.Contains(err.Error(), "gates.entries[0].command.environment.RHINO_CHECK_MESSAGE: invalid type: string \"hello\", expected struct EnvironmentProjection at line 8 column 32") {
		t.Fatalf("literal env not refused like the oracle: %v", err)
	}
}

// TestGateEnvironRefusesLiteralMapping proves a `{literal}` environment
// mapping is refused with the oracle's unknown-field wording.
func TestGateEnvironRefusesLiteralMapping(t *testing.T) {
	_, err := config.Parse("schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      command:\n        executable: ./check\n        environment:\n          RHINO_CHECK_MESSAGE: {literal: hello}\n      run-on:\n        pre-commit: {}\n        pull-request:\n          reason: replay check\n  composition:\n    pull-request:\n      relation: exact\n")
	if err == nil || !strings.Contains(err.Error(), "unknown field `literal`, expected `input` at line 8 column 33") {
		t.Fatalf("literal mapping not refused like the oracle: %v", err)
	}
}

// TestGateEnvironKeepsInputProjection proves an input projection entry
// decodes as a reference the run resolves instead of a literal.
func TestGateEnvironKeepsInputProjection(t *testing.T) {
	document, err := config.Parse("schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      inputs:\n        message: { kind: commit-message }\n      command:\n        executable: ./check\n        environment:\n          RHINO_CHECK_MESSAGE: {input: message.text}\n      run-on:\n        pre-commit:\n          bind:\n            message: { source: hook-message-file }\n        pull-request:\n          reason: replay check\n          bind:\n            message: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: exact\n")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	entry, ok := document.GatesOn("pre-commit")[0].Environment["RHINO_CHECK_MESSAGE"]
	if !ok || entry.Input != "message.text" {
		t.Fatalf("projection entry not kept: %v", document.GatesOn("pre-commit")[0].Environment)
	}
}
