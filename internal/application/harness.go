// Harness adapters: canonical discovery plus deterministic adapter projection.
//
// Canonical bodies are never copied into an adapter. Every generated body is
// a small, provenance-bearing route to one file in the root instruction or
// `.agents` tree. Profiles are configuration data, so this file has no
// consumer-specific branch.
package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// AdapterFile is one generated adapter file, held in memory until every
// profile has been discovered and validated. The bytes never come from an
// adapter directory: generation is a projection of canonical sources only.
type AdapterFile struct {
	Path     string
	Contents string
}

// AdapterTransaction is the complete replacement of declared adapter
// families plus individually managed adapter files. Roots may replace whole
// generated directories; exact paths permit a root-level route without
// claiming its neighbours.
type AdapterTransaction struct {
	Roots      []string
	ExactPaths []string
	Files      []AdapterFile
}

// AdapterStore is the narrow write port used only by canonical adapter
// generation. A caller supplies all desired bytes before asking the store to
// replace any family, and validation never receives it, so validation cannot
// repair drift.
type AdapterStore interface {
	Replace(root string, transaction AdapterTransaction) error
}

// ModelLaunch is one declared model-listing command. Like a toolchain
// launch it carries no hook surface or inherited environment, and the child
// is given no shell.
type ModelLaunch struct {
	// Arguments is the declared vector. The first value is the executable;
	// there is no shell, so a value holding a space stays one argument.
	Arguments []string
	// Directory is the repository root, the child's working directory.
	Directory string
	// TimeoutSeconds bounds the child and is always stated.
	TimeoutSeconds uint64
}

// ModelResolver is the only capability that starts a declared model-listing
// command. Adapter generation alone is handed a real one; adapter validation
// never receives it, so checking bindings against a recorded model can never
// start a process, whatever the configuration declares.
type ModelResolver interface {
	// Resolve reports the child's standard output when it started and
	// exited zero.
	Resolve(launch ModelLaunch) (string, error)
}

// closedAdapterStore fails every write closed, the way the oracle's
// NoAdapterStore does: validation never holds it, and generation without a
// wired entry point refuses before touching the repository.
type closedAdapterStore struct{}

func (closedAdapterStore) Replace(string, AdapterTransaction) error {
	return fmt.Errorf("this entry point has no canonical-adapter write boundary")
}

// closedModelResolver fails every resolution closed, the way the oracle's
// NoModelResolver does: generation then renders the fallback and says so,
// while validation never receives it at all.
type closedModelResolver struct{}

func (closedModelResolver) Resolve(ModelLaunch) (string, error) {
	return "", fmt.Errorf("this entry point has no model-resolution process boundary")
}

// Harness vocabulary. The canonical bodies themselves are deliberately not
// duplicated here: they remain the repository's root instruction and
// `.agents` trees. This model states only the portable meaning each profile
// must be able to carry.

type harnessRequirements struct {
	capabilities []string
	grants       []string
	denials      []string
	constraints  []string
	routes       []string
	identities   []string
}

type harnessCanonicalAgent struct {
	name           string
	description    string
	tier           *string
	grants         string
	hasGrants      bool
	denials        string
	hasDenials     bool
	constraints    string
	hasConstraints bool
	dispatches     *string
}

type harnessCanonicalDocument struct {
	name        string
	description string
}

type harnessCanonical struct {
	agents *harnessCanonicalAgent
	skills *harnessCanonicalDocument
}

type harnessInstructionAdapter struct {
	path  string
	route string
}

type harnessTierFields struct {
	model  string
	effort string
}

type harnessEmptyTier struct {
	model  *string
	effort *string
}

type harnessDispatches struct {
	field  string
	format string
}

type harnessModelResolve struct {
	command  []string
	pattern  string
	fallback string
}

type harnessTier struct {
	model        *string
	modelResolve *harnessModelResolve
	effort       *string
}

type harnessTranslation struct {
	when          string
	capability    *string
	field         string
	members       []string
	absentMembers []string
	entries       map[string]string
}

type harnessAdapter struct {
	path         string
	format       string
	routeField   string
	route        string
	identity     map[string]string
	fixed        map[string]string
	tierFields   *harnessTierFields
	emptyTier    *harnessEmptyTier
	lists        map[string]string
	dispatches   *harnessDispatches
	agents       *[]string
	absent       []string
	translations []harnessTranslation
}

type harnessProfile struct {
	id                 string
	supports           harnessRequirements
	instructionAdapter *harnessInstructionAdapter
	agentAdapter       *harnessAdapter
	skillAdapter       *harnessAdapter
	tiers              map[string]harnessTier
}

type harnessConfig struct {
	canonical      *harnessCanonical
	agentsRoot     *string
	requirements   harnessRequirements
	profiles       []harnessProfile
	markerSurfaces []string
}

// decodeHarness projects the declared harness group. An absent or null group
// is undeclared; an explicitly empty mapping is a declared empty policy,
// which the planner refuses with its own reason. Unknown keys refuse: a
// policy the tool silently ignored would be a check that never ran.
func decodeHarness(document *config.Document) (*harnessConfig, *Refusal) {
	mapping, ok := document.Harness()
	if !ok {
		return nil, &Refusal{Code: domain.ConfigUndeclared,
			Message: "[harness-adapters] harness: no adapter profiles are declared"}
	}
	decoded := &harnessConfig{}
	for key := range mapping {
		switch key {
		case "canonical", "agents-root", "requirements", "profiles", "marker-surfaces":
		default:
			return nil, &Refusal{Code: domain.ConfigUnusable,
				Message: fmt.Sprintf("harness names unknown key `%s`", key)}
		}
	}
	var err error
	if decoded.canonical, err = decodeHarnessCanonical(mapping); err != nil {
		return nil, &Refusal{Code: domain.ConfigUnusable, Message: err.Error()}
	}
	if decoded.agentsRoot, err = decodeHarnessAgentsRoot(mapping); err != nil {
		return nil, &Refusal{Code: domain.ConfigUnusable, Message: err.Error()}
	}
	if decoded.requirements, err = decodeHarnessRequirements(mapping, "harness.requirements"); err != nil {
		return nil, &Refusal{Code: domain.ConfigUnusable, Message: err.Error()}
	}
	if decoded.profiles, err = decodeHarnessProfiles(mapping); err != nil {
		return nil, &Refusal{Code: domain.ConfigUnusable, Message: err.Error()}
	}
	if decoded.markerSurfaces, err = decodeHarnessStringList(mapping, "marker-surfaces", "marker surfaces"); err != nil {
		return nil, &Refusal{Code: domain.ConfigUnusable, Message: err.Error()}
	}
	if err := validateHarnessTiers(decoded); err != nil {
		return nil, &Refusal{Code: domain.ConfigUnusable, Message: err.Error()}
	}
	return decoded, nil
}

// harnessSemanticError reports a refusal against the repository
// configuration file. The oracle pins every harness semantic refusal to
// line 1, so this port does the same.
func harnessSemanticError(format string, args ...any) error {
	return fmt.Errorf(config.Path+": line 1: "+format, args...)
}

// validateHarnessTiers pins a tier model and effort together or pins
// neither, refused here, before any file is read, by every harness command.
func validateHarnessTiers(decoded *harnessConfig) error {
	for _, profile := range decoded.profiles {
		names := make([]string, 0, len(profile.tiers))
		for name := range profile.tiers {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			tier := profile.tiers[name]
			key := fmt.Sprintf("harness.profiles.%s.tiers.%s", profile.id, name)
			if tier.model != nil && tier.modelResolve != nil {
				return harnessSemanticError("%s: declares both a model and a model-resolve; a tier names one model", key)
			}
			if (tier.model != nil || tier.modelResolve != nil) != (tier.effort != nil) {
				return harnessSemanticError("%s: declares a model or an effort without the other; a tier pins both or neither", key)
			}
			if tier.modelResolve != nil {
				if err := validateHarnessModelResolve(key, tier.modelResolve); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// validateHarnessModelResolve checks a resolving tier against the closed
// model-name vocabulary before any command reads it.
func validateHarnessModelResolve(key string, resolve *harnessModelResolve) error {
	if len(resolve.command) == 0 || strings.TrimSpace(resolve.command[0]) == "" {
		return harnessSemanticError("%s: declares a model-resolve command with no executable", key)
	}
	if strings.Count(resolve.pattern, "*") != 1 || !isModelPattern(resolve.pattern) {
		return harnessSemanticError("%s: declares a model-resolve pattern that is not one `*` among model-name characters", key)
	}
	if resolve.fallback == "" || !isModelName(resolve.fallback) {
		return harnessSemanticError("%s: declares a model-resolve fallback that is not a model name", key)
	}
	return nil
}

// isModelCharacter reports whether a character may appear in a model name,
// the way a resolving tier reads command output.
func isModelCharacter(character rune) bool {
	return character >= 'a' && character <= 'z' ||
		character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9' ||
		character == '.' || character == '_' || character == '-'
}

func isModelName(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if !isModelCharacter(character) {
			return false
		}
	}
	return true
}

func isModelPattern(value string) bool {
	for _, character := range value {
		if character != '*' && !isModelCharacter(character) {
			return false
		}
	}
	return true
}

func decodeHarnessCanonical(mapping map[string]any) (*harnessCanonical, error) {
	raw, present := mapping["canonical"]
	if !present || raw == nil {
		return nil, nil
	}
	entry, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("harness canonical names no canonical declaration")
	}
	decoded := &harnessCanonical{}
	for key := range entry {
		if key != "agents" && key != "skills" {
			return nil, fmt.Errorf("harness canonical names unknown key `%s`", key)
		}
	}
	if rawAgents, present := entry["agents"]; present && rawAgents != nil {
		agents, ok := rawAgents.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("harness canonical agents names no agent declaration")
		}
		decoded.agents = &harnessCanonicalAgent{}
		for key := range agents {
			switch key {
			case "name", "description", "tier", "grants", "denials", "constraints", "dispatches":
			default:
				return nil, fmt.Errorf("harness canonical agents names unknown key `%s`", key)
			}
		}
		var err error
		if decoded.agents.name, err = harnessRequiredString(agents, "name", "harness canonical agents names no name"); err != nil {
			return nil, err
		}
		if decoded.agents.description, err = harnessRequiredString(agents, "description", "harness canonical agents names no description"); err != nil {
			return nil, err
		}
		if decoded.agents.tier, err = harnessOptionalString(agents, "tier", "harness canonical agents tier names no tier"); err != nil {
			return nil, err
		}
		if decoded.agents.grants, err = harnessRequiredString(agents, "grants", "harness canonical agents names no grants"); err != nil {
			return nil, err
		}
		decoded.agents.hasGrants = true
		if decoded.agents.denials, err = harnessRequiredString(agents, "denials", "harness canonical agents names no denials"); err != nil {
			return nil, err
		}
		decoded.agents.hasDenials = true
		if decoded.agents.constraints, err = harnessRequiredString(agents, "constraints", "harness canonical agents names no constraints"); err != nil {
			return nil, err
		}
		decoded.agents.hasConstraints = true
		if decoded.agents.dispatches, err = harnessOptionalString(agents, "dispatches", "harness canonical agents dispatches names no dispatches"); err != nil {
			return nil, err
		}
	}
	if rawSkills, present := entry["skills"]; present && rawSkills != nil {
		skills, ok := rawSkills.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("harness canonical skills names no skill declaration")
		}
		decoded.skills = &harnessCanonicalDocument{}
		for key := range skills {
			if key != "name" && key != "description" {
				return nil, fmt.Errorf("harness canonical skills names unknown key `%s`", key)
			}
		}
		var err error
		if decoded.skills.name, err = harnessRequiredString(skills, "name", "harness canonical skills names no name"); err != nil {
			return nil, err
		}
		if decoded.skills.description, err = harnessRequiredString(skills, "description", "harness canonical skills names no description"); err != nil {
			return nil, err
		}
	}
	return decoded, nil
}

func decodeHarnessAgentsRoot(mapping map[string]any) (*string, error) {
	raw, present := mapping["agents-root"]
	if !present || raw == nil {
		return nil, nil
	}
	root, ok := raw.(string)
	if !ok {
		return nil, fmt.Errorf("harness agents-root names no agents root")
	}
	return &root, nil
}

// decodeHarnessSupports reads one profile's declared support axes. An
// absent `supports` block supports nothing, never a refusal.
func decodeHarnessSupports(entry map[string]any) (harnessRequirements, error) {
	const key = "harness.profiles supports"
	var decoded harnessRequirements
	raw, present := entry["supports"]
	if !present || raw == nil {
		return decoded, nil
	}
	return decodeHarnessAxes(raw, key)
}

func decodeHarnessRequirements(mapping map[string]any, key string) (harnessRequirements, error) {
	var decoded harnessRequirements
	raw, present := mapping["requirements"]
	if !present || raw == nil {
		return decoded, nil
	}
	return decodeHarnessAxes(raw, key)
}

