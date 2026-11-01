package application

import "testing"

// These helpers and render guards are unreachable from a validated profile or a
// well-formed canonical tree, so each is exercised on its own with the input
// that makes it refuse, and each assertion names the reason it returns.

func TestCutHarnessAffixNeedsBothAffixes(t *testing.T) {
	if version, ok := cutHarnessAffix("model-a-1.0", "model-a-", ""); !ok || version != "1.0" {
		t.Fatalf("prefix-only affix = %q, %v", version, ok)
	}
	if _, ok := cutHarnessAffix("other-1.0", "model-a-", ""); ok {
		t.Fatal("a token without the prefix matched")
	}
	if _, ok := cutHarnessAffix("model-a-1.0", "model-a-", ".x"); ok {
		t.Fatal("a token without the suffix matched")
	}
}

func TestHarnessVersionOrderingComparesDigitsThenLength(t *testing.T) {
	if !harnessVersionGreater(harnessVersionKey("1.3"), harnessVersionKey("1.2")) {
		t.Fatal("1.3 did not order above 1.2")
	}
	if harnessVersionGreater(harnessVersionKey("1.2"), harnessVersionKey("1.3")) {
		t.Fatal("1.2 ordered above 1.3")
	}
	if harnessVersionEqual(harnessVersionKey("1.2"), harnessVersionKey("1.2.0")) {
		t.Fatal("versions of different length compared equal")
	}
}

func TestHarnessExactPathsRefusesAnInvalidPath(t *testing.T) {
	if _, err := harnessExactPaths([]string{"adapters"}, []string{"../x"}); err == nil {
		t.Fatal("a climbing exact path was accepted")
	}
}

func TestHarnessAdapterRootsRefusesAnInvalidRoot(t *testing.T) {
	if _, err := harnessAdapterRoots([]string{"adapters/../alpha"}); err == nil {
		t.Fatal("a climbing adapter root was accepted")
	}
}

func TestHarnessSourceIDWithoutAnExtensionIsThePath(t *testing.T) {
	if got := harnessSourceID("README"); got != "README" {
		t.Fatalf("harnessSourceID(README) = %q", got)
	}
}

func TestNormalHarnessPathRefusesEmptyClimbingAndBackslashPaths(t *testing.T) {
	for _, path := range []string{"/", "a/../b", "a\\b", "a//b"} {
		if _, ok := normalHarnessPath(path); ok {
			t.Fatalf("normalHarnessPath(%q) accepted an unusable path", path)
		}
	}
}

func TestHarnessAdapterRootRefusesAPatternThatClimbs(t *testing.T) {
	if _, ok := harnessAdapterRoot("adapters/../{name}.md"); ok {
		t.Fatal("a climbing adapter pattern named a root")
	}
}

// TestRenderHarnessFamilyRefusesWhatTheFamilyCannotWrite proves a family whose
// pattern names no root, whose source carries no metadata, or whose catalog
// would overwrite an output already planned refuses with its own reason.
func TestRenderHarnessFamilyRefusesWhatTheFamilyCannotWrite(t *testing.T) {
	profile := &harnessProfile{id: "alpha"}
	models := harnessResolvedModels{rendered: map[string]string{}, recorded: map[string]string{}}
	goodAdapter := &harnessAdapter{path: "adapters/alpha/agents/{name}.md", format: "front-matter", routeField: "body", route: "Read {path}."}

	badRoot := &harnessAdapter{path: "adapters/../{name}.md", format: "front-matter", routeField: "body", route: "Read {path}."}
	if reason, failed := renderHarnessFamily(profile, badRoot, "agent", nil, models, map[string]string{}); !failed || reason != "profile `alpha` has an invalid adapter path" {
		t.Fatalf("climbing family path not refused: %q, %v", reason, failed)
	}

	noMetadata := []harnessSource{{id: "agent/x", path: ".agents/agents/x.md", kind: "agent"}}
	if reason, failed := renderHarnessFamily(profile, goodAdapter, "agent", noMetadata, models, map[string]string{}); !failed || reason != "profile `alpha` renders a source with no canonical metadata" {
		t.Fatalf("source without metadata not refused: %q, %v", reason, failed)
	}

	catalogTaken := map[string]string{"adapters/alpha/agents/catalog.json": "held\n"}
	if reason, failed := renderHarnessFamily(profile, goodAdapter, "agent", nil, models, catalogTaken); !failed || reason != "generated adapter path `adapters/alpha/agents/catalog.json` is duplicated" {
		t.Fatalf("catalog over a planned output not refused: %q, %v", reason, failed)
	}

	provenanceTaken := map[string]string{"adapters/alpha/agents/provenance.json": "held\n"}
	if reason, failed := renderHarnessFamily(profile, goodAdapter, "agent", nil, models, provenanceTaken); !failed || reason != "generated adapter path `adapters/alpha/agents/provenance.json` is duplicated" {
		t.Fatalf("provenance over a planned output not refused: %q, %v", reason, failed)
	}
}

// TestRenderHarnessProfileRefusesAnInstructionItCannotWrite proves an instruction
// adapter refuses when discovery lost the root instruction, when its route names
// the path twice, or when its output is already planned.
func TestRenderHarnessProfileRefusesAnInstructionItCannotWrite(t *testing.T) {
	models := harnessResolvedModels{rendered: map[string]string{}, recorded: map[string]string{}}
	instruction := harnessSource{id: "instruction", path: "AGENTS.md", kind: "instruction"}

	lost := &harnessProfile{id: "alpha", instructionAdapter: &harnessInstructionAdapter{path: "CLAUDE.md", route: "Read {path}."}}
	if reason, failed := renderHarnessProfile(lost, nil, models, map[string]string{}); !failed || reason != "harness: canonical discovery lost the root instruction" {
		t.Fatalf("lost instruction not refused: %q, %v", reason, failed)
	}

	twice := &harnessProfile{id: "alpha", instructionAdapter: &harnessInstructionAdapter{path: "CLAUDE.md", route: "Read {path} and {path}."}}
	if reason, failed := renderHarnessProfile(twice, []harnessSource{instruction}, models, map[string]string{}); !failed || reason != "adapter route does not declare exactly one `{path}` placeholder" {
		t.Fatalf("two-placeholder instruction route not refused: %q, %v", reason, failed)
	}

	planned := &harnessProfile{id: "alpha", instructionAdapter: &harnessInstructionAdapter{path: "CLAUDE.md", route: "Read {path}."}}
	if reason, failed := renderHarnessProfile(planned, []harnessSource{instruction}, models, map[string]string{"CLAUDE.md": "held\n"}); !failed || reason != "generated adapter path `CLAUDE.md` is duplicated" {
		t.Fatalf("instruction over a planned output not refused: %q, %v", reason, failed)
	}
}
