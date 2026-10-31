// Quality-gates validation: the declared workflow layout, headings, verdicts,
// and cycle ceiling.
//
// A repository that runs bounded quality gates declares where its workflows
// live and what its gate contract names. This validator checks only the
// structure that declaration implies: where each gate, propagation, and
// workflow module sits, which headings each gate and propagation carries, the
// verdicts a gate names, and that no gate can run more than three cycles.
// Whether a gate is well written is the gate's own judgement, never RHINO's.
//
// Each finding carries the rule it answers to, `QG01` to `QG11`, as its
// `rule` detail. A declaration this check could not apply as written is
// refused before any file is read: `QG09`, a configured cycle ceiling outside
// 1 to 3, while the configuration is read, and the rest by this check itself.
package application

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// Quality-gate structure constants: the category, the file suffixes a gate or
// propagation file carries, the workflow index name, and the cycle input with
// its ceiling of three.
const (
	qualityGatesCategory   = "quality-gates"
	qualityGateSuffix      = "-quality-gate.md"
	qualityPropagationSfx  = "-propagation.md"
	qualityIndexName       = "README.md"
	qualityCycleInput      = "max-cycles"
	qualityCycleFloor      = 1
	qualityCycleCeilingMax = 3
)

// qualityGateFamily is one declared gate family with its optional overrides.
type qualityGateFamily struct {
	family   string
	subject  string
	judge    string
	repairer string
	hasJudge bool
	hasFixer bool
}

// judgeAgent names the agent judging this family's gate.
func (f qualityGateFamily) judgeAgent() string {
	if f.hasJudge {
		return f.judge
	}
	return f.family + "-checker"
}

// repairAgent names the agent repairing this family's gate findings.
func (f qualityGateFamily) repairAgent() string {
	if f.hasFixer {
		return f.repairer
	}
	return f.family + "-fixer"
}

// qualityGatesPolicy is the declared workflow layout and gate contract.
type qualityGatesPolicy struct {
	root               string
	agents             string
	groups             []string
	gateGroup          string
	gateHeadings       []string
	verdictHeading     string
	verdicts           []string
	retiredInputs      []string
	propagationHeading []string
	gates              []qualityGateFamily
}

// wordBudgetRule is what the repository means by a word.
type wordBudgetRule int

const (
	wordsLettersDigits wordBudgetRule = iota
	wordsWhitespace
)

// wordBudgetSurface is one declared surface with its failure limit.
type wordBudgetSurface struct {
	glob string
	fail int
}

// wordBudget is the declared budget the split-workflow check measures
// against when the repository declares one.
type wordBudget struct {
	rule     wordBudgetRule
	surfaces []wordBudgetSurface
}

func qualityGatesValidate(document *config.Document, tree Tree, format Format) Outcome {
	mapping, ok := document.PolicyGroup("governance", "quality-gates")
	if !ok {
		report := NewReport(qualityGatesCategory, "file")
		report.Refusal = strptr("policies.governance.quality-gates is not declared, and RHINO holds no default for it")
		report.RefusalCode = domain.ConfigUndeclared
		return report.Render(format)
	}
	policy, refusal := decodeQualityGates(mapping)
	if refusal != nil {
		return qualityGatesRefused(format, domain.ConfigUnusable, refusal.Error())
	}
	var budget *wordBudget
	if raw, present := document.PolicyGroup("governance", "word-budget"); present {
		decoded, err := decodeWordBudget(raw)
		if err != nil {
			return qualityGatesRefused(format, domain.ConfigUnusable, err.Error())
		}
		budget = decoded
	}
	report, refusalReason, refusalCode := auditQualityGates(tree, policy, budget)
	if refusalReason != nil {
		return qualityGatesRefused(format, refusalCode, *refusalReason)
	}
	return report.Render(format)
}

// declarationKey is the dotted policy path every declaration refusal names.
const declarationKey = "policies.governance.quality-gates"

// declarationRefusal prefixes a declaration reason the way a configuration
// read error reads: file, line, key, then reason.
func declarationRefusal(reason string) error {
	return fmt.Errorf("%s: line 1: %s: %s", config.Path, declarationKey, reason)
}

// checkQualityGatesDeclaration applies the declaration rules `repo-config
// validate` owns. Commands that never read the key keep the result they had.
func checkQualityGatesDeclaration(document *config.Document) error {
	mapping, ok := document.PolicyGroup("governance", "quality-gates")
	if !ok {
		return nil
	}
	_, err := decodeQualityGates(mapping)
	return err
}

