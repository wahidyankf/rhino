package application_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// memoryHarnessStore records the transaction generation planned and applies
// it to the memory tree the way the disk store would: families replaced
// wholesale, exact files replaced singly, everything else kept.
type memoryHarnessStore struct {
	tree         *memoryTree
	calls        int
	last         application.AdapterTransaction
	replaceError error
}

func (s *memoryHarnessStore) Replace(root string, transaction application.AdapterTransaction) error {
	s.calls++
	s.last = transaction
	if s.replaceError != nil {
		return s.replaceError
	}
	_ = root
	kept := map[string]string{}
	for path, contents := range s.tree.files {
		under := false
		for _, family := range transaction.Roots {
			if path == family || strings.HasPrefix(path, family+"/") {
				under = true
				break
			}
		}
		if under {
			continue
		}
		exact := false
		for _, single := range transaction.ExactPaths {
			if path == single {
				exact = true
				break
			}
		}
		if exact {
			continue
		}
		kept[path] = contents
	}
	for _, file := range transaction.Files {
		kept[file.Path] = file.Contents
	}
	s.tree.files = kept
	return nil
}

// stubModelResolver answers queued outputs in order and records every
// launch, so tests prove validation never resolves and generation resolves
// each tier once with the declared narrow launch.
type stubModelResolver struct {
	outputs  []string
	failure  string
	launches []application.ModelLaunch
}

func (s *stubModelResolver) Resolve(launch application.ModelLaunch) (string, error) {
	s.launches = append(s.launches, launch)
	if s.failure != "" {
		return "", errors.New(s.failure)
	}
	if len(s.outputs) == 0 {
		return "", errors.New("the synthetic resolver had no declared output")
	}
	output := s.outputs[0]
	s.outputs = s.outputs[1:]
	return output, nil
}

func harnessTree(body string) *memoryTree {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = "schema: rhino/repo-config/v2\n" + body
	return tree
}

func runHarnessValidate(tree *memoryTree, words ...string) application.Outcome {
	return application.ExecuteWithHarness(tree, args(words...), nil, nil, nil, nil, nil, nil)
}

func runHarnessGenerate(tree *memoryTree, store *memoryHarnessStore, resolver *stubModelResolver, words ...string) application.Outcome {
	var heldStore application.AdapterStore
	if store != nil {
		heldStore = store
	}
	var heldResolver application.ModelResolver
	if resolver != nil {
		heldResolver = resolver
	}
	return application.ExecuteWithHarness(tree, args(words...), nil, nil, nil, nil, heldStore, heldResolver)
}

const harnessInstructionConfig = `harness:
  profiles:
    - id: alpha
      instruction-adapter:
        path: CLAUDE.md
        route: "Read {path} and follow it."
`

const harnessRootInstruction = "---\nname: root\n---\nRoot.\n"

func TestHarnessLeavesAcceptNoFlags(t *testing.T) {
	for _, words := range [][]string{
		{"harness", "adapters", "validate", "--file", "a.md"},
		{"harness", "adapters", "generate", "--file", "a.md"},
	} {
		tree := harnessTree("harness:\n  profiles: []\n")
		outcome := runHarnessValidate(tree, words...)
		if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "not accepted") {
			t.Fatalf("%v: expected flag refusal, got %+v", words, outcome)
		}
	}
}

func TestHarnessUndeclaredGroup(t *testing.T) {
	rows := []struct {
		name    string
		words   []string
		stream  string
		payload string
	}{
		{"validate text", []string{"harness", "adapters", "validate"},
			"rhino: [harness-adapters] harness: no adapter profiles are declared\n", ""},
		{"generate text", []string{"harness", "adapters", "generate"},
			"rhino: [harness-adapters] harness: no adapter profiles are declared\n", ""},
		{"validate json", []string{"harness", "adapters", "validate", "--output", "json"},
			"", `"code":"rhino.config.undeclared","message":"[harness-adapters] harness: no adapter profiles are declared"`},
		{"generate json", []string{"harness", "adapters", "generate", "--output", "json"},
			"", `"code":"rhino.config.undeclared","message":"[harness-adapters] harness: no adapter profiles are declared"`},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			tree := harnessTree("policies: {}\n")
			outcome := runHarnessValidate(tree, row.words...)
			if outcome.ExitCode != 2 {
				t.Fatalf("exit = %d, want 2", outcome.ExitCode)
			}
			if row.stream != "" && outcome.Stderr != row.stream {
				t.Fatalf("stderr = %q, want %q", outcome.Stderr, row.stream)
			}
			if row.payload != "" && !strings.Contains(outcome.Stderr, row.payload) {
				t.Fatalf("stderr = %q, want it to contain %q", outcome.Stderr, row.payload)
			}
		})
	}
}

func harnessAgentProfile(adapter string) string {
	return `harness:
  canonical:
    agents:
      name: name
      description: description
      grants: grants
      denials: denials
      constraints: constraints
  profiles:
    - id: alpha
` + adapter
}

const harnessFrontMatterAdapter = `      agent-adapter:
        path: adapters/alpha/agents/{name}.md
        format: front-matter
        route-field: body
        route: "Read {path}."
`

const harnessReviewerAgent = `---
name: reviewer
description: Review changes.
grants:
  - read
denials:
  - net
---
# Reviewer
`

