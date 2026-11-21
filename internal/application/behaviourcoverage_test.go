package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// behaviourConfig declares the corpus, the fixtures, and the three levels, each
// with a glob that matches one file of its own.
const behaviourConfig = `schema: rhino/repo-config/v2
policies:
  behaviour:
    corpus:
      - specs/behaviours
    fixtures:
      - test/fixtures
    levels:
      unit:
        - internal/**/*_test.go
      integration:
        - tests/integration/**
      e2e:
        - tests/e2e/**
`

// behaviourWholeFileConfig is behaviourConfig with a whole-file glob that the
// unit test file matches, so a whole-file reference is allowed there.
const behaviourWholeFileConfig = `schema: rhino/repo-config/v2
policies:
  behaviour:
    corpus:
      - specs/behaviours
    fixtures:
      - test/fixtures
    whole-file:
      - internal/**/*_test.go
    levels:
      unit:
        - internal/**/*_test.go
      integration:
        - tests/integration/**
      e2e:
        - tests/e2e/**
`

// behaviourMissingLevelConfig declares no e2e level, so the configuration is
// unusable.
const behaviourMissingLevelConfig = `schema: rhino/repo-config/v2
policies:
  behaviour:
    corpus:
      - specs/behaviours
    levels:
      unit:
        - internal/**/*_test.go
      integration:
        - tests/integration/**
`

// behaviourMissingLevelGlobConfig points the e2e level at a directory that holds
// no file, so the level inspects nothing.
const behaviourMissingLevelGlobConfig = `schema: rhino/repo-config/v2
policies:
  behaviour:
    corpus:
      - specs/behaviours
    fixtures:
      - test/fixtures
    levels:
      unit:
        - internal/**/*_test.go
      integration:
        - tests/integration/**
      e2e:
        - tests/e2e-missing/**
`

// The exemption comment and tag pairs a scenario may carry.
const (
	integrationExemption = "  # Exemption(integration): the status view has no integration boundary; alternative-proof: internal/application/status_test.go\n" +
		"  @integration-exempt\n"
	e2eExemption = "  # Exemption(e2e): the status view has no end-to-end boundary; alternative-proof: internal/application/status_test.go\n" +
		"  @e2e-exempt\n"
)

// scenarioBlock is the status scenario under the given tags and comments.
func scenarioBlock(exemptions ...string) string {
	return strings.Join(exemptions, "") + "  Scenario: Status shows queued work\n    Given a queued job\n    When status runs\n    Then the queue is shown\n"
}

// statusFeature is one feature file with one scenario under the given tags.
func statusFeature(exemptions ...string) string {
	return "Feature: Status\n\n" + scenarioBlock(exemptions...)
}

// statusTest is a unit test file holding one reference line.
func statusTest(reference string) string {
	return "package application\n\n// @scenario " + reference + "\nfunc TestStatusShowsQueuedWork() {}\n"
}

// behaviourFiles is the tree every behaviour rule accepts: a feature whose one
// scenario is exempt at integration and e2e, a unit test that binds it, and one
// file for each level glob.
func behaviourFiles() map[string]string {
	return map[string]string{
		"specs/behaviours/status.feature":     statusFeature(integrationExemption, e2eExemption),
		"internal/application/status_test.go": statusTest("specs/behaviours/status.feature: Status shows queued work"),
		"tests/integration/smoke_test.go":     "package integration\n",
		"tests/e2e/smoke_test.go":             "package e2e\n",
	}
}

// withFiles returns the accepted tree with the given files replaced or added.
func withFiles(replace map[string]string) map[string]string {
	files := behaviourFiles()
	for path, content := range replace {
		files[path] = content
	}
	return files
}

// runBehaviour runs `behaviour coverage validate` over one in-memory tree, in
// JSON, so every run is read the same way.
func runBehaviour(t *testing.T, config string, files map[string]string, extra ...string) wiringResult {
	t.Helper()
	args := append([]string{"--output", "json"}, extra...)
	return runCommand(t, []string{"behaviour", "coverage", "validate"}, config, files, args...)
}

// behaviourText runs `behaviour coverage validate` in text over one in-memory
// tree and returns the outcome as written.
func behaviourText(config string, files map[string]string) application.Outcome {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = config
	for path, content := range files {
		tree.files[path] = content
	}
	return application.Execute(tree, []string{"behaviour", "coverage", "validate"}, nil)
}

