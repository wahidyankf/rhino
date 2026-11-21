// Wiring rules W01 to W07: the registry, the hook directory, and the workflow
// directory agree on which surface runs where. Tree validator: it reads the
// files it names, starts no process, and reaches no network.
package application

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// gateRunCall matches one `gate run --surface <name>` call on a line.
var gateRunCall = regexp.MustCompile(`gate\s+run\b.*?--surface(?:\s+|=)([a-z][a-z-]*)`)

// releaseVersion matches a release written by name, such as v0.13.4.
var releaseVersion = regexp.MustCompile(`v[0-9]+\.[0-9]+\.[0-9]+`)

// triggeredBy names the workflow trigger a surface a hook never runs needs. A
// workflow with the trigger counts as calling the surface; any other caller
// does not.
var triggeredBy = map[string]string{"main": "push", "scheduled": "schedule", "manual": "workflow_dispatch"}

// defaultAllowed is the allowed list of each local surface a repository does
// not declare: staged-file formatting, the commit-message lint, and the leak
// scan.
var defaultAllowed = map[string][]string{
	"pre-commit": {"format-staged"},
	"commit-msg": {"commit-message-lint"},
	"pre-push":   {"leak-scan"},
}

// wiringCall is one `gate run --surface` call: the surface it names, the file
// and line that hold it, and the workflow holding it, nil for a hook file.
type wiringCall struct {
	surface  string
	path     string
	line     int
	workflow *config.Workflow
}

// wiringRunLine is one line of a workflow step's run text, for the release rule.
type wiringRunLine struct {
	path string
	line int
	text string
}

// wiringLock is a root-level lock file that names one release.
type wiringLock struct {
	path    string
	tool    string
	version string
}

// wiringScan is everything the wiring rules read from the tree.
type wiringScan struct {
	files    map[string]bool
	scanned  []string
	calls    []wiringCall
	runLines []wiringRunLine
	locks    []wiringLock
	findings []Finding
}

// ruleFault is why the wiring rules cannot read the tree, with its closed code.
type ruleFault struct {
	code   domain.ErrorCode
	reason string
}

// gateWiringValidate runs the wiring rules over the declared registry and the
// tree.
func gateWiringValidate(document *config.Document, tree Tree, format Format) Outcome {
	wiring, declared, err := document.Wiring()
	if err != nil {
		return reportRefused("gate-wiring", format, err.Error())
	}
	if !declared {
		report := NewReport("gate-wiring", "surface")
		report.Refusal = strptr("gates.wiring is not declared, and RHINO holds no default for it")
		report.RefusalCode = domain.ConfigUndeclared
		return report.Render(format)
	}
	scan, fault := scanWiring(wiring, tree)
	if fault != nil {
		report := NewReport("gate-wiring", "surface")
		report.Refusal = strptr(fault.reason)
		report.RefusalCode = fault.code
		return report.Render(format)
	}
	declaredSurfaces := 0
	findings := scan.findings
	for _, surface := range Surfaces {
		if len(document.GateIDsOn(surface)) > 0 {
			declaredSurfaces++
		}
	}
	findings = append(findings, wiringSurfaceFindings(document, wiring, scan)...)
	findings = append(findings, wiringEntryFindings(document, wiring, scan)...)

	report := NewReport("gate-wiring", "surface")
	report.Inspected = declaredSurfaces
	report.Scanned = scan.scanned
	report.Findings = findings
	return renderRuleReport(report, format)
}