func TestHarnessPlannerRefusesBeforeAnyWrite(t *testing.T) {
	rows := []struct {
		name   string
		config string
		files  map[string]string
		cause  string
	}{
		{"empty roster", "harness:\n  profiles: []\n", nil,
			"harness: at least one adapter profile is required"},
		{"agents need canonical", "harness:\n  profiles:\n    - id: alpha\n" + harnessFrontMatterAdapter, nil,
			"harness: agent adapters require a canonical agent field declaration"},
		{"dispatches need canonical field", "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints}\n  profiles:\n    - id: alpha\n      agent-adapter: {path: 'adapters/alpha/{name}.md', format: front-matter, route-field: body, route: 'See {path}.', dispatches: {field: tools, format: 'spawn({names})'}}\n", nil,
			"harness: a dispatches adapter key requires a canonical dispatches field declaration"},
		{"skills need canonical", "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints}\n  profiles:\n    - id: alpha\n      skill-adapter: {path: 'adapters/alpha/{name}.md', format: front-matter, route-field: body, route: 'See {path}.'}\n", nil,
			"harness: skill adapters require a canonical skill field declaration"},
		{"empty identifier", "harness:\n  profiles:\n    - id: '  '\n      instruction-adapter: {path: OUT.md, route: 'See {path}.'}\n", nil,
			"harness: an adapter profile has an empty id"},
		{"duplicated identifier", "harness:\n  profiles:\n    - id: alpha\n      instruction-adapter: {path: A.md, route: 'See {path}.'}\n    - id: alpha\n      instruction-adapter: {path: B.md, route: 'See {path}.'}\n", nil,
			"harness: duplicated adapter profile `alpha`"},
		{"no native representation", "harness:\n  profiles:\n    - id: alpha\n", nil,
			"profile `alpha` declares no native adapter representation"},
		{"adapter root under agents", harnessAgentProfile("      agent-adapter:\n        path: .agents/other/{name}.md\n        format: front-matter\n        route-field: body\n        route: 'See {path}.'\n"), nil,
			"profile `alpha` has an invalid adapter path"},
		{"adapter placeholder doubled", harnessAgentProfile("      agent-adapter:\n        path: adapters/{name}/{name}.md\n        format: front-matter\n        route-field: body\n        route: 'See {path}.'\n"), nil,
			"profile `alpha` has an invalid adapter path"},
		{"instruction under agents", "harness:\n  profiles:\n    - id: alpha\n      instruction-adapter: {path: .agents/OUT.md, route: 'See {path}.'}\n", nil,
			"profile `alpha` has an invalid instruction adapter path"},
		{"instruction is the root", "harness:\n  profiles:\n    - id: alpha\n      instruction-adapter: {path: AGENTS.md, route: 'See {path}.'}\n", nil,
			"profile `alpha` has an invalid instruction adapter path"},
		{"route without placeholder", "harness:\n  profiles:\n    - id: alpha\n      instruction-adapter: {path: OUT.md, route: 'See it.'}\n", nil,
			"profile `alpha` must declare exactly one `{path}` route placeholder"},
		{"route with two placeholders", "harness:\n  profiles:\n    - id: alpha\n      instruction-adapter: {path: OUT.md, route: 'See {path} and {path}.'}\n", nil,
			"profile `alpha` must declare exactly one `{path}` route placeholder"},
		{"required capability gap", "harness:\n  requirements: {capabilities: [read]}\n  profiles:\n    - id: alpha\n      instruction-adapter: {path: OUT.md, route: 'See {path}.'}\n", nil,
			"profile `alpha` cannot represent required capability `read`"},
		{"empty required capability", "harness:\n  requirements: {capabilities: ['']}\n  profiles:\n    - id: alpha\n      instruction-adapter: {path: OUT.md, route: 'See {path}.'}\n", nil,
			"harness: required capability is empty"},
		{"nested family roots", "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints}\n    skills: {name: name, description: description}\n  profiles:\n    - id: alpha\n" + harnessFrontMatterAdapter + "      skill-adapter:\n        path: adapters/alpha/{name}.md\n        format: front-matter\n        route-field: body\n        route: 'See {path}.'\n", nil,
			"harness: adapter roots must be distinct, non-overlapping repository paths"},
		{"route field mismatches format", harnessAgentProfile("      agent-adapter:\n        path: adapters/alpha/agents/{name}.toml\n        format: toml\n        route-field: body\n        route: 'See {path}.'\n"), nil,
			"profile `alpha` has a route field incompatible with its adapter format"},
		{"empty route field", harnessAgentProfile("      agent-adapter:\n        path: adapters/alpha/agents/{name}.toml\n        format: toml\n        route-field: '  '\n        route: 'See {path}.'\n"), nil,
			"profile `alpha` has an empty adapter route field"},
		{"exact agent pattern with a bad route", "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints}\n  profiles:\n    - id: alpha\n      agent-adapter: {path: '.agents/agents/{name}/adapter.md', format: front-matter, route-field: body, route: 'See it.'}\n", nil,
			"profile `alpha` must declare exactly one `{path}` route placeholder"},
		{"translation members collide with a scalar", harnessAgentProfile("      agent-adapter:\n        path: adapters/alpha/agents/{name}.md\n        format: front-matter\n        route-field: body\n        route: 'See {path}.'\n        fixed: {tools: agent}\n        translations: [{when: always, field: tools, members: [run]}]\n"), nil,
			"profile `alpha` has an invalid capability translation"},
		{"translation entries rival members", harnessAgentProfile("      agent-adapter:\n        path: adapters/alpha/agents/{name}.md\n        format: front-matter\n        route-field: body\n        route: 'See {path}.'\n        translations: [{when: always, field: tools, members: [run]}, {when: always, field: tools, entries: {mode: strict}}]\n"), nil,
			"profile `alpha` mixes incompatible capability translations"},
		{"translation entries meet a fixed scalar", harnessAgentProfile("      agent-adapter:\n        path: adapters/alpha/agents/{name}.md\n        format: front-matter\n        route-field: body\n        route: 'See {path}.'\n        fixed: {tools: agent}\n        translations: [{when: always, field: tools, entries: {mode: strict}}]\n"), nil,
			"profile `alpha` has an invalid capability translation"},
		{"allow-map with a missing canonical field", harnessAgentProfile("      agent-adapter:\n        path: adapters/alpha/agents/{name}.md\n        format: front-matter\n        route-field: body\n        route: 'See {path}.'\n        translations: [{when: always, field: tools, entries: {mode: strict}}]\n        dispatches: {field: tools, format: allow-map}\n"), nil,
			"harness: a dispatches adapter key requires a canonical dispatches field declaration"},
		{"flat agent pattern stays invalid", "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints}\n  profiles:\n    - id: alpha\n      agent-adapter: {path: '.agents/agents/{name}.md', format: front-matter, route-field: body, route: 'See {path}.'}\n", nil,
			"profile `alpha` has an invalid adapter path"},
		{"duplicated instruction path", "harness:\n  profiles:\n    - id: alpha\n      instruction-adapter: {path: OUT.md, route: 'See {path}.'}\n    - id: beta\n      instruction-adapter: {path: OUT.md, route: 'See {path}.'}\n", nil,
			"harness: exact adapter paths must be distinct and outside every adapter root"},
		{"instruction path inside a family", "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints}\n  profiles:\n    - id: alpha\n      instruction-adapter: {path: 'adapters/alpha/agents/OUT.md', route: 'See {path}.'}\n" + harnessFrontMatterAdapter, nil,
			"harness: exact adapter paths must be distinct and outside every adapter root"},
		{"exact pattern under the wrong root", "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints}\n  profiles:\n    - id: alpha\n      agent-adapter: {path: '.agents/skills/{name}/adapter.md', format: front-matter, route-field: body, route: 'See {path}.'}\n", nil,
			"profile `alpha` has an invalid adapter path"},
		{"pattern sample escapes the root", "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints}\n  profiles:\n    - id: alpha\n      agent-adapter: {path: 'adapters/../gone/{name}/adapter.md', format: front-matter, route-field: body, route: 'See {path}.'}\n", nil,
			"profile `alpha` has an invalid adapter path"},
		{"exact agent root is not a family", "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints}\n  profiles:\n    - id: alpha\n      agent-adapter: {path: '.agents/other/{name}/adapter.md', format: front-matter, route-field: body, route: 'See {path}.'}\n", nil,
			"profile `alpha` has an invalid adapter path"},
		{"repeated native field", harnessAgentProfile("      agent-adapter:\n        path: adapters/alpha/agents/{name}.md\n        format: front-matter\n        route-field: body\n        route: 'See {path}.'\n        identity: {title: name}\n        fixed: {title: agent}\n"), nil,
			"profile `alpha` repeats or omits an adapter field"},
		{"dispatches collide with a field", "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints, dispatches: dispatches}\n  profiles:\n    - id: alpha\n      agent-adapter:\n        path: adapters/alpha/agents/{name}.md\n        format: front-matter\n        route-field: body\n        route: 'See {path}.'\n        fixed: {tools: agent}\n        dispatches: {field: tools, format: 'spawn({names})'}\n", nil,
			"profile `alpha` repeats or omits an adapter field"},
		{"invalid dispatches declaration", "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints, dispatches: dispatches}\n  profiles:\n    - id: alpha\n      agent-adapter:\n        path: adapters/alpha/agents/{name}.md\n        format: front-matter\n        route-field: body\n        route: 'See {path}.'\n        dispatches: {field: a.b.c, format: allow-map}\n", nil,
			"profile `alpha` has an invalid dispatches declaration"},
		{"translation without its capability", harnessAgentProfile("      agent-adapter:\n        path: adapters/alpha/agents/{name}.md\n        format: front-matter\n        route-field: body\n        route: 'See {path}.'\n        translations: [{when: requires, field: access, members: [read]}]\n"), nil,
			"profile `alpha` has an invalid capability translation"},
		{"mixed translation kinds", harnessAgentProfile("      agent-adapter:\n        path: adapters/alpha/agents/{name}.md\n        format: front-matter\n        route-field: body\n        route: 'See {path}.'\n        translations: [{when: always, field: access, members: [read]}, {when: always, field: access, entries: {mode: strict}}]\n"), nil,
			"profile `alpha` mixes incompatible capability translations"},
		{"tier fields share a name", harnessAgentProfile("      agent-adapter:\n        path: adapters/alpha/agents/{name}.md\n        format: front-matter\n        route-field: body\n        route: 'See {path}.'\n        tier-fields: {model: model, effort: model}\n"), nil,
			"profile `alpha` has invalid tier fields"},
		{"missing declared agents root", "harness:\n  agents-root: gone\n  profiles:\n    - id: alpha\n      instruction-adapter: {path: OUT.md, route: 'See {path}.'}\n", nil,
			"canonical agents root `gone` is missing"},
		{"invalid declared agents root", "harness:\n  agents-root: '..'\n  profiles:\n    - id: alpha\n      instruction-adapter: {path: OUT.md, route: 'See {path}.'}\n", nil,
			"harness: invalid canonical agents root `..`"},
		{"missing root instruction", "harness:\n  profiles:\n    - id: alpha\n      instruction-adapter: {path: OUT.md, route: 'See {path}.'}\n", map[string]string{},
			"canonical source `AGENTS.md` is missing"},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			for _, leaf := range [][]string{{"harness", "adapters", "validate"}, {"harness", "adapters", "generate"}} {
				tree := harnessTree(row.config)
				if row.files == nil {
					tree.files["AGENTS.md"] = harnessRootInstruction
				}
				store := &memoryHarnessStore{tree: tree}
				outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, leaf...)
				if outcome.ExitCode != 2 {
					t.Fatalf("%v: exit = %d, want 2 (%q)", leaf, outcome.ExitCode, outcome.Stderr)
				}
				if !strings.Contains(outcome.Stderr, "[harness-adapters] "+row.cause) {
					t.Fatalf("%v: stderr = %q, want it to contain %q", leaf, outcome.Stderr, row.cause)
				}
				if store.calls != 0 {
					t.Fatalf("%v: refused planning wrote %d transactions, want none", leaf, store.calls)
				}
			}
		})
	}
}