// TestBehaviourCoverageRules gives each rule one passing and one failing fixture.
func TestBehaviourCoverageRules(t *testing.T) {
	cases := []struct {
		name   string
		config string
		files  map[string]string
		exit   int
		kinds  []string
	}{
		{name: "a bound and exempt corpus passes", config: behaviourConfig, files: behaviourFiles(), exit: 0},
		{name: "B01 fails when a scenario has no unit reference", config: behaviourConfig,
			files: withFiles(map[string]string{"internal/application/status_test.go": "package application\n\nfunc TestStatusShowsQueuedWork() {}\n"}),
			exit:  1, kinds: []string{"B01"}},
		{name: "B02 passes with an integration reference and no integration exemption", config: behaviourConfig,
			files: withFiles(map[string]string{
				"specs/behaviours/status.feature":  statusFeature(e2eExemption),
				"tests/integration/status_test.go": statusTest("specs/behaviours/status.feature: Status shows queued work"),
			}), exit: 0},
		{name: "B02 fails with neither an integration reference nor an exemption", config: behaviourConfig,
			files: withFiles(map[string]string{"specs/behaviours/status.feature": statusFeature(e2eExemption)}),
			exit:  1, kinds: []string{"B02"}},
		{name: "B03 passes with an end-to-end reference and no end-to-end exemption", config: behaviourConfig,
			files: withFiles(map[string]string{
				"specs/behaviours/status.feature": statusFeature(integrationExemption),
				"tests/e2e/status_test.go":        statusTest("specs/behaviours/status.feature: Status shows queued work"),
			}), exit: 0},
		{name: "B03 fails with neither an end-to-end reference nor an exemption", config: behaviourConfig,
			files: withFiles(map[string]string{"specs/behaviours/status.feature": statusFeature(integrationExemption)}),
			exit:  1, kinds: []string{"B03"}},
		{name: "B04 fails when the comment is missing above an exemption tag", config: behaviourConfig,
			files: withFiles(map[string]string{"specs/behaviours/status.feature": statusFeature(
				"  @integration-exempt\n", e2eExemption)}),
			exit: 1, kinds: []string{"B04"}},
		{name: "B04 fails on an unknown exemption tag such as unit-exempt", config: behaviourConfig,
			files: withFiles(map[string]string{"specs/behaviours/status.feature": statusFeature(
				"  # Exemption(unit): the status view has no unit boundary; alternative-proof: internal/application/status_test.go\n"+
					"  @unit-exempt\n", integrationExemption, e2eExemption)}),
			exit: 1, kinds: []string{"B04"}},
		{name: "B04 fails when the reason is on the deny list", config: behaviourConfig,
			files: withFiles(map[string]string{"specs/behaviours/status.feature": statusFeature(
				"  # Exemption(integration): the integration run is too slow for the runtime budget; alternative-proof: internal/application/status_test.go\n"+
					"  @integration-exempt\n", e2eExemption)}),
			exit: 1, kinds: []string{"B04"}},
		{name: "B05 passes for a whole-file reference that a whole-file glob matches", config: behaviourWholeFileConfig,
			files: withFiles(map[string]string{"internal/application/status_test.go": statusTest("specs/behaviours/status.feature: *")}),
			exit:  0},
		{name: "B05 fails for a whole-file reference that no whole-file glob matches", config: behaviourConfig,
			files: withFiles(map[string]string{"internal/application/status_test.go": statusTest("specs/behaviours/status.feature: *")}),
			exit:  1, kinds: []string{"B05"}},
		{name: "B05 fails for a reference that names a scenario the feature lacks", config: behaviourConfig,
			files: withFiles(map[string]string{"internal/application/status_test.go": statusTest("specs/behaviours/status.feature: Status shows idle work")}),
			exit:  1, kinds: []string{"B05"}},
		{name: "B06 fails when an exemption is also referenced at its level", config: behaviourConfig,
			files: withFiles(map[string]string{"tests/integration/status_test.go": statusTest("specs/behaviours/status.feature: Status shows queued work")}),
			exit:  1, kinds: []string{"B06"}},
		{name: "B07 fails for a feature file outside the corpus and the fixtures", config: behaviourConfig,
			files: withFiles(map[string]string{"specs/other/status.feature": "Feature: Other\n\n  Scenario: Other holds nothing\n    Given nothing\n"}),
			exit:  1, kinds: []string{"B07"}},
		{name: "B07 passes for a feature file that is a fixture input", config: behaviourConfig,
			files: withFiles(map[string]string{"test/fixtures/sample.feature": "Feature: Sample\n\n  Scenario: Sample holds nothing\n    Given nothing\n"}),
			exit:  0},
		{name: "B08 fails when two scenarios share a name in one feature file", config: behaviourConfig,
			files: withFiles(map[string]string{"specs/behaviours/status.feature": "Feature: Status\n\n" +
				scenarioBlock(integrationExemption, e2eExemption) + "\n" + scenarioBlock(integrationExemption, e2eExemption)}),
			exit: 1, kinds: []string{"B08"}},
		{name: "B09 fails when a corpus root holds no scenario", config: behaviourConfig,
			files: withFiles(map[string]string{"specs/behaviours/status.feature": "Feature: Status\n"}),
			exit:  1, kinds: []string{"B09"}},
		{name: "B09 fails when a level glob matches no test file", config: behaviourMissingLevelGlobConfig,
			files: behaviourFiles(), exit: 1, kinds: []string{"B09"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := runBehaviour(t, tc.config, tc.files)
			if result.exit != tc.exit {
				t.Fatalf("exit %d; want %d\nstdout: %s\nstderr: %s", result.exit, tc.exit, result.stdout, result.stderr)
			}
			for _, kind := range tc.kinds {
				if !wiringHasKind(result.kinds, kind) {
					t.Errorf("kinds %v do not include %s", result.kinds, kind)
				}
			}
		})
	}
}

