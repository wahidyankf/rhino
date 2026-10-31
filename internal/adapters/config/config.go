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

// GateInput is one declared typed input with its portable kind: files,
// regular-files, commit-message, commit-range, or repository-state.
type GateInput struct {
	Kind string
}

// GateArg is one argv item: either a literal or a resolved input field with
// its closed expansion mode, single or repeat. A malformed item is kept as
// decoded so dispatch refuses it with the argv shape message.
type GateArg struct {
	Literal    string
	HasLiteral bool
	Input      string
	HasInput   bool
	Expand     string
}

// GateEnv is one environment variable, mapped to one declared input
// field. Values never come from inherited process environment or a shell
// template.
type GateEnv struct {
	Input string
}

// GateBinding is how one surface supplies one declared input: the generic
// source, the explicit range selector where the source needs one, and the
// fallback ref a push-update range resolves for a new ref.
type GateBinding struct {
	Source   string
	Range    string
	Fallback *string
}

// GateRunOn is one surface membership: the at-least reason and the input
// binds the surface supplies.
type GateRunOn struct {
	Reason *string
	Bind   map[string]GateBinding
}

// Gate is one declared lifecycle gate with its typed inputs, launchable
// command, and per-surface binds. The gate run resolves every input at
// dispatch and projects the command from the resolved values.
type Gate struct {
	ID          string
	Kind        string
	Inputs      map[string]GateInput
	RunOn       map[string]GateRunOn
	Executable  string
	Args        []GateArg
	Environment map[string]GateEnv
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
	if err := strictGateShapes(text); err != nil {
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
		if kind, ok := mapping["type"].(string); ok {
			gate.Kind = kind
		}
		if inputs, ok := mapping["inputs"].(map[string]any); ok {
			gate.Inputs = decodeGateInputs(inputs)
		}
		if runOn, ok := mapping["run-on"].(map[string]any); ok {
			if _, ok := runOn["ci"]; ok {
				return fmt.Errorf("%s: run-on surface `ci` is retired; declare a lifecycle surface", Path)
			}
			gate.RunOn = decodeGateRunOn(runOn)
			if err := validatePushFallbacks(gate); err != nil {
				return err
			}
		}
		if command, ok := mapping["command"].(map[string]any); ok {
			executable, _ := command["executable"].(string)
			gate.Executable = executable
			if args, ok := command["args"].([]any); ok {
				gate.Args = decodeGateArgs(args)
			}
			if environment, ok := command["environment"].(map[string]any); ok {
				gate.Environment = decodeGateEnv(environment)
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
				if reason := gate.RunOn[surface].Reason; reason != nil && strings.TrimSpace(*reason) != "" {
					reasoned[gate.ID] = true
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

// decodeGateInputs projects the declared typed inputs. The kinds stay raw:
// dispatch resolves only what the run-on binds supply.
func decodeGateInputs(raw map[string]any) map[string]GateInput {
	out := make(map[string]GateInput, len(raw))
	for name, item := range raw {
		mapping, _ := item.(map[string]any)
		kind, _ := mapping["kind"].(string)
		out[name] = GateInput{Kind: kind}
	}
	return out
}

// decodeGateArgs projects argv items: a `{literal: ...}` mapping is a
// literal, and an `{input, expand}` mapping is a typed projection.
// Anything else, including a plain string, is kept malformed so dispatch
// refuses it with the argv shape message: the oracle accepts only the
// mapping form.
func decodeGateArgs(raw []any) []GateArg {
	out := make([]GateArg, 0, len(raw))
	for _, item := range raw {
		if _, ok := item.(string); ok {
			out = append(out, GateArg{})
			continue
		}
		mapping, ok := item.(map[string]any)
		if !ok {
			out = append(out, GateArg{})
			continue
		}
		var arg GateArg
		if literal, ok := mapping["literal"].(string); ok {
			arg.Literal, arg.HasLiteral = literal, true
		} else if _, present := mapping["literal"]; present {
			arg.HasLiteral = true
		}
		if input, ok := mapping["input"].(string); ok {
			arg.Input, arg.HasInput = input, true
		} else if _, present := mapping["input"]; present {
			arg.HasInput = true
		}
		if expand, ok := mapping["expand"].(string); ok {
			arg.Expand = expand
		} else if _, present := mapping["expand"]; present {
			arg.Expand = "\x00invalid"
		}
		out = append(out, arg)
	}
	return out
}

// decodeGateEnv projects environment entries: an `{input: ...}` mapping
// is a projection. Anything else is kept with an empty reference so the
// strict shape check refuses it first; dispatch never sees a literal.
func decodeGateEnv(raw map[string]any) map[string]GateEnv {
	out := make(map[string]GateEnv, len(raw))
	for name, item := range raw {
		mapping, _ := item.(map[string]any)
		input, _ := mapping["input"].(string)
		out[name] = GateEnv{Input: input}
	}
	return out
}

// decodeGateRunOn projects the per-surface membership: the at-least reason
// and the input binds the surface supplies.
func decodeGateRunOn(raw map[string]any) map[string]GateRunOn {
	out := make(map[string]GateRunOn, len(raw))
	for surface, item := range raw {
		mapping, _ := item.(map[string]any)
		entry := GateRunOn{}
		if reason, ok := mapping["reason"].(string); ok {
			entry.Reason = &reason
		}
		if binds, ok := mapping["bind"].(map[string]any); ok {
			entry.Bind = make(map[string]GateBinding, len(binds))
			for name, rawBind := range binds {
				bindMapping, _ := rawBind.(map[string]any)
				bind := GateBinding{}
				bind.Source, _ = bindMapping["source"].(string)
				bind.Range, _ = bindMapping["range"].(string)
				if fallback, ok := bindMapping["fallback"].(string); ok {
					bind.Fallback = &fallback
				}
				entry.Bind[name] = bind
			}
		}
		out[surface] = entry
	}
	return out
}

// validatePushFallbacks refuses push-update binds without a declared
// fallback, with an invalid fallback, or with a fallback on another
// source, exactly like v0.15.0 config validation.
func validatePushFallbacks(gate Gate) error {
	surfaces := make([]string, 0, len(gate.RunOn))
	for surface := range gate.RunOn {
		surfaces = append(surfaces, surface)
	}
	sort.Slice(surfaces, func(left, right int) bool {
		return surfaceOrder(surfaces[left]) < surfaceOrder(surfaces[right])
	})
	for _, surface := range surfaces {
		membership := gate.RunOn[surface]
		names := make([]string, 0, len(membership.Bind))
		for name := range membership.Bind {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			binding := membership.Bind[name]
			if binding.Fallback != nil && binding.Source != "push-updates" {
				return fmt.Errorf("%s: line 1: %s: uses `fallback` with a source other than `push-updates`", Path, name)
			}
			if binding.Source != "push-updates" {
				continue
			}
			if binding.Fallback == nil {
				return fmt.Errorf("%s: line 1: %s: uses `push-updates` without a declared fallback ref", Path, name)
			}
			fallback := strings.TrimSpace(*binding.Fallback)
			if fallback == "" || strings.HasPrefix(fallback, "-") || containsWhitespace(fallback) {
				return fmt.Errorf("%s: line 1: %s: declares an invalid push-update fallback ref", Path, name)
			}
		}
	}
	return nil
}

// surfaceOrder matches the oracle's Surface enum order for deterministic
// validation.
func surfaceOrder(surface string) int {
	switch surface {
	case "pre-commit":
		return 0
	case "commit-msg":
		return 1
	case "pre-push":
		return 2
	case "pull-request":
		return 3
	case "main":
		return 4
	case "scheduled":
		return 5
	case "manual":
		return 6
	default:
		return 7
	}
}

// containsWhitespace reports whether the fallback holds any whitespace the
// oracle rejects, using Unicode space like Rust's `char::is_whitespace`.
func containsWhitespace(value string) bool {
	for _, character := range value {
		if character == ' ' || character == '\t' || character == '\n' || character == '\r' {
			return true
		}
		if character < 0x20 || character == 0x7f {
			return true
		}
		if character == 0x0b || character == 0x0c || character == 0x85 || character == 0xa0 {
			return true
		}
	}
	return false
}

// strictGateShapes refuses argv and environment shapes the oracle's serde
// model refuses before its semantic validation: a plain scalar or sequence
// where a struct belongs, an unknown field inside one, or an environment
// mapping missing its `input`. The wording and the line/column match the
// v0.15.0 malformed errors byte for byte.
func strictGateShapes(text string) error {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(text), &root); err != nil {
		return nil
	}
	gates := gateEntriesNodes(&root)
	for index, entry := range gates {
		command := mappingValue(entry, "command")
		if command == nil || command.Kind != yaml.MappingNode {
			continue
		}
		if args := mappingValue(command, "args"); args != nil && args.Kind == yaml.SequenceNode {
			for position := 0; position < len(args.Content); position++ {
				item := args.Content[position]
				path := fmt.Sprintf("gates.entries[%d].command.args[%d]", index, position)
				if item.Kind != yaml.MappingNode {
					return fmt.Errorf("%s: %s: %s at line %d column %d", Path, path, serdeTypeName(item, "Argument"), item.Line, item.Column)
				}
				for field := 0; field < len(item.Content); field += 2 {
					key := item.Content[field]
					name := key.Value
					if name != "literal" && name != "input" && name != "expand" {
						return fmt.Errorf("%s: %s: unknown field `%s`, expected one of `literal`, `input`, `expand` at line %d column %d", Path, path, name, key.Line, key.Column)
					}
				}
			}
		}
		if environment := mappingValue(command, "environment"); environment != nil && environment.Kind == yaml.MappingNode {
			for field := 0; field < len(environment.Content); field += 2 {
				nameNode := environment.Content[field]
				valueNode := environment.Content[field+1]
				name := nameNode.Value
				path := fmt.Sprintf("gates.entries[%d].command.environment.%s", index, name)
				if valueNode.Kind != yaml.MappingNode {
					return fmt.Errorf("%s: %s: %s at line %d column %d", Path, path, serdeTypeName(valueNode, "EnvironmentProjection"), valueNode.Line, valueNode.Column)
				}
				seenInput := false
				for inner := 0; inner < len(valueNode.Content); inner += 2 {
					key := valueNode.Content[inner]
					if key.Value == "input" {
						seenInput = true
						continue
					}
					return fmt.Errorf("%s: %s: unknown field `%s`, expected `input` at line %d column %d", Path, path, key.Value, key.Line, key.Column)
				}
				if !seenInput {
					return fmt.Errorf("%s: %s: missing field `input` at line %d column %d", Path, path, valueNode.Line, valueNode.Column)
				}
			}
		}
	}
	return nil
}

// gateEntriesNodes locates the gates.entries sequence in document order.
func gateEntriesNodes(root *yaml.Node) []*yaml.Node {
	document := root
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		document = root.Content[0]
	}
	if document.Kind != yaml.MappingNode {
		return nil
	}
	gates := mappingValue(document, "gates")
	if gates == nil || gates.Kind != yaml.MappingNode {
		return nil
	}
	entries := mappingValue(gates, "entries")
	if entries == nil || entries.Kind != yaml.SequenceNode {
		return nil
	}
	return entries.Content
}

// mappingValue returns the value for one mapping key, or nil.
func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index < len(node.Content); index += 2 {
		if node.Content[index].Value == key {
			return node.Content[index+1]
		}
	}
	return nil
}

// serdeTypeName renders one non-struct YAML node the way serde_yaml names
// the mismatched type in its malformed errors.
func serdeTypeName(node *yaml.Node, structure string) string {
	switch node.Kind {
	case yaml.SequenceNode:
		return "invalid type: sequence, expected struct " + structure
	case yaml.MappingNode:
		return "invalid type: map, expected struct " + structure
	case yaml.AliasNode:
		return "invalid type: sequence, expected struct " + structure
	default:
		switch node.Tag {
		case "!!int":
			return fmt.Sprintf("invalid type: integer `%s`, expected struct %s", node.Value, structure)
		case "!!float":
			return fmt.Sprintf("invalid type: floating point `%s`, expected struct %s", node.Value, structure)
		case "!!bool":
			return fmt.Sprintf("invalid type: boolean `%s`, expected struct %s", node.Value, structure)
		case "!!null":
			return fmt.Sprintf("invalid type: unit value, expected struct %s", structure)
		default:
			return fmt.Sprintf("invalid type: string %s, expected struct %s", quoteSerdeString(node.Value), structure)
		}
	}
}

// quoteSerdeString renders a YAML string the way Rust's debug format does
// for serde's `string "..."` invalid-type message.
func quoteSerdeString(value string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, character := range value {
		switch character {
		case '"':
			out.WriteString("\\\"")
		case '\\':
			out.WriteString("\\\\")
		case '\n':
			out.WriteString("\\n")
		case '\r':
			out.WriteString("\\r")
		case '\t':
			out.WriteString("\\t")
		default:
			if character < ' ' || character == 0x7f {
				fmt.Fprintf(&out, "\\u{%x}", character)
			} else {
				out.WriteRune(character)
			}
		}
	}
	out.WriteByte('"')
	return out.String()
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