func decodeHarnessAxes(raw any, key string) (harnessRequirements, error) {
	var decoded harnessRequirements
	entry, ok := raw.(map[string]any)
	if !ok {
		return decoded, fmt.Errorf("%s names no requirement axes", key)
	}
	for _, axis := range []struct {
		name   string
		target *[]string
	}{
		{"capabilities", &decoded.capabilities},
		{"grants", &decoded.grants},
		{"denials", &decoded.denials},
		{"constraints", &decoded.constraints},
		{"routes", &decoded.routes},
		{"identities", &decoded.identities},
	} {
		values, err := decodeHarnessAxis(entry, axis.name)
		if err != nil {
			return decoded, fmt.Errorf("%s %s", key, err.Error())
		}
		*axis.target = values
	}
	for name := range entry {
		switch name {
		case "capabilities", "grants", "denials", "constraints", "routes", "identities":
		default:
			return decoded, fmt.Errorf("%s names unknown key `%s`", key, name)
		}
	}
	return decoded, nil
}

// decodeHarnessAxis reads one requirement axis. An absent key is an empty
// axis, never a refusal.
func decodeHarnessAxis(entry map[string]any, axis string) ([]string, error) {
	raw, present := entry[axis]
	if !present || raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s names no %s", axis, axis)
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		value, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%s names no %s", axis, axis)
		}
		values = append(values, value)
	}
	return values, nil
}

// decodeHarnessStringList reads one declared string list. An absent key is
// an empty axis, never a refusal.
func decodeHarnessStringList(mapping map[string]any, key, what string) ([]string, error) {
	raw, present := mapping[key]
	if !present || raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("harness %s names no %s", key, what)
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		value, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("harness %s names no %s", key, what)
		}
		values = append(values, value)
	}
	return values, nil
}

// harnessRequiredString reads one required scalar. A missing or mistyped
// value refuses with the caller's reason.
func harnessRequiredString(mapping map[string]any, key, reason string) (string, error) {
	raw, present := mapping[key]
	if !present || raw == nil {
		return "", fmt.Errorf("%s", reason)
	}
	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s", reason)
	}
	return value, nil
}

// harnessOptionalString reads one optional scalar. An absent or null value
// is no declaration; a mistyped one refuses with the caller's reason.
func harnessOptionalString(mapping map[string]any, key, reason string) (*string, error) {
	raw, present := mapping[key]
	if !present || raw == nil {
		return nil, nil
	}
	value, ok := raw.(string)
	if !ok {
		return nil, fmt.Errorf("%s", reason)
	}
	return &value, nil
}

func decodeHarnessProfiles(mapping map[string]any) ([]harnessProfile, error) {
	raw, present := mapping["profiles"]
	if !present || raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("harness profiles names no adapter profiles")
	}
	profiles := make([]harnessProfile, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("harness profiles names no adapter profile")
		}
		profile, err := decodeHarnessProfile(entry)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

func decodeHarnessProfile(entry map[string]any) (harnessProfile, error) {
	profile := harnessProfile{}
	for key := range entry {
		switch key {
		case "id", "supports", "instruction-adapter", "agent-adapter", "skill-adapter", "tiers":
		default:
			return profile, fmt.Errorf("harness profiles names unknown key `%s`", key)
		}
	}
	var err error
	if profile.id, err = harnessRequiredString(entry, "id", "harness profiles id names no identifier"); err != nil {
		return profile, err
	}
	if profile.supports, err = decodeHarnessSupports(entry); err != nil {
		return profile, err
	}
	if profile.instructionAdapter, err = decodeHarnessInstructionAdapter(entry); err != nil {
		return profile, err
	}
	if profile.agentAdapter, err = decodeHarnessAdapter(entry, "agent-adapter", true); err != nil {
		return profile, err
	}
	if profile.skillAdapter, err = decodeHarnessAdapter(entry, "skill-adapter", false); err != nil {
		return profile, err
	}
	if profile.tiers, err = decodeHarnessTiers(entry); err != nil {
		return profile, err
	}
	return profile, nil
}

func decodeHarnessInstructionAdapter(entry map[string]any) (*harnessInstructionAdapter, error) {
	raw, present := entry["instruction-adapter"]
	if !present || raw == nil {
		return nil, nil
	}
	mapping, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("harness profiles instruction-adapter names no instruction adapter")
	}
	decoded := &harnessInstructionAdapter{}
	for key := range mapping {
		if key != "path" && key != "route" {
			return nil, fmt.Errorf("harness profiles instruction-adapter names unknown key `%s`", key)
		}
	}
	var err error
	if decoded.path, err = harnessRequiredString(mapping, "path", "harness profiles instruction-adapter names no path"); err != nil {
		return nil, err
	}
	if decoded.route, err = harnessRequiredString(mapping, "route", "harness profiles instruction-adapter names no route"); err != nil {
		return nil, err
	}
	return decoded, nil
}

// decodeHarnessAdapter reads one native profile representation. The agent
// flag selects which key names it; a skill adapter never selects agents,
// tiers, or dispatch lists, which the planner refuses rather than this
// decoder, so the declaration order of refusals stays the planner's.
func decodeHarnessAdapter(entry map[string]any, key string, _ bool) (*harnessAdapter, error) {
	raw, present := entry[key]
	if !present || raw == nil {
		return nil, nil
	}
	mapping, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("harness profiles %s names no adapter", key)
	}
	decoded := &harnessAdapter{
		identity: map[string]string{},
		fixed:    map[string]string{},
		lists:    map[string]string{},
	}
	for name := range mapping {
		switch name {
		case "path", "format", "route-field", "route", "identity", "fixed",
			"tier-fields", "empty-tier", "lists", "dispatches",
			"agents", "absent", "translations":
		default:
			return nil, fmt.Errorf("harness profiles %s names unknown key `%s`", key, name)
		}
	}
	var err error
	if decoded.path, err = harnessRequiredString(mapping, "path", fmt.Sprintf("harness profiles %s names no path", key)); err != nil {
		return nil, err
	}
	format, err := harnessRequiredString(mapping, "format", fmt.Sprintf("harness profiles %s names no format", key))
	if err != nil {
		return nil, err
	}
	if format != "front-matter" && format != "toml" {
		return nil, fmt.Errorf("harness profiles %s names unknown format `%s`: expected `front-matter` or `toml`", key, format)
	}
	decoded.format = format
	if decoded.routeField, err = harnessRequiredString(mapping, "route-field", fmt.Sprintf("harness profiles %s names no route field", key)); err != nil {
		return nil, err
	}
	if decoded.route, err = harnessRequiredString(mapping, "route", fmt.Sprintf("harness profiles %s names no route", key)); err != nil {
		return nil, err
	}
	if decoded.identity, err = decodeHarnessStringMap(mapping, "identity", key, decodeHarnessIdentity); err != nil {
		return nil, err
	}
	if decoded.fixed, err = decodeHarnessStringMap(mapping, "fixed", key, decodeHarnessFixedValue); err != nil {
		return nil, err
	}
	if decoded.lists, err = decodeHarnessStringMap(mapping, "lists", key, decodeHarnessListName); err != nil {
		return nil, err
	}
	if decoded.tierFields, err = decodeHarnessTierFields(mapping, key); err != nil {
		return nil, err
	}
	if decoded.emptyTier, err = decodeHarnessEmptyTier(mapping, key); err != nil {
		return nil, err
	}
	if decoded.dispatches, err = decodeHarnessDispatches(mapping, key); err != nil {
		return nil, err
	}
	if decoded.agents, err = decodeHarnessAgentSelection(mapping, key); err != nil {
		return nil, err
	}
	if decoded.absent, err = decodeHarnessStringList(mapping, "absent", "absent fields"); err != nil {
		return nil, err
	}
	if decoded.translations, err = decodeHarnessTranslations(mapping, key); err != nil {
		return nil, err
	}
	return decoded, nil
}

func decodeHarnessIdentity(raw any) (string, error) {
	value, ok := raw.(string)
	if !ok || (value != "name" && value != "description") {
		return "", fmt.Errorf("harness profiles identity names unknown identity `%v`: expected `name` or `description`", raw)
	}
	return value, nil
}

func decodeHarnessFixedValue(raw any) (string, error) {
	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("harness profiles fixed names no fixed value")
	}
	return value, nil
}

func decodeHarnessListName(raw any) (string, error) {
	value, ok := raw.(string)
	if !ok || value != "skills" {
		return "", fmt.Errorf("harness profiles lists names unknown list `%v`: expected `skills`", raw)
	}
	return value, nil
}

// decodeHarnessStringMap reads one native field map keyed by native field.
// An absent key is an empty map, never a refusal.
func decodeHarnessStringMap(mapping map[string]any, key, adapter string, value func(any) (string, error)) (map[string]string, error) {
	raw, present := mapping[key]
	if !present || raw == nil {
		return map[string]string{}, nil
	}
	entry, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("harness profiles %s %s names no %s", adapter, key, key)
	}
	decoded := make(map[string]string, len(entry))
	for name, item := range entry {
		projected, err := value(item)
		if err != nil {
			return nil, err
		}
		decoded[name] = projected
	}
	return decoded, nil
}

func decodeHarnessTierFields(mapping map[string]any, adapter string) (*harnessTierFields, error) {
	raw, present := mapping["tier-fields"]
	if !present || raw == nil {
		return nil, nil
	}
	entry, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("harness profiles %s tier-fields names no tier fields", adapter)
	}
	decoded := &harnessTierFields{}
	for name := range entry {
		if name != "model" && name != "effort" {
			return nil, fmt.Errorf("harness profiles %s tier-fields names unknown key `%s`", adapter, name)
		}
	}
	var err error
	if decoded.model, err = harnessRequiredString(entry, "model", fmt.Sprintf("harness profiles %s tier-fields names no model", adapter)); err != nil {
		return nil, err
	}
	if decoded.effort, err = harnessRequiredString(entry, "effort", fmt.Sprintf("harness profiles %s tier-fields names no effort", adapter)); err != nil {
		return nil, err
	}
	return decoded, nil
}

func decodeHarnessEmptyTier(mapping map[string]any, adapter string) (*harnessEmptyTier, error) {
	raw, present := mapping["empty-tier"]
	if !present || raw == nil {
		return nil, nil
	}
	entry, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("harness profiles %s empty-tier names no empty tier", adapter)
	}
	decoded := &harnessEmptyTier{}
	for name := range entry {
		if name != "model" && name != "effort" {
			return nil, fmt.Errorf("harness profiles %s empty-tier names unknown key `%s`", adapter, name)
		}
	}
	var err error
	if decoded.model, err = harnessOptionalString(entry, "model", fmt.Sprintf("harness profiles %s empty-tier model names no model", adapter)); err != nil {
		return nil, err
	}
	if decoded.effort, err = harnessOptionalString(entry, "effort", fmt.Sprintf("harness profiles %s empty-tier effort names no effort", adapter)); err != nil {
		return nil, err
	}
	return decoded, nil
}

func decodeHarnessDispatches(mapping map[string]any, adapter string) (*harnessDispatches, error) {
	raw, present := mapping["dispatches"]
	if !present || raw == nil {
		return nil, nil
	}
	entry, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("harness profiles %s dispatches names no dispatches", adapter)
	}
	decoded := &harnessDispatches{}
	for name := range entry {
		if name != "field" && name != "format" {
			return nil, fmt.Errorf("harness profiles %s dispatches names unknown key `%s`", adapter, name)
		}
	}
	var err error
	if decoded.field, err = harnessRequiredString(entry, "field", fmt.Sprintf("harness profiles %s dispatches names no field", adapter)); err != nil {
		return nil, err
	}
	if decoded.format, err = harnessRequiredString(entry, "format", fmt.Sprintf("harness profiles %s dispatches names no format", adapter)); err != nil {
		return nil, err
	}
	return decoded, nil
}

func decodeHarnessAgentSelection(mapping map[string]any, adapter string) (*[]string, error) {
	raw, present := mapping["agents"]
	if !present || raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("harness profiles %s agents names no agents", adapter)
	}
	selected := make([]string, 0, len(items))
	for _, item := range items {
		name, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("harness profiles %s agents names no agents", adapter)
		}
		selected = append(selected, name)
	}
	return &selected, nil
}

func decodeHarnessTranslations(mapping map[string]any, adapter string) ([]harnessTranslation, error) {
	raw, present := mapping["translations"]
	if !present || raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("harness profiles %s translations names no translations", adapter)
	}
	translations := make([]harnessTranslation, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("harness profiles %s translations names no translation", adapter)
		}
		translation, err := decodeHarnessTranslation(entry, adapter)
		if err != nil {
			return nil, err
		}
		translations = append(translations, translation)
	}
	return translations, nil
}

