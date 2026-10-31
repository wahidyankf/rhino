package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
	"github.com/wahidyankf/rhino/internal/domain"
)

// envTree is a memory tree with link-aware reads and an optional Git index
// snapshot, the two boundaries environment operations depend on.
type envTree struct {
	root     string
	files    map[string]string
	unread   map[string]bool
	binary   map[string]bool
	links    map[string]bool
	indexed  []string
	hasIndex bool
	writes   int
}

func newEnvTree() *envTree {
	return &envTree{root: "/root", files: map[string]string{}, unread: map[string]bool{}, binary: map[string]bool{}, links: map[string]bool{}}
}

func (m *envTree) Root() string { return m.root }

func (m *envTree) Files() []string {
	var out []string
	for path := range m.files {
		out = append(out, path)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func (m *envTree) Read(path string) (string, *application.ReadError) {
	if m.links[path] || m.throughLink(path) {
		return "", &application.ReadError{Kind: "unreadable", Reason: path + " passes through a symbolic link"}
	}
	if m.unread[path] {
		return "", &application.ReadError{Kind: "unreadable", Reason: path + ": permission denied"}
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

func (m *envTree) throughLink(path string) bool {
	for link := range m.links {
		if path != link && strings.HasPrefix(path, link+"/") {
			return true
		}
	}
	return false
}

func (m *envTree) Children(directory string) []string {
	return application.ChildrenOf(m.Files(), directory)
}

func (m *envTree) IsDirectory(path string) bool {
	return application.IsDirectoryOf(m.Files(), path)
}

func (m *envTree) Exists(path string) bool {
	if _, ok := m.files[path]; ok {
		return true
	}
	prefix := path + "/"
	for file := range m.files {
		if strings.HasPrefix(file, prefix) {
			return true
		}
	}
	return false
}

// Links reports no links: the environment fixture models only the links its
// PassesThroughLink answers.
func (m *envTree) Links() []application.TreeLink { return nil }

func (m *envTree) PassesThroughLink(path string) bool {
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

func (m *envTree) RootedAt(path string) (application.Tree, error) {
	return m, nil
}

func (m *envTree) IndexedFiles() ([]string, error) {
	if !m.hasIndex {
		return nil, errTest("this tree has no Git index snapshot boundary")
	}
	return append([]string(nil), m.indexed...), nil
}

// memoryEnvStore applies declared writes to its tree, refusing the shapes a
// disk store refuses: an existing target and a file blocking a parent.
type memoryEnvStore struct {
	tree     *envTree
	creates  int
	restores int
	refuse   string
}

func (s *memoryEnvStore) Create(_ string, files []application.EnvironmentFile) error {
	s.creates++
	if s.refuse != "" {
		return errTest(s.refuse)
	}
	for _, file := range files {
		if _, ok := s.tree.files[file.Path]; ok {
			return errTest("environment target `" + file.Path + "` already exists")
		}
		if parent := blockingFile(s.tree.files, file.Path); parent != "" {
			return errTest("cannot create environment parent `" + file.Path + "`: `" + parent + "` is a file")
		}
	}
	for _, file := range files {
		s.tree.files[file.Path] = file.Contents
	}
	return nil
}

func (s *memoryEnvStore) Restore(_ string, files []application.EnvironmentRestoreFile) error {
	s.restores++
	if s.refuse != "" {
		return errTest(s.refuse)
	}
	for _, file := range files {
		if _, ok := s.tree.files[file.Path]; ok && !file.Replace {
			return errTest("environment target `" + file.Path + "` already exists")
		}
		if parent := blockingFile(s.tree.files, file.Path); parent != "" {
			return errTest("cannot create environment parent `" + file.Path + "`: `" + parent + "` is a file")
		}
	}
	for _, file := range files {
		s.tree.files[file.Path] = file.Contents
	}
	return nil
}

func blockingFile(files map[string]string, path string) string {
	parts := strings.Split(path, "/")
	for i := 1; i < len(parts); i++ {
		parent := strings.Join(parts[:i], "/")
		if _, ok := files[parent]; ok {
			return parent
		}
	}
	return ""
}

func envConfig(body string) *envTree {
	tree := newEnvTree()
	tree.files["repo-config.yml"] = "schema: rhino/repo-config/v2\n" + body
	return tree
}

func runEnv(tree *envTree, store *memoryEnvStore, words ...string) application.Outcome {
	var s application.EnvironmentStore
	if store != nil {
		s = store
	}
	return application.ExecuteWithStores(tree, args(words...), nil, nil, s)
}

func TestEnvironmentInitPlansThenAppliesDeclaredTargets(t *testing.T) {
	tree := envConfig("environment:\n  examples:\n    - source: .env.example\n      target: .env.generated\n")
	tree.files[".env.example"] = "EXAMPLE_KEY=placeholder\n"
	store := &memoryEnvStore{tree: tree}

	planned := runEnv(tree, store, "env", "init")
	if planned.ExitCode != 0 || !strings.Contains(planned.Stdout, "[environment-init] planned 1 declared targets; pass --apply to write") {
		t.Fatalf("plan not reported: %+v", planned)
	}
	if _, ok := tree.files[".env.generated"]; ok {
		t.Fatal("plan wrote its target")
	}
	if store.creates != 0 {
		t.Fatalf("plan reached the write boundary: %d", store.creates)
	}

	plannedJSON := runEnv(tree, store, "env", "init", "--output", "json")
	if plannedJSON.ExitCode != 0 || plannedJSON.Stdout != "{\"command\":\"environment-init\",\"count\":1,\"schemaVersion\":1,\"status\":\"planned\",\"targets\":[\".env.generated\"]}\n" {
		t.Fatalf("plan json differs: %+v", plannedJSON)
	}

	applied := runEnv(tree, store, "env", "init", "--apply")
	if applied.ExitCode != 0 || !strings.Contains(applied.Stdout, "[environment-init] created declared targets") {
		t.Fatalf("apply not reported: %+v", applied)
	}
	if tree.files[".env.generated"] != "EXAMPLE_KEY=placeholder\n" {
		t.Fatalf("apply wrote wrong bytes: %q", tree.files[".env.generated"])
	}

	appliedJSON := runEnv(tree, store, "env", "init", "--apply", "--output", "json")
	if appliedJSON.ExitCode != 2 || !strings.Contains(appliedJSON.Stderr, "already exists") {
		t.Fatalf("repeat apply did not refuse: %+v", appliedJSON)
	}
}

func TestEnvironmentInitRefusesUnusablePolicy(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config string
		setup  func(*envTree)
		code   domain.ErrorCode
		want   string
	}{
		{"absent section", "", nil, domain.ConfigUndeclared, "environment section is not declared"},
		{"empty examples", "environment: {}\n", nil, domain.ConfigUndeclared, "declares no example targets"},
		{"escaping source", "environment:\n  examples:\n    - source: ../outside\n      target: .env.generated\n", nil, domain.PathEscapesRoot, "leaves the repository root"},
		{"same source and target", "environment:\n  examples:\n    - source: .env.example\n      target: .env.example\n", func(tr *envTree) { tr.files[".env.example"] = "x\n" }, domain.ConfigUnusable, "source and target are the same"},
		{"duplicate target", "environment:\n  examples:\n    - source: a.example\n      target: .env.generated\n    - source: b.example\n      target: .env.generated\n", func(tr *envTree) { tr.files["a.example"] = "a\n"; tr.files["b.example"] = "b\n" }, domain.ConfigUnusable, "declared more than once"},
		{"missing source", "environment:\n  examples:\n    - source: missing.example\n      target: .env.generated\n", nil, domain.FileMissing, "is missing"},
		{"unreadable source", "environment:\n  examples:\n    - source: .env.example\n      target: .env.generated\n", func(tr *envTree) { tr.files[".env.example"] = "x\n"; tr.unread[".env.example"] = true }, domain.FileUnreadable, "is unreadable"},
		{"link source", "environment:\n  examples:\n    - source: .env.example\n      target: .env.generated\n", func(tr *envTree) { tr.files[".env.example"] = "x\n"; tr.links[".env.example"] = true }, domain.FileUnreadable, "is unreadable"},
	} {
		tree := envConfig(tc.config)
		if tc.setup != nil {
			tc.setup(tree)
		}
		outcome := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "init", "--apply", "--output", "json")
		if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, tc.want) || !strings.Contains(outcome.Stderr, tc.code.String()) {
			t.Fatalf("%s: %+v", tc.name, outcome)
		}
		if _, ok := tree.files[".env.generated"]; ok {
			t.Fatalf("%s wrote its target", tc.name)
		}
	}
}

func TestEnvironmentBackupRequiresExplicitDirAndCopiesTargets(t *testing.T) {
	configured := func() (*envTree, *memoryEnvStore) {
		tree := envConfig("environment:\n  examples:\n    - source: .env.example\n      target: .env.saved\n")
		tree.files[".env.saved"] = "SAVED_KEY=placeholder\n"
		return tree, &memoryEnvStore{tree: tree}
	}

	tree, store := configured()
	if outcome := runEnv(tree, store, "env", "backup"); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "requires --dir") {
		t.Fatalf("missing --dir not refused: %+v", outcome)
	}

	for _, dir := range []string{"", "/outside", "../outside", "safe/../../outside"} {
		tree, store := configured()
		outcome := runEnv(tree, store, "env", "backup", "--dir", dir)
		if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "backup destination leaves the repository root") {
			t.Fatalf("dir %q not refused: %+v", dir, outcome)
		}
		if store.creates != 0 {
			t.Fatalf("dir %q reached the write boundary", dir)
		}
	}

	tree, store = configured()
	done := runEnv(tree, store, "env", "backup", "--dir", "saved")
	if done.ExitCode != 0 || done.Stdout != "[environment-backup] completed 1 declared files into saved\n" {
		t.Fatalf("backup text differs: %+v", done)
	}
	if tree.files["saved/.env.saved"] != "SAVED_KEY=placeholder\n" {
		t.Fatalf("backup wrote wrong bytes: %q", tree.files["saved/.env.saved"])
	}

	tree, store = configured()
	empty := envConfig("environment: {}\n")
	emptyStore := &memoryEnvStore{tree: empty}
	zero := runEnv(empty, emptyStore, "env", "backup", "--dir", "saved", "--output", "json")
	if zero.ExitCode != 0 || zero.Stdout != "{\"command\":\"environment-backup\",\"count\":0,\"destination\":\"saved\",\"files\":[],\"schemaVersion\":1,\"status\":\"completed\"}\n" {
		t.Fatalf("empty backup json differs: %+v", zero)
	}
	if emptyStore.creates != 0 {
		t.Fatal("empty backup reached the write boundary")
	}

	missing := envConfig("environment:\n  examples:\n    - source: .env.example\n      target: .env.never-created\n")
	if outcome := runEnv(missing, &memoryEnvStore{tree: missing}, "env", "backup", "--dir", "saved"); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "target `.env.never-created` is missing") {
		t.Fatalf("missing target not refused: %+v", outcome)
	}
}

