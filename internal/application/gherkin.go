// Gherkin reading for the behaviour rules. A feature file is read as text: its
// scenarios, the tags above each one, and the line directly above each tag. A
// `.feature.md` file contributes only the lines of its gherkin fences. Nothing
// here runs a scenario or a test.
package application

import (
	"regexp"
	"strings"
)

// scenarioKeyword matches the line that opens a scenario, outline, or example.
var scenarioKeyword = regexp.MustCompile(`^(Scenario Outline|Scenario|Example):\s*(.*)$`)

// scenarioReference matches one `@scenario <feature-path>: <name>` reference. The
// feature path ends at the first `.feature` or `.feature.md` followed by a colon.
var scenarioReference = regexp.MustCompile(`@scenario\s+(\S+?\.feature(?:\.md)?):(.*)$`)

// featureTag is one tag, with the line it stands on.
type featureTag struct {
	Name string
	Line int
}

// featureScenario is one scenario and the tags written directly above it.
type featureScenario struct {
	Name string
	Line int
	Tags []featureTag
}

// featureFile is one parsed feature file. Lines holds the text the scan read, so
// the line directly above a tag can be checked.
type featureFile struct {
	Path       string
	Lines      []string
	HasFeature bool
	Scenarios  []featureScenario
	Blocked    []featureTag
}

// behaviourReference is one `@scenario` line in a test file.
type behaviourReference struct {
	Feature string
	Name    string
	Line    int
}

// isFeatureFile reports whether a path is a feature file the corpus may hold.
func isFeatureFile(path string) bool {
	return strings.HasSuffix(path, ".feature") || strings.HasSuffix(path, ".feature.md")
}

// parseFeature reads one feature file's text into its scenarios and tags.
func parseFeature(path, text string) featureFile {
	lines := strings.Split(text, "\n")
	if strings.HasSuffix(path, ".feature.md") {
		lines = gherkinFences(lines)
	}
	feature := featureFile{Path: path, Lines: lines}
	var pending []featureTag
	inDocstring := false
	for index, raw := range lines {
		line := strings.TrimSpace(raw)
		// A docstring is step text, not Gherkin structure: a scenario-like line
		// inside `"""` delimiters is never a scenario or a tag.
		if strings.HasPrefix(line, `"""`) {
			inDocstring = !inDocstring
			continue
		}
		if inDocstring || line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "@") {
			for _, name := range strings.Fields(line) {
				if strings.HasPrefix(name, "@") {
					pending = append(pending, featureTag{Name: name, Line: index + 1})
				}
			}
			continue
		}
		switch {
		case strings.HasPrefix(line, "Feature:"):
			feature.HasFeature = true
			feature.Blocked = append(feature.Blocked, pending...)
		case strings.HasPrefix(line, "Rule:"), strings.HasPrefix(line, "Background:"):
			feature.Blocked = append(feature.Blocked, pending...)
		default:
			if match := scenarioKeyword.FindStringSubmatch(line); match != nil {
				feature.Scenarios = append(feature.Scenarios, featureScenario{
					Name: strings.TrimSpace(match[2]), Line: index + 1, Tags: pending,
				})
			}
		}
		pending = nil
	}
	return feature
}

// gherkinFences keeps the lines inside ```gherkin fences and blanks the rest, so
// line numbers stay the file's own.
func gherkinFences(lines []string) []string {
	out := make([]string, len(lines))
	inside := false
	for index, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		switch {
		case !inside && trimmed == "```gherkin":
			inside = true
		case inside && trimmed == "```":
			inside = false
		case inside:
			out[index] = raw
		}
	}
	return out
}

// aboveLine returns the trimmed line directly above a 1-based line number, or the
// empty string when there is none.
func (f featureFile) aboveLine(line int) string {
	if line < 2 || line-2 >= len(f.Lines) {
		return ""
	}
	return strings.TrimSpace(f.Lines[line-2])
}

// parseReferences reads every `@scenario` line of one test file.
func parseReferences(text string) []behaviourReference {
	var references []behaviourReference
	for index, line := range strings.Split(text, "\n") {
		match := scenarioReference.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		references = append(references, behaviourReference{
			Feature: match[1], Name: strings.TrimSpace(match[2]), Line: index + 1,
		})
	}
	return references
}
