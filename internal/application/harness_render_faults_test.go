package application_test

import (
	"strings"
	"testing"
)

// harnessRenderReviewer is one canonical agent carrying every field the render
// rows read: a tier, grants, denials, and a skill list.
const harnessRenderReviewer = "---\nname: reviewer\ndescription: Review changes thoroughly.\ntier: fast\ngrants:\n  - read\ndenials:\n  - net\nskills:\n  - tidy\n---\n"

// harnessRenderTree declares one agent adapter. agent and profile are extra
// lines for the agent adapter and for the profile itself.
func harnessRenderTree(agent, profile string) *memoryTree {
	config := "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints, tier: tier}\n" +
		"    skills: {name: name, description: description}\n  profiles:\n    - id: alpha\n      agent-adapter:\n" +
		"        path: adapters/alpha/agents/{name}.md\n        format: front-matter\n        route-field: body\n        route: \"Read {path}.\"\n" +
		"        identity: {title: name}\n" + agent + profile
	tree := harnessTree(config)
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files[".agents/agents/reviewer.md"] = harnessRenderReviewer
	return tree
}

// harnessPinnedTier declares the fast tier with a pinned model and effort, so
// tier fields render into the adapter.
const harnessPinnedTier = "      tiers: {fast: {model: model-a-1.0, effort: low}}\n"

// TestHarnessRenderRefusesATierFieldThatCollides proves a tier field, a
// translation, or an entry written onto a field another writer already holds
// refuses the run with the reason its rendering step names.
func TestHarnessRenderRefusesATierFieldThatCollides(t *testing.T) {
	cases := []struct {
		name    string
		agent   string
		profile string
		reason  string
	}{
		{
			name:    "a tier model that lands on an identity field refuses",
			agent:   "        tier-fields: {model: title, effort: effort}\n",
			profile: harnessPinnedTier,
			reason:  "adapter output repeats field `title`",
		},
		{
			name:    "a tier effort that lands on an identity field refuses",
			agent:   "        tier-fields: {model: model, effort: title}\n",
			profile: harnessPinnedTier,
			reason:  "adapter output repeats field `title`",
		},
		{
			name: "a member translation onto a tier field refuses",
			agent: "        tier-fields: {model: model, effort: effort}\n" +
				"        translations:\n          - {when: requires, capability: read, field: model, members: [read-files]}\n",
			profile: harnessPinnedTier,
			reason:  "adapter output conflicts at `model`",
		},
		{
			name: "an entry translation onto a tier field refuses",
			agent: "        tier-fields: {model: model, effort: effort}\n" +
				"        translations:\n          - {when: requires, capability: read, field: model, entries: {host: example}}\n",
			profile: harnessPinnedTier,
			reason:  "adapter output conflicts at `model`",
		},
		{
			name:   "absent members removed from a field that is not written refuse",
			agent:  "        translations:\n          - {when: requires, capability: read, field: access, absent-members: [read-files]}\n",
			reason: "adapter output cannot remove members from undeclared field `access`",
		},
		{
			name:   "an absent field that the source writes refuses",
			agent:  "        absent: [title]\n",
			reason: "requires an absent adapter field",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := harnessRenderTree(tc.agent, tc.profile)
			outcome := runHarnessValidate(tree, "harness", "adapters", "validate")
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
		})
	}
}

// TestHarnessRenderFillsAnUnpinnedTierFromTheEmptyTierValues proves a tier
// declared as an empty mapping renders the profile's empty-tier model, since
// the tier pins nothing of its own.
func TestHarnessRenderFillsAnUnpinnedTierFromTheEmptyTierValues(t *testing.T) {
	tree := harnessRenderTree("        tier-fields: {model: model, effort: effort}\n        empty-tier: {model: session}\n", "      tiers: {fast: {}}\n")
	store := &memoryHarnessStore{tree: tree}
	outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate")
	if outcome.ExitCode != 0 {
		t.Fatalf("generate exit = %d (%q)", outcome.ExitCode, outcome.Stderr)
	}
	adapter := tree.files["adapters/alpha/agents/reviewer.md"]
	if !strings.Contains(adapter, "model: session\n") || strings.Contains(adapter, "effort:") {
		t.Fatalf("empty-tier values not rendered as expected: %q", adapter)
	}
}

// TestHarnessRenderMergesEntryTranslationsOnOneField proves two translations
// that each add entries to one field both land in that field, so neither entry
// is dropped.
func TestHarnessRenderMergesEntryTranslationsOnOneField(t *testing.T) {
	agent := "        translations:\n" +
		"          - {when: requires, capability: read, field: access, entries: {alpha: one}}\n" +
		"          - {when: denies, capability: net, field: access, entries: {beta: two}}\n"
	tree := harnessRenderTree(agent, "")
	store := &memoryHarnessStore{tree: tree}
	outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate")
	if outcome.ExitCode != 0 {
		t.Fatalf("generate exit = %d (%q)", outcome.ExitCode, outcome.Stderr)
	}
	adapter := tree.files["adapters/alpha/agents/reviewer.md"]
	if !strings.Contains(adapter, "alpha: one") || !strings.Contains(adapter, "beta: two") {
		t.Fatalf("merged entries missing: %q", adapter)
	}
}
