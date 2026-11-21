// Behaviour coverage rules B01 to B09: every scenario in the corpus is bound to a
// unit test, bound or exempt at the integration and e2e levels, and no reference
// or exemption is stale or malformed. Tree validator: it reads the corpus and the
// test files as text, runs no test, starts no process, and reaches no network.
package application

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// behaviourRules lists the rule identifiers in order.
var behaviourRules = []string{"B01", "B02", "B03", "B04", "B05", "B06", "B07", "B08", "B09"}

// exemptionLevels maps each exemption tag the spec names to the level it exempts.
var exemptionLevels = map[string]string{"@integration-exempt": "integration", "@e2e-exempt": "e2e"}

// exemptionComment matches the comment that must sit directly above an exemption tag.
var exemptionComment = regexp.MustCompile(`^#\s*Exemption\(([^)]*)\):\s*(.*?);\s*alternative-proof:\s*(.*)$`)

// deniedReason matches an exemption reason that names an excuse the spec refuses.
var deniedReason = regexp.MustCompile(`(?i)\b(difficult(y|ies)?|runtime|run[- ]time|flak(y|iness)|costs?|costly|todo|fixme|tbd|wip|unfinished|work in progress|not yet)\b`)

// behaviourTest is one test file that a level glob matches, with the levels it
// matches, whether a whole-file glob matches it, and its reference lines.
type behaviourTest struct {
	Path       string
	Levels     []string
	WholeFile  bool
	References []behaviourReference
}

// behaviourCorpus is everything the behaviour rules read from the tree.
type behaviourCorpus struct {
	corpora    []featureFile
	loose      []string
	tests      []behaviourTest
	levelFiles map[string]int
	scanned    []string
}

// coverageKey names one scenario at one level.
type coverageKey struct {
	feature  string
	scenario string
	level    string
}

// behaviourCoverageValidate runs the rules the rules flag selects, or every rule
// when the flag is absent.
func behaviourCoverageValidate(document *config.Document, tree Tree, rules *string, format Format) Outcome {
	behaviour, declared, err := document.Behaviour()
	if err != nil {
		return reportRefused("behaviour-coverage", format, err.Error())
	}
	if !declared {
		report := NewReport("behaviour-coverage", "scenario")
		report.Refusal = strptr("policies.behaviour is not declared, and RHINO holds no default for it")
		report.RefusalCode = domain.ConfigUndeclared
		return report.Render(format)
	}
	enabled, refusal := selectedRules(rules)
	if refusal != "" {
		report := Refuse("behaviour-coverage", refusal)
		report.RefusalCode = domain.ArgsUnrecognized
		return report.Render(format)
	}
	excluded, err := document.ScanExcludeDirectories()
	if err != nil {
		return reportRefused("behaviour-coverage", format, err.Error())
	}
	corpus, fault := readBehaviour(behaviour, tree, scanFiles(tree.Files(), excluded))
	if fault != nil {
		report := Refuse("behaviour-coverage", fault.reason)
		report.RefusalCode = fault.code
		return report.Render(format)
	}
	report := NewReport("behaviour-coverage", "scenario")
	for _, feature := range corpus.corpora {
		report.Inspected += len(feature.Scenarios)
	}
	report.Scanned = corpus.scanned
	report.Findings = behaviourFindings(document, behaviour, corpus, enabled)
	return renderRuleReport(report, format)
}

// selectedRules reads the --rules flag. Without it every rule runs; with it only
// the named rules run, and an unknown name refuses the run.
func selectedRules(rules *string) (map[string]bool, string) {
	enabled := map[string]bool{}
	if rules == nil {
		for _, rule := range behaviourRules {
			enabled[rule] = true
		}
		return enabled, ""
	}
	for _, name := range strings.Split(*rules, ",") {
		rule := strings.TrimSpace(name)
		if !contains(behaviourRules, rule) {
			return nil, fmt.Sprintf("`--rules` names no rule `%s`; expected B01 to B09, separated by commas", rule)
		}
		enabled[rule] = true
	}
	return enabled, ""
}

