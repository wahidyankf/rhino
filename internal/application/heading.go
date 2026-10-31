// Heading-hierarchy validation: one level-1 heading and bounded drops.
package application

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// headingRules is the repository's two rules: whether a governed document
// holds exactly one level-1 heading, and how far a heading may drop below the
// one before it. What counts as a heading is the format's, not the
// repository's, so the fence reader decides that here.
type headingRules struct {
	surfaces []string
	singleH1 bool
	maxJump  int
}

// heading is one ATX heading: its one-based line and its level.
type heading struct {
	line  int
	level int
}

func headingValidate(document *config.Document, tree Tree, format Format) Outcome {
	mapping, ok := document.PolicyGroup("markdown", "heading-hierarchy")
	if !ok {
		report := NewReport("heading-hierarchy", "file")
		report.Refusal = strptr("policies.markdown.heading-hierarchy is not declared, and RHINO holds no default for it")
		report.RefusalCode = domain.ConfigUndeclared
		return report.Render(format)
	}
	rules, refusal := decodeHeading(mapping)
	if refusal != nil {
		return headingRefused(format, domain.ConfigUnusable, refusal.Error())
	}
	for _, pattern := range rules.surfaces {
		if _, err := Match(pattern, "probe.md"); err != nil {
			return headingRefused(format, domain.ConfigUnusable,
				"md-heading-hierarchy.surfaces: "+err.Error())
		}
	}
	shared, err := document.ScanExcludeDirectories()
	if err != nil {
		return headingRefused(format, domain.ConfigUnusable, err.Error())
	}
	files, surfaceErr := surfaceFiles(tree, rules.surfaces, shared)
	if surfaceErr != nil {
		return headingRefused(format, surfaceErr.code, surfaceErr.reason)
	}
	report := NewReport("heading-hierarchy", "file")
	for _, file := range files {
		path := file.path
		if !isMarkdown(path) || !matchesAny(rules.surfaces, path) {
			continue
		}
		text, readErr := tree.Read(file.source)
		if readErr != nil {
			switch readErr.Kind {
			case "missing":
				continue
			case "not-text":
				return headingRefused(format, domain.FileUnreadable, path+": holds no text")
			default:
				return headingRefused(format, domain.FileUnreadable, path+": "+readErr.Reason)
			}
		}
		report.Scanned = append(report.Scanned, path)
		report.Inspected++
		inspectHeading(path, text, rules, report)
	}
	return report.Render(format)
}

// decodeHeading projects the declared rules. Both rules are required: a
// repository whose documents are sections of a larger whole legitimately has
// no level-1 heading, and assuming either answer would enforce a structure
// nobody declared.
func decodeHeading(mapping map[string]any) (headingRules, error) {
	var rules headingRules
	for key := range mapping {
		if key != "surfaces" && key != "single-h1" && key != "max-level-jump" {
			return rules, fmt.Errorf("policies.markdown.heading-hierarchy names unknown key `%s`", key)
		}
	}
	rawSurfaces, ok := mapping["surfaces"].([]any)
	if !ok {
		return rules, fmt.Errorf("md-heading-hierarchy.surfaces names no surfaces")
	}
	for _, item := range rawSurfaces {
		entry, ok := item.(map[string]any)
		if !ok {
			return rules, fmt.Errorf("md-heading-hierarchy.surfaces names no surface")
		}
		for key := range entry {
			if key != "glob" {
				return rules, fmt.Errorf("md-heading-hierarchy.surfaces names unknown key `%s`", key)
			}
		}
		glob, _ := entry["glob"].(string)
		if strings.TrimSpace(glob) == "" {
			return rules, fmt.Errorf("md-heading-hierarchy.surfaces names no glob")
		}
		rules.surfaces = append(rules.surfaces, glob)
	}
	single, ok := mapping["single-h1"].(bool)
	if !ok {
		return rules, fmt.Errorf("policies.markdown.heading-hierarchy names no single-h1")
	}
	rules.singleH1 = single
	jump, ok := mapping["max-level-jump"].(int)
	if !ok || jump < 0 {
		return rules, fmt.Errorf("policies.markdown.heading-hierarchy names no max-level-jump")
	}
	rules.maxJump = jump
	return rules, nil
}

