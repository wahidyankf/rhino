package application_test

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

const qualityGateBase = `schema: rhino/repo-config/v2
policies:
  governance:
    quality-gates:
      root: flows
      agents: agents
      groups: [plan, quality]
      gate-group: quality
      gate-headings: [Entry, Inputs, Verdict]
      verdict-heading: Verdict
      verdicts: [PASS]
      retired-inputs: [max-audits]
      propagation-headings: [Contract, Scope]
      gates: [{family: plan}]
`

const qualityGateFile = "# Gate\n\n## Entry\n\n## Inputs\n\n| Input | Default |\n| `max-cycles` | 3 |\n\n## Verdict\n\n`PASS`\n"

const qualityPropagationFile = "# Propagation\n\n## Contract\n\n## Scope\n"

func qualityTree(config string, files map[string]string) *memoryTree {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = config
	for path, content := range files {
		tree.files[path] = content
	}
	return tree
}

func conformingQualityTree() *memoryTree {
	return qualityTree(qualityGateBase, map[string]string{
		"flows/README.md":                    "# Flows\n",
		"flows/quality/plan-quality-gate.md": qualityGateFile,
		"flows/quality/plan-propagation.md":  qualityPropagationFile,
		"agents/plan-checker.md":             "# Checker\n",
		"agents/plan-fixer.md":               "# Fixer\n",
	})
}

