package application

import (
	"strings"
	"testing"
)

// harnessStubTree answers only the reads the harness helpers under test make.
// Any other method is promoted from the nil embedded Tree and panics, so a
// test that reaches an unplanned read fails loudly instead of passing.
type harnessStubTree struct {
	Tree
	files  map[string]string
	errors map[string]*ReadError
	listed []string
}

func (s harnessStubTree) Root() string    { return "/root" }
func (s harnessStubTree) Files() []string { return s.listed }

func (s harnessStubTree) Read(path string) (string, *ReadError) {
	if readErr, ok := s.errors[path]; ok {
		return "", readErr
	}
	if text, ok := s.files[path]; ok {
		return text, nil
	}
	return "", &ReadError{Kind: "missing", Reason: "missing"}
}

func TestHarnessAdapterRootNeedsAFamilyDirectory(t *testing.T) {
	if _, ok := harnessAdapterRoot("{name}.md"); ok {
		t.Fatal("a pattern with no directory named a family root")
	}
}

func TestValidHarnessDispatchesRefusesABlankSegment(t *testing.T) {
	if validHarnessDispatches(&harnessDispatches{field: ".x", format: harnessAllowMap}) {
		t.Fatal("a dispatch field with a blank segment was accepted")
	}
}

func TestIsExactHarnessAgentPatternRefusesABracedDirectory(t *testing.T) {
	if isExactHarnessAgentPattern("adapters/{x}/{name}/x.md") {
		t.Fatal("a pattern with a second placeholder was accepted as an exact agent pattern")
	}
}

func TestHarnessRecordedModelsIgnoresAnAdapterOutsideTheRepository(t *testing.T) {
	if recorded := harnessRecordedModels(nil, &harnessAdapter{path: "../{name}.md"}); len(recorded) != 0 {
		t.Fatalf("an adapter outside the repository recorded models %v", recorded)
	}
}

// TestResolveHarnessRecordedKeepsTheModelAProvenanceRecorded proves a tier whose
// provenance recorded a model renders and records that model, not the fallback.
func TestResolveHarnessRecordedKeepsTheModelAProvenanceRecorded(t *testing.T) {
	tree := harnessStubTree{files: map[string]string{
		"adapters/alpha/agents/provenance.json": `{"resolvedModels": {"fast": "m-1"}}`,
	}}
	profile := &harnessProfile{
		id:           "alpha",
		agentAdapter: &harnessAdapter{path: "adapters/alpha/agents/{name}.md"},
		tiers:        map[string]harnessTier{"fast": {modelResolve: &harnessModelResolve{fallback: "fallback-model"}}},
	}
	models := resolveHarnessRecorded(profile, tree)
	if models.recorded["fast"] != "m-1" || models.rendered["fast"] != "m-1" {
		t.Fatalf("the recorded model was not kept: %+v", models)
	}
}

// TestHarnessProvenanceJSONListsEachRecordedModel proves every recorded tier is
// written, each followed by a comma except the last.
func TestHarnessProvenanceJSONListsEachRecordedModel(t *testing.T) {
	got := harnessProvenanceJSON("alpha", nil, map[string]string{"fast": "m-1", "slow": "m-2"})
	if !strings.Contains(got, "    \"fast\": \"m-1\",\n") || !strings.Contains(got, "    \"slow\": \"m-2\"\n") {
		t.Fatalf("recorded models were not listed with separators:\n%s", got)
	}
}

// TestRenderHarnessProfileRefusesASkillFamilyOutsideTheRepository proves a skill
// adapter whose path climbs out of the repository refuses the profile.
func TestRenderHarnessProfileRefusesASkillFamilyOutsideTheRepository(t *testing.T) {
	profile := &harnessProfile{id: "alpha", skillAdapter: &harnessAdapter{path: "adapters/../{name}"}}
	models := harnessResolvedModels{rendered: map[string]string{}, recorded: map[string]string{}}
	reason, failed := renderHarnessProfile(profile, nil, models, map[string]string{})
	if !failed || reason != "profile `alpha` has an invalid adapter path" {
		t.Fatalf("a skill family outside the repository was not refused: %q, %v", reason, failed)
	}
}

