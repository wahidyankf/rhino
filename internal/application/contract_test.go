package application_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
	"github.com/wahidyankf/rhino/internal/domain"
)

type memoryTree struct {
	files           map[string]string
	unreadable      map[string]bool
	vanished        map[string]bool
	binary          map[string]bool
	links           map[string]bool
	root            string
	indexed         []string
	hasIndex        bool
	indexedRegular  []string
	hasRegularIndex bool
	changed         map[[2]string][]string
	changedRegular  map[[2]string][]string
	commitMessages  map[[2]string]string
	gitRefs         map[string]string
	hookMessageFile string
	hasHookFile     bool
}

func newMemoryTree(root string) *memoryTree {
	return &memoryTree{files: map[string]string{}, unreadable: map[string]bool{}, vanished: map[string]bool{}, binary: map[string]bool{}, links: map[string]bool{}, root: root, changed: map[[2]string][]string{}, changedRegular: map[[2]string][]string{}, commitMessages: map[[2]string]string{}, gitRefs: map[string]string{}}
}

// stagePaths sorts and deduplicates one staged selection, the way the
// oracle's memory tree does when a scenario states it.
func stagePaths(paths []string) []string {
	held := append([]string(nil), paths...)
	sort.Strings(held)
	out := held[:0]
	for _, path := range held {
		if len(out) == 0 || out[len(out)-1] != path {
			out = append(out, path)
		}
	}
	return out
}

// setIndexedFiles stages the Git index snapshot the tree reports.
func (m *memoryTree) setIndexedFiles(paths ...string) {
	m.indexed = stagePaths(paths)
	m.hasIndex = true
}

// setIndexedRegularFiles stages the regular-file index snapshot.
func (m *memoryTree) setIndexedRegularFiles(paths ...string) {
	m.indexedRegular = stagePaths(paths)
	m.hasRegularIndex = true
}

// setChangedFiles stages the changed paths one explicit range reports.
func (m *memoryTree) setChangedFiles(base, head string, paths ...string) {
	m.changed[[2]string{base, head}] = stagePaths(paths)
}

// setChangedRegularFiles stages the regular-file paths one range reports.
func (m *memoryTree) setChangedRegularFiles(base, head string, paths ...string) {
	m.changedRegular[[2]string{base, head}] = stagePaths(paths)
}

// setCommitMessages stages the message text one explicit range reports.
func (m *memoryTree) setCommitMessages(base, head, messages string) {
	m.commitMessages[[2]string{base, head}] = messages
}

// setGitRef stages one fallback ref resolution.
func (m *memoryTree) setGitRef(reference, commit string) {
	m.gitRefs[reference] = commit
}

// setHookMessageFile names Git's canonical message file for the run.
func (m *memoryTree) setHookMessageFile(path string) {
	m.hookMessageFile = path
	m.hasHookFile = true
}

// markUnreadable stages a permission failure for one path.
func (m *memoryTree) markUnreadable(path string) {
	m.unreadable[path] = true
}

// markBinary stages non-text bytes for one path.
func (m *memoryTree) markBinary(path string) {
	m.binary[path] = true
}

func (m *memoryTree) Root() string { return m.root }