func TestExecuteQualityGatesFindings(t *testing.T) {
	cases := []struct {
		name     string
		setup    func(tree *memoryTree)
		config   string
		exitCode int
		contains []string
		absent   []string
	}{
		{
			name:     "a conforming layout is clean and lists what it read",
			setup:    func(tree *memoryTree) {},
			exitCode: 0,
			contains: []string{
				`"command":"quality-gates"`,
				`"exitCode":0`,
				`"inspected":2`,
				`"scanned":["flows/quality/plan-propagation.md","flows/quality/plan-quality-gate.md"]`,
			},
		},
		{
			name: "the workflow root holds its index and declared groups only",
			setup: func(tree *memoryTree) {
				tree.files["flows/notes.md"] = "# Notes\n"
				tree.files["flows/README.md/stray.md"] = "# Stray\n"
			},
			exitCode: 1,
			contains: []string{
				`"kind":"unexpected-workflow-entry"`,
				`"path":"flows/notes.md"`,
				`"path":"flows/README.md"`,
				`"rule":"QG01"`,
			},
		},
		{
			name: "a module directory is paired",
			setup: func(tree *memoryTree) {
				tree.files["flows/plan/bare/001-one.md"] = "# One\n"
			},
			exitCode: 1,
			contains: []string{
				`"kind":"unpaired-workflow-module-directory","path":"flows/plan/bare"`,
				`"kind":"missing-workflow-module-index","path":"flows/plan/bare/README.md"`,
				`"rule":"QG10"`,
			},
		},
		{
			name: "a module directory holds only its index and numbered modules",
			setup: func(tree *memoryTree) {
				tree.files["flows/plan/run.md"] = "# Run\n"
				tree.files["flows/plan/run/README.md"] = "- [One](001-one.md)\n"
				tree.files["flows/plan/run/001-one.md"] = "# One\n"
				tree.files["flows/plan/run/002-two.md"] = "# Two\n"
				tree.files["flows/plan/run/notes.md"] = "# Notes\n"
				tree.files["flows/plan/run/nested/001-deep.md"] = "# Deep\n"
			},
			exitCode: 1,
			contains: []string{
				`"kind":"unlisted-workflow-module","path":"flows/plan/run/002-two.md"`,
				`"kind":"unexpected-workflow-module-entry","path":"flows/plan/run/notes.md"`,
				`"kind":"unexpected-workflow-module-entry","path":"flows/plan/run/nested"`,
			},
			absent: []string{"001-one.md\",\"message\":\"the module directory"},
		},
		{
			name: "gate files sit in the gate group for declared families only",
			setup: func(tree *memoryTree) {
				tree.files["flows/plan/docs-propagation.md"] = qualityPropagationFile
				tree.files["flows/quality/ci-quality-gate.md"] = qualityGateFile
				tree.files["elsewhere/x-quality-gate.md"] = qualityGateFile
				tree.files["flows/quality/quality-gate.md"] = "# Bare\n"
			},
			exitCode: 1,
			contains: []string{
				`"kind":"misplaced-quality-gate-file","path":"flows/plan/docs-propagation.md"`,
				`"kind":"undeclared-quality-gate-family","path":"flows/quality/ci-quality-gate.md"`,
				`"family":"ci"`,
				`"rule":"QG03"`,
			},
			absent: []string{
				"elsewhere",
				"flows/quality/quality-gate.md\",\"message",
			},
		},
		{
			name: "a declared family names each missing file and agent",
			setup: func(tree *memoryTree) {
				delete(tree.files, "flows/quality/plan-quality-gate.md")
				delete(tree.files, "flows/quality/plan-propagation.md")
				delete(tree.files, "agents/plan-checker.md")
				delete(tree.files, "agents/plan-fixer.md")
			},
			exitCode: 1,
			contains: []string{
				`"kind":"missing-quality-gate-file","path":"flows/quality/plan-quality-gate.md"`,
				`"kind":"missing-quality-gate-file","path":"flows/quality/plan-propagation.md"`,
				`"kind":"missing-gate-agent","path":"agents/plan-checker.md"`,
				`"kind":"missing-gate-agent","path":"agents/plan-fixer.md"`,
				`"rule":"QG02"`,
				`"rule":"QG08"`,
			},
		},
		{
			name: "gate headings are matched in order and outside fences",
			setup: func(tree *memoryTree) {
				tree.files["flows/quality/plan-quality-gate.md"] = "# Gate\n\n```text\n## Entry\n```\n\n## Verdict\n\n`PASS`\n\n## Inputs\n\n| `max-cycles` | 3 |\n"
			},
			exitCode: 1,
			contains: []string{
				`"kind":"missing-gate-heading"`,
				`"kind":"gate-heading-out-of-order"`,
				`"rule":"QG04"`,
			},
		},
		{
			name: "only capitalised code spans in the verdict section read as verdicts",
			setup: func(tree *memoryTree) {
				tree.files["flows/quality/plan-quality-gate.md"] = "## Entry\n\n## Inputs\n\n| `max-cycles` | 3 |\n\n## Verdict\n\n`PASS`, `mode`, `A`, `HOLD_2`\n\n## After\n\n`LATER`\n"
			},
			exitCode: 1,
			contains: []string{
				`"kind":"unknown-gate-verdict"`,
				`"verdict":"HOLD_2"`,
				`"rule":"QG07"`,
			},
			absent: []string{
				`"verdict":"PASS"`,
				`"verdict":"mode"`,
				`"verdict":"A"`,
				"LATER",
			},
		},
		{
			name: "the gate names a retired input",
			setup: func(tree *memoryTree) {
				tree.files["flows/quality/plan-quality-gate.md"] = "## Entry\n\n## Inputs\n\n| `max-cycles` | 3 |\n\nUses no `max-audits` here.\n\n## Verdict\n\n`PASS`\n"
			},
			exitCode: 1,
			contains: []string{
				`"kind":"retired-gate-input"`,
				`"input":"max-audits"`,
				`"rule":"QG06"`,
			},
		},
		{
			name: "a gate without a bounded max-cycles input is unbounded",
			setup: func(tree *memoryTree) {
				tree.files["flows/quality/plan-quality-gate.md"] = "## Entry\n\n## Inputs\n\n| Input | Default |\n\n## Verdict\n\n`PASS`\n"
			},
			exitCode: 1,
			contains: []string{
				`"kind":"unbounded-gate-cycles"`,
				"the gate declares no max-cycles input bounded by 1 to 3",
			},
		},
		{
			name: "a missing propagation heading is reported",
			setup: func(tree *memoryTree) {
				tree.files["flows/quality/plan-propagation.md"] = "# Propagation\n\n## Contract\n"
			},
			exitCode: 1,
			contains: []string{
				`"kind":"missing-propagation-heading"`,
				`"heading":"Scope"`,
				`"rule":"QG05"`,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := conformingQualityTree()
			if tc.config != "" {
				tree.files["repo-config.yml"] = tc.config
			}
			tc.setup(tree)
			outcome := application.Execute(tree, args("governance", "quality-gates", "validate", "--output", "json"), nil)
			if outcome.ExitCode != tc.exitCode {
				t.Fatalf("expected exit %d: %+v", tc.exitCode, outcome)
			}
			for _, want := range tc.contains {
				if !strings.Contains(outcome.Stdout+outcome.Stderr, want) {
					t.Errorf("missing %q in:\n%s\n%s", want, outcome.Stdout, outcome.Stderr)
				}
			}
			for _, missing := range tc.absent {
				if strings.Contains(outcome.Stdout+outcome.Stderr, missing) {
					t.Errorf("unexpected %q in:\n%s\n%s", missing, outcome.Stdout, outcome.Stderr)
				}
			}
		})
	}
}

