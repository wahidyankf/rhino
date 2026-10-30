package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
	"github.com/wahidyankf/rhino/internal/domain"
)

func TestCountDetailRenders(t *testing.T) {
	report := application.NewReport("word-budget", "file")
	finding := application.NewFinding("over-budget", "docs/a.md", "too long")
	finding.Details = append(finding.Details, application.CountDetail("words", 42))
	report.Findings = append(report.Findings, finding)
	outcome := report.Render(application.Text)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	if !strings.Contains(outcome.Stderr, "words=42") {
		t.Fatalf("count detail missing: %q", outcome.Stderr)
	}
	json := report.Render(application.JSON)
	if !strings.Contains(json.Stdout, "\"words\":42") {
		t.Fatalf("count json missing: %q", json.Stdout)
	}
}

func TestErrorStrings(t *testing.T) {
	parsed, refusal := application.Parse(args("nope"))
	if refusal == nil || parsed.Run != nil {
		t.Fatal("expected refusal")
	}
	if refusal.Error() != "rhino: "+refusal.Message {
		t.Fatalf("unexpected refusal string: %q", refusal.Error())
	}
	readErr := &application.ReadError{Kind: "missing", Reason: "x: gone"}
	if readErr.Error() != "x: gone" {
		t.Fatalf("unexpected read error: %q", readErr.Error())
	}
	if got := domain.ErrorCode(99).String(); got != "rhino.args.unrecognized" {
		t.Fatalf("unexpected default code: %q", got)
	}
}

func TestMatchBracketEdges(t *testing.T) {
	for _, pattern := range []string{"[!]a]", "[]a]", "[a-]"} {
		if _, err := application.Match(pattern, "a"); err != nil {
			t.Fatalf("valid class refused: %q: %v", pattern, err)
		}
	}
	if _, err := application.Match("[!]a", "a"); err == nil {
		t.Fatal("class without a closer was not refused")
	}
	if ok, err := application.Match("[a-]x", "-x"); err != nil || !ok {
		t.Fatalf("trailing dash class failed: %v %v", ok, err)
	}
}