func inspectHeading(path, text string, rules headingRules, report *Report) {
	found := collectHeadings(text)
	if rules.singleH1 {
		tops := 0
		for _, item := range found {
			if item.level != 1 {
				continue
			}
			tops++
			if tops == 1 {
				continue
			}
			// Every one after the first, so a document with three reports
			// two extras rather than one ambiguous "there are too many".
			finding := NewFinding("multiple-h1", path, "declares a second level-1 heading")
			at := item.line
			finding.Line = &at
			report.Findings = append(report.Findings, finding)
		}
		if tops == 0 {
			finding := NewFinding("missing-h1", path, "declares no level-1 heading")
			at := 1
			finding.Line = &at
			report.Findings = append(report.Findings, finding)
		}
	}
	// The first heading is the level the rest are measured from. It cannot
	// drop below something that is not there, and a document that opens deep
	// is either missing-h1's business or nobody's.
	previous := -1
	for _, item := range found {
		if previous >= 0 && item.level > previous+rules.maxJump {
			finding := NewFinding("heading-level-jump", path,
				fmt.Sprintf("drops from level %d to level %d", previous, item.level))
			at := item.line
			finding.Line = &at
			jump := item.level - previous
			finding.Details = append(finding.Details, CountDetail("jump", jump))
			report.Findings = append(report.Findings, finding)
		}
		previous = item.level
	}
}

// collectHeadings lists every ATX heading as its line and level. A run of
// hashes is a heading only when a space follows it: #hashtag is a word, and
// treating it as a heading would invent a level-1 heading mid-paragraph.
func collectHeadings(text string) []heading {
	var found []heading
	for _, line := range proseLines(text) {
		trimmed := strings.TrimLeftFunc(line.text, unicode.IsSpace)
		level := 0
		for level < len(trimmed) && trimmed[level] == '#' {
			level++
		}
		if level < 1 || level > 6 || level >= len(trimmed) || trimmed[level] != ' ' {
			continue
		}
		found = append(found, heading{line: line.number, level: level})
	}
	return found
}

// proseLine is one line of prose with its one-based position in the file.
type proseLine struct {
	number int
	text   string
}

// proseLines returns the lines that are prose rather than a fenced example.
// A fence closes on the same character at the same length or longer, which is
// what lets a document show a fenced block inside a fenced block.
func proseLines(text string) []proseLine {
	var openChar rune
	openLen := 0
	var lines []proseLine
	for index, line := range strings.Split(text, "\n") {
		char, length := fenceOf(line)
		switch {
		case openLen == 0 && length > 0:
			openChar, openLen = char, length
		case openLen > 0 && length > 0 && char == openChar && length >= openLen:
			openChar, openLen = 0, 0
		case openLen == 0:
			lines = append(lines, proseLine{number: index + 1, text: line})
		}
	}
	return lines
}

// fenceOf reports the fence character and run length a line opens or closes
// with. Three backticks or three tildes start a fence; anything shorter is
// prose that happens to carry punctuation.
func fenceOf(line string) (rune, int) {
	trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
	var char rune
	for _, first := range trimmed {
		char = first
		break
	}
	if char != '`' && char != '~' {
		return 0, 0
	}
	length := 0
	for _, c := range trimmed {
		if c != char {
			break
		}
		length++
	}
	if length < 3 {
		return 0, 0
	}
	return char, length
}

func headingRefused(format Format, code domain.ErrorCode, reason string) Outcome {
	report := NewReport("heading-hierarchy", "file")
	report.Refusal = &reason
	report.RefusalCode = code
	return report.Render(format)
}