func TestExecuteQualityGatesCycleBounds(t *testing.T) {
	rows := []struct {
		name    string
		row     string
		bounded bool
	}{
		{"values and default inside the ceiling", "| `max-cycles` | 1, 2, or 3 | 3 |", true},
		{"bare name with a bounded default", "| max-cycles | 3", true},
		{"a value above the ceiling", "| `max-cycles` | 1 to 4 | 3 |", false},
		{"a default above the ceiling", "| `max-cycles` | 1, 2, or 3 | 0 |", false},
		{"a default that is no number", "| `max-cycles` | 1, 2, or 3 | three |", false},
		{"a row with no values", "| `max-cycles` |", false},
		{"a row naming another input", "| Input | Default |", false},
	}
	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			tree := conformingQualityTree()
			tree.files["flows/quality/plan-quality-gate.md"] = "## Entry\n\n## Inputs\n\n" + tc.row + "\n\n## Verdict\n\n`PASS`\n"
			outcome := application.Execute(tree, args("governance", "quality-gates", "validate", "--output", "json"), nil)
			if bounded := !strings.Contains(outcome.Stdout+outcome.Stderr, "unbounded-gate-cycles"); bounded != tc.bounded {
				t.Fatalf("row %q: bounded=%v, output:\n%s\n%s", tc.row, bounded, outcome.Stdout, outcome.Stderr)
			}
		})
	}
}

func withWordBudget(config, glob string, fail int, rule string) string {
	if rule == "" {
		rule = "whitespace-separated"
	}
	return config + "    word-budget:\n      count: " + rule + "\n      surfaces:\n        - glob: \"" + glob + "\"\n          fail: " + strconv.Itoa(fail) + "\n"
}

func TestExecuteQualityGatesSplitMeasuredAgainstGoverningBudget(t *testing.T) {
	base := func() *memoryTree {
		tree := conformingQualityTree()
		tree.files["flows/plan/run.md"] = "one two three"
		tree.files["flows/plan/run/README.md"] = "- [One](001-one.md)\n"
		tree.files["flows/plan/run/001-one.md"] = "four five"
		return tree
	}
	fits := application.Execute(withBudgetTree(base(), "flows/**/*.md", 5), args("governance", "quality-gates", "validate", "--output", "json"), nil)
	if !strings.Contains(fits.Stdout, `"words":5,"limit":5`) {
		t.Fatalf("fitting split not reported:\n%s\n%s", fits.Stdout, fits.Stderr)
	}
	over := application.Execute(withBudgetTree(base(), "flows/**/*.md", 4), args("governance", "quality-gates", "validate"), nil)
	if strings.Contains(over.Stdout+over.Stderr, "unnecessary-workflow-split") {
		t.Fatalf("over-budget split accused:\n%s\n%s", over.Stdout, over.Stderr)
	}
	ungoverned := application.Execute(withBudgetTree(base(), "docs/**/*.md", 50), args("governance", "quality-gates", "validate"), nil)
	if strings.Contains(ungoverned.Stdout+ungoverned.Stderr, "unnecessary-workflow-split") {
		t.Fatalf("ungoverned split accused:\n%s\n%s", ungoverned.Stdout, ungoverned.Stderr)
	}
	unsplit := conformingQualityTree()
	unsplit.files["flows/plan/empty.md"] = "one"
	unsplit.files["flows/plan/empty/README.md"] = "# Empty\n"
	emptyOutcome := application.Execute(withBudgetTree(unsplit, "flows/**/*.md", 50), args("governance", "quality-gates", "validate"), nil)
	if strings.Contains(emptyOutcome.Stdout+emptyOutcome.Stderr, `unnecessary-workflow-split","path":"flows/plan/empty.md`) {
		t.Fatalf("module-less directory accused:\n%s\n%s", emptyOutcome.Stdout, emptyOutcome.Stderr)
	}
}

