package application_test

import (
	"strings"
	"testing"
)

// harnessScanExclusionConfig is a valid harness declaration whose scan section
// is malformed. Harness runs read the shared exclusions, so the malformed section
// must refuse the run rather than be read as an empty list.
const harnessScanExclusionConfig = "scan:\n  exclude-directories: x\n" + harnessInstructionConfig

// TestHarnessValidateRefusesAMalformedScanExclusion proves a harness validate run
// refuses a malformed shared scan exclusion section.
func TestHarnessValidateRefusesAMalformedScanExclusion(t *testing.T) {
	tree := harnessTree(harnessScanExclusionConfig)
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files["CLAUDE.md"] = "Read AGENTS.md and follow it.\n"
	outcome := runHarnessValidate(tree, "harness", "adapters", "validate")
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, "scan.exclude-directories names no directory") {
		t.Fatalf("malformed scan exclusions not refused by validate: %+v", outcome)
	}
}

// TestHarnessGenerateRefusesAMalformedScanExclusion proves a harness generate run
// refuses a malformed shared scan exclusion section before it writes anything.
func TestHarnessGenerateRefusesAMalformedScanExclusion(t *testing.T) {
	tree := harnessTree(harnessScanExclusionConfig)
	tree.files["AGENTS.md"] = harnessRootInstruction
	outcome := runHarnessGenerate(tree, nil, nil, "harness", "adapters", "generate")
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, "scan.exclude-directories names no directory") {
		t.Fatalf("malformed scan exclusions not refused by generate: %+v", outcome)
	}
	if _, written := tree.files["CLAUDE.md"]; written {
		t.Fatal("generate wrote an adapter before refusing the scan exclusions")
	}
}

// TestHarnessGenerateRefusesAMarkerSurfaceThatCannotBeRead proves generation reads
// every declared marker surface before it acts, so an unreadable marker file
// refuses the run instead of being generated over.
func TestHarnessGenerateRefusesAMarkerSurfaceThatCannotBeRead(t *testing.T) {
	tree := harnessTree(harnessInstructionConfig + "  marker-surfaces: ['docs/*.md']\n")
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files["docs/a.md"] = "rhino generated\n"
	tree.unreadable["docs/a.md"] = true
	outcome := runHarnessGenerate(tree, nil, nil, "harness", "adapters", "generate")
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, "docs/a.md:") {
		t.Fatalf("an unreadable marker surface did not refuse generation: %+v", outcome)
	}
	if _, written := tree.files["CLAUDE.md"]; written {
		t.Fatal("generate wrote an adapter before refusing the marker surface")
	}
}

// TestHarnessGenerateRefusesWithExitTwoWhenAGeneratedMarkerIsStale proves generation
// still writes the adapters it owns, then refuses with exit 2 and the harness refusal
// code when a declared marker surface claims RHINO output that generation does not
// write. The same finding stays exit 1 under validation, which is read-only.
func TestHarnessGenerateRefusesWithExitTwoWhenAGeneratedMarkerIsStale(t *testing.T) {
	tree := harnessTree(harnessInstructionConfig + "  marker-surfaces: ['.codex/config.toml']\n")
	tree.files["AGENTS.md"] = harnessRootInstruction
	tree.files[".codex/config.toml"] = "[agents]\n# Rhino generated: codex agents\n"
	store := &memoryHarnessStore{tree: tree}
	outcome := runHarnessGenerate(tree, store, &stubModelResolver{}, "harness", "adapters", "generate", "--output", "json")
	if outcome.ExitCode != 2 {
		t.Fatalf("generate with a stale marker exited %d, want 2: %+v", outcome.ExitCode, outcome)
	}
	if outcome.Stdout != "" {
		t.Fatalf("a refused generate wrote to stdout: %q", outcome.Stdout)
	}
	if !strings.Contains(outcome.Stderr, `"code":"rhino.harness.refused"`) ||
		!strings.Contains(outcome.Stderr, ".codex/config.toml: stale-marker") {
		t.Fatalf("the refusal does not carry the harness code and the stale marker: %q", outcome.Stderr)
	}
	if _, written := tree.files["CLAUDE.md"]; !written {
		t.Fatal("generate did not write the adapters it owns before refusing the stale marker")
	}
	validated := runHarnessValidate(tree, "harness", "adapters", "validate")
	if validated.ExitCode != 1 || !strings.Contains(validated.Stderr, ".codex/config.toml: stale-marker") {
		t.Fatalf("validate no longer reports the stale marker as exit 1: %+v", validated)
	}
}
