package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/wahidyankf/rhino/internal/application"
	"gopkg.in/yaml.v3"
)

// The unit world runs one scenario in process. It builds the repository in the
// in-memory file system, declares the Git state, the gate children, the model
// commands, and the mutation fixtures the scenario names, and runs the command
// through Application.RunArgs with every port replaced by a double. No child
// process starts, no file is read or written on disk, and no signal reaches the
// process: a child's observable behaviour (exit status, argument vector,
// environment, the files it rewrites, and a signal it sends) is what the double
// returns.

const (
	unitRepo    = "/scenario/repo"
	unitOutside = "/scenario/outside"
)

// unitChild is one declared executable: a gate child, a mutation child, or a
// model command. Its behaviour is what the scenario states.
type unitChild struct {
	id        string
	exit      int
	mutation  bool
	signal    syscall.Signal
	hasSignal bool
	rewrite   *unitRewrite
}

// unitRewrite is the change a declared mutation child makes to one path in its
// working directory.
type unitRewrite struct {
	path    string
	content string
}

// unitModel is one declared model-listing command: what it prints, and whether
// it can be started.
type unitModel struct {
	output   string
	canStart bool
}

// unitInvocation records one CLI run and the repository state either side.
type unitInvocation struct {
	stdout   string
	stderr   string
	exitCode int
	before   map[string]string
	after    map[string]string
}

// unitWorld is the state of one scenario.
type unitWorld struct {
	files        *memFS
	git          *memGit
	config       string
	stdin        *string
	stdoutClosed bool
	runs         []unitInvocation
	snapshotOn   bool
	children     map[string]*unitChild
	logs         map[string]unitRecord
	models       map[string]*unitModel
	modelStarted map[string]bool
	signals      *unitSignals
	ctx          context.Context
}

// unitRecord is what a child run recorded: its argument vector and the projected
// environment it received.
type unitRecord struct {
	argv []string
	env  []string
}

// newUnitWorld creates an empty repository, outside directory, and bin directory.
func newUnitWorld() *unitWorld {
	files := newMemFS()
	files.mkdirAll(unitRepo)
	files.mkdirAll(unitOutside)
	files.mkdirAll("/scenario/bin")
	return &unitWorld{
		files:        files,
		children:     map[string]*unitChild{},
		logs:         map[string]unitRecord{},
		models:       map[string]*unitModel{},
		modelStarted: map[string]bool{},
		signals:      &unitSignals{},
	}
}

// repoPath resolves a repository-relative path and refuses one that leaves the
// repository.
func (w *unitWorld) repoPath(rel string) (string, error) {
	clean := path.Clean(rel)
	if memLeavesRoot(rel) {
		return "", fmt.Errorf("scenario path %q leaves the repository", rel)
	}
	return path.Join(unitRepo, clean), nil
}

// writeRepoFile writes content at a repository-relative path. A file with a
// shebang is executable, because the scenarios use such files as children.
func (w *unitWorld) writeRepoFile(rel, content string) error {
	location, err := w.repoPath(rel)
	if err != nil {
		return err
	}
	perm := fs.FileMode(0o644)
	if strings.HasPrefix(content, "#!") {
		perm = 0o755
	}
	w.files.writeFile(location, content, perm)
	return nil
}

// readRepoFile reads a repository-relative file for an assertion from the in-memory
// tree, following links as a file read does.
func (w *unitWorld) readRepoFile(rel string) (string, error) {
	location, err := w.repoPath(rel)
	if err != nil {
		return "", err
	}
	resolved, ok := w.files.resolve(location)
	if !ok {
		return "", fmt.Errorf("read %s: no such file", rel)
	}
	entry, ok := w.files.lstat(resolved)
	if !ok || entry.kind != memFile {
		return "", fmt.Errorf("read %s: no such file", rel)
	}
	return entry.content, nil
}

// workingContent returns the working-tree bytes of a file, or nothing when it does
// not exist, so an index entry for a new path starts empty.
func (w *unitWorld) workingContent(rel string) string {
	content, err := w.readRepoFile(rel)
	if err != nil {
		return ""
	}
	return content
}

// createLink replaces whatever is at a repository path with a symbolic link.
func (w *unitWorld) createLink(rel, target string) error {
	location, err := w.repoPath(rel)
	if err != nil {
		return err
	}
	w.files.removeAll(location)
	w.files.symlink(location, target)
	return nil
}

