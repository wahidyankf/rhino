package application_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// wiringConfig is a registry every wiring rule accepts: one entry on each local
// surface, each also on the pull-request replay, a hook directory, and a
// workflow directory.
const wiringEntries = "schema: rhino/repo-config/v2\n" +
	"gates:\n" +
	"  composition:\n" +
	"    pull-request:\n" +
	"      relation: exact\n" +
	"  entries:\n" +
	"    - id: format-staged\n" +
	"      command:\n" +
	"        executable: formatter\n" +
	"      run-on:\n" +
	"        pre-commit: {}\n" +
	"        pull-request: {}\n" +
	"    - id: commit-message-lint\n" +
	"      command:\n" +
	"        executable: linter\n" +
	"      run-on:\n" +
	"        commit-msg: {}\n" +
	"        pull-request: {}\n" +
	"    - id: leak-scan\n" +
	"      command:\n" +
	"        executable: scanner\n" +
	"      run-on:\n" +
	"        pre-push: {}\n" +
	"        pull-request: {}\n"

// wiringBlock is the gates.wiring section every wiring fixture shares: the hook
// directory and the workflow directory.
const wiringBlock = "  wiring:\n" +
	"    hooks:\n" +
	"      directory: .husky\n" +
	"    ci:\n" +
	"      directory: .github/workflows\n"

// wiringConfig is wiringEntries followed by the shared wiring block.
const wiringConfig = wiringEntries + wiringBlock

// wiringWorkflow calls the pull-request surface from one workflow step.
const wiringWorkflow = "on:\n" +
	"  pull_request:\n" +
	"jobs:\n" +
	"  quality-gate:\n" +
	"    runs-on: ubuntu-24.04\n" +
	"    steps:\n" +
	"      - run: go run ./cmd/rhino gate run --surface pull-request\n"

// wiringFiles is the tree every wiring rule accepts: three hooks that call the
// three local surfaces, and one workflow that calls pull-request.
func wiringFiles() map[string]string {
	return map[string]string{
		".husky/pre-commit":                     "exec go run ./cmd/rhino gate run --surface pre-commit\n",
		".husky/commit-msg":                     "exec go run ./cmd/rhino gate run --surface commit-msg --message-file \"$1\"\n",
		".husky/pre-push":                       "exec go run ./cmd/rhino gate run --surface pre-push --push-updates-stdin\n",
		".github/workflows/pr-quality-gate.yml": wiringWorkflow,
	}
}

// wiringScanConfig adds a second pre-push entry, wired and on the replay, so
// that pre-push holds an entry the default list does not allow.
const wiringScanConfig = wiringEntries +
	"    - id: public-scan\n" +
	"      command:\n" +
	"        executable: public-scanner\n" +
	"      run-on:\n" +
	"        pre-push: {}\n" +
	"        pull-request: {}\n" +
	wiringBlock

// wiringDeclaredList is wiringScanConfig with the pre-push list declared to
// allow both entries.
const wiringDeclaredList = wiringScanConfig +
	"    local-surfaces:\n" +
	"      pre-push: [leak-scan, public-scan]\n"

// wiringNarrowList is wiringScanConfig with the pre-push list declared to
// allow only leak-scan, so public-scan is outside it.
const wiringNarrowList = wiringScanConfig +
	"    local-surfaces:\n" +
	"      pre-push: [leak-scan]\n"

// wiringResult is one run of gate wiring validate in JSON: its exit code, the
// rule identifiers of its findings in order, the refusal code and message when
// it refused, and both streams.
type wiringResult struct {
	exit    int
	kinds   []string
	code    string
	message string
	stdout  string
	stderr  string
}

// runGateWiring runs `gate wiring validate` over one in-memory tree. A refusal
// returns its JSON error code and message; a run that reports findings returns
// their kinds.
func runGateWiring(t *testing.T, config string, files map[string]string, extra ...string) wiringResult {
	t.Helper()
	return runCommand(t, []string{"gate", "wiring", "validate"}, config, files, extra...)
}

// runCommand runs one validator command over one in-memory tree, with the same
// JSON reading for every validator. command names the command path.
func runCommand(t *testing.T, command []string, config string, files map[string]string, extra ...string) wiringResult {
	t.Helper()
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = config
	for path, content := range files {
		tree.files[path] = content
	}
	return runOnTree(t, tree, command, extra...)
}

