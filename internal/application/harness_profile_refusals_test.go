package application_test

import (
	"strings"
	"testing"
)

// harnessCanonicalAll declares every canonical field a native adapter may
// name, so a profile row is refused for its own shape and not a missing source.
const harnessCanonicalAll = `harness:
  canonical:
    agents:
      name: name
      description: description
      grants: grants
      denials: denials
      constraints: constraints
      dispatches: dispatches
    skills:
      name: name
      description: description
`

// harnessSkillProfile is one skill adapter with its required fields; a row
// appends the one field its refusal depends on.
const harnessSkillProfile = `    - id: alpha
      skill-adapter:
        path: adapters/alpha/skills/{name}/SKILL.md
        format: front-matter
        route-field: body
        route: "Read {path}."
`

// harnessAgentProfile is one agent adapter with its required fields.
const harnessAgentAdapterProfile = `    - id: alpha
      agent-adapter:
        path: adapters/alpha/agents/{name}.md
        format: front-matter
        route-field: body
        route: "Read {path}."
`

// TestHarnessProfileRefusesAnUnusableShape proves each adapter shape the
// planner cannot project refuses the run with the reason that names it.
func TestHarnessProfileRefusesAnUnusableShape(t *testing.T) {
	cases := []struct {
		name     string
		profiles string
		reason   string
	}{
		{
			name: "an agent path with no name placeholder refuses as an invalid adapter path",
			profiles: `    - id: alpha
      agent-adapter:
        path: adapters/alpha/agents/x.md
        format: front-matter
        route-field: body
        route: "Read {path}."
`,
			reason: "profile `alpha` has an invalid adapter path",
		},
		{
			name: "a skill path under the canonical tree refuses as an invalid adapter path",
			profiles: `    - id: alpha
      skill-adapter:
        path: .agents/skills/{name}/SKILL.md
        format: front-matter
        route-field: body
        route: "Read {path}."
`,
			reason: "profile `alpha` has an invalid adapter path",
		},
		{
			name: "an instruction path that climbs out of the repository refuses",
			profiles: `    - id: alpha
      instruction-adapter:
        path: ../AGENTS.md
        route: "Read {path}."
`,
			reason: "profile `alpha` has an invalid instruction adapter path",
		},
		{
			name:     "a skill adapter that projects a tier refuses",
			profiles: harnessSkillProfile + "        tier-fields:\n          model: m\n          effort: e\n",
			reason:   "profile `alpha` projects a tier into a skill adapter",
		},
		{
			name:     "a skill adapter that projects an empty tier refuses",
			profiles: harnessSkillProfile + "        empty-tier:\n          model: m\n",
			reason:   "profile `alpha` projects an empty tier into a skill adapter",
		},
		{
			name:     "a skill adapter that projects an agent list refuses",
			profiles: harnessSkillProfile + "        lists:\n          x: skills\n",
			reason:   "profile `alpha` projects an agent list into a skill adapter",
		},
		{
			name:     "a skill adapter that selects agents refuses",
			profiles: harnessSkillProfile + "        agents: [a]\n",
			reason:   "profile `alpha` selects agents in a skill adapter",
		},
		{
			name:     "a skill adapter that projects a dispatch list refuses",
			profiles: harnessSkillProfile + "        dispatches:\n          field: f\n          format: allow-map\n",
			reason:   "profile `alpha` projects a dispatch list into a skill adapter",
		},
		{
			name:     "an empty tier without tier fields refuses",
			profiles: harnessAgentAdapterProfile + "        empty-tier:\n          model: m\n",
			reason:   "profile `alpha` declares empty-tier values without tier fields",
		},
		{
			name:     "a dispatch field nested beyond one map refuses",
			profiles: harnessAgentAdapterProfile + "        dispatches:\n          field: a.b.c\n          format: allow-map\n",
			reason:   "profile `alpha` has an invalid dispatches declaration",
		},
		{
			name: "two profiles sharing one adapter root refuse as not distinct",
			profiles: harnessAgentAdapterProfile +
				`    - id: beta
      agent-adapter:
        path: adapters/alpha/agents/{name}.md
        format: front-matter
        route-field: body
        route: "Read {path}."
`,
			reason: "adapter roots must be distinct, non-overlapping repository paths",
		},
		{
			name: "a profile root nested inside another refuses as overlapping",
			profiles: `    - id: alpha
      agent-adapter:
        path: adapters/shared/{name}.md
        format: front-matter
        route-field: body
        route: "Read {path}."
    - id: beta
      agent-adapter:
        path: adapters/shared/inner/{name}.md
        format: front-matter
        route-field: body
        route: "Read {path}."
`,
			reason: "adapter roots must be distinct, non-overlapping repository paths",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := harnessTree(harnessCanonicalAll + "  profiles:\n" + tc.profiles)
			outcome := runHarnessValidate(tree, "harness", "adapters", "validate")
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
		})
	}
}

// TestHarnessModelResolveRefusesAnEmptyFallback proves a resolving tier whose
// fallback is empty refuses as not a model name, before any command runs.
func TestHarnessModelResolveRefusesAnEmptyFallback(t *testing.T) {
	tree := harnessTree(`harness:
  profiles:
    - id: alpha
      tiers:
        fast:
          effort: high
          model-resolve:
            command: [x]
            pattern: "*"
            fallback: ""
`)
	outcome := runHarnessValidate(tree, "harness", "adapters", "validate")
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, "declares a model-resolve fallback that is not a model name") {
		t.Fatalf("empty fallback not refused: %+v", outcome)
	}
}