func TestHarnessDiscoveryAndRenderRefusals(t *testing.T) {
	rows := []struct {
		name    string
		profile string
		files   map[string]string
		cause   string
	}{
		{"selection without a source", harnessFrontMatterAdapter + "        agents: [ghost]\n",
			map[string]string{".agents/agents/reviewer.md": harnessReviewerAgent},
			"profile `alpha` selects agent `ghost` with no canonical source"},
		{"tier missing from an agent", harnessFrontMatterAdapter + "      tiers: {fast: {model: m1, effort: low}}\n",
			map[string]string{".agents/agents/reviewer.md": harnessReviewerAgent},
			"harness-tier-missing: canonical agent `.agents/agents/reviewer.md` declares no tier in its `tier` field"},
		{"tier undeclared by a profile", "      agent-adapter:\n        path: adapters/alpha/agents/{name}.md\n        format: front-matter\n        route-field: body\n        route: 'See {path}.'\n      tiers: {slow: {model: m9, effort: low}}\n",
			map[string]string{".agents/agents/reviewer.md": "---\nname: reviewer\ndescription: Review changes.\ntier: fast\ngrants:\n  - read\ndenials:\n  - net\n---\n"},
			"harness-tier-undeclared: profile `alpha` declares no tier `fast` for canonical agent `.agents/agents/reviewer.md`"},
		{"agent name mismatches its path", harnessFrontMatterAdapter,
			map[string]string{".agents/agents/reviewer.md": "---\nname: other\ndescription: Review changes.\ngrants:\n  - read\ndenials:\n  - net\n---\n"},
			"canonical agent `.agents/agents/reviewer.md` has a name that does not match its path"},
		{"front matter never opens", harnessFrontMatterAdapter,
			map[string]string{".agents/agents/reviewer.md": "name: reviewer\n"},
			"canonical source `.agents/agents/reviewer.md` has no leading front matter"},
		{"front matter never closes", harnessFrontMatterAdapter,
			map[string]string{".agents/agents/reviewer.md": "---\nname: reviewer\n"},
			"canonical source `.agents/agents/reviewer.md` has unterminated front matter"},
		{"indented front matter key", harnessFrontMatterAdapter,
			map[string]string{".agents/agents/reviewer.md": "---\n name: reviewer\n---\n"},
			"canonical source `.agents/agents/reviewer.md` has malformed front matter"},
		{"repeated front matter key", harnessFrontMatterAdapter,
			map[string]string{".agents/agents/reviewer.md": "---\nname: reviewer\nname: reviewer\n---\n"},
			"canonical source `.agents/agents/reviewer.md` repeats or omits a front matter key"},
		{"missing required metadata", harnessFrontMatterAdapter,
			map[string]string{".agents/agents/reviewer.md": "---\nname: reviewer\n---\n"},
			"canonical source `.agents/agents/reviewer.md` has no `description` metadata field"},
		{"blank scalar metadata", harnessFrontMatterAdapter,
			map[string]string{".agents/agents/reviewer.md": "---\nname: reviewer\ndescription: '   '\ngrants:\n  - read\ndenials:\n  - net\n---\n"},
			"canonical source `.agents/agents/reviewer.md` has a non-scalar `description` metadata field"},
		{"scalar where a list belongs", harnessFrontMatterAdapter,
			map[string]string{".agents/agents/reviewer.md": "---\nname: reviewer\ndescription: Review changes.\ngrants: read\ndenials: [net]\n---\n"},
			"canonical source `.agents/agents/reviewer.md` has a non-list `grants` metadata field"},
		{"absent field already rendered", "      agent-adapter:\n        path: adapters/alpha/agents/{name}.md\n        format: front-matter\n        route-field: body\n        route: 'See {path}.'\n        fixed: {kind: agent}\n        absent: [kind]\n",
			map[string]string{".agents/agents/reviewer.md": "---\nname: reviewer\ndescription: Review changes.\ntier: fast\ngrants:\n  - read\ndenials:\n  - net\n---\n"},
			"canonical source `.agents/agents/reviewer.md` requires an absent adapter field"},
		{"unscoped spawn tool", "      agent-adapter:\n        path: adapters/alpha/agents/{name}.md\n        format: front-matter\n        route-field: body\n        route: 'See {path}.'\n        translations: [{when: always, field: tools, members: [run]}]\n        dispatches: {field: tools, format: 'run({names})'}\n",
			map[string]string{".agents/agents/reviewer.md": "---\nname: reviewer\ndescription: Review changes.\ntier: fast\ngrants:\n  - read\ndenials:\n  - net\ndispatches:\n  - run\n---\n"},
			"harness-dispatch-unscoped: profile `alpha` renders an unscoped `run` for canonical agent `.agents/agents/reviewer.md`"},
		{"absent removal without a member list", "      agent-adapter:\n        path: adapters/alpha/agents/{name}.md\n        format: front-matter\n        route-field: body\n        route: 'See {path}.'\n        translations: [{when: always, field: tools, absent-members: [run]}]\n",
			map[string]string{".agents/agents/reviewer.md": "---\nname: reviewer\ndescription: Review changes.\ntier: fast\ngrants:\n  - read\ndenials:\n  - net\n---\n"},
			"adapter output cannot remove members from undeclared field `tools`"},
		{"dispatches collide with entries", "      agent-adapter:\n        path: adapters/alpha/agents/{name}.md\n        format: front-matter\n        route-field: body\n        route: 'See {path}.'\n        translations: [{when: always, field: tools, entries: {mode: strict}}]\n        dispatches: {field: tools, format: 'spawn({names})'}\n",
			map[string]string{".agents/agents/reviewer.md": "---\nname: reviewer\ndescription: Review changes.\ntier: fast\ngrants:\n  - read\ndenials:\n  - net\ndispatches:\n  - x\n---\n"},
			"adapter output conflicts at `tools`"},
		{"allow-map repeats translation entries", "      agent-adapter:\n        path: adapters/alpha/agents/{name}.md\n        format: front-matter\n        route-field: body\n        route: 'See {path}.'\n        translations: [{when: always, field: tools, entries: {mode: strict}}]\n        dispatches: {field: tools, format: allow-map}\n",
			map[string]string{".agents/agents/reviewer.md": "---\nname: reviewer\ndescription: Review changes.\ntier: fast\ngrants:\n  - read\ndenials:\n  - net\ndispatches:\n  - x\n---\n"},
			"adapter output repeats field `tools`"},
	}
	prefix := "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints, tier: tier, dispatches: dispatches}\n  profiles:\n    - id: alpha\n"
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			for _, leaf := range [][]string{{"harness", "adapters", "validate"}, {"harness", "adapters", "generate"}} {
				tree := harnessTree(prefix + row.profile)
				tree.files["AGENTS.md"] = harnessRootInstruction
				for path, contents := range row.files {
					tree.files[path] = contents
				}
				store := &memoryHarnessStore{tree: tree}
				outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, leaf...)
				if outcome.ExitCode != 2 {
					t.Fatalf("%v: exit = %d, want 2 (%q)", leaf, outcome.ExitCode, outcome.Stderr)
				}
				if !strings.Contains(outcome.Stderr, "[harness-adapters] "+row.cause) {
					t.Fatalf("%v: stderr = %q, want it to contain %q", leaf, outcome.Stderr, row.cause)
				}
				if store.calls != 0 {
					t.Fatalf("%v: refused rendering wrote %d transactions, want none", leaf, store.calls)
				}
			}
		})
	}
}

func TestHarnessValidateFindingsAndClean(t *testing.T) {
	rows := []struct {
		name   string
		extra  map[string]string
		stream string
		stderr string
		code   int
	}{
		{"missing adapter text", nil,
			"", "[harness-adapters] CLAUDE.md: missing-adapter\n", 1},
		{"missing adapter json", nil,
			"{\"schemaVersion\":1,\"command\":\"harness-adapters\",\"status\":\"findings\",\"findings\":[\"CLAUDE.md: missing-adapter\"]}\n", "", 1},
		{"divergent adapter", map[string]string{"CLAUDE.md": "stale words\n"},
			"", "[harness-adapters] CLAUDE.md: divergent-adapter\n", 1},
		{"clean text", map[string]string{"CLAUDE.md": "Read AGENTS.md and follow it.\n"},
			"[harness-adapters] canonical adapter validation is clean\n", "", 0},
		{"clean json", map[string]string{"CLAUDE.md": "Read AGENTS.md and follow it.\n"},
			"{\"schemaVersion\":1,\"command\":\"harness-adapters\",\"status\":\"clean\",\"message\":\"canonical adapter validation is clean\"}\n", "", 0},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			tree := harnessTree(harnessInstructionConfig)
			tree.files["AGENTS.md"] = harnessRootInstruction
			for path, contents := range row.extra {
				tree.files[path] = contents
			}
			words := []string{"harness", "adapters", "validate"}
			if strings.Contains(row.stream, "json") || strings.Contains(row.name, "json") {
				words = append(words, "--output", "json")
			}
			outcome := runHarnessValidate(tree, words...)
			if outcome.ExitCode != row.code {
				t.Fatalf("exit = %d, want %d (%q)", outcome.ExitCode, row.code, outcome.Stderr)
			}
			if row.stream != "" && outcome.Stdout != row.stream {
				t.Fatalf("stdout = %q, want %q", outcome.Stdout, row.stream)
			}
			if row.stderr != "" && outcome.Stderr != row.stderr {
				t.Fatalf("stderr = %q, want %q", outcome.Stderr, row.stderr)
			}
		})
	}
}

func TestHarnessPlanRefusalJSONCarriesHarnessCode(t *testing.T) {
	tree := harnessTree("harness:\n  profiles: []\n")
	tree.files["AGENTS.md"] = harnessRootInstruction
	for _, leaf := range [][]string{{"harness", "adapters", "validate"}, {"harness", "adapters", "generate"}} {
		outcome := runHarnessGenerate(tree, &memoryHarnessStore{tree: tree}, &stubModelResolver{},
			append(append([]string{}, leaf...), "--output", "json")...)
		if outcome.ExitCode != 2 {
			t.Fatalf("%v: exit = %d, want 2", leaf, outcome.ExitCode)
		}
		want := "{\"schemaVersion\":1,\"error\":{\"code\":\"rhino.harness.refused\",\"message\":\"[harness-adapters] harness: at least one adapter profile is required\"}}\n"
		if outcome.Stderr != want {
			t.Fatalf("%v: stderr = %q, want %q", leaf, outcome.Stderr, want)
		}
	}
}

func TestHarnessGenerateWritesThenValidatesClean(t *testing.T) {
	tree := harnessTree(harnessInstructionConfig)
	tree.files["AGENTS.md"] = harnessRootInstruction
	store := &memoryHarnessStore{tree: tree}
	outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate")
	if outcome.ExitCode != 0 {
		t.Fatalf("generate exit = %d (%q)", outcome.ExitCode, outcome.Stderr)
	}
	if outcome.Stdout != "[harness-adapters] canonical adapter generation is current\n" {
		t.Fatalf("generate stdout = %q", outcome.Stdout)
	}
	if store.calls != 1 {
		t.Fatalf("generate wrote %d transactions, want 1", store.calls)
	}
	if got := tree.files["CLAUDE.md"]; got != "Read AGENTS.md and follow it.\n" {
		t.Fatalf("CLAUDE.md = %q", got)
	}
	// A current generation plans no differences, so the store stays idle.
	again := &memoryHarnessStore{tree: tree}
	regenerated := runHarnessGenerate(tree, again, &stubModelResolver{}, "harness", "adapters", "generate")
	if regenerated.ExitCode != 0 || again.calls != 0 {
		t.Fatalf("regenerate = %+v, store calls = %d", regenerated, again.calls)
	}
	validated := runHarnessValidate(tree, "harness", "adapters", "validate")
	if validated.ExitCode != 0 || validated.Stdout != "[harness-adapters] canonical adapter validation is clean\n" {
		t.Fatalf("validate after generate = %+v", validated)
	}
}

func TestHarnessGenerateRendersFrontMatterAndCatalog(t *testing.T) {
	config := `harness:
  canonical:
    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints, tier: tier}
    skills: {name: name, description: description}
  profiles:
    - id: alpha
      agent-adapter:
        path: adapters/alpha/agents/{name}.md
        format: front-matter
        route-field: body
        route: "Read {path}."
        identity: {title: name}
        fixed: {kind: agent}
        lists: {abilities: skills}
        tier-fields: {model: model, effort: effort}
        empty-tier: {model: session}
        translations:
          - {when: requires, capability: read, field: access, members: [read-files]}
      tiers: {fast: {model: model-a-1.0, effort: low}}
      skill-adapter:
        path: adapters/alpha/skills/{name}.toml
        format: toml
        route-field: source
        route: "Read {path}."
        identity: {title: name}
`
	tree := harnessTree(config)
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files[".agents/agents/reviewer.md"] = "---\nname: reviewer\ndescription: Review changes thoroughly.\ntier: fast\ngrants:\n  - read\ndenials:\n  - net\nskills:\n  - tidy\n---\n"
	tree.files[".agents/skills/tidy/SKILL.md"] = "---\nname: tidy\ndescription: Tidy the working tree thoroughly indeed.\n---\n"
	store := &memoryHarnessStore{tree: tree}
	outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate")
	if outcome.ExitCode != 0 {
		t.Fatalf("generate exit = %d (%q)", outcome.ExitCode, outcome.Stderr)
	}
	wantAdapter := `---
abilities:
  - tidy
access: read-files
effort: low
kind: agent
model: model-a-1.0
title: reviewer
---

Read .agents/agents/reviewer.md.
`
	if got := tree.files["adapters/alpha/agents/reviewer.md"]; got != wantAdapter {
		t.Fatalf("adapter = %q, want %q", got, wantAdapter)
	}
	wantSkill := "source = \"Read .agents/skills/tidy/SKILL.md.\"\ntitle = \"tidy\"\n"
	if got := tree.files["adapters/alpha/skills/tidy.toml"]; got != wantSkill {
		t.Fatalf("skill adapter = %q, want %q", got, wantSkill)
	}
	catalog := tree.files["adapters/alpha/agents/catalog.json"]
	for _, fragment := range []string{
		"\"schemaVersion\": 1",
		"\"profile\": \"alpha\"",
		"\"id\": \"instruction\"",
		"\"path\": \"AGENTS.md\"",
		"\"id\": \"agent/reviewer\"",
		"\"path\": \".agents/agents/reviewer.md\"",
	} {
		if !strings.Contains(catalog, fragment) {
			t.Fatalf("catalog = %q, want it to contain %q", catalog, fragment)
		}
	}
	if strings.Contains(catalog, "resolvedModels") {
		t.Fatalf("catalog without resolving tiers must omit resolvedModels: %q", catalog)
	}
	// Sources sort by path bytes, so the dotted skills tree leads the
	// root instruction, which leads nothing here.
	if strings.Index(catalog, ".agents/agents/reviewer.md") > strings.Index(catalog, "AGENTS.md") {
		t.Fatalf("catalog sources are not path-sorted: %q", catalog)
	}
}