func TestEnvironmentRestoreReplacesOnlyWithForce(t *testing.T) {
	configured := func() (*envTree, *memoryEnvStore) {
		tree := envConfig("environment:\n  examples:\n    - source: .env.example\n      target: .env.fixture\n")
		tree.files["saved/.env.fixture"] = "SYNTHETIC=restored\n"
		tree.files[".env.fixture"] = "SYNTHETIC=current\n"
		return tree, &memoryEnvStore{tree: tree}
	}

	tree, store := configured()
	if outcome := runEnv(tree, store, "env", "restore", "--dir", "saved"); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "use --force") {
		t.Fatalf("conflict without --force not refused: %+v", outcome)
	}
	if tree.files[".env.fixture"] != "SYNTHETIC=current\n" {
		t.Fatal("conflict overwrote its target")
	}

	tree, store = configured()
	restored := runEnv(tree, store, "env", "restore", "--dir", "saved", "--force")
	if restored.ExitCode != 0 || restored.Stdout != "[environment-restore] restored declared targets\n" {
		t.Fatalf("restore text differs: %+v", restored)
	}
	if tree.files[".env.fixture"] != "SYNTHETIC=restored\n" {
		t.Fatalf("restore wrote wrong bytes: %q", tree.files[".env.fixture"])
	}

	tree, store = configured()
	json := runEnv(tree, store, "env", "restore", "--dir", "saved", "--force", "--output", "json")
	if json.ExitCode != 0 || json.Stdout != "{\"command\":\"environment-restore\",\"count\":1,\"schemaVersion\":1,\"status\":\"completed\",\"targets\":[\".env.fixture\"]}\n" {
		t.Fatalf("restore json differs: %+v", json)
	}

	fresh := envConfig("environment:\n  examples:\n    - source: .env.example\n      target: .env.fixture\n")
	fresh.files["saved/.env.fixture"] = "SYNTHETIC=restored\n"
	freshStore := &memoryEnvStore{tree: fresh}
	if outcome := runEnv(fresh, freshStore, "env", "restore", "--dir", "saved", "--force"); outcome.ExitCode != 0 {
		t.Fatalf("absent target with --force refused: %+v", outcome)
	}

	if outcome := runEnv(tree, store, "env", "restore"); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "requires --dir") {
		t.Fatalf("missing --dir not refused: %+v", outcome)
	}
	vacant := envConfig("environment:\n  examples:\n    - source: .env.example\n      target: .env.fixture\n")
	if outcome := runEnv(vacant, &memoryEnvStore{tree: vacant}, "env", "restore", "--dir", "saved", "--force"); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "backup `saved/.env.fixture` is missing") {
		t.Fatalf("missing backup not refused: %+v", outcome)
	}
}

