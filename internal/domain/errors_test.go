package domain_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/domain"
)

func TestErrorCodesAreClosedNamespacedAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, code := range domain.All {
		text := code.String()
		parts := strings.Split(text, ".")
		if len(parts) != 3 {
			t.Fatalf("%s is not tool.area.reason", text)
		}
		if parts[0] != "rhino" {
			t.Fatalf("%s does not name the tool first", text)
		}
		if parts[1] == "" || parts[2] == "" {
			t.Fatalf("%s has an empty area or reason", text)
		}
		if seen[text] {
			t.Fatalf("%s appears twice", text)
		}
		seen[text] = true
	}
	if len(domain.All) != 18 {
		t.Fatalf("expected 18 codes, got %d", len(domain.All))
	}
}

func TestBodyRendersTextAndJSON(t *testing.T) {
	text := domain.Body(domain.Text, domain.ArgsUnrecognized, "no such flag\n")
	if text != "rhino: no such flag\n" {
		t.Fatalf("unexpected text body %q", text)
	}
	json := domain.Body(domain.JSON, domain.GateChildNotFound, "gate `a`: gone")
	expected := "{\"schemaVersion\":1,\"error\":{\"code\":\"rhino.gate.child-not-found\",\"message\":\"gate `a`: gone\"}}\n"
	if json != expected {
		t.Fatalf("unexpected json body %q", json)
	}
}

func TestBodyEscapesMessage(t *testing.T) {
	json := domain.Body(domain.JSON, domain.FileUnreadable, "a \"quoted\" \\ path\twith\r\nbreaks\x01")
	if strings.Count(json, "\n") != 1 {
		t.Fatalf("message broke the document: %q", json)
	}
	for _, want := range []string{"\\\"quoted\\\"", "\\\\", "\\t", "\\r", "\\n", "\\u0001"} {
		if !strings.Contains(json, want) {
			t.Fatalf("missing escape %s in %q", want, json)
		}
	}
}
