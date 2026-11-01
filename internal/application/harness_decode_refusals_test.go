package application_test

import (
	"strings"
	"testing"
)

// harnessAgentHead is one profile whose agent adapter names every required
// field. Rows append or remove one field at a time.
const harnessAgentHead = `harness:
  profiles:
    - id: alpha
      agent-adapter:
        path: adapters/alpha/agents/{name}.md
        format: front-matter
        route-field: body
        route: "Read {path}."
`

// TestHarnessDecodeRefusesEachMalformedField proves each declared harness
// section refuses a single missing or mistyped field with the reason its
// decoder names, before any file is read.
func TestHarnessDecodeRefusesEachMalformedField(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		reason string
	}{
		{
			name: "canonical agents without a description",
			body: `harness:
  canonical:
    agents:
      name: name
`,
			reason: "harness canonical agents names no description",
		},
		{
			name: "canonical agents without grants",
			body: `harness:
  canonical:
    agents:
      name: name
      description: description
`,
			reason: "harness canonical agents names no grants",
		},
		{
			name: "canonical agents without denials",
			body: `harness:
  canonical:
    agents:
      name: name
      description: description
      grants: grants
`,
			reason: "harness canonical agents names no denials",
		},
		{
			name: "canonical agents without constraints",
			body: `harness:
  canonical:
    agents:
      name: name
      description: description
      grants: grants
      denials: denials
`,
			reason: "harness canonical agents names no constraints",
		},
		{
			name: "canonical skills without a name",
			body: `harness:
  canonical:
    skills:
      description: description
`,
			reason: "harness canonical skills names no name",
		},
		{
			name: "an instruction adapter without a path",
			body: `harness:
  profiles:
    - id: alpha
      instruction-adapter:
        route: "Read {path} and follow it."
`,
			reason: "harness profiles instruction-adapter names no path",
		},
		{
			name: "a native adapter without a path",
			body: `harness:
  profiles:
    - id: alpha
      agent-adapter:
        format: front-matter
        route-field: body
        route: "Read {path}."
`,
			reason: "harness profiles agent-adapter names no path",
		},
		{
			name: "a native adapter without a route field",
			body: `harness:
  profiles:
    - id: alpha
      agent-adapter:
        path: adapters/alpha/agents/{name}.md
        format: front-matter
        route: "Read {path}."
`,
			reason: "harness profiles agent-adapter names no route field",
		},
		{
			name: "a native adapter without a route",
			body: `harness:
  profiles:
    - id: alpha
      agent-adapter:
        path: adapters/alpha/agents/{name}.md
        format: front-matter
        route-field: body
`,
			reason: "harness profiles agent-adapter names no route",
		},
		{
			name:   "tier fields without a model",
			body:   harnessAgentHead + "        tier-fields:\n          effort: high\n",
			reason: "harness profiles agent-adapter tier-fields names no model",
		},
		{
			name:   "an empty tier with an unknown key",
			body:   harnessAgentHead + "        empty-tier:\n          bogus: x\n",
			reason: "harness profiles agent-adapter empty-tier names unknown key `bogus`",
		},
		{
			name:   "an empty tier whose effort is not a scalar",
			body:   harnessAgentHead + "        empty-tier:\n          effort: [x]\n",
			reason: "harness profiles agent-adapter empty-tier effort names no effort",
		},
		{
			name:   "dispatches with an unknown key",
			body:   harnessAgentHead + "        dispatches:\n          field: f\n          format: x\n          bogus: y\n",
			reason: "harness profiles agent-adapter dispatches names unknown key `bogus`",
		},
		{
			name:   "dispatches without a field",
			body:   harnessAgentHead + "        dispatches:\n          format: x\n",
			reason: "harness profiles agent-adapter dispatches names no field",
		},
		{
			name:   "a translation without a field",
			body:   harnessAgentHead + "        translations:\n          - when: always\n",
			reason: "harness profiles agent-adapter translations names no field",
		},
		{
			name:   "a translation whose absent members are not a list",
			body:   harnessAgentHead + "        translations:\n          - when: always\n            field: f\n            absent-members: x\n",
			reason: "harness absent-members names no absent members",
		},
		{
			name: "a tier whose effort is not a scalar",
			body: `harness:
  profiles:
    - id: alpha
      tiers:
        fast:
          effort: [x]
`,
			reason: "harness profiles tiers effort names no effort",
		},
		{
			name: "a model resolution with an unknown key",
			body: `harness:
  profiles:
    - id: alpha
      tiers:
        fast:
          model-resolve:
            command: [x]
            bogus: y
`,
			reason: "harness profiles tiers model-resolve names unknown key `bogus`",
		},
		{
			name: "a model resolution whose command is not a list",
			body: `harness:
  profiles:
    - id: alpha
      tiers:
        fast:
          model-resolve:
            command: x
`,
			reason: "harness profiles tiers model-resolve names no command",
		},
		{
			name: "a model resolution without a pattern",
			body: `harness:
  profiles:
    - id: alpha
      tiers:
        fast:
          model-resolve:
            command: [x]
            fallback: f
`,
			reason: "harness profiles tiers model-resolve names no pattern",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := harnessTree(tc.body)
			outcome := runHarnessValidate(tree, "harness", "adapters", "validate")
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, tc.reason) {
				t.Fatalf("expected refusal naming %q: %+v", tc.reason, outcome)
			}
		})
	}
}