func TestHarnessValidateReportsStaleFamilyFiles(t *testing.T) {
	tree := harnessTree(harnessAgentProfile(harnessFrontMatterAdapter))
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files[".agents/agents/reviewer.md"] = harnessReviewerAgent
	store := &memoryHarnessStore{tree: tree}
	generated := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate")
	if generated.ExitCode != 0 {
		t.Fatalf("generate exit = %d (%q)", generated.ExitCode, generated.Stderr)
	}
	tree.files["adapters/alpha/agents/gone.md"] = "x\n"
	outcome := runHarnessValidate(tree, "harness", "adapters", "validate")
	if outcome.ExitCode != 1 {
		t.Fatalf("exit = %d, want 1 (%q)", outcome.ExitCode, outcome.Stderr)
	}
	if outcome.Stderr != "[harness-adapters] adapters/alpha/agents/gone.md: stale-adapter\n" {
		t.Fatalf("stderr = %q", outcome.Stderr)
	}
}

func TestHarnessGenerateWithoutAStoreRefusesClosed(t *testing.T) {
	tree := harnessTree(harnessInstructionConfig)
	tree.files["AGENTS.md"] = harnessRootInstruction
	outcome := runHarnessGenerate(tree, nil, &stubModelResolver{}, "harness", "adapters", "generate")
	if outcome.ExitCode != 2 {
		t.Fatalf("exit = %d, want 2", outcome.ExitCode)
	}
	want := "rhino: [harness-adapters] this entry point has no canonical-adapter write boundary\n"
	if outcome.Stderr != want {
		t.Fatalf("stderr = %q, want %q", outcome.Stderr, want)
	}
}

func TestHarnessGenerateResolvesNewestModelAndWarnsOnFallback(t *testing.T) {
	canonical := "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints, tier: tier}\n  profiles:\n    - id: alpha\n      agent-adapter: {path: 'adapters/alpha/{name}.md', format: front-matter, route-field: body, route: 'See {path}.', tier-fields: {model: model, effort: effort}}\n      tiers: {fast: {model-resolve: {command: [list-models], pattern: 'model-*', fallback: model-0.9}, effort: low}}\n"
	tree := harnessTree(canonical)
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files[".agents/agents/reviewer.md"] = "---\nname: reviewer\ndescription: Review.\ntier: fast\n---\n"
	resolver := &stubModelResolver{outputs: []string{"model-1.9 model-1.10 model-2x model-1.02"}}
	store := &memoryHarnessStore{tree: tree}
	outcome := runHarnessGenerate(tree, store, resolver, "harness", "adapters", "generate")
	if outcome.ExitCode != 0 {
		t.Fatalf("generate exit = %d (%q)", outcome.ExitCode, outcome.Stderr)
	}
	if len(resolver.launches) != 1 {
		t.Fatalf("resolved %d times, want 1", len(resolver.launches))
	}
	launch := resolver.launches[0]
	if len(launch.Arguments) != 1 || launch.Arguments[0] != "list-models" || launch.Directory != "/root" || launch.TimeoutSeconds != 30 {
		t.Fatalf("launch = %+v, want the declared narrow vector", launch)
	}
	// 1.10 beats 1.9 numerically, while 1.02 loses on its shorter digit run.
	if got := tree.files["adapters/alpha/reviewer.md"]; !strings.Contains(got, "model: model-1.10\n") {
		t.Fatalf("adapter = %q, want the newest resolved model", got)
	}
	if !strings.Contains(tree.files["adapters/alpha/provenance.json"], "\"fast\": \"model-1.10\"") {
		t.Fatalf("provenance = %q, want the recorded model", tree.files["adapters/alpha/provenance.json"])
	}
	// A version tie breaks toward the larger token, digit run for digit run.
	resolver.outputs = []string{"model-01 model-1"}
	delete(tree.files, "adapters/alpha/reviewer.md")
	delete(tree.files, "adapters/alpha/catalog.json")
	delete(tree.files, "adapters/alpha/provenance.json")
	tied := runHarnessGenerate(tree, &memoryHarnessStore{tree: tree}, resolver, "harness", "adapters", "generate")
	if tied.ExitCode != 0 {
		t.Fatalf("tied generate exit = %d (%q)", tied.ExitCode, tied.Stderr)
	}
	if got := tree.files["adapters/alpha/reviewer.md"]; !strings.Contains(got, "model: model-1\n") {
		t.Fatalf("tied adapter = %q, want the larger token", got)
	}
	// A failed run keeps the recorded model and says so on standard error.
	resolver.failure = "could not start model command: missing"
	regenerate := runHarnessGenerate(tree, &memoryHarnessStore{tree: tree}, resolver, "harness", "adapters", "generate")
	if regenerate.ExitCode != 0 {
		t.Fatalf("regenerate exit = %d (%q)", regenerate.ExitCode, regenerate.Stderr)
	}
	wantWarning := "[harness-adapters] warning: profile `alpha` tier `fast` resolved no model (could not start model command: missing); using the last resolved model `model-1`\n"
	if regenerate.Stderr != wantWarning {
		t.Fatalf("regenerate stderr = %q, want %q", regenerate.Stderr, wantWarning)
	}
	if got := tree.files["adapters/alpha/reviewer.md"]; !strings.Contains(got, "model: model-1\n") {
		t.Fatalf("adapter after failed resolve = %q", got)
	}
}

func TestHarnessReadsBlockScalarsAndLiteralDashes(t *testing.T) {
	config := `harness:
  canonical:
    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints}
  profiles:
    - id: alpha
      agent-adapter:
        path: adapters/alpha/{name}.md
        format: front-matter
        route-field: body
        route: "Read {path}."
        identity: {title: name, about: description}
        fixed: {dash: -lead}
`
	tree := harnessTree(config)
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files[".agents/agents/reviewer.md"] = "---\nname: reviewer\ndescription: >-\n  Line one\n  Line two\ngrants:\n  - read\ndenials:\n  - net\n---\n"
	store := &memoryHarnessStore{tree: tree}
	outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate")
	if outcome.ExitCode != 0 {
		t.Fatalf("generate exit = %d (%q)", outcome.ExitCode, outcome.Stderr)
	}
	// A block scalar joins its lines with one space, and a leading dash
	// stays a literal block, never a bare scalar.
	want := "---\nabout: Line one Line two\ndash: |-\n  -lead\ntitle: reviewer\n---\n\nRead .agents/agents/reviewer.md.\n"
	if got := tree.files["adapters/alpha/reviewer.md"]; got != want {
		t.Fatalf("adapter = %q, want %q", got, want)
	}
}

func TestHarnessRendersQuotedMultilineTOML(t *testing.T) {
	config := "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints}\n  profiles:\n    - id: alpha\n      agent-adapter: {path: 'adapters/alpha/{name}.toml', format: toml, route-field: source, route: 'See {path}.', fixed: {dash: '-lead', mixed: \"a\\u0001b\\nc\", quotes3: 'a\"\"\"b', squote: \"it'\\\\s\", both: \"a\\\"b'c\"}}\n"
	tree := harnessTree(config)
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files[".agents/agents/reviewer.md"] = harnessReviewerAgent
	store := &memoryHarnessStore{tree: tree}
	if outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate"); outcome.ExitCode != 0 {
		t.Fatalf("generate exit = %d (%q)", outcome.ExitCode, outcome.Stderr)
	}
	// A double quote beside a single quote fits neither single-line form,
	// so the writer opens a multi-line basic string instead of escaping.
	want := "both = \"\"\"a\"b'c\"\"\"\ndash = \"-lead\"\nmixed = \"\"\"\na\\u0001b\nc\"\"\"\nquotes3 = 'a\"\"\"b'\nsource = \"See .agents/agents/reviewer.md.\"\nsquote = '''it'\\s'''\n"
	if got := tree.files["adapters/alpha/reviewer.toml"]; got != want {
		t.Fatalf("adapter = %q, want %q", got, want)
	}
}

func TestHarnessResolvingTiersNeedNoAdapter(t *testing.T) {
	config := "harness:\n  profiles:\n    - id: alpha\n      instruction-adapter: {path: OUT.md, route: 'See {path}.'}\n      tiers: {fast: {model: model-a-1.0, effort: low}}\n"
	tree := harnessTree(config)
	tree.files["AGENTS.md"] = harnessRootInstruction
	store := &memoryHarnessStore{tree: tree}
	if outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate"); outcome.ExitCode != 0 {
		t.Fatalf("generate exit = %d (%q)", outcome.ExitCode, outcome.Stderr)
	}
	if got := tree.files["OUT.md"]; got != "See AGENTS.md.\n" {
		t.Fatalf("OUT.md = %q", got)
	}
}

