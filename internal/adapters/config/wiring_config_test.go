package config_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/config"
)

// wiringDocument parses a configuration whose gates section is the given text.
func wiringDocument(t *testing.T, gates string) *config.Document {
	t.Helper()
	document, err := config.Parse("schema: rhino/repo-config/v2\ngates:\n" + gates)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return document
}

// TestWiringDeclarationRefusals proves every malformed gates.wiring section is
// refused with a message that names the field at fault.
func TestWiringDeclarationRefusals(t *testing.T) {
	cases := []struct {
		name    string
		gates   string
		message string
	}{
		{name: "a wiring section that is not a mapping", gates: "  wiring: [x]\n", message: "gates.wiring must be a mapping"},
		{name: "an unknown key in the wiring section", message: "unknown key `bogus`",
			gates: "  wiring:\n    hooks:\n      directory: .husky\n    ci:\n      none: no workflows\n    bogus: 1\n"},
		{name: "hooks with no directory", message: "gates.wiring.hooks.directory",
			gates: "  wiring:\n    hooks: {}\n    ci:\n      none: no workflows\n"},
		{name: "an absolute hook directory", message: "gates.wiring.hooks.directory",
			gates: "  wiring:\n    hooks:\n      directory: /abs\n    ci:\n      none: no workflows\n"},
		{name: "a hook directory that climbs out of the root", message: "gates.wiring.hooks.directory",
			gates: "  wiring:\n    hooks:\n      directory: ../hooks\n    ci:\n      none: no workflows\n"},
		{name: "a ci section that is not a mapping", message: "gates.wiring.ci declares neither",
			gates: "  wiring:\n    hooks:\n      directory: .husky\n    ci: [x]\n"},
		{name: "a missing ci section", message: "gates.wiring.ci declares neither",
			gates: "  wiring:\n    hooks:\n      directory: .husky\n"},
		{name: "an unknown key in the ci section", message: "gates.wiring.ci names unknown key `other`",
			gates: "  wiring:\n    hooks:\n      directory: .husky\n    ci:\n      other: x\n"},
		{name: "a ci section that names both a directory and a reason", message: "declares both directory and none",
			gates: "  wiring:\n    hooks:\n      directory: .husky\n    ci:\n      directory: .github/workflows\n      none: x\n"},
		{name: "a ci reason that is blank", message: "gates.wiring.ci.none needs a reason",
			gates: "  wiring:\n    hooks:\n      directory: .husky\n    ci:\n      none: \"   \"\n"},
		{name: "a ci directory that climbs out of the root", message: "gates.wiring.ci.directory names no repository directory",
			gates: "  wiring:\n    hooks:\n      directory: .husky\n    ci:\n      directory: ../workflows\n"},
		{name: "a ci section that names neither a directory nor a reason", message: "gates.wiring.ci declares neither",
			gates: "  wiring:\n    hooks:\n      directory: .husky\n    ci: {}\n"},
		{name: "local surfaces that are not a mapping", message: "local-surfaces must be a mapping",
			gates: "  wiring:\n    hooks:\n      directory: .husky\n    ci:\n      none: no workflows\n    local-surfaces: [x]\n"},
		{name: "a local surface that is not a local surface", message: "`post-merge`, which is not a local surface",
			gates: "  wiring:\n    hooks:\n      directory: .husky\n    ci:\n      none: no workflows\n    local-surfaces:\n      post-merge: [x]\n"},
		{name: "an allowed list that is not a list", message: "local-surfaces.pre-push names no entry or kind",
			gates: "  wiring:\n    hooks:\n      directory: .husky\n    ci:\n      none: no workflows\n    local-surfaces:\n      pre-push: leak-scan\n"},
		{name: "an allowed entry that is not a string", message: "local-surfaces.pre-push names no entry or kind",
			gates: "  wiring:\n    hooks:\n      directory: .husky\n    ci:\n      none: no workflows\n    local-surfaces:\n      pre-push: [1]\n"},
		{name: "an allowed entry that is blank", message: "local-surfaces.pre-push names no entry or kind",
			gates: "  wiring:\n    hooks:\n      directory: .husky\n    ci:\n      none: no workflows\n    local-surfaces:\n      pre-push: [\"\"]\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document := wiringDocument(t, tc.gates)
			_, declared, err := document.Wiring()
			if err == nil || !declared {
				t.Fatalf("declared %v err %v; want a declared section and an error", declared, err)
			}
			if !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.message)
			}
		})
	}
}

