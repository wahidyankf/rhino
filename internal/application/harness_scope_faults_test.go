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