// ensureGit starts the repository's Git state on first use.
func (w *unitWorld) ensureGit() {
	if w.git == nil {
		w.git = newMemGit()
	}
}

// placeExecutable puts a fixture where the configured command finds it. A command
// with a slash is a repository-relative path; a bare name is found on PATH, so
// its fixture lives in the scenario's bin directory.
func (w *unitWorld) placeExecutable(command, body string) error {
	if strings.Contains(command, "/") {
		return w.writeRepoFile(command, body)
	}
	w.files.writeFile(path.Join("/scenario/bin", command), body, 0o755)
	return nil
}

// trackDeclaredChild commits a repository-relative child, so the index snapshot
// and the replay checkouts, which hold only committed content, can start it.
func (w *unitWorld) trackDeclaredChild(command string, staged bool) error {
	if !strings.Contains(command, "/") {
		return nil
	}
	w.ensureGit()
	content := w.workingContent(command)
	w.git.trackExecutable(path.Clean(command), content, staged)
	return nil
}

// childPrologue makes a fixture child refuse to run in the live repository. Only a
// declared mutation child uses it, because a mutation must never run against the
// working tree it was asked to check.
func (w *unitWorld) childPrologue() string {
	if !w.snapshotOn {
		return ""
	}
	return "if [ \"$(pwd -P)\" = " + shellQuote(unitRepo) + " ]; then\n" +
		"  echo 'declared mutation ran in the live repository' >&2\n  exit 1\nfi\n"
}

// shellQuote quotes one word for a POSIX shell fixture.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// declaredGate is the subset of a gate entry the fixtures need.
type declaredGate struct {
	ID      string `yaml:"id"`
	Type    string `yaml:"type"`
	Command struct {
		Executable string `yaml:"executable"`
	} `yaml:"command"`
}

// gateByID returns the declared gate with the given identifier.
func (w *unitWorld) gateByID(id string) (declaredGate, error) {
	var document struct {
		Gates struct {
			Entries []declaredGate `yaml:"entries"`
		} `yaml:"gates"`
	}
	if err := yaml.Unmarshal([]byte(w.config), &document); err != nil {
		return declaredGate{}, fmt.Errorf("read scenario configuration: %w", err)
	}
	for _, gate := range document.Gates.Entries {
		if gate.ID == id {
			return gate, nil
		}
	}
	return declaredGate{}, fmt.Errorf("the configuration declares no gate %q", id)
}

// mutationGate returns the first declared mutation gate.
func (w *unitWorld) mutationGate() (declaredGate, error) {
	var document struct {
		Gates struct {
			Entries []declaredGate `yaml:"entries"`
		} `yaml:"gates"`
	}
	if err := yaml.Unmarshal([]byte(w.config), &document); err != nil {
		return declaredGate{}, fmt.Errorf("read scenario configuration: %w", err)
	}
	for _, gate := range document.Gates.Entries {
		if gate.Type == "mutation" {
			return gate, nil
		}
	}
	return declaredGate{}, errors.New("the configuration declares no mutation gate")
}

// childFor returns the declared child an invocation's executable names.
func (w *unitWorld) childFor(argv0 string) (*unitChild, bool) {
	child, ok := w.children[argv0]
	return child, ok
}

// modelCommandPath finds the path the configuration names for a model command, so
// the fixture is keyed by the path the declared command will use.
func (w *unitWorld) modelCommandPath(name string) string {
	pattern := regexp.MustCompile(`[A-Za-z0-9_./-]*` + regexp.QuoteMeta(name) + `[A-Za-z0-9_./-]*`)
	if match := pattern.FindString(w.config); match != "" {
		return match
	}
	return name
}

// snapshot describes every repository entry and the Git index, so two equal
// snapshots mean no byte of the repository changed.
func (w *unitWorld) snapshot() map[string]string {
	result := map[string]string{}
	prefix := unitRepo + "/"
	for p, entry := range w.files.entries {
		if !strings.HasPrefix(p, prefix) {
			continue
		}
		rel := strings.TrimPrefix(p, prefix)
		switch entry.kind {
		case memLink:
			result[rel] = "link " + entry.target
		case memDir:
			result[rel] = fmt.Sprintf("dir %o", entry.perm.Perm())
		default:
			result[rel] = fmt.Sprintf("file %o %q", entry.perm.Perm(), entry.content)
		}
	}
	if w.git != nil {
		result[".git/index"] = fmt.Sprintf("%v", w.git.index)
	}
	return result
}