// decodeQualityGates projects the declared gate vocabulary. Unknown keys
// refuse: a policy the tool silently ignored would be a check that never ran.
func decodeQualityGates(mapping map[string]any) (qualityGatesPolicy, error) {
	var policy qualityGatesPolicy
	for key := range mapping {
		switch key {
		case "root", "agents", "groups", "gate-group", "gate-headings",
			"verdict-heading", "verdicts", "retired-inputs",
			"propagation-headings", "defaults", "gates":
		default:
			return qualityGatesPolicy{}, fmt.Errorf("policies.governance.quality-gates names unknown key `%s`", key)
		}
	}
	var err error
	if policy.root, err = qualityStringField(mapping, "root", "workflow root"); err != nil {
		return qualityGatesPolicy{}, err
	}
	if policy.agents, err = qualityStringField(mapping, "agents", "agent directory"); err != nil {
		return qualityGatesPolicy{}, err
	}
	if policy.groups, err = qualityStringList(mapping, "groups", "workflow groups"); err != nil {
		return qualityGatesPolicy{}, err
	}
	if policy.gateGroup, err = qualityStringField(mapping, "gate-group", "gate group"); err != nil {
		return qualityGatesPolicy{}, err
	}
	if policy.gateHeadings, err = qualityStringList(mapping, "gate-headings", "gate headings"); err != nil {
		return qualityGatesPolicy{}, err
	}
	if policy.verdictHeading, err = qualityStringField(mapping, "verdict-heading", "verdict heading"); err != nil {
		return qualityGatesPolicy{}, err
	}
	if policy.verdicts, err = qualityStringList(mapping, "verdicts", "gate verdicts"); err != nil {
		return qualityGatesPolicy{}, err
	}
	if policy.propagationHeading, err = qualityStringList(mapping, "propagation-headings", "propagation headings"); err != nil {
		return qualityGatesPolicy{}, err
	}
	if policy.retiredInputs, err = qualityOptionalStringList(mapping, "retired-inputs", "retired inputs"); err != nil {
		return qualityGatesPolicy{}, err
	}
	if err = checkQualityDefaults(mapping["defaults"]); err != nil {
		return qualityGatesPolicy{}, err
	}
	if policy.gates, err = decodeQualityFamilies(mapping["gates"]); err != nil {
		return qualityGatesPolicy{}, err
	}
	if err = checkQualityDeclaration(policy); err != nil {
		return qualityGatesPolicy{}, err
	}
	return policy, nil
}

// qualityStringField projects one required declared string.
func qualityStringField(mapping map[string]any, key, what string) (string, error) {
	value, ok := mapping[key].(string)
	if !ok {
		return "", fmt.Errorf("quality-gates.%s names no %s", key, what)
	}
	return value, nil
}

// qualityStringList projects one required declared list of strings.
func qualityStringList(mapping map[string]any, key, what string) ([]string, error) {
	raw, ok := mapping[key].([]any)
	if !ok {
		return nil, fmt.Errorf("quality-gates.%s names no %s", key, what)
	}
	var out []string
	for _, item := range raw {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("quality-gates.%s names no %s", key, what)
		}
		out = append(out, text)
	}
	return out, nil
}

// qualityOptionalStringList projects one optional declared list of strings.
func qualityOptionalStringList(mapping map[string]any, key, what string) ([]string, error) {
	raw, present := mapping[key]
	if !present || raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("quality-gates.%s names no %s", key, what)
	}
	var out []string
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("quality-gates.%s names no %s", key, what)
		}
		out = append(out, text)
	}
	return out, nil
}

// checkQualityDefaults validates the optional run defaults without keeping
// them: the mode never changes what a gate file may declare, and the cycle
// ceiling was already refused while the configuration was read.
func checkQualityDefaults(raw any) error {
	if raw == nil {
		return nil
	}
	mapping, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("quality-gates.defaults names no run defaults")
	}
	for key := range mapping {
		if key != "mode" && key != "max-cycles" {
			return fmt.Errorf("quality-gates.defaults names unknown key `%s`", key)
		}
	}
	if mode, present := mapping["mode"]; present && mode != nil {
		switch mode {
		case "lax", "normal", "strict", "all":
		default:
			return fmt.Errorf("quality-gates.defaults.mode names no gate mode")
		}
	}
	if ceiling, present := mapping["max-cycles"]; present && ceiling != nil {
		cycles, ok := ceiling.(int)
		if !ok || cycles < qualityCycleFloor || cycles > qualityCycleCeilingMax {
			return fmt.Errorf("%s: line 1: %s.defaults.max-cycles: must be 1, 2, or 3; a gate never runs more than three cycles",
				config.Path, declarationKey)
		}
	}
	return nil
}