// readBehaviour reads the corpus features, the loose feature files, and the test
// files each level matches. A glob that cannot be matched refuses the run before
// any file is read.
func readBehaviour(behaviour *config.Behaviour, tree Tree, files []string) (*behaviourCorpus, *ruleFault) {
	globs := append([]string{}, behaviour.WholeFile...)
	for _, level := range config.BehaviourLevels {
		globs = append(globs, behaviour.Levels[level]...)
	}
	for _, glob := range globs {
		if _, err := Match(glob, ""); err != nil {
			return nil, &ruleFault{code: domain.ConfigUnusable, reason: config.Path + ": policies.behaviour: " + err.Error()}
		}
	}
	corpus := &behaviourCorpus{levelFiles: map[string]int{}}
	corpusRoots := prefixesOf(behaviour.Corpus)
	fixtureRoots := prefixesOf(behaviour.Fixtures)
	var corpusPaths []string
	for _, file := range files {
		if !isFeatureFile(file) || hasPrefixIn(file, fixtureRoots) {
			continue
		}
		if hasPrefixIn(file, corpusRoots) {
			corpusPaths = append(corpusPaths, file)
		} else {
			corpus.loose = append(corpus.loose, file)
		}
	}
	for _, path := range corpusPaths {
		text, fault := readRuleFile(tree, path)
		if fault != nil {
			return nil, fault
		}
		feature := parseFeature(path, text)
		if !feature.HasFeature {
			return nil, &ruleFault{code: domain.FileUnreadable, reason: path + ": holds no Feature line"}
		}
		corpus.corpora = append(corpus.corpora, feature)
		corpus.scanned = append(corpus.scanned, path)
	}
	tests := map[string]*behaviourTest{}
	var order []string
	for _, level := range config.BehaviourLevels {
		for _, file := range files {
			if isFeatureFile(file) || !matchesAnyGlob(behaviour.Levels[level], file) {
				continue
			}
			corpus.levelFiles[level]++
			test, seen := tests[file]
			if !seen {
				test = &behaviourTest{Path: file, WholeFile: matchesAnyGlob(behaviour.WholeFile, file)}
				tests[file] = test
				order = append(order, file)
			}
			test.Levels = append(test.Levels, level)
		}
	}
	for _, path := range order {
		text, fault := readRuleFile(tree, path)
		if fault != nil {
			return nil, fault
		}
		test := tests[path]
		test.References = parseReferences(text)
		corpus.tests = append(corpus.tests, *test)
		corpus.scanned = append(corpus.scanned, path)
	}
	return corpus, nil
}

// matchesAnyGlob reports whether a file matches one of the globs. Every glob was
// checked by readBehaviour, so a match cannot fail on a bad pattern.
func matchesAnyGlob(globs []string, file string) bool {
	matched := false
	for _, glob := range globs {
		if ok, _ := Match(glob, file); ok {
			matched = true
		}
	}
	return matched
}

// behaviourFindings evaluates every enabled rule over the corpus and the tests.
func behaviourFindings(document *config.Document, behaviour *config.Behaviour, corpus *behaviourCorpus, enabled map[string]bool) []Finding {
	features := map[string]featureFile{}
	for _, feature := range corpus.corpora {
		features[feature.Path] = feature
	}
	covered := coveredScenarios(corpus.tests, features)
	var findings []Finding
	for _, feature := range corpus.corpora {
		findings = append(findings, scenarioFindings(feature, covered, enabled)...)
		if enabled["B08"] {
			findings = append(findings, duplicateFindings(feature)...)
		}
	}
	if enabled["B05"] {
		findings = append(findings, referenceFindings(corpus.tests, features)...)
	}
	if enabled["B07"] {
		for _, path := range corpus.loose {
			findings = append(findings, behaviourFinding("B07", path, 0,
				"feature file sits outside the corpus roots and the fixtures roots"))
		}
	}
	if enabled["B09"] {
		findings = append(findings, emptyFindings(document, behaviour, corpus)...)
	}
	return findings
}

// coveredScenarios indexes the scenario bindings the test files make. A
// reference counts only when it names a corpus feature, and a whole-file `*`
// counts only in a test file a whole-file glob matches (P13).
func coveredScenarios(tests []behaviourTest, features map[string]featureFile) map[coverageKey]bool {
	covered := map[coverageKey]bool{}
	for _, test := range tests {
		for _, reference := range test.References {
			feature, known := features[reference.Feature]
			if !known {
				continue
			}
			for _, level := range test.Levels {
				if reference.Name != "*" {
					covered[coverageKey{reference.Feature, reference.Name, level}] = true
					continue
				}
				if !test.WholeFile {
					continue
				}
				for _, scenario := range feature.Scenarios {
					covered[coverageKey{reference.Feature, scenario.Name, level}] = true
				}
			}
		}
	}
	return covered
}