func TestEnvironmentValidateReadsOnlyDeclaredSources(t *testing.T) {
	config := "environment:\n" +
		"  contracts:\n    - path: .env.example\n      keys: [RUST_KEY, GO_KEY]\n" +
		"  detectors:\n    - language: rust\n      paths: [src/main.rs]\n" +
		"    - language: go\n      paths: [cmd/service/main.go]\n"
	tree := envConfig(config)
	tree.files["src/main.rs"] = "std::env::var(\"RUST_KEY\");\n// SYNTHETIC=value\n"
	tree.files["cmd/service/main.go"] = "os.LookupEnv(\"GO_KEY\")\n"
	tree.files[".env.example"] = "RUST_KEY=\nGO_KEY=\n"

	clean := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "validate")
	if clean.ExitCode != 0 || clean.Stdout != "[environment-validate] checked 2 declared files, no findings\n" {
		t.Fatalf("clean validate differs: %+v", clean)
	}
	if strings.Contains(clean.Stdout+clean.Stderr, "SYNTHETIC") {
		t.Fatalf("validate reported a value: %+v", clean)
	}

	tree.files["cmd/service/main.go"] = "os.Getenv(\"OUTSIDE\")\n// SYNTHETIC=value\n"
	found := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "validate")
	if found.ExitCode != 1 {
		t.Fatalf("undeclared read not reported: %+v", found)
	}
	if !strings.Contains(found.Stderr, "[environment-validate] .env.example: declared-but-unread (GO_KEY)") ||
		!strings.Contains(found.Stderr, "[environment-validate] cmd/service/main.go: read-but-undeclared (OUTSIDE)") {
		t.Fatalf("findings differ: %+v", found)
	}
	if strings.Contains(found.Stdout+found.Stderr, "SYNTHETIC") {
		t.Fatalf("findings reported a value: %+v", found)
	}

	json := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "validate", "--output", "json")
	if json.ExitCode != 1 || json.Stdout != "{\"command\":\"environment-validate\",\"exitCode\":1,\"findings\":[{\"key\":\"GO_KEY\",\"path\":\".env.example\",\"rule\":\"declared-but-unread\"},{\"key\":\"OUTSIDE\",\"path\":\"cmd/service/main.go\",\"rule\":\"read-but-undeclared\"}],\"schemaVersion\":1}\n" {
		t.Fatalf("findings json differs: %+v", json)
	}

	dynamic := envConfig("environment:\n  detectors:\n    - language: rust\n      paths: [src/main.rs]\n")
	dynamic.files["src/main.rs"] = "std::env::var(name);\n// SYNTHETIC=value\n"
	redacted := runEnv(dynamic, &memoryEnvStore{tree: dynamic}, "env", "validate")
	if redacted.ExitCode != 1 || !strings.Contains(redacted.Stderr, "unsupported-dynamic-access (<dynamic>)") {
		t.Fatalf("dynamic access not redacted: %+v", redacted)
	}
	if strings.Contains(redacted.Stdout+redacted.Stderr, "SYNTHETIC") {
		t.Fatalf("dynamic finding reported a value: %+v", redacted)
	}
}