// decodeQualityFamilies projects the declared gate families. An entry that
// is not a mapping, or that names no family, reads as empty and fails the
// declaration check below: the run still refuses, with the one message that
// describes what a family has to be.
func decodeQualityFamilies(raw any) ([]qualityGateFamily, error) {
	if raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("quality-gates.gates names no gate families")
	}
	families := make([]qualityGateFamily, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			families = append(families, qualityGateFamily{})
			continue
		}
		var family qualityGateFamily
		for key := range entry {
			if key != "family" && key != "subject" && key != "judge" && key != "repairer" {
				return nil, fmt.Errorf("quality-gates.gates: unknown field `%s`, expected one of `family`, `subject`, `judge`, `repairer`", key)
			}
		}
		family.family, _ = entry["family"].(string)
		family.subject, _ = entry["subject"].(string)
		if judge, present := entry["judge"]; present && judge != nil {
			text, ok := judge.(string)
			if !ok {
				return nil, fmt.Errorf("quality-gates.gates.judge names no agent")
			}
			family.judge, family.hasJudge = text, true
		}
		if repairer, present := entry["repairer"]; present && repairer != nil {
			text, ok := repairer.(string)
			if !ok {
				return nil, fmt.Errorf("quality-gates.gates.repairer names no agent")
			}
			family.repairer, family.hasFixer = text, true
		}
		if subject, present := entry["subject"]; present && subject != nil {
			if _, ok := subject.(string); !ok {
				return nil, fmt.Errorf("quality-gates.gates.subject names no subject")
			}
		}
		families = append(families, family)
	}
	return families, nil
}

// checkQualityDeclaration applies the declaration rules the structure check
// needs before it can read a file.
func checkQualityDeclaration(policy qualityGatesPolicy) error {
	if !isExactPath(policy.root) || !isExactPath(policy.agents) {
		return declarationRefusal("`root` and `agents` must be exact repository-relative paths")
	}
	if !uniqueSimpleNames(policy.groups) || !containsName(policy.groups, policy.gateGroup) {
		return declarationRefusal("`groups` must be unique simple names that include `gate-group`")
	}
	families := make([]string, 0, len(policy.gates))
	for _, gate := range policy.gates {
		families = append(families, gate.family)
	}
	if !uniqueSimpleNames(families) {
		return declarationRefusal("every `gates[].family` must be a unique simple name")
	}
	for _, gate := range policy.gates {
		if gate.hasJudge && !isLayerSimpleName(gate.judge) {
			return declarationRefusal("every `gates[].judge` and `gates[].repairer` must be a simple agent name")
		}
		if gate.hasFixer && !isLayerSimpleName(gate.repairer) {
			return declarationRefusal("every `gates[].judge` and `gates[].repairer` must be a simple agent name")
		}
	}
	if !uniqueText(policy.gateHeadings) || !uniqueText(policy.propagationHeading) ||
		!uniqueText(policy.verdicts) || hasBlank(policy.retiredInputs) {
		return declarationRefusal("headings, verdicts, and retired inputs must be non-empty and unique")
	}
	if !containsName(policy.gateHeadings, policy.verdictHeading) {
		return declarationRefusal("`verdict-heading` must be one of `gate-headings`")
	}
	return nil
}

// uniqueSimpleNames answers whether every name is a simple name exactly once.
func uniqueSimpleNames(names []string) bool {
	seen := map[string]bool{}
	for _, name := range names {
		if !isLayerSimpleName(name) || seen[name] {
			return false
		}
		seen[name] = true
	}
	return true
}

// containsName answers whether a declared list names a value.
func containsName(names []string, value string) bool {
	for _, name := range names {
		if name == value {
			return true
		}
	}
	return false
}

// uniqueText answers whether every value is non-blank exactly once.
func uniqueText(values []string) bool {
	if len(values) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, value := range values {
		if strings.TrimSpace(value) == "" || seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}

// hasBlank answers whether any value is blank.
func hasBlank(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return true
		}
	}
	return false
}

