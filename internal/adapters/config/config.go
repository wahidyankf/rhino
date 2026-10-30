// Package config reads repo-config.yml without defaults.
package config

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Path is the single configuration file this tool reads.
const Path = "repo-config.yml"

// Gate is one declared lifecycle gate with a launchable command subset.
type Gate struct {
	ID         string
	RunOn      map[string]any
	Executable string
	Args       []any
}

// Argv projects literal-only arguments. Anything else is refused so a
// caller never silently runs a narrower command than declared.
func (g Gate) Argv() ([]string, error) {
	argv := []string{g.Executable}
	for _, arg := range g.Args {
		switch value := arg.(type) {
		case string:
			argv = append(argv, value)
		case map[string]any:
			if len(value) == 1 {
				if literal, ok := value["literal"]; ok {
					text, ok := literal.(string)
					if !ok {
						return nil, fmt.Errorf("literal argument names no text")
					}
					argv = append(argv, text)
					continue
				}
			}
			return nil, fmt.Errorf("argument was not resolved for this surface")
		default:
			return nil, fmt.Errorf("argument was not resolved for this surface")
		}
	}
	return argv, nil
}

// Document is the parsed grouped v0.4 configuration.
type Document struct {
	raw      map[string]any
	policies map[string]any
	gates    []Gate
	hasGates bool
	relation map[string]string
}

var ownerPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// Parse validates the configuration text.
func Parse(text string) (*Document, error) {
	var raw map[string]any
	if err := yaml.Unmarshal([]byte(text), &raw); err != nil {
		return nil, fmt.Errorf("%s: %s", Path, strings.TrimSpace(err.Error()))
	}
	if raw == nil {
		raw = map[string]any{}
	}
	schema, _ := raw["schema"].(string)
	if strings.TrimSpace(schema) == "" {
		lowered := strings.ToLower(text)
		if strings.Contains(lowered, "rhino/repo-config/v1") || strings.Contains(lowered, "ose/repo-config") {
			return nil, fmt.Errorf("%s: this stable build no longer accepts predecessor configuration", Path)
		}
		return nil, fmt.Errorf("%s: line 1 declares no schema: expected `schema: rhino/repo-config/v2`", Path)
	}
	if schema != "rhino/repo-config/v2" {
		return nil, fmt.Errorf("%s: unrecognized schema `%s`: this stable build accepts only `rhino/repo-config/v2` and no longer accepts predecessor configuration", Path, schema)
	}
	known := map[string]bool{
		"schema": true, "repository": true, "scan": true, "harness": true,
		"policies": true, "environment": true, "toolchains": true, "gates": true, "extensions": true,
	}
	var unknown []string
	for key := range raw {
		if !known[key] {
			unknown = append(unknown, key)
		}
	}
	sort.Strings(unknown)
	if len(unknown) > 0 {
		return nil, fmt.Errorf("%s: unknown core key `%s`", Path, unknown[0])
	}
	if ext, ok := raw["extensions"].(map[string]any); ok {
		var bad []string
		for owner := range ext {
			if !ownerPattern.MatchString(owner) {
				bad = append(bad, owner)
			}
		}
		sort.Strings(bad)
		if len(bad) > 0 {
			return nil, fmt.Errorf("%s: extension owner `%s` is outside the portable namespace", Path, bad[0])
		}
	}
	document := &Document{raw: raw}
	if policies, ok := raw["policies"].(map[string]any); ok {
		document.policies = policies
	}
	if err := checkQualityCycleCeiling(raw); err != nil {
		return nil, err
	}
	if gatesRaw, ok := raw["gates"].(map[string]any); ok {
		document.hasGates = true
		if err := document.decodeGates(gatesRaw); err != nil {
			return nil, err
		}
	}
	return document, nil
}

func (d *Document) decodeGates(gatesRaw map[string]any) error {
	entries, _ := gatesRaw["entries"].([]any)
	seen := map[string]bool{}
	for _, item := range entries {
		mapping, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: gate entry names no id", Path)
		}
		id, _ := mapping["id"].(string)
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("%s: gate entry names no id", Path)
		}
		if seen[id] {
			return fmt.Errorf("%s: gate id `%s` is duplicated", Path, id)
		}
		seen[id] = true
		gate := Gate{ID: id}
		if runOn, ok := mapping["run-on"].(map[string]any); ok {
			gate.RunOn = runOn
			if _, ok := runOn["ci"]; ok {
				return fmt.Errorf("%s: run-on surface `ci` is retired; declare a lifecycle surface", Path)
			}
		}
		if command, ok := mapping["command"].(map[string]any); ok {
			executable, _ := command["executable"].(string)
			gate.Executable = executable
			if args, ok := command["args"].([]any); ok {
				gate.Args = args
			}
		}
		d.gates = append(d.gates, gate)
	}
	d.relation = map[string]string{}
	if composition, ok := gatesRaw["composition"].(map[string]any); ok {
		for surface, item := range composition {
			if mapping, ok := item.(map[string]any); ok {
				if relation, ok := mapping["relation"].(string); ok {
					d.relation[surface] = relation
				}
			}
		}
	}
	local := map[string]bool{}
	onPR := map[string]bool{}
	reasoned := map[string]bool{}
	for _, gate := range d.gates {
		if gate.RunOn == nil {
			continue
		}
		for surface := range gate.RunOn {
			local[gate.ID] = true
			if surface == "pull-request" {
				onPR[gate.ID] = true
				if membership, ok := gate.RunOn[surface].(map[string]any); ok {
					if reason, ok := membership["reason"].(string); ok && strings.TrimSpace(reason) != "" {
						reasoned[gate.ID] = true
					}
				}
			}
		}
	}
	relation, declared := d.relation["pull-request"]
	if len(local) > 0 && !declared {
		return fmt.Errorf("%s: must declare the pull-request relation when local or pull-request gates exist", Path)
	}
	switch relation {
	case "", "exact", "at-least":
	default:
		return fmt.Errorf("%s: pull-request relation `%s` is not exact or at-least", Path, relation)
	}
	if relation == "exact" {
		var missing []string
		for id := range local {
			if !onPR[id] {
				missing = append(missing, id)
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			return fmt.Errorf("%s: gate `%s` is missing from pull-request composition", Path, missing[0])
		}
	}
	if relation == "at-least" {
		for id := range local {
			if !onPR[id] {
				continue
			}
			onlyPR := true
			for _, gate := range d.gates {
				if gate.ID != id {
					continue
				}
				for surface := range gate.RunOn {
					if surface != "pull-request" {
						onlyPR = false
					}
				}
			}
			if onlyPR && !reasoned[id] {
				return fmt.Errorf("%s: gate `%s` is an extra pull-request gate and needs its own non-empty reason", Path, id)
			}
		}
	}
	return nil
}

