package config_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/config"
)

func TestParseAcceptsGroupedDocument(t *testing.T) {
	document, err := config.Parse("schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    heading-hierarchy:\n      surfaces: []\n")
	if err != nil {
		t.Fatalf("grouped document refused: %v", err)
	}
	if !document.PolicyOwner("policies.markdown.heading-hierarchy") {
		t.Fatal("declared owner not found")
	}
	if document.PolicyOwner("policies.markdown.naming") {
		t.Fatal("undeclared owner reported as declared")
	}
}

func TestParseRefusesBadShapes(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"missing schema", "policies: {}\n", "declares no schema"},
		{"predecessor comment", "# schema: rhino/repo-config/v1\n", "no longer accepts predecessor"},
		{"unknown schema", "schema: rhino/repo-config/v9\n", "unrecognized schema"},
		{"unknown core key", "schema: rhino/repo-config/v2\nnope: {}\n", "unknown core key"},
		{"bad owner", "schema: rhino/repo-config/v2\nextensions:\n  Bad!: {}\n", "extension owner"},
		{"retired ci", "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: a\n      run-on:\n        ci: {}\n", "retired"},
		{"at-least extra", "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: local\n      run-on:\n        pre-commit: {}\n        pull-request: {}\n    - id: extra\n      run-on:\n        pull-request: {}\n  composition:\n    pull-request:\n      relation: at-least\n", "non-empty reason"},
	}
	for _, tc := range cases {
		if _, err := config.Parse(tc.text); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: got %v, want %q", tc.name, err, tc.want)
		}
	}
}

func TestParseRefusesQualityCycleCeilingOutsideOneToThree(t *testing.T) {
	base := "schema: rhino/repo-config/v2\npolicies:\n  governance:\n    quality-gates:\n      root: flows\n      gates: []\n"
	for _, ceiling := range []string{"0", "4", "9", "-1"} {
		text := base + "      defaults:\n        max-cycles: " + ceiling + "\n"
		if _, err := config.Parse(text); err == nil || !strings.Contains(err.Error(), "policies.governance.quality-gates.defaults.max-cycles") {
			t.Fatalf("ceiling %s: got %v", ceiling, err)
		}
	}
	for _, ceiling := range []string{"1", "2", "3"} {
		text := base + "      defaults:\n        max-cycles: " + ceiling + "\n"
		if _, err := config.Parse(text); err != nil {
			t.Fatalf("ceiling %s refused: %v", ceiling, err)
		}
	}
	if _, err := config.Parse(base); err != nil {
		t.Fatalf("absent ceiling refused: %v", err)
	}
}

func TestPolicyGroupAndGateIDs(t *testing.T) {
	document, err := config.Parse("schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n      surfaces: []\n")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if _, ok := document.PolicyGroup("markdown", "naming"); !ok {
		t.Fatal("declared group not found")
	}
	if _, ok := document.PolicyGroup("markdown", "missing"); ok {
		t.Fatal("missing name found")
	}
	if _, ok := document.PolicyGroup("missing", "naming"); ok {
		t.Fatal("missing group found")
	}
	if ids := document.GateIDsOn("pre-commit"); len(ids) != 0 {
		t.Fatalf("unexpected ids: %v", ids)
	}
}

func TestGateArgvProjectsLiteralsOnly(t *testing.T) {
	document, err := config.Parse("schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      command:\n        executable: ./rhino\n        args:\n          - repo-config\n          - {literal: validate}\n      run-on:\n        pre-commit: {}\n        pull-request:\n          reason: replay check\n  composition:\n    pull-request:\n      relation: exact\n")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	gates := document.GatesOn("pre-commit")
	if len(gates) != 1 {
		t.Fatalf("expected one gate, got %d", len(gates))
	}
	argv, err := gates[0].Argv()
	if err != nil {
		t.Fatalf("argv failed: %v", err)
	}
	if len(argv) != 3 || argv[0] != "./rhino" || argv[2] != "validate" {
		t.Fatalf("unexpected argv %v", argv)
	}
	bad, err := config.Parse("schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      command:\n        executable: ./x\n        args:\n          - {input: repository.root}\n      run-on:\n        pre-commit: {}\n        pull-request:\n          reason: replay check\n  composition:\n    pull-request:\n      relation: exact\n")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if _, err := bad.GatesOn("pre-commit")[0].Argv(); err == nil {
		t.Fatal("unresolved input projected without refusal")
	}
}