// decodeWordBudget projects the declared word budget the split-workflow
// check measures against. Unknown keys refuse, as they do everywhere else.
func decodeWordBudget(mapping map[string]any) (*wordBudget, error) {
	for key := range mapping {
		if key != "count" && key != "surfaces" {
			return nil, fmt.Errorf("policies.governance.word-budget names unknown key `%s`", key)
		}
	}
	budget := &wordBudget{}
	switch rule, _ := mapping["count"].(string); rule {
	case "letters-and-digits":
		budget.rule = wordsLettersDigits
	case "whitespace-separated":
		budget.rule = wordsWhitespace
	default:
		return nil, fmt.Errorf("word-budget.count names no word rule")
	}
	raw, present := mapping["surfaces"]
	if !present || raw == nil {
		return budget, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("word-budget.surfaces names no budget surfaces")
	}
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("word-budget.surfaces names no budget surfaces")
		}
		for key := range entry {
			if key != "glob" && key != "fail" && key != "warn" && key != "target" {
				return nil, fmt.Errorf("word-budget.surfaces name unknown key `%s`", key)
			}
		}
		glob, ok := entry["glob"].(string)
		if !ok {
			return nil, fmt.Errorf("word-budget.surfaces names no budget surfaces")
		}
		fail, ok := entry["fail"].(int)
		if !ok {
			return nil, fmt.Errorf("word-budget.surfaces names no budget surfaces")
		}
		budget.surfaces = append(budget.surfaces, wordBudgetSurface{glob: glob, fail: fail})
	}
	for _, surface := range budget.surfaces {
		// Validated against a probe path: a pattern this check cannot
		// compile would otherwise quietly govern nothing.
		if _, err := Match(surface.glob, "probe"); err != nil {
			return nil, fmt.Errorf("governance-word-budget.surfaces: %s", err.Error())
		}
	}
	return budget, nil
}

// limit answers the last declared surface limit governing a path: where two
// globs match one file, the last matching entry wins, which is how a specific
// surface overrides a general tree.
func (b *wordBudget) limit(path string) *int {
	var governed *int
	for _, surface := range b.surfaces {
		ok, err := Match(surface.glob, path)
		if err != nil {
			continue
		}
		if ok {
			fail := surface.fail
			governed = &fail
		}
	}
	return governed
}

// wordPattern is what this repository means by a word when it counts runs of
// letters, marks, and digits, optionally joined by an apostrophe, hyphen, or
// underscore to another such run.
var wordPattern = regexp.MustCompile(`[\p{L}\p{M}\p{N}]+(?:['’_-][\p{L}\p{M}\p{N}]+)*`)

// countWords counts the words a text holds by the repository's own rule.
func (b *wordBudget) countWords(text string) int {
	if b.rule == wordsWhitespace {
		return len(strings.Fields(text))
	}
	return len(wordPattern.FindAllString(text, -1))
}

func qualityGatesRefused(format Format, code domain.ErrorCode, reason string) Outcome {
	report := NewReport(qualityGatesCategory, "file")
	report.Refusal = &reason
	report.RefusalCode = code
	return report.Render(format)
}

// qualityAudit is one run over the declared workflow layout.
type qualityAudit struct {
	tree   Tree
	policy qualityGatesPolicy
	budget *wordBudget
	report *Report
	// scanned holds every path already listed as scanned: a gate that is
	// also a split workflow's entrypoint is read by two rules but
	// inspected once.
	scanned map[string]bool
}

// auditQualityGates checks the declared layout. A refusal reason with its
// code means the run could not happen at all.
func auditQualityGates(tree Tree, policy qualityGatesPolicy, budget *wordBudget) (*Report, *string, domain.ErrorCode) {
	for _, declared := range []string{policy.root, policy.agents} {
		if tree.PassesThroughLink(declared) {
			reason := declared + ": passes through a symbolic link, which RHINO never follows"
			return nil, &reason, domain.FileUnreadable
		}
	}
	audit := &qualityAudit{
		tree:    tree,
		policy:  policy,
		budget:  budget,
		report:  NewReport(qualityGatesCategory, "file"),
		scanned: map[string]bool{},
	}
	audit.workflowRoot()
	if reason, code := audit.moduleDirectories(); reason != nil {
		return nil, reason, code
	}
	audit.placement()
	audit.families()
	if reason, code := audit.gates(); reason != nil {
		return nil, reason, code
	}
	if reason, code := audit.propagations(); reason != nil {
		return nil, reason, code
	}
	return audit.report, nil, domain.ConfigUnusable
}

// group names one declared workflow group under the workflow root.
func (a *qualityAudit) group(name string) string {
	return a.policy.root + "/" + name
}

