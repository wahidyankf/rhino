package config_test

import (
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/config"
)

const gateEnvConfig = "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      command:\n        executable: ./check\n        environment:\n          RHINO_CHECK_MESSAGE: hello\n      run-on:\n        pre-commit: {}\n        pull-request:\n          reason: replay check\n  composition:\n    pull-request:\n      relation: exact\n"

func TestGateEnvironProjectsLiteralEnvironment(t *testing.T) {
	document, err := config.Parse(gateEnvConfig)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	environment, err := document.GatesOn("pre-commit")[0].Environ()
	if err != nil {
		t.Fatalf("environ failed: %v", err)
	}
	if len(environment) != 1 || environment["RHINO_CHECK_MESSAGE"] != "hello" {
		t.Fatalf("unexpected projected environment %v", environment)
	}
}

func TestGateEnvironRefusesUnresolvedProjection(t *testing.T) {
	document, err := config.Parse("schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      command:\n        executable: ./check\n        environment:\n          RHINO_CHECK_MESSAGE: {input: message.text}\n      run-on:\n        pre-commit: {}\n        pull-request:\n          reason: replay check\n  composition:\n    pull-request:\n      relation: exact\n")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if _, err := document.GatesOn("pre-commit")[0].Environ(); err == nil {
		t.Fatal("unresolved environment projection was not refused")
	}
}