func (m *memoryTree) Files() []string {
	var out []string
	for path := range m.files {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func (m *memoryTree) Read(path string) (string, *application.ReadError) {
	if m.unreadable[path] {
		return "", &application.ReadError{Kind: "unreadable", Reason: path + ": permission denied"}
	}
	if m.vanished[path] {
		return "", &application.ReadError{Kind: "missing", Reason: path + ": no such file"}
	}
	if m.binary[path] {
		return "", &application.ReadError{Kind: "not-text", Reason: path + ": holds no text"}
	}
	text, ok := m.files[path]
	if !ok {
		return "", &application.ReadError{Kind: "missing", Reason: path + ": no such file"}
	}
	return text, nil
}

func (m *memoryTree) Children(directory string) []string {
	return application.ChildrenOf(m.Files(), directory)
}

func (m *memoryTree) IsDirectory(path string) bool {
	return application.IsDirectoryOf(m.Files(), path)
}

func (m *memoryTree) Exists(path string) bool {
	if _, ok := m.files[path]; ok && !m.unreadable[path] && !m.binary[path] {
		return true
	}
	prefix := path + "/"
	for file := range m.files {
		if len(file) > len(prefix) && file[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// Links reports no links: the in-memory tree models links only for the
// PassesThroughLink answers the validators read.
func (m *memoryTree) Links() []application.TreeLink { return nil }

func (m *memoryTree) PassesThroughLink(path string) bool {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return false
	}
	current := ""
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == "" || segment == "." {
			continue
		}
		if current == "" {
			current = segment
		} else {
			current += "/" + segment
		}
		if m.links[current] {
			return true
		}
	}
	return false
}

// IndexedFiles reports the staged Git index snapshot, or refuses without one.
func (m *memoryTree) IndexedFiles() ([]string, error) {
	if !m.hasIndex {
		return nil, errTest("this tree has no Git index snapshot boundary")
	}
	return append([]string(nil), m.indexed...), nil
}

// IndexedRegularFiles reports the staged regular-file snapshot.
func (m *memoryTree) IndexedRegularFiles() ([]string, error) {
	if !m.hasRegularIndex {
		return nil, errTest("this tree has no regular-file Git index snapshot boundary")
	}
	return append([]string(nil), m.indexedRegular...), nil
}

// ChangedFiles reports the staged changed paths for one explicit range.
func (m *memoryTree) ChangedFiles(base, head string) ([]string, error) {
	paths, ok := m.changed[[2]string{base, head}]
	if !ok {
		return nil, errTest("this tree has no changed-file range boundary")
	}
	return append([]string(nil), paths...), nil
}

// ChangedRegularFiles reports the staged regular-file paths for one range.
func (m *memoryTree) ChangedRegularFiles(base, head string) ([]string, error) {
	paths, ok := m.changedRegular[[2]string{base, head}]
	if !ok {
		return nil, errTest("this tree has no regular-file changed-file range boundary")
	}
	return append([]string(nil), paths...), nil
}

// ResolveGitRef resolves one staged fallback ref.
func (m *memoryTree) ResolveGitRef(reference string) (string, error) {
	commit, ok := m.gitRefs[reference]
	if !ok {
		return "", errTest("the declared fallback ref `" + reference + "` does not resolve")
	}
	return commit, nil
}

// CommitMessages reports the staged message text for one explicit range.
func (m *memoryTree) CommitMessages(base, head string) (string, error) {
	messages, ok := m.commitMessages[[2]string{base, head}]
	if !ok {
		return "", errTest("this tree has no commit-message range boundary")
	}
	return messages, nil
}

// HookMessageFile reads only Git's canonical message file for the run.
func (m *memoryTree) HookMessageFile(path string) (string, *application.ReadError) {
	if !m.hasHookFile {
		return "", &application.ReadError{Kind: "unreadable", Reason: "the Git hook message-file boundary is unavailable"}
	}
	if path != m.hookMessageFile {
		return "", &application.ReadError{Kind: "unreadable", Reason: "the supplied path is not Git's canonical COMMIT_EDITMSG"}
	}
	if m.unreadable[path] {
		return "", &application.ReadError{Kind: "unreadable", Reason: "permission denied"}
	}
	if m.binary[path] {
		return "", &application.ReadError{Kind: "not-text", Reason: path + ": holds no text"}
	}
	if m.vanished[path] {
		return "", &application.ReadError{Kind: "missing", Reason: path + ": no such file"}
	}
	text, ok := m.files[path]
	if !ok {
		return "", &application.ReadError{Kind: "missing", Reason: path + ": no such file"}
	}
	return text, nil
}

func (m *memoryTree) RootedAt(path string) (application.Tree, error) {
	if path == "missing" {
		return nil, errTest("`--root missing` names nothing usable")
	}
	return &memoryTree{files: m.files, unreadable: m.unreadable, binary: m.binary, root: m.root + "/" + path}, nil
}

type testError string

func errTest(message string) error { return testError(message) }

func (e testError) Error() string { return string(e) }

func args(words ...string) []string { return words }

func TestParseCoversEveryLeaf(t *testing.T) {
	commands := [][]string{
		{"repo-config", "validate"},
		{"gate", "run", "--surface", "pre-commit"},
		{"gate", "list"},
		{"gate", "validate"},
		{"governance", "vendor", "validate"},
		{"governance", "layers", "validate"},
		{"governance", "traceability", "validate"},
		{"governance", "quality-gates", "validate"},
		{"governance", "word-budget", "validate"},
		{"governance", "directory-map", "validate", "--directory", "docs"},
		{"harness", "adapters", "validate"},
		{"harness", "adapters", "generate"},
		{"env", "backup", "--dir", "backups"},
		{"env", "validate"},
		{"env", "init", "--apply"},
		{"env", "restore", "--dir", "backups", "--force"},
		{"toolchain", "validate"},
		{"toolchain", "provision", "--apply"},
		{"md", "frontmatter", "validate"},
		{"md", "heading-hierarchy", "validate"},
		{"metadata", "validate"},
		{"md", "internal-link", "validate"},
		{"md", "mermaid", "validate", "--file", "docs/a.md"},
		{"md", "naming", "validate"},
		{"md", "readme-index", "validate"},
		{"convention", "emoji", "validate"},
		{"convention", "license", "validate"},
		{"version", "--json"},
	}
	for _, command := range commands {
		parsed, refusal := application.Parse(args(command...))
		if refusal != nil {
			t.Fatalf("%v refused: %v", command, refusal)
		}
		if parsed.Run == nil {
			t.Fatalf("%v did not run", command)
		}
	}
}

func TestParseRefusesUnknownCommandFlagAndLeafFlag(t *testing.T) {
	if _, refusal := application.Parse(args("nope", "validate")); refusal == nil || refusal.Code != domain.ArgsUnrecognized {
		t.Fatalf("unknown command was not refused as unrecognized: %+v", refusal)
	}
	if _, refusal := application.Parse(args("repo-config", "validate", "--no-such-flag")); refusal == nil || refusal.Code != domain.ArgsUnrecognized {
		t.Fatalf("unknown flag was not refused as unrecognized: %+v", refusal)
	}
	parsed, refusal := application.Parse(args("repo-config", "validate", "--file", "x.md"))
	if refusal == nil || !strings.Contains(refusal.Message, "`--file` is not accepted") {
		t.Fatalf("leaf flag was not refused: %+v parsed=%+v", refusal, parsed)
	}
}

func TestParseRefusesContradictionsAndIncompleteRanges(t *testing.T) {
	if _, refusal := application.Parse(args("--json", "--output", "text", "version")); refusal == nil || refusal.Code != domain.ArgsIncomplete {
		t.Fatalf("json contradiction was not refused: %+v", refusal)
	}
	if _, refusal := application.Parse(args("gate", "run", "--surface", "pull-request", "--base", "a")); refusal == nil || refusal.Code != domain.ArgsIncomplete {
		t.Fatalf("half range was not refused: %+v", refusal)
	}
	if _, refusal := application.Parse(args("gate", "run", "--surface", "commit-msg", "--message-file", "m", "--push-updates-stdin")); refusal == nil {
		t.Fatalf("cross-surface combination was not refused")
	}
}

func TestParseRetiredLeavesAreUnknown(t *testing.T) {
	for _, command := range [][]string{
		{"governance", "roots", "validate"},
		{"governance", "companions", "validate"},
		{"governance", "instructions", "validate"},
		{"plan", "validate"},
		{"harness", "parity", "validate"},
		{"md", "word-count", "inspect"},
		{"repo-config", "migrate"},
		{"gate", "emit"},
	} {
		if _, refusal := application.Parse(args(command...)); refusal == nil || !strings.Contains(refusal.Message, "unrecognized command") {
			t.Fatalf("%v was not refused as unrecognized: %+v", command, refusal)
		}
	}
}

func TestHelpAnswersEverySpelling(t *testing.T) {
	for _, command := range [][]string{{"--help"}, {"-h"}, {"help"}, {"md", "--help"}, {"--version"}} {
		parsed, refusal := application.Parse(args(command...))
		if refusal != nil {
			t.Fatalf("%v refused: %v", command, refusal)
		}
		if command[0] == "--version" {
			if parsed.Run == nil || parsed.Run.Category != "version" {
				t.Fatalf("--version did not become version: %+v", parsed.Run)
			}
			continue
		}
		if parsed.Help == nil {
			t.Fatalf("%v did not answer help", command)
		}
	}
}

func TestExecuteVersionShapes(t *testing.T) {
	tree := newMemoryTree("/root")
	text := application.Execute(tree, args("version"), nil)
	if text.ExitCode != 0 || !strings.HasPrefix(text.Stdout, "v") {
		t.Fatalf("unexpected version text: %+v", text)
	}
	json := application.Execute(tree, args("version", "--json"), nil)
	if json.ExitCode != 0 || !strings.Contains(json.Stdout, "\"schemaVersion\":1") || !strings.Contains(json.Stdout, "\"commit\"") {
		t.Fatalf("unexpected version json: %+v", json)
	}
}

func TestExecuteGroupedContracts(t *testing.T) {
	grouped := func(text string) *memoryTree {
		tree := newMemoryTree("/root")
		tree.files["repo-config.yml"] = text
		return tree
	}
	clean := application.Execute(grouped("schema: rhino/repo-config/v2\n"), args("repo-config", "validate"), nil)
	if clean.ExitCode != 0 {
		t.Fatalf("grouped config refused: %+v", clean)
	}
	unknown := application.Execute(grouped("schema: rhino/repo-config/v2\nunknown-core: {}\n"), args("repo-config", "validate"), nil)
	if unknown.ExitCode != 2 || !strings.Contains(unknown.Stderr, "unknown core key") {
		t.Fatalf("unknown core key not refused: %+v", unknown)
	}
	legacy := application.Execute(grouped("# schema: rhino/repo-config/v1\n"), args("repo-config", "validate"), nil)
	if legacy.ExitCode != 2 || !strings.Contains(legacy.Stderr, "no longer accepts predecessor configuration") {
		t.Fatalf("predecessor not refused: %+v", legacy)
	}
	badOwner := application.Execute(grouped("schema: rhino/repo-config/v2\nextensions:\n  Invalid_owner: {}\n"), args("repo-config", "validate"), nil)
	if badOwner.ExitCode != 2 || !strings.Contains(badOwner.Stderr, "extension owner") {
		t.Fatalf("bad extension owner not refused: %+v", badOwner)
	}
	omitted := application.Execute(grouped("schema: rhino/repo-config/v2\n"), args("md", "heading-hierarchy", "validate"), nil)
	if omitted.ExitCode != 2 || !strings.Contains(omitted.Stderr, "policies.markdown.heading-hierarchy") {
		t.Fatalf("omitted policy not refused at owner: %+v", omitted)
	}
	duplicated := application.Execute(grouped("schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: a\n      run-on:\n        main: {}\n    - id: a\n      run-on:\n        scheduled: {}\n"), args("repo-config", "validate"), nil)
	if duplicated.ExitCode != 2 || !strings.Contains(duplicated.Stderr, "duplicated") {
		t.Fatalf("duplicated ids not refused: %+v", duplicated)
	}
	missingPR := application.Execute(grouped("schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: local-check\n      run-on:\n        pre-commit: {}\n  composition:\n    pull-request:\n      relation: exact\n"), args("gate", "validate"), nil)
	if missingPR.ExitCode != 2 || !strings.Contains(missingPR.Stderr, "missing from pull-request composition") {
		t.Fatalf("exact composition gap not refused: %+v", missingPR)
	}
}

func TestExecuteGateViews(t *testing.T) {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: lifecycle-check\n      run-on:\n        pre-commit: {}\n        commit-msg: {}\n        pre-push: {}\n        pull-request: {}\n        main: {}\n        scheduled: {}\n        manual: {}\n  composition:\n    pull-request:\n      relation: exact\n"
	list := application.Execute(tree, args("gate", "list"), nil)
	if list.ExitCode != 0 || !strings.Contains(list.Stdout, "pre-commit: lifecycle-check") {
		t.Fatalf("unexpected gate list: %+v", list)
	}
	validate := application.Execute(tree, args("gate", "validate"), nil)
	if validate.ExitCode != 0 {
		t.Fatalf("gate validate refused: %+v", validate)
	}
	undeclared := newMemoryTree("/root")
	undeclared.files["repo-config.yml"] = "schema: rhino/repo-config/v2\n"
	if outcome := application.Execute(undeclared, args("gate", "list"), nil); outcome.ExitCode != 2 {
		t.Fatalf("omitted gates listed: %+v", outcome)
	}
}

func TestExecuteEmojiFindingsAndFaults(t *testing.T) {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = "schema: rhino/repo-config/v2\npolicies:\n  conventions:\n    emoji:\n      prohibited:\n        - glob: \"**/*.json\"\n"
	tree.files["config/a.json"] = "held \U0001F98F\n"
	findings := application.Execute(tree, args("convention", "emoji", "validate"), nil)
	if findings.ExitCode != 1 || !strings.Contains(findings.Stderr, "U+1F98F") {
		t.Fatalf("emoji finding missing: %+v", findings)
	}
	tree.files["config/a.json"] = "plain\n"
	clean := application.Execute(tree, args("convention", "emoji", "validate"), nil)
	if clean.ExitCode != 0 {
		t.Fatalf("clean tree reported findings: %+v", clean)
	}
	tree.unreadable["config/a.json"] = true
	tree.files["config/a.json"] = "held \U0001F98F\n"
	if outcome := application.Execute(tree, args("convention", "emoji", "validate"), nil); outcome.ExitCode != 2 {
		t.Fatalf("unreadable emoji file did not refuse: %+v", outcome)
	}
	binary := newMemoryTree("/root")
	binary.files["repo-config.yml"] = "schema: rhino/repo-config/v2\npolicies:\n  conventions:\n    emoji:\n      prohibited:\n        - glob: \"**/*.json\"\n"
	binary.files["config/a.json"] = "bytes"
	binary.binary["config/a.json"] = true
	if outcome := application.Execute(binary, args("convention", "emoji", "validate"), nil); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "holds no text") {
		t.Fatalf("binary emoji file did not refuse: %+v", outcome)
	}
}

func TestExecuteMissingConfigAndBadGlob(t *testing.T) {
	empty := newMemoryTree("/root")
	if outcome := application.Execute(empty, args("repo-config", "validate"), nil); outcome.ExitCode != 2 {
		t.Fatalf("missing config did not refuse: %+v", outcome)
	}
	bad := newMemoryTree("/root")
	bad.files["repo-config.yml"] = "schema: rhino/repo-config/v2\npolicies:\n  conventions:\n    emoji:\n      prohibited:\n        - glob: \"[\"\n"
	bad.files["x.json"] = "{}\n"
	if outcome := application.Execute(bad, args("convention", "emoji", "validate"), nil); outcome.ExitCode != 2 {
		t.Fatalf("bad glob did not refuse: %+v", outcome)
	}
}

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"**/*.json", "config/a.json", true},
		{"**/*.json", "config/a.md", false},
		{"docs/**", "docs/a/b.md", true},
		{"docs/*.md", "docs/a/b.md", false},
		{"*.md", "README.md", true},
		{"linked/**/*.json", "linked/a.json", true},
	}
	for _, tc := range cases {
		got, err := application.Match(tc.pattern, tc.path)
		if err != nil || got != tc.want {
			t.Fatalf("Match(%q,%q)=%v,%v want %v", tc.pattern, tc.path, got, err, tc.want)
		}
	}
	if _, err := application.Match("/abs", "a"); err == nil {
		t.Fatal("absolute pattern was not an error")
	}
	if _, err := application.Match("../up", "a"); err == nil {
		t.Fatal("climbing pattern was not an error")
	}
}