// TestWiringDeclarationReadsEachForm proves the valid forms read into the fields
// the rules use: a workflow directory, a reason with no workflows, and allowed
// lists that are absent, empty, or named.
func TestWiringDeclarationReadsEachForm(t *testing.T) {
	workflows := wiringDocument(t, "  wiring:\n    hooks:\n      directory: .husky\n    ci:\n      directory: .github/workflows\n"+
		"    local-surfaces:\n      pre-push: [leak-scan, public-scan]\n      pre-commit: []\n")
	wiring, declared, err := workflows.Wiring()
	if err != nil || !declared {
		t.Fatalf("declared %v err %v", declared, err)
	}
	if !wiring.Workflows || wiring.WorkflowDirectory != ".github/workflows" || wiring.HookDirectory != ".husky" {
		t.Fatalf("directories not read: %+v", wiring)
	}
	if !reflect.DeepEqual(wiring.LocalAllowed["pre-push"], []string{"leak-scan", "public-scan"}) {
		t.Fatalf("pre-push allowed %v", wiring.LocalAllowed["pre-push"])
	}
	if allowed, ok := wiring.LocalAllowed["pre-commit"]; !ok || len(allowed) != 0 {
		t.Fatalf("an empty list must be declared and allow nothing, got %v (declared %v)", allowed, ok)
	}

	reason := wiringDocument(t, "  wiring:\n    hooks:\n      directory: .husky\n    ci:\n      none: This repository hosts no workflows.\n")
	wiring, _, err = reason.Wiring()
	if err != nil || wiring.Workflows || wiring.NoWorkflowsReason != "This repository hosts no workflows." {
		t.Fatalf("reason form: workflows %v reason %q err %v", wiring.Workflows, wiring.NoWorkflowsReason, err)
	}
	if len(wiring.LocalAllowed) != 0 {
		t.Fatalf("omitted local-surfaces must declare no list, got %v", wiring.LocalAllowed)
	}
}

// TestWiringAbsentIsUndeclared proves an omitted section is reported as undeclared,
// and that it is not an error.
func TestWiringAbsentIsUndeclared(t *testing.T) {
	document, err := config.Parse("schema: rhino/repo-config/v2\ngates:\n  entries: []\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if wiring, declared, err := document.Wiring(); wiring != nil || declared || err != nil {
		t.Fatalf("absent wiring: %v %v %v", wiring, declared, err)
	}
}

// behaviourDocument parses a configuration whose policies.behaviour is the given text.
func behaviourDocument(t *testing.T, behaviour string) *config.Document {
	t.Helper()
	document, err := config.Parse("schema: rhino/repo-config/v2\npolicies:\n  behaviour:\n" + behaviour)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return document
}

// behaviourLevels is a levels section that declares all three levels.
const behaviourLevels = "    levels:\n      unit: [internal/**/*_test.go]\n      integration: [tests/integration/**]\n      e2e: [tests/e2e/**]\n"

