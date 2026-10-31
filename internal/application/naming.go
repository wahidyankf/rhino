// Filename validation: kebab-case, path-prefixed, and fragment shapes.
package application

import (
	"fmt"
	"strings"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// Finding kinds the naming validator reports.
const (
	namingInvalidKind   = "invalid-md-name"
	namingFragmentKind  = "fragmented-md-name"
	namingSurfacesKey   = "md-naming.surfaces"
	namingExemptKey     = "md-naming.exempt"
	namingKebabStyle    = "kebab-case"
	namingPrefixedStyle = "path-prefixed"
)

// fragments are the closed mechanical shapes of half a document.
var namingFragments = []string{"-part-", "-continuation-", "-continued"}

// namingSurface is one declared surface: a glob plus its filename style.
type namingSurface struct {
	glob      string
	style     string
	separator string
	hasSep    bool
}

func namingValidate(document *config.Document, tree Tree, format Format) Outcome {
	mapping, ok := document.PolicyGroup("markdown", "naming")
	if !ok {
		report := NewReport("naming", "file")
		report.Refusal = strptr("policies.markdown.naming is not declared, and RHINO holds no default for it")
		report.RefusalCode = domain.ConfigUndeclared
		return report.Render(format)
	}
	surfaces, exempt, refusal := decodeNaming(mapping)
	if refusal != nil {
		return namingRefused(format, domain.ConfigUnusable, refusal.Error())
	}
	for _, pattern := range surfaceGlobs(surfaces) {
		if _, err := Match(pattern, "probe.md"); err != nil {
			return namingRefused(format, domain.ConfigUnusable,
				namingSurfacesKey+": "+err.Error())
		}
	}
	for _, pattern := range exempt {
		if _, err := Match(pattern, "probe.md"); err != nil {
			return namingRefused(format, domain.ConfigUnusable,
				namingExemptKey+": "+err.Error())
		}
	}
	roots := make([]string, len(surfaces))
	for i, surface := range surfaces {
		roots[i] = literalRoot(surface.glob)
	}
	shared, scanErr := document.ScanExcludeDirectories()
	if scanErr != nil {
		return namingRefused(format, domain.ConfigUnusable, scanErr.Error())
	}
	report := NewReport("naming", "file")
	for _, path := range scanMarkdown(tree.Files(), shared) {
		// A fragment shape is not a style: exemptions excuse the style,
		// never half a document.
		if finding, ok := namingFragment(path); ok {
			report.Findings = append(report.Findings, finding)
		}
		if matchesAny(exempt, path) {
			continue
		}
		governed := -1
		for i, surface := range surfaces {
			ok, err := Match(surface.glob, path)
			if err != nil {
				return namingRefused(format, domain.ConfigUnusable,
					namingSurfacesKey+": "+err.Error())
			}
			// The last matching surface governs: a later declaration is
			// the more specific intent.
			if ok {
				governed = i
			}
		}
		if governed < 0 {
			continue
		}
		report.Scanned = append(report.Scanned, path)
		report.Inspected++
		if finding, ok := inspectName(path, surfaces[governed], roots[governed]); ok {
			report.Findings = append(report.Findings, finding)
		}
	}
	return report.Render(format)
}

// decodeNaming projects the declared naming policy into surfaces. Every key
// is required: the difference between "nothing is exempt" and "the exemptions
// were forgotten" stays visible in the file.
func decodeNaming(mapping map[string]any) ([]namingSurface, []string, error) {
	for key := range mapping {
		if key != "surfaces" && key != "exempt" {
			return nil, nil, fmt.Errorf("policies.markdown.naming names unknown key `%s`", key)
		}
	}
	rawSurfaces, ok := mapping["surfaces"].([]any)
	if !ok {
		return nil, nil, fmt.Errorf("policies.markdown.naming.surfaces names no surfaces")
	}
	rawExempt, ok := mapping["exempt"].([]any)
	if !ok {
		return nil, nil, fmt.Errorf("policies.markdown.naming.exempt names no exemptions")
	}
	surfaces := make([]namingSurface, 0, len(rawSurfaces))
	for _, item := range rawSurfaces {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("policies.markdown.naming.surfaces names no surface")
		}
		for key := range entry {
			if key != "glob" && key != "style" && key != "separator" {
				return nil, nil, fmt.Errorf(namingSurfacesKey+" names unknown key `%s`", key)
			}
		}
		glob, _ := entry["glob"].(string)
		if strings.TrimSpace(glob) == "" {
			return nil, nil, fmt.Errorf(namingSurfacesKey + " names no glob")
		}
		style, _ := entry["style"].(string)
		separator, _ := entry["separator"].(string)
		_, hasSep := entry["separator"]
		switch style {
		case namingKebabStyle:
			if hasSep {
				return nil, nil, fmt.Errorf(namingSurfacesKey +
					": a kebab-case surface declares no separator")
			}
		case namingPrefixedStyle:
			if !hasSep || strings.TrimSpace(separator) == "" {
				return nil, nil, fmt.Errorf(namingSurfacesKey +
					": a path-prefixed surface declares no separator")
			}
		default:
			return nil, nil, fmt.Errorf(namingSurfacesKey+
				": unrecognized style `%s`: expected `kebab-case` or `path-prefixed`", style)
		}
		surfaces = append(surfaces, namingSurface{glob: glob, style: style, separator: separator, hasSep: hasSep})
	}
	exempt := make([]string, 0, len(rawExempt))
	for _, item := range rawExempt {
		pattern, ok := item.(string)
		if !ok || strings.TrimSpace(pattern) == "" {
			return nil, nil, fmt.Errorf(namingExemptKey + " names no glob")
		}
		exempt = append(exempt, pattern)
	}
	return surfaces, exempt, nil
}