func decodeHarnessTranslation(entry map[string]any, adapter string) (harnessTranslation, error) {
	translation := harnessTranslation{entries: map[string]string{}}
	for name := range entry {
		switch name {
		case "when", "capability", "field", "members", "absent-members", "entries":
		default:
			return translation, fmt.Errorf("harness profiles %s translations names unknown key `%s`", adapter, name)
		}
	}
	when, err := harnessRequiredString(entry, "when", fmt.Sprintf("harness profiles %s translations names no occasion", adapter))
	if err != nil {
		return translation, err
	}
	switch when {
	case "always", "requires", "denies", "constrains", "no-dispatches":
		translation.when = when
	default:
		return translation, fmt.Errorf("harness profiles %s translations names unknown occasion `%s`: expected `always`, `requires`, `denies`, `constrains`, or `no-dispatches`", adapter, when)
	}
	if translation.capability, err = harnessOptionalString(entry, "capability", fmt.Sprintf("harness profiles %s translations capability names no capability", adapter)); err != nil {
		return translation, err
	}
	if translation.field, err = harnessRequiredString(entry, "field", fmt.Sprintf("harness profiles %s translations names no field", adapter)); err != nil {
		return translation, err
	}
	if translation.members, err = decodeHarnessStringList(entry, "members", "members"); err != nil {
		return translation, err
	}
	if translation.absentMembers, err = decodeHarnessStringList(entry, "absent-members", "absent members"); err != nil {
		return translation, err
	}
	rawEntries, present := entry["entries"]
	if present && rawEntries != nil {
		mapping, ok := rawEntries.(map[string]any)
		if !ok {
			return translation, fmt.Errorf("harness profiles %s translations entries names no entries", adapter)
		}
		for name, item := range mapping {
			value, ok := item.(string)
			if !ok {
				return translation, fmt.Errorf("harness profiles %s translations entries names no entry", adapter)
			}
			translation.entries[name] = value
		}
	}
	return translation, nil
}

func decodeHarnessTiers(entry map[string]any) (map[string]harnessTier, error) {
	raw, present := entry["tiers"]
	if !present || raw == nil {
		return nil, nil
	}
	mapping, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("harness profiles tiers names no tiers")
	}
	tiers := make(map[string]harnessTier, len(mapping))
	for name, item := range mapping {
		declared, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("harness profiles tiers names no tier")
		}
		tier, err := decodeHarnessTier(declared)
		if err != nil {
			return nil, err
		}
		tiers[name] = tier
	}
	return tiers, nil
}

func decodeHarnessTier(declared map[string]any) (harnessTier, error) {
	tier := harnessTier{}
	for name := range declared {
		switch name {
		case "model", "model-resolve", "effort":
		default:
			return tier, fmt.Errorf("harness profiles tiers names unknown key `%s`", name)
		}
	}
	var err error
	if tier.model, err = harnessOptionalString(declared, "model", "harness profiles tiers model names no model"); err != nil {
		return tier, err
	}
	if tier.effort, err = harnessOptionalString(declared, "effort", "harness profiles tiers effort names no effort"); err != nil {
		return tier, err
	}
	raw, present := declared["model-resolve"]
	if present && raw != nil {
		mapping, ok := raw.(map[string]any)
		if !ok {
			return tier, fmt.Errorf("harness profiles tiers model-resolve names no model resolution")
		}
		resolve := &harnessModelResolve{}
		for name := range mapping {
			switch name {
			case "command", "pattern", "fallback":
			default:
				return tier, fmt.Errorf("harness profiles tiers model-resolve names unknown key `%s`", name)
			}
		}
		rawCommand, present := mapping["command"]
		if !present || rawCommand == nil {
			return tier, fmt.Errorf("harness profiles tiers model-resolve names no command")
		}
		argv, ok := rawCommand.([]any)
		if !ok {
			return tier, fmt.Errorf("harness profiles tiers model-resolve names no command")
		}
		resolve.command = make([]string, 0, len(argv))
		for _, argument := range argv {
			text, ok := argument.(string)
			if !ok {
				return tier, fmt.Errorf("harness profiles tiers model-resolve names no command")
			}
			resolve.command = append(resolve.command, text)
		}
		if resolve.pattern, err = harnessRequiredString(mapping, "pattern", "harness profiles tiers model-resolve names no pattern"); err != nil {
			return tier, err
		}
		if resolve.fallback, err = harnessRequiredString(mapping, "fallback", "harness profiles tiers model-resolve names no fallback"); err != nil {
			return tier, err
		}
		tier.modelResolve = resolve
	}
	return tier, nil
}

// Entry points. Validation reads the model each binding recorded and never
// starts the declared command; generation resolves through the one boundary
// that may start it.

func harnessValidate(document *config.Document, tree Tree, format Format) Outcome {
	decoded, refusal := decodeHarness(document)
	if refusal != nil {
		return harnessDecodeRefused("harness-adapters-validate", format, refusal)
	}
	plan, reason, failed := harnessPlan(decoded, tree, nil, true)
	if failed {
		return harnessRefused(format, reason)
	}
	excluded, excludesErr := document.ScanExcludeDirectories()
	if excludesErr != nil {
		return reportRefused("harness-adapters-validate", format, excludesErr.Error())
	}
	differences := harnessDifferences(tree, plan)
	markers, refusalOutcome, failed := harnessStaleMarkers(decoded, tree, plan, excluded, format)
	if failed {
		return refusalOutcome
	}
	differences = append(differences, markers...)
	sort.Strings(differences)
	if len(differences) == 0 {
		return harnessClean(format, "canonical adapter validation is clean")
	}
	return harnessFindings(format, differences)
}

func harnessGenerate(document *config.Document, tree Tree, adapters AdapterStore, models ModelResolver, format Format) Outcome {
	decoded, refusal := decodeHarness(document)
	if refusal != nil {
		return harnessDecodeRefused("harness-adapters-generate", format, refusal)
	}
	if adapters == nil {
		adapters = closedAdapterStore{}
	}
	if models == nil {
		models = closedModelResolver{}
	}
	plan, reason, failed := harnessPlan(decoded, tree, models, false)
	if failed {
		return harnessRefused(format, reason)
	}
	// Read before anything is written, so a marker surface that cannot be
	// read refuses the run before generation acts.
	excluded, excludesErr := document.ScanExcludeDirectories()
	if excludesErr != nil {
		return reportRefused("harness-adapters-generate", format, excludesErr.Error())
	}
	markers, refusalOutcome, failed := harnessStaleMarkers(decoded, tree, plan, excluded, format)
	if failed {
		return refusalOutcome
	}
	if len(harnessDifferences(tree, plan)) != 0 {
		if err := adapters.Replace(tree.Root(), plan.transaction); err != nil {
			return harnessRefused(format, err.Error())
		}
	}
	// Generation owns only its families and exact files, so it cannot clear
	// a marker elsewhere; it writes what it owns and reports the rest.
	var outcome Outcome
	if len(markers) == 0 {
		outcome = harnessClean(format, "canonical adapter generation is current")
	} else {
		outcome = harnessFindings(format, markers)
	}
	for _, warning := range plan.warnings {
		outcome.Stderr += "[harness-adapters] warning: " + warning + "\n"
	}
	return outcome
}

// harnessDecodeRefused renders a configuration refusal. An omitted group
// already carries its own prefix, and a structural refusal follows the
// sibling convention of naming only the offending key. A semantic tier
// refusal names the asking leaf, the way the oracle's parse errors do.
func harnessDecodeRefused(category string, format Format, refusal *Refusal) Outcome {
	if strings.HasPrefix(refusal.Message, "[harness-adapters]") {
		return RefuseWith(format, refusal.Code, refusal.Message)
	}
	if strings.HasPrefix(refusal.Message, config.Path+": ") {
		return reportRefused(category, format, refusal.Message)
	}
	return RefuseWith(format, refusal.Code, refusal.Message)
}

// harnessPlan renders, without writing, every adapter family the
// configuration declares. Semantic loss happens here, before any store is
// referenced.
type harnessPlanResult struct {
	transaction AdapterTransaction
	desired     map[string]string
	warnings    []string
}