// scanWiring reads the hook files, the workflow files, and the root lock files,
// and records every `gate run` call and every line the release rule reads.
func scanWiring(wiring *config.Wiring, tree Tree) (*wiringScan, *ruleFault) {
	files := tree.Files()
	scan := &wiringScan{files: map[string]bool{}}
	for _, path := range files {
		scan.files[path] = true
	}
	for _, path := range directFiles(files, wiring.HookDirectory, nil) {
		text, fault := readRuleFile(tree, path)
		if fault != nil {
			return nil, fault
		}
		scan.scanned = append(scan.scanned, path)
		scan.scanHook(path, text)
	}
	if wiring.Workflows {
		for _, path := range directFiles(files, wiring.WorkflowDirectory, isYAMLFile) {
			text, fault := readRuleFile(tree, path)
			if fault != nil {
				return nil, fault
			}
			workflow, err := config.ParseWorkflow(text)
			if err != nil {
				return nil, &ruleFault{code: domain.FileUnreadable, reason: path + ": holds no readable YAML: " + err.Error()}
			}
			scan.scanned = append(scan.scanned, path)
			scan.scanWorkflow(path, workflow)
		}
	}
	for _, path := range directFiles(files, "", isLockFile) {
		text, fault := readRuleFile(tree, path)
		if fault != nil {
			return nil, fault
		}
		version := lockVersion(text)
		if version == "" {
			continue
		}
		scan.scanned = append(scan.scanned, path)
		scan.locks = append(scan.locks, wiringLock{path: path, tool: strings.TrimSuffix(path, ".lock"), version: version})
	}
	return scan, nil
}

// scanHook records the calls a hook file makes. A comment line makes none, and
// a `set +e` in a file that calls gate run is a suppressed failure.
func (s *wiringScan) scanHook(path, text string) {
	var calls []wiringCall
	var errexitOff []int
	for index, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if surface, ok := gateRunSurface(line); ok {
			calls = append(calls, wiringCall{surface: surface, path: path, line: index + 1})
			if suppressedCall(line) {
				s.findings = append(s.findings, suppressedFinding(path, index+1, "the `gate run` call ends in a suppressing `|| true` or `|| :`"))
			}
		}
		if strings.Contains(line, "set +e") {
			errexitOff = append(errexitOff, index+1)
		}
	}
	s.calls = append(s.calls, calls...)
	if len(calls) > 0 {
		for _, line := range errexitOff {
			s.findings = append(s.findings, suppressedFinding(path, line, "the hook turns errexit off with `set +e` around `gate run`"))
		}
	}
}

// scanWorkflow records the calls each workflow step makes and the lines of
// every step's run text. A step that continues on error, or that turns errexit
// off, around a call is a suppressed failure.
func (s *wiringScan) scanWorkflow(path string, workflow *config.Workflow) {
	for _, step := range workflow.Steps {
		if step.Run == "" {
			continue
		}
		var calls []wiringCall
		var errexitOff []int
		for index, raw := range strings.Split(step.Run, "\n") {
			line := strings.TrimSpace(raw)
			lineNumber := step.RunLine + index
			s.runLines = append(s.runLines, wiringRunLine{path: path, line: lineNumber, text: line})
			if surface, ok := gateRunSurface(line); ok {
				calls = append(calls, wiringCall{surface: surface, path: path, line: lineNumber, workflow: workflow})
				if suppressedCall(line) {
					s.findings = append(s.findings, suppressedFinding(path, lineNumber, "the `gate run` call ends in a suppressing `|| true` or `|| :`"))
				}
			}
			if strings.Contains(line, "set +e") {
				errexitOff = append(errexitOff, lineNumber)
			}
		}
		if len(calls) > 0 && step.ContinueOnError {
			s.findings = append(s.findings, suppressedFinding(path, step.Line, "the step that calls `gate run` continues on error"))
		}
		if len(calls) > 0 {
			for _, line := range errexitOff {
				s.findings = append(s.findings, suppressedFinding(path, line, "the step turns errexit off with `set +e` around `gate run`"))
			}
		}
		s.calls = append(s.calls, calls...)
	}
}

