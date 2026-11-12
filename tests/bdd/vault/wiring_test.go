package vault_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// suiteRunners are the adapter runners that call vault.RunSuite on the corpus.
// Each is checked by its syntax tree: a Test function must call RunSuite, and
// no Test function may skip before that call. The compliance check proves the
// shared registry binds the corpus; this check proves each runner uses the
// registry and cannot skip it.
var suiteRunners = []string{
	"../../../internal/application/environment_vault_behaviour_test.go",
	"../../../internal/adapters/runtime/environment_vault_behaviour_test.go",
}

// textWiring names, for each runner that does not call RunSuite, the text that
// proves it drives this corpus through the shared registry.
var textWiring = []struct {
	file     string
	required []string
}{
	{file: "../behaviours_test.go", required: []string{"vault.WithDriver("}},
	{file: "../steps_test.go", required: []string{"vault.Bindings()"}},
}

// missingText returns every required text the source lacks.
func missingText(source string, required []string) []string {
	var missing []string
	for _, text := range required {
		if !strings.Contains(source, text) {
			missing = append(missing, text)
		}
	}
	return missing
}

// suiteProblems parses one runner source and reports each way it fails to run
// the corpus: no Test function calls vault.RunSuite, or a Test function calls a
// skip method on t before that call. A parse failure is a problem too.
func suiteProblems(source string) []string {
	file, err := parser.ParseFile(token.NewFileSet(), "runner.go", source, 0)
	if err != nil {
		return []string{"does not parse: " + err.Error()}
	}
	var problems []string
	called := false
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}
		runAt, skipAt := token.NoPos, token.NoPos
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			receiver, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			switch {
			case receiver.Name == "vault" && selector.Sel.Name == "RunSuite" && !runAt.IsValid():
				runAt = call.Pos()
			case receiver.Name == "t" && isSkip(selector.Sel.Name) && !skipAt.IsValid():
				skipAt = call.Pos()
			}
			return true
		})
		if !runAt.IsValid() {
			continue
		}
		called = true
		if skipAt.IsValid() && skipAt < runAt {
			problems = append(problems, fn.Name.Name+" skips before vault.RunSuite")
		}
	}
	if !called {
		problems = append(problems, "no Test function calls vault.RunSuite")
	}
	return problems
}

// isSkip reports whether a testing.T method skips the running test.
func isSkip(method string) bool {
	return method == "Skip" || method == "Skipf" || method == "SkipNow"
}

// TestEveryRunnerDrivesTheCorpus fails when any adapter's runner stops calling
// the shared registry on this corpus, naming the runner and the problem.
func TestEveryRunnerDrivesTheCorpus(t *testing.T) {
	for _, file := range suiteRunners {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, problem := range suiteProblems(string(source)) {
			t.Errorf("%s: %s", file, problem)
		}
	}
	for _, runner := range textWiring {
		source, err := os.ReadFile(runner.file)
		if err != nil {
			t.Fatal(err)
		}
		for _, text := range missingText(string(source), runner.required) {
			t.Errorf("%s does not contain %q, so it does not run the corpus", runner.file, text)
		}
	}
}

// TestSuiteProblemsRejectARunnerThatSkipsOrNeverRuns proves the syntax check
// fires on each way a runner can avoid the corpus.
func TestSuiteProblemsRejectARunnerThatSkipsOrNeverRuns(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{"skips before RunSuite", "package x\n\nfunc TestUnit(t *testing.T) {\n\tt.Skip()\n\tvault.RunSuite(t, c, nil)\n}\n"},
		{"skips with Skipf before RunSuite", "package x\n\nfunc TestUnit(t *testing.T) {\n\tt.Skipf(\"x\")\n\tvault.RunSuite(t, c, nil)\n}\n"},
		{"skips with SkipNow before RunSuite", "package x\n\nfunc TestUnit(t *testing.T) {\n\tt.SkipNow()\n\tvault.RunSuite(t, c, nil)\n}\n"},
		{"never calls RunSuite", "package x\n\nfunc TestUnit(t *testing.T) {\n\tt.Log(\"no corpus\")\n}\n"},
		{"calls RunSuite outside a Test function", "package x\n\nfunc helper(t *testing.T) {\n\tvault.RunSuite(t, c, nil)\n}\n"},
		{"does not parse", "package x\n\nfunc ("},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if problems := suiteProblems(tc.source); len(problems) == 0 {
				t.Fatal("a runner that cannot run the corpus was accepted")
			}
		})
	}
}

// TestSuiteProblemsAcceptsARunnerThatRunsTheCorpus proves the check passes a
// runner that calls RunSuite, and ignores skips and bodiless declarations outside
// the Test function that calls it.
func TestSuiteProblemsAcceptsARunnerThatRunsTheCorpus(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{"calls RunSuite with no skip", "package x\n\nfunc TestUnit(t *testing.T) {\n\tvault.RunSuite(t, c, nil)\n}\n"},
		{"skips in a helper that no Test calls", "package x\n\nfunc helper(t *testing.T) {\n\tt.Skip()\n}\n\n" +
			"func TestUnit(t *testing.T) {\n\tvault.RunSuite(t, c, nil)\n}\n"},
		{"declares a Test without a body", "package x\n\nfunc TestLinked(t *testing.T)\n\n" +
			"func TestUnit(t *testing.T) {\n\tvault.RunSuite(t, c, nil)\n}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if problems := suiteProblems(tc.source); len(problems) != 0 {
				t.Fatalf("a runner that runs the corpus was rejected: %v", problems)
			}
		})
	}
}