// gateDirectory names the one group holding every gate and propagation.
func (a *qualityAudit) gateDirectory() string {
	return a.group(a.policy.gateGroup)
}

// present answers whether a path reads as a file: an unreadable file is
// present but unread, never absent.
func (a *qualityAudit) present(path string) bool {
	_, readErr := a.tree.Read(path)
	return readErr == nil || readErr.Kind != "missing"
}

// read reads one file as text, listing it as scanned exactly once.
func (a *qualityAudit) read(path string) (string, *string, domain.ErrorCode) {
	text, readErr := a.tree.Read(path)
	if readErr != nil {
		var reason string
		switch readErr.Kind {
		case "missing":
			reason = path + ": vanished while it was read"
		case "not-text":
			reason = path + ": holds no text"
		default:
			reason = path + ": " + readErr.Reason
		}
		code := domain.FileUnreadable
		return "", &reason, code
	}
	if !a.scanned[path] {
		a.scanned[path] = true
		a.report.Scanned = append(a.report.Scanned, path)
		a.report.Inspected++
	}
	return text, nil, domain.ConfigUnusable
}

// found attaches the rule a finding answers to, then records it.
func (a *qualityAudit) found(rule string, finding Finding) {
	finding.Details = append(finding.Details, TextDetail("rule", rule))
	a.report.Findings = append(a.report.Findings, finding)
}

// workflowRoot applies QG01: the workflow root holds its index and the
// declared groups only.
func (a *qualityAudit) workflowRoot() {
	for _, entry := range a.tree.Children(a.policy.root) {
		name := baseName(entry)
		allowed := name == qualityIndexName
		if a.tree.IsDirectory(entry) {
			allowed = containsName(a.policy.groups, name)
		}
		if !allowed {
			a.found("QG01", NewFinding("unexpected-workflow-entry", entry,
				"the workflow root holds only its README and the declared groups"))
		}
	}
}

// moduleDirectories applies QG10 and QG11: every directory inside a group is
// one workflow's modules, beside that workflow's entrypoint and listed by
// its index.
func (a *qualityAudit) moduleDirectories() (*string, domain.ErrorCode) {
	for _, group := range a.policy.groups {
		for _, directory := range a.tree.Children(a.group(group)) {
			if a.tree.IsDirectory(directory) {
				if reason, code := a.moduleDirectory(directory); reason != nil {
					return reason, code
				}
			}
		}
	}
	return nil, domain.ConfigUnusable
}

func (a *qualityAudit) moduleDirectory(directory string) (*string, domain.ErrorCode) {
	entrypoint := directory + ".md"
	paired := a.present(entrypoint)
	if !paired {
		a.found("QG10", NewFinding("unpaired-workflow-module-directory", directory,
			"a module directory sits beside a workflow file of the same name"))
	}
	var modules []string
	indexed := false
	for _, entry := range a.tree.Children(directory) {
		file := !a.tree.IsDirectory(entry)
		name := baseName(entry)
		if file && name == qualityIndexName {
			indexed = true
		} else if file && isModuleName(name) {
			modules = append(modules, entry)
		} else {
			a.found("QG10", NewFinding("unexpected-workflow-module-entry", entry,
				"a module directory holds only README.md and NNN-*.md modules"))
		}
	}
	sort.Strings(modules)
	index := directory + "/" + qualityIndexName
	if indexed {
		text, reason, code := a.read(index)
		if reason != nil {
			return reason, code
		}
		listed := readmeTargets(index, text)
		for _, module := range modules {
			if !listed[module] {
				a.found("QG10", NewFinding("unlisted-workflow-module", module,
					"the module directory's README does not list this module"))
			}
		}
	} else {
		a.found("QG10", NewFinding("missing-workflow-module-index", index,
			"a module directory carries a README listing its modules"))
	}
	if a.budget != nil && paired && len(modules) > 0 {
		if limit := a.budget.limit(entrypoint); limit != nil {
			words := 0
			for _, path := range append([]string{entrypoint}, modules...) {
				text, reason, code := a.read(path)
				if reason != nil {
					return reason, code
				}
				words += a.budget.countWords(text)
			}
			if words <= *limit {
				finding := NewFinding("unnecessary-workflow-split", entrypoint,
					"the workflow and its modules fit the word budget as one file")
				finding.Details = append(finding.Details, CountDetail("words", words))
				finding.Details = append(finding.Details, CountDetail("limit", *limit))
				a.found("QG11", finding)
			}
		}
	}
	return nil, domain.ConfigUnusable
}

