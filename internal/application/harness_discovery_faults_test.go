package application_test

import (
	"strings"
	"testing"
)

// harnessDiscoveryConfig declares one agent adapter and one skill adapter, with
// both canonical shapes, so discovery reads both canonical trees.
const harnessDiscoveryConfig = "harness:\n  canonical:\n" +
	"    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints, tier: tier}\n" +
	"    skills: {name: name, description: description}\n" +
	"  profiles:\n    - id: alpha\n      agent-adapter:\n" +
	"        path: adapters/alpha/agents/{name}.md\n        format: front-matter\n        route-field: body\n        route: \"Read {path}.\"\n        identity: {title: name}\n" +
	"      skill-adapter:\n" +
	"        path: adapters/alpha/skills/{name}.toml\n        format: toml\n        route-field: source\n        route: \"Read {path}.\"\n        identity: {title: name}\n"

// harnessDiscoveryTidy is one valid canonical skill whose name matches its path.
const harnessDiscoveryTidy = "---\nname: tidy\ndescription: Tidy the working tree thoroughly indeed.\n---\n"

// harnessDiscoveryTree stages the root instruction, one agent, and one skill.
func harnessDiscoveryTree(config string) *memoryTree {
	tree := harnessTree(config)
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files[".agents/agents/reviewer.md"] = harnessRenderReviewer
	tree.files[".agents/skills/tidy/SKILL.md"] = harnessDiscoveryTidy
	return tree
}

// TestHarnessDiscoveryRefusesABrokenCanonicalSource proves a canonical source
// that cannot be read, cannot be parsed, or names something other than its path
// refuses the generation, so no adapter is written from a partial canonical set.
func TestHarnessDiscoveryRefusesABrokenCanonicalSource(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(*memoryTree)
		reason string
	}{
		{
			name: "an unreadable canonical agent refuses",
			setup: func(tree *memoryTree) {
				tree.unreadable[".agents/agents/reviewer.md"] = true
			},
			reason: "canonical source `.agents/agents/reviewer.md` cannot be read",
		},
		{
			name: "a canonical skill with no front matter refuses",
			setup: func(tree *memoryTree) {
				tree.files[".agents/skills/tidy/SKILL.md"] = "no front matter here\n"
			},
			reason: "canonical source `.agents/skills/tidy/SKILL.md`",
		},
		{
			name: "a canonical skill whose name differs from its directory refuses",
			setup: func(tree *memoryTree) {
				tree.files[".agents/skills/tidy/SKILL.md"] = "---\nname: other\ndescription: Tidy the working tree thoroughly indeed.\n---\n"
			},
			reason: "canonical skill `.agents/skills/tidy/SKILL.md` has a name that does not match its path",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := harnessDiscoveryTree(harnessDiscoveryConfig)
			tc.setup(tree)
			store := &memoryHarnessStore{tree: tree}
			outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate")
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
			if store.calls != 0 {
				t.Fatalf("a refused generation wrote %d transactions", store.calls)
			}
		})
	}
}

// TestHarnessDiscoveryIgnoresASkillFileNestedBelowASkill proves a SKILL.md held
// two directories below the skill root is not a canonical skill, so a malformed
// one there does not refuse the generation.
func TestHarnessDiscoveryIgnoresASkillFileNestedBelowASkill(t *testing.T) {
	tree := harnessDiscoveryTree(harnessDiscoveryConfig)
	tree.files[".agents/skills/tidy/extra/SKILL.md"] = "not a canonical skill\n"
	store := &memoryHarnessStore{tree: tree}
	outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate")
	if outcome.ExitCode != 0 {
		t.Fatalf("nested skill file refused the generation: %+v", outcome)
	}
	if _, rendered := tree.files["adapters/alpha/skills/extra.toml"]; rendered {
		t.Fatalf("nested skill file was rendered as a canonical skill")
	}
	if _, rendered := tree.files["adapters/alpha/skills/tidy.toml"]; !rendered {
		t.Fatalf("the canonical skill was not rendered")
	}
}

// TestHarnessDiscoveryRefusesAFamilyWithoutItsCanonicalDeclaration proves an
// adapter family whose canonical field declaration is absent refuses, naming the
// declaration it needs.
func TestHarnessDiscoveryRefusesAFamilyWithoutItsCanonicalDeclaration(t *testing.T) {
	agentsGone := strings.Replace(harnessDiscoveryConfig,
		"    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints, tier: tier}\n", "", 1)
	skillsGone := strings.Replace(harnessDiscoveryConfig,
		"    skills: {name: name, description: description}\n", "", 1)
	for _, tc := range []struct {
		name   string
		config string
		reason string
	}{
		{"an agent adapter with no canonical agents refuses", agentsGone, "agent adapters require a canonical agent field declaration"},
		{"a skill adapter with no canonical skills refuses", skillsGone, "skill adapters require a canonical skill field declaration"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := harnessDiscoveryTree(tc.config)
			outcome := runHarnessValidate(tree, "harness", "adapters", "validate")
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
		})
	}
}