// wiringSurfaceFindings evaluates the rules that read one surface at a time:
// W01 (a declared surface is called), W02 (a call names a declared surface),
// W03 (the pull-request surface is called by a workflow), W04 (each local
// surface has its hook file), and W06 (a release named by a workflow matches
// its lock).
func wiringSurfaceFindings(document *config.Document, wiring *config.Wiring, scan *wiringScan) []Finding {
	var findings []Finding
	for _, surface := range Surfaces {
		ids := document.GateIDsOn(surface)
		if len(ids) == 0 {
			continue
		}
		if !calledOn(scan.calls, surface, false) {
			findings = append(findings, entryFinding(document, "W01", ids[0],
				"gates.entries."+ids[0]+".run-on."+surface,
				fmt.Sprintf("surface `%s` holds entries, and no hook or workflow step calls `gate run --surface %s`", surface, surface)))
		}
	}
	for _, call := range scan.calls {
		if len(document.GateIDsOn(call.surface)) == 0 {
			findings = append(findings, Finding{Kind: "W02", Path: call.path, Line: intPtr(call.line),
				Message: fmt.Sprintf("`gate run --surface %s` names a surface the registry does not declare", call.surface)})
		}
	}
	if ids := document.GateIDsOn("pull-request"); len(ids) > 0 && wiring.Workflows && !calledOn(scan.calls, "pull-request", true) {
		findings = append(findings, entryFinding(document, "W03", ids[0],
			"gates.entries."+ids[0]+".run-on.pull-request",
			"the pull-request surface holds entries, and no workflow step calls `gate run --surface pull-request`"))
	}
	for _, surface := range config.LocalSurfaces {
		ids := document.GateIDsOn(surface)
		if len(ids) == 0 {
			continue
		}
		hookPath := directoryPrefix(wiring.HookDirectory) + surface
		if !scan.files[hookPath] {
			finding := Finding{Kind: "W04", Path: hookPath, Message: fmt.Sprintf("surface `%s` holds entries, and no hook file of that name exists", surface)}
			field := "gates.wiring.hooks.directory"
			finding.Field = &field
			findings = append(findings, finding)
		}
	}
	findings = append(findings, releaseFindings(scan)...)
	return findings
}

// calledOn reports whether a call names a surface and counts as a call of it. A
// surface a trigger starts counts only when a workflow with that trigger makes
// the call. workflowOnly limits the count to workflow calls, as W03 requires.
func calledOn(calls []wiringCall, surface string, workflowOnly bool) bool {
	for _, call := range calls {
		if call.surface != surface {
			continue
		}
		if workflowOnly && call.workflow == nil {
			continue
		}
		if _, triggered := triggeredBy[surface]; !triggered {
			return true
		}
		if call.workflow != nil && triggerMatches(call.workflow, surface) {
			return true
		}
	}
	return false
}

// triggerMatches reports whether a workflow starts on the trigger a triggered
// surface needs. The main surface needs a push to the main branch.
func triggerMatches(workflow *config.Workflow, surface string) bool {
	if surface == "main" {
		return workflow.Triggers["push"] && contains(workflow.PushBranches, "main")
	}
	return workflow.Triggers[triggeredBy[surface]]
}

// releaseFindings is W06: a workflow step that names a release by name must
// name the release its lock file holds. A line that reads the lock file is not
// naming a release, so it passes.
func releaseFindings(scan *wiringScan) []Finding {
	var findings []Finding
	for _, lock := range scan.locks {
		word := regexp.MustCompile(`\b` + regexp.QuoteMeta(lock.tool) + `\b`)
		for _, line := range scan.runLines {
			if !word.MatchString(line.text) || strings.Contains(line.text, lock.path) {
				continue
			}
			for _, release := range releaseVersion.FindAllString(line.text, -1) {
				if release == lock.version {
					continue
				}
				findings = append(findings, Finding{Kind: "W06", Path: line.path, Line: intPtr(line.line),
					Message: fmt.Sprintf("the step names release `%s` for `%s`, but `%s` names `%s`", release, lock.tool, lock.path, lock.version)})
			}
		}
	}
	return findings
}

// wiringEntryFindings is W07: when the pull-request surface holds entries, each
// local surface holds only the entries its allowed list names. An entry is
// allowed by its identifier or by its kind.
func wiringEntryFindings(document *config.Document, wiring *config.Wiring, scan *wiringScan) []Finding {
	if len(document.GateIDsOn("pull-request")) == 0 {
		return nil
	}
	var findings []Finding
	for _, surface := range config.LocalSurfaces {
		allowed, declared := wiring.LocalAllowed[surface]
		if !declared {
			allowed = defaultAllowed[surface]
		}
		for _, gate := range document.GatesOn(surface) {
			kind := gate.Kind
			if kind == "" {
				kind = "check"
			}
			if contains(allowed, gate.ID) || contains(allowed, kind) {
				continue
			}
			findings = append(findings, entryFinding(document, "W07", gate.ID,
				"gates.wiring.local-surfaces."+surface,
				fmt.Sprintf("entry `%s` runs on local surface `%s`, outside that surface's allowed list", gate.ID, surface)))
		}
	}
	return findings
}