// sameSnapshot reports whether two snapshots describe identical trees.
func sameSnapshot(before, after map[string]string) bool {
	if len(before) != len(after) {
		return false
	}
	for key, value := range before {
		if other, ok := after[key]; !ok || other != value {
			return false
		}
	}
	return true
}

// invoke runs the CLI with the pipe-separated arguments in the repository. The
// placeholders {base} and {head} resolve to the scenario's commit names.
func (w *unitWorld) invoke(joined string) error {
	var args []string
	if joined != "" {
		args = strings.Split(joined, "|")
	}
	for i, arg := range args {
		if strings.Contains(arg, "{base}") || strings.Contains(arg, "{head}") {
			w.ensureGit()
			arg = strings.ReplaceAll(arg, "{base}", w.git.base)
			args[i] = strings.ReplaceAll(arg, "{head}", w.git.head)
		}
	}
	before := w.snapshot()
	var stdout, stderr bytes.Buffer
	var stdin io.Reader = strings.NewReader("")
	if w.stdin != nil {
		stdin = strings.NewReader(*w.stdin)
	}
	var out io.Writer = &stdout
	if w.stdoutClosed {
		out = unitClosedWriter{}
	}
	w.stdin = nil
	w.stdoutClosed = false

	repository := newMemTree(w.files, unitRepo, w.git)
	app := Application{
		Stdin:       stdin,
		Stdout:      out,
		Stderr:      &stderr,
		Launcher:    w,
		Environment: w,
		Toolchains:  w,
		Adapters:    w,
		Models:      w,
		Mutations:   w,
	}
	app.currentDirectory = func() (application.Tree, error) { return repository, nil }
	ctx, stop := signalContext(context.Background(), w.signals)
	w.ctx = ctx
	code := app.RunArgs(ctx, args)
	stop()
	w.runs = append(w.runs, unitInvocation{
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		exitCode: code,
		before:   before,
		after:    w.snapshot(),
	})
	return nil
}

// lastRun returns the most recent invocation, or an error when none ran.
func (w *unitWorld) lastRun() (unitInvocation, error) {
	if len(w.runs) == 0 {
		return unitInvocation{}, errors.New("no CLI invocation has run in this scenario")
	}
	return w.runs[len(w.runs)-1], nil
}

// unitClosedWriter is a standard output whose reader has closed. Writing to it
// fails with the broken-pipe error, as a write to a closed pipe does.
type unitClosedWriter struct{}

func (unitClosedWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return 0, syscall.EPIPE
}

// unitSignals is the operating-system signal source for one run. The runner
// stores the channel the run registers, and a child delivers a signal into it.
type unitSignals struct {
	channel chan<- os.Signal
}

func (s *unitSignals) Notify(channel chan<- os.Signal, _ ...os.Signal) { s.channel = channel }

func (s *unitSignals) Stop(chan<- os.Signal) {}

// deliver sends a signal to the run that registered for it.
func (s *unitSignals) deliver(sig syscall.Signal) {
	s.channel <- sig
}

// Launch starts one declared gate child in process. A child that is not declared
// is not found; one declared as not executable is not executable. A child that
// sends a signal delivers it to the run and waits until the run is cancelled.
func (w *unitWorld) Launch(launch application.GateLaunch) application.Launched {
	if len(launch.Argv) == 0 {
		return application.Launched{Refused: "declares no executable command"}
	}
	child, ok := w.childFor(launch.Argv[0])
	if !ok {
		return w.undeclaredLaunch(launch.Argv, launch.Directory)
	}
	if child.mutation && w.snapshotOn && launch.Directory == unitRepo {
		return application.Launched{Code: 1}
	}
	w.logs[child.id] = unitRecord{argv: launch.Argv[1:], env: unitEnvironment(launch.Environment, launch.Surface)}
	if child.hasSignal {
		w.signals.deliver(child.signal)
		<-w.ctx.Done()
		return application.Launched{Refused: "gate child interrupted"}
	}
	return application.Launched{Code: child.exit}
}

// undeclaredLaunch answers for an executable no step declared, as the operating
// system would: a command with a slash is a file in dir, and a bare name is a file
// on PATH. A missing file is not found, and a file without the execute bit is not
// executable. An executable file runs its body, which the double models for the
// declared forms it can contain (see unitBody).
func (w *unitWorld) undeclaredLaunch(argv []string, dir string) application.Launched {
	run, status := w.undeclaredRun(argv, dir)
	switch status {
	case 127:
		return application.Launched{NotFound: true}
	case 126:
		return application.Launched{NotExecutable: true}
	}
	return application.Launched{Code: run.code}
}