// scenarioFindings evaluates B01, B02, B03, B04, and B06 for every scenario of a
// feature, and B04 for an exemption tag above a Feature, Rule, or Background.
func scenarioFindings(feature featureFile, covered map[coverageKey]bool, enabled map[string]bool) []Finding {
	var findings []Finding
	if enabled["B04"] {
		for _, tag := range feature.Blocked {
			if strings.HasSuffix(tag.Name, "-exempt") {
				findings = append(findings, behaviourFinding("B04", feature.Path, tag.Line,
					fmt.Sprintf("exemption tag `%s` sits on a Feature, Rule, or Background; it belongs on one scenario", tag.Name)))
			}
		}
	}
	for _, scenario := range feature.Scenarios {
		valid := map[string]int{}
		for _, tag := range scenario.Tags {
			level, problem := exemptionProblem(feature, tag)
			if problem != "" {
				if enabled["B04"] {
					findings = append(findings, behaviourFinding("B04", feature.Path, tag.Line, problem))
				}
				continue
			}
			if level != "" {
				valid[level] = tag.Line
			}
		}
		findings = append(findings, bindingFindings(feature, scenario, valid, covered, enabled)...)
	}
	return findings
}

// bindingFindings evaluates the coverage rules for one scenario: B01 needs a unit
// reference, B02 and B03 need a reference or a valid exemption at their level, and
// B06 fails a valid exemption whose level a reference also covers.
func bindingFindings(feature featureFile, scenario featureScenario, valid map[string]int, covered map[coverageKey]bool, enabled map[string]bool) []Finding {
	var findings []Finding
	bound := func(level string) bool {
		return covered[coverageKey{feature.Path, scenario.Name, level}]
	}
	if enabled["B01"] && !bound("unit") {
		findings = append(findings, behaviourFinding("B01", feature.Path, scenario.Line,
			fmt.Sprintf("scenario `%s` has no `@scenario` reference from a unit test file", scenario.Name)))
	}
	for _, level := range []string{"integration", "e2e"} {
		rule := "B02"
		if level == "e2e" {
			rule = "B03"
		}
		if enabled[rule] && !bound(level) && valid[level] == 0 {
			findings = append(findings, behaviourFinding(rule, feature.Path, scenario.Line,
				fmt.Sprintf("scenario `%s` has no %s reference and no valid exemption at that level", scenario.Name, level)))
		}
		if enabled["B06"] && valid[level] > 0 && bound(level) {
			findings = append(findings, behaviourFinding("B06", feature.Path, valid[level],
				fmt.Sprintf("stale exemption — remove the tag: scenario `%s` is exempt at the %s level and also referenced there",
					scenario.Name, level)))
		}
	}
	return findings
}

// exemptionProblem checks one exemption tag. It returns the level the tag exempts
// when the tag is valid, or a problem when it is not; a tag that is not an
// exemption returns neither.
func exemptionProblem(feature featureFile, tag featureTag) (string, string) {
	level, known := exemptionLevels[tag.Name]
	if !known {
		if !strings.HasSuffix(tag.Name, "-exempt") {
			return "", ""
		}
		return "", fmt.Sprintf("`%s` is an unknown exemption tag; the exemption tags are `@integration-exempt` and `@e2e-exempt`", tag.Name)
	}
	match := exemptionComment.FindStringSubmatch(feature.aboveLine(tag.Line))
	if match == nil {
		return "", fmt.Sprintf("exemption tag `%s` has no `# Exemption(%s): <reason>; alternative-proof: <target>` line directly above it",
			tag.Name, level)
	}
	if match[1] != level {
		return "", fmt.Sprintf("the comment above `%s` names level `%s`, not `%s`", tag.Name, match[1], level)
	}
	reason, target := strings.TrimSpace(match[2]), strings.TrimSpace(match[3])
	if reason == "" || target == "" {
		return "", fmt.Sprintf("the exemption above `%s` names no reason or no alternative-proof target", tag.Name)
	}
	if deniedReason.MatchString(reason) {
		return "", fmt.Sprintf("the exemption reason above `%s` is on the deny list", tag.Name)
	}
	return level, ""
}