// TestBehaviourDeclarationRefusals proves every malformed policies.behaviour section
// is refused with a message that names the field at fault.
func TestBehaviourDeclarationRefusals(t *testing.T) {
	cases := []struct {
		name      string
		behaviour string
		message   string
	}{
		{name: "a behaviour section that is not a mapping", behaviour: "    [x]\n", message: "policies.behaviour must be a mapping"},
		{name: "an unknown key in the behaviour section", message: "unknown key `bogus`",
			behaviour: "    corpus: [specs/behaviours]\n    bogus: 1\n" + behaviourLevels},
		{name: "no corpus", message: "policies.behaviour.corpus names no entry",
			behaviour: behaviourLevels},
		{name: "a corpus that is not a list", message: "policies.behaviour.corpus must be a list",
			behaviour: "    corpus: specs/behaviours\n" + behaviourLevels},
		{name: "a corpus root that is absolute", message: "is not a repository directory",
			behaviour: "    corpus: [/specs]\n" + behaviourLevels},
		{name: "a fixtures list that is not a list", message: "policies.behaviour.fixtures must be a list",
			behaviour: "    corpus: [specs/behaviours]\n    fixtures: test/fixtures\n" + behaviourLevels},
		{name: "a whole-file glob that is blank", message: "policies.behaviour.whole-file names an empty entry",
			behaviour: "    corpus: [specs/behaviours]\n    whole-file: [\"\"]\n" + behaviourLevels},
		{name: "no levels", message: "policies.behaviour.levels names no unit",
			behaviour: "    corpus: [specs/behaviours]\n"},
		{name: "levels that are not a mapping", message: "policies.behaviour.levels names no unit",
			behaviour: "    corpus: [specs/behaviours]\n    levels: [x]\n"},
		{name: "a level that names no such level", message: "unknown level `smoke`",
			behaviour: "    corpus: [specs/behaviours]\n    levels:\n      unit: []\n      integration: []\n      e2e: []\n      smoke: []\n"},
		{name: "a level with no glob list", message: "policies.behaviour.e2e names no entry",
			behaviour: "    corpus: [specs/behaviours]\n    levels:\n      unit: []\n      integration: []\n"},
		{name: "a level whose list is not a list", message: "policies.behaviour.unit must be a list",
			behaviour: "    corpus: [specs/behaviours]\n    levels:\n      unit: internal/**/*_test.go\n      integration: []\n      e2e: []\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document := behaviourDocument(t, tc.behaviour)
			_, declared, err := document.Behaviour()
			if err == nil || !declared {
				t.Fatalf("declared %v err %v; want a declared section and an error", declared, err)
			}
			if !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.message)
			}
		})
	}
}

// TestBehaviourDeclarationReadsEachForm proves a full section reads every key, and
// that an omitted section is undeclared rather than an error.
func TestBehaviourDeclarationReadsEachForm(t *testing.T) {
	document := behaviourDocument(t, "    corpus: [specs/behaviours]\n    fixtures: [test/fixtures]\n"+
		"    whole-file: [internal/**/*_test.go]\n"+behaviourLevels)
	behaviour, declared, err := document.Behaviour()
	if err != nil || !declared {
		t.Fatalf("declared %v err %v", declared, err)
	}
	if !reflect.DeepEqual(behaviour.Corpus, []string{"specs/behaviours"}) ||
		!reflect.DeepEqual(behaviour.Fixtures, []string{"test/fixtures"}) ||
		!reflect.DeepEqual(behaviour.WholeFile, []string{"internal/**/*_test.go"}) ||
		!reflect.DeepEqual(behaviour.Levels["e2e"], []string{"tests/e2e/**"}) {
		t.Fatalf("section not read: %+v", behaviour)
	}

	absent, err := config.Parse("schema: rhino/repo-config/v2\npolicies:\n  markdown: {}\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if behaviour, declared, err := absent.Behaviour(); behaviour != nil || declared || err != nil {
		t.Fatalf("absent behaviour: %v %v %v", behaviour, declared, err)
	}
}