func TestHarnessUnreadableAndForeignBytesRefuse(t *testing.T) {
	t.Run("non-text canonical source", func(t *testing.T) {
		tree := harnessTree(harnessAgentProfile(harnessFrontMatterAdapter))
		tree.files["AGENTS.md"] = harnessRootInstruction
		tree.files[".agents/agents/reviewer.md"] = "x"
		tree.binary[".agents/agents/reviewer.md"] = true
		outcome := runHarnessValidate(tree, "harness", "adapters", "validate")
		if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "canonical source `.agents/agents/reviewer.md` holds non-text bytes") {
			t.Fatalf("outcome = %+v", outcome)
		}
	})
	t.Run("unreadable marker", func(t *testing.T) {
		config := harnessInstructionConfig + "  marker-surfaces: ['docs/*.md']\n"
		tree := harnessTree(config)
		tree.files["AGENTS.md"] = harnessRootInstruction
		tree.files["CLAUDE.md"] = "Read AGENTS.md and follow it.\n"
		tree.files["docs/notes.md"] = "rhino generated\n"
		tree.unreadable["docs/notes.md"] = true
		outcome := runHarnessValidate(tree, "harness", "adapters", "validate")
		if outcome.ExitCode != 2 || outcome.Stderr != "rhino: [harness-adapters] docs/notes.md: docs/notes.md: permission denied\n" {
			t.Fatalf("outcome = %+v", outcome)
		}
		jsoned := runHarnessValidate(tree, "harness", "adapters", "validate", "--output", "json")
		if jsoned.ExitCode != 2 || !strings.Contains(jsoned.Stderr, `"code":"rhino.file.unreadable"`) {
			t.Fatalf("json outcome = %+v", jsoned)
		}
	})
	t.Run("malformed provenance resolves the fallback", func(t *testing.T) {
		config := "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints, tier: tier}\n  profiles:\n    - id: alpha\n      agent-adapter: {path: 'adapters/alpha/{name}.md', format: front-matter, route-field: body, route: 'See {path}.', tier-fields: {model: model, effort: effort}}\n      tiers: {fast: {model-resolve: {command: [list-models], pattern: 'model-*', fallback: model-0.9}, effort: low}}\n"
		tree := harnessTree(config)
		tree.files["AGENTS.md"] = harnessRootInstruction
		tree.files[".agents/agents/reviewer.md"] = "---\nname: reviewer\ndescription: Review.\ntier: fast\n---\n"
		tree.files["adapters/alpha/provenance.json"] = "not json\n"
		tree.files["adapters/alpha/catalog.json"] = "{}\n"
		tree.files["adapters/alpha/reviewer.md"] = "stale\n"
		store := &memoryHarnessStore{tree: tree}
		outcome := runHarnessGenerate(tree, store, &stubModelResolver{failure: "boom"}, "harness", "adapters", "generate")
		if outcome.ExitCode != 0 {
			t.Fatalf("generate exit = %d (%q)", outcome.ExitCode, outcome.Stderr)
		}
		if !strings.Contains(outcome.Stderr, "using the fallback `model-0.9`") {
			t.Fatalf("stderr = %q", outcome.Stderr)
		}
	})
}

func TestHarnessAppendsTranslationMembersInOrder(t *testing.T) {
	config := "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints}\n  profiles:\n    - id: alpha\n      agent-adapter: {path: 'adapters/alpha/{name}.md', format: front-matter, route-field: body, route: 'See {path}.', translations: [{when: requires, capability: read, field: access, members: [a]}, {when: requires, capability: write, field: access, members: [b]}]}\n"
	tree := harnessTree(config)
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files[".agents/agents/reviewer.md"] = "---\nname: reviewer\ndescription: R.\ngrants:\n  - read\n  - write\n---\n"
	store := &memoryHarnessStore{tree: tree}
	if outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate"); outcome.ExitCode != 0 {
		t.Fatalf("generate exit = %d (%q)", outcome.ExitCode, outcome.Stderr)
	}
	// Members accumulate in translation order, then render as one list.
	want := "---\naccess: |-\n  a, b\n---\n\nSee .agents/agents/reviewer.md.\n"
	if got := tree.files["adapters/alpha/reviewer.md"]; got != want {
		t.Fatalf("adapter = %q, want %q", got, want)
	}
}

func TestHarnessReadsSingleQuotedNames(t *testing.T) {
	tree := harnessTree(harnessAgentProfile(harnessFrontMatterAdapter))
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files[".agents/agents/reviewer.md"] = "---\nname: 'reviewer'\ndescription: Review changes.\ngrants:\n  - read\ndenials:\n  - net\n---\n"
	if outcome := runHarnessValidate(tree, "harness", "adapters", "validate"); outcome.ExitCode != 1 {
		t.Fatalf("outcome = %+v", outcome)
	}
}

func TestHarnessRefusesUnportableGeneratedPaths(t *testing.T) {
	config := "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints}\n  profiles:\n    - id: alpha\n      agent-adapter: {path: 'adapters/alpha/{name}.md', format: front-matter, route-field: body, route: 'See {path}.'}\n"
	tree := harnessTree(config)
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files[".agents/agents/back\\slash.md"] = "---\nname: 'back\\slash'\ndescription: R.\ngrants:\n  - read\ndenials:\n  - net\n---\n"
	store := &memoryHarnessStore{tree: tree}
	outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate")
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "generated adapter path `adapters/alpha/back\\slash.md` is invalid") {
		t.Fatalf("outcome = %+v", outcome)
	}
}

func TestHarnessRendersDispatchShapesExactly(t *testing.T) {
	config := `harness:
  canonical:
    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints, dispatches: dispatches}
  profiles:
    - id: alpha
      agent-adapter:
        path: adapters/alpha/{name}.md
        format: front-matter
        route-field: body
        route: "Read {path}."
        dispatches: {field: tools, format: "run({names})"}
    - id: beta
      agent-adapter:
        path: adapters/beta/{name}.md
        format: front-matter
        route-field: body
        route: "Read {path}."
        dispatches: {field: perms.tools, format: allow-map}
`
	tree := harnessTree(config)
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files[".agents/agents/reviewer.md"] = "---\nname: reviewer\ndescription: R.\ngrants:\n  - read\ndenials:\n  - net\ndispatches:\n  - helper\n---\n"
	store := &memoryHarnessStore{tree: tree}
	if outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate"); outcome.ExitCode != 0 {
		t.Fatalf("generate exit = %d (%q)", outcome.ExitCode, outcome.Stderr)
	}
	// A member template starts its own list; an allow map nests its table.
	wantAlpha := "---\ntools: |-\n  run(helper)\n---\n\nRead .agents/agents/reviewer.md.\n"
	if got := tree.files["adapters/alpha/reviewer.md"]; got != wantAlpha {
		t.Fatalf("alpha adapter = %q, want %q", got, wantAlpha)
	}
	wantBeta := "---\nperms:\n  tools:\n    \"*\": deny\n    helper: allow\n---\n\nRead .agents/agents/reviewer.md.\n"
	if got := tree.files["adapters/beta/reviewer.md"]; got != wantBeta {
		t.Fatalf("beta adapter = %q, want %q", got, wantBeta)
	}
}

func TestHarnessValidationKeepsRecordedFallbacks(t *testing.T) {
	config := "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints, tier: tier}\n  profiles:\n    - id: alpha\n      agent-adapter: {path: 'adapters/alpha/{name}.md', format: front-matter, route-field: body, route: 'See {path}.', tier-fields: {model: model, effort: effort}}\n      tiers: {fast: {model-resolve: {command: [list-models], pattern: 'model-*', fallback: model-0.9}, effort: low}}\n"
	tree := harnessTree(config)
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files[".agents/agents/reviewer.md"] = "---\nname: reviewer\ndescription: Review.\ntier: fast\n---\n"
	// No generation ran, so validation keeps the declared fallback without
	// starting anything, and reports the absent files.
	outcome := runHarnessValidate(tree, "harness", "adapters", "validate")
	if outcome.ExitCode != 1 {
		t.Fatalf("exit = %d, want 1 (%q)", outcome.ExitCode, outcome.Stderr)
	}
	want := "[harness-adapters] adapters/alpha/catalog.json: missing-adapter\n[harness-adapters] adapters/alpha/provenance.json: missing-adapter\n[harness-adapters] adapters/alpha/reviewer.md: missing-adapter\n"
	if outcome.Stderr != want {
		t.Fatalf("stderr = %q, want %q", outcome.Stderr, want)
	}
}

func TestHarnessSkipsBlankFrontMatterLines(t *testing.T) {
	tree := harnessTree(harnessAgentProfile(harnessFrontMatterAdapter))
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files[".agents/agents/reviewer.md"] = "---\nname: reviewer\n\ndescription: Review changes.\ngrants:\n  - read\ndenials:\n  - net\n---\n"
	if outcome := runHarnessGenerate(tree, &memoryHarnessStore{tree: tree}, &stubModelResolver{}, "harness", "adapters", "generate"); outcome.ExitCode != 0 {
		t.Fatalf("generate exit = %d (%q)", outcome.ExitCode, outcome.Stderr)
	}
}

func TestHarnessOwnedFamilyFilesAreNeverStaleMarkers(t *testing.T) {
	config := "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints}\n  profiles:\n    - id: alpha\n" + harnessFrontMatterAdapter + "  marker-surfaces: ['adapters/**']\n"
	tree := harnessTree(config)
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files[".agents/agents/reviewer.md"] = harnessReviewerAgent
	store := &memoryHarnessStore{tree: tree}
	if generated := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate"); generated.ExitCode != 0 {
		t.Fatalf("generate = %+v", generated)
	}
	// A marker under an owned family root that generation did not write is
	// the adapter comparison's stale file, never a stale marker: ownership
	// stays with the family either way.
	tree.files["adapters/alpha/agents/stray.md"] = "rhino generated\n"
	outcome := runHarnessValidate(tree, "harness", "adapters", "validate")
	want := "[harness-adapters] adapters/alpha/agents/stray.md: stale-adapter\n"
	if outcome.ExitCode != 1 || outcome.Stderr != want {
		t.Fatalf("outcome = %+v", outcome)
	}
}

func TestHarnessRendersNestedDispatchTables(t *testing.T) {
	config := `harness:
  agents-root: canon/agents
  canonical:
    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints, dispatches: dispatches}
  profiles:
    - id: alpha
      agent-adapter:
        path: adapters/alpha/{name}.toml
        format: toml
        route-field: source
        route: "Read {path}."
        translations:
          - {when: always, field: caps, entries: {"": deny}}
        dispatches: {field: perms.tools, format: allow-map}
`
	tree := harnessTree(config)
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files["canon/agents/reviewer.md"] = "---\nname: reviewer\ndescription: R.\ngrants:\n  - read\ndenials:\n  - net\ndispatches:\n  - helper\n---\n"
	store := &memoryHarnessStore{tree: tree}
	if outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate"); outcome.ExitCode != 0 {
		t.Fatalf("generate exit = %d (%q)", outcome.ExitCode, outcome.Stderr)
	}
	// An empty parent header opens the nested table, and an empty key
	// quotes itself: both spellings come from the TOML writer's table.
	want := "[caps]\n\"\" = \"deny\"\n[perms]\n[perms.tools]\n\"*\" = \"deny\"\nhelper = \"allow\"\nsource = \"Read canon/agents/reviewer.md.\"\n"
	if got := tree.files["adapters/alpha/reviewer.toml"]; got != want {
		t.Fatalf("adapter = %q, want %q", got, want)
	}
}