// TestRenderHarnessAdapterRefusesARouteWithoutAPlaceholder proves an adapter
// whose route names no `{path}` refuses the source it would route.
func TestRenderHarnessAdapterRefusesARouteWithoutAPlaceholder(t *testing.T) {
	adapter := &harnessAdapter{path: "adapters/alpha/agents/{name}.md", format: "front-matter", routeField: "body", route: "no placeholder"}
	source := &harnessSource{id: "agent/x", path: ".agents/agents/x.md", kind: "agent", metadata: &harnessMetadata{name: "x"}}
	_, reason, failed := renderHarnessAdapter(&harnessProfile{id: "alpha"}, adapter, source, map[string]string{})
	if !failed || reason != "adapter route does not declare exactly one `{path}` placeholder" {
		t.Fatalf("a route with no placeholder was not refused: %q, %v", reason, failed)
	}
}

// TestRenderHarnessAdapterRefusesARouteThatRepeatsAFieldOnTOML proves a TOML
// adapter whose route field is already written by its identity refuses, rather
// than writing the field twice.
func TestRenderHarnessAdapterRefusesARouteThatRepeatsAFieldOnTOML(t *testing.T) {
	adapter := &harnessAdapter{
		path:       "adapters/alpha/agents/{name}.md",
		format:     "toml",
		routeField: "description",
		route:      "Read {path}.",
		identity:   map[string]string{"description": "description"},
	}
	source := &harnessSource{id: "agent/x", path: ".agents/agents/x.md", kind: "agent",
		metadata: &harnessMetadata{name: "x", description: "d"}}
	if _, reason, failed := renderHarnessAdapter(&harnessProfile{id: "alpha"}, adapter, source, map[string]string{}); !failed || reason != "adapter output repeats field `description`" {
		t.Fatalf("a repeated TOML field was not refused: %q, %v", reason, failed)
	}
}

// TestHarnessParseFrontMatterRefusesMalformedLines proves a front-matter line
// with no colon, and one with an empty key, each refuse the source.
func TestHarnessParseFrontMatterRefusesMalformedLines(t *testing.T) {
	cases := []struct {
		contents string
		want     string
	}{
		{"---\njust words\n---\n", "canonical source `p.md` has malformed front matter"},
		{"---\n: value\n---\n", "canonical source `p.md` repeats or omits a front matter key"},
	}
	for _, tc := range cases {
		if _, reason, failed := harnessParseFrontMatter("p.md", tc.contents); !failed || reason != tc.want {
			t.Fatalf("front matter %q: got %q, %v, want %q", tc.contents, reason, failed, tc.want)
		}
	}
}

// TestHarnessQuotedFrontMatterReadsAsItsText proves a value wrapped in matching
// double or single quotes reads as the text inside them.
func TestHarnessQuotedFrontMatterReadsAsItsText(t *testing.T) {
	if got := harnessUnquote(`"x"`); got != "x" {
		t.Fatalf("a double-quoted value read as %q", got)
	}
	if got := harnessUnquote(`'x'`); got != "x" {
		t.Fatalf("a single-quoted value read as %q", got)
	}
	if harnessStartsWhitespace("") {
		t.Fatal("an empty line started with whitespace")
	}
}

// TestHarnessParseMetadataRefusesAMissingOrShapedField proves a canonical source
// with no name, a tier written as a list, or dispatches written as a scalar each
// refuses, naming the field.
func TestHarnessParseMetadataRefusesAMissingOrShapedField(t *testing.T) {
	tier := "tier"
	dispatches := "dispatches"
	base := &harnessCanonicalAgent{name: "name", description: "description"}
	withTier := &harnessCanonicalAgent{name: "name", description: "description", tier: &tier}
	withDispatches := &harnessCanonicalAgent{name: "name", description: "description", dispatches: &dispatches}
	cases := []struct {
		name     string
		contents string
		shape    *harnessCanonicalAgent
		want     string
	}{
		{"a source with no name is refused", "---\ndescription: d\n---\n", base,
			"canonical source `p.md` has no `name` metadata field"},
		{"a tier written as a list is refused", "---\nname: n\ndescription: d\ntier:\n  - x\n---\n", withTier,
			"canonical source `p.md` has a non-scalar `tier` metadata field"},
		{"dispatches written as a scalar are refused", "---\nname: n\ndescription: d\ndispatches: x\n---\n", withDispatches,
			"canonical source `p.md` has a non-list `dispatches` metadata field"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, reason, failed := harnessParseMetadata("p.md", tc.contents, tc.shape); !failed || reason != tc.want {
				t.Fatalf("got %q, %v, want %q", reason, failed, tc.want)
			}
		})
	}
}

