package config_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/config"
)

// mutationGate builds one configuration with a single gate entry body, so each
// case changes only the lines it is about.
func mutationGate(entry string) string {
	return "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: format-staged\n" + entry +
		"  composition:\n    pull-request:\n      relation: exact\n"
}

const mutationCommand = "      command:\n        executable: ./format\n"

const mutationRunOn = "      run-on:\n        pre-commit: {}\n        pull-request: {}\n"

// TestMutationGateDecodesItsPairedContract proves a complete mutation gate
// parses, keeps its kind, and exposes both halves of its contract.
func TestMutationGateDecodesItsPairedContract(t *testing.T) {
	text := mutationGate("      type: mutation\n      mutation:\n        local: apply-index\n        ci: verify-clean\n" +
		mutationCommand + mutationRunOn)
	document, err := config.Parse(text)
	if err != nil {
		t.Fatalf("paired mutation gate refused: %v", err)
	}
	gates := document.GatesOn("pre-commit")
	if len(gates) != 1 || gates[0].Kind != "mutation" || gates[0].Mutation == nil {
		t.Fatalf("mutation contract not decoded: %+v", gates)
	}
	if gates[0].Mutation.Local != "apply-index" || gates[0].Mutation.CI != "verify-clean" {
		t.Fatalf("contract halves wrong: %+v", gates[0].Mutation)
	}
}

// TestMutationGateRefusesMalformedContractShapes proves a contract missing a
// half, naming an unknown variant, or carrying an unknown field is refused in
// the serde wording the oracle uses.
func TestMutationGateRefusesMalformedContractShapes(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"missing pull-request replay", mutationGate("      type: mutation\n      mutation:\n        local: apply-index\n" +
			mutationCommand + mutationRunOn), "missing field `ci`"},
		{"missing local operation", mutationGate("      type: mutation\n      mutation:\n        ci: verify-clean\n" +
			mutationCommand + mutationRunOn), "missing field `local`"},
		{"unknown local operation", mutationGate("      type: mutation\n      mutation:\n        local: rewrite\n        ci: verify-clean\n" +
			mutationCommand + mutationRunOn), "unknown variant `rewrite`, expected `apply-index`"},
		{"unknown replay operation", mutationGate("      type: mutation\n      mutation:\n        local: apply-index\n        ci: publish\n" +
			mutationCommand + mutationRunOn), "unknown variant `publish`, expected `verify-clean`"},
		{"unknown contract field", mutationGate("      type: mutation\n      mutation:\n        local: apply-index\n        ci: verify-clean\n        extra: yes\n" +
			mutationCommand + mutationRunOn), "unknown field `extra`, expected one of `local`, `ci`"},
		{"unknown gate type", mutationGate("      type: fancy\n" + mutationCommand + mutationRunOn), "unknown variant `fancy`, expected one of `check`, `mutation`"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.Parse(tc.text)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
}

// TestMutationGateRefusesAMismatchedKindOrLifecycle proves a check cannot carry
// a mutation contract, a mutation needs one, and a mutation runs only on its
// closed pre-commit and pull-request lifecycle.
func TestMutationGateRefusesAMismatchedKindOrLifecycle(t *testing.T) {
	contract := "      mutation:\n        local: apply-index\n        ci: verify-clean\n"
	cases := []struct {
		name string
		text string
		want string
	}{
		{"check with contract", mutationGate("      type: check\n" + contract + mutationCommand + mutationRunOn),
			"format-staged: is a check but declares a mutation contract; use `type: mutation`"},
		{"untyped gate with contract", mutationGate(contract + mutationCommand + mutationRunOn),
			"format-staged: is a check but declares a mutation contract; use `type: mutation`"},
		{"mutation without contract", mutationGate("      type: mutation\n" + mutationCommand + mutationRunOn),
			"format-staged: is a mutation but does not declare paired `apply-index` and `verify-clean` behavior"},
		{"mutation without pull-request replay", mutationGate("      type: mutation\n" + contract + mutationCommand +
			"      run-on:\n        pre-commit: {}\n"), "format-staged: is a mutation and must run at both pre-commit and pull-request"},
		{"mutation on another surface", mutationGate("      type: mutation\n" + contract + mutationCommand +
			"      run-on:\n        pre-commit: {}\n        pull-request: {}\n        main: {}\n"),
			"format-staged: is a mutation outside its closed pre-commit and pull-request lifecycle"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.Parse(tc.text)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
}