// runOnTree runs one command over a prepared tree and reads its outcome.
func runOnTree(t *testing.T, tree *memoryTree, command []string, extra ...string) wiringResult {
	t.Helper()
	args := append(append([]string{}, command...), extra...)
	outcome := application.Execute(tree, args, nil)
	result := wiringResult{exit: outcome.ExitCode, stdout: outcome.Stdout, stderr: outcome.Stderr}
	if outcome.ExitCode == 2 {
		var refusal struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(outcome.Stderr), &refusal); err != nil {
			t.Fatalf("exit 2 with stderr that is not a refusal document: %v\nstderr: %s", err, outcome.Stderr)
		}
		result.code = refusal.Error.Code
		result.message = refusal.Error.Message
		return result
	}
	var document struct {
		Violations []struct {
			Kind string `json:"kind"`
		} `json:"violations"`
	}
	if err := json.Unmarshal([]byte(outcome.Stdout), &document); err != nil {
		t.Fatalf("exit %d with stdout that is not a findings document: %v\nstdout: %q\nstderr: %q",
			outcome.ExitCode, err, outcome.Stdout, outcome.Stderr)
	}
	for _, violation := range document.Violations {
		result.kinds = append(result.kinds, violation.Kind)
	}
	return result
}

// wiringHasKind reports whether a rule identifier appears among the findings.
func wiringHasKind(kinds []string, want string) bool {
	for _, kind := range kinds {
		if kind == want {
			return true
		}
	}
	return false
}

func setFile(path, content string) func(map[string]string) {
	return func(files map[string]string) { files[path] = content }
}

func dropFile(path string) func(map[string]string) {
	return func(files map[string]string) { delete(files, path) }
}

