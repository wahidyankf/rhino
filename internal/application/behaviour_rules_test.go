package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/domain"
)

// behaviourTree is the accepted tree with the given files replaced, and the
// given paths removed.
func behaviourTree(replace map[string]string, remove ...string) map[string]string {
	files := behaviourFiles()
	for _, path := range remove {
		delete(files, path)
	}
	for path, content := range replace {
		files[path] = content
	}
	return files
}

// exemptionBlock is the integration and e2e exemption pair a scenario carries.
var exemptionBlock = integrationExemption + e2eExemption

// TestBehaviourCoverageRulesOnEachShape proves each feature and comment shape the
// rules read: fenced `.feature.md` files, scenario outlines and examples, a
// Rule or Background that carries an exemption tag, and every malformed exemption
// comment.
func TestBehaviourCoverageRulesOnEachShape(t *testing.T) {
	const statusRef = "specs/behaviours/status.feature.md: Status shows queued work"
	cases := []struct {
		name  string
		files map[string]string
		exit  int
		kind  string
	}{
		{name: "a fenced gherkin block in a .feature.md file is the corpus", exit: 0,
			files: behaviourTree(map[string]string{
				"specs/behaviours/status.feature.md":  "# Status\n\n```gherkin\n" + statusFeature(exemptionBlock) + "```\n",
				"internal/application/status_test.go": statusTest(statusRef),
			}, "specs/behaviours/status.feature")},
		{name: "a .feature.md file whose gherkin fence holds no scenario (B09)", exit: 1, kind: "B09",
			files: behaviourTree(map[string]string{
				"specs/behaviours/status.feature.md": "# Status\n\n```gherkin\nFeature: Status\n```\n",
			}, "specs/behaviours/status.feature")},
		{name: "a scenario outline is one scenario and is bound", exit: 0,
			files: behaviourTree(map[string]string{
				"specs/behaviours/status.feature": "Feature: Status\n\n" + exemptionBlock +
					"  Scenario Outline: Status shows queued work\n    Given <state>\n\n    Examples:\n      | state |\n      | queued |\n",
			})},
		{name: "an example with no binding is a scenario that needs one (B01)", exit: 1, kind: "B01",
			files: behaviourTree(map[string]string{
				"specs/behaviours/status.feature": "Feature: Status\n\n" + exemptionBlock +
					"  Scenario: Status shows queued work\n    Given a queued job\n\n" +
					"  Example: Status shows idle work\n    Given an idle job\n",
			})},
		{name: "an exemption tag on a Rule is refused (B04)", exit: 1, kind: "B04",
			files: behaviourTree(map[string]string{
				"specs/behaviours/status.feature": "Feature: Status\n\n  @integration-exempt\n  Rule: Queue rules\n\n" +
					statusFeature(e2eExemption)[len("Feature: Status\n\n"):],
			})},
		{name: "an exemption tag on a Background is refused (B04)", exit: 1, kind: "B04",
			files: behaviourTree(map[string]string{
				"specs/behaviours/status.feature": "Feature: Status\n\n  @e2e-exempt\n  Background:\n    Given a queue\n\n" +
					statusFeature(integrationExemption)[len("Feature: Status\n\n"):],
			})},
		{name: "an exemption comment that names another level is refused (B04)", exit: 1, kind: "B04",
			files: behaviourTree(map[string]string{
				"specs/behaviours/status.feature": statusFeature(
					"  # Exemption(e2e): the status view has no integration boundary; alternative-proof: internal/application/status_test.go\n"+
						"  @integration-exempt\n", e2eExemption),
			})},
		{name: "an exemption comment with an empty reason is refused (B04)", exit: 1, kind: "B04",
			files: behaviourTree(map[string]string{
				"specs/behaviours/status.feature": statusFeature(
					"  # Exemption(integration):  ; alternative-proof: internal/application/status_test.go\n"+
						"  @integration-exempt\n", e2eExemption),
			})},
		{name: "an exemption comment with an empty target is refused (B04)", exit: 1, kind: "B04",
			files: behaviourTree(map[string]string{
				"specs/behaviours/status.feature": statusFeature(
					"  # Exemption(integration): the status view has no integration boundary; alternative-proof:\n"+
						"  @integration-exempt\n", e2eExemption),
			})},
		{name: "an exemption tag on the first line has no comment above it (B04)", exit: 1, kind: "B04",
			files: behaviourTree(map[string]string{
				"specs/behaviours/status.feature": "@integration-exempt\nScenario: Status shows queued work\n" +
					"  Given a queued job\nFeature: Status\n",
			})},
		{name: "an exemption that a reference covers at the e2e level is stale (B06)", exit: 1, kind: "B06",
			files: behaviourTree(map[string]string{
				"specs/behaviours/status.feature": statusFeature(integrationExemption, e2eExemption),
				"tests/e2e/status_test.go":        statusTest("specs/behaviours/status.feature: Status shows queued work"),
			})},
		{name: "a scenario-like line inside a docstring is text, not a scenario", exit: 0,
			files: behaviourTree(map[string]string{
				"specs/behaviours/status.feature": "Feature: Status\n\n  Background:\n    Given file \"status.feature\" contains this Markdown:\n" +
					"      \"\"\"\n      Feature: Status\n        Scenario: Status shows queued work\n      \"\"\"\n\n" +
					exemptionBlock + "  Scenario: Status shows queued work\n    Given a queued job\n",
				"internal/application/status_test.go": statusTest("specs/behaviours/status.feature: Status shows queued work"),
			})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := runBehaviour(t, behaviourConfig, tc.files)
			if result.exit != tc.exit {
				t.Fatalf("exit %d; want %d\nstdout: %s\nstderr: %s", result.exit, tc.exit, result.stdout, result.stderr)
			}
			if tc.kind != "" && !wiringHasKind(result.kinds, tc.kind) {
				t.Fatalf("kinds %v do not include %s", result.kinds, tc.kind)
			}
		})
	}
}