// undeclaredRun runs an undeclared executable. It returns what the body does, or
// the start status (127 or 126) when the executable cannot start.
func (w *unitWorld) undeclaredRun(argv []string, dir string) (unitRun, int) {
	if code, ok := w.shellCommandV(argv); ok {
		return unitRun{code: code}, 0
	}
	location := path.Join("/scenario/bin", argv[0])
	if strings.Contains(argv[0], "/") {
		location = path.Join(dir, argv[0])
	}
	resolved, ok := w.files.resolve(location)
	if !ok {
		return unitRun{}, 127
	}
	entry, ok := w.files.lstat(resolved)
	if !ok || entry.kind != memFile {
		return unitRun{}, 127
	}
	if entry.perm&0o111 == 0 {
		return unitRun{}, 126
	}
	return unitBody(entry.content), 0
}

// shellCommandV models the one shell form a scenario declares, `sh -c "command -v
// NAME >/dev/null"`: the command succeeds when NAME is an executable in the
// repository's node_modules/.bin, the directory the child's PATH names. Any other
// shell form is not modelled and does not match.
func (w *unitWorld) shellCommandV(argv []string) (int, bool) {
	if len(argv) < 3 || argv[0] != "sh" || argv[1] != "-c" {
		return 0, false
	}
	const prefix, suffix = "command -v ", " >/dev/null"
	script := argv[2]
	if !strings.HasPrefix(script, prefix) || !strings.HasSuffix(script, suffix) {
		return 0, false
	}
	name := strings.TrimSuffix(strings.TrimPrefix(script, prefix), suffix)
	entry, ok := w.files.lstat(path.Join(unitRepo, "node_modules/.bin", name))
	if ok && entry.kind == memFile && entry.perm&0o111 != 0 {
		return 0, true
	}
	return 1, true
}

// unitMaxOutput is the bound on what a toolchain probe may print, as the disk
// runner states it. A probe that prints more is refused.
const unitMaxOutput = 64 * 1024

// unitRun is what a repository executable's body does when it runs: its exit
// status, the text it prints, how many bytes its `yes | head` pipeline writes, and
// how long it sleeps. Sleeping is virtual: the double compares the duration with
// the declared timeout and starts no clock.
type unitRun struct {
	code    int
	output  string
	emitted int
	sleep   uint64
}

// unitBody interprets the lines of an executable's body that the scenarios use:
// `echo` prints its text, `exit` sets the status, `sleep` waits, and
// `yes x | head -c N` writes N bytes. Any other line does nothing, as a comment
// or a shebang does.
func unitBody(body string) unitRun {
	var run unitRun
	for _, line := range strings.Split(body, "\n") {
		fields := strings.Fields(line)
		switch {
		case strings.HasPrefix(line, "echo "):
			run.output += strings.TrimPrefix(line, "echo ") + "\n"
		case len(fields) == 2 && fields[0] == "exit":
			if status, err := parseCode(fields[1]); err == nil {
				run.code = status
			}
		case len(fields) == 2 && fields[0] == "sleep":
			if seconds, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
				run.sleep += seconds
			}
		case len(fields) == 6 && fields[0] == "yes" && fields[2] == "|" && fields[3] == "head" && fields[4] == "-c":
			if bytes, err := strconv.Atoi(fields[5]); err == nil {
				run.emitted += bytes
			}
		}
	}
	return run
}

// Run starts one declared toolchain probe or provision vector in process. The
// executable is looked up as a file in the repository, or on PATH for a bare name.
func (w *unitWorld) Run(launch application.ToolchainLaunch) (application.ToolchainResult, error) {
	if strings.TrimSpace(launch.Executable) == "" || strings.HasPrefix(launch.Executable, "-") {
		return application.ToolchainResult{}, fmt.Errorf("the declared executable is invalid")
	}
	argv := append([]string{launch.Executable}, launch.Arguments...)
	run, status := w.undeclaredRun(argv, unitRepo)
	if status != 0 {
		return application.ToolchainResult{}, &application.ChildStartError{Status: status,
			Err: fmt.Errorf("could not start declared toolchain: %s", launch.Executable)}
	}
	// A probe that sleeps past its declared timeout is cut short, and one that
	// prints past the bound is refused; neither reports output or a status.
	if launch.TimeoutSeconds != nil && run.sleep > *launch.TimeoutSeconds {
		return application.ToolchainResult{}, fmt.Errorf("the declared toolchain exceeded its timeout")
	}
	if run.emitted+len(run.output) > unitMaxOutput {
		return application.ToolchainResult{}, fmt.Errorf("the declared toolchain emitted too much output")
	}
	return application.ToolchainResult{Code: run.code, Stdout: run.output}, nil
}

