// Front-matter validation: declared schema over leading declarations.
package application

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// frontmatterSurface is one declared surface: a glob plus the schema it
// enforces. Every key list is the repository's; the reading is the tool's.
type frontmatterSurface struct {
	glob    string
	require []string
	values  map[string][]string
	isoDate []string
	forbid  []string
}

// frontmatterEntry is one top-level declaration and its line.
type frontmatterEntry struct {
	line  int
	key   string
	value string
}

func frontmatterValidate(document *config.Document, tree Tree, format Format) Outcome {
	mapping, ok := document.PolicyGroup("markdown", "frontmatter")
	if !ok {
		report := NewReport("frontmatter", "file")
		report.Refusal = strptr("policies.markdown.frontmatter is not declared, and RHINO holds no default for it")
		report.RefusalCode = domain.ConfigUndeclared
		return report.Render(format)
	}
	surfaces, refusal := decodeFrontmatter(mapping)
	if refusal != nil {
		return frontmatterRefused(format, domain.ConfigUnusable, refusal.Error())
	}
	for _, pattern := range frontmatterGlobs(surfaces) {
		if _, err := Match(pattern, "probe.md"); err != nil {
			return frontmatterRefused(format, domain.ConfigUnusable,
				"md-frontmatter.surfaces: "+err.Error())
		}
	}
	shared, err := document.ScanExcludeDirectories()
	if err != nil {
		return frontmatterRefused(format, domain.ConfigUnusable, err.Error())
	}
	files, surfaceErr := surfaceFiles(tree, frontmatterGlobs(surfaces), shared)
	if surfaceErr != nil {
		return frontmatterRefused(format, surfaceErr.code, surfaceErr.reason)
	}
	report := NewReport("frontmatter", "file")
	for _, file := range files {
		path := file.path
		if !isMarkdown(path) {
			continue
		}
		// The last matching surface governs: a later declaration is the
		// more specific intent, the same rule every surface list states.
		governed := -1
		for i, surface := range surfaces {
			ok, err := Match(surface.glob, path)
			if err != nil {
				return frontmatterRefused(format, domain.ConfigUnusable,
					"md-frontmatter.surfaces: "+err.Error())
			}
			if ok {
				governed = i
			}
		}
		if governed < 0 {
			continue
		}
		text, readErr := tree.Read(file.source)
		if readErr != nil {
			switch readErr.Kind {
			case "missing":
				continue
			case "not-text":
				return frontmatterRefused(format, domain.FileUnreadable, path+": holds no text")
			default:
				return frontmatterRefused(format, domain.FileUnreadable, path+": "+readErr.Reason)
			}
		}
		report.Scanned = append(report.Scanned, path)
		report.Inspected++
		inspectFrontmatter(path, text, surfaces[governed], report)
	}
	return report.Render(format)
}

// decodeFrontmatter projects the declared schema into surfaces. The required
// lists stay visible: a surface that requires nothing and one whose
// requirements were forgotten have to look different in the file.
func decodeFrontmatter(mapping map[string]any) ([]frontmatterSurface, error) {
	for key := range mapping {
		if key != "surfaces" {
			return nil, fmt.Errorf("policies.markdown.frontmatter names unknown key `%s`", key)
		}
	}
	rawSurfaces, ok := mapping["surfaces"].([]any)
	if !ok {
		return nil, fmt.Errorf("md-frontmatter.surfaces names no surfaces")
	}
	surfaces := make([]frontmatterSurface, 0, len(rawSurfaces))
	for _, item := range rawSurfaces {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("md-frontmatter.surfaces names no surface")
		}
		for key := range entry {
			if key != "glob" && key != "require" && key != "enum" && key != "iso-date" && key != "forbid" {
				return nil, fmt.Errorf("md-frontmatter.surfaces names unknown key `%s`", key)
			}
		}
		glob, _ := entry["glob"].(string)
		if strings.TrimSpace(glob) == "" {
			return nil, fmt.Errorf("md-frontmatter.surfaces names no glob")
		}
		rawRequire, ok := entry["require"].([]any)
		if !ok {
			return nil, fmt.Errorf("md-frontmatter.surfaces names no require")
		}
		require := make([]string, 0, len(rawRequire))
		for _, key := range rawRequire {
			name, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("md-frontmatter.surfaces names no require")
			}
			require = append(require, name)
		}
		values := map[string][]string{}
		if rawEnum, present := entry["enum"]; present {
			mapping, ok := rawEnum.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("md-frontmatter.surfaces names no enum")
			}
			for key, raw := range mapping {
				rawPermitted, ok := raw.([]any)
				if !ok {
					return nil, fmt.Errorf("md-frontmatter.surfaces names no enum")
				}
				var permitted []string
				for _, value := range rawPermitted {
					text, ok := value.(string)
					if !ok {
						return nil, fmt.Errorf("md-frontmatter.surfaces names no enum")
					}
					permitted = append(permitted, text)
				}
				values[key] = permitted
			}
		}
		isoDate, err := stringList(entry, "iso-date")
		if err != nil {
			return nil, err
		}
		forbid, err := stringList(entry, "forbid")
		if err != nil {
			return nil, err
		}
		surfaces = append(surfaces, frontmatterSurface{
			glob: glob, require: require, values: values, isoDate: isoDate, forbid: forbid,
		})
	}
	return surfaces, nil
}

func stringList(entry map[string]any, key string) ([]string, error) {
	raw, present := entry[key]
	if !present {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("md-frontmatter.surfaces names no %s", key)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("md-frontmatter.surfaces names no %s", key)
		}
		out = append(out, text)
	}
	return out, nil
}