func withBudgetTree(tree *memoryTree, glob string, fail int) *memoryTree {
	tree.files["repo-config.yml"] = withWordBudget(qualityGateBase, glob, fail, "")
	return tree
}

func mutateQualityConfig(replace, with string) string {
	if !strings.Contains(qualityGateBase, replace) {
		panic("anchor missing: " + replace)
	}
	return strings.Replace(qualityGateBase, replace, with, 1)
}

func TestExecuteQualityGatesDeclarationRefusals(t *testing.T) {
	cases := []struct {
		name   string
		config string
		reason string
	}{
		{"a climbing root cannot be applied", mutateQualityConfig("      root: flows\n", "      root: ../flows\n"), "exact repository-relative"},
		{"an absolute agent directory cannot be applied", mutateQualityConfig("      agents: agents\n", "      agents: /agents\n"), "exact repository-relative"},
		{"groups without the gate group cannot be applied", mutateQualityConfig("      groups: [plan, quality]\n", "      groups: [plan]\n"), "include `gate-group`"},
		{"repeated groups cannot be applied", mutateQualityConfig("      groups: [plan, quality]\n", "      groups: [plan, quality, plan]\n"), "include `gate-group`"},
		{"non-simple groups cannot be applied", mutateQualityConfig("      groups: [plan, quality]\n", "      groups: [Plan, quality]\n"), "include `gate-group`"},
		{"repeated families cannot be applied", mutateQualityConfig("      gates: [{family: plan}]\n", "      gates: [{family: plan}, {family: plan}]\n"), "unique simple name"},
		{"a non-simple family cannot be applied", mutateQualityConfig("      gates: [{family: plan}]\n", "      gates: [{family: p_lan}]\n"), "unique simple name"},
		{"a non-simple judge cannot be applied", mutateQualityConfig("      gates: [{family: plan}]\n", "      gates: [{family: plan, judge: plan_tester}]\n"), "simple agent name"},
		{"a non-simple repairer cannot be applied", mutateQualityConfig("      gates: [{family: plan}]\n", "      gates: [{family: plan, repairer: \"plan fixer\"}]\n"), "simple agent name"},
		{"repeated gate headings cannot be applied", mutateQualityConfig("      gate-headings: [Entry, Inputs, Verdict]\n", "      gate-headings: [Entry, Entry]\n"), "non-empty and unique"},
		{"a blank gate heading cannot be applied", mutateQualityConfig("      gate-headings: [Entry, Inputs, Verdict]\n", "      gate-headings: [' ', Verdict]\n"), "non-empty and unique"},
		{"no verdicts cannot be applied", mutateQualityConfig("      verdicts: [PASS]\n", "      verdicts: []\n"), "non-empty and unique"},
		{"a blank retired input cannot be applied", mutateQualityConfig("      retired-inputs: [max-audits]\n", "      retired-inputs: ['']\n"), "non-empty and unique"},
		{"no propagation headings cannot be applied", mutateQualityConfig("      propagation-headings: [Contract, Scope]\n", "      propagation-headings: []\n"), "non-empty and unique"},
		{"a verdict heading outside the gate headings cannot be applied", mutateQualityConfig("      verdict-heading: Verdict\n", "      verdict-heading: Outcome\n"), "one of `gate-headings`"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := qualityTree(tc.config, map[string]string{})
			outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
			if outcome.ExitCode != 2 {
				t.Fatalf("expected exit 2: %+v", outcome)
			}
			for _, want := range []string{"policies.governance.quality-gates", tc.reason} {
				if !strings.Contains(outcome.Stderr, want) {
					t.Errorf("missing %q in:\n%s", want, outcome.Stderr)
				}
			}
		})
	}
}

