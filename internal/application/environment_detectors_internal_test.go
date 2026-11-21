package application

import (
	"reflect"
	"testing"
)

// These readers take a declared source as a string and answer with names only.
// Each case states the literal names it must report and whether a computed or
// unterminated reference must mark the source dynamic, so no reader needs a
// double to run.

func TestDetectorAccessesReadsEachLanguagesLiteralNames(t *testing.T) {
	cases := []struct {
		name     string
		language string
		source   string
		keys     []string
		dynamic  bool
	}{
		{"rust reads both std readers", "rust", `std::env::var("A") std::env::var_os('B')`, []string{"A", "B"}, false},
		{"rust marks a computed name dynamic", "rust", `std::env::var(name)`, nil, true},
		{"f-sharp reads its environment call", "f-sharp", `Environment.GetEnvironmentVariable("C")`, []string{"C"}, false},
		{"go reads each stdlib reader", "go", `os.Getenv("D") os.LookupEnv("E") os.LookupEnv,"F"`, []string{"D", "E", "F"}, false},
		{"terraform reads get_env and env", "terraform", `get_env("G") env("H")`, []string{"G", "H"}, false},
		{"ansible reads a literal env lookup", "ansible", `lookup('env', 'I') lookup('file', 'x') lookup('env', name)`, []string{"I"}, true},
		{"type-script reads dot, bracket, and yaml keys", "type-script", "process.env.J;\nprocess.env[\"K\"];\nprocess.env[name];\nLOCAL_KEY: x\nlower: y\n", []string{"J", "K", "LOCAL_KEY"}, true},
		{"an unknown language reads nothing", "python", `os.getenv("L")`, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			keys, dynamic := detectorAccesses(tc.language, tc.source)
			assertAccessNames(t, keys, dynamic, tc.keys, tc.dynamic)
		})
	}
}

func TestQuotedCallsMarksAnUnterminatedLiteralDynamic(t *testing.T) {
	keys, dynamic := quotedCalls(`os.Getenv("abc`, []string{"os.Getenv("})
	assertAccessNames(t, keys, dynamic, nil, true)
}

func TestQuotedValueSplitsOnlyAClosedQuotedLiteral(t *testing.T) {
	cases := []struct {
		name, value, literal, rest string
		ok                         bool
	}{
		{"a blank value has no literal", "", "", "", false},
		{"an unquoted value has no literal", "x", "", "x", false},
		{"a closed single-quoted literal splits", "'a' rest", "a", " rest", true},
		{"an unterminated double quote has no literal", `"b`, "", `"b`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			literal, rest, ok := quotedValue(tc.value)
			if literal != tc.literal || rest != tc.rest || ok != tc.ok {
				t.Fatalf("quotedValue(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.value, literal, rest, ok, tc.literal, tc.rest, tc.ok)
			}
		})
	}
}

func TestKeyBytesAndUpperKeysFollowTheirAlphabets(t *testing.T) {
	for _, c := range []byte("_AZaz09") {
		if !isKeyByte(c) {
			t.Fatalf("isKeyByte(%q) = false, want true", c)
		}
	}
	for _, c := range []byte("- ") {
		if isKeyByte(c) {
			t.Fatalf("isKeyByte(%q) = true, want false", c)
		}
	}
	if !isUpperKey("A_9") || isUpperKey("Ab") || isUpperKey("A-") {
		t.Fatal("isUpperKey does not follow the upper-case key alphabet")
	}
}

func TestTypescriptAndAnsibleReadersMarkAnEmptyOrOpenReferenceDynamic(t *testing.T) {
	keys, dynamic := typescriptAccesses("process.env.")
	assertAccessNames(t, keys, dynamic, nil, true)

	keys, dynamic = ansibleAccesses("lookup(")
	assertAccessNames(t, keys, dynamic, nil, true)

	keys, dynamic = ansibleAccesses("lookup('env'")
	assertAccessNames(t, keys, dynamic, nil, true)

	keys, dynamic = ansibleAccesses("lookup('env', ")
	assertAccessNames(t, keys, dynamic, nil, true)
}

func assertAccessNames(t *testing.T, got []string, dynamic bool, want []string, wantDynamic bool) {
	t.Helper()
	if !reflect.DeepEqual(got, want) || dynamic != wantDynamic {
		t.Fatalf("names = %v dynamic = %v, want %v dynamic = %v", got, dynamic, want, wantDynamic)
	}
}