func TestEnvironmentStagingGuardReadsOnlyTheIndex(t *testing.T) {
	tree := envConfig("environment:\n  staged:\n    forbidden: [\".env*\"]\n    allowed: [\".env.example\"]\n")
	tree.indexed = []string{".env.example", ".env.fixture", "notes/ordinary.md"}
	tree.hasIndex = true

	outcome := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "validate")
	if outcome.ExitCode != 1 || !strings.Contains(outcome.Stderr, "[environment-validate] .env.fixture: staged-environment-file") {
		t.Fatalf("staged finding differs: %+v", outcome)
	}
	if strings.Contains(outcome.Stdout+outcome.Stderr, "ordinary.md") {
		t.Fatalf("guard reported an allowed path: %+v", outcome)
	}

	broad := envConfig("environment:\n  staged:\n    forbidden: [\".env*\"]\n    allowed: [\".env*\"]\n")
	if outcome := runEnv(broad, &memoryEnvStore{tree: broad}, "env", "validate"); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "allows only exact repository-relative paths") {
		t.Fatalf("broad allowlist not refused: %+v", outcome)
	}

	patterned := envConfig("environment:\n  staged:\n    forbidden: [\"[\"]\n")
	if outcome := runEnv(patterned, &memoryEnvStore{tree: patterned}, "env", "validate"); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "invalid path pattern") {
		t.Fatalf("invalid pattern not refused: %+v", outcome)
	}

	unindexed := envConfig("environment:\n  staged:\n    forbidden: [\".env*\"]\n")
	if outcome := runEnv(unindexed, &memoryEnvStore{tree: unindexed}, "env", "validate"); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "staging guard refused") {
		t.Fatalf("missing index not refused: %+v", outcome)
	}
}