func harnessPlan(decoded *harnessConfig, tree Tree, models ModelResolver, recordOnly bool) (*harnessPlanResult, string, bool) {
	if len(decoded.profiles) == 0 {
		return nil, "harness: at least one adapter profile is required", true
	}
	roots, staticExact, reason, failed := validateHarnessProfiles(decoded)
	if failed {
		return nil, reason, true
	}
	agentsRoot, reason, failed := harnessAgentsRoot(decoded)
	if failed {
		return nil, reason, true
	}
	// A declared root names a tree the repository must hold: refusing a
	// missing one here keeps generation from writing adapters for an empty
	// discovery. An omitted root keeps the historical tree, which every
	// existing repository already holds, so no existence check applies there.
	if decoded.agentsRoot != nil {
		trimmed := strings.TrimSuffix(agentsRoot, "/")
		found := false
		for _, path := range tree.Files() {
			if strings.HasPrefix(path, agentsRoot) {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Sprintf("canonical agents root `%s` is missing", trimmed), true
		}
	}
	sources, reason, failed := harnessDiscover(tree, decoded, agentsRoot)
	if failed {
		return nil, reason, true
	}
	for _, profile := range decoded.profiles {
		if reason, failed := requireHarnessSelectedSources(&profile, sources); failed {
			return nil, reason, true
		}
	}
	if reason, failed := requireHarnessDeclaredTiers(decoded, sources); failed {
		return nil, reason, true
	}
	plan := &harnessPlanResult{desired: map[string]string{}}
	for i := range decoded.profiles {
		profile := &decoded.profiles[i]
		// Validation keeps the model each binding recorded and never starts
		// the declared command; generation resolves through the one boundary
		// that may start it.
		modelsResolved := harnessResolvedModels{rendered: map[string]string{}, recorded: map[string]string{}}
		if recordOnly {
			modelsResolved = resolveHarnessRecorded(profile, tree)
		} else {
			modelsResolved = resolveHarnessModels(profile, tree, models, &plan.warnings)
		}
		if reason, failed := renderHarnessProfile(profile, sources, modelsResolved, plan.desired); failed {
			return nil, reason, true
		}
	}
	// Per-agent exact outputs merge into the canonical agents tree: every
	// desired file outside every family root is an individually managed file.
	// Family files stay under their roots; instruction files are the static
	// exact set. The remainder is the exact agent set.
	exactPaths := append([]string(nil), staticExact...)
	for path := range plan.desired {
		under := false
		for _, root := range roots {
			if underHarnessRoot(path, root) {
				under = true
				break
			}
		}
		if !under && !containsHarnessPath(exactPaths, path) {
			exactPaths = append(exactPaths, path)
		}
	}
	normalizedExact, err := harnessExactPaths(roots, exactPaths)
	if err != nil {
		return nil, "harness: " + err.Error(), true
	}
	files := make([]AdapterFile, 0, len(plan.desired))
	paths := make([]string, 0, len(plan.desired))
	for path := range plan.desired {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		files = append(files, AdapterFile{Path: path, Contents: plan.desired[path]})
	}
	plan.transaction = AdapterTransaction{Roots: roots, ExactPaths: normalizedExact, Files: files}
	return plan, "", false
}

// validateHarnessProfiles checks every adapter family declaration before any
// file is read. It returns the family roots and the static exact set.
func validateHarnessProfiles(decoded *harnessConfig) ([]string, []string, string, bool) {
	fail := func(reason string) ([]string, []string, string, bool) {
		return nil, nil, reason, true
	}
	needsAgents := false
	for _, profile := range decoded.profiles {
		if profile.agentAdapter != nil {
			needsAgents = true
		}
	}
	needsSkills := false
	for _, profile := range decoded.profiles {
		if profile.skillAdapter != nil {
			needsSkills = true
		}
	}
	if needsAgents && (decoded.canonical == nil || decoded.canonical.agents == nil) {
		return fail("harness: agent adapters require a canonical agent field declaration")
	}
	needsDispatches := false
	for _, profile := range decoded.profiles {
		if profile.agentAdapter != nil && profile.agentAdapter.dispatches != nil {
			needsDispatches = true
		}
	}
	if needsDispatches && (decoded.canonical == nil || decoded.canonical.agents == nil || decoded.canonical.agents.dispatches == nil) {
		return fail("harness: a dispatches adapter key requires a canonical dispatches field declaration")
	}
	if needsSkills && (decoded.canonical == nil || decoded.canonical.skills == nil) {
		return fail("harness: skill adapters require a canonical skill field declaration")
	}
	seen := map[string]bool{}
	var roots []string
	var exactPaths []string
	for _, profile := range decoded.profiles {
		if strings.TrimSpace(profile.id) == "" {
			return fail("harness: an adapter profile has an empty id")
		}
		if seen[profile.id] {
			return fail(fmt.Sprintf("harness: duplicated adapter profile `%s`", profile.id))
		}
		seen[profile.id] = true
		declaresOutput := false
		if profile.agentAdapter != nil {
			if isExactHarnessAgentPattern(profile.agentAdapter.path) {
				if reason, failed := validateHarnessExactAgent(profile.id, profile.agentAdapter); failed {
					return fail(reason)
				}
			} else {
				root, reason, failed := validateHarnessAdapter(profile.id, profile.agentAdapter, true)
				if failed {
					return fail(reason)
				}
				roots = append(roots, root)
			}
			declaresOutput = true
		}
		if profile.skillAdapter != nil {
			root, reason, failed := validateHarnessAdapter(profile.id, profile.skillAdapter, false)
			if failed {
				return fail(reason)
			}
			roots = append(roots, root)
			declaresOutput = true
		}
		if profile.instructionAdapter != nil {
			path, ok := normalHarnessPath(profile.instructionAdapter.path)
			if !ok || path != profile.instructionAdapter.path || path == harnessRootInstruction || strings.HasPrefix(path, ".agents/") {
				return fail(fmt.Sprintf("profile `%s` has an invalid instruction adapter path", profile.id))
			}
			if reason, failed := validateHarnessRoute(profile.id, profile.instructionAdapter.route); failed {
				return fail(reason)
			}
			exactPaths = append(exactPaths, path)
			declaresOutput = true
		}
		if !declaresOutput {
			return fail(fmt.Sprintf("profile `%s` declares no native adapter representation", profile.id))
		}
		for _, axis := range []struct {
			name     string
			required []string
			support  []string
		}{
			{"capability", decoded.requirements.capabilities, profile.supports.capabilities},
			{"grant", decoded.requirements.grants, profile.supports.grants},
			{"denial", decoded.requirements.denials, profile.supports.denials},
			{"constraint", decoded.requirements.constraints, profile.supports.constraints},
			{"route", decoded.requirements.routes, profile.supports.routes},
			{"identity", decoded.requirements.identities, profile.supports.identities},
		} {
			if reason, failed := requireHarnessAxis(profile.id, axis.name, axis.required, axis.support); failed {
				return fail(reason)
			}
		}
	}
	normalizedRoots, err := harnessAdapterRoots(roots)
	if err != nil {
		return fail("harness: " + err.Error())
	}
	normalizedExact, err := harnessExactPaths(normalizedRoots, exactPaths)
	if err != nil {
		return fail("harness: " + err.Error())
	}
	return normalizedRoots, normalizedExact, "", false
}

// harnessRootInstruction is the repository root instruction the canonical
// discovery always reads.
const harnessRootInstruction = "AGENTS.md"

// harnessAgentsRootValue is the historical canonical agents tree, kept when
// the configuration declares no agents root.
const harnessAgentsHistory = ".agents/agents/"

// harnessSkillsRoot is the canonical skills tree discovery always reads.
const harnessSkillsRoot = ".agents/skills/"

// harnessAllowMap is the dispatch format that writes a deny-all entry and
// one allow per name.
const harnessAllowMap = "allow-map"

// harnessNamesPlaceholder is the placeholder a dispatch member template
// replaces with the names.
const harnessNamesPlaceholder = "{names}"

// harnessGeneratedMarker is the phrase that claims a file, or a region of
// one, as RHINO output. Matched case-insensitively on any line. RHINO
// writes it nowhere, so a file outside every adapter family that carries it
// tells a maintainer the region is machine-owned when nothing owns it.
const harnessGeneratedMarker = "rhino generated"

// Whether an agent pattern writes one file per agent inside the canonical
// agents tree instead of replacing a family directory.
//
// The shape is exactly one `{name}` placeholder with a literal path segment
// after it, under the canonical agents directory: `.agents/agents/{name}`
// plus `/` and more. A flat `.agents/agents/{name}.md` has no such segment
// and stays invalid, so generation can never overwrite a canonical source.
func isExactHarnessAgentPattern(pattern string) bool {
	head, tail, found := strings.Cut(pattern, "{name}")
	if !found {
		return false
	}
	if strings.Count(pattern, "{name}") != 1 {
		return false
	}
	if !strings.HasPrefix(tail, "/") || len(tail) <= 1 {
		return false
	}
	if strings.ContainsAny(head, "{}") || strings.ContainsAny(tail, "{}") {
		return false
	}
	sample := strings.ReplaceAll(pattern, "{name}", "adapter")
	normalized, ok := normalHarnessPath(sample)
	if !ok || normalized != sample {
		return false
	}
	root := strings.TrimSuffix(head, "/")
	normalizedRoot, ok := normalHarnessPath(root)
	if !ok || normalizedRoot != root {
		return false
	}
	return normalizedRoot == strings.TrimSuffix(harnessAgentsHistory, "/")
}

func validateHarnessExactAgent(profile string, adapter *harnessAdapter) (string, bool) {
	if !isExactHarnessAgentPattern(adapter.path) {
		return fmt.Sprintf("profile `%s` has an invalid adapter path", profile), true
	}
	return validateHarnessAdapterFields(profile, adapter, true)
}

func validateHarnessAdapter(profile string, adapter *harnessAdapter, agent bool) (string, string, bool) {
	root, ok := harnessAdapterRoot(adapter.path)
	if !ok {
		return "", fmt.Sprintf("profile `%s` has an invalid adapter path", profile), true
	}
	if strings.HasPrefix(root, ".agents/") || root == harnessRootInstruction {
		return "", fmt.Sprintf("profile `%s` has an invalid adapter path", profile), true
	}
	if reason, failed := validateHarnessAdapterFields(profile, adapter, agent); failed {
		return "", reason, true
	}
	return root, "", false
}

func validateHarnessAdapterFields(profile string, adapter *harnessAdapter, agent bool) (string, bool) {
	fail := func(reason string) (string, bool) { return reason, true }
	if reason, failed := validateHarnessRoute(profile, adapter.route); failed {
		return fail(reason)
	}
	if (adapter.format == "front-matter") != (adapter.routeField == "body") {
		return fail(fmt.Sprintf("profile `%s` has a route field incompatible with its adapter format", profile))
	}
	if strings.TrimSpace(adapter.routeField) == "" {
		return fail(fmt.Sprintf("profile `%s` has an empty adapter route field", profile))
	}
	if !agent && adapter.tierFields != nil {
		return fail(fmt.Sprintf("profile `%s` projects a tier into a skill adapter", profile))
	}
	if !agent && adapter.emptyTier != nil {
		return fail(fmt.Sprintf("profile `%s` projects an empty tier into a skill adapter", profile))
	}
	if adapter.emptyTier != nil && adapter.tierFields == nil {
		return fail(fmt.Sprintf("profile `%s` declares empty-tier values without tier fields", profile))
	}
	if !agent && len(adapter.lists) != 0 {
		return fail(fmt.Sprintf("profile `%s` projects an agent list into a skill adapter", profile))
	}
	if !agent && adapter.agents != nil {
		return fail(fmt.Sprintf("profile `%s` selects agents in a skill adapter", profile))
	}
	if adapter.dispatches != nil {
		if !agent {
			return fail(fmt.Sprintf("profile `%s` projects a dispatch list into a skill adapter", profile))
		}
		if !validHarnessDispatches(adapter.dispatches) {
			return fail(fmt.Sprintf("profile `%s` has an invalid dispatches declaration", profile))
		}
	}
	directFields := map[string]bool{}
	for _, names := range [][]string{
		sortedHarnessMapKeys(adapter.identity),
		sortedHarnessMapKeys(adapter.fixed),
		sortedHarnessMapKeys(adapter.lists),
	} {
		for _, field := range names {
			if strings.TrimSpace(field) == "" || directFields[field] {
				return fail(fmt.Sprintf("profile `%s` repeats or omits an adapter field", profile))
			}
			directFields[field] = true
		}
	}
	if adapter.dispatches != nil {
		root := adapter.dispatches.field
		if parent, _, found := strings.Cut(root, "."); found {
			root = parent
		}
		if directFields[root] {
			return fail(fmt.Sprintf("profile `%s` repeats or omits an adapter field", profile))
		}
	}
	translationFields := map[string]bool{}
	for _, translation := range adapter.translations {
		members := len(translation.members) != 0 || len(translation.absentMembers) != 0
		entries := len(translation.entries) != 0
		occurrences := 0
		if members {
			occurrences++
		}
		if entries {
			occurrences++
		}
		capability := translation.when == "always" || translation.when == "no-dispatches"
		if strings.TrimSpace(translation.field) == "" ||
			occurrences != 1 ||
			capability != (translation.capability == nil) ||
			directFields[translation.field] {
			return fail(fmt.Sprintf("profile `%s` has an invalid capability translation", profile))
		}
		if previous, present := translationFields[translation.field]; present && previous != members {
			return fail(fmt.Sprintf("profile `%s` mixes incompatible capability translations", profile))
		}
		translationFields[translation.field] = members
	}
	if adapter.tierFields != nil {
		fields := adapter.tierFields
		if strings.TrimSpace(fields.model) == "" ||
			strings.TrimSpace(fields.effort) == "" ||
			fields.model == fields.effort {
			return fail(fmt.Sprintf("profile `%s` has invalid tier fields", profile))
		}
	}
	return "", false
}

// sortedHarnessMapKeys lists one native field map's keys in the sorted
// order the oracle's ordered maps compare them in.
func sortedHarnessMapKeys(group map[string]string) []string {
	names := make([]string, 0, len(group))
	for name := range group {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// A dispatch field is one native field, or one map field and its key; an
// allow map may nest, and a member template adds to a top-level member list
// and names the list exactly once.
func validHarnessDispatches(dispatches *harnessDispatches) bool {
	segments := strings.Split(dispatches.field, ".")
	if len(segments) > 2 {
		return false
	}
	for _, segment := range segments {
		if strings.TrimSpace(segment) == "" {
			return false
		}
	}
	return dispatches.format == harnessAllowMap ||
		(len(segments) == 1 && strings.Count(dispatches.format, harnessNamesPlaceholder) == 1)
}

func validateHarnessRoute(profile, route string) (string, bool) {
	if strings.Count(route, "{path}") != 1 {
		return fmt.Sprintf("profile `%s` must declare exactly one `{path}` route placeholder", profile), true
	}
	return "", false
}

func harnessAdapterRoot(pattern string) (string, bool) {
	if strings.Count(pattern, "{name}") != 1 {
		return "", false
	}
	sample := strings.ReplaceAll(pattern, "{name}", "adapter")
	normalized, ok := normalHarnessPath(sample)
	if !ok || normalized != sample {
		return "", false
	}
	head, _, _ := strings.Cut(pattern, "{name}")
	root := strings.TrimSuffix(head, "/")
	normalizedRoot, ok := normalHarnessPath(root)
	if !ok {
		return "", false
	}
	return normalizedRoot, true
}

func requireHarnessAxis(profile, axis string, required, supported []string) (string, bool) {
	support := map[string]bool{}
	for _, item := range supported {
		support[item] = true
	}
	for _, item := range required {
		if strings.TrimSpace(item) == "" {
			return fmt.Sprintf("harness: required %s is empty", axis), true
		}
		if !support[item] {
			return fmt.Sprintf("profile `%s` cannot represent required %s `%s`", profile, axis, item), true
		}
	}
	return "", false
}

// harnessAgentsRoot reports the canonical agents root this run discovers,
// with its trailing separator. An omitted `agents-root` keeps the
// historical `.agents/agents/` tree, so a repository that declares nothing
// discovers exactly what it always did. A declared root is normalized like
// every other repository path; one that climbs out of the repository, or is
// empty, refuses here before discovery reads it.
func harnessAgentsRoot(decoded *harnessConfig) (string, string, bool) {
	if decoded.agentsRoot == nil {
		return harnessAgentsHistory, "", false
	}
	root, ok := normalHarnessPath(*decoded.agentsRoot)
	if !ok {
		return "", fmt.Sprintf("harness: invalid canonical agents root `%s`", *decoded.agentsRoot), true
	}
	return root + "/", "", false
}

func containsHarnessPath(paths []string, path string) bool {
	for _, held := range paths {
		if held == path {
			return true
		}
	}
	return false
}

// normalHarnessPath normalizes one repository path the way every adapter
// path is normalized: trimmed of separators, with no empty, current-dir,
// parent-dir, or backslash segments.
func normalHarnessPath(path string) (string, bool) {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return "", false
	}
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == "" || segment == "." || segment == ".." || strings.ContainsRune(segment, '\\') {
			return "", false
		}
	}
	return trimmed, true
}

func underHarnessRoot(path, root string) bool {
	return path == root || (strings.HasPrefix(path, root) && len(path) > len(root) && path[len(root)] == '/')
}

// harnessAdapterRoots normalizes family roots and refuses duplicates and
// nesting: one family inside another would make ownership ambiguous.
func harnessAdapterRoots(roots []string) ([]string, error) {
	normalized := make([]string, 0, len(roots))
	for _, root := range roots {
		path, ok := normalHarnessPath(root)
		if !ok {
			return nil, fmt.Errorf("invalid adapter root `%s`", root)
		}
		normalized = append(normalized, path)
	}
	sort.Strings(normalized)
	deduped := normalized[:0]
	for i, path := range normalized {
		if i == 0 || path != normalized[i-1] {
			deduped = append(deduped, path)
		}
	}
	normalized = deduped
	if len(normalized) != len(roots) {
		return nil, fmt.Errorf("adapter roots must be distinct, non-overlapping repository paths")
	}
	for i := 1; i < len(normalized); i++ {
		if underHarnessRoot(normalized[i], normalized[i-1]) {
			return nil, fmt.Errorf("adapter roots must be distinct, non-overlapping repository paths")
		}
	}
	return normalized, nil
}

// harnessSource is one canonical document an adapter family projects.
type harnessSource struct {
	id       string
	path     string
	digest   string
	kind     string
	metadata *harnessMetadata
}

type harnessMetadata struct {
	name        string
	description string
	tier        *string
	grants      map[string]bool
	denials     map[string]bool
	constraints map[string]bool
	dispatches  []string
	lists       map[string][]string
}

func harnessDiscover(tree Tree, decoded *harnessConfig, agentsRoot string) ([]harnessSource, string, bool) {
	fail := func(reason string) ([]harnessSource, string, bool) { return nil, reason, true }
	instruction, reason, failed := harnessReadSource(tree, harnessRootInstruction, "instruction", "instruction", nil)
	if failed {
		return fail(reason)
	}
	sources := []harnessSource{instruction}
	ids := map[string]bool{instruction.id: true}
	needsAgents := false
	for _, profile := range decoded.profiles {
		if profile.agentAdapter != nil {
			needsAgents = true
		}
	}
	if needsAgents {
		if decoded.canonical == nil || decoded.canonical.agents == nil {
			return fail("harness: agent adapters require a canonical agent field declaration")
		}
		shape := decoded.canonical.agents
		for _, path := range tree.Files() {
			relative, found := strings.CutPrefix(path, agentsRoot)
			if !found {
				continue
			}
			if !strings.HasSuffix(relative, ".md") || strings.ContainsRune(relative, '/') || relative == "README.md" {
				continue
			}
			name := harnessSourceID(relative)
			source, reason, failed := harnessReadSource(tree, path, "agent/"+name, "agent", shape)
			if failed {
				return fail(reason)
			}
			if source.metadata != nil && source.metadata.name != name {
				return fail(fmt.Sprintf("canonical agent `%s` has a name that does not match its path", path))
			}
			if ids[source.id] {
				return fail(fmt.Sprintf("duplicate canonical agent id `%s`", source.id))
			}
			ids[source.id] = true
			sources = append(sources, source)
		}
	}
	needsSkills := false
	for _, profile := range decoded.profiles {
		if profile.skillAdapter != nil {
			needsSkills = true
		}
	}
	if needsSkills {
		if decoded.canonical == nil || decoded.canonical.skills == nil {
			return fail("harness: skill adapters require a canonical skill field declaration")
		}
		shape := harnessSkillShape(decoded.canonical.skills)
		for _, path := range tree.Files() {
			relative, found := strings.CutPrefix(path, harnessSkillsRoot)
			if !found {
				continue
			}
			if !strings.HasSuffix(relative, "/SKILL.md") || strings.Count(relative, "/") != 1 {
				continue
			}
			name := strings.TrimSuffix(relative, "/SKILL.md")
			source, reason, failed := harnessReadSource(tree, path, "skill/"+name, "skill", shape)
			if failed {
				return fail(reason)
			}
			if source.metadata != nil && source.metadata.name != name {
				return fail(fmt.Sprintf("canonical skill `%s` has a name that does not match its path", path))
			}
			if ids[source.id] {
				return fail(fmt.Sprintf("duplicate canonical skill id `%s`", source.id))
			}
			ids[source.id] = true
			sources = append(sources, source)
		}
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].path < sources[j].path })
	return sources, "", false
}

