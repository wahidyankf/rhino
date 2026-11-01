package application_test

import (
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// TestHarnessValidateNeverStartsTheModelResolver hands validation the same
// resolver generation uses. Generation must start it, which is the positive
// control; validation, given the same tree and resolver, must start nothing.
func TestHarnessValidateNeverStartsTheModelResolver(t *testing.T) {
	canonical := "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints, tier: tier}\n  profiles:\n    - id: alpha\n      agent-adapter: {path: 'adapters/alpha/{name}.md', format: front-matter, route-field: body, route: 'See {path}.', tier-fields: {model: model, effort: effort}}\n      tiers: {fast: {model-resolve: {command: [list-models], pattern: 'model-*', fallback: model-0.9}, effort: low}}\n"
	tree := harnessTree(canonical)
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files[".agents/agents/reviewer.md"] = "---\nname: reviewer\ndescription: Review.\ntier: fast\n---\n"
	resolver := &stubModelResolver{outputs: []string{"model-1.9 model-1.10"}}

	validated := application.ExecuteWithHarness(tree, args("harness", "adapters", "validate"), nil, nil, nil, nil, nil, resolver)
	if len(resolver.launches) != 0 {
		t.Fatalf("validate started the resolver %d times (exit %d), want none", len(resolver.launches), validated.ExitCode)
	}

	generated := runHarnessGenerate(tree, &memoryHarnessStore{tree: tree}, resolver, "harness", "adapters", "generate")
	if generated.ExitCode != 0 || len(resolver.launches) != 1 {
		t.Fatalf("control: generate exit %d with %d launches, want exit 0 and one launch", generated.ExitCode, len(resolver.launches))
	}
}