// unitEnvironment is the projected environment a child receives: the projected
// entries and the selecting surface, sorted as the launcher sorts them.
func unitEnvironment(projected map[string]string, surface string) []string {
	merged := map[string]string{"RHINO_GATE_SURFACE": surface}
	for name, value := range projected {
		merged[name] = value
	}
	var out []string
	for name, value := range merged {
		out = append(out, name+"="+value)
	}
	sort.Strings(out)
	return out
}

// ApplyIndex runs a declared mutator against a snapshot of the index and writes
// back only the selected paths it changed, as the disk runner does.
func (w *unitWorld) ApplyIndex(launch application.MutationLaunch) (application.MutationOutcome, error) {
	if w.git == nil {
		return application.MutationOutcome{}, errors.New("this entry point has no index mutation boundary")
	}
	if len(launch.SelectedPaths) == 0 {
		return application.MutationOutcome{}, nil
	}
	for _, p := range launch.SelectedPaths {
		if memLeavesRoot(p) {
			return application.MutationOutcome{}, fmt.Errorf("selected index path escapes its root `%s`", p)
		}
	}
	before := map[string]string{}
	for p, blob := range w.git.index {
		before[p] = blob.content
	}
	snapshot := map[string]string{}
	for p, content := range before {
		snapshot[p] = content
	}
	code, err := w.runMutator(launch.Argv, launch.Environment, launch.Surface, snapshot)
	if err != nil {
		return application.MutationOutcome{}, err
	}
	if code != 0 {
		return application.MutationOutcome{Code: code}, nil
	}
	changed := diffContents(before, snapshot)
	for _, p := range changed {
		if !contains(launch.SelectedPaths, p) {
			return application.MutationOutcome{}, fmt.Errorf("mutator changed `%s` outside its selected index paths", p)
		}
	}
	var divergences []string
	for _, p := range changed {
		content, present := snapshot[p]
		if present {
			blob, indexed := w.git.index[p]
			if !indexed {
				blob = memBlob{mode: "100644"}
			}
			blob.content = content
			w.git.index[p] = blob
			if w.workingMatches(p, before[p], present) {
				w.setWorking(p, content)
			} else {
				divergences = append(divergences, p)
			}
			continue
		}
		delete(w.git.index, p)
		if w.workingMatches(p, before[p], present) {
			w.files.removeAll(path.Join(unitRepo, p))
		} else {
			divergences = append(divergences, p)
		}
	}
	return application.MutationOutcome{Code: 0, Changes: changed, Divergences: divergences}, nil
}

// VerifyClean replays the candidate revision in a disposable checkout, runs the
// mutator there, and reports every path the replay changed.
func (w *unitWorld) VerifyClean(launch application.MutationLaunch) (application.MutationOutcome, error) {
	if launch.Revision == "" {
		return application.MutationOutcome{}, errors.New("a disposable pull-request replay requires an explicit candidate revision")
	}
	w.ensureGit()
	tree, ok := w.git.commits[launch.Revision]
	if !ok {
		return application.MutationOutcome{}, errors.New("the candidate revision does not resolve")
	}
	before := map[string]string{}
	for p, blob := range tree {
		before[p] = blob.content
	}
	replay := map[string]string{}
	for p, content := range before {
		replay[p] = content
	}
	code, err := w.runMutator(launch.Argv, launch.Environment, launch.Surface, replay)
	if err != nil {
		return application.MutationOutcome{}, err
	}
	if code != 0 {
		return application.MutationOutcome{Code: code}, nil
	}
	// A clean replay exits 0; any change the replay made is a finding, so the
	// outcome carries status 1 with the changed paths, as the disk replay does.
	changes := diffContents(before, replay)
	status := 0
	if len(changes) > 0 {
		status = 1
	}
	return application.MutationOutcome{Code: status, Changes: changes}, nil
}