func TestHarnessMarkerScanHonoursExcludes(t *testing.T) {
	config := "scan:\n  exclude-directories: [gen]\n" + harnessInstructionConfig + "  marker-surfaces: ['**']\n"
	tree := harnessTree(config)
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files["CLAUDE.md"] = "Read AGENTS.md and follow it.\n"
	tree.files["gen/notes.md"] = "rhino generated\n"
	tree.files["other.md"] = "rhino generated\n"
	outcome := runHarnessValidate(tree, "harness", "adapters", "validate")
	if outcome.ExitCode != 1 || outcome.Stderr != "[harness-adapters] other.md: stale-marker\n" {
		t.Fatalf("outcome = %+v", outcome)
	}
}

func TestHarnessGenerateFallsBackWithoutARecord(t *testing.T) {
	canonical := "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints, tier: tier}\n  profiles:\n    - id: alpha\n      agent-adapter: {path: 'adapters/alpha/{name}.md', format: front-matter, route-field: body, route: 'See {path}.', tier-fields: {model: model, effort: effort}}\n      tiers: {fast: {model-resolve: {command: [list-models], pattern: 'model-*', fallback: model-0.9}, effort: low}}\n"
	tree := harnessTree(canonical)
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files[".agents/agents/reviewer.md"] = "---\nname: reviewer\ndescription: Review.\ntier: fast\n---\n"
	resolver := &stubModelResolver{outputs: []string{"nothing matching here"}}
	store := &memoryHarnessStore{tree: tree}
	outcome := runHarnessGenerate(tree, store, resolver, "harness", "adapters", "generate")
	if outcome.ExitCode != 0 {
		t.Fatalf("generate exit = %d (%q)", outcome.ExitCode, outcome.Stderr)
	}
	wantWarning := "[harness-adapters] warning: profile `alpha` tier `fast` resolved no model (no output token matched `model-*`); using the fallback `model-0.9`\n"
	if outcome.Stderr != wantWarning {
		t.Fatalf("stderr = %q, want %q", outcome.Stderr, wantWarning)
	}
	if got := tree.files["adapters/alpha/reviewer.md"]; !strings.Contains(got, "model: model-0.9\n") {
		t.Fatalf("adapter = %q, want the fallback model", got)
	}
	// Validation never starts the declared command: it keeps the recorded
	// fallback silently.
	validated := runHarnessValidate(tree, "harness", "adapters", "validate")
	if validated.ExitCode != 0 || validated.Stderr != "" {
		t.Fatalf("validate = %+v, want silent clean", validated)
	}
}

func TestHarnessMarkerSurfaces(t *testing.T) {
	config := harnessInstructionConfig + "  marker-surfaces: ['docs/*.md']\n"
	newTree := func() *memoryTree {
		tree := harnessTree(config)
		tree.files["AGENTS.md"] = harnessRootInstruction
		tree.files["CLAUDE.md"] = "Read AGENTS.md and follow it.\n"
		return tree
	}
	t.Run("unowned marker is stale", func(t *testing.T) {
		tree := newTree()
		tree.files["docs/notes.md"] = "# Notes\n<!-- rhino generated -->\n"
		outcome := runHarnessValidate(tree, "harness", "adapters", "validate")
		if outcome.ExitCode != 1 || outcome.Stderr != "[harness-adapters] docs/notes.md: stale-marker\n" {
			t.Fatalf("outcome = %+v", outcome)
		}
		jsoned := runHarnessValidate(tree, "harness", "adapters", "validate", "--output", "json")
		want := "{\"schemaVersion\":1,\"command\":\"harness-adapters\",\"status\":\"findings\",\"findings\":[\"docs/notes.md: stale-marker\"]}\n"
		if jsoned.ExitCode != 1 || jsoned.Stdout != want {
			t.Fatalf("json outcome = %+v", jsoned)
		}
	})
	t.Run("owned marker is not stale", func(t *testing.T) {
		config := "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints}\n  profiles:\n    - id: alpha\n" + harnessFrontMatterAdapter + "  marker-surfaces: ['adapters/**']\n"
		tree := harnessTree(config)
		tree.files["AGENTS.md"] = harnessRootInstruction
		tree.files[".agents/agents/reviewer.md"] = harnessReviewerAgent
		store := &memoryHarnessStore{tree: tree}
		if generated := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate"); generated.ExitCode != 0 {
			t.Fatalf("generate = %+v", generated)
		}
		// A generation marker inside an owned family file reads as a
		// divergent adapter, never as a stale marker: the comparison
		// already owns every file under the family roots.
		tree.files["adapters/alpha/agents/reviewer.md"] += "<!-- rhino generated -->\n"
		outcome := runHarnessValidate(tree, "harness", "adapters", "validate")
		want := "[harness-adapters] adapters/alpha/agents/reviewer.md: divergent-adapter\n"
		if outcome.ExitCode != 1 || outcome.Stderr != want {
			t.Fatalf("outcome = %+v", outcome)
		}
	})
	t.Run("generate reports markers after writing", func(t *testing.T) {
		tree := newTree()
		delete(tree.files, "CLAUDE.md")
		tree.files["docs/notes.md"] = "rhino generated\n"
		store := &memoryHarnessStore{tree: tree}
		outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate")
		if outcome.ExitCode != 1 {
			t.Fatalf("exit = %d, want 1 (%q)", outcome.ExitCode, outcome.Stderr)
		}
		if outcome.Stderr != "[harness-adapters] docs/notes.md: stale-marker\n" {
			t.Fatalf("stderr = %q", outcome.Stderr)
		}
		if tree.files["CLAUDE.md"] != "Read AGENTS.md and follow it.\n" {
			t.Fatalf("CLAUDE.md = %q", tree.files["CLAUDE.md"])
		}
	})
	t.Run("climbing marker surface refuses", func(t *testing.T) {
		tree := harnessTree(harnessInstructionConfig + "  marker-surfaces: ['../docs/*.md']\n")
		tree.files["AGENTS.md"] = harnessRootInstruction
		outcome := runHarnessValidate(tree, "harness", "adapters", "validate")
		if outcome.ExitCode != 2 {
			t.Fatalf("exit = %d, want 2", outcome.ExitCode)
		}
		if !strings.Contains(outcome.Stderr, "[harness-adapters] harness.marker-surfaces: `../docs/*.md` must be relative to the repository root") {
			t.Fatalf("stderr = %q", outcome.Stderr)
		}
	})
}

func TestHarnessExactAgentPatternRoundTrip(t *testing.T) {
	config := `harness:
  canonical:
    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints}
  profiles:
    - id: alpha
      agent-adapter:
        path: .agents/agents/{name}/adapter.md
        format: front-matter
        route-field: body
        route: "Read {path}."
        identity: {title: name}
`
	tree := harnessTree(config)
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files[".agents/agents/reviewer.md"] = harnessReviewerAgent
	store := &memoryHarnessStore{tree: tree}
	outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate")
	if outcome.ExitCode != 0 {
		t.Fatalf("generate exit = %d (%q)", outcome.ExitCode, outcome.Stderr)
	}
	if got := tree.files[".agents/agents/reviewer/adapter.md"]; !strings.Contains(got, "title: reviewer\n") {
		t.Fatalf("exact adapter = %q", got)
	}
	validated := runHarnessValidate(tree, "harness", "adapters", "validate")
	if validated.ExitCode != 0 {
		t.Fatalf("validate = %+v", validated)
	}
}

