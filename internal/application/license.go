// License validation: configured license paths, identifiers, and digests.
//
// A path can require exact identifiers, a SHA-256 digest, or both; omitted
// checks are never supplied here. Exclusions are exact repository-relative
// paths, skipped before any read. A path behind a link is never followed,
// and a path that cannot be read fails the run closed.
package application

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// licensePath is one declared license file with its optional checks.
type licensePath struct {
	path        string
	identifiers []string
	sha256      *string
	hasDigest   bool
}

func licenseValidate(document *config.Document, tree Tree, format Format) Outcome {
	mapping, ok := document.PolicyGroup("conventions", "license")
	if !ok {
		report := NewReport("license", "license file")
		report.Refusal = strptr("policies.conventions.license is not declared, and RHINO holds no default for it")
		report.RefusalCode = domain.ConfigUndeclared
		return report.Render(format)
	}
	paths, exclusions, refusal := decodeLicense(mapping)
	if refusal != nil {
		return licenseRefused(format, domain.ConfigUnusable, refusal.Error())
	}
	for _, excluded := range exclusions {
		if !licenseExactPath(excluded) {
			return licenseRefused(format, domain.ConfigUnusable,
				"license exclusions must be exact repository-relative paths")
		}
	}
	report := NewReport("license", "license file")
	seen := map[string]bool{}
	for _, entry := range paths {
		if !licenseExactPath(entry.path) {
			return licenseRefused(format, domain.ConfigUnusable,
				"license paths must be exact repository-relative paths")
		}
		if seen[entry.path] {
			return licenseRefused(format, domain.ConfigUnusable, "license paths must not repeat")
		}
		seen[entry.path] = true
		excluded := false
		for _, skip := range exclusions {
			if skip == entry.path {
				excluded = true
			}
		}
		if excluded {
			continue
		}
		if entry.hasDigest && !isLicenseDigest(*entry.sha256) {
			return licenseRefused(format, domain.ConfigUnusable,
				"a license sha256 must be 64 lowercase hexadecimal characters")
		}
		if tree.PassesThroughLink(entry.path) {
			return licenseRefused(format, domain.FileUnreadable, entry.path+": "+entry.path+
				" passes through a symbolic link, which RHINO never follows")
		}
		text, readErr := tree.Read(entry.path)
		if readErr != nil {
			switch readErr.Kind {
			case "missing":
				report.Findings = append(report.Findings,
					NewFinding("missing-license", entry.path, "the configured license file is absent"))
				continue
			case "not-text":
				return licenseRefused(format, domain.FileUnreadable, entry.path+": holds no text")
			case "escapes":
				return licenseRefused(format, domain.FileUnreadable, entry.path+": "+entry.path+
					" passes through a symbolic link, which RHINO never follows")
			default:
				return licenseRefused(format, domain.FileUnreadable, entry.path+": "+readErr.Reason)
			}
		}
		report.Scanned = append(report.Scanned, entry.path)
		report.Inspected++
		for _, identifier := range entry.identifiers {
			if !strings.Contains(text, identifier) {
				finding := NewFinding("license-identifier-mismatch", entry.path,
					"the configured license identifier is absent")
				finding.Details = append(finding.Details, TextDetail("identifier", identifier))
				report.Findings = append(report.Findings, finding)
			}
		}
		if entry.hasDigest && licenseDigestOf(text) != *entry.sha256 {
			report.Findings = append(report.Findings,
				NewFinding("license-digest-mismatch", entry.path, "the configured license digest does not match"))
		}
	}
	return report.Render(format)
}

// decodeLicense projects the declared license policy. Unknown keys refuse:
// a policy the tool silently ignored would be a check that never ran.
func decodeLicense(mapping map[string]any) ([]licensePath, []string, error) {
	for key := range mapping {
		if key != "paths" && key != "exclusions" {
			return nil, nil, fmt.Errorf("policies.conventions.license names unknown key `%s`", key)
		}
	}
	var rawPaths []any
	if mapping["paths"] != nil {
		var ok bool
		rawPaths, ok = mapping["paths"].([]any)
		if !ok {
			return nil, nil, fmt.Errorf("policies.conventions.license.paths names no license paths")
		}
	}
	rawExclusions, ok := mapping["exclusions"].([]any)
	if mapping["exclusions"] != nil && !ok {
		return nil, nil, fmt.Errorf("policies.conventions.license.exclusions names no exclusions")
	}
	_ = rawExclusions
	var exclusions []string
	if mapping["exclusions"] != nil {
		for _, item := range rawExclusions {
			path, ok := item.(string)
			if !ok || strings.TrimSpace(path) == "" {
				return nil, nil, fmt.Errorf("policies.conventions.license.exclusions names no exclusion")
			}
			exclusions = append(exclusions, path)
		}
	}
	paths := make([]licensePath, 0, len(rawPaths))
	for _, item := range rawPaths {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("policies.conventions.license.paths names no license path")
		}
		for key := range entry {
			if key != "path" && key != "identifiers" && key != "sha256" {
				return nil, nil, fmt.Errorf("policies.conventions.license.paths names unknown key `%s`", key)
			}
		}
		path, _ := entry["path"].(string)
		if strings.TrimSpace(path) == "" {
			return nil, nil, fmt.Errorf("policies.conventions.license.paths names no license path")
		}
		var identifiers []string
		if entry["identifiers"] != nil {
			rawIdentifiers, ok := entry["identifiers"].([]any)
			if !ok {
				return nil, nil, fmt.Errorf("policies.conventions.license.paths names no license identifiers")
			}
			for _, raw := range rawIdentifiers {
				identifier, ok := raw.(string)
				if !ok || identifier == "" {
					return nil, nil, fmt.Errorf("policies.conventions.license.paths names no license identifier")
				}
				identifiers = append(identifiers, identifier)
			}
		}
		var digest *string
		hasDigest := false
		if entry["sha256"] != nil {
			text, ok := entry["sha256"].(string)
			if !ok {
				return nil, nil, fmt.Errorf("policies.conventions.license.paths names no license digest")
			}
			digest = &text
			hasDigest = true
		}
		paths = append(paths, licensePath{path: path, identifiers: identifiers, sha256: digest, hasDigest: hasDigest})
	}
	return paths, exclusions, nil
}

// licenseExactPath answers whether a path names one exact file: no empty
// parts, no climbing, no absolute root, and no glob characters that would
// quietly widen one file into many.
func licenseExactPath(path string) bool {
	if strings.TrimSpace(path) == "" || strings.HasPrefix(path, "/") {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
		if strings.Contains(part, "\\") {
			return false
		}
		if strings.ContainsAny(part, "*?[]{}") {
			return false
		}
	}
	return true
}

// licenseDigestOf hashes file text the way the declared digest was computed.
func licenseDigestOf(text string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(text)))
}

// isLicenseDigest answers whether a value has the shape of a SHA-256 digest:
// 64 lowercase hexadecimal characters, nothing else.
func isLicenseDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for i := 0; i < len(value); i++ {
		byte := value[i]
		if !(byte >= '0' && byte <= '9') && !(byte >= 'a' && byte <= 'f') {
			return false
		}
	}
	return true
}

func licenseRefused(format Format, code domain.ErrorCode, reason string) Outcome {
	report := NewReport("license", "license file")
	report.Refusal = &reason
	report.RefusalCode = code
	return report.Render(format)
}
