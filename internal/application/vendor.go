// Vendor validation: declared portable roots stay free of declared terms.
//
// A root is scanned file by file; files outside every root, and files under
// an excluded binding root, are never read and never accused. An exception
// pairs one term with exact paths, so a broad exception cannot silently
// disable another term or another part of the tree. A root behind a link is
// never followed, and a file that cannot be read fails the run closed.
package application

import (
	"fmt"
	"strings"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// vendorException pairs one forbidden term with the exact paths it may appear
// in. The pairing is the rule: an exception never travels to another term.
type vendorException struct {
	term  string
	paths []string
}

// vendorPolicy is the declared portable-source vocabulary.
type vendorPolicy struct {
	roots                []string
	excludedBindingRoots []string
	forbiddenTerms       []string
	vocabularyExceptions []vendorException
}

func vendorValidate(document *config.Document, tree Tree, format Format) Outcome {
	mapping, ok := document.PolicyGroup("governance", "vendor")
	if !ok {
		report := NewReport("vendor", "file")
		report.Refusal = strptr("policies.governance.vendor is not declared, and RHINO holds no default for it")
		report.RefusalCode = domain.ConfigUndeclared
		return report.Render(format)
	}
	policy, refusal := decodeVendor(mapping)
	if refusal != nil {
		return vendorRefused(format, domain.ConfigUnusable, refusal.Error())
	}
	for _, root := range policy.roots {
		if !isExactPath(root) {
			return vendorRefused(format, domain.ConfigUnusable,
				"vendor roots and excluded binding roots must be exact repository-relative paths")
		}
	}
	for _, root := range policy.excludedBindingRoots {
		if !isExactPath(root) {
			return vendorRefused(format, domain.ConfigUnusable,
				"vendor roots and excluded binding roots must be exact repository-relative paths")
		}
	}
	for _, term := range policy.forbiddenTerms {
		if strings.TrimSpace(term) == "" {
			return vendorRefused(format, domain.ConfigUnusable,
				"vendor forbidden terms must not be empty")
		}
	}
	for _, exception := range policy.vocabularyExceptions {
		if strings.TrimSpace(exception.term) == "" {
			return vendorRefused(format, domain.ConfigUnusable,
				"vendor exception terms and paths must be exact repository-relative paths")
		}
		for _, path := range exception.paths {
			if !isExactPath(path) {
				return vendorRefused(format, domain.ConfigUnusable,
					"vendor exception terms and paths must be exact repository-relative paths")
			}
		}
	}
	for _, root := range policy.roots {
		if tree.PassesThroughLink(root) {
			return vendorRefused(format, domain.FileUnreadable,
				root+": passes through a symbolic link, which RHINO never follows")
		}
	}
	report := NewReport("vendor", "file")
	for _, path := range tree.Files() {
		inside := false
		for _, root := range policy.roots {
			if vendorUnder(path, root) {
				inside = true
				break
			}
		}
		if !inside {
			continue
		}
		excluded := false
		for _, root := range policy.excludedBindingRoots {
			if vendorUnder(path, root) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}
		text, readErr := tree.Read(path)
		if readErr != nil {
			switch readErr.Kind {
			case "missing":
				continue
			case "not-text":
				return vendorRefused(format, domain.FileUnreadable, path+": holds no text")
			default:
				return vendorRefused(format, domain.FileUnreadable, path+": "+readErr.Reason)
			}
		}
		report.Scanned = append(report.Scanned, path)
		report.Inspected++
		for _, term := range policy.forbiddenTerms {
			if strings.Contains(text, term) && !vendorExcepted(policy, path, term) {
				finding := NewFinding("forbidden-vendor-term", path,
					"a configured vendor term appears outside its exact exception")
				finding.Details = append(finding.Details, TextDetail("term", term))
				report.Findings = append(report.Findings, finding)
			}
		}
	}
	return report.Render(format)
}

// decodeVendor projects the declared vendor vocabulary. Unknown keys refuse:
// a policy the tool silently ignored would be a check that never ran.
func decodeVendor(mapping map[string]any) (vendorPolicy, error) {
	var policy vendorPolicy
	for key := range mapping {
		if key != "roots" && key != "excluded-binding-roots" &&
			key != "forbidden-terms" && key != "vocabulary-exceptions" {
			return vendorPolicy{}, fmt.Errorf("policies.governance.vendor names unknown key `%s`", key)
		}
	}
	roots, err := decodeVendorStrings(mapping, "roots", "roots")
	if err != nil {
		return vendorPolicy{}, err
	}
	policy.roots = roots
	excluded, err := decodeVendorStrings(mapping, "excluded-binding-roots", "roots")
	if err != nil {
		return vendorPolicy{}, err
	}
	policy.excludedBindingRoots = excluded
	terms, err := decodeVendorStrings(mapping, "forbidden-terms", "terms")
	if err != nil {
		return vendorPolicy{}, err
	}
	policy.forbiddenTerms = terms
	exceptions, err := decodeVendorExceptions(mapping)
	if err != nil {
		return vendorPolicy{}, err
	}
	policy.vocabularyExceptions = exceptions
	return policy, nil
}

// decodeVendorStrings projects one declared string list. An absent key holds
// nothing; a present one must hold strings.
func decodeVendorStrings(mapping map[string]any, key, noun string) ([]string, error) {
	raw, present := mapping[key]
	if !present {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("vendor.%s names no %s", key, noun)
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		value, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("vendor.%s names no %s", key, noun)
		}
		values = append(values, value)
	}
	return values, nil
}

// decodeVendorExceptions projects the exact term-and-path exceptions.
func decodeVendorExceptions(mapping map[string]any) ([]vendorException, error) {
	raw, present := mapping["vocabulary-exceptions"]
	if !present {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("vendor.vocabulary-exceptions names no exceptions")
	}
	exceptions := make([]vendorException, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("vendor.vocabulary-exceptions names no exceptions")
		}
		for key := range entry {
			if key != "term" && key != "paths" {
				return nil, fmt.Errorf("vendor.vocabulary-exceptions name unknown key `%s`", key)
			}
		}
		term, ok := entry["term"].(string)
		if !ok {
			return nil, fmt.Errorf("vendor.vocabulary-exceptions names no term")
		}
		var paths []string
		if entry["paths"] != nil {
			rawPaths, ok := entry["paths"].([]any)
			if !ok {
				return nil, fmt.Errorf("vendor.vocabulary-exceptions names no paths")
			}
			for _, rawPath := range rawPaths {
				path, ok := rawPath.(string)
				if !ok {
					return nil, fmt.Errorf("vendor.vocabulary-exceptions names no paths")
				}
				paths = append(paths, path)
			}
		}
		exceptions = append(exceptions, vendorException{term: term, paths: paths})
	}
	return exceptions, nil
}

// vendorUnder answers whether a path is the root itself or lives under it: a
// name that merely shares the root's prefix is outside it.
func vendorUnder(path, root string) bool {
	return path == root || strings.HasPrefix(path, root+"/")
}

// vendorExcepted answers whether an exact exception covers this term at this
// path. The term and the path must both match; neither travels alone.
func vendorExcepted(policy vendorPolicy, path, term string) bool {
	for _, exception := range policy.vocabularyExceptions {
		if exception.term != term {
			continue
		}
		for _, allowed := range exception.paths {
			if allowed == path {
				return true
			}
		}
	}
	return false
}

func vendorRefused(format Format, code domain.ErrorCode, reason string) Outcome {
	report := NewReport("vendor", "file")
	report.Refusal = &reason
	report.RefusalCode = code
	return report.Render(format)
}