// placement applies QG03: a gate or propagation sits directly in the gate
// group, and only for a declared family.
func (a *qualityAudit) placement() {
	gateDirectory := a.gateDirectory()
	prefix := a.policy.root + "/"
	declared := a.declared()
	for _, path := range a.tree.Files() {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		family, ok := gateFamily(path)
		if !ok {
			continue
		}
		if parentDir(path) != gateDirectory {
			a.found("QG03", NewFinding("misplaced-quality-gate-file", path,
				"every gate and propagation sits directly in the gate group"))
		} else if !declared[family] {
			finding := NewFinding("undeclared-quality-gate-family", path, "the gate family is not declared")
			finding.Details = append(finding.Details, TextDetail("family", family))
			a.found("QG03", finding)
		}
	}
}

// declared lists every declared gate family.
func (a *qualityAudit) declared() map[string]bool {
	declared := map[string]bool{}
	for _, gate := range a.policy.gates {
		declared[gate.family] = true
	}
	return declared
}

// families applies QG02 and QG08: each declared family holds its gate, its
// propagation, its judge, and its repairer.
func (a *qualityAudit) families() {
	gateDirectory := a.gateDirectory()
	ordered := append([]qualityGateFamily(nil), a.policy.gates...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].family < ordered[j].family })
	for _, gate := range ordered {
		for _, suffix := range []string{qualityGateSuffix, qualityPropagationSfx} {
			path := gateDirectory + "/" + gate.family + suffix
			if !a.present(path) {
				finding := NewFinding("missing-quality-gate-file", path, "a declared gate family has no file here")
				finding.Details = append(finding.Details, TextDetail("family", gate.family))
				a.found("QG02", finding)
			}
		}
		roles := []struct {
			role  string
			agent string
		}{
			{"judge", gate.judgeAgent()},
			{"repairer", gate.repairAgent()},
		}
		for _, role := range roles {
			path := a.policy.agents + "/" + role.agent + ".md"
			if !a.present(path) {
				finding := NewFinding("missing-gate-agent", path, "a declared gate family has no agent here")
				finding.Details = append(finding.Details, TextDetail("family", gate.family))
				finding.Details = append(finding.Details, TextDetail("role", role.role))
				finding.Details = append(finding.Details, TextDetail("agent", role.agent))
				a.found("QG08", finding)
			}
		}
	}
}

// gateFiles lists the gate or propagation files directly in the gate group.
func (a *qualityAudit) gateFiles(suffix string) []string {
	var out []string
	for _, path := range a.tree.Children(a.gateDirectory()) {
		if a.tree.IsDirectory(path) {
			continue
		}
		if _, ok := gateFamily(path); !ok || !strings.HasSuffix(path, suffix) {
			continue
		}
		out = append(out, path)
	}
	return out
}

// gates applies QG04, QG06, and QG07, for every gate in the gate group.
func (a *qualityAudit) gates() (*string, domain.ErrorCode) {
	for _, path := range a.gateFiles(qualityGateSuffix) {
		text, reason, code := a.read(path)
		if reason != nil {
			return reason, code
		}
		document := parseQualityDocument(text)
		a.gateHeadings(path, document)
		a.cycleCeiling(path, document)
		a.retiredInputs(path, text)
		a.verdicts(path, document)
	}
	return nil, domain.ConfigUnusable
}

// gateHeadings applies QG04: the declared gate headings, all present, in
// declared order.
func (a *qualityAudit) gateHeadings(path string, document qualityDocument) {
	furthest := -1
	for _, heading := range a.policy.gateHeadings {
		position, line, ok := document.heading(heading)
		if !ok {
			finding := NewFinding("missing-gate-heading", path, "a required gate heading is absent")
			finding.Details = append(finding.Details, TextDetail("heading", heading))
			a.found("QG04", finding)
			continue
		}
		if position < furthest {
			finding := NewFinding("gate-heading-out-of-order", path, "a required gate heading appears before one it must follow")
			finding.Line = &line
			finding.Details = append(finding.Details, TextDetail("heading", heading))
			a.found("QG04", finding)
			continue
		}
		furthest = position
	}
}