// TestLineOfNamesGatesAndLevels proves LineOf finds a gate entry and a policy key
// by its path, and answers zero for a path the document does not hold.
func TestLineOfNamesGatesAndLevels(t *testing.T) {
	document, err := config.Parse("schema: rhino/repo-config/v2\n" +
		"policies:\n  behaviour:\n    levels:\n      unit: []\n" +
		"gates:\n  entries:\n    - id: format-staged\n      command:\n        executable: formatter\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if line := document.LineOf("gates", "entries", "format-staged"); line != 8 {
		t.Fatalf("gate entry line %d; want 8", line)
	}
	if line := document.LineOf("policies", "behaviour", "levels", "unit"); line != 5 {
		t.Fatalf("level line %d; want 5", line)
	}
	for _, path := range [][]string{
		{"gates", "entries", "no-such-gate"},
		{"gates", "missing"},
		{"schema", "beneath-a-scalar"},
	} {
		if line := document.LineOf(path...); line != 0 {
			t.Errorf("LineOf(%v) = %d; want 0", path, line)
		}
	}
	var empty config.Document
	if line := empty.LineOf("gates"); line != 0 {
		t.Fatalf("a document with no node: line %d; want 0", line)
	}
}

// TestParseWorkflowReadsEachShape proves the workflow reader accepts each trigger
// and step shape, and that a file that is not YAML is an error.
func TestParseWorkflowReadsEachShape(t *testing.T) {
	if _, err := config.ParseWorkflow("on: [push\n"); err == nil {
		t.Fatal("a workflow that is not YAML must be an error")
	}

	empty, err := config.ParseWorkflow("")
	if err != nil || len(empty.Triggers) != 0 || len(empty.Steps) != 0 {
		t.Fatalf("an empty file reads as an empty workflow: %+v %v", empty, err)
	}

	scalar, _ := config.ParseWorkflow("on: push\njobs:\n  j:\n    steps:\n      - run: echo one\n")
	if !scalar.Triggers["push"] || len(scalar.PushBranches) != 0 || scalar.Steps[0].Run != "echo one" {
		t.Fatalf("scalar trigger: %+v", scalar)
	}

	sequence, _ := config.ParseWorkflow("on: [push, schedule]\n")
	if !sequence.Triggers["push"] || !sequence.Triggers["schedule"] {
		t.Fatalf("sequence trigger: %+v", sequence.Triggers)
	}

	branches, _ := config.ParseWorkflow("on:\n  push:\n    branches: [main, dev]\n  push2: x\n  workflow_dispatch:\n")
	if !reflect.DeepEqual(branches.PushBranches, []string{"main", "dev"}) || !branches.Triggers["workflow_dispatch"] {
		t.Fatalf("mapping trigger: branches %v triggers %v", branches.PushBranches, branches.Triggers)
	}

	bare, _ := config.ParseWorkflow("on:\n  push:\n")
	if !bare.Triggers["push"] || len(bare.PushBranches) != 0 {
		t.Fatalf("push with no value: %+v", bare)
	}

	noEvents, _ := config.ParseWorkflow("jobs: []\n")
	if len(noEvents.Triggers) != 0 {
		t.Fatalf("a workflow with no on key starts on nothing: %+v", noEvents.Triggers)
	}

	notJobs, _ := config.ParseWorkflow("on: push\njobs: [1]\n")
	if len(notJobs.Steps) != 0 {
		t.Fatalf("jobs that are not a mapping have no steps: %+v", notJobs.Steps)
	}

	notSteps, _ := config.ParseWorkflow("on: push\njobs:\n  j:\n    steps: x\n")
	if len(notSteps.Steps) != 0 {
		t.Fatalf("steps that are not a sequence have no steps: %+v", notSteps.Steps)
	}

	mapped, _ := config.ParseWorkflow("on: push\njobs:\n  j:\n    steps:\n      - run: {a: 1}\n")
	if len(mapped.Steps) != 1 || mapped.Steps[0].Run != "" {
		t.Fatalf("a run that is a mapping reads as no text: %+v", mapped.Steps)
	}

	literal, _ := config.ParseWorkflow("on: push\njobs:\n  j:\n    steps:\n" +
		"      - run: |\n          echo hi\n        continue-on-error: true\n")
	step := literal.Steps[0]
	if step.RunLine != 6 || !step.ContinueOnError || !strings.Contains(step.Run, "echo hi") {
		t.Fatalf("literal step: %+v", step)
	}
}