// duplicateFindings is B08: a scenario name that a feature file uses twice.
func duplicateFindings(feature featureFile) []Finding {
	var findings []Finding
	seen := map[string]bool{}
	for _, scenario := range feature.Scenarios {
		if seen[scenario.Name] {
			findings = append(findings, behaviourFinding("B08", feature.Path, scenario.Line,
				fmt.Sprintf("scenario `%s` is named twice in this feature file", scenario.Name)))
		}
		seen[scenario.Name] = true
	}
	return findings
}

// referenceFindings is B05: a reference that names no scenario, a feature that is
// not a corpus feature, a scenario the feature does not hold, or a `*` outside a
// whole-file glob.
func referenceFindings(tests []behaviourTest, features map[string]featureFile) []Finding {
	var findings []Finding
	for _, test := range tests {
		for _, reference := range test.References {
			if message := referenceProblem(test, reference, features); message != "" {
				findings = append(findings, behaviourFinding("B05", test.Path, reference.Line, message))
			}
		}
	}
	return findings
}

// referenceProblem returns why a reference is invalid, or the empty string.
func referenceProblem(test behaviourTest, reference behaviourReference, features map[string]featureFile) string {
	feature, known := features[reference.Feature]
	switch {
	case reference.Name == "":
		return fmt.Sprintf("the reference to `%s` names no scenario", reference.Feature)
	case !known:
		return fmt.Sprintf("the reference names feature `%s`, which is not a corpus feature", reference.Feature)
	case reference.Name == "*" && !test.WholeFile:
		return fmt.Sprintf("`*` names every scenario of `%s`, but no `policies.behaviour.whole-file` glob matches `%s`",
			reference.Feature, test.Path)
	case reference.Name != "*" && !hasScenario(feature, reference.Name):
		return fmt.Sprintf("the reference names scenario `%s`, which `%s` does not hold", reference.Name, reference.Feature)
	}
	return ""
}

// emptyFindings is B09: a corpus root that holds no scenario, or a level whose
// globs match no test file. Nothing inspected is not a pass.
func emptyFindings(document *config.Document, behaviour *config.Behaviour, corpus *behaviourCorpus) []Finding {
	var findings []Finding
	for _, root := range behaviour.Corpus {
		if !holdsScenario(corpus.corpora, root) {
			finding := behaviourFinding("B09", root, 0,
				fmt.Sprintf("corpus `%s` holds no scenario; nothing inspected is not a pass", root))
			field := "policies.behaviour.corpus"
			finding.Field = &field
			findings = append(findings, finding)
		}
	}
	for _, level := range config.BehaviourLevels {
		if corpus.levelFiles[level] > 0 {
			continue
		}
		finding := behaviourFinding("B09", config.Path,
			document.LineOf("policies", "behaviour", "levels", level),
			fmt.Sprintf("level `%s` matches no test file; nothing at that level is inspected, and that is not a pass", level))
		field := "policies.behaviour.levels." + level
		finding.Field = &field
		findings = append(findings, finding)
	}
	return findings
}

// behaviourFinding builds one finding; a line of zero means the finding has no
// line.
func behaviourFinding(rule, path string, line int, message string) Finding {
	finding := Finding{Kind: rule, Path: path, Message: message}
	if line > 0 {
		finding.Line = intPtr(line)
	}
	return finding
}

// hasScenario reports whether a feature holds a scenario of that name.
func hasScenario(feature featureFile, name string) bool {
	for _, scenario := range feature.Scenarios {
		if scenario.Name == name {
			return true
		}
	}
	return false
}

// holdsScenario reports whether any corpus feature under a root holds a scenario.
func holdsScenario(features []featureFile, root string) bool {
	prefix := directoryPrefix(root)
	for _, feature := range features {
		if strings.HasPrefix(feature.Path, prefix) && len(feature.Scenarios) > 0 {
			return true
		}
	}
	return false
}

// prefixesOf turns repository directories into the prefixes of the files under them.
func prefixesOf(directories []string) []string {
	prefixes := make([]string, 0, len(directories))
	for _, directory := range directories {
		prefixes = append(prefixes, directoryPrefix(directory))
	}
	return prefixes
}

// hasPrefixIn reports whether a path lies under one of the prefixes.
func hasPrefixIn(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