// cycleCeiling applies QG06: the cycle input's values and default stay
// within 1 to 3.
func (a *qualityAudit) cycleCeiling(path string, document qualityDocument) {
	type cellRow struct {
		line  int
		cells []string
	}
	var rows []cellRow
	for _, line := range document.lines {
		cells, ok := qualityTableRow(line.text)
		if !ok {
			continue
		}
		if len(cells) == 0 || strings.Trim(cells[0], "`") != qualityCycleInput {
			continue
		}
		rows = append(rows, cellRow{line: line.number, cells: cells})
	}
	if len(rows) == 0 {
		a.found("QG06", NewFinding("unbounded-gate-cycles", path,
			"the gate declares no max-cycles input bounded by 1 to 3"))
	}
	for _, row := range rows {
		defValue, defOk := parseCycleValue(strings.Trim(row.cells[len(row.cells)-1], "`"))
		bounded := len(row.cells) > 1 && defOk && defValue >= qualityCycleFloor && defValue <= qualityCycleCeilingMax
		if bounded {
			for _, cell := range row.cells[1:] {
				for _, value := range qualityIntegers(cell) {
					if value < qualityCycleFloor || value > qualityCycleCeilingMax {
						bounded = false
						break
					}
				}
			}
		}
		if !bounded {
			finding := NewFinding("unbounded-gate-cycles", path,
				"the max-cycles input allows a value or default outside 1 to 3")
			finding.Line = &row.line
			a.found("QG06", finding)
		}
	}
}

// parseCycleValue parses one cycle bound.
func parseCycleValue(text string) (int, bool) {
	value := 0
	for _, character := range text {
		if character < '0' || character > '9' {
			return 0, false
		}
		value = value*10 + int(character-'0')
	}
	return value, len(text) > 0
}

// retiredInputs applies QG06: no gate names a retired input anywhere in its
// text.
func (a *qualityAudit) retiredInputs(path, text string) {
	for _, input := range a.policy.retiredInputs {
		for number, line := range strings.Split(text, "\n") {
			if namesQualityToken(line, input) {
				at := number + 1
				finding := NewFinding("retired-gate-input", path, "the gate names a retired input")
				finding.Line = &at
				finding.Details = append(finding.Details, TextDetail("input", input))
				a.found("QG06", finding)
				break
			}
		}
	}
}

// verdicts applies QG07: the verdict section names only declared verdicts.
func (a *qualityAudit) verdicts(path string, document qualityDocument) {
	position, start, ok := document.heading(a.policy.verdictHeading)
	if !ok {
		return
	}
	end := -1
	if position+1 < len(document.headings) {
		end = document.headings[position+1].number
	}
	for _, line := range document.lines {
		if line.number <= start || (end >= 0 && line.number >= end) {
			continue
		}
		for _, span := range qualityCodeSpans(line.text) {
			if !isVerdictShape(span) {
				continue
			}
			if !containsName(a.policy.verdicts, span) {
				finding := NewFinding("unknown-gate-verdict", path,
					"the gate names a verdict outside the declared verdicts")
				finding.Line = &line.number
				finding.Details = append(finding.Details, TextDetail("verdict", span))
				a.found("QG07", finding)
			}
		}
	}
}

// propagations applies QG05: every declared propagation heading is present.
func (a *qualityAudit) propagations() (*string, domain.ErrorCode) {
	for _, path := range a.gateFiles(qualityPropagationSfx) {
		text, reason, code := a.read(path)
		if reason != nil {
			return reason, code
		}
		document := parseQualityDocument(text)
		for _, heading := range a.policy.propagationHeading {
			if _, _, ok := document.heading(heading); !ok {
				finding := NewFinding("missing-propagation-heading", path, "a required propagation heading is absent")
				finding.Details = append(finding.Details, TextDetail("heading", heading))
				a.found("QG05", finding)
			}
		}
	}
	return nil, domain.ConfigUnusable
}

// qualityLine is one Markdown line outside fenced code.
type qualityLine struct {
	number int
	text   string
}

// qualityHeading is one level-one or level-two heading among those lines.
type qualityHeading struct {
	number int
	level  int
	title  string
}

// qualityDocument is a Markdown file's lines outside fenced code, and the
// level-one and level-two headings among them. Fenced lines are examples,
// never structure.
type qualityDocument struct {
	lines    []qualityLine
	headings []qualityHeading
}