// runMutator runs the declared mutator over a snapshot. It records the argument
// vector and environment, and applies the child's rewrite to the snapshot.
func (w *unitWorld) runMutator(argv []string, projected map[string]string, surface string, snapshot map[string]string) (int, error) {
	if len(argv) == 0 {
		return 0, errors.New("the gate declares no command")
	}
	child, ok := w.childFor(argv[0])
	if !ok {
		run, status := w.undeclaredRun(argv, unitRepo)
		if status != 0 {
			return 0, &application.ChildStartError{Status: status, Err: fmt.Errorf("%s: cannot be started", argv[0])}
		}
		return run.code, nil
	}
	w.logs[child.id] = unitRecord{argv: argv[1:], env: unitEnvironment(projected, surface)}
	if child.rewrite != nil {
		snapshot[child.rewrite.path] = child.rewrite.content
	}
	return child.exit, nil
}

// workingMatches reports whether the working-tree copy of a path still holds the
// content the index held before the run, so a copy-back may replace it.
func (w *unitWorld) workingMatches(p, content string, present bool) bool {
	location := path.Join(unitRepo, p)
	entry, ok := w.files.lstat(location)
	if !ok {
		return !present && content == ""
	}
	return entry.kind == memFile && entry.content == content
}

// setWorking writes the content of a path into the working tree.
func (w *unitWorld) setWorking(p, content string) {
	location := path.Join(unitRepo, p)
	perm := fs.FileMode(0o644)
	if entry, ok := w.files.lstat(location); ok {
		perm = entry.perm
	}
	w.files.writeFile(location, content, perm)
}

// diffContents lists the paths whose content differs between two maps, added and
// removed paths included, sorted.
func diffContents(before, after map[string]string) []string {
	changed := map[string]bool{}
	for p, content := range after {
		if old, ok := before[p]; !ok || old != content {
			changed[p] = true
		}
	}
	for p := range before {
		if _, ok := after[p]; !ok {
			changed[p] = true
		}
	}
	return sortedKeys(changed)
}

// contains reports whether a list names a value.
func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// Resolve runs a declared model command in process. It starts only when the
// command can start, records that it started, and returns what it printed.
func (w *unitWorld) Resolve(launch application.ModelLaunch) (string, error) {
	if len(launch.Arguments) == 0 {
		return "", errors.New("the model command declares no executable")
	}
	for name, model := range w.models {
		if w.modelCommandPath(name) != launch.Arguments[0] {
			continue
		}
		if !model.canStart {
			return "", errors.New("the model command cannot be started")
		}
		w.modelStarted[name] = true
		return model.output, nil
	}
	return "", errors.New("the model command is not declared")
}

// Replace installs one adapter transaction: each managed root is replaced whole,
// each exact path is replaced alone, and every desired file is written.
func (w *unitWorld) Replace(root string, transaction application.AdapterTransaction) error {
	if _, ok := w.files.lstat(root); !ok {
		return fmt.Errorf("cannot inspect adapter root: %s", root)
	}
	for _, managed := range transaction.Roots {
		location := path.Join(root, managed)
		w.files.removeAll(location)
		w.files.mkdirAll(location)
	}
	for _, exact := range transaction.ExactPaths {
		w.files.removeAll(path.Join(root, exact))
	}
	for _, file := range transaction.Files {
		w.files.writeFile(path.Join(root, file.Path), file.Contents, 0o644)
	}
	return nil
}

// Create writes the declared environment files under the root. A target whose
// parent is an existing file cannot be written, as on disk.
func (w *unitWorld) Create(root string, files []application.EnvironmentFile) error {
	for _, file := range files {
		location := path.Join(root, file.Path)
		if err := w.writableTarget(location); err != nil {
			return err
		}
		if _, exists := w.files.lstat(location); exists {
			return fmt.Errorf("`%s` already exists", file.Path)
		}
		w.files.writeFile(location, file.Contents, 0o600)
	}
	return nil
}

// Restore rewrites the declared environment files from their backup bytes.
func (w *unitWorld) Restore(root string, files []application.EnvironmentRestoreFile) error {
	for _, file := range files {
		location := path.Join(root, file.Path)
		if err := w.writableTarget(location); err != nil {
			return err
		}
		w.files.writeFile(location, file.Contents, 0o600)
	}
	return nil
}

// writableTarget refuses a target one of whose parent directories is a file.
func (w *unitWorld) writableTarget(location string) error {
	for parent := path.Dir(location); parent != "/" && parent != "."; parent = path.Dir(parent) {
		entry, ok := w.files.lstat(parent)
		if ok && entry.kind != memDir {
			return fmt.Errorf("%s is not a directory", parent)
		}
	}
	return nil
}

// parseCode reads a declared exit status.
func parseCode(text string) (int, error) {
	return strconv.Atoi(text)
}
