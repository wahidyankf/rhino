package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// argvGateConfig declares one pre-commit check whose argv is the given items.
func argvGateConfig(items string) string {
	return "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      command:\n        executable: ./check\n        args:\n" +
		items +
		"      run-on:\n        pre-commit: {}\n        pull-request:\n          reason: replay check\n  composition:\n    pull-request:\n      relation: exact\n"
}

// TestGateArgvRefusesAScalarOfTheWrongType proves an argv item that is not a
// mapping refuses with the serde-style type message for its own YAML type, so
// the mistake is named at the item that holds it.
func TestGateArgvRefusesAScalarOfTheWrongType(t *testing.T) {
	cases := []struct {
		name   string
		item   string
		reason string
	}{
		{"an integer item", "          - 1\n", "gates.entries[0].command.args[0]: invalid type: integer `1`, expected struct Argument"},
		{"a float item", "          - 1.5\n", "invalid type: floating point `1.5`, expected struct Argument"},
		{"a boolean item", "          - true\n", "invalid type: boolean `true`, expected struct Argument"},
		{"a null item", "          - ~\n", "invalid type: unit value, expected struct Argument"},
		{"a sequence item", "          - [a]\n", "invalid type: sequence, expected struct Argument"},
		{"a string item with a backslash", `          - "a\\b"` + "\n", `invalid type: string "a\\b", expected struct Argument`},
		{"a string item with a line feed", `          - "a\nb"` + "\n", `invalid type: string "a\nb", expected struct Argument`},
		{"a string item with a carriage return", `          - "a\rb"` + "\n", `invalid type: string "a\rb", expected struct Argument`},
		{"a string item with a tab", `          - "a\tb"` + "\n", `invalid type: string "a\tb", expected struct Argument`},
		{"a string item with a control character", `          - "a\x07b"` + "\n", `invalid type: string "a\u{7}b", expected struct Argument`},
		{"an aliased item that is not a mapping", "          - &one {literal: x}\n          - *one\n", "args[1]: invalid type: sequence, expected struct Argument"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := gateTree(argvGateConfig(tc.item))
			outcome := application.Execute(tree, args("gate", "validate"), nil)
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
		})
	}
}

// TestGateArgvRefusesAMalformedMappingItem proves an argv mapping whose input is
// not a name, or whose expansion is not a word, refuses with the reason for the
// shape it breaks, so no projection runs from a half-read item.
func TestGateArgvRefusesAMalformedMappingItem(t *testing.T) {
	cases := []struct {
		name   string
		item   string
		reason string
	}{
		{"an input that is not a name omits the expansion mode", "          - {input: 1}\n", "omits the closed expansion mode from an input argv item"},
		{"an expansion that is not a word names no known variant", "          - {input: range.base, expand: 1}\n", "unknown variant"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := gateTree(argvGateConfig(tc.item))
			outcome := application.Execute(tree, args("gate", "validate"), nil)
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
		})
	}
}

// TestGateArgvThroughAnAliasedCommandRefusesAnUntypedItem proves a command
// reached through a YAML alias is read without the argv shape check, and that
// the untyped items it holds still refuse, rather than running with a
// malformed argument.
func TestGateArgvThroughAnAliasedCommandRefusesAnUntypedItem(t *testing.T) {
	for _, item := range []string{"[x]", "[1]"} {
		config := "schema: rhino/repo-config/v2\nextensions:\n  tool: &cmd\n    executable: ./check\n    args: " + item +
			"\ngates:\n  entries:\n    - id: check\n      command: *cmd\n      run-on:\n        pre-commit: {}\n        pull-request:\n          reason: replay check\n  composition:\n    pull-request:\n      relation: exact\n"
		tree := gateTree(config)
		outcome := application.Execute(tree, args("gate", "validate"), nil)
		if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, "must hold exactly one literal or typed input argv item") {
			t.Fatalf("argv item %s through an alias not refused: %+v", item, outcome)
		}
	}
}

// TestGateValidationRefusesAnUnusableCommandOrBinding proves a gate whose
// executable is blank, whose environment names an undeclared input, or whose
// surface leaves a declared input unbound refuses before any gate runs.
func TestGateValidationRefusesAnUnusableCommandOrBinding(t *testing.T) {
	const head = "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      command:\n"
	const tail = "      run-on:\n        pre-commit: {}\n        pull-request:\n          reason: replay check\n  composition:\n    pull-request:\n      relation: exact\n"
	cases := []struct {
		name   string
		config string
		reason string
	}{
		{
			name:   "a blank executable refuses",
			config: head + "        executable: \" \"\n" + tail,
			reason: "has an empty or line-breaking executable",
		},
		{
			name:   "an environment projection of an undeclared input refuses",
			config: head + "        executable: ./check\n        environment:\n          NAME: { input: nope.paths }\n" + tail,
			reason: "nope.paths",
		},
		{
			name: "a declared input that a surface leaves unbound refuses",
			config: "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: range-check\n      type: check\n      inputs:\n        range: { kind: commit-range }\n" +
				"      command:\n        executable: ./check\n      run-on:\n        pre-commit: {}\n        pull-request:\n          reason: replay check\n" +
				"          bind:\n            range: { source: explicit-range, range: explicit }\n  composition:\n    pull-request:\n      relation: exact\n",
			reason: "has no binding for `range-check` at pre-commit",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := gateTree(tc.config)
			outcome := application.Execute(tree, args("gate", "validate"), nil)
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
		})
	}
}

// TestPushFallbackRefusesAnUnsafeRef proves a push-updates fallback ref that
// holds whitespace or a control character is refused, because the oracle reads
// the ref as a single token.
func TestPushFallbackRefusesAnUnsafeRef(t *testing.T) {
	for _, fallback := range []string{`"origin main"`, `"refs/a\tb"`, `"refs/a\x01b"`, `"refs/a\u0085b"`, `"refs/a b"`} {
		config := strings.Replace(pushGateConfig, "refs/remotes/origin/main", fallback, 1)
		tree := gateTree(config)
		outcome := application.Execute(tree, args("gate", "validate"), nil)
		if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, "declares an invalid push-update fallback ref") {
			t.Fatalf("fallback %s not refused: %+v", fallback, outcome)
		}
	}
}

// TestGateWithNoRunOnRunsOnNoSurface proves a gate that declares no run-on
// surface is never launched by a surface run, while a gate that names the
// surface still is.
func TestGateWithNoRunOnRunsOnNoSurface(t *testing.T) {
	config := "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: check\n      command:\n        executable: ./check\n      run-on:\n        pre-commit: {}\n        pull-request:\n          reason: replay check\n" +
		"    - id: idle\n      command:\n        executable: ./idle\n  composition:\n    pull-request:\n      relation: exact\n"
	tree := gateTree(config)
	launcher := &stubLauncher{}
	outcome := application.ExecuteWithLauncher(tree, args("gate", "run", "--surface", "pre-commit"), nil, launcher)
	if outcome.ExitCode != 0 {
		t.Fatalf("pre-commit run refused: %+v", outcome)
	}
	if len(launcher.seen) != 1 || launcher.seen[0][0] != "./check" {
		t.Fatalf("expected only ./check to launch, got %v", launcher.seen)
	}
}
