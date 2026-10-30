package application

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wahidyankf/rhino/internal/domain"
)

// Detail is one measured value attached to a finding.
type Detail struct {
	Key   string
	Text  string
	Count *int
}

// CountDetail builds a numeric detail.
func CountDetail(key string, value int) Detail { return Detail{Key: key, Count: &value} }

// TextDetail builds a text detail.
func TextDetail(key, value string) Detail { return Detail{Key: key, Text: value} }

// Finding is one policy violation.
type Finding struct {
	Kind    string
	Path    string
	Line    *int
	Column  *int
	Field   *string
	Message string
	Details []Detail
}

// NewFinding builds a finding.
func NewFinding(kind, path, message string) Finding {
	return Finding{Kind: kind, Path: path, Message: message}
}

// Outcome is everything one invocation produces.
type Outcome struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// Clean reports checked-and-clean.
func Clean(stdout string) Outcome { return Outcome{ExitCode: 0, Stdout: stdout} }

// Refused reports an unusable invocation. Never 1.
func Refused(stderr string) Outcome { return Outcome{ExitCode: 2, Stderr: stderr} }

// RefuseWith renders with the closed code for the requested format.
func RefuseWith(format Format, code domain.ErrorCode, message string) Outcome {
	return Refused(domain.Body(format, code, message))
}

func (f Finding) render(category string) string {
	if len(f.Details) > 0 {
		pairs := make([]string, 0, len(f.Details))
		for _, detail := range f.Details {
			if detail.Count != nil {
				pairs = append(pairs, fmt.Sprintf("%s=%d", detail.Key, *detail.Count))
			} else {
				pairs = append(pairs, detail.Key+"="+detail.Text)
			}
		}
		suffix := " (" + strings.Join(pairs, ", ") + ")"
		return findingLine(category, f, suffix)
	}
	return findingLine(category, f, "")
}

func findingLine(category string, finding Finding, suffix string) string {
	if finding.Column != nil {
		line := 1
		if finding.Line != nil {
			line = *finding.Line
		}
		field := ""
		if finding.Field != nil {
			field = " " + *finding.Field
		}
		return fmt.Sprintf("[%s] %s:%d:%d %s%s %s%s\n",
			category, finding.Path, line, *finding.Column, finding.Kind, field, finding.Message, suffix)
	}
	position := finding.Path
	if finding.Line != nil {
		position = fmt.Sprintf("%s:%d", finding.Path, *finding.Line)
	}
	return fmt.Sprintf("[%s] %s: %s%s\n", category, position, finding.Message, suffix)
}

// Report is one validator run.
type Report struct {
	Category     string
	Subject      string
	Inspected    int
	Scanned      []string
	Notes        []string
	Measurements []struct {
		Key   string
		Value int
	}
	Findings    []Finding
	Refusal     *string
	RefusalCode domain.ErrorCode
}

// NewReport starts a report.
func NewReport(category, subject string) *Report {
	return &Report{Category: category, Subject: subject, RefusalCode: domain.ConfigUnusable}
}

// Refuse marks the run as not-having-happened.
func Refuse(category string, reason string) *Report {
	report := NewReport(category, "configuration file")
	report.Refusal = &reason
	return report
}

// Render maps the report to an outcome: 0 clean, 1 findings, 2 refusal.
func (r *Report) Render(format Format) Outcome {
	if r.Refusal != nil {
		return RefuseWith(format, r.RefusalCode, "["+r.Category+"] "+*r.Refusal)
	}
	if format == JSON {
		return r.asJSON()
	}
	return r.finish()
}

// compareFindings orders findings by the contract's key: path, line, column,
// kind, field. An absent position sorts before any present one, so a finding
// about the file itself reads before one about its tenth line.
func compareFindings(left, right Finding) int {
	if left.Path != right.Path {
		if left.Path < right.Path {
			return -1
		}
		return 1
	}
	leftLine, leftHasLine := lineOf(left)
	rightLine, rightHasLine := lineOf(right)
	if value := compareOptional(leftLine, leftHasLine, rightLine, rightHasLine); value != 0 {
		return value
	}
	leftColumn, leftHasColumn := columnOf(left)
	rightColumn, rightHasColumn := columnOf(right)
	if value := compareOptional(leftColumn, leftHasColumn, rightColumn, rightHasColumn); value != 0 {
		return value
	}
	if left.Kind != right.Kind {
		if left.Kind < right.Kind {
			return -1
		}
		return 1
	}
	var leftField, rightField string
	if left.Field != nil {
		leftField = *left.Field
	}
	if right.Field != nil {
		rightField = *right.Field
	}
	switch {
	case leftField < rightField:
		return -1
	case leftField > rightField:
		return 1
	}
	return 0
}

func lineOf(finding Finding) (int, bool) {
	if finding.Line == nil {
		return 0, false
	}
	return *finding.Line, true
}