// checkQualityCycleCeiling refuses a configured gate cycle ceiling outside
// 1 to 3 while the configuration is read, the way the oracle has since the
// key was added: a gate never runs more than three cycles, and neither does
// the default a gate run reads.
func checkQualityCycleCeiling(raw map[string]any) error {
	policies, _ := raw["policies"].(map[string]any)
	governance, _ := policies["governance"].(map[string]any)
	gates, _ := governance["quality-gates"].(map[string]any)
	defaults, _ := gates["defaults"].(map[string]any)
	ceiling, present := defaults["max-cycles"]
	if !present || ceiling == nil {
		return nil
	}
	cycles, ok := ceiling.(int)
	if !ok || cycles < 1 || cycles > 3 {
		return fmt.Errorf("%s: line 1: policies.governance.quality-gates.defaults.max-cycles: must be 1, 2, or 3; a gate never runs more than three cycles", Path)
	}
	return nil
}

// PolicyOwner reports whether the dotted owner is declared.
func (d *Document) PolicyOwner(owner string) bool {
	parts := strings.Split(owner, ".")
	if len(parts) != 3 || parts[0] != "policies" {
		return false
	}
	group, ok := d.policies[parts[1]].(map[string]any)
	if !ok {
		return false
	}
	_, ok = group[parts[2]]
	return ok
}

// Environment returns the raw top-level environment group. An absent or null
// group is undeclared; an explicitly empty mapping is a declared empty
// policy, which plans and zero-count operations accept.
func (d *Document) Environment() (map[string]any, bool) {
	raw, present := d.raw["environment"]
	if !present || raw == nil {
		return nil, false
	}
	mapping, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	return mapping, true
}

// Harness returns the raw top-level harness group. An absent or null group
// is undeclared; an explicitly empty mapping is a declared empty policy,
// which the planner refuses with its own reason.
func (d *Document) Harness() (map[string]any, bool) {
	raw, present := d.raw["harness"]
	if !present || raw == nil {
		return nil, false
	}
	mapping, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	return mapping, true
}

// Toolchains returns the raw top-level toolchains group. An absent or null
// group is undeclared; an explicitly empty mapping is a declared empty
// policy, which plans and zero-count operations accept.
func (d *Document) Toolchains() (map[string]any, bool) {
	raw, present := d.raw["toolchains"]
	if !present || raw == nil {
		return nil, false
	}
	mapping, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	return mapping, true
}

// PolicyGroup returns the raw mapping for a policy group.
func (d *Document) PolicyGroup(group, name string) (map[string]any, bool) {
	policies, ok := d.policies[group].(map[string]any)
	if !ok {
		return nil, false
	}
	mapping, ok := policies[name].(map[string]any)
	return mapping, ok
}

// ScanExcludeDirectories returns the directory names excluded from every
// tree walk. An omitted scan section excludes nothing; a malformed one is
// refused rather than read as an empty list.
func (d *Document) ScanExcludeDirectories() ([]string, error) {
	raw, present := d.raw["scan"]
	if !present {
		return nil, nil
	}
	section, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: scan.exclude-directories names no directory", Path)
	}
	for key := range section {
		if key != "exclude-directories" {
			return nil, fmt.Errorf("%s: scan names unknown key `%s`", Path, key)
		}
	}
	list, present := section["exclude-directories"]
	if !present {
		return nil, nil
	}
	items, ok := list.([]any)
	if !ok {
		return nil, fmt.Errorf("%s: scan.exclude-directories names no directory", Path)
	}
	var excluded []string
	for _, item := range items {
		name, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%s: scan.exclude-directories names no directory", Path)
		}
		excluded = append(excluded, name)
	}
	return excluded, nil
}

// HasGates reports whether the gates group is declared. An omitted group is
// refused, never read as an empty lifecycle.
func (d *Document) HasGates() bool { return d.hasGates }

// GateIDsOn lists gate ids running on one surface, in declaration order.
func (d *Document) GateIDsOn(surface string) []string {
	var ids []string
	for _, gate := range d.gates {
		if gate.RunOn == nil {
			continue
		}
		if _, ok := gate.RunOn[surface]; ok {
			ids = append(ids, gate.ID)
		}
	}
	return ids
}

// GatesOn returns the gates running on one surface.
func (d *Document) GatesOn(surface string) []Gate {
	var selected []Gate
	for _, gate := range d.gates {
		if gate.RunOn == nil {
			continue
		}
		if _, ok := gate.RunOn[surface]; ok {
			selected = append(selected, gate)
		}
	}
	return selected
}