func TestEnvironmentRefusesWithoutAWriteBoundary(t *testing.T) {
	tree := envConfig("environment:\n  examples:\n    - source: .env.example\n      target: .env.generated\n")
	tree.files[".env.example"] = "EXAMPLE_KEY=placeholder\n"

	planned := application.Execute(tree, args("env", "init"), nil)
	if planned.ExitCode != 0 {
		t.Fatalf("plan needs no boundary: %+v", planned)
	}
	if outcome := application.Execute(tree, args("env", "init", "--apply"), nil); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "no environment write boundary") {
		t.Fatalf("unwired init did not refuse: %+v", outcome)
	}

	backed := envConfig("environment:\n  examples:\n    - source: .env.example\n      target: .env.saved\n")
	backed.files[".env.saved"] = "SAVED_KEY=placeholder\n"
	if outcome := application.Execute(backed, args("env", "backup", "--dir", "saved"), nil); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "no environment write boundary") {
		t.Fatalf("unwired backup did not refuse: %+v", outcome)
	}

	empty := envConfig("environment: {}\n")
	if outcome := application.Execute(empty, args("env", "backup", "--dir", "saved"), nil); outcome.ExitCode != 0 {
		t.Fatalf("empty backup needs no boundary: %+v", outcome)
	}

	if outcome := application.Execute(tree, args("env", "validate"), nil); outcome.ExitCode != 0 {
		t.Fatalf("validate needs no boundary: %+v", outcome)
	}
}

func TestEnvironmentPolicyValidationCoversDeclaredShapes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config string
		want   string
	}{
		{"contract path escapes", "environment:\n  contracts:\n    - path: ../outside\n      keys: [KEY]\n", "contract paths must be exact repository-relative files"},
		{"contract key invalid", "environment:\n  contracts:\n    - path: .env.example\n      keys: [invalid-key]\n", "contract keys must be unique valid environment names"},
		{"detector without paths", "environment:\n  detectors:\n    - language: rust\n      paths: []\n", "each detector requires one or more exact source paths"},
		{"detector path escapes", "environment:\n  detectors:\n    - language: rust\n      paths: [../outside]\n", "detector source paths must be exact and unique per language"},
		{"allowlist without expiry", "environment:\n  allowlists:\n    - rule: read-but-undeclared\n      path: src/app.rs\n      key: KEY\n      reason: reason\n      owner: owner\n      expiry: not-a-date\n", "unique exact rule, path, key, reason, owner, and ISO expiry"},
		{"unknown language", "environment:\n  detectors:\n    - language: cobol\n      paths: [src/app.cbl]\n", "unknown language"},
	} {
		tree := envConfig(tc.config)
		outcome := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "validate")
		if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, tc.want) {
			t.Fatalf("%s: %+v", tc.name, outcome)
		}
	}

	allowlisted := envConfig("environment:\n" +
		"  contracts:\n    - path: .env.example\n      keys: [ALLOWED]\n" +
		"  detectors:\n    - language: rust\n      paths: [src/app.rs]\n" +
		"  allowlists:\n    - rule: read-but-undeclared\n      path: src/app.rs\n      key: OUTSIDE\n      reason: third-party type definition\n      owner: tooling\n      expiry: 2099-01-01\n")
	allowlisted.files["src/app.rs"] = "std::env::var(\"ALLOWED\"); std::env::var(\"OUTSIDE\");\n"
	if outcome := runEnv(allowlisted, &memoryEnvStore{tree: allowlisted}, "env", "validate"); outcome.ExitCode != 0 {
		t.Fatalf("exact allowlist did not suppress: %+v", outcome)
	}

	absent := envConfig("")
	for _, cmd := range [][]string{{"env", "backup", "--dir", "saved"}, {"env", "validate"}, {"env", "init", "--apply"}, {"env", "restore", "--dir", "saved", "--force"}} {
		if outcome := runEnv(absent, &memoryEnvStore{tree: absent}, cmd...); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "environment section is not declared") {
			t.Fatalf("%v without section: %+v", cmd, outcome)
		}
	}
}

