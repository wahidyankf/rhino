package config

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// The closed vocabularies below are the ones the oracle deserializes into
// enums. A value outside them is an unknown variant, reported before any
// binding rule is applied.
var (
	gateInputKinds   = []string{"files", "regular-files", "commit-message", "commit-range", "repository-state"}
	gateInputSources = []string{"git-index", "hook-message-file", "push-updates", "explicit-range", "checkout"}
	gateExpansions   = []string{"single", "repeat"}
	portableNameRe   = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	environmentRe    = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
)

// sourceAccepts is the oracle's source_accepts table: which generic source
// may supply which input kind.
func sourceAccepts(kind, source string) bool {
	switch kind {
	case "files":
		return source == "git-index" || source == "explicit-range" || source == "checkout"
	case "regular-files":
		return source == "git-index" || source == "explicit-range"
	case "commit-message":
		return source == "hook-message-file" || source == "explicit-range"
	case "commit-range":
		return source == "push-updates" || source == "explicit-range"
	case "repository-state":
		return source == "checkout"
	}
	return false
}

// inputHasField is the oracle's input_has_field: the projected fields each
// kind offers to argv and environment.
func inputHasField(kind, field string) bool {
	switch kind {
	case "files", "regular-files":
		return field == "paths"
	case "commit-message":
		return field == "text"
	case "commit-range":
		return field == "base" || field == "head"
	case "repository-state":
		return field == "root"
	}
	return false
}

// validateGateInputs applies the oracle's static typed-input rules to one gate,
// in the oracle's order, so a declaration is refused before any child is
// named. It runs after the gate's membership, inputs, and command are decoded.
func validateGateInputs(gate Gate) error {
	fail := func(subject, reason string) error {
		return fmt.Errorf("%s: line 1: %s: %s", Path, subject, reason)
	}
	inputNames := sortedKeys(gate.Inputs)
	for _, name := range inputNames {
		if !portableNameRe.MatchString(name) {
			return fail(name, "is not a portable input name; expected lowercase kebab-case")
		}
		if kind := gate.Inputs[name].Kind; !containsString(gateInputKinds, kind) {
			return fmt.Errorf("%s: line 1: %s: unknown variant `%s`, expected one of %s", Path, name, kind, quoteList(gateInputKinds))
		}
	}
	for _, surface := range sortedSurfaces(gate.RunOn) {
		membership := gate.RunOn[surface]
		for _, name := range sortedKeys(membership.Bind) {
			binding := membership.Bind[name]
			if _, declared := gate.Inputs[name]; !declared {
				return fail(name, fmt.Sprintf("binds an input not declared by `%s` at %s", gate.ID, surface))
			}
			if !containsString(gateInputSources, binding.Source) {
				return fmt.Errorf("%s: line 1: %s: unknown variant `%s`, expected one of %s", Path, name, binding.Source, quoteList(gateInputSources))
			}
		}
		for _, name := range inputNames {
			binding, bound := membership.Bind[name]
			if !bound {
				return fail(name, fmt.Sprintf("has no binding for `%s` at %s", gate.ID, surface))
			}
			kind := gate.Inputs[name].Kind
			if !sourceAccepts(kind, binding.Source) {
				return fail(name, fmt.Sprintf("declares `%s` but %s is not a compatible source", kind, binding.Source))
			}
			if binding.Range != "" && binding.Source != "explicit-range" {
				return fail(name, "uses `range` with a source other than `explicit-range`")
			}
			if binding.Source == "explicit-range" && binding.Range != "explicit" {
				return fail(name, "uses `explicit-range` without `range: explicit`")
			}
			if binding.Fallback != nil && binding.Source != "push-updates" {
				return fail(name, "uses `fallback` with a source other than `push-updates`")
			}
			if binding.Source == "push-updates" {
				if binding.Fallback == nil {
					return fail(name, "uses `push-updates` without a declared fallback ref")
				}
				fallback := strings.TrimSpace(*binding.Fallback)
				if fallback == "" || strings.HasPrefix(fallback, "-") || containsWhitespace(fallback) {
					return fail(name, "declares an invalid push-update fallback ref")
				}
			}
		}
	}
	return validateGateCommand(gate)
}