func columnOf(finding Finding) (int, bool) {
	if finding.Column == nil {
		return 0, false
	}
	return *finding.Column, true
}

func compareOptional(left int, leftOk bool, right int, rightOk bool) int {
	switch {
	case !leftOk && !rightOk:
		return 0
	case !leftOk:
		return -1
	case !rightOk:
		return 1
	case left < right:
		return -1
	case left > right:
		return 1
	}
	return 0
}

func plural(subject string, count int) string {
	if count == 1 {
		return subject
	}
	// `directory` -> `directories`, not `directorys`.
	if stem, ok := strings.CutSuffix(subject, "y"); ok {
		return stem + "ies"
	}
	// `harness` is one of the subjects, and "4 harnesss" reads as a defect
	// in the tool rather than as a count of four.
	if strings.HasSuffix(subject, "s") {
		return subject + "es"
	}
	return subject + "s"
}

func (r *Report) finish() Outcome {
	var summary strings.Builder
	checked := "no findings"
	switch len(r.Findings) {
	case 1:
		checked = "1 finding"
	default:
		if len(r.Findings) > 1 {
			checked = fmt.Sprintf("%d findings", len(r.Findings))
		}
	}
	summary.WriteString(fmt.Sprintf("[%s] checked %d %s, %s\n",
		r.Category, r.Inspected, plural(r.Subject, r.Inspected), checked))
	for _, note := range r.Notes {
		summary.WriteString("[" + r.Category + "] " + note + "\n")
	}
	scanned := append([]string(nil), r.Scanned...)
	sort.Strings(scanned)
	for _, path := range scanned {
		summary.WriteString("[" + r.Category + "] scanned " + path + "\n")
	}
	code := 0
	if len(r.Findings) > 0 {
		code = 1
	}
	return Outcome{ExitCode: code, Stdout: summary.String(), Stderr: stderrOf(r)}
}

func stderrOf(r *Report) string {
	structural := len(r.Findings) > 0
	for _, finding := range r.Findings {
		if finding.Column == nil {
			structural = false
		}
	}
	ordered := append([]Finding(nil), r.Findings...)
	if structural {
		sort.SliceStable(ordered, func(i, j int) bool {
			return compareFindings(ordered[i], ordered[j]) < 0
		})
		var out strings.Builder
		for _, finding := range ordered {
			out.WriteString(finding.render(r.Category))
		}
		return out.String()
	}
	rendered := make([]string, 0, len(ordered))
	for _, finding := range ordered {
		rendered = append(rendered, finding.render(r.Category))
	}
	sort.Strings(rendered)
	return strings.Join(rendered, "")
}

func (r *Report) asJSON() Outcome {
	scanned := append([]string(nil), r.Scanned...)
	sort.Strings(scanned)
	findings := append([]Finding(nil), r.Findings...)
	sort.SliceStable(findings, func(i, j int) bool {
		return compareFindings(findings[i], findings[j]) < 0
	})
	violations := make([]string, 0, len(findings))
	for _, finding := range findings {
		fields := []string{
			`"kind":` + domain.Quote(finding.Kind),
			`"path":` + domain.Quote(finding.Path),
			`"message":` + domain.Quote(finding.Message),
		}
		if finding.Line != nil {
			fields = append(fields, fmt.Sprintf(`"line":%d`, *finding.Line))
		}
		if finding.Column != nil {
			fields = append(fields, fmt.Sprintf(`"column":%d`, *finding.Column))
		}
		if finding.Field != nil {
			fields = append(fields, `"field":`+domain.Quote(*finding.Field))
		}
		for _, detail := range finding.Details {
			if detail.Count != nil {
				fields = append(fields, fmt.Sprintf(`%s:%d`, domain.Quote(detail.Key), *detail.Count))
			} else {
				fields = append(fields, domain.Quote(detail.Key)+":"+domain.Quote(detail.Text))
			}
		}
		violations = append(violations, "{"+strings.Join(fields, ",")+"}")
	}
	quoted := func(values []string) string {
		out := make([]string, 0, len(values))
		for _, value := range values {
			out = append(out, domain.Quote(value))
		}
		return strings.Join(out, ",")
	}
	measurements := ""
	for _, measure := range r.Measurements {
		measurements += fmt.Sprintf(",%s:%d", domain.Quote(measure.Key), measure.Value)
	}
	code := 0
	if len(r.Findings) > 0 {
		code = 1
	}
	body := fmt.Sprintf("{\"schemaVersion\":1,\"command\":%s,\"exitCode\":%d,\"subject\":%s,\"inspected\":%d,\"scanned\":[%s],\"notes\":[%s],\"violations\":[%s]%s}\n",
		domain.Quote(r.Category), code, domain.Quote(r.Subject),
		r.Inspected, quoted(scanned), quoted(r.Notes), strings.Join(violations, ","), measurements)
	return Outcome{ExitCode: code, Stdout: body}
}