// parseQualityDocument parses one gate or propagation file the way the
// oracle does: fence-aware, level-one and level-two headings only.
func parseQualityDocument(text string) qualityDocument {
	var document qualityDocument
	// The oracle iterates text.lines, which drops a trailing newline
	// without producing a final empty line; Split keeps it, so the last
	// empty piece of a newline-terminated file is skipped.
	raw := strings.Split(text, "\n")
	if len(raw) > 0 && raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1]
	}
	fenced := false
	for index, line := range raw {
		number := index + 1
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		for _, marker := range []struct {
			prefix string
			level  int
		}{{"# ", 1}, {"## ", 2}} {
			if title, ok := strings.CutPrefix(line, marker.prefix); ok {
				document.headings = append(document.headings, qualityHeading{
					number: number,
					level:  marker.level,
					title:  strings.TrimSpace(title),
				})
			}
		}
		document.lines = append(document.lines, qualityLine{number: number, text: line})
	}
	return document
}

// heading answers the first level-two heading with an exact title, with its
// position among all headings and its line number.
func (d qualityDocument) heading(title string) (int, int, bool) {
	for position, heading := range d.headings {
		if heading.level == 2 && heading.title == title {
			return position, heading.number, true
		}
	}
	return 0, 0, false
}

// baseName answers the last segment of a repository-relative path.
func baseName(path string) string {
	if index := strings.LastIndexByte(path, '/'); index >= 0 {
		return path[index+1:]
	}
	return path
}

// parentDir answers the directory holding a repository-relative path.
func parentDir(path string) string {
	if index := strings.LastIndexByte(path, '/'); index >= 0 {
		return path[:index]
	}
	return ""
}

// gateFamily answers the family a gate or propagation file is named for.
func gateFamily(path string) (string, bool) {
	name := baseName(path)
	for _, suffix := range []string{qualityGateSuffix, qualityPropagationSfx} {
		if family, ok := strings.CutSuffix(name, suffix); ok && family != "" {
			return family, true
		}
	}
	return "", false
}

// isModuleName answers whether a file name is one numbered workflow module:
// three digits, a hyphen, a name, and the extension.
func isModuleName(name string) bool {
	if len(name) <= len("000-.md") || !strings.HasSuffix(name, ".md") {
		return false
	}
	if name[3] != '-' {
		return false
	}
	for i := 0; i < 3; i++ {
		if name[i] < '0' || name[i] > '9' {
			return false
		}
	}
	return true
}

// qualityTableRow splits one Markdown table row into its trimmed cells.
func qualityTableRow(line string) ([]string, bool) {
	inner, ok := strings.CutPrefix(strings.TrimSpace(line), "|")
	if !ok {
		return nil, false
	}
	inner, _ = strings.CutSuffix(inner, "|")
	cells := strings.Split(inner, "|")
	for i, cell := range cells {
		cells[i] = strings.TrimSpace(cell)
	}
	return cells, true
}

// qualityIntegers lists every unsigned integer a table cell holds.
func qualityIntegers(text string) []int {
	var out []int
	for _, field := range strings.FieldsFunc(text, func(character rune) bool {
		return character < '0' || character > '9'
	}) {
		value := 0
		for i := 0; i < len(field); i++ {
			value = value*10 + int(field[i]-'0')
		}
		out = append(out, value)
	}
	return out
}

// namesQualityToken answers whether a token appears in a line as a whole
// name, not inside a longer one.
func namesQualityToken(line, token string) bool {
	if token == "" {
		return false
	}
	joins := func(character rune) bool {
		return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '-' || character == '_'
	}
	runes := []rune(line)
	want := []rune(token)
	for at := 0; at+len(want) <= len(runes); at++ {
		matched := true
		for i, character := range want {
			if runes[at+i] != character {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}
		if at > 0 && joins(runes[at-1]) {
			continue
		}
		if at+len(want) < len(runes) && joins(runes[at+len(want)]) {
			continue
		}
		return true
	}
	return false
}

// qualityCodeSpans lists every other backtick-delimited span: the text of
// the first, third, and further odd code spans on a line.
func qualityCodeSpans(line string) []string {
	parts := strings.Split(line, "`")
	var out []string
	for i := 1; i < len(parts); i += 2 {
		out = append(out, parts[i])
	}
	return out
}

// isVerdictShape answers whether a code span reads as a verdict: longer than
// one character, starting uppercase, holding only uppercase, digits, and
// underscores.
func isVerdictShape(span string) bool {
	if len(span) <= 1 {
		return false
	}
	for i, character := range span {
		if i == 0 && (character < 'A' || character > 'Z') {
			return false
		}
		upper := character >= 'A' && character <= 'Z'
		digit := character >= '0' && character <= '9'
		if !upper && !digit && character != '_' {
			return false
		}
	}
	return true
}