func TestExecuteQualityGatesMalformedShapes(t *testing.T) {
	cases := []struct {
		name   string
		config string
		want   string
	}{
		{"an unknown policy key is refused", mutateQualityConfig("      root: flows\n", "      root: flows\n      extra: 1\n"), "names unknown key `extra`"},
		{"a missing workflow root is refused", mutateQualityConfig("      root: flows\n", ""), "quality-gates.root names no workflow root"},
		{"a non-string workflow root is refused", mutateQualityConfig("      root: flows\n", "      root: 3\n"), "quality-gates.root names no workflow root"},
		{"a non-list group set is refused", mutateQualityConfig("      groups: [plan, quality]\n", "      groups: plan\n"), "quality-gates.groups names no workflow groups"},
		{"a non-string group is refused", mutateQualityConfig("      groups: [plan, quality]\n", "      groups: [plan, 3]\n"), "quality-gates.groups names no workflow groups"},
		{"a non-list family set is refused", mutateQualityConfig("      gates: [{family: plan}]\n", "      gates: plan\n"), "quality-gates.gates names no gate families"},
		{"a non-mapping family reads as no family", mutateQualityConfig("      gates: [{family: plan}]\n", "      gates: [plan]\n"), "unique simple name"},
		{"an unknown family key is refused", mutateQualityConfig("      gates: [{family: plan}]\n", "      gates: [{family: plan, boss: x}]\n"), "name unknown key `boss`"},
		{"a non-string judge is refused", mutateQualityConfig("      gates: [{family: plan}]\n", "      gates: [{family: plan, judge: 3}]\n"), "gates.judge names no agent"},
		{"a non-string repairer is refused", mutateQualityConfig("      gates: [{family: plan}]\n", "      gates: [{family: plan, repairer: 3}]\n"), "gates.repairer names no agent"},
		{"a non-string subject is refused", mutateQualityConfig("      gates: [{family: plan}]\n", "      gates: [{family: plan, subject: 3}]\n"), "gates.subject names no subject"},
		{"a non-mapping defaults is refused", mutateQualityConfig("      gates: [{family: plan}]\n", "      gates: [{family: plan}]\n      defaults: [1]\n"), "quality-gates.defaults names no run defaults"},
		{"an unknown defaults key is refused", mutateQualityConfig("      gates: [{family: plan}]\n", "      gates: [{family: plan}]\n      defaults: {turbo: 1}\n"), "defaults names unknown key `turbo`"},
		{"an unknown gate mode is refused", mutateQualityConfig("      gates: [{family: plan}]\n", "      gates: [{family: plan}]\n      defaults: {mode: turbo}\n"), "defaults.mode names no gate mode"},
		{"a non-integer cycle ceiling is refused", mutateQualityConfig("      gates: [{family: plan}]\n", "      gates: [{family: plan}]\n      defaults: {max-cycles: three}\n"), "max-cycles: must be 1, 2, or 3"},
		{"a non-list word-budget surface set is refused", qualityGateBase + "    word-budget:\n      count: whitespace-separated\n      surfaces: flows\n", "word-budget.surfaces names no budget surfaces"},
		{"a non-mapping word-budget surface is refused", qualityGateBase + "    word-budget:\n      count: whitespace-separated\n      surfaces: [flows]\n", "word-budget.surfaces names no budget surfaces"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := qualityTree(tc.config, map[string]string{})
			outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
			if outcome.ExitCode != 2 {
				t.Fatalf("expected exit 2: %+v", outcome)
			}
			if !strings.Contains(outcome.Stderr, tc.want) {
				t.Errorf("missing %q in:\n%s", tc.want, outcome.Stderr)
			}
		})
	}
}

