package application

import (
	"strings"
	"testing"
)

// TestPluralFollowsTheSubjectSpelling proves a count reads the singular for one
// and the plural spelling the subject needs otherwise, including the -y and the
// -s endings that take their own form.
func TestPluralFollowsTheSubjectSpelling(t *testing.T) {
	cases := []struct {
		subject string
		count   int
		want    string
	}{
		{"file", 1, "file"},
		{"file", 2, "files"},
		{"directory", 2, "directories"},
		{"harness", 2, "harnesses"},
	}
	for _, tc := range cases {
		if got := plural(tc.subject, tc.count); got != tc.want {
			t.Errorf("plural(%q, %d) = %q, want %q", tc.subject, tc.count, got, tc.want)
		}
	}
}

// TestReportCarriesNotesAndMeasurements proves a report's notes and measurements
// reach the text summary and the JSON document, for any producer that sets them.
func TestReportCarriesNotesAndMeasurements(t *testing.T) {
	report := NewReport("sample", "file")
	report.Notes = []string{"one note"}
	report.Measurements = append(report.Measurements, struct {
		Key   string
		Value int
	}{Key: "lines", Value: 3})
	text := report.finish()
	if !strings.Contains(text.Stdout, "[sample] one note\n") {
		t.Fatalf("note missing from the text summary: %q", text.Stdout)
	}
	document := report.asJSON()
	if !strings.Contains(document.Stdout, `"one note"`) || !strings.Contains(document.Stdout, `"lines":3`) {
		t.Fatalf("note or measurement missing from the JSON document: %q", document.Stdout)
	}
}