// TestBehaviourCoverageReferenceProblems proves B05 names each invalid reference:
// a feature that is not in the corpus, a reference with no scenario, and a
// whole-file reference to a feature that is not in the corpus.
func TestBehaviourCoverageReferenceProblems(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
	}{
		{name: "a reference to a feature outside the corpus", files: behaviourTree(map[string]string{
			"internal/application/status_test.go": statusTest("specs/other/status.feature: Status shows queued work"),
		})},
		{name: "a reference that names no scenario", files: behaviourTree(map[string]string{
			"internal/application/status_test.go": statusTest("specs/behaviours/status.feature:"),
		})},
		{name: "a whole-file reference to a feature outside the corpus", files: behaviourTree(map[string]string{
			"internal/application/status_test.go": statusTest("specs/other/status.feature: *"),
		})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := runBehaviour(t, behaviourConfig, tc.files)
			if result.exit != 1 || !wiringHasKind(result.kinds, "B05") {
				t.Fatalf("exit %d kinds %v; want exit 1 with B05", result.exit, result.kinds)
			}
		})
	}
}

// TestBehaviourCoverageRefusesInvalidInputs proves each input the rules cannot
// read refuses the run with the closed code its cause carries.
func TestBehaviourCoverageRefusesInvalidInputs(t *testing.T) {
	const invalidGlobConfig = `schema: rhino/repo-config/v2
policies:
  behaviour:
    corpus:
      - specs/behaviours
    levels:
      unit:
        - internal/[*
      integration:
        - tests/integration/**
      e2e:
        - tests/e2e/**
`
	const badScanConfig = behaviourConfig + "scan: \"not a mapping\"\n"
	cases := []struct {
		name   string
		config string
		files  map[string]string
		mutate func(*memoryTree)
		code   domain.ErrorCode
	}{
		{name: "a level glob that cannot be matched", config: invalidGlobConfig, files: behaviourFiles(),
			code: domain.ConfigUnusable},
		{name: "a scan section that is not a mapping", config: badScanConfig, files: behaviourFiles(),
			code: domain.ConfigUnusable},
		{name: "a corpus feature with no Feature line", config: behaviourConfig,
			files: behaviourTree(map[string]string{"specs/behaviours/status.feature": "Scenario: Lonely\n"}),
			code:  domain.FileUnreadable},
		{name: "an unreadable corpus feature", config: behaviourConfig, files: behaviourFiles(),
			mutate: func(tree *memoryTree) { tree.unreadable["specs/behaviours/status.feature"] = true },
			code:   domain.FileUnreadable},
		{name: "an unreadable test file", config: behaviourConfig, files: behaviourFiles(),
			mutate: func(tree *memoryTree) { tree.unreadable["internal/application/status_test.go"] = true },
			code:   domain.FileUnreadable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newMemoryTree("/root")
			tree.files["repo-config.yml"] = tc.config
			for path, content := range tc.files {
				tree.files[path] = content
			}
			if tc.mutate != nil {
				tc.mutate(tree)
			}
			result := runOnTree(t, tree, []string{"behaviour", "coverage", "validate"}, "--output", "json")
			if result.exit != 2 || result.code != tc.code.String() {
				t.Fatalf("exit %d code %q; want exit 2 with %s\nstderr: %s", result.exit, result.code, tc.code.String(), result.stderr)
			}
		})
	}
}

// TestBehaviourCoverageRulesFlagRefusesAnEmptyName proves an empty --rules value is
// not a rule, so it refuses the run rather than running no rule at all.
func TestBehaviourCoverageRulesFlagRefusesAnEmptyName(t *testing.T) {
	result := runBehaviour(t, behaviourConfig, behaviourFiles(), "--rules", "")
	if result.exit != 2 || result.code != domain.ArgsUnrecognized.String() {
		t.Fatalf("exit %d code %q; want exit 2 with %s", result.exit, result.code, domain.ArgsUnrecognized.String())
	}
	if !strings.Contains(result.message, "`--rules` names no rule") {
		t.Fatalf("message %q does not name the flag", result.message)
	}
}

// TestBehaviourCoverageNamesAnEmptyLevelAndRoot proves B09 reports a corpus root
// that holds no scenario and a level whose list is empty, each on its own finding.
func TestBehaviourCoverageNamesAnEmptyLevelAndRoot(t *testing.T) {
	const emptyRootConfig = `schema: rhino/repo-config/v2
policies:
  behaviour:
    corpus:
      - specs/behaviours
      - specs/empty
    levels:
      unit:
        - internal/**/*_test.go
      integration:
        - tests/integration/**
      e2e: []
`
	result := runBehaviour(t, emptyRootConfig, behaviourFiles())
	if result.exit != 1 || !wiringHasKind(result.kinds, "B09") {
		t.Fatalf("exit %d kinds %v; want exit 1 with B09", result.exit, result.kinds)
	}
	count := 0
	for _, kind := range result.kinds {
		if kind == "B09" {
			count++
		}
	}
	if count < 2 {
		t.Fatalf("B09 findings %d; want one for the empty root and one for the empty e2e level", count)
	}
}