// TestBehaviourCoverageRulesFlag proves --rules runs only the named rules and
// refuses an unknown rule identifier.
func TestBehaviourCoverageRulesFlag(t *testing.T) {
	unbound := withFiles(map[string]string{"internal/application/status_test.go": "package application\n\nfunc TestStatusShowsQueuedWork() {}\n"})

	skipped := runBehaviour(t, behaviourConfig, unbound, "--rules", "B04,B05,B06,B07,B08")
	if skipped.exit != 0 {
		t.Fatalf("--rules B04,B05,B06,B07,B08 over a B01 failure: exit %d; want 0\nstdout: %s", skipped.exit, skipped.stdout)
	}

	named := runBehaviour(t, behaviourConfig, unbound, "--rules", "B01")
	if named.exit != 1 || !wiringHasKind(named.kinds, "B01") {
		t.Fatalf("--rules B01: exit %d kinds %v; want exit 1 with B01", named.exit, named.kinds)
	}

	unknown := runBehaviour(t, behaviourConfig, behaviourFiles(), "--rules", "B10")
	if unknown.exit != 2 || !strings.Contains(unknown.stderr, "B10") {
		t.Fatalf("--rules B10: exit %d stderr %q; want exit 2 naming B10", unknown.exit, unknown.stderr)
	}
}

// TestBehaviourCoverageRefusesUnusableConfig proves an undeclared section and an
// unusable one each refuse with the closed code for their case.
func TestBehaviourCoverageRefusesUnusableConfig(t *testing.T) {
	undeclared := runBehaviour(t, "schema: rhino/repo-config/v2\n", behaviourFiles())
	if undeclared.exit != 2 || undeclared.code != "rhino.config.undeclared" {
		t.Fatalf("undeclared: exit %d code %q; want 2 rhino.config.undeclared", undeclared.exit, undeclared.code)
	}
	missing := runBehaviour(t, behaviourMissingLevelConfig, behaviourFiles())
	if missing.exit != 2 || missing.code != "rhino.config.unusable" {
		t.Fatalf("missing e2e level: exit %d code %q; want 2 rhino.config.unusable", missing.exit, missing.code)
	}
}

// TestBehaviourCoverageTextNamesRuleAndMessage proves the text rendering names
// the rule, the file, and the message, and that the stale-exemption message is
// the one the spec gives.
func TestBehaviourCoverageTextNamesRuleAndMessage(t *testing.T) {
	stale := withFiles(map[string]string{"tests/integration/status_test.go": statusTest("specs/behaviours/status.feature: Status shows queued work")})
	outcome := behaviourText(behaviourConfig, stale)
	if outcome.ExitCode != 1 {
		t.Fatalf("exit %d; want 1\nstderr: %s", outcome.ExitCode, outcome.Stderr)
	}
	for _, want := range []string{"specs/behaviours/status.feature:", "B06", "stale exemption — remove the tag"} {
		if !strings.Contains(outcome.Stderr, want) {
			t.Errorf("stderr does not contain %q:\n%s", want, outcome.Stderr)
		}
	}
}