// TestHarnessStaleMarkersSkipsAMarkerThatVanished proves a marker surface file
// the walk listed and the read found gone is neither a finding nor a refusal.
func TestHarnessStaleMarkersSkipsAMarkerThatVanished(t *testing.T) {
	tree := harnessStubTree{
		listed: []string{"notes/a.md"},
		errors: map[string]*ReadError{"notes/a.md": {Kind: "missing", Reason: "gone"}},
	}
	decoded := &harnessConfig{markerSurfaces: []string{"notes/*.md"}}
	markers, outcome, failed := harnessStaleMarkers(decoded, tree, &harnessPlanResult{desired: map[string]string{}}, nil, Text)
	if failed || len(markers) != 0 || outcome.ExitCode != 0 {
		t.Fatalf("a vanished marker was reported: %v, %+v, %v", markers, outcome, failed)
	}
}

// TestHarnessStaleMarkersLeavesAGeneratedPathAlone proves a marker-bearing file at
// an exact adapter path that generation writes is not a stale marker, while a
// marker-bearing file under a governed surface that generation does not write is
// still reported.
func TestHarnessStaleMarkersLeavesAGeneratedPathAlone(t *testing.T) {
	marked := "<!-- " + harnessGeneratedMarker + " -->\n"
	instruction := "adapters/alpha/CLAUDE.md"
	exactAgent := "adapters/alpha/agents/x.md"
	stray := "adapters/alpha/notes/stray.md"
	tree := harnessStubTree{
		listed: []string{instruction, exactAgent, stray},
		files:  map[string]string{instruction: marked, exactAgent: marked, stray: marked},
	}
	decoded := &harnessConfig{markerSurfaces: []string{"adapters/alpha/*.md", "adapters/alpha/agents/*.md", "adapters/alpha/notes/*.md"}}
	plan := &harnessPlanResult{desired: map[string]string{instruction: "x", exactAgent: "y"}}
	markers, outcome, failed := harnessStaleMarkers(decoded, tree, plan, nil, Text)
	if failed || outcome.ExitCode != 0 {
		t.Fatalf("stale-marker check refused: %+v, %v", outcome, failed)
	}
	if len(markers) != 1 || markers[0] != stray+": stale-marker" {
		t.Fatalf("stale markers were %v, want only %s", markers, stray)
	}
}

// TestHarnessDifferencesNamesAnAdapterThatIsNotText proves an adapter path that
// holds non-text, or cannot be read, is named with that reason rather than
// compared as if it matched.
func TestHarnessDifferencesNamesAnAdapterThatIsNotText(t *testing.T) {
	tree := harnessStubTree{errors: map[string]*ReadError{
		"adapters/a/x.md": {Kind: "not-text", Reason: "binary"},
		"adapters/a/y.md": {Kind: "unreadable", Reason: "permission denied"},
	}}
	plan := &harnessPlanResult{desired: map[string]string{"adapters/a/x.md": "x", "adapters/a/y.md": "y"}}
	differences := harnessDifferences(tree, plan)
	want := map[string]bool{
		"adapters/a/x.md: non-text-adapter":                      true,
		"adapters/a/y.md: unreadable-adapter: permission denied": true,
	}
	for _, difference := range differences {
		if !want[difference] {
			t.Fatalf("unexpected difference %q in %v", difference, differences)
		}
		delete(want, difference)
	}
	if len(want) != 0 {
		t.Fatalf("expected differences missing: %v (got %v)", want, differences)
	}
}

// TestHarnessDiscoverRefusesAFamilyWithoutItsCanonicalDeclaration proves an agent
// or skill family whose canonical declaration is absent refuses discovery, so no
// family is generated from an undeclared source.
func TestHarnessDiscoverRefusesAFamilyWithoutItsCanonicalDeclaration(t *testing.T) {
	tree := harnessStubTree{files: map[string]string{"AGENTS.md": "# Agents\n"}}
	agents := &harnessConfig{profiles: []harnessProfile{{id: "alpha", agentAdapter: &harnessAdapter{}}}}
	if _, reason, failed := harnessDiscover(tree, agents, ".agents/agents/"); !failed || reason != "harness: agent adapters require a canonical agent field declaration" {
		t.Fatalf("an agent family without its declaration was discovered: %q, %v", reason, failed)
	}
	skills := &harnessConfig{profiles: []harnessProfile{{id: "alpha", skillAdapter: &harnessAdapter{}}}}
	if _, reason, failed := harnessDiscover(tree, skills, ".agents/agents/"); !failed || reason != "harness: skill adapters require a canonical skill field declaration" {
		t.Fatalf("a skill family without its declaration was discovered: %q, %v", reason, failed)
	}
}