func TestExecuteQualityGatesWordBudgetShapes(t *testing.T) {
	t.Run("an unknown word-budget key is refused", func(t *testing.T) {
		tree := qualityTree(qualityGateBase+"    word-budget:\n      count: whitespace-separated\n      unknown: 1\n", map[string]string{})
		outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
		if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "names unknown key `unknown`") {
			t.Fatalf("unexpected outcome: %+v", outcome)
		}
	})
	t.Run("an unknown word rule is refused", func(t *testing.T) {
		tree := qualityTree(qualityGateBase+"    word-budget:\n      count: words\n", map[string]string{})
		outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
		if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "word-budget.count names no word rule") {
			t.Fatalf("unexpected outcome: %+v", outcome)
		}
	})
	t.Run("an unknown surface key is refused", func(t *testing.T) {
		config := qualityGateBase + "    word-budget:\n      count: whitespace-separated\n      surfaces:\n        - glob: \"flows/**/*.md\"\n          fail: 5\n          ceiling: 5\n"
		tree := qualityTree(config, map[string]string{})
		outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
		if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "name unknown key `ceiling`") {
			t.Fatalf("unexpected outcome: %+v", outcome)
		}
	})
	t.Run("a non-integer surface limit is refused", func(t *testing.T) {
		config := qualityGateBase + "    word-budget:\n      count: whitespace-separated\n      surfaces:\n        - glob: \"flows/**/*.md\"\n          fail: many\n"
		tree := qualityTree(config, map[string]string{})
		outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
		if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "word-budget.surfaces names no budget surfaces") {
			t.Fatalf("unexpected outcome: %+v", outcome)
		}
	})
	t.Run("an unusable surface glob is refused", func(t *testing.T) {
		config := qualityGateBase + "    word-budget:\n      count: whitespace-separated\n      surfaces:\n        - glob: \"[\"\n          fail: 5\n"
		tree := qualityTree(config, map[string]string{})
		outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
		if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "governance-word-budget.surfaces") {
			t.Fatalf("unexpected outcome: %+v", outcome)
		}
	})
	t.Run("the last matching surface governs the entrypoint", func(t *testing.T) {
		narrow := "    word-budget:\n      count: whitespace-separated\n      surfaces:\n" +
			"        - glob: \"flows/**/*.md\"\n          fail: 100\n" +
			"        - glob: \"flows/plan/run.md\"\n          fail: 4\n"
		wide := "    word-budget:\n      count: whitespace-separated\n      surfaces:\n" +
			"        - glob: \"flows/plan/run.md\"\n          fail: 4\n" +
			"        - glob: \"flows/**/*.md\"\n          fail: 100\n"
		base := func() *memoryTree {
			tree := conformingQualityTree()
			tree.files["flows/plan/run.md"] = "one two three"
			tree.files["flows/plan/run/README.md"] = "- [One](001-one.md)\n"
			tree.files["flows/plan/run/001-one.md"] = "four five"
			return tree
		}
		narrowTree := base()
		narrowTree.files["repo-config.yml"] = qualityGateBase + narrow
		narrowOutcome := application.Execute(narrowTree, args("governance", "quality-gates", "validate", "--output", "json"), nil)
		if strings.Contains(narrowOutcome.Stdout, "unnecessary-workflow-split") {
			t.Fatalf("narrow surface should govern without a finding:\n%s", narrowOutcome.Stdout)
		}
		wideTree := base()
		wideTree.files["repo-config.yml"] = qualityGateBase + wide
		wideOutcome := application.Execute(wideTree, args("governance", "quality-gates", "validate", "--output", "json"), nil)
		if !strings.Contains(wideOutcome.Stdout, `"words":5,"limit":100`) {
			t.Fatalf("wide surface did not govern:\n%s", wideOutcome.Stdout)
		}
	})
	t.Run("letters and digits count what a reader counts", func(t *testing.T) {
		config := qualityGateBase + "    word-budget:\n      count: letters-and-digits\n      surfaces:\n        - glob: \"flows/**/*.md\"\n          fail: 5\n"
		tree := conformingQualityTree()
		tree.files["repo-config.yml"] = config
		tree.files["flows/plan/run.md"] = "http://example.test/x"
		tree.files["flows/plan/run/README.md"] = "- [One](001-one.md)\n"
		tree.files["flows/plan/run/001-one.md"] = "a"
		outcome := application.Execute(tree, args("governance", "quality-gates", "validate", "--output", "json"), nil)
		if !strings.Contains(outcome.Stdout, `"words":5,"limit":5`) {
			t.Fatalf("letters-and-digits count disagrees:\n%s", outcome.Stdout)
		}
	})
	t.Run("a tree behind the agent directory link is refused", func(t *testing.T) {
		tree := conformingQualityTree()
		tree.links["agents"] = true
		outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
		if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "agents: passes through a symbolic link") {
			t.Fatalf("unexpected outcome: %+v", outcome)
		}
	})
	t.Run("an unreadable propagation refuses the run", func(t *testing.T) {
		tree := conformingQualityTree()
		tree.unreadable["flows/quality/plan-propagation.md"] = true
		outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
		if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "plan-propagation.md") {
			t.Fatalf("unexpected outcome: %+v", outcome)
		}
	})
	t.Run("an unreadable module index refuses the run", func(t *testing.T) {
		tree := conformingQualityTree()
		tree.files["flows/plan/run.md"] = "# Run\n"
		tree.files["flows/plan/run/README.md"] = "- [One](001-one.md)\n"
		tree.files["flows/plan/run/001-one.md"] = "# One\n"
		tree.unreadable["flows/plan/run/README.md"] = true
		outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
		if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "flows/plan/run/README.md") {
			t.Fatalf("unexpected outcome: %+v", outcome)
		}
	})
}

