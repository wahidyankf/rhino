package application

import (
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/config"
)

// TestEnvDialectReadsEachForm proves the env dialect reads every form pull writes
// and the forms push and diff accept: bare, single-quoted, double-quoted, and the
// close-quote, backslash-quote, open-quote run the single-quoted form writes for a quote.
func TestEnvDialectReadsEachForm(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"empty", "", ""},
		{"bare", "marker-plain_value.1:2@3%4+5,6", "marker-plain_value.1:2@3%4+5,6"},
		{"single quoted with a space", "'a b'", "a b"},
		{"double quoted", `"a b"`, "a b"},
		{"escaped quote between quoted runs", `'it'\''s'`, "it's"},
		{"escaped quote alone", `\'`, "'"},
		{"single quoted keeps a backslash", `'a\b'`, `a\b`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := decodeEnvValue(c.value)
			if !ok || got != c.want {
				t.Fatalf("decodeEnvValue(%q) = %q, %t; want %q", c.value, got, ok, c.want)
			}
		})
	}
}

// TestEnvDialectRefusesWhatItCannotRead proves each malformed value is refused,
// so push and diff never read a value they cannot reproduce.
func TestEnvDialectRefusesWhatItCannotRead(t *testing.T) {
	for _, value := range []string{
		`'unterminated`,
		`"unterminated`,
		`"has \ backslash"`,
		`"has $ substitution"`,
		"\"has ` backtick\"",
		`a b`,
		`a#b`,
		`$(cmd)`,
	} {
		if _, ok := decodeEnvValue(value); ok {
			t.Fatalf("decodeEnvValue(%q) read a value the dialect does not hold", value)
		}
	}
}

// TestEnvDialectLinesAndKeys proves the line rules: blank lines and comments are
// skipped, an export prefix is read, and a bad or repeated line is numbered.
func TestEnvDialectLinesAndKeys(t *testing.T) {
	values, bad := parseEnvDialect("\n# comment\n  export ALPHA_KEY=marker-alpha\nBETA_KEY='marker beta'\r\n")
	if bad != 0 || values["ALPHA_KEY"] != "marker-alpha" || values["BETA_KEY"] != "marker beta" {
		t.Fatalf("values = %v, bad = %d", values, bad)
	}
	for _, text := range []string{
		"1BAD=x\n",
		"NO_EQUALS\n",
		"A=1\nA=2\n",
		"A=1\nB=\"unterminated\n",
	} {
		if _, bad := parseEnvDialect(text); bad == 0 {
			t.Fatalf("parseEnvDialect(%q) read a malformed file", text)
		}
	}
	if _, bad := parseEnvDialect("A=1\nB=2\nC=\"x\n"); bad != 3 {
		t.Fatalf("bad line = %d, want 3", bad)
	}
}

// TestEnvDialectWritesWhatItReads proves encoding and decoding agree on every
// value pull can receive, including the quote and the space.
func TestEnvDialectWritesWhatItReads(t *testing.T) {
	for _, value := range []string{"", "plain", "a b", "it's", "'", "x\"y", `a\b`, "$HOME"} {
		encoded := encodeEnvValue(value)
		decoded, ok := decodeEnvValue(encoded)
		if !ok || decoded != value {
			t.Fatalf("value %q encodes to %q, which reads back as %q, %t", value, encoded, decoded, ok)
		}
	}
	if got := encodeEnvValue("plain-value"); got != "plain-value" {
		t.Fatalf("a plain value is written bare, got %q", got)
	}
}

// TestGenerationNamesAreReadAndCounted proves only base.gNNNN names count as
// generations, that generation zero never does, and that the newest is found.
func TestGenerationNamesAreReadAndCounted(t *testing.T) {
	attachments := []VaultAttachment{
		{ID: "a", FileName: "service.json.g0001"},
		{ID: "b", FileName: "service.json.g0003"},
		{ID: "c", FileName: "service.json.g0000"},
		{ID: "d", FileName: "service.json.gx"},
		{ID: "e", FileName: "other.g0009"},
		{ID: "f", FileName: "service.json"},
	}
	generations := generationsOf("service.json", attachments)
	if len(generations) != 2 || generations[1].ID != "a" || generations[3].ID != "b" {
		t.Fatalf("generations = %v", generations)
	}
	if highestGeneration(generations) != 3 || highestGeneration(map[int]VaultAttachment{}) != 0 {
		t.Fatalf("highest generation is wrong")
	}
	if generationName("service.json", 12) != "service.json.g0012" {
		t.Fatalf("generation name = %q", generationName("service.json", 12))
	}
}

// TestEntryModeReadsTheDeclaredOrDefault proves a declared octal mode is read
// and an undeclared one takes the default.
func TestEntryModeReadsTheDeclaredOrDefault(t *testing.T) {
	if got := entryMode(config.VaultEntry{}); got != 0o600 {
		t.Fatalf("default mode = %o", got)
	}
	if got := entryMode(config.VaultEntry{Mode: "0640"}); got != 0o640 {
		t.Fatalf("declared mode = %o", got)
	}
}