// harnessSkillShape adapts a canonical skill document to the shared shape
// interface: skills declare only name and description.
func harnessSkillShape(document *harnessCanonicalDocument) *harnessCanonicalAgent {
	return &harnessCanonicalAgent{name: document.name, description: document.description}
}

func harnessSourceID(path string) string {
	if index := strings.LastIndex(path, "."); index >= 0 {
		return path[:index]
	}
	return path
}

func harnessReadSource(tree Tree, path, id, kind string, shape *harnessCanonicalAgent) (harnessSource, string, bool) {
	fail := func(reason string) (harnessSource, string, bool) { return harnessSource{}, reason, true }
	contents, readErr := tree.Read(path)
	if readErr != nil {
		switch readErr.Kind {
		case "missing":
			return fail(fmt.Sprintf("canonical source `%s` is missing", path))
		case "not-text":
			return fail(fmt.Sprintf("canonical source `%s` holds non-text bytes", path))
		default:
			return fail(fmt.Sprintf("canonical source `%s` cannot be read: %s", path, readErr.Reason))
		}
	}
	source := harnessSource{id: id, path: path, digest: harnessDigest(contents), kind: kind}
	if shape != nil {
		metadata, reason, failed := harnessParseMetadata(path, contents, shape)
		if failed {
			return fail(reason)
		}
		source.metadata = metadata
	}
	return source, "", false
}

func harnessDigest(contents string) string {
	sum := sha256.Sum256([]byte(contents))
	return hex.EncodeToString(sum[:])
}

// A selection naming an agent with no canonical source fails closed, rather
// than rendering fewer agents than the repository asked for.
func requireHarnessSelectedSources(profile *harnessProfile, sources []harnessSource) (string, bool) {
	for _, name := range harnessAgentSelectionOf(profile) {
		found := false
		for _, source := range sources {
			if harnessSourceAgentName(&source) == name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Sprintf("profile `%s` selects agent `%s` with no canonical source", profile.id, name), true
		}
	}
	return "", false
}

// Every canonical agent names a tier when the canonical shape declares a
// tier field, and every profile with a `tiers` block declares each tier its
// rendered agents name. A profile without a `tiers` block maps no tier, so
// it is exempt; an explicitly empty mapping is a declaration like any other.
func requireHarnessDeclaredTiers(decoded *harnessConfig, sources []harnessSource) (string, bool) {
	var tierField *string
	if decoded.canonical != nil && decoded.canonical.agents != nil {
		tierField = decoded.canonical.agents.tier
	}
	if tierField != nil {
		for _, source := range sources {
			if source.kind == "agent" && harnessSourceAgentTier(&source) == "" {
				return fmt.Sprintf("harness-tier-missing: canonical agent `%s` declares no tier in its `%s` field", source.path, *tierField), true
			}
		}
	}
	for _, profile := range decoded.profiles {
		if profile.agentAdapter == nil || len(profile.tiers) == 0 {
			continue
		}
		for _, source := range harnessSelectedSources(&profile, sources) {
			if tier := harnessSourceAgentTier(&source); tier != "" {
				if _, declared := profile.tiers[tier]; !declared {
					return fmt.Sprintf("harness-tier-undeclared: profile `%s` declares no tier `%s` for canonical agent `%s`", profile.id, tier, source.path), true
				}
			}
		}
	}
	return "", false
}

// harnessSourceAgentTier is a canonical agent's tier; every other source
// kind has none.
func harnessSourceAgentTier(source *harnessSource) string {
	if source.kind != "agent" || source.metadata == nil || source.metadata.tier == nil {
		return ""
	}
	return *source.metadata.tier
}

// The sources one profile renders: every source, less any canonical agent
// its agent adapter does not select. Both of the profile's catalogs read
// this set.
func harnessSelectedSources(profile *harnessProfile, sources []harnessSource) []harnessSource {
	selection := harnessAgentSelectionOf(profile)
	if selection == nil {
		return sources
	}
	selected := make([]harnessSource, 0, len(sources))
	for _, source := range sources {
		if name := harnessSourceAgentName(&source); name != "" {
			keep := false
			for _, wanted := range selection {
				if wanted == name {
					keep = true
					break
				}
			}
			if !keep {
				continue
			}
		}
		selected = append(selected, source)
	}
	return selected
}

// harnessAgentSelectionOf returns the agent names a profile's agent adapter
// selects, or nil when it declares none.
func harnessAgentSelectionOf(profile *harnessProfile) []string {
	if profile.agentAdapter == nil || profile.agentAdapter.agents == nil {
		return nil
	}
	return *profile.agentAdapter.agents
}

// harnessSourceAgentName is a canonical agent's name; every other source
// kind has none.
func harnessSourceAgentName(source *harnessSource) string {
	if source.kind != "agent" || source.metadata == nil {
		return ""
	}
	return source.metadata.name
}

// Resolve each of a profile's resolving tiers once.
//
// A run that resolves a model records it. A run that cannot, and every
// validation, keeps the model the binding family last recorded, or the
// declared fallback when it never recorded one; a failed run says so.
type harnessResolvedModels struct {
	rendered map[string]string
	recorded map[string]string
}

// resolveHarnessRecorded keeps the model each binding recorded, or the
// declared fallback when it never recorded one. Validation resolves this
// way, so checking bindings against a recorded model can never start a
// process.
func resolveHarnessRecorded(profile *harnessProfile, tree Tree) harnessResolvedModels {
	models := harnessResolvedModels{rendered: map[string]string{}, recorded: map[string]string{}}
	if profile.agentAdapter == nil {
		return models
	}
	recorded := harnessRecordedModels(tree, profile.agentAdapter)
	for _, tier := range sortedHarnessTierNames(profile.tiers) {
		mapping := profile.tiers[tier]
		if mapping.modelResolve == nil {
			continue
		}
		if last, ok := recorded[tier]; ok {
			models.recorded[tier] = last
			models.rendered[tier] = last
		} else {
			models.rendered[tier] = mapping.modelResolve.fallback
		}
	}
	return models
}