func TestHarnessDecodeRefusesMalformedGroups(t *testing.T) {
	rows := []struct {
		name     string
		fragment string
		cause    string
	}{
		{"unknown group key", "bogus: 1", "harness names unknown key `bogus`"},
		{"canonical not a mapping", "canonical: [1]", "harness canonical names no canonical declaration"},
		{"canonical unknown key", "canonical: {bogus: 1}", "harness canonical names unknown key `bogus`"},
		{"agents not a mapping", "canonical: {agents: [1]}", "harness canonical agents names no agent declaration"},
		{"agents unknown key", "canonical: {agents: {name: n, description: d, grants: g, denials: e, constraints: c, bogus: 1}}",
			"harness canonical agents names unknown key `bogus`"},
		{"agents missing name", "canonical: {agents: {description: d, grants: g, denials: e, constraints: c}}",
			"harness canonical agents names no name"},
		{"agents mistyped name", "canonical: {agents: {name: 1, description: d, grants: g, denials: e, constraints: c}}",
			"harness canonical agents names no name"},
		{"agents mistyped tier", "canonical: {agents: {name: n, description: d, grants: g, denials: e, constraints: c, tier: 1}}",
			"harness canonical agents tier names no tier"},
		{"agents mistyped dispatches", "canonical: {agents: {name: n, description: d, grants: g, denials: e, constraints: c, dispatches: 1}}",
			"harness canonical agents dispatches names no dispatches"},
		{"skills not a mapping", "canonical: {skills: [1]}", "harness canonical skills names no skill declaration"},
		{"skills unknown key", "canonical: {skills: {name: n, description: d, bogus: 1}}",
			"harness canonical skills names unknown key `bogus`"},
		{"skills missing description", "canonical: {skills: {name: n}}", "harness canonical skills names no description"},
		{"agents root mistyped", "agents-root: [1]", "harness agents-root names no agents root"},
		{"requirements not a mapping", "requirements: [1]", "harness.requirements names no requirement axes"},
		{"requirements unknown key", "requirements: {bogus: [a]}", "harness.requirements names unknown key `bogus`"},
		{"requirements axis mistyped", "requirements: {capabilities: x}", "harness.requirements capabilities names no capabilities"},
		{"requirements item mistyped", "requirements: {capabilities: [1]}", "harness.requirements capabilities names no capabilities"},
		{"profiles not a list", "profiles: {}", "harness profiles names no adapter profiles"},
		{"profile not a mapping", "profiles: [1]", "harness profiles names no adapter profile"},
		{"profile unknown key", "profiles: [{id: a, bogus: 1}]", "harness profiles names unknown key `bogus`"},
		{"profile missing id", "profiles: [{}]", "harness profiles id names no identifier"},
		{"supports not a mapping", "profiles: [{id: a, supports: [1]}]", "harness.profiles supports names no requirement axes"},
		{"supports axis mistyped", "profiles: [{id: a, supports: {capabilities: 1}}]",
			"harness.profiles supports capabilities names no capabilities"},
		{"supports item mistyped", "profiles: [{id: a, supports: {grants: [1]}}]",
			"harness.profiles supports grants names no grants"},
		{"instruction adapter mistyped", "profiles: [{id: a, instruction-adapter: [1]}]",
			"harness profiles instruction-adapter names no instruction adapter"},
		{"instruction unknown key", "profiles: [{id: a, instruction-adapter: {path: O.md, route: 'See {path}.', bogus: 1}}]",
			"harness profiles instruction-adapter names unknown key `bogus`"},
		{"instruction missing route", "profiles: [{id: a, instruction-adapter: {path: O.md}}]",
			"harness profiles instruction-adapter names no route"},
		{"adapter mistyped", "profiles: [{id: a, agent-adapter: [1]}]", "harness profiles agent-adapter names no adapter"},
		{"adapter unknown key", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', bogus: 1}}]",
			"harness profiles agent-adapter names unknown key `bogus`"},
		{"adapter missing format", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', route-field: body, route: 'See {path}.'}}]",
			"harness profiles agent-adapter names no format"},
		{"adapter unknown format", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: yaml, route-field: body, route: 'See {path}.'}}]",
			"harness profiles agent-adapter names unknown format `yaml`: expected `front-matter` or `toml`"},
		{"identity mistyped", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', identity: [1]}}]",
			"harness profiles agent-adapter identity names no identity"},
		{"identity unknown value", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', identity: {t: bogus}}}]",
			"harness profiles identity names unknown identity `bogus`: expected `name` or `description`"},
		{"fixed value mistyped", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', fixed: {k: 1}}}]",
			"harness profiles fixed names no fixed value"},
		{"lists mistyped", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', lists: [1]}}]",
			"harness profiles agent-adapter lists names no lists"},
		{"lists unknown name", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', lists: {t: agents}}}]",
			"harness profiles lists names unknown list `agents`: expected `skills`"},
		{"tier fields mistyped", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', tier-fields: [1]}}]",
			"harness profiles agent-adapter tier-fields names no tier fields"},
		{"tier fields unknown", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', tier-fields: {model: m, effort: e, bogus: 1}}}]",
			"harness profiles agent-adapter tier-fields names unknown key `bogus`"},
		{"tier fields missing effort", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', tier-fields: {model: m}}}]",
			"harness profiles agent-adapter tier-fields names no effort"},
		{"empty tier mistyped", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', empty-tier: [1]}}]",
			"harness profiles agent-adapter empty-tier names no empty tier"},
		{"empty tier model mistyped", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', empty-tier: {model: 1}}}]",
			"harness profiles agent-adapter empty-tier model names no model"},
		{"dispatches mistyped", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', dispatches: [1]}}]",
			"harness profiles agent-adapter dispatches names no dispatches"},
		{"dispatches missing format", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', dispatches: {field: t}}}]",
			"harness profiles agent-adapter dispatches names no format"},
		{"agents selection mistyped", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', agents: x}}]",
			"harness profiles agent-adapter agents names no agents"},
		{"agents item mistyped", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', agents: [1]}}]",
			"harness profiles agent-adapter agents names no agents"},
		{"absent mistyped", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', absent: x}}]",
			"harness absent names no absent fields"},
		{"translations mistyped", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', translations: x}}]",
			"harness profiles agent-adapter translations names no translations"},
		{"translation mistyped", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', translations: [1]}}]",
			"harness profiles agent-adapter translations names no translation"},
		{"translation unknown key", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', translations: [{when: always, field: f, members: [m], bogus: 1}]}}]",
			"harness profiles agent-adapter translations names unknown key `bogus`"},
		{"translation missing occasion", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', translations: [{field: f, members: [m]}]}}]",
			"harness profiles agent-adapter translations names no occasion"},
		{"translation unknown occasion", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', translations: [{when: sometimes, field: f, members: [m]}]}}]",
			"harness profiles agent-adapter translations names unknown occasion `sometimes`"},
		{"translation capability mistyped", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', translations: [{when: requires, capability: 1, field: f, members: [m]}]}}]",
			"harness profiles agent-adapter translations capability names no capability"},
		{"translation members mistyped", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', translations: [{when: always, field: f, members: x}]}}]",
			"harness members names no members"},
		{"translation entries mistyped", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', translations: [{when: always, field: f, entries: 1}]}}]",
			"harness profiles agent-adapter translations entries names no entries"},
		{"translation entry mistyped", "profiles: [{id: a, agent-adapter: {path: 'a/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', translations: [{when: always, field: f, entries: {k: 1}}]}}]",
			"harness profiles agent-adapter translations entries names no entry"},
		{"tiers mistyped", "profiles: [{id: a, tiers: [1]}]", "harness profiles tiers names no tiers"},
		{"tier mistyped", "profiles: [{id: a, tiers: {fast: 1}}]", "harness profiles tiers names no tier"},
		{"tier unknown key", "profiles: [{id: a, tiers: {fast: {bogus: 1}}}]", "harness profiles tiers names unknown key `bogus`"},
		{"tier model mistyped", "profiles: [{id: a, tiers: {fast: {model: 1}}}]", "harness profiles tiers model names no model"},
		{"resolve mistyped", "profiles: [{id: a, tiers: {fast: {model-resolve: 1}}}]",
			"harness profiles tiers model-resolve names no model resolution"},
		{"resolve command missing", "profiles:\n- id: a\n  tiers:\n    fast:\n      model-resolve:\n        pattern: 'x-*'\n        fallback: x",
			"harness profiles tiers model-resolve names no command"},
		{"resolve command item mistyped", "profiles:\n- id: a\n  tiers:\n    fast:\n      model-resolve:\n        command:\n        - 1\n        pattern: 'x-*'\n        fallback: x",
			"harness profiles tiers model-resolve names no command"},
		{"resolve missing fallback", "profiles: [{id: a, tiers: {fast: {model-resolve: {command: [list], pattern: 'x-*'}}}}]",
			"harness profiles tiers model-resolve names no fallback"},
		{"marker surfaces mistyped", "marker-surfaces: {}", "harness marker-surfaces names no marker surfaces"},
		{"marker surface item mistyped", "marker-surfaces: [1]", "harness marker-surfaces names no marker surfaces"},
		{"skill adapter threading", "profiles: [{id: a, skill-adapter: {path: 's/{n}.md', format: front-matter, route-field: body, route: 'See {path}.', bogus: 1}}]",
			"harness profiles skill-adapter names unknown key `bogus`"},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			tree := harnessTree("harness:\n  " + strings.ReplaceAll(row.fragment, "\n", "\n  ") + "\n")
			tree.files["AGENTS.md"] = harnessRootInstruction
			outcome := runHarnessValidate(tree, "harness", "adapters", "validate")
			if outcome.ExitCode != 2 {
				t.Fatalf("exit = %d, want 2 (%q)", outcome.ExitCode, outcome.Stderr)
			}
			if !strings.Contains(outcome.Stderr, "rhino: "+row.cause) {
				t.Fatalf("stderr = %q, want it to contain %q", outcome.Stderr, row.cause)
			}
		})
	}
}

func TestHarnessRendersNativeFormatsExactly(t *testing.T) {
	config := `harness:
  agents-root: canon/agents
  canonical:
    agents: {name: name, description: description, tier: tier, grants: grants, denials: denials, constraints: constraints, dispatches: dispatches}
    skills: {name: name, description: description}
  requirements: {capabilities: [read], grants: [read, write], denials: [network], constraints: [small, focused], routes: [route-a], identities: [reviewer]}
  profiles:
    - id: alpha
      supports: {capabilities: [read], grants: [read, write], denials: [network], constraints: [small, focused], routes: [route-a], identities: [reviewer]}
      agent-adapter:
        path: adapters/alpha/agents/{name}.md
        format: front-matter
        route-field: body
        route: "Read {path} \u2014 follow it."
        identity: {title: name, about: description}
        fixed: {kind: agent, note: "line1\nline2"}
        lists: {abilities: skills}
        tier-fields: {model: model, effort: effort}
        translations:
          - {when: always, field: team, members: [core]}
          - {when: requires, capability: read, field: access, members: [read-files]}
          - {when: denies, capability: network, field: access, absent-members: [net-access]}
          - {when: constrains, capability: small, field: limits, members: [tiny]}
          - {when: no-dispatches, field: notes, entries: {mode: strict}}
          - {when: requires, capability: write, field: notes, entries: {level: high}}
        dispatches: {field: permissions, format: allow-map}
        absent: [forbidden]
      tiers: {fast: {model: model-a-1.0, effort: low}}
      skill-adapter:
        path: adapters/alpha/skills/{name}.toml
        format: toml
        route-field: source
        route: "Read {path}."
        identity: {title: name}
        fixed: {kind: skill, quoted: "say \"hi\" \\ bye"}
        translations:
          - {when: always, field: caps, entries: {"*": deny, "my key": allow}}
    - id: beta
      supports: {capabilities: [read], grants: [read, write], denials: [network], constraints: [small, focused], routes: [route-a], identities: [reviewer]}
      agent-adapter:
        path: adapters/beta/{name}.toml
        format: toml
        route-field: source
        route: "Read {path}."
        identity: {title: name}
        fixed: {kind: agent, quoted: "say \"hi\" \\ bye"}
        lists: {tags: skills}
        translations:
          - {when: always, field: caps, entries: {"*": deny, "my key": allow}}
`
	tree := harnessTree(config)
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files["canon/agents/reviewer.md"] = "---\nname: reviewer\ndescription: Reviews changes.\ntier: fast\ngrants:\n  - read\n  - write\ndenials:\n  - network\nconstraints:\n  - small\n  - focused\ndispatches:\n  - helper\n  - run\nskills:\n  - tidy\n  - C++\n---\n# R\n"
	tree.files[".agents/skills/tidy/SKILL.md"] = "---\nname: tidy\ndescription: Tidy the tree.\n---\n# T\n"
	store := &memoryHarnessStore{tree: tree}
	if outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate"); outcome.ExitCode != 0 {
		t.Fatalf("generate exit = %d (%q)", outcome.ExitCode, outcome.Stderr)
	}
	wantReviewer := `---
abilities:
  - tidy
  - "C++"
about: Reviews changes.
access: read-files
effort: low
kind: agent
limits: tiny
model: model-a-1.0
note: |-
  line1
  line2
notes:
  level: high
permissions:
  "*": deny
  helper: allow
  run: allow
team: core
title: reviewer
---

Read canon/agents/reviewer.md ` + "\u2014" + ` follow it.
`
	if got := tree.files["adapters/alpha/agents/reviewer.md"]; got != wantReviewer {
		t.Fatalf("front-matter adapter = %q, want %q", got, wantReviewer)
	}
	wantBeta := "[caps]\n\"*\" = \"deny\"\n\"my key\" = \"allow\"\nkind = \"agent\"\nquoted = 'say \"hi\" \\ bye'\nsource = \"Read canon/agents/reviewer.md.\"\ntags = [\"tidy\", \"C++\"]\ntitle = \"reviewer\"\n"
	if got := tree.files["adapters/beta/reviewer.toml"]; got != wantBeta {
		t.Fatalf("toml agent adapter = %q, want %q", got, wantBeta)
	}
	wantSkill := "[caps]\n\"*\" = \"deny\"\n\"my key\" = \"allow\"\nkind = \"skill\"\nquoted = 'say \"hi\" \\ bye'\nsource = \"Read .agents/skills/tidy/SKILL.md.\"\ntitle = \"tidy\"\n"
	if got := tree.files["adapters/alpha/skills/tidy.toml"]; got != wantSkill {
		t.Fatalf("toml skill adapter = %q, want %q", got, wantSkill)
	}
	if validated := runHarnessValidate(tree, "harness", "adapters", "validate"); validated.ExitCode != 0 {
		t.Fatalf("validate = %+v", validated)
	}
}