func TestEnvironmentDetectorLanguagesReportOnlyNames(t *testing.T) {
	for _, tc := range []struct {
		name     string
		language string
		source   string
		keys     string
		want     int
		stderr   string
	}{
		{"rust var_os", "rust", "std::env::var_os(\"R2\")\n// SYNTHETIC=value\n", "[R2]", 0, ""},
		{"rust dynamic", "rust", "std::env::var(name);\n// SYNTHETIC=value\n", "[]", 1, "unsupported-dynamic-access (<dynamic>)"},
		{"rust unterminated", "rust", "std::env::var(\"ABC\n", "[]", 1, "unsupported-dynamic-access (<dynamic>)"},
		{"type-script brackets", "type-script", "process.env[\"TS\"]\n", "[TS]", 0, ""},
		{"type-script dotted", "type-script", "process.env.TS2\n", "[TS2]", 0, ""},
		{"type-script object key", "type-script", "export const environment = {\n  SERVICE_TOKEN: text(),\n};\n", "[SERVICE_TOKEN]", 0, ""},
		{"type-script dynamic", "type-script", "process.env[value]; process.env.\n", "[]", 1, "unsupported-dynamic-access (<dynamic>)"},
		{"f-sharp", "f-sharp", "Environment.GetEnvironmentVariable(\"FS\")\n", "[FS]", 0, ""},
		{"f-sharp dynamic", "f-sharp", "Environment.GetEnvironmentVariable(name)\n", "[]", 1, "unsupported-dynamic-access (<dynamic>)"},
		{"go injected lookup", "go", "resolve(os.LookupEnv, \"G2\")\n", "[G2]", 0, ""},
		{"go dynamic", "go", "os.Getenv(name)\n", "[]", 1, "unsupported-dynamic-access (<dynamic>)"},
		{"terraform env", "terraform", "value = env(\"TF2\")\n", "[TF2]", 0, ""},
		{"terraform dynamic", "terraform", "value = get_env(name)\n", "[]", 1, "unsupported-dynamic-access (<dynamic>)"},
		{"ansible env", "ansible", "{{ lookup('env', 'A2') }}\n", "[A2]", 0, ""},
		{"ansible other lookup ignored", "ansible", "{{ lookup('file', 'ignored') }}\n", "[]", 0, ""},
		{"ansible missing comma", "ansible", "{{ lookup('env') }}\n", "[]", 1, "unsupported-dynamic-access (<dynamic>)"},
		{"ansible unquoted key", "ansible", "{{ lookup('env', name) }}\n", "[]", 1, "unsupported-dynamic-access (<dynamic>)"},
		{"ansible unquoted lookup", "ansible", "{{ lookup(name, 'A3') }}\n", "[]", 1, "unsupported-dynamic-access (<dynamic>)"},
	} {
		tree := envConfig("environment:\n" +
			"  contracts:\n    - path: cfg\n      keys: " + tc.keys + "\n" +
			"  detectors:\n    - language: " + tc.language + "\n      paths: [src/code]\n")
		tree.files["src/code"] = tc.source
		tree.files["cfg"] = "declared\n"
		outcome := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "validate")
		if outcome.ExitCode != tc.want {
			t.Fatalf("%s: exit %d, want %d: %+v", tc.name, outcome.ExitCode, tc.want, outcome)
		}
		if tc.stderr != "" && !strings.Contains(outcome.Stderr, tc.stderr) {
			t.Fatalf("%s stderr missing %q: %+v", tc.name, tc.stderr, outcome)
		}
		if strings.Contains(outcome.Stdout+outcome.Stderr, "SYNTHETIC") {
			t.Fatalf("%s reported a value: %+v", tc.name, outcome)
		}
	}
}