func sortedHarnessTierNames(tiers map[string]harnessTier) []string {
	names := make([]string, 0, len(tiers))
	for name := range tiers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func resolveHarnessModels(profile *harnessProfile, tree Tree, resolver ModelResolver, warnings *[]string) harnessResolvedModels {
	models := harnessResolvedModels{rendered: map[string]string{}, recorded: map[string]string{}}
	if profile.agentAdapter == nil {
		return models
	}
	recorded := harnessRecordedModels(tree, profile.agentAdapter)
	for _, tier := range sortedHarnessTierNames(profile.tiers) {
		mapping := profile.tiers[tier]
		if mapping.modelResolve == nil {
			continue
		}
		last, hasLast := recorded[tier]
		resolved, reason, failed := harnessNewestModel(resolver, tree.Root(), mapping.modelResolve)
		var record *string
		if !failed {
			record = &resolved
		} else {
			model, source := mapping.modelResolve.fallback, "the fallback"
			if hasLast {
				model, source = last, "the last resolved model"
			}
			*warnings = append(*warnings, fmt.Sprintf("profile `%s` tier `%s` resolved no model (%s); using %s `%s`",
				profile.id, tier, reason, source, model))
			if hasLast {
				kept := last
				record = &kept
			}
		}
		if record != nil {
			models.recorded[tier] = *record
			models.rendered[tier] = *record
		} else {
			models.rendered[tier] = mapping.modelResolve.fallback
		}
	}
	return models
}

// The models a binding family's provenance recorded, by tier. A family that
// was never generated, or whose provenance holds no record, has none.
func harnessRecordedModels(tree Tree, adapter *harnessAdapter) map[string]string {
	recorded := map[string]string{}
	root, ok := harnessAdapterRoot(adapter.path)
	if !ok {
		return recorded
	}
	text, readErr := tree.Read(root + "/provenance.json")
	if readErr != nil {
		return recorded
	}
	var provenance struct {
		ResolvedModels map[string]string `json:"resolvedModels"`
	}
	if err := json.Unmarshal([]byte(text), &provenance); err != nil {
		return recorded
	}
	if provenance.ResolvedModels != nil {
		return provenance.ResolvedModels
	}
	return recorded
}

// Run the declared command and choose the newest model its output names.
func harnessNewestModel(resolver ModelResolver, directory string, resolve *harnessModelResolve) (string, string, bool) {
	output, err := resolver.Resolve(ModelLaunch{
		Arguments:      append([]string(nil), resolve.command...),
		Directory:      directory,
		TimeoutSeconds: harnessResolveTimeoutSeconds,
	})
	if err != nil {
		return "", err.Error(), true
	}
	prefix, suffix, found := strings.Cut(resolve.pattern, "*")
	_ = found
	var bestKey []harnessVersionPart
	var bestToken string
	matched := false
	for _, token := range harnessModelTokens(output) {
		version, ok := cutHarnessAffix(token, prefix, suffix)
		if !ok || version == "" || !isHarnessVersion(version) {
			continue
		}
		key := harnessVersionKey(version)
		if !matched || harnessVersionGreater(key, bestKey) ||
			(harnessVersionEqual(key, bestKey) && token > bestToken) {
			bestKey, bestToken, matched = key, token, true
		}
	}
	if !matched {
		return "", fmt.Sprintf("no output token matched `%s`", resolve.pattern), true
	}
	return bestToken, "", false
}

// harnessResolveTimeoutSeconds bounds one declared model-listing command.
const harnessResolveTimeoutSeconds = 30

func harnessModelTokens(output string) []string {
	var tokens []string
	var current strings.Builder
	flush := func() {
		if current.Len() != 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}
	for _, character := range output {
		if isModelCharacter(character) {
			current.WriteRune(character)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}

func cutHarnessAffix(token, prefix, suffix string) (string, bool) {
	rest, found := strings.CutPrefix(token, prefix)
	if !found {
		return "", false
	}
	version, found := strings.CutSuffix(rest, suffix)
	if !found {
		return "", false
	}
	return version, true
}

func isHarnessVersion(version string) bool {
	for _, character := range version {
		if (character < '0' || character > '9') && character != '.' {
			return false
		}
	}
	return true
}

type harnessVersionPart struct {
	length int
	digits string
}

// A version as dot-separated numbers, each compared by value however many
// digits it is written with.
func harnessVersionKey(version string) []harnessVersionPart {
	parts := strings.Split(version, ".")
	key := make([]harnessVersionPart, 0, len(parts))
	for _, part := range parts {
		digits := strings.TrimLeft(part, "0")
		key = append(key, harnessVersionPart{length: len(digits), digits: digits})
	}
	return key
}

func harnessVersionGreater(left, right []harnessVersionPart) bool {
	for i := 0; i < len(left) && i < len(right); i++ {
		if left[i].length != right[i].length {
			return left[i].length > right[i].length
		}
		if left[i].digits != right[i].digits {
			return left[i].digits > right[i].digits
		}
	}
	return len(left) > len(right)
}

func harnessVersionEqual(left, right []harnessVersionPart) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// harnessExactPaths normalizes individually managed adapter files and
// refuses any overlap with a replaceable family. A path in a family is
// already covered by that family's atomic replacement; listing it twice
// would make ownership ambiguous.
func harnessExactPaths(roots, exact []string) ([]string, error) {
	normalized := make([]string, 0, len(exact))
	for _, path := range exact {
		clean, ok := normalHarnessPath(path)
		if !ok {
			return nil, fmt.Errorf("invalid exact adapter path `%s`", path)
		}
		normalized = append(normalized, clean)
	}
	sort.Strings(normalized)
	deduped := normalized[:0]
	for i, path := range normalized {
		if i == 0 || path != normalized[i-1] {
			deduped = append(deduped, path)
		}
	}
	normalized = deduped
	if len(normalized) != len(exact) {
		return nil, fmt.Errorf("exact adapter paths must be distinct and outside every adapter root")
	}
	for _, path := range normalized {
		for _, root := range roots {
			if underHarnessRoot(path, root) {
				return nil, fmt.Errorf("exact adapter paths must be distinct and outside every adapter root")
			}
		}
	}
	return normalized, nil
}

func renderHarnessProfile(profile *harnessProfile, sources []harnessSource, models harnessResolvedModels, desired map[string]string) (string, bool) {
	selected := harnessSelectedSources(profile, sources)
	if profile.instructionAdapter != nil {
		var instruction *harnessSource
		for i := range selected {
			if selected[i].kind == "instruction" {
				instruction = &selected[i]
				break
			}
		}
		if instruction == nil {
			return "harness: canonical discovery lost the root instruction", true
		}
		route, reason, failed := harnessRoute(profile.instructionAdapter.route, instruction.path)
		if failed {
			return reason, true
		}
		if reason, failed := harnessAddFile(desired, profile.instructionAdapter.path, route+"\n"); failed {
			return reason, true
		}
	}
	if profile.agentAdapter != nil {
		if reason, failed := renderHarnessFamily(profile, profile.agentAdapter, "agent", selected, models, desired); failed {
			return reason, true
		}
	}
	if profile.skillAdapter != nil {
		empty := harnessResolvedModels{rendered: map[string]string{}, recorded: map[string]string{}}
		if reason, failed := renderHarnessFamily(profile, profile.skillAdapter, "skill", selected, empty, desired); failed {
			return reason, true
		}
	}
	return "", false
}

func renderHarnessFamily(profile *harnessProfile, adapter *harnessAdapter, kind string, sources []harnessSource, models harnessResolvedModels, desired map[string]string) (string, bool) {
	root, ok := harnessAdapterRoot(adapter.path)
	if !ok {
		return fmt.Sprintf("profile `%s` has an invalid adapter path", profile.id), true
	}
	for i := range sources {
		source := &sources[i]
		if source.kind != kind {
			continue
		}
		if source.metadata == nil {
			return fmt.Sprintf("profile `%s` renders a source with no canonical metadata", profile.id), true
		}
		rendered, reason, failed := renderHarnessAdapter(profile, adapter, source, models.rendered)
		if failed {
			return reason, true
		}
		path := strings.ReplaceAll(adapter.path, "{name}", source.metadata.name)
		if reason, failed := harnessAddFile(desired, path, rendered); failed {
			return reason, true
		}
	}
	catalog, reason, failed := harnessCatalogJSON(profile.id, sources)
	if failed {
		return reason, true
	}
	if reason, failed := harnessAddFile(desired, root+"/catalog.json", catalog); failed {
		return reason, true
	}
	provenance, reason, failed := harnessProvenanceJSON(profile.id, sources, models.recorded)
	if failed {
		return reason, true
	}
	if reason, failed := harnessAddFile(desired, root+"/provenance.json", provenance); failed {
		return reason, true
	}
	return "", false
}

// harnessRenderField is one native field value: a scalar, a member list, a
// map of entries, or an ordered sequence.
type harnessRenderField struct {
	scalar   *string
	members  []string
	isMember bool
	entries  map[string]harnessEntryValue
	isEntry  bool
	sequence []string
	isSeq    bool
}

type harnessEntryValue struct {
	scalar *string
	nested map[string]string
}

func renderHarnessAdapter(profile *harnessProfile, adapter *harnessAdapter, source *harnessSource, resolved map[string]string) (string, string, bool) {
	fail := func(reason string) (string, string, bool) { return "", reason, true }
	metadata := source.metadata
	fields := map[string]*harnessRenderField{}
	identityNames := make([]string, 0, len(adapter.identity))
	for name := range adapter.identity {
		identityNames = append(identityNames, name)
	}
	sort.Strings(identityNames)
	for _, field := range identityNames {
		value := metadata.description
		if adapter.identity[field] == "name" {
			value = metadata.name
		}
		if reason, failed := harnessAddScalar(fields, field, value); failed {
			return fail(reason)
		}
	}
	fixedNames := make([]string, 0, len(adapter.fixed))
	for name := range adapter.fixed {
		fixedNames = append(fixedNames, name)
	}
	sort.Strings(fixedNames)
	for _, field := range fixedNames {
		if reason, failed := harnessAddScalar(fields, field, adapter.fixed[field]); failed {
			return fail(reason)
		}
	}
	listNames := make([]string, 0, len(adapter.lists))
	for name := range adapter.lists {
		listNames = append(listNames, name)
	}
	sort.Strings(listNames)
	for _, field := range listNames {
		values, present := metadata.lists[adapter.lists[field]]
		if present && len(values) != 0 {
			if reason, failed := harnessAddField(fields, field, &harnessRenderField{sequence: append([]string(nil), values...), isSeq: true}); failed {
				return fail(reason)
			}
		}
	}
	if metadata.tier != nil && adapter.tierFields != nil {
		if mapping, declared := profile.tiers[*metadata.tier]; declared {
			// A mapping pins both or neither, so an unpinned one is the
			// explicitly empty mapping, which renders only the profile's
			// empty-tier values. A resolving mapping pins the model this
			// run resolved for its tier.
			var pinned *string
			if mapping.model != nil {
				pinned = mapping.model
			} else if mapping.modelResolve != nil {
				if model, resolved := resolved[*metadata.tier]; resolved {
					kept := model
					pinned = &kept
				}
			}
			var model, effort *string
			if pinned != nil && mapping.effort != nil {
				model, effort = pinned, mapping.effort
			} else if adapter.emptyTier != nil {
				model, effort = adapter.emptyTier.model, adapter.emptyTier.effort
			}
			if model != nil {
				if reason, failed := harnessAddScalar(fields, adapter.tierFields.model, *model); failed {
					return fail(reason)
				}
			}
			if effort != nil {
				if reason, failed := harnessAddScalar(fields, adapter.tierFields.effort, *effort); failed {
					return fail(reason)
				}
			}
		}
	}
	absentMembers := map[string]map[string]bool{}
	for _, translation := range adapter.translations {
		applies := false
		switch translation.when {
		case "always":
			applies = true
		case "requires":
			applies = translation.capability != nil && metadata.grants[*translation.capability]
		case "denies":
			applies = translation.capability != nil && metadata.denials[*translation.capability]
		case "constrains":
			applies = translation.capability != nil && metadata.constraints[*translation.capability]
		case "no-dispatches":
			applies = len(metadata.dispatches) == 0
		}
		if !applies {
			continue
		}
		if len(translation.absentMembers) == 0 {
			if reason, failed := harnessAddTranslation(fields, &translation); failed {
				return fail(reason)
			}
		} else {
			held := absentMembers[translation.field]
			if held == nil {
				held = map[string]bool{}
				absentMembers[translation.field] = held
			}
			for _, member := range translation.absentMembers {
				held[member] = true
			}
		}
	}
	if adapter.dispatches != nil && len(metadata.dispatches) != 0 {
		if reason, failed := harnessAddDispatches(fields, adapter.dispatches, metadata.dispatches); failed {
			return fail(reason)
		}
	}
	for _, field := range sortedHarnessKeys(absentMembers) {
		entry, present := fields[field]
		if !present || !entry.isMember {
			return fail(fmt.Sprintf("adapter output cannot remove members from undeclared field `%s`", field))
		}
		kept := entry.members[:0]
		for _, member := range entry.members {
			if !absentMembers[field][member] {
				kept = append(kept, member)
			}
		}
		entry.members = kept
	}
	if reason, failed := refuseHarnessUnscopedDispatch(profile, adapter, source, fields); failed {
		return fail(reason)
	}
	for _, field := range adapter.absent {
		if _, present := fields[field]; present {
			return fail(fmt.Sprintf("canonical source `%s` requires an absent adapter field", source.path))
		}
	}
	route, reason, failed := harnessRoute(adapter.route, source.path)
	if failed {
		return fail(reason)
	}
	if adapter.format == "front-matter" {
		return renderHarnessFrontMatter(fields, route)
	}
	if reason, failed := harnessAddScalar(fields, adapter.routeField, route); failed {
		return fail(reason)
	}
	return renderHarnessTOML(fields)
}

func sortedHarnessKeys(values map[string]map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func harnessAddScalar(fields map[string]*harnessRenderField, field, value string) (string, bool) {
	kept := value
	return harnessAddField(fields, field, &harnessRenderField{scalar: &kept})
}

// harnessAddField places one whole native field, refusing a second writer
// of the same field.
func harnessAddField(fields map[string]*harnessRenderField, field string, value *harnessRenderField) (string, bool) {
	if _, present := fields[field]; present {
		return fmt.Sprintf("adapter output repeats field `%s`", field), true
	}
	fields[field] = value
	return "", false
}

func harnessAddTranslation(fields map[string]*harnessRenderField, translation *harnessTranslation) (string, bool) {
	fail := func(reason string) (string, bool) { return reason, true }
	if len(translation.absentMembers) != 0 {
		return fail(fmt.Sprintf("adapter output cannot add absent members at `%s`", translation.field))
	}
	if len(translation.members) != 0 {
		entry, present := fields[translation.field]
		if !present {
			fields[translation.field] = &harnessRenderField{members: append([]string(nil), translation.members...), isMember: true}
			return "", false
		}
		if !entry.isMember {
			return fail(fmt.Sprintf("adapter output conflicts at `%s`", translation.field))
		}
		entry.members = append(entry.members, translation.members...)
		return "", false
	}
	entry, present := fields[translation.field]
	if !present {
		fields[translation.field] = &harnessRenderField{entries: harnessScalarEntries(translation.entries), isEntry: true}
		return "", false
	}
	if !entry.isEntry {
		return fail(fmt.Sprintf("adapter output conflicts at `%s`", translation.field))
	}
	for name, value := range harnessScalarEntries(translation.entries) {
		entry.entries[name] = value
	}
	return "", false
}

func harnessScalarEntries(entries map[string]string) map[string]harnessEntryValue {
	decoded := make(map[string]harnessEntryValue, len(entries))
	for name, value := range entries {
		kept := value
		decoded[name] = harnessEntryValue{scalar: &kept}
	}
	return decoded
}

// A member-template dispatch format `<P>({names})` declares `<P>` as the
// profile's spawn tool. A rendered agent whose dispatch field still holds a
// bare `<P>` may spawn any agent, whether or not it also carries a scoped
// list, so it is refused before anything is written. An allow map, or a
// template without that exact suffix, declares no spawn tool.
func refuseHarnessUnscopedDispatch(profile *harnessProfile, adapter *harnessAdapter, source *harnessSource, fields map[string]*harnessRenderField) (string, bool) {
	if adapter.dispatches == nil {
		return "", false
	}
	tool, found := strings.CutSuffix(adapter.dispatches.format, "("+harnessNamesPlaceholder+")")
	if !found {
		return "", false
	}
	if entry, present := fields[adapter.dispatches.field]; present && entry.isMember {
		for _, member := range entry.members {
			if member == tool {
				return fmt.Sprintf("harness-dispatch-unscoped: profile `%s` renders an unscoped `%s` for canonical agent `%s`",
					profile.id, tool, source.path), true
			}
		}
	}
	return "", false
}

// Render a canonical dispatch list into its declared native field.
//
// A member template adds one member to a member list, after whatever the
// translations placed there. An allow map denies every agent and then allows
// each named one, either as the whole field or as one key of a map field.
func harnessAddDispatches(fields map[string]*harnessRenderField, dispatches *harnessDispatches, names []string) (string, bool) {
	fail := func(reason string) (string, bool) { return reason, true }
	if dispatches.format != harnessAllowMap {
		member := strings.ReplaceAll(dispatches.format, harnessNamesPlaceholder, strings.Join(names, ", "))
		entry, present := fields[dispatches.field]
		if !present {
			fields[dispatches.field] = &harnessRenderField{members: []string{}, isMember: true}
			entry = fields[dispatches.field]
		}
		if !entry.isMember {
			return fail(fmt.Sprintf("adapter output conflicts at `%s`", dispatches.field))
		}
		entry.members = append(entry.members, member)
		return "", false
	}
	allowed := map[string]string{"*": "deny"}
	for _, name := range names {
		allowed[name] = "allow"
	}
	parent, key, found := strings.Cut(dispatches.field, ".")
	if !found {
		return harnessAddField(fields, dispatches.field, &harnessRenderField{entries: harnessScalarEntries(allowed), isEntry: true})
	}
	entry, present := fields[parent]
	if !present {
		fields[parent] = &harnessRenderField{entries: map[string]harnessEntryValue{}, isEntry: true}
		entry = fields[parent]
	}
	if !entry.isEntry {
		return fail(fmt.Sprintf("adapter output conflicts at `%s`", dispatches.field))
	}
	if _, present := entry.entries[key]; present {
		return fail(fmt.Sprintf("adapter output conflicts at `%s`", dispatches.field))
	}
	entry.entries[key] = harnessEntryValue{nested: allowed}
	return "", false
}

func harnessAddFile(desired map[string]string, path, contents string) (string, bool) {
	clean, ok := normalHarnessPath(path)
	if !ok {
		return fmt.Sprintf("generated adapter path `%s` is invalid", path), true
	}
	if _, present := desired[clean]; present {
		return fmt.Sprintf("generated adapter path `%s` is duplicated", clean), true
	}
	desired[clean] = contents
	return "", false
}

func harnessRoute(template, path string) (string, string, bool) {
	if strings.Count(template, "{path}") != 1 {
		return "", "adapter route does not declare exactly one `{path}` placeholder", true
	}
	return strings.ReplaceAll(template, "{path}", path), "", false
}

// harnessCanonicalValue is one front-matter field: a scalar, or a list in
// the order the canonical source wrote it.
type harnessCanonicalValue struct {
	scalar string
	list   []string
	isList bool
}

func harnessParseMetadata(path, contents string, shape *harnessCanonicalAgent) (*harnessMetadata, string, bool) {
	fail := func(reason string) (*harnessMetadata, string, bool) { return nil, reason, true }
	fields, reason, failed := harnessParseFrontMatter(path, contents)
	if failed {
		return fail(reason)
	}
	metadata := &harnessMetadata{
		grants:      map[string]bool{},
		denials:     map[string]bool{},
		constraints: map[string]bool{},
		lists:       map[string][]string{},
	}
	name, reason, failed := harnessRequiredScalar(fields, shape.name, path)
	if failed {
		return fail(reason)
	}
	metadata.name = name
	description, reason, failed := harnessRequiredScalar(fields, shape.description, path)
	if failed {
		return fail(reason)
	}
	metadata.description = description
	if shape.tier != nil {
		tier, reason, failed := harnessOptionalScalar(fields, *shape.tier, path)
		if failed {
			return fail(reason)
		}
		metadata.tier = tier
	}
	for _, axis := range []struct {
		field    string
		declared bool
		target   map[string]bool
	}{
		{shape.grants, shape.hasGrants, metadata.grants},
		{shape.denials, shape.hasDenials, metadata.denials},
		{shape.constraints, shape.hasConstraints, metadata.constraints},
	} {
		if !axis.declared {
			continue
		}
		values, reason, failed := harnessOrderedList(fields, axis.field, path)
		if failed {
			return fail(reason)
		}
		for _, value := range values {
			axis.target[value] = true
		}
	}
	if shape.dispatches != nil {
		dispatches, reason, failed := harnessOrderedList(fields, *shape.dispatches, path)
		if failed {
			return fail(reason)
		}
		metadata.dispatches = dispatches
	}
	for name, value := range fields {
		if value.isList {
			metadata.lists[name] = append([]string(nil), value.list...)
		}
	}
	return metadata, "", false
}

func harnessParseFrontMatter(path, contents string) (map[string]harnessCanonicalValue, string, bool) {
	fail := func(reason string) (map[string]harnessCanonicalValue, string, bool) {
		return nil, reason, true
	}
	lines := harnessLines(contents)
	if len(lines) == 0 || lines[0] != "---" {
		return fail(fmt.Sprintf("canonical source `%s` has no leading front matter", path))
	}
	close := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRightFunc(lines[i], unicode.IsSpace) == "---" {
			close = i
			break
		}
	}
	if close < 0 {
		return fail(fmt.Sprintf("canonical source `%s` has unterminated front matter", path))
	}
	header := lines[1:close]
	fields := map[string]harnessCanonicalValue{}
	index := 0
	for index < len(header) {
		line := header[index]
		if strings.TrimSpace(line) == "" {
			index++
			continue
		}
		if harnessStartsWhitespace(line) {
			return fail(fmt.Sprintf("canonical source `%s` has malformed front matter", path))
		}
		key, raw, found := strings.Cut(line, ":")
		if !found {
			return fail(fmt.Sprintf("canonical source `%s` has malformed front matter", path))
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return fail(fmt.Sprintf("canonical source `%s` repeats or omits a front matter key", path))
		}
		if _, present := fields[key]; present {
			return fail(fmt.Sprintf("canonical source `%s` repeats or omits a front matter key", path))
		}
		raw = strings.TrimSpace(raw)
		index++
		var value harnessCanonicalValue
		switch raw {
		case ">", ">-", "|", "|-":
			var parts []string
			for index < len(header) && harnessStartsWhitespace(header[index]) {
				parts = append(parts, strings.TrimSpace(header[index]))
				index++
			}
			value = harnessCanonicalValue{scalar: strings.TrimSpace(strings.Join(parts, " "))}
		default:
			if raw == "" {
				var members []string
				for index < len(header) && harnessStartsWhitespace(header[index]) {
					member := strings.TrimSpace(header[index])
					bare, found := strings.CutPrefix(member, "- ")
					if !found {
						return fail(fmt.Sprintf("canonical source `%s` has a non-list metadata value", path))
					}
					unquoted := harnessUnquote(bare)
					if strings.TrimSpace(member) == "" || containsHarnessString(members, unquoted) {
						return fail(fmt.Sprintf("canonical source `%s` has an invalid metadata list", path))
					}
					members = append(members, unquoted)
					index++
				}
				value = harnessCanonicalValue{list: members, isList: true}
			} else {
				value = harnessCanonicalValue{scalar: harnessUnquote(raw)}
			}
		}
		fields[key] = value
	}
	return fields, "", false
}

// harnessLine splits text the way the oracle's str::lines does: a trailing
// newline ends the last line without adding another, and a trailing
// carriage return never survives on one.
func harnessLines(text string) []string {
	raw := strings.Split(text, "\n")
	if len(raw) != 0 && raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1]
	}
	for i, line := range raw {
		raw[i] = strings.TrimSuffix(line, "\r")
	}
	return raw
}

// harnessStartsWhitespace reports leading whitespace the way the oracle's
// char::is_whitespace does: Unicode White Space, which Go spells the same.
func harnessStartsWhitespace(line string) bool {
	for _, first := range line {
		return unicode.IsSpace(first)
	}
	return false
}

func harnessUnquote(value string) string {
	if len(value) >= 2 {
		if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
			return value[1 : len(value)-1]
		}
		if strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`) {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func containsHarnessString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func harnessRequiredScalar(fields map[string]harnessCanonicalValue, field, path string) (string, string, bool) {
	value, reason, failed := harnessOptionalScalar(fields, field, path)
	if failed {
		return "", reason, true
	}
	if value == nil {
		return "", fmt.Sprintf("canonical source `%s` has no `%s` metadata field", path, field), true
	}
	return *value, "", false
}

func harnessOptionalScalar(fields map[string]harnessCanonicalValue, field, path string) (*string, string, bool) {
	value, present := fields[field]
	if !present {
		return nil, "", false
	}
	if value.isList || strings.TrimSpace(value.scalar) == "" {
		return nil, fmt.Sprintf("canonical source `%s` has a non-scalar `%s` metadata field", path, field), true
	}
	kept := value.scalar
	return &kept, "", false
}

// harnessOrderedList reads a declared list in the order the canonical
// source wrote it.
func harnessOrderedList(fields map[string]harnessCanonicalValue, field, path string) ([]string, string, bool) {
	value, present := fields[field]
	if !present {
		return nil, "", false
	}
	if !value.isList {
		return nil, fmt.Sprintf("canonical source `%s` has a non-list `%s` metadata field", path, field), true
	}
	return append([]string(nil), value.list...), "", false
}

func renderHarnessFrontMatter(fields map[string]*harnessRenderField, route string) (string, string, bool) {
	var out strings.Builder
	out.WriteString("---\n")
	for _, field := range sortedHarnessFieldNames(fields) {
		value := fields[field]
		switch {
		case value.scalar != nil:
			renderHarnessYAMLField(&out, field, *value.scalar, "")
		case value.isMember:
			renderHarnessYAMLField(&out, field, strings.Join(value.members, ", "), "")
		case value.isEntry:
			out.WriteString(field + ":\n")
			for _, name := range sortedHarnessEntryNames(value.entries) {
				entry := value.entries[name]
				switch {
				case entry.scalar != nil:
					renderHarnessYAMLField(&out, harnessYAMLItem(name), *entry.scalar, "  ")
				default:
					out.WriteString("  " + harnessYAMLItem(name) + ":\n")
					for _, nested := range sortedHarnessNestedNames(entry.nested) {
						renderHarnessYAMLField(&out, harnessYAMLItem(nested), entry.nested[nested], "    ")
					}
				}
			}
		case value.isSeq:
			out.WriteString(field + ":\n")
			for _, item := range value.sequence {
				out.WriteString("  - " + harnessYAMLItem(item) + "\n")
			}
		}
	}
	out.WriteString("---\n\n")
	out.WriteString(route)
	out.WriteString("\n")
	return out.String(), "", false
}

func sortedHarnessFieldNames(fields map[string]*harnessRenderField) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedHarnessEntryNames(entries map[string]harnessEntryValue) []string {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedHarnessNestedNames(nested map[string]string) []string {
	names := make([]string, 0, len(nested))
	for name := range nested {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func renderHarnessTOML(fields map[string]*harnessRenderField) (string, string, bool) {
	var out strings.Builder
	for _, field := range sortedHarnessFieldNames(fields) {
		value := fields[field]
		switch {
		case value.scalar != nil:
			out.WriteString(field + " = " + harnessTOMLScalar(*value.scalar) + "\n")
		case value.isMember:
			out.WriteString(field + " = " + harnessTOMLScalar(strings.Join(value.members, ", ")) + "\n")
		case value.isEntry:
			out.WriteString("[" + field + "]\n")
			// Every value before any nested table, because a TOML table
			// header ends the table whose values precede it.
			for _, name := range sortedHarnessEntryNames(value.entries) {
				if entry := value.entries[name]; entry.scalar != nil {
					out.WriteString(harnessTOMLKey(name) + " = " + harnessTOMLScalar(*entry.scalar) + "\n")
				}
			}
			for _, name := range sortedHarnessEntryNames(value.entries) {
				if entry := value.entries[name]; entry.nested != nil {
					out.WriteString("[" + field + "." + harnessTOMLKey(name) + "]\n")
					for _, nested := range sortedHarnessNestedNames(entry.nested) {
						out.WriteString(harnessTOMLKey(nested) + " = " + harnessTOMLScalar(entry.nested[nested]) + "\n")
					}
				}
			}
		case value.isSeq:
			out.WriteString(field + " = " + harnessTOMLArray(value.sequence) + "\n")
		}
	}
	return out.String(), "", false
}

func renderHarnessYAMLField(out *strings.Builder, field, value, indentation string) {
	if scalar, ok := harnessYAMLScalarOf(value); ok {
		out.WriteString(indentation + field + ": " + scalar + "\n")
		return
	}
	out.WriteString(indentation + field + ": |-\n")
	for _, line := range harnessLines(value) {
		out.WriteString(indentation + "  " + line + "\n")
	}
}

func harnessYAMLScalarOf(value string) (string, bool) {
	if value == "" || strings.HasPrefix(value, "-") {
		return "", false
	}
	for _, character := range value {
		if !(character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == ' ' || character == '-' || character == '_' || character == '.') {
			return "", false
		}
	}
	return value, true
}

// One block-sequence item: plain when YAML reads it back unchanged,
// otherwise a double-quoted scalar, whose JSON spelling YAML also accepts.
func harnessYAMLItem(value string) string {
	if plain, ok := harnessYAMLScalarOf(value); ok {
		return plain
	}
	return domain.Quote(value)
}

// A key as written: bare when TOML reads it back as written, otherwise
// quoted, as a `*` entry must be.
func harnessTOMLKey(key string) string {
	if key == "" {
		return harnessTOMLScalar(key)
	}
	for _, character := range key {
		if !(character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' || character == '.') {
			return harnessTOMLScalar(key)
		}
	}
	return key
}

func harnessTOMLScalar(value string) string {
	return harnessTOMLString(value)
}

// harnessTOMLString renders one TOML string the way the oracle's TOML
// writer does: a pretty basic string when nothing needs escaping, a literal
// string when only quotes or backslashes would, a pretty multi-line string
// when only double quotes stand in the way, a multi-line literal when only
// single quotes do, and an escaped fallback when control codes force one.
// The order matters: each form is the first the writer accepts.
func harnessTOMLString(value string) string {
	bytes := []byte(value)
	escape, escapeCodes, newline := false, false, false
	maxSingle, maxDouble := 0, 0
	runSingle, runDouble := 0, 0
	for _, byte := range bytes {
		if byte == '\'' {
			runSingle++
			if maxSingle < runSingle {
				maxSingle = runSingle
			}
		} else {
			runSingle = 0
		}
		if byte == '"' {
			runDouble++
			if maxDouble < runDouble {
				maxDouble = runDouble
			}
		} else {
			runDouble = 0
		}
		switch {
		case byte == '\\':
			escape = true
		case byte == '\t':
			// A tab is always allowed; it stays neutral here.
		case byte == '\n':
			newline = true
		case byte <= 0x1f || byte == 0x7f:
			escapeCodes = true
		}
	}
	switch {
	case !escapeCodes && !escape && maxDouble == 0 && !newline:
		return harnessTOMLBasic(value, false, false)
	case !escapeCodes && maxSingle == 0 && !newline:
		return "'" + value + "'"
	case !escapeCodes && !escape && maxDouble <= 2:
		return harnessTOMLBasic(value, true, newline)
	case !escapeCodes && maxSingle <= 2:
		if newline {
			return "'''\n" + value + "'''"
		}
		return "'''" + value + "'''"
	default:
		return harnessTOMLBasic(value, newline, newline)
	}
}

// harnessTOMLEscape reports the short escape for one byte that always
// escapes in its form, or false when the byte passes through or stops the
// run for a longer encoding.
func harnessTOMLEscape(byte byte, multiLine bool) (string, bool) {
	switch byte {
	case 0x08:
		return "\\b", true
	case 0x09:
		return "\\t", true
	case 0x0a:
		if !multiLine {
			return "\\n", true
		}
	case 0x0c:
		return "\\f", true
	case 0x0d:
		return "\\r", true
	case 0x5c:
		return "\\\\", true
	}
	return "", false
}

// harnessTOMLBasic escapes one basic string, single- or multi-line. A
// multi-line string keeps newlines raw and tolerates up to two double quotes
// in a row; anything longer, and every other control, escapes.
func harnessTOMLBasic(value string, multiLine, newlinePrefix bool) string {
	var out strings.Builder
	allowedQuotes := 0
	if multiLine {
		out.WriteString("\"\"\"")
		if newlinePrefix {
			out.WriteByte('\n')
		}
		allowedQuotes = 2
	} else {
		out.WriteByte('"')
	}
	stream := []byte(value)
	for len(stream) != 0 {
		unescapedEnd := 0
		escaped := ""
		hasEscaped := false
		runQuotes := 0
		index := 0
		for index < len(stream) {
			byte := stream[index]
			if byte == '"' {
				runQuotes++
				if allowedQuotes < runQuotes {
					escaped, hasEscaped = "\\\"", true
					break
				}
			} else {
				runQuotes = 0
			}
			// A multi-line string keeps newlines raw; every shorter form
			// escapes them. Any other control stops the run for a \uXXXX.
			if escape, ok := harnessTOMLEscape(byte, multiLine); ok {
				escaped, hasEscaped = escape, true
				break
			}
			if (byte <= 0x1f || byte == 0x7f) && !(byte == 0x0a && multiLine) {
				break
			}
			unescapedEnd = index + 1
			index++
		}
		out.Write(stream[:unescapedEnd])
		out.WriteString(escaped)
		end := unescapedEnd
		if hasEscaped {
			end++
		}
		stream = stream[end:]
		if !hasEscaped && len(stream) != 0 {
			fmt.Fprintf(&out, "\\u%04X", stream[0])
			stream = stream[1:]
		}
	}
	if multiLine {
		out.WriteString("\"\"\"")
	} else {
		out.WriteByte('"')
	}
	return out.String()
}

func harnessTOMLArray(values []string) string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, harnessTOMLString(value))
	}
	return "[" + strings.Join(items, ", ") + "]"
}

// harnessCatalogJSON renders one family's catalog the way the oracle's
// pretty JSON printer does: two-space indent, sorted keys, `": "` between
// every key and value.
func harnessCatalogJSON(profile string, sources []harnessSource) (string, string, bool) {
	var out strings.Builder
	out.WriteString("{\n")
	out.WriteString("  \"schemaVersion\": 1,\n")
	out.WriteString("  \"profile\": " + domain.Quote(profile) + ",\n")
	out.WriteString("  \"sources\": [")
	if len(sources) != 0 {
		out.WriteString("\n")
		for i, source := range sources {
			out.WriteString("    {\n")
			out.WriteString("      \"id\": " + domain.Quote(source.id) + ",\n")
			out.WriteString("      \"path\": " + domain.Quote(source.path) + ",\n")
			out.WriteString("      \"digest\": " + domain.Quote(source.digest) + "\n")
			if i+1 < len(sources) {
				out.WriteString("    },\n")
			} else {
				out.WriteString("    }\n")
			}
		}
		out.WriteString("  ]\n")
	} else {
		out.WriteString("]\n")
	}
	out.WriteString("}\n")
	return out.String(), "", false
}

func harnessProvenanceJSON(profile string, sources []harnessSource, resolved map[string]string) (string, string, bool) {
	catalog, _, _ := harnessCatalogJSON(profile, sources)
	body := strings.TrimSuffix(catalog, "}\n")
	var out strings.Builder
	out.WriteString(body)
	names := make([]string, 0, len(resolved))
	for name := range resolved {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		out.WriteString("}\n")
		return out.String(), "", false
	}
	// The pretty printer joins siblings: the sources array closes with a
	// comma before the recorded models open, not on its own line.
	out.Reset()
	out.WriteString(strings.TrimSuffix(body, "\n") + ",\n")
	out.WriteString("  \"resolvedModels\": {\n")
	for i, name := range names {
		out.WriteString("    " + domain.Quote(name) + ": " + domain.Quote(resolved[name]))
		if i+1 < len(names) {
			out.WriteString(",")
		}
		out.WriteString("\n")
	}
	out.WriteString("  }\n}\n")
	return out.String(), "", false
}

// Every declared marker-surface file that claims RHINO generation and that
// generation does not write, as `stale-marker` findings.
//
// A file generation owns -- one it writes, or any file under a family root
// or at an exact adapter path -- is never a stale marker: the adapter
// comparison already reports it.
func harnessStaleMarkers(decoded *harnessConfig, tree Tree, plan *harnessPlanResult, excluded []string, format Format) ([]string, Outcome, bool) {
	if len(decoded.markerSurfaces) == 0 {
		return nil, Outcome{}, false
	}
	patterns := decoded.markerSurfaces
	for _, pattern := range patterns {
		if _, err := Match(pattern, "probe"); err != nil {
			return nil, reportRefused("harness-adapters", format,
				"harness.marker-surfaces: "+err.Error()), true
		}
	}
	var reached []string
	for _, path := range tree.Files() {
		if harnessExcluded(path, excluded) {
			continue
		}
		if harnessGoverned(patterns, path) < 0 {
			continue
		}
		reached = append(reached, path)
	}
	sort.Strings(reached)
	var stale []string
	for _, path := range reached {
		if harnessGoverned(patterns, path) < 0 {
			continue
		}
		if _, present := plan.desired[path]; present {
			continue
		}
		if containsHarnessPath(plan.transaction.ExactPaths, path) {
			continue
		}
		under := false
		for _, root := range plan.transaction.Roots {
			if underHarnessRoot(path, root) {
				under = true
				break
			}
		}
		if under {
			continue
		}
		text, readErr := tree.Read(path)
		if readErr != nil {
			switch readErr.Kind {
			case "missing", "not-text":
				// Gone since the walk, or not text: neither can carry a marker.
				continue
			default:
				return nil, RefuseWith(format, domain.FileUnreadable,
					"[harness-adapters] "+path+": "+readErr.Reason), true
			}
		}
		for _, line := range strings.Split(text, "\n") {
			if strings.Contains(strings.ToLower(line), harnessGeneratedMarker) {
				stale = append(stale, path+": stale-marker")
				break
			}
		}
	}
	return stale, Outcome{}, false
}

// harnessGoverned reports the index of the last surface covering a path, or
// -1 when no declared surface covers it. The last match rather than the
// first: where two surfaces cover one file, the later declaration is the
// more specific intent.
func harnessGoverned(patterns []string, path string) int {
	governed := -1
	for i, pattern := range patterns {
		if ok, err := Match(pattern, path); err == nil && ok {
			governed = i
		}
	}
	return governed
}

// harnessExcluded reports whether any directory on the way to a file is one
// the repository excludes. The file's own name is never tested.
func harnessExcluded(path string, excluded []string) bool {
	segments := strings.Split(path, "/")
	for _, segment := range segments[:len(segments)-1] {
		for _, name := range excluded {
			if segment == name {
				return true
			}
		}
	}
	return false
}

// Compare every currently visible adapter against the complete desired
// model. It deliberately has no write port, so validation cannot repair
// drift.
func harnessDifferences(tree Tree, plan *harnessPlanResult) []string {
	var differences []string
	paths := make([]string, 0, len(plan.desired))
	for path := range plan.desired {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		desired := plan.desired[path]
		current, readErr := tree.Read(path)
		switch {
		case readErr == nil && current == desired:
		case readErr == nil:
			differences = append(differences, path+": divergent-adapter")
		case readErr.Kind == "missing":
			differences = append(differences, path+": missing-adapter")
		case readErr.Kind == "not-text":
			differences = append(differences, path+": non-text-adapter")
		default:
			differences = append(differences, path+": unreadable-adapter: "+readErr.Reason)
		}
	}
	for _, path := range tree.Files() {
		under := false
		for _, root := range plan.transaction.Roots {
			if underHarnessRoot(path, root) {
				under = true
				break
			}
		}
		if under {
			if _, present := plan.desired[path]; !present {
				differences = append(differences, path+": stale-adapter")
			}
		}
	}
	sort.Strings(differences)
	return differences
}

func harnessClean(format Format, message string) Outcome {
	if format == JSON {
		return Clean(fmt.Sprintf("{\"schemaVersion\":1,\"command\":\"harness-adapters\",\"status\":\"clean\",\"message\":%s}\n",
			domain.Quote(message)))
	}
	return Clean("[harness-adapters] " + message + "\n")
}

func harnessFindings(format Format, differences []string) Outcome {
	if format == JSON {
		quoted := make([]string, 0, len(differences))
		for _, difference := range differences {
			quoted = append(quoted, domain.Quote(difference))
		}
		return Outcome{ExitCode: 1, Stdout: fmt.Sprintf(
			"{\"schemaVersion\":1,\"command\":\"harness-adapters\",\"status\":\"findings\",\"findings\":[%s]}\n",
			strings.Join(quoted, ","))}
	}
	var out strings.Builder
	for _, difference := range differences {
		out.WriteString("[harness-adapters] " + difference + "\n")
	}
	return Outcome{ExitCode: 1, Stderr: out.String()}
}

func harnessRefused(format Format, reason string) Outcome {
	return RefuseWith(format, domain.HarnessRefused, "[harness-adapters] "+reason)
}