func TestExecuteQualityGatesSplitGateCountedOnce(t *testing.T) {
	tree := conformingQualityTree()
	tree.files["flows/quality/plan-quality-gate/README.md"] = "- [One](001-one.md)\n"
	tree.files["flows/quality/plan-quality-gate/001-one.md"] = "four five\n"
	outcome := application.Execute(
		withBudgetTree(tree, "flows/**/*.md", 500),
		args("governance", "quality-gates", "validate", "--output", "json"), nil)
	var report struct {
		Inspected int      `json:"inspected"`
		Scanned   []string `json:"scanned"`
	}
	if err := json.Unmarshal([]byte(outcome.Stdout), &report); err != nil {
		t.Fatalf("report is not one JSON object: %v\n%s", err, outcome.Stdout)
	}
	seen := map[string]bool{}
	for _, path := range report.Scanned {
		if seen[path] {
			t.Fatalf("duplicate scan of %q in %v", path, report.Scanned)
		}
		seen[path] = true
	}
	if report.Inspected != len(seen) {
		t.Fatalf("inspected %d but scanned %d: %+v", report.Inspected, len(seen), report)
	}
}

func TestExecuteQualityGatesRefusals(t *testing.T) {
	t.Run("an undeclared policy fails closed", func(t *testing.T) {
		tree := qualityTree("schema: rhino/repo-config/v2\n", map[string]string{})
		outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
		if outcome.ExitCode != 2 {
			t.Fatalf("expected exit 2: %+v", outcome)
		}
		if !strings.Contains(outcome.Stderr, "policies.governance.quality-gates is not declared") {
			t.Fatalf("missing declaration refusal:\n%s", outcome.Stderr)
		}
	})
	t.Run("a declared root behind a link refuses the run", func(t *testing.T) {
		tree := conformingQualityTree()
		tree.links["flows"] = true
		outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
		if outcome.ExitCode != 2 {
			t.Fatalf("expected exit 2: %+v", outcome)
		}
		if !strings.Contains(outcome.Stderr, "flows: passes through a symbolic link") {
			t.Fatalf("missing link refusal:\n%s", outcome.Stderr)
		}
	})
	t.Run("a file that cannot be read refuses the run", func(t *testing.T) {
		for name, mark := range map[string]func(*memoryTree){
			"unreadable": func(tree *memoryTree) { tree.unreadable["flows/quality/plan-quality-gate.md"] = true },
			"vanished":   func(tree *memoryTree) { tree.vanished["flows/quality/plan-quality-gate.md"] = true },
			"binary":     func(tree *memoryTree) { tree.binary["flows/quality/plan-quality-gate.md"] = true },
		} {
			tree := conformingQualityTree()
			mark(tree)
			outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
			if outcome.ExitCode != 2 {
				t.Fatalf("%s: expected exit 2: %+v", name, outcome)
			}
			if !strings.Contains(outcome.Stderr, "plan-quality-gate.md") {
				t.Fatalf("%s: missing path in refusal:\n%s", name, outcome.Stderr)
			}
		}
	})
	t.Run("a configured cycle ceiling outside 1 to 3 is refused while read", func(t *testing.T) {
		config := strings.Replace(qualityGateBase, "      gates: [{family: plan}]\n", "      gates: [{family: plan}]\n      defaults:\n        max-cycles: 9\n", 1)
		tree := qualityTree(config, map[string]string{})
		outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
		if outcome.ExitCode != 2 {
			t.Fatalf("expected exit 2: %+v", outcome)
		}
		if !strings.Contains(outcome.Stderr, "policies.governance.quality-gates.defaults.max-cycles") {
			t.Fatalf("missing ceiling refusal:\n%s", outcome.Stderr)
		}
	})
	t.Run("a declaration the check cannot apply is refused", func(t *testing.T) {
		config := strings.Replace(qualityGateBase, "      root: flows\n", "      root: ../flows\n", 1)
		tree := qualityTree(config, map[string]string{})
		outcome := application.Execute(tree, args("governance", "quality-gates", "validate"), nil)
		if outcome.ExitCode != 2 {
			t.Fatalf("expected exit 2: %+v", outcome)
		}
		if !strings.Contains(outcome.Stderr, "policies.governance.quality-gates") {
			t.Fatalf("missing declaration key:\n%s", outcome.Stderr)
		}
		if !strings.Contains(outcome.Stderr, "exact repository-relative") {
			t.Fatalf("missing declaration reason:\n%s", outcome.Stderr)
		}
	})
	t.Run("diagnostics carry their rule and repeat byte for byte", func(t *testing.T) {
		tree := qualityTree(qualityGateBase, map[string]string{"flows/extra.md": "# Extra\n"})
		first := application.Execute(tree, args("governance", "quality-gates", "validate", "--output", "json"), nil)
		second := application.Execute(tree, args("governance", "quality-gates", "validate", "--output", "json"), nil)
		if first.Stdout != second.Stdout || first.ExitCode != second.ExitCode {
			t.Fatalf("nondeterministic report:\n%+v\n%+v", first, second)
		}
		for _, want := range []string{`"rule":"QG01"`, `"rule":"QG02"`, `"rule":"QG08"`} {
			if !strings.Contains(first.Stdout, want) {
				t.Fatalf("missing %q in:\n%s", want, first.Stdout)
			}
		}
	})
}

