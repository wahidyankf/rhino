// Layers validation: the declared governance layer and category structure.
//
// The root's direct children are the layers: every ordered layer must exist,
// and nothing else may sit beside them. Inside a layer, only directories
// count as categories, so a file at the layer's top level stays silent while
// an undeclared directory is accused. A root behind a link is never read,
// and a malformed declaration fails the run closed.
package application

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// layerPolicy is the declared directory vocabulary and its required order.
type layerPolicy struct {
	root       string
	order      []string
	categories map[string][]string
}

func layersValidate(document *config.Document, tree Tree, format Format) Outcome {
	mapping, ok := document.PolicyGroup("governance", "layers")
	if !ok {
		report := NewReport("layers", "governance directory")
		report.Refusal = strptr("policies.governance.layers is not declared, and RHINO holds no default for it")
		report.RefusalCode = domain.ConfigUndeclared
		return report.Render(format)
	}
	policy, refusal := decodeLayers(mapping)
	if refusal != nil {
		return layersRefused(format, domain.ConfigUnusable, refusal.Error())
	}
	if !isExactPath(policy.root) {
		return layersRefused(format, domain.ConfigUnusable,
			"a layer root and every layer name must be exact repository-relative names")
	}
	for _, layer := range policy.order {
		if !isLayerSimpleName(layer) {
			return layersRefused(format, domain.ConfigUnusable,
				"a layer root and every layer name must be exact repository-relative names")
		}
	}
	seen := map[string]bool{}
	for _, layer := range policy.order {
		if seen[layer] {
			return layersRefused(format, domain.ConfigUnusable,
				"layer order and categories must use unique exact simple names")
		}
		seen[layer] = true
	}
	for layer := range policy.categories {
		if !seen[layer] {
			return layersRefused(format, domain.ConfigUnusable,
				"layer order and categories must use unique exact simple names")
		}
	}
	for _, categories := range policy.categories {
		for _, category := range categories {
			if !isLayerSimpleName(category) {
				return layersRefused(format, domain.ConfigUnusable,
					"layer order and categories must use unique exact simple names")
			}
		}
	}
	if tree.PassesThroughLink(policy.root) {
		return layersRefused(format, domain.FileUnreadable,
			policy.root+": passes through a symbolic link, which RHINO never follows")
	}
	report := NewReport("layers", "governance directory")
	actual := map[string]bool{}
	for _, child := range tree.Children(policy.root) {
		if name, ok := strings.CutPrefix(child, policy.root+"/"); ok {
			actual[name] = true
		}
	}
	report.Inspected = len(actual)
	for _, layer := range policy.order {
		path := policy.root + "/" + layer
		if !actual[layer] {
			report.Findings = append(report.Findings, NewFinding("missing-governance-layer", path,
				"a declared governance layer is absent"))
			continue
		}
		present := map[string]bool{}
		for _, child := range tree.Children(path) {
			if !tree.IsDirectory(child) {
				continue
			}
			if name, ok := strings.CutPrefix(child, path+"/"); ok {
				present[name] = true
			}
		}
		for _, category := range policy.categories[layer] {
			if !present[category] {
				report.Findings = append(report.Findings, NewFinding("missing-governance-category",
					path+"/"+category, "a declared governance category is absent"))
			}
		}
		var unexpected []string
		for category := range present {
			if !layerLists(policy.categories[layer], category) {
				unexpected = append(unexpected, category)
			}
		}
		sort.Strings(unexpected)
		for _, category := range unexpected {
			report.Findings = append(report.Findings, NewFinding("unexpected-governance-category",
				path+"/"+category, "the governance category is not declared for its layer"))
		}
	}
	var extra []string
	for layer := range actual {
		if !seen[layer] {
			extra = append(extra, layer)
		}
	}
	sort.Strings(extra)
	for _, layer := range extra {
		report.Findings = append(report.Findings, NewFinding("unexpected-governance-layer",
			policy.root+"/"+layer, "the governance layer is not declared"))
	}
	return report.Render(format)
}

// decodeLayers projects the declared layer vocabulary. Unknown keys refuse:
// a policy the tool silently ignored would be a check that never ran. An
// absent order or category map holds nothing; a present one must hold names.
func decodeLayers(mapping map[string]any) (layerPolicy, error) {
	var policy layerPolicy
	for key := range mapping {
		if key != "root" && key != "order" && key != "categories" {
			return layerPolicy{}, fmt.Errorf("policies.governance.layers names unknown key `%s`", key)
		}
	}
	if mapping["root"] != nil {
		root, ok := mapping["root"].(string)
		if !ok {
			return layerPolicy{}, fmt.Errorf("layers.root names no layer root")
		}
		policy.root = root
	}
	if mapping["order"] != nil {
		raw, ok := mapping["order"].([]any)
		if !ok {
			return layerPolicy{}, fmt.Errorf("layers.order names no layer order")
		}
		for _, item := range raw {
			layer, ok := item.(string)
			if !ok {
				return layerPolicy{}, fmt.Errorf("layers.order names no layer")
			}
			policy.order = append(policy.order, layer)
		}
	}
	policy.categories = map[string][]string{}
	if mapping["categories"] != nil {
		raw, ok := mapping["categories"].(map[string]any)
		if !ok {
			return layerPolicy{}, fmt.Errorf("layers.categories names no categories")
		}
		for layer, value := range raw {
			items, ok := value.([]any)
			if value != nil && !ok {
				return layerPolicy{}, fmt.Errorf("layers.categories names no categories")
			}
			var categories []string
			for _, item := range items {
				category, ok := item.(string)
				if !ok {
					return layerPolicy{}, fmt.Errorf("layers.categories names no category")
				}
				categories = append(categories, category)
			}
			policy.categories[layer] = categories
		}
	}
	return policy, nil
}

// isLayerSimpleName answers whether a value is a simple name: lowercase
// ASCII letters, digits, and hyphens, nothing else and never empty.
func isLayerSimpleName(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		lower := character >= 'a' && character <= 'z'
		digit := character >= '0' && character <= '9'
		if !lower && !digit && character != '-' {
			return false
		}
	}
	return true
}

// layerLists answers whether a declared category list names a category.
func layerLists(categories []string, category string) bool {
	for _, declared := range categories {
		if declared == category {
			return true
		}
	}
	return false
}

func layersRefused(format Format, code domain.ErrorCode, reason string) Outcome {
	report := NewReport("layers", "governance directory")
	report.Refusal = &reason
	report.RefusalCode = code
	return report.Render(format)
}