func TestGateWiringRules(t *testing.T) {
	cases := []struct {
		name   string
		config string
		mutate func(map[string]string)
		exit   int
		kinds  []string
		code   string
	}{
		{name: "W01 passes when every surface with entries is called", config: wiringConfig, exit: 0},
		{name: "W01 fails when a surface with entries is called by nothing", config: wiringConfig,
			mutate: setFile(".husky/pre-push", "exec echo pushed\n"), exit: 1, kinds: []string{"W01"}},
		{name: "W02 passes when every hook call names a declared surface", config: wiringConfig, exit: 0},
		{name: "W02 fails when a hook calls a surface nothing declares", config: wiringConfig,
			mutate: setFile(".husky/pre-rebase", "exec go run ./cmd/rhino gate run --surface main\n"),
			exit:   1, kinds: []string{"W02"}},
		{name: "W03 passes when a workflow step calls the pull-request surface", config: wiringConfig, exit: 0},
		{name: "W03 fails when no workflow step calls the pull-request surface", config: wiringConfig,
			mutate: setFile(".github/workflows/pr-quality-gate.yml",
				"on:\n  pull_request:\njobs:\n  quality-gate:\n    runs-on: ubuntu-24.04\n    steps:\n      - run: echo checked\n"),
			exit: 1, kinds: []string{"W03"}},
		{name: "W04 passes when each local surface has its own hook file", config: wiringConfig, exit: 0},
		{name: "W04 fails when a local surface with entries has no hook file", config: wiringConfig,
			mutate: dropFile(".husky/pre-commit"), exit: 1, kinds: []string{"W04"}},
		{name: "W04 fails when the declared hook directory holds no file",
			config: strings.Replace(wiringConfig, "directory: .husky", "directory: .git-hooks", 1),
			exit:   1, kinds: []string{"W04"}},
		{name: "W05 passes when no gate call is suppressed", config: wiringConfig, exit: 0},
		{name: "W05 fails when a hook appends an or-true to a gate call", config: wiringConfig,
			mutate: setFile(".husky/pre-push", "exec go run ./cmd/rhino gate run --surface pre-push || true\n"),
			exit:   1, kinds: []string{"W05"}},
		{name: "W05 fails when a workflow step continues on error after a gate call", config: wiringConfig,
			mutate: setFile(".github/workflows/pr-quality-gate.yml",
				"on:\n  pull_request:\njobs:\n  quality-gate:\n    runs-on: ubuntu-24.04\n    steps:\n"+
					"      - run: go run ./cmd/rhino gate run --surface pull-request\n        continue-on-error: true\n"),
			exit: 1, kinds: []string{"W05"}},
		{name: "W06 passes when a workflow step installs the release its lock names", config: wiringConfig,
			mutate: func(files map[string]string) {
				files["hippo.lock"] = "version=v0.13.4\n"
				files[".github/workflows/pr-quality-gate.yml"] = wiringWorkflow +
					"      - run: gh release download v0.13.4 --repo wahidyankf/hippo\n"
			}, exit: 0},
		{name: "W06 fails when a workflow step installs a release other than its lock names", config: wiringConfig,
			mutate: func(files map[string]string) {
				files["hippo.lock"] = "version=v0.13.4\n"
				files[".github/workflows/pr-quality-gate.yml"] = wiringWorkflow +
					"      - run: gh release download v0.12.0 --repo wahidyankf/hippo\n"
			}, exit: 1, kinds: []string{"W06"}},
		{name: "W07 passes when a local surface holds only entries its declared list allows", config: wiringDeclaredList, exit: 0},
		{name: "W07 fails when a local surface holds an entry outside its default list", config: wiringScanConfig,
			exit: 1, kinds: []string{"W07"}},
		{name: "W07 fails when a declared list omits an entry the surface holds", config: wiringNarrowList,
			exit: 1, kinds: []string{"W07"}},
		{name: "a repository with no hosted workflows passes with a reason and no entries",
			config: "schema: rhino/repo-config/v2\ngates:\n  entries: []\n  wiring:\n    hooks:\n      directory: .git-hooks\n    ci:\n      none: This repository hosts no workflows.\n",
			exit:   0},
		{name: "a repository that declares no wiring is refused", config: "schema: rhino/repo-config/v2\ngates:\n  entries: []\n",
			exit: 2, code: "rhino.config.undeclared"},
		{name: "a workflow directory declared together with a reason of no workflows is refused",
			config: wiringConfig + "      none: This repository hosts no workflows.\n", exit: 2, code: "rhino.config.unusable"},
		{name: "a reason of no workflows with no text is refused",
			config: "schema: rhino/repo-config/v2\ngates:\n  entries: []\n  wiring:\n    hooks:\n      directory: .husky\n    ci:\n      none: \"\"\n",
			exit:   2, code: "rhino.config.unusable"},
		{name: "a wiring section with no hook directory is refused",
			config: "schema: rhino/repo-config/v2\ngates:\n  entries: []\n  wiring:\n    ci:\n      none: No workflows.\n",
			exit:   2, code: "rhino.config.unusable"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			files := wiringFiles()
			if test.mutate != nil {
				test.mutate(files)
			}
			result := runGateWiring(t, test.config, files, "--output", "json")
			if result.exit != test.exit {
				t.Fatalf("exit %d; want %d\nstdout: %s\nstderr: %s", result.exit, test.exit, result.stdout, result.stderr)
			}
			if test.code != "" && result.code != test.code {
				t.Fatalf("refusal code %q; want %q: %s", result.code, test.code, result.message)
			}
			for _, kind := range test.kinds {
				if !wiringHasKind(result.kinds, kind) {
					t.Fatalf("findings %v do not include %s", result.kinds, kind)
				}
			}
			if test.exit == 0 && len(result.kinds) != 0 {
				t.Fatalf("clean run reported findings %v", result.kinds)
			}
		})
	}
}

// TestGateWiringTextNamesRuleAndEntry proves the text rendering names the path,
// the rule, the field, and the entry and surface a W07 finding is about.
func TestGateWiringTextNamesRuleAndEntry(t *testing.T) {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = wiringScanConfig
	for path, content := range wiringFiles() {
		tree.files[path] = content
	}
	outcome := application.Execute(tree, []string{"gate", "wiring", "validate"}, nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("exit %d; want 1\nstdout: %s\nstderr: %s", outcome.ExitCode, outcome.Stdout, outcome.Stderr)
	}
	for _, want := range []string{"repo-config.yml:", "W07", "public-scan", "pre-push"} {
		if !strings.Contains(outcome.Stderr, want) {
			t.Fatalf("stderr does not contain %q:\n%s", want, outcome.Stderr)
		}
	}
}

// TestGateWiringRefusesRulesFlag proves the leaf does not accept the behaviour
// command's `--rules` filter, and says so by naming the leaf.
func TestGateWiringRefusesRulesFlag(t *testing.T) {
	result := runGateWiring(t, wiringConfig, wiringFiles(), "--rules", "W01", "--output", "json")
	if result.exit != 2 || !strings.Contains(result.message, "`--rules` is not accepted by `gate wiring validate`") {
		t.Fatalf("exit %d message %q; want a refusal naming `--rules` and `gate wiring validate`", result.exit, result.message)
	}
}