func TestHarnessSelectsDeclaredAgents(t *testing.T) {
	config := `harness:
  canonical:
    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints}
  profiles:
    - id: alpha
      agent-adapter:
        path: adapters/alpha/{name}.md
        format: front-matter
        route-field: body
        route: "Read {path}."
        agents: [kept]
`
	tree := harnessTree(config)
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files[".agents/agents/kept.md"] = harnessAgent("kept")
	tree.files[".agents/agents/dropped.md"] = harnessAgent("dropped")
	store := &memoryHarnessStore{tree: tree}
	if outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate"); outcome.ExitCode != 0 {
		t.Fatalf("generate exit = %d (%q)", outcome.ExitCode, outcome.Stderr)
	}
	if _, present := tree.files["adapters/alpha/kept.md"]; !present {
		t.Fatalf("selected agent was not rendered: %v", fileNames(tree))
	}
	if _, present := tree.files["adapters/alpha/dropped.md"]; present {
		t.Fatalf("unselected agent was rendered")
	}
	catalog := tree.files["adapters/alpha/catalog.json"]
	if !strings.Contains(catalog, "agent/kept") || strings.Contains(catalog, "agent/dropped") {
		t.Fatalf("catalog selects wrongly: %q", catalog)
	}
}

func harnessAgent(name string) string {
	return "---\nname: " + name + "\ndescription: Agent " + name + ".\ngrants:\n  - read\ndenials:\n  - net\n---\n"
}

func fileNames(tree *memoryTree) []string {
	names := make([]string, 0, len(tree.files))
	for name := range tree.files {
		names = append(names, name)
	}
	return names
}

func TestHarnessGenerateWithoutModelsWarnsAndFallsBack(t *testing.T) {
	config := "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints, tier: tier}\n  profiles:\n    - id: alpha\n      agent-adapter: {path: 'adapters/alpha/{name}.md', format: front-matter, route-field: body, route: 'See {path}.', tier-fields: {model: model, effort: effort}}\n      tiers: {fast: {model-resolve: {command: [list-models], pattern: 'model-*', fallback: model-0.9}, effort: low}}\n"
	tree := harnessTree(config)
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files[".agents/agents/reviewer.md"] = "---\nname: reviewer\ndescription: Review.\ntier: fast\n---\n"
	store := &memoryHarnessStore{tree: tree}
	outcome := runHarnessGenerate(tree, store, nil, "harness", "adapters", "generate")
	if outcome.ExitCode != 0 {
		t.Fatalf("generate exit = %d (%q)", outcome.ExitCode, outcome.Stderr)
	}
	wantWarning := "[harness-adapters] warning: profile `alpha` tier `fast` resolved no model (this entry point has no model-resolution process boundary); using the fallback `model-0.9`\n"
	if outcome.Stderr != wantWarning {
		t.Fatalf("stderr = %q, want %q", outcome.Stderr, wantWarning)
	}
	if got := tree.files["adapters/alpha/reviewer.md"]; !strings.Contains(got, "model: model-0.9\n") {
		t.Fatalf("adapter = %q", got)
	}
}

func TestHarnessFrontMatterEdgeCases(t *testing.T) {
	rows := []struct {
		name     string
		contents string
		cause    string
	}{
		{"indented key that is not a list", "---\nname: reviewer\ndescription: Review changes.\n  stray\n---\n",
			"canonical source `.agents/agents/reviewer.md` has malformed front matter"},
		{"indented line inside a list without a dash", "---\nname: reviewer\ndescription: Review changes.\ngrants:\n  - read\n  stray\ndenials:\n  - net\n---\n",
			"canonical source `.agents/agents/reviewer.md` has a non-list metadata value"},
		{"duplicated list member", "---\nname: reviewer\ndescription: Review changes.\ngrants:\n  - read\n  - read\ndenials:\n  - net\n---\n",
			"canonical source `.agents/agents/reviewer.md` has an invalid metadata list"},
		{"blank member", "---\nname: reviewer\ndescription: Review changes.\ngrants:\n  - read\n  -\ndenials:\n  - net\n---\n",
			"canonical source `.agents/agents/reviewer.md` has a non-list metadata value"},
	}
	prefix := "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints}\n  profiles:\n    - id: alpha\n" + harnessFrontMatterAdapter
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			tree := harnessTree(prefix)
			tree.files["AGENTS.md"] = harnessRootInstruction
			tree.files[".agents/agents/reviewer.md"] = row.contents
			outcome := runHarnessValidate(tree, "harness", "adapters", "validate")
			if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "[harness-adapters] "+row.cause) {
				t.Fatalf("outcome = %+v", outcome)
			}
		})
	}
	t.Run("nbsp indent is whitespace", func(t *testing.T) {
		tree := harnessTree(prefix)
		tree.files["AGENTS.md"] = harnessRootInstruction
		tree.files[".agents/agents/reviewer.md"] = "---\n name: reviewer\ndescription: Review changes.\ngrants:\n  - read\ndenials:\n  - net\n---\n"
		outcome := runHarnessValidate(tree, "harness", "adapters", "validate")
		if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "has malformed front matter") {
			t.Fatalf("outcome = %+v", outcome)
		}
	})
}

func TestHarnessTOMLControlEscapes(t *testing.T) {
	config := "harness:\n  canonical:\n    agents: {name: name, description: description, grants: grants, denials: denials, constraints: constraints}\n  profiles:\n    - id: alpha\n      agent-adapter: {path: 'adapters/alpha/{name}.toml', format: toml, route-field: source, route: 'See {path}.', fixed: {tabbed: \"a\\tb\", rung: \"a\\rb\", chimed: \"a\\bb\", fed: \"a\\fb\", unit: \"a\\u0001b\"}}\n"
	tree := harnessTree(config)
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files[".agents/agents/reviewer.md"] = harnessReviewerAgent
	store := &memoryHarnessStore{tree: tree}
	if outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate"); outcome.ExitCode != 0 {
		t.Fatalf("generate exit = %d (%q)", outcome.ExitCode, outcome.Stderr)
	}
	want := "chimed = \"a\\bb\"\nfed = \"a\\fb\"\nrung = \"a\\rb\"\nsource = \"See .agents/agents/reviewer.md.\"\ntabbed = \"a\\tb\"\nunit = \"a\\u0001b\"\n"
	if got := tree.files["adapters/alpha/reviewer.toml"]; got != want {
		t.Fatalf("adapter = %q, want %q", got, want)
	}
}

func TestHarnessTierModelAndEffortTravelTogether(t *testing.T) {
	rows := []struct {
		name  string
		tier  string
		cause string
	}{
		{"model without effort", "model: m1", "declares a model or an effort without the other; a tier pins both or neither"},
		{"effort without model", "effort: low", "declares a model or an effort without the other; a tier pins both or neither"},
		{"model and resolve together", "model: m1\nmodel-resolve: {command: [list], pattern: \"x-*\", fallback: x-1}",
			"declares both a model and a model-resolve; a tier names one model"},
		{"resolve without effort", "model-resolve: {command: [list], pattern: \"x-*\", fallback: x-1}",
			"declares a model or an effort without the other; a tier pins both or neither"},
		{"empty resolve executable", "model-resolve: {command: [''], pattern: \"x-*\", fallback: x-1}\neffort: low",
			"declares a model-resolve command with no executable"},
		{"starless pattern", "model-resolve: {command: [list], pattern: \"plain\", fallback: x-1}\neffort: low",
			"declares a model-resolve pattern that is not one `*` among model-name characters"},
		{"double star pattern", "model-resolve: {command: [list], pattern: \"x-*-*\", fallback: x-1}\neffort: low",
			"declares a model-resolve pattern that is not one `*` among model-name characters"},
		{"spaced pattern", "model-resolve: {command: [list], pattern: \"x- *\", fallback: x-1}\neffort: low",
			"declares a model-resolve pattern that is not one `*` among model-name characters"},
		{"empty fallback", "model-resolve: {command: [list], pattern: \"x-*\", fallback: ''}\neffort: low",
			"declares a model-resolve fallback that is not a model name"},
		{"spaced fallback", "model-resolve: {command: [list], pattern: \"x-*\", fallback: 'x 1'}\neffort: low",
			"declares a model-resolve fallback that is not a model name"},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			for _, leaf := range [][]string{{"harness", "adapters", "validate"}, {"harness", "adapters", "generate"}} {
				tree := harnessTree("harness:\n  profiles:\n    - id: alpha\n      tiers:\n        fast:\n          " + strings.ReplaceAll(row.tier, "\n", "\n          ") + "\n")
				tree.files["AGENTS.md"] = harnessRootInstruction
				store := &memoryHarnessStore{tree: tree}
				outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, leaf...)
				if outcome.ExitCode != 2 {
					t.Fatalf("%v: exit = %d, want 2", leaf, outcome.ExitCode)
				}
				want := "[harness-adapters-" + leaf[2] + "] repo-config.yml: line 1: harness.profiles.alpha.tiers.fast: " + row.cause
				if !strings.Contains(outcome.Stderr, want) {
					t.Fatalf("%v: stderr = %q, want it to contain %q", leaf, outcome.Stderr, want)
				}
			}
		})
	}
}