func TestEnvironmentFindingsOrderBreaksKeyTies(t *testing.T) {
	tree := envConfig("environment:\n  detectors:\n    - language: rust\n      paths: [src/app.rs]\n")
	tree.files["src/app.rs"] = "std::env::var(\"BETA\"); std::env::var(\"ALPHA\");\n"
	outcome := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "validate")
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	alpha := strings.Index(outcome.Stderr, "(ALPHA)")
	beta := strings.Index(outcome.Stderr, "(BETA)")
	if alpha < 0 || beta < 0 || alpha > beta {
		t.Fatalf("key order wrong: %q", outcome.Stderr)
	}
	json := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "validate", "--output", "json")
	alphaJSON := strings.Index(json.Stdout, "\"ALPHA\"")
	betaJSON := strings.Index(json.Stdout, "\"BETA\"")
	if alphaJSON < 0 || betaJSON < 0 || alphaJSON > betaJSON {
		t.Fatalf("json key order wrong: %q", json.Stdout)
	}
}

func TestEnvironmentDecodeRefusesEveryMalformedShape(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config string
		want   string
	}{
		{"unknown top key", "environment:\n  bogus: {}\n", "unknown key `bogus`"},
		{"examples non-list", "environment:\n  examples: {}\n", "names no examples"},
		{"example non-map", "environment:\n  examples: [x]\n", "names no source and target"},
		{"example unknown key", "environment:\n  examples:\n    - source: a\n      target: b\n      extra: c\n", "unknown key `extra`"},
		{"example missing target", "environment:\n  examples:\n    - source: a\n", "names no source and target"},
		{"example numeric source", "environment:\n  examples:\n    - source: 7\n      target: b\n", "names no source and target"},
		{"staged non-map", "environment:\n  staged: []\n", "names no staged policy"},
		{"staged unknown key", "environment:\n  staged:\n    unknown: []\n", "unknown key `unknown`"},
		{"staged non-list", "environment:\n  staged:\n    forbidden: {}\n", "names no paths"},
		{"staged numeric pattern", "environment:\n  staged:\n    forbidden: [7]\n", "names no paths"},
		{"contracts non-list", "environment:\n  contracts: {}\n", "names no contracts"},
		{"contract non-map", "environment:\n  contracts: [x]\n", "names no contracts"},
		{"contract unknown key", "environment:\n  contracts:\n    - path: a\n      keys: []\n      extra: 1\n", "unknown key `extra`"},
		{"contract missing path", "environment:\n  contracts:\n    - keys: [K]\n", "names no path"},
		{"contract numeric key", "environment:\n  contracts:\n    - path: a\n      keys: [7]\n", "names no paths"},
		{"detectors non-list", "environment:\n  detectors: {}\n", "names no detectors"},
		{"detector non-map", "environment:\n  detectors: [x]\n", "names no detectors"},
		{"detector unknown key", "environment:\n  detectors:\n    - language: rust\n      paths: []\n      extra: 1\n", "unknown key `extra`"},
		{"detector missing language", "environment:\n  detectors:\n    - paths: []\n", "names no language"},
		{"allowlists non-list", "environment:\n  allowlists: {}\n", "names no allowlists"},
		{"allowlist non-map", "environment:\n  allowlists: [x]\n", "names no allowlists"},
		{"allowlist unknown key", "environment:\n  allowlists:\n    - rule: r\n      path: p\n      key: K\n      reason: why\n      owner: who\n      expiry: 2099-01-01\n      extra: 1\n", "unknown key `extra`"},
		{"allowlist missing owner", "environment:\n  allowlists:\n    - rule: r\n      path: p\n      key: K\n      reason: why\n      expiry: 2099-01-01\n", "names no rule, path, key, reason, owner, and expiry"},
	} {
		tree := envConfig(tc.config)
		outcome := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "validate", "--output", "json")
		if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, tc.want) || !strings.Contains(outcome.Stderr, "rhino.config.unusable") {
			t.Fatalf("%s: %+v", tc.name, outcome)
		}
	}
}

