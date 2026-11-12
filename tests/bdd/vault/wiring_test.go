package vault_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// suiteRunners are the adapter runners that call vault.RunSuite on the corpora.
// Each is checked by its syntax tree: a Test function must call RunSuite for
// every corpus in suiteCorpora, and no Test function may skip before that call.
// The compliance check proves the shared registry binds each corpus; this check
// proves each runner uses the registry on every corpus and cannot skip it.
var suiteRunners = []string{
	"../../../internal/application/environment_vault_behaviour_test.go",
	"../../../internal/adapters/runtime/environment_vault_behaviour_test.go",
}

// suiteCorpora are the constants each runner passes to vault.RunSuite as the
// corpus path: the validation corpus and the command corpus.
var suiteCorpora = []string{"environmentVaultCorpus", "environmentVaultCommandsCorpus"}

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
// the corpora: a corpus no Test function passes to vault.RunSuite, or a Test
// function that calls a skip method on t before that call. A parse failure is a
// problem too.
func suiteProblems(source string, corpora []string) []string {
	file, err := parser.ParseFile(token.NewFileSet(), "runner.go", source, 0)
	if err != nil {
		return []string{"does not parse: " + err.Error()}
	}
	var problems []string
	ran := map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}
		skipAt := token.NoPos
		var runs []*ast.CallExpr
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
			case receiver.Name == "vault" && selector.Sel.Name == "RunSuite":
				runs = append(runs, call)
			case receiver.Name == "t" && isSkip(selector.Sel.Name) && !skipAt.IsValid():
				skipAt = call.Pos()
			}
			return true
		})
		for _, run := range runs {
			if skipAt.IsValid() && skipAt < run.Pos() {
				problems = append(problems, fn.Name.Name+" skips before vault.RunSuite")
			}
			if len(run.Args) >= 2 {
				if corpus, ok := run.Args[1].(*ast.Ident); ok {
					ran[corpus.Name] = true
				}
			}
		}
	}
	if len(ran) == 0 {
		problems = append(problems, "no Test function calls vault.RunSuite")
	}
	for _, corpus := range corpora {
		if !ran[corpus] {
			problems = append(problems, "no Test function runs the corpus "+corpus+" through vault.RunSuite")
		}
	}
	return problems
}

// isSkip reports whether a testing.T method skips the running test.
func isSkip(method string) bool {
	return method == "Skip" || method == "Skipf" || method == "SkipNow"
}

// TestEveryRunnerDrivesTheCorpus fails when any adapter's runner stops calling
// the shared registry on any corpus, naming the runner and the problem.
func TestEveryRunnerDrivesTheCorpus(t *testing.T) {
	for _, file := range suiteRunners {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, problem := range suiteProblems(string(source), suiteCorpora) {
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
// fires on each way a runner can avoid a corpus.
func TestSuiteProblemsRejectARunnerThatSkipsOrNeverRuns(t *testing.T) {
	both := []string{"corpusA", "corpusB"}
	cases := []struct {
		name   string
		source string
	}{
		{"skips before RunSuite", "package x\n\nfunc TestUnit(t *testing.T) {\n\tt.Skip()\n\tvault.RunSuite(t, corpusA, nil)\n\tvault.RunSuite(t, corpusB, nil)\n}\n"},
		{"skips with Skipf before RunSuite", "package x\n\nfunc TestUnit(t *testing.T) {\n\tt.Skipf(\"x\")\n\tvault.RunSuite(t, corpusA, nil)\n\tvault.RunSuite(t, corpusB, nil)\n}\n"},
		{"skips with SkipNow before RunSuite", "package x\n\nfunc TestUnit(t *testing.T) {\n\tt.SkipNow()\n\tvault.RunSuite(t, corpusA, nil)\n\tvault.RunSuite(t, corpusB, nil)\n}\n"},
		{"never calls RunSuite", "package x\n\nfunc TestUnit(t *testing.T) {\n\tt.Log(\"no corpus\")\n}\n"},
		{"calls RunSuite outside a Test function", "package x\n\nfunc helper(t *testing.T) {\n\tvault.RunSuite(t, corpusA, nil)\n}\n"},
		{"omits the second corpus", "package x\n\nfunc TestUnit(t *testing.T) {\n\tvault.RunSuite(t, corpusA, nil)\n}\n"},
		{"runs a corpus without naming it", "package x\n\nfunc TestUnit(t *testing.T) {\n\tvault.RunSuite(t, \"x.feature\", nil)\n\tvault.RunSuite(t, corpusB, nil)\n}\n"},
		{"does not parse", "package x\n\nfunc ("},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if problems := suiteProblems(tc.source, both); len(problems) == 0 {
				t.Fatal("a runner that cannot run the corpora was accepted")
			}
		})
	}
}

// TestSuiteProblemsAcceptsARunnerThatRunsTheCorpora proves the check passes a
// runner that calls RunSuite on every corpus, and ignores skips in a helper that
// no Test function calls.
func TestSuiteProblemsAcceptsARunnerThatRunsTheCorpora(t *testing.T) {
	both := []string{"corpusA", "corpusB"}
	cases := []struct {
		name   string
		source string
	}{
		{"calls RunSuite on each corpus with no skip", "package x\n\nfunc TestUnit(t *testing.T) {\n\tvault.RunSuite(t, corpusA, nil)\n\tvault.RunSuite(t, corpusB, nil)\n}\n"},
		{"skips in a helper that no Test calls", "package x\n\nfunc helper(t *testing.T) {\n\tt.Skip()\n}\n\n" +
			"func TestUnit(t *testing.T) {\n\tvault.RunSuite(t, corpusA, nil)\n}\n\n" +
			"func TestCommands(t *testing.T) {\n\tvault.RunSuite(t, corpusB, nil)\n}\n"},
		{"declares a Test without a body", "package x\n\nfunc TestLinked(t *testing.T)\n\n" +
			"func TestUnit(t *testing.T) {\n\tvault.RunSuite(t, corpusA, nil)\n\tvault.RunSuite(t, corpusB, nil)\n}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if problems := suiteProblems(tc.source, both); len(problems) != 0 {
				t.Fatalf("a runner that runs the corpora was rejected: %v", problems)
			}
		})
	}
}
