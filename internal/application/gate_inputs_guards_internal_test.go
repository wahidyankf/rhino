package application

import (
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/config"
)

// Config validation refuses each reference below before a gate runs, so these
// guards are exercised on the helpers directly. Each test asserts the refusal
// reason and that a resolvable reference still resolves.

func TestReferencedValuesRefusesAReferenceItCannotResolve(t *testing.T) {
	inputs := map[string]gateResolvedInput{
		"files": {kind: "files", files: []string{"a.md"}},
		"range": {kind: "range", base: "base-sha", head: "head-sha"},
	}
	values, reason := referencedValues(inputs, "files.paths")
	if reason != "" || len(values) != 1 || values[0] != "a.md" {
		t.Fatalf("a resolvable reference did not resolve: %q, %q", values, reason)
	}
	for _, tc := range []struct {
		reference string
		want      string
	}{
		{"files", "`files` is not an input field reference"},
		{"absent.paths", "`absent.paths` was not resolved for this surface"},
		{"range.paths", "`range.paths` is not available on its resolved input"},
	} {
		values, reason := referencedValues(inputs, tc.reference)
		if reason != tc.want || values != nil {
			t.Fatalf("%s: got %q, %q; want refusal %q", tc.reference, values, reason, tc.want)
		}
	}
}

func TestResolveGateInputRefusesAnUnknownKind(t *testing.T) {
	_, refusal, ok := resolveGateInput("no-such-kind", config.GateBinding{}, nil, nil, nil, nil)
	if ok || refusal == nil || refusal.reason != "has an invalid typed input binding" {
		t.Fatalf("unknown input kind not refused: %+v, %v", refusal, ok)
	}
}

func TestProjectGateCommandRefusesAnUnresolvableProjection(t *testing.T) {
	inputs := map[string]gateResolvedInput{"files": {kind: "files", files: []string{"a.md"}}}
	argv, env, reason := projectGateCommand("./check", []config.GateArg{{}}, nil, inputs)
	if argv != nil || env != nil || reason != "has an invalid argv item after validation" {
		t.Fatalf("empty argv item projected: %v, %v, %q", argv, env, reason)
	}
	argv, _, reason = projectGateCommand("./check",
		[]config.GateArg{{HasInput: true, Input: "absent.paths", Expand: "single"}}, nil, inputs)
	if argv != nil || reason != "`absent.paths` was not resolved for this surface" {
		t.Fatalf("argv item over an unresolved input projected: %v, %q", argv, reason)
	}
	_, env, reason = projectGateCommand("./check", nil,
		map[string]config.GateEnv{"NAME": {Input: "absent.value"}}, inputs)
	if env != nil || reason != "`absent.value` was not resolved for this surface" {
		t.Fatalf("environment over an unresolved input projected: %v, %q", env, reason)
	}
}

func TestParsePushRangeRefusesANewRefWithoutAFallback(t *testing.T) {
	const local = "1234567890abcdef1234567890abcdef12345678"
	const zero = "0000000000000000000000000000000000000000"
	stdin := "refs/heads/a " + local + " refs/heads/a " + zero + "\n"
	if _, reason := parsePushRange(stdin, nil); reason != "received a new push ref without a declared fallback ref" {
		t.Fatalf("new ref without a fallback not refused: %q", reason)
	}
	fallback := "refs/remotes/origin/main"
	got, reason := parsePushRange(stdin, &fallback)
	if reason != "" || !got.newRef || got.fallback != fallback || got.head != local {
		t.Fatalf("new ref with a fallback did not resolve: %+v, %q", got, reason)
	}
}