func frontmatterGlobs(surfaces []frontmatterSurface) []string {
	globs := make([]string, 0, len(surfaces))
	for _, surface := range surfaces {
		globs = append(globs, surface.glob)
	}
	return globs
}

func inspectFrontmatter(path, text string, surface frontmatterSurface, report *Report) {
	entries, unterminated := readHead(text)
	if unterminated {
		finding := NewFinding("unterminated-frontmatter", path, "front matter opens and never closes")
		at := 1
		finding.Line = &at
		report.Findings = append(report.Findings, finding)
		return
	}
	held := func(name string) *frontmatterEntry {
		for i := range entries {
			if entries[i].key == name {
				return &entries[i]
			}
		}
		return nil
	}
	for _, key := range surface.require {
		if held(key) == nil {
			// Line 1, because the fault is the absence and the absence has
			// no position of its own; the file's first line is where a
			// reader starts looking for it.
			finding := NewFinding("missing-frontmatter-key", path,
				fmt.Sprintf("front matter declares no `%s`", key))
			at := 1
			finding.Line = &at
			report.Findings = append(report.Findings, finding)
		}
	}
	for _, key := range surface.forbid {
		if entry := held(key); entry != nil {
			finding := NewFinding("forbidden-frontmatter-key", path,
				fmt.Sprintf("`%s` is a key this surface forbids", key))
			at := entry.line
			finding.Line = &at
			report.Findings = append(report.Findings, finding)
		}
	}
	// A closed set and a date are rules about a value that is there. A key
	// that is absent is require's business; reporting it twice would make
	// one fault read as two.
	for key, permitted := range surface.values {
		entry := held(key)
		if entry == nil || contains(permitted, entry.value) {
			continue
		}
		finding := NewFinding("invalid-frontmatter-value", path,
			fmt.Sprintf("`%s` holds a value outside its declared set", key))
		at := entry.line
		finding.Line = &at
		finding.Details = append(finding.Details, TextDetail("value", entry.value))
		report.Findings = append(report.Findings, finding)
	}
	for _, key := range surface.isoDate {
		entry := held(key)
		if entry == nil || isISODate(entry.value) {
			continue
		}
		finding := NewFinding("invalid-frontmatter-value", path,
			fmt.Sprintf("`%s` does not hold an ISO calendar date", key))
		at := entry.line
		finding.Line = &at
		finding.Details = append(finding.Details, TextDetail("value", entry.value))
		report.Findings = append(report.Findings, finding)
	}
}

// readHead reads a document's front matter from its first line. The block has
// to be the first thing in the file: a fence further down is a thematic break
// or a table rule, not front matter.
func readHead(text string) (entries []frontmatterEntry, unterminated bool) {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimRightFunc(lines[0], unicode.IsSpace) != "---" {
		return nil, false
	}
	for index, line := range lines[1:] {
		if strings.TrimRightFunc(line, unicode.IsSpace) == "---" {
			return entries, false
		}
		if entry, ok := declaration(index+2, line); ok {
			entries = append(entries, entry)
		}
	}
	return nil, true
}

// declaration reads one top-level declaration. Indented lines belong to the
// value above them, so they are never top-level keys; neither is a comment.
func declaration(line int, text string) (frontmatterEntry, bool) {
	if text == "" {
		return frontmatterEntry{}, false
	}
	for _, first := range text {
		if unicode.IsSpace(first) {
			return frontmatterEntry{}, false
		}
		break
	}
	if strings.HasPrefix(strings.TrimLeftFunc(text, unicode.IsSpace), "#") {
		return frontmatterEntry{}, false
	}
	key, value, ok := strings.Cut(text, ":")
	if !ok {
		return frontmatterEntry{}, false
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return frontmatterEntry{}, false
	}
	return frontmatterEntry{line: line, key: key, value: unquoted(strings.TrimSpace(value))}, true
}

// unquoted strips the quotes a document may or may not have written the value
// with. The two spellings are the same value, so they get the same answer.
func unquoted(value string) string {
	for _, quote := range []string{"\"", "'"} {
		if inner, ok := strings.CutPrefix(value, quote); ok {
			if stripped, ok := strings.CutSuffix(inner, quote); ok {
				return stripped
			}
		}
	}
	return value
}

// isISODate reports YYYY-MM-DD dates the calendar actually has. The calendar
// check is the point: a well-shaped 2026-02-30 is the failure a pattern match
// cannot see.
func isISODate(value string) bool {
	parts := strings.Split(value, "-")
	if len(parts) != 3 {
		return false
	}
	year, month, day := parts[0], parts[1], parts[2]
	if len(year) != 4 || len(month) != 2 || len(day) != 2 {
		return false
	}
	for _, part := range parts {
		for i := 0; i < len(part); i++ {
			if part[i] < '0' || part[i] > '9' {
				return false
			}
		}
	}
	yearNumber, monthNumber, dayNumber := digits(year), digits(month), digits(day)
	if monthNumber < 1 || monthNumber > 12 || dayNumber < 1 {
		return false
	}
	return dayNumber <= daysIn(yearNumber, monthNumber)
}

func digits(value string) int {
	number := 0
	for i := 0; i < len(value); i++ {
		number = number*10 + int(value[i]-'0')
	}
	return number
}

func daysIn(year, month int) int {
	switch month {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	default:
		if year%4 == 0 && (year%100 != 0 || year%400 == 0) {
			return 29
		}
		return 28
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func frontmatterRefused(format Format, code domain.ErrorCode, reason string) Outcome {
	report := NewReport("frontmatter", "file")
	report.Refusal = &reason
	report.RefusalCode = code
	return report.Render(format)
}