func surfaceGlobs(surfaces []namingSurface) []string {
	globs := make([]string, 0, len(surfaces))
	for _, surface := range surfaces {
		globs = append(globs, surface.glob)
	}
	return globs
}

func matchesAny(patterns []string, path string) bool {
	for _, pattern := range patterns {
		if ok, err := Match(pattern, path); err == nil && ok {
			return true
		}
	}
	return false
}

// isMarkdown reports whether a path names a Markdown file, whatever case
// its extension shouts in.
func isMarkdown(path string) bool {
	base := path
	if index := strings.LastIndex(base, "/"); index >= 0 {
		base = base[index+1:]
	}
	dot := strings.LastIndex(base, ".")
	if dot < 0 {
		return false
	}
	return strings.EqualFold(base[dot+1:], "md")
}

// namingFragment reports the closed mechanical shapes of half a document:
// -part- and -continuation- take a number, -continued takes nothing. The
// ending is tested rather than the substring, so -parted and -part-two stay
// ordinary names.
func namingFragment(path string) (Finding, bool) {
	file := path
	if index := strings.LastIndex(file, "/"); index >= 0 {
		file = file[index+1:]
	}
	stem := file
	if dot := strings.LastIndex(stem, "."); dot >= 0 {
		stem = stem[:dot]
	}
	fragmented := false
	for _, shape := range namingFragments {
		if shape == "-continued" {
			if strings.HasSuffix(stem, shape) {
				fragmented = true
			}
			continue
		}
		if index := strings.LastIndex(stem, shape); index >= 0 {
			tail := stem[index+len(shape):]
			if tail != "" && isDigits(tail) {
				fragmented = true
			}
		}
	}
	if !fragmented {
		return Finding{}, false
	}
	return NewFinding(namingFragmentKind, path,
		"is named as a fragment; a document that outgrew its budget splits into an entrypoint and a companion set rather than into numbered halves"), true
}

func isDigits(value string) bool {
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func inspectName(path string, surface namingSurface, root string) (Finding, bool) {
	directory, file := path, path
	if index := strings.LastIndex(path, "/"); index >= 0 {
		directory, file = path[:index], path[index+1:]
	} else {
		directory = ""
	}
	stem := file
	if dot := strings.LastIndex(stem, "."); dot >= 0 {
		stem = stem[:dot]
	}
	if surface.style == namingKebabStyle {
		if isKebabCase(stem) {
			return Finding{}, false
		}
		return NewFinding(namingInvalidKind, path,
			"is not named in the declared kebab-case style"), true
	}
	expected := encodedPrefix(directory, root)
	named := false
	if prefix, content, ok := strings.Cut(stem, surface.separator); ok {
		named = prefix == expected && isKebabCase(content)
	} else {
		// A file directly at the surface root has no directory to encode,
		// so it carries a content name and no separator.
		named = expected == "" && isKebabCase(stem)
	}
	if named {
		return Finding{}, false
	}
	finding := NewFinding(namingInvalidKind, path,
		"is not named in the declared path-prefixed style")
	finding.Details = append(finding.Details, TextDetail("prefix", expected))
	return finding, true
}

// isKebabCase reports lowercase alphanumeric runs joined by single hyphens.
func isKebabCase(name string) bool {
	if name == "" {
		return false
	}
	for _, part := range strings.Split(name, "-") {
		if part == "" {
			return false
		}
		for _, character := range part {
			lower := character >= 'a' && character <= 'z'
			digit := character >= '0' && character <= '9'
			if !lower && !digit {
				return false
			}
		}
	}
	return true
}

// encodedPrefix is the prefix a directory encodes to, relative to its
// surface's root: one encoded segment per level, joined by hyphens.
func encodedPrefix(directory, root string) string {
	below := directory
	if root != "" {
		if trimmed, ok := strings.CutPrefix(directory, root); ok {
			below = trimmed
		} else {
			below = directory
		}
	}
	below = strings.Trim(below, "/")
	var segments []string
	for _, segment := range strings.Split(below, "/") {
		if segment == "" {
			continue
		}
		segments = append(segments, encodeSegment(segment))
	}
	return strings.Join(segments, "-")
}

// encodeSegment encodes one directory level: each hyphen-separated word
// concatenated after encoding.
func encodeSegment(segment string) string {
	var out strings.Builder
	for _, word := range strings.Split(segment, "-") {
		if word == "" {
			continue
		}
		out.WriteString(encodeWord(word))
	}
	return out.String()
}

// encodeWord keeps two characters of a word, or one character and an
// underscore so a one-character word stays visible.
func encodeWord(word string) string {
	lowered := strings.ToLower(word)
	characters := []rune(lowered)
	if len(characters) == 1 {
		return lowered + "_"
	}
	return string(characters[:2])
}

// literalRoot is the fixed leading path of a glob: every segment before the
// first one carrying a metacharacter. Prefixes are relative to it, derived
// from the glob so the two cannot disagree.
func literalRoot(glob string) string {
	var segments []string
	for _, segment := range strings.Split(glob, "/") {
		if strings.ContainsAny(segment, "*?[{") {
			break
		}
		segments = append(segments, segment)
	}
	return strings.Join(segments, "/")
}

func namingRefused(format Format, code domain.ErrorCode, reason string) Outcome {
	report := NewReport("naming", "file")
	report.Refusal = &reason
	report.RefusalCode = code
	return report.Render(format)
}
