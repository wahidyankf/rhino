package application

import "testing"

// These helpers answer their contract for inputs the audit reaches only with
// names no repository path produces in practice, so they are exercised directly.

func TestBaseNameOfAPathWithNoSeparatorIsThePath(t *testing.T) {
	if got := baseName("README.md"); got != "README.md" {
		t.Fatalf("baseName(README.md) = %q", got)
	}
}

func TestParentDirOfAPathWithNoSeparatorIsEmpty(t *testing.T) {
	if got := parentDir("README.md"); got != "" {
		t.Fatalf("parentDir(README.md) = %q, want empty", got)
	}
}

// TestIsModuleNameNeedsThreeDigitsAndAHyphen proves a numbered module name is
// three digits, a hyphen, a name, and the extension, and nothing shorter.
func TestIsModuleNameNeedsThreeDigitsAndAHyphen(t *testing.T) {
	for _, name := range []string{"1-a.md", "a00-x.md", "001x.md"} {
		if isModuleName(name) {
			t.Fatalf("isModuleName(%q) accepted a name that is not a numbered module", name)
		}
	}
	if !isModuleName("001-one.md") {
		t.Fatal("isModuleName refused a numbered module")
	}
}

func TestNamesQualityTokenNeedsAnEmptyTokenToFail(t *testing.T) {
	if namesQualityToken("the gate", "") {
		t.Fatal("an empty token named a quality token")
	}
}

// TestNamesQualityTokenRefusesATokenInsideALongerName proves a token that is
// only part of a longer name, on either side, is not that name.
func TestNamesQualityTokenRefusesATokenInsideALongerName(t *testing.T) {
	if namesQualityToken("xgate", "gate") {
		t.Fatal("a token preceded by a name character was named")
	}
	if namesQualityToken("gates", "gate") {
		t.Fatal("a token followed by a name character was named")
	}
	if !namesQualityToken("the gate here", "gate") {
		t.Fatal("a whole token was not named")
	}
}

func TestIsVerdictShapeRefusesAPunctuatedSpan(t *testing.T) {
	if isVerdictShape("A-B") {
		t.Fatal("a verdict with a hyphen was read as a verdict")
	}
	if !isVerdictShape("PASS_2") {
		t.Fatal("an uppercase verdict with a digit was not read as one")
	}
}

// TestWordBudgetLimitIgnoresAGlobThatCannotMatch proves a surface whose glob the
// matcher cannot compile governs no file, rather than refusing the audit mid-run.
func TestWordBudgetLimitIgnoresAGlobThatCannotMatch(t *testing.T) {
	budget := &wordBudget{surfaces: []wordBudgetSurface{{glob: "[", fail: 1}}}
	if limit := budget.limit("a.md"); limit != nil {
		t.Fatalf("an uncompilable glob governed a file with limit %d", *limit)
	}
}