// entryFinding names one gate entry in the registry, with its line when the
// document holds it.
func entryFinding(document *config.Document, rule, gateID, field, message string) Finding {
	finding := Finding{Kind: rule, Path: config.Path, Message: message}
	if line := document.LineOf("gates", "entries", gateID); line > 0 {
		finding.Line = intPtr(line)
	}
	finding.Field = &field
	return finding
}

// suppressedFinding is W05 at one line of a hook or a workflow.
func suppressedFinding(path string, line int, message string) Finding {
	return Finding{Kind: "W05", Path: path, Line: intPtr(line), Message: message}
}

// gateRunSurface returns the surface a line calls through `gate run`.
func gateRunSurface(line string) (string, bool) {
	match := gateRunCall.FindStringSubmatch(line)
	if match == nil {
		return "", false
	}
	return match[1], true
}

// suppressedCall reports whether a line that calls gate run ends in an or-true
// that hides its failure.
func suppressedCall(line string) bool {
	return strings.HasSuffix(line, "|| true") || strings.HasSuffix(line, "|| :")
}

// lockVersion reads the `version=` line of a lock file, or returns the empty
// string when it names none.
func lockVersion(text string) string {
	for _, raw := range strings.Split(text, "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(raw), "version="); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// readRuleFile reads one file the wiring rules name, refusing one that holds
// no text or cannot be read.
func readRuleFile(tree Tree, path string) (string, *ruleFault) {
	text, readErr := tree.Read(path)
	if readErr == nil {
		return text, nil
	}
	if readErr.Kind == "not-text" {
		return "", &ruleFault{code: domain.FileUnreadable, reason: path + ": holds no text"}
	}
	return "", &ruleFault{code: domain.FileUnreadable, reason: path + ": " + readErr.Reason}
}

// directFiles lists the files that sit directly in a directory, in the order
// the tree lists them, and keeps those a predicate accepts. An empty directory
// names the repository root.
func directFiles(files []string, directory string, keep func(string) bool) []string {
	prefix := directoryPrefix(directory)
	var out []string
	for _, file := range files {
		if !strings.HasPrefix(file, prefix) || strings.Contains(file[len(prefix):], "/") {
			continue
		}
		if keep != nil && !keep(file) {
			continue
		}
		out = append(out, file)
	}
	return out
}

// directoryPrefix is the repository-relative prefix of a directory's files, with
// a trailing slash; the repository root has the empty prefix.
func directoryPrefix(directory string) string {
	trimmed := strings.TrimSuffix(strings.TrimPrefix(directory, "./"), "/")
	if trimmed == "" || trimmed == "." {
		return ""
	}
	return trimmed + "/"
}

func isYAMLFile(file string) bool {
	return strings.HasSuffix(file, ".yml") || strings.HasSuffix(file, ".yaml")
}

func isLockFile(file string) bool {
	return strings.HasSuffix(file, ".lock")
}

func intPtr(value int) *int { return &value }

// renderRuleReport renders a rule run. JSON keeps the report's sorted
// violation document; text names each finding as its path, its line, its rule,
// its field, and its message, in the same order.
func renderRuleReport(report *Report, format Format) Outcome {
	outcome := report.Render(format)
	if format == Text {
		outcome.Stderr = renderRuleFindings(report.Category, report.Findings)
	}
	return outcome
}

// renderRuleFindings lists findings in the contract's order: path, line,
// column, rule, and field.
func renderRuleFindings(category string, findings []Finding) string {
	ordered := append([]Finding(nil), findings...)
	sort.SliceStable(ordered, func(i, j int) bool { return compareFindings(ordered[i], ordered[j]) < 0 })
	var out strings.Builder
	for _, finding := range ordered {
		position := finding.Path
		if finding.Line != nil {
			position = fmt.Sprintf("%s:%d", finding.Path, *finding.Line)
		}
		field := ""
		if finding.Field != nil {
			field = " " + *finding.Field
		}
		fmt.Fprintf(&out, "[%s] %s %s%s: %s\n", category, position, finding.Kind, field, finding.Message)
	}
	return out.String()
}
