package config

import (
	"fmt"
	"strings"
)

// LocalSurfaces are the surfaces a repository runs in its own hooks, before a
// change reaches a remote. Wiring rule W07 reads each one's allowed list.
var LocalSurfaces = []string{"pre-commit", "commit-msg", "pre-push"}

// Wiring is the declared gates.wiring section: where the hook files and the
// workflow files live, the reason a repository without workflows gives, and the
// allowed lists W07 reads for each local surface a repository declares.
type Wiring struct {
	HookDirectory     string
	Workflows         bool
	WorkflowDirectory string
	NoWorkflowsReason string
	LocalAllowed      map[string][]string
}

// Wiring returns the declared gates.wiring section. An omitted section is
// reported as undeclared; a malformed one is an error that names the field.
func (d *Document) Wiring() (*Wiring, bool, error) {
	gates, _ := d.raw["gates"].(map[string]any)
	raw, present := gates["wiring"]
	if !present || raw == nil {
		return nil, false, nil
	}
	section, ok := raw.(map[string]any)
	if !ok {
		return nil, true, fmt.Errorf("%s: gates.wiring must be a mapping", Path)
	}
	if unknown := unknownKeys(section, "hooks", "ci", "local-surfaces"); unknown != "" {
		return nil, true, fmt.Errorf("%s: gates.wiring names unknown key `%s`", Path, unknown)
	}
	hooks, _ := section["hooks"].(map[string]any)
	hookDirectory, ok := repositoryDirectory(hooks, "directory")
	if !ok {
		return nil, true, fmt.Errorf("%s: gates.wiring.hooks.directory names no repository directory", Path)
	}
	wiring := &Wiring{HookDirectory: hookDirectory, LocalAllowed: map[string][]string{}}
	if err := decodeCI(wiring, section["ci"]); err != nil {
		return nil, true, err
	}
	if err := decodeLocalSurfaces(wiring, section["local-surfaces"]); err != nil {
		return nil, true, err
	}
	return wiring, true, nil
}

// decodeCI reads the ci section: either a repository-relative workflow
// directory, or a reason that the repository hosts no workflows. A repository
// names one of the two, never both.
func decodeCI(wiring *Wiring, raw any) error {
	ci, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("%s: gates.wiring.ci declares neither directory nor none", Path)
	}
	if unknown := unknownKeys(ci, "directory", "none"); unknown != "" {
		return fmt.Errorf("%s: gates.wiring.ci names unknown key `%s`", Path, unknown)
	}
	_, hasDirectory := ci["directory"]
	_, hasNone := ci["none"]
	switch {
	case hasDirectory && hasNone:
		return fmt.Errorf("%s: gates.wiring.ci declares both directory and none; a repository declares one of them", Path)
	case hasNone:
		reason, _ := ci["none"].(string)
		if strings.TrimSpace(reason) == "" {
			return fmt.Errorf("%s: gates.wiring.ci.none needs a reason that says why the repository hosts no workflows", Path)
		}
		wiring.NoWorkflowsReason = reason
		return nil
	case hasDirectory:
		directory, ok := repositoryDirectory(ci, "directory")
		if !ok {
			return fmt.Errorf("%s: gates.wiring.ci.directory names no repository directory", Path)
		}
		wiring.Workflows = true
		wiring.WorkflowDirectory = directory
		return nil
	default:
		return fmt.Errorf("%s: gates.wiring.ci declares neither directory nor none", Path)
	}
}

// decodeLocalSurfaces reads the optional allowed lists. Each key names a local
// surface, and each list names entry identifiers or kinds. An empty list
// allows nothing.
func decodeLocalSurfaces(wiring *Wiring, raw any) error {
	if raw == nil {
		return nil
	}
	lists, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("%s: gates.wiring.local-surfaces must be a mapping of local surfaces", Path)
	}
	for _, surface := range sortedKeys(lists) {
		if !containsString(LocalSurfaces, surface) {
			return fmt.Errorf("%s: gates.wiring.local-surfaces names `%s`, which is not a local surface", Path, surface)
		}
		items, ok := lists[surface].([]any)
		if !ok {
			return fmt.Errorf("%s: gates.wiring.local-surfaces.%s names no entry or kind", Path, surface)
		}
		allowed := []string{}
		for _, item := range items {
			name, ok := item.(string)
			if !ok || strings.TrimSpace(name) == "" {
				return fmt.Errorf("%s: gates.wiring.local-surfaces.%s names no entry or kind", Path, surface)
			}
			allowed = append(allowed, name)
		}
		wiring.LocalAllowed[surface] = allowed
	}
	return nil
}

// repositoryDirectory reads one required directory field. A directory must be
// non-empty, relative to the repository root, and must not climb out of it.
func repositoryDirectory(mapping map[string]any, key string) (string, bool) {
	value, ok := mapping[key].(string)
	if !ok || !validRepositoryDirectory(value) {
		return "", false
	}
	return value, true
}

// validRepositoryDirectory reports whether a value names a non-empty directory
// relative to the repository root that does not climb out of it.
func validRepositoryDirectory(value string) bool {
	if strings.TrimSpace(value) == "" || strings.HasPrefix(value, "/") {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == ".." {
			return false
		}
	}
	return true
}

// unknownKeys returns the first key of a mapping that the allowed names do not
// name, in sorted order, or the empty string when every key is allowed.
func unknownKeys(mapping map[string]any, allowed ...string) string {
	for _, key := range sortedKeys(mapping) {
		if !containsString(allowed, key) {
			return key
		}
	}
	return ""
}