func TestExecuteQualityGatesCustomJudgeAndRepairer(t *testing.T) {
	config := strings.Replace(qualityGateBase, "      gates: [{family: plan}]\n", "      gates:\n        - family: plan\n          judge: plan-tester\n          repairer: developer\n", 1)
	half := qualityTree(config, map[string]string{
		"flows/README.md":                    "# Flows\n",
		"flows/quality/plan-quality-gate.md": qualityGateFile,
		"flows/quality/plan-propagation.md":  qualityPropagationFile,
		"agents/developer.md":                "# Developer\n",
	})
	outcome := application.Execute(half, args("governance", "quality-gates", "validate", "--output", "json"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected exit 1: %+v", outcome)
	}
	combined := outcome.Stdout + outcome.Stderr
	for _, want := range []string{"agents/plan-tester.md", `"role":"judge"`, `"agent":"plan-tester"`} {
		if !strings.Contains(combined, want) {
			t.Fatalf("missing %q in:\n%s", want, combined)
		}
	}
	for _, missing := range []string{"plan-checker", "plan-fixer", `"role":"repairer"`} {
		if strings.Contains(combined, missing) {
			t.Fatalf("unexpected %q in:\n%s", missing, combined)
		}
	}
	half.files["agents/plan-tester.md"] = "# Tester\n"
	clean := application.Execute(half, args("governance", "quality-gates", "validate", "--output", "json"), nil)
	if clean.ExitCode != 0 {
		t.Fatalf("expected exit 0: %+v", clean)
	}
}