func TestEnvironmentValidateRefusesUnreadableDetectorSources(t *testing.T) {
	config := "environment:\n  detectors:\n    - language: rust\n      paths: [src/app.rs]\n"
	unreadable := envConfig(config)
	unreadable.files["src/app.rs"] = "std::env::var(\"K\")\n"
	unreadable.unread["src/app.rs"] = true
	if outcome := runEnv(unreadable, &memoryEnvStore{tree: unreadable}, "env", "validate"); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "is unreadable") {
		t.Fatalf("unreadable source: %+v", outcome)
	}
	binary := envConfig(config)
	binary.files["src/app.rs"] = "bytes"
	binary.binary["src/app.rs"] = true
	if outcome := runEnv(binary, &memoryEnvStore{tree: binary}, "env", "validate"); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "holds no text") {
		t.Fatalf("binary source: %+v", outcome)
	}
	missing := envConfig(config)
	if outcome := runEnv(missing, &memoryEnvStore{tree: missing}, "env", "validate"); outcome.ExitCode != 1 || !strings.Contains(outcome.Stderr, "declared-source-unread") {
		t.Fatalf("missing source: %+v", outcome)
	}
	absolute := envConfig("environment:\n  staged:\n    forbidden: [/abs]\n")
	if outcome := runEnv(absolute, &memoryEnvStore{tree: absolute}, "env", "validate"); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "must be relative to the repository root") {
		t.Fatalf("absolute pattern: %+v", outcome)
	}
	plain := newMemoryTree("/root")
	plain.files["repo-config.yml"] = "schema: rhino/repo-config/v2\nenvironment:\n  staged:\n    forbidden: [sec*]\n"
	if outcome := application.Execute(plain, args("env", "validate"), nil); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "no Git index snapshot boundary") {
		t.Fatalf("index-less tree: %+v", outcome)
	}
}

func TestEnvironmentWriteBoundariesRefuseHonestly(t *testing.T) {
	tree := envConfig("environment:\n  examples:\n    - source: ex\n      target: gen\n")
	tree.files["ex"] = "x\n"
	if outcome := runEnv(tree, &memoryEnvStore{tree: tree, refuse: "boom"}, "env", "init", "--apply"); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "environment initialization refused: boom") {
		t.Fatalf("init store: %+v", outcome)
	}
	backed := envConfig("environment:\n  examples:\n    - source: ex\n      target: saved-ex\n")
	backed.files["saved-ex"] = "x\n"
	if outcome := runEnv(backed, &memoryEnvStore{tree: backed, refuse: "boom"}, "env", "backup", "--dir", "saved", "--output", "json"); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "rhino.file.unwritable") {
		t.Fatalf("backup store: %+v", outcome)
	}
	restored := envConfig("environment:\n  examples:\n    - source: ex\n      target: gen\n")
	restored.files["saved/gen"] = "x\n"
	if outcome := runEnv(restored, &memoryEnvStore{tree: restored, refuse: "boom"}, "env", "restore", "--dir", "saved", "--force"); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "environment restore refused: boom") {
		t.Fatalf("restore store: %+v", outcome)
	}
	if _, ok := tree.files["gen"]; ok {
		t.Fatal("refused init wrote its target")
	}
	empty := envConfig("environment: {}\n")
	zero := runEnv(empty, &memoryEnvStore{tree: empty}, "env", "restore", "--dir", "saved", "--force", "--output", "json")
	if zero.ExitCode != 0 || zero.Stdout != "{\"command\":\"environment-restore\",\"count\":0,\"schemaVersion\":1,\"status\":\"completed\",\"targets\":[]}\n" {
		t.Fatalf("empty restore json differs: %+v", zero)
	}
	escaping := envConfig("environment:\n  examples:\n    - source: ex\n      target: ../outside\n")
	escaping.files["ex"] = "x\n"
	if outcome := runEnv(escaping, &memoryEnvStore{tree: escaping}, "env", "backup", "--dir", "saved"); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "environment example leaves the repository root") {
		t.Fatalf("escaping target: %+v", outcome)
	}
	unreadable := envConfig("environment:\n  examples:\n    - source: ex\n      target: saved-ex\n")
	unreadable.files["saved-ex"] = "x\n"
	unreadable.unread["saved-ex"] = true
	if outcome := runEnv(unreadable, &memoryEnvStore{tree: unreadable}, "env", "backup", "--dir", "saved"); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "is unreadable") {
		t.Fatalf("unreadable target: %+v", outcome)
	}
	blocked := envConfig("environment:\n  examples:\n    - source: ex\n      target: gen\n")
	blocked.files["saved/gen"] = "x\n"
	blocked.files["gen"] = "current\n"
	blocked.binary["saved/gen"] = true
	if outcome := runEnv(blocked, &memoryEnvStore{tree: blocked}, "env", "restore", "--dir", "saved", "--force"); outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "is unreadable") {
		t.Fatalf("unreadable backup: %+v", outcome)
	}
}
