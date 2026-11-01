package application

import "testing"

// These predicates answer their contract for inputs the policy decoder refuses
// before a run reaches them, so each is exercised directly with that input.

// TestCompareEnvironmentFindingsOrdersByRuleThenPathThenKey proves the finding
// order: the rule first, then the path, then the key, with equal findings equal.
func TestCompareEnvironmentFindingsOrdersByRuleThenPathThenKey(t *testing.T) {
	base := environmentFinding{rule: "a", path: "p", key: "k"}
	cases := []struct {
		name        string
		left, right environmentFinding
		want        int
	}{
		{"an earlier rule sorts first", environmentFinding{rule: "a"}, environmentFinding{rule: "b"}, -1},
		{"a later rule sorts last", environmentFinding{rule: "b"}, environmentFinding{rule: "a"}, 1},
		{"an earlier path sorts first", environmentFinding{rule: "a", path: "a"}, environmentFinding{rule: "a", path: "b"}, -1},
		{"a later path sorts last", environmentFinding{rule: "a", path: "b"}, environmentFinding{rule: "a", path: "a"}, 1},
		{"an earlier key sorts first", environmentFinding{rule: "a", path: "p", key: "a"}, environmentFinding{rule: "a", path: "p", key: "b"}, -1},
		{"a later key sorts last", environmentFinding{rule: "a", path: "p", key: "b"}, environmentFinding{rule: "a", path: "p", key: "a"}, 1},
		{"equal findings compare equal", base, base, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := compareEnvironmentFindings(tc.left, tc.right); got != tc.want {
				t.Fatalf("compare = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestIsRelativeFileRefusesAnAbsoluteOrBlankPath(t *testing.T) {
	for _, path := range []string{"", "   ", "/etc/passwd"} {
		if isRelativeFile(path) {
			t.Fatalf("isRelativeFile(%q) accepted a path that is not inside the root", path)
		}
	}
	if !isRelativeFile(".env.example") {
		t.Fatal("a plain relative file was refused")
	}
}

func TestIsEnvironmentKeyNeedsALetterOrUnderscoreToStart(t *testing.T) {
	if isEnvironmentKey("") {
		t.Fatal("an empty key was accepted")
	}
	if isEnvironmentKey("1ABC") {
		t.Fatal("a key that starts with a digit was accepted")
	}
	if !isEnvironmentKey("_ABC_1") {
		t.Fatal("a valid key was refused")
	}
}

func TestDetectorAccessesReadsNothingForAnUnknownLanguage(t *testing.T) {
	keys, ok := detectorAccesses("cobol", "")
	if ok || keys != nil {
		t.Fatalf("an unknown language yielded keys %v, %v", keys, ok)
	}
}

func TestIsUpperKeyRefusesALowercaseLetter(t *testing.T) {
	if isUpperKey("Abc") {
		t.Fatal("a key with a lowercase letter was read as upper case")
	}
	if !isUpperKey("KEY_1") {
		t.Fatal("an upper-case key was refused")
	}
}

// TestCompileStagedRefusesAnAllowedPatternThatDoesNotCompile proves the allowed
// list is compiled as patterns just as the forbidden list is, so an allowed
// pattern the matcher cannot read refuses the guard.
func TestCompileStagedRefusesAnAllowedPatternThatDoesNotCompile(t *testing.T) {
	_, _, err := compileStaged(&stagedPolicy{allowed: []string{"["}})
	if err == nil {
		t.Fatal("an uncompilable allowed pattern compiled")
	}
}
