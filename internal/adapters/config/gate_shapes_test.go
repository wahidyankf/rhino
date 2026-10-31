package config_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/config"
)

// TestGateShapesRefuseOracleTypes proves every non-struct argv shape is
// refused with the oracle's serde wording, covering each type branch.
func TestGateShapesRefuseOracleTypes(t *testing.T) {
	base := func(arg string) string {
		return "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      type: check\n      command:\n        executable: ./check\n        args:\n          - " + arg + "\n      run-on:\n        pre-commit: {}\n        pull-request:\n          reason: replay\n  composition:\n    pull-request:\n      relation: exact\n"
	}
	cases := []struct {
		name string
		arg  string
		want string
	}{
		{"int", "42", "invalid type: integer `42`, expected struct Argument at line 9 column 13"},
		{"float", "3.14", "invalid type: floating point `3.14`, expected struct Argument at line 9 column 13"},
		{"bool", "true", "invalid type: boolean `true`, expected struct Argument at line 9 column 13"},
		{"null", "null", "invalid type: unit value, expected struct Argument at line 9 column 13"},
		{"sequence", "[a]", "invalid type: sequence, expected struct Argument at line 9 column 13"},
		{"quoted", `'he"llo'`, "invalid type: string \"he\\\"llo\", expected struct Argument at line 9 column 13"},
		{"unknown field", "{bogus: 1}", "unknown field `bogus`, expected one of `literal`, `input`, `expand` at line 9 column 14"},
	}
	for _, tc := range cases {
		_, err := config.Parse(base(tc.arg))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: got %v, want %q", tc.name, err, tc.want)
		}
	}
}

// TestGateEnvShapesRefuseOracleTypes proves every non-struct environment
// shape is refused with the oracle's serde wording.
func TestGateEnvShapesRefuseOracleTypes(t *testing.T) {
	base := func(value string) string {
		return "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      type: check\n      command:\n        executable: ./check\n        environment:\n          HELLO: " + value + "\n      run-on:\n        pre-commit: {}\n        pull-request:\n          reason: replay\n  composition:\n    pull-request:\n      relation: exact\n"
	}
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"int", "42", "invalid type: integer `42`, expected struct EnvironmentProjection at line 9 column 18"},
		{"bool", "true", "invalid type: boolean `true`, expected struct EnvironmentProjection at line 9 column 18"},
		{"null", "null", "invalid type: unit value, expected struct EnvironmentProjection at line 9 column 18"},
		{"sequence", "[a]", "invalid type: sequence, expected struct EnvironmentProjection at line 9 column 18"},
		{"empty mapping", "{}", "missing field `input` at line 9 column 18"},
		{"extra field", "{input: a.b, extra: 1}", "unknown field `extra`, expected `input` at line 9 column 31"},
	}
	for _, tc := range cases {
		_, err := config.Parse(base(tc.value))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: got %v, want %q", tc.name, err, tc.want)
		}
	}
}

// TestGateFallbackWhitespaceVariants proves whitespace fallbacks are refused,
// covering each whitespace branch.
func TestGateFallbackWhitespaceVariants(t *testing.T) {
	base := func(fallback string) string {
		return "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: range-check\n      type: check\n      inputs:\n        range: { kind: commit-range }\n      command:\n        executable: ./check\n        args:\n          - { input: range.base, expand: single }\n      run-on:\n        pre-push:\n          bind:\n            range: { source: push-updates, fallback: " + fallback + " }\n        pull-request:\n          bind:\n            range: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: exact\n"
	}
	for _, tc := range []struct {
		name     string
		fallback string
	}{
		{"space", `"a b"`},
		{"tab", `"a\tb"`},
		{"empty", `""`},
		{"dash", `"-bad"`},
	} {
		_, err := config.Parse(base(tc.fallback))
		if err == nil {
			t.Fatalf("%s: fallback accepted", tc.name)
		}
	}
}

// TestGateSurfacesOrderFallbackValidation proves fallback validation visits
// every known surface, covering the surface order branches.
func TestGateSurfacesOrderFallbackValidation(t *testing.T) {
	_, err := config.Parse("schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      type: check\n      command:\n        executable: ./check\n      run-on:\n        pre-commit: {}\n        commit-msg: {}\n        pre-push: {}\n        pull-request: {}\n        main: {}\n        scheduled: {}\n        manual: {}\n        custom: {}\n  composition:\n    pull-request:\n      relation: exact\n")
	if err != nil {
		t.Fatalf("all-surface gate refused: %v", err)
	}
}