// validateGateCommand checks the typed argv and environment projections: every
// reference names a declared input and one of its fields, and every declared
// input is projected somewhere.
func validateGateCommand(gate Gate) error {
	fail := func(subject, reason string) error {
		return fmt.Errorf("%s: line 1: %s: %s", Path, subject, reason)
	}
	if gate.Executable == "" {
		if len(gate.Inputs) > 0 {
			return fail(gate.ID, "declares typed inputs but no typed executable and argv command")
		}
		return nil
	}
	if strings.TrimSpace(gate.Executable) == "" || strings.ContainsAny(gate.Executable, "\n\r") {
		return fail(gate.ID, "has an empty or line-breaking executable")
	}
	projected := map[string]bool{}
	for _, arg := range gate.Args {
		switch {
		case arg.HasLiteral && !arg.HasInput && arg.Expand == "":
		case arg.HasLiteral && !arg.HasInput:
			return fail("args", "adds `expand` to a literal argv item")
		case arg.HasInput && !arg.HasLiteral && arg.Expand == "":
			return fail("args", "omits the closed expansion mode from an input argv item")
		case arg.HasInput && !arg.HasLiteral:
			if !containsString(gateExpansions, arg.Expand) {
				return fmt.Errorf("%s: line 1: %s: unknown variant `%s`, expected one of %s", Path, gate.ID, arg.Expand, quoteList(gateExpansions))
			}
			name, kind, field, err := resolveGateReference(gate, arg.Input)
			if err != nil {
				return err
			}
			if arg.Expand == "repeat" && !((kind == "files" || kind == "regular-files") && field == "paths") {
				return fail(arg.Input, "uses `expand: repeat` for a field that is not a file-path sequence")
			}
			projected[name] = true
		default:
			return fail("args", "must hold exactly one literal or typed input argv item")
		}
	}
	for _, name := range sortedKeys(gate.Environment) {
		if !environmentRe.MatchString(name) {
			return fail(name, "is not an uppercase environment projection name")
		}
		input, _, _, err := resolveGateReference(gate, gate.Environment[name].Input)
		if err != nil {
			return err
		}
		projected[input] = true
	}
	for _, name := range sortedKeys(gate.Inputs) {
		if !projected[name] {
			return fail(name, "is declared but never projected through typed argv or environment")
		}
	}
	return nil
}

// resolveGateReference splits an `input.field` reference and checks both
// halves against the gate's declared inputs.
func resolveGateReference(gate Gate, reference string) (string, string, string, error) {
	name, field, found := strings.Cut(reference, ".")
	if !found || name == "" || field == "" || strings.Contains(field, ".") {
		return "", "", "", fmt.Errorf("%s: line 1: %s: is not an `input.field` reference", Path, reference)
	}
	input, declared := gate.Inputs[name]
	if !declared {
		return "", "", "", fmt.Errorf("%s: line 1: %s: names an input this gate does not declare", Path, reference)
	}
	if !inputHasField(input.Kind, field) {
		return "", "", "", fmt.Errorf("%s: line 1: %s: does not name a field available on a %s input", Path, reference, input.Kind)
	}
	return name, input.Kind, field, nil
}

// sortedSurfaces lists a gate's surfaces in lifecycle order, so composition and
// validation visit them the same way.
func sortedSurfaces(runOn map[string]GateRunOn) []string {
	surfaces := make([]string, 0, len(runOn))
	for surface := range runOn {
		surfaces = append(surfaces, surface)
	}
	sort.Slice(surfaces, func(left, right int) bool {
		return surfaceOrder(surfaces[left]) < surfaceOrder(surfaces[right])
	})
	return surfaces
}

// sortedKeys returns a map's keys in lexical order, so validation is
// deterministic. It serves the input, bind, and environment maps alike.
func sortedKeys[V any](items map[string]V) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// quoteList renders a closed vocabulary the way serde lists expected variants.
func quoteList(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = "`" + value + "`"
	}
	return strings.Join(quoted, ", ")
}
