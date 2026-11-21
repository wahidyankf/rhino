package config

import (
	"fmt"
	"strings"
)

// BehaviourLevels are the three levels a behaviour reference or exemption names.
var BehaviourLevels = []string{"unit", "integration", "e2e"}

// Behaviour is the declared policies.behaviour section: the corpus roots, the
// fixture directories, the whole-file globs a `*` reference may sit in, and the
// test-file globs of each level.
type Behaviour struct {
	Corpus    []string
	Fixtures  []string
	WholeFile []string
	Levels    map[string][]string
}

// Behaviour returns the declared policies.behaviour section. An omitted section
// is reported as undeclared; a malformed one is an error that names the field.
func (d *Document) Behaviour() (*Behaviour, bool, error) {
	raw, present := d.policies["behaviour"]
	if !present || raw == nil {
		return nil, false, nil
	}
	section, ok := raw.(map[string]any)
	if !ok {
		return nil, true, fmt.Errorf("%s: policies.behaviour must be a mapping", Path)
	}
	if unknown := unknownKeys(section, "corpus", "fixtures", "whole-file", "levels"); unknown != "" {
		return nil, true, fmt.Errorf("%s: policies.behaviour names unknown key `%s`", Path, unknown)
	}
	behaviour := &Behaviour{Levels: map[string][]string{}}
	var err error
	if behaviour.Corpus, err = behaviourDirectories(section, "corpus", true); err != nil {
		return nil, true, err
	}
	if behaviour.Fixtures, err = behaviourDirectories(section, "fixtures", false); err != nil {
		return nil, true, err
	}
	if behaviour.WholeFile, err = behaviourItems(section, "whole-file", false); err != nil {
		return nil, true, err
	}
	levels, ok := section["levels"].(map[string]any)
	if !ok {
		return nil, true, fmt.Errorf("%s: policies.behaviour.levels names no unit, integration, or e2e glob list", Path)
	}
	if unknown := unknownKeys(levels, BehaviourLevels...); unknown != "" {
		return nil, true, fmt.Errorf("%s: policies.behaviour.levels names unknown level `%s`", Path, unknown)
	}
	for _, level := range BehaviourLevels {
		globs, err := behaviourItems(levels, level, true)
		if err != nil {
			return nil, true, err
		}
		behaviour.Levels[level] = globs
	}
	return behaviour, true, nil
}

// behaviourDirectories reads a list of repository-relative directories.
func behaviourDirectories(mapping map[string]any, key string, required bool) ([]string, error) {
	items, err := behaviourItems(mapping, key, required)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if !validRepositoryDirectory(item) {
			return nil, fmt.Errorf("%s: policies.behaviour.%s names `%s`, which is not a repository directory", Path, key, item)
		}
	}
	return items, nil
}

// behaviourItems reads one list of non-empty strings. A missing required key
// names the field; a missing optional key reads as an empty list.
func behaviourItems(mapping map[string]any, key string, required bool) ([]string, error) {
	raw, present := mapping[key]
	if !present || raw == nil {
		if required {
			return nil, fmt.Errorf("%s: policies.behaviour.%s names no entry", Path, key)
		}
		return nil, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s: policies.behaviour.%s must be a list", Path, key)
	}
	items := make([]string, 0, len(list))
	for _, item := range list {
		text, ok := item.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("%s: policies.behaviour.%s names an empty entry", Path, key)
		}
		items = append(items, text)
	}
	return items, nil
}
