package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"syscall"

	"github.com/cucumber/godog"
)

// The unit step registry binds every step text the behaviour corpus uses to the
// in-memory world. The texts and their patterns are those of tests/bdd/steps.go;
// each handler does the same thing against the in-memory state instead of the
// disk, so one scenario means the same at both levels.

// unitBinding pairs a step expression with its handler.
type unitBinding struct {
	pattern string
	handler any
}

// unitWorldKey carries the scenario world through the godog context.
type unitWorldKey struct{}

// unitWorldFrom returns the scenario world of a step's context.
func unitWorldFrom(ctx context.Context) (*unitWorld, error) {
	w, ok := ctx.Value(unitWorldKey{}).(*unitWorld)
	if !ok || w == nil {
		return nil, errors.New("no scenario world in context")
	}
	return w, nil
}

// unitTableRows returns the data rows of a two-column path/content table.
func unitTableRows(table *godog.Table) ([][2]string, error) {
	if table == nil || len(table.Rows) == 0 {
		return nil, errors.New("table has no rows")
	}
	header := table.Rows[0].Cells
	if len(header) != 2 || header[0].Value != "path" || header[1].Value != "content" {
		return nil, errors.New("table header must be | path | content |")
	}
	var rows [][2]string
	for _, row := range table.Rows[1:] {
		if len(row.Cells) != 2 {
			return nil, fmt.Errorf("table row has %d cells; want 2", len(row.Cells))
		}
		rows = append(rows, [2]string{row.Cells[0].Value, row.Cells[1].Value})
	}
	return rows, nil
}

// unitBindings returns every step definition of the corpus the unit level runs.
func unitBindings() []unitBinding {
	return []unitBinding{
		// Preparing the repository.
		{`^the configuration file is this text:$`, unitConfigurationText},
		{`^the repository contains:$`, unitRepositoryContains},
		{`^the directory outside the repository contains:$`, unitOutsideContains},
		{`^file "([^"]+)" contains this Markdown:$`, unitFileContains},
		{`^the repository contains an executable "([^"]+)"$`, unitRepositoryExecutable},
		{`^the repository contains an executable "([^"]+)" with the body:$`, unitRepositoryExecutableBody},
		{`^the repository contains a symbolic link at "([^"]+)"$`, unitDanglingLink},
		{`^the repository contains a symbolic link at "([^"]+)" to "([^"]+)" inside it$`, unitLinkInside},
		{`^the repository contains a symbolic link at "([^"]+)" to a directory outside it holding "([^"]+)"$`, unitLinkOutside},
		{`^file "([^"]+)" cannot be opened$`, unitFileCannotBeOpened},
		{`^the working tree path "([^"]+)" is a symbolic link to "([^"]+)"$`, unitWorkingTreeLink},
		{`^standard output is closed before the run$`, unitStandardOutputClosed},
		{`^the standard input is:$`, unitStandardInput},
		{`^the Git index selects the regular file "([^"]+)"$`, unitIndexSelectsFile},
		{`^the Git index selects no regular files$`, unitIndexSelectsNone},
		{`^the Git index stages "([^"]+)" with the content "([^"]+)"$`, unitIndexStages},
		{`^the explicit range selects the regular file "([^"]+)"$`, unitRangeSelectsFile},
		{`^the explicit range selects no regular files$`, unitRangeSelectsNone},
		{`^the explicit range changes "([^"]+)" to "([^"]+)"$`, unitRangeChanges},

		// Declared children and commands.
		{`^the declared mutation runs against its isolated snapshot$`, unitMutationRunsInSnapshot},
		{`^the declared mutation rewrites "([^"]+)" to "([^"]+)" in its (isolated snapshot|verify-clean replay)$`, unitMutationRewrites},
		{`^the gate "([^"]+)" exits (\d+)$`, unitGateExits},
		{`^the gate "([^"]+)" interrupts the CLI with signal "([A-Z]+)"$`, unitGateInterrupts},
		{`^the model command "([^"]+)" prints:$`, unitModelPrints},
		{`^the model command "([^"]+)" cannot be started$`, unitModelCannotStart},

		// Acting.
		{`^I invoke the CLI with "([^"]*)"$`, unitInvoke},
		{`^I run the "([^"]+)" validator$`, unitRunValidator},

		// Asserting on the last run.
		{`^the exit code is (\d+)$`, unitExitCodeIs},
		{`^stdout contains "(.*)"$`, unitStdoutContains},
		{`^stderr contains "(.*)"$`, unitStderrContains},
		{`^stdout JSON property "([^"]+)" is (-?\d+)$`, unitStdoutPropertyIs},
		{`^the first stdout JSON violation kind is "([^"]+)"$`, unitFirstViolationKindIs},
		{`^the repository is unchanged by the inspection$`, unitUnchangedByInspection},
		{`^the last adapter generation changes the repository$`, unitGenerationChanges},
		{`^the last adapter generation makes no repository change$`, unitUnchangedByInspection},
		{`^the two runs are byte-identical$`, unitTwoRunsIdentical},
		{`^the file "([^"]+)" still contains "([^"]+)"$`, unitFileStillContains},
		{`^the generated adapter at "([^"]+)" contains "([^"]+)"$`, unitFileStillContains},
		{`^the generated adapter at "([^"]+)" does not contain "([^"]+)"$`, unitFileDoesNotContain},
		{`^the generated adapter at "([^"]+)" is exactly:$`, unitFileIsExactly},
		{`^no generated adapter exists at "([^"]+)"$`, unitNoFileAt},
		{`^the working tree file "([^"]+)" holds "([^"]+)"$`, unitWorkingFileHolds},
		{`^the Git index holds "([^"]+)" at "([^"]+)"$`, unitIndexHoldsAt},
		{`^the gate "([^"]+)" received the argument vector "([^"]*)"$`, unitReceivedArguments},
		{`^the gate "([^"]+)" received the environment projection "([^"]+)"$`, unitReceivedEnvironment},
		{`^the model command "([^"]+)" was started$`, unitModelWasStarted},
		{`^the model command "([^"]+)" was not started$`, unitModelWasNotStarted},
	}
}

func unitConfigurationText(ctx context.Context, doc *godog.DocString) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	w.config = doc.Content
	return w.writeRepoFile("repo-config.yml", doc.Content)
}

func unitRepositoryContains(ctx context.Context, table *godog.Table) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	rows, err := unitTableRows(table)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := w.writeRepoFile(row[0], row[1]); err != nil {
			return err
		}
	}
	return nil
}

// unitOutsideContains writes files beside the repository, in the scenario's
// outside directory, refusing a row that leaves that directory.
func unitOutsideContains(ctx context.Context, table *godog.Table) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	rows, err := unitTableRows(table)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if memLeavesRoot(row[0]) {
			return fmt.Errorf("scenario path %q leaves the outside directory", row[0])
		}
		w.files.writeFile(path.Join(unitOutside, path.Clean(row[0])), row[1], 0o644)
	}
	return nil
}

func unitFileContains(ctx context.Context, rel string, doc *godog.DocString) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	return w.writeRepoFile(rel, doc.Content)
}

func unitRepositoryExecutable(ctx context.Context, rel string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	return w.writeRepoFile(rel, "#!/bin/sh\nexit 0\n")
}

func unitRepositoryExecutableBody(ctx context.Context, rel string, doc *godog.DocString) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	return w.writeRepoFile(rel, doc.Content)
}

func unitDanglingLink(ctx context.Context, rel string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	return w.createLink(rel, "missing-target")
}

func unitLinkInside(ctx context.Context, rel, target string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	return w.createLink(rel, target)
}

// unitLinkOutside places a directory outside the repository that holds one file,
// and links the repository path to it by absolute path.
func unitLinkOutside(ctx context.Context, rel, file string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	held := path.Join(unitOutside, rel)
	w.files.mkdirAll(held)
	w.files.writeFile(path.Join(held, file), "# Held outside the repository\n", 0o644)
	return w.createLink(rel, held)
}

// unitFileCannotBeOpened makes an existing file unreadable to the process.
func unitFileCannotBeOpened(ctx context.Context, rel string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	if err := w.writeRepoFile(rel, "# Sealed\n"); err != nil {
		return err
	}
	location, err := w.repoPath(rel)
	if err != nil {
		return err
	}
	entry, ok := w.files.lstat(location)
	if !ok {
		return fmt.Errorf("%s was not written", rel)
	}
	entry.unreadable = true
	return nil
}

func unitWorkingTreeLink(ctx context.Context, rel, target string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	return w.createLink(rel, target)
}

func unitStandardOutputClosed(ctx context.Context) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	w.stdoutClosed = true
	return nil
}

func unitStandardInput(ctx context.Context, doc *godog.DocString) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	text := doc.Content
	w.stdin = &text
	return nil
}

func unitIndexSelectsFile(ctx context.Context, rel string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	w.ensureGit()
	w.git.setIndex(rel, w.workingContent(rel))
	return nil
}

func unitIndexSelectsNone(ctx context.Context) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	w.ensureGit()
	w.git.resetIndex()
	return nil
}

func unitIndexStages(ctx context.Context, rel, content string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	w.ensureGit()
	w.git.setIndex(rel, content)
	return nil
}

func unitRangeSelectsFile(ctx context.Context, rel string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	w.ensureGit()
	w.git.setRange(rel, w.workingContent(rel))
	return nil
}

// unitRangeSelectsNone leaves the head at the base commit, so the range changes
// nothing.
func unitRangeSelectsNone(ctx context.Context) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	w.ensureGit()
	return nil
}

func unitRangeChanges(ctx context.Context, rel, content string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	w.ensureGit()
	w.git.setRange(rel, content)
	return nil
}

// unitMutationRunsInSnapshot makes every declared mutation child refuse to run in
// the live repository.
func unitMutationRunsInSnapshot(ctx context.Context) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	w.snapshotOn = true
	return nil
}

// unitMutationRewrites installs the declared mutation command as a fixture that
// writes content to a path in its working directory.
func unitMutationRewrites(ctx context.Context, rel, content, _ string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	w.snapshotOn = true
	gate, err := w.mutationGate()
	if err != nil {
		return err
	}
	dir := path.Dir(path.Clean(rel))
	body := "#!/bin/sh\n" + w.childPrologue() + "mkdir -p " + shellQuote(dir) + "\n" +
		"printf '%s' " + shellQuote(content) + " > " + shellQuote(rel) + "\n"
	if err := w.placeExecutable(gate.Command.Executable, body); err != nil {
		return err
	}
	w.children[gate.Command.Executable] = &unitChild{id: gate.ID, mutation: true,
		rewrite: &unitRewrite{path: path.Clean(rel), content: content}}
	return w.trackDeclaredChild(gate.Command.Executable, true)
}

// unitGateExits installs a gate child that records its argument vector, its
// environment, and its working directory, then exits with the declared status.
func unitGateExits(ctx context.Context, id, code string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	gate, err := w.gateByID(id)
	if err != nil {
		return err
	}
	exit, err := parseCode(code)
	if err != nil {
		return err
	}
	body := "#!/bin/sh\n" + w.childPrologue() + "exit " + code + "\n"
	if err := w.placeExecutable(gate.Command.Executable, body); err != nil {
		return err
	}
	mutation := gate.Type == "mutation"
	w.children[gate.Command.Executable] = &unitChild{id: id, exit: exit, mutation: mutation}
	if !mutation {
		return nil
	}
	return w.trackDeclaredChild(gate.Command.Executable, false)
}

// unitGateInterrupts installs a gate child that sends a signal to the run that
// launched it, so the run's own signal handling decides the exit status.
func unitGateInterrupts(ctx context.Context, id, signal string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	gate, err := w.gateByID(id)
	if err != nil {
		return err
	}
	var sig syscall.Signal
	switch signal {
	case "INT":
		sig = syscall.SIGINT
	case "TERM":
		sig = syscall.SIGTERM
	default:
		return fmt.Errorf("the corpus names unknown signal %q", signal)
	}
	if err := w.placeExecutable(gate.Command.Executable, "#!/bin/sh\nkill -s "+signal+" $PPID\nsleep 10\n"); err != nil {
		return err
	}
	w.children[gate.Command.Executable] = &unitChild{id: id, signal: sig, hasSignal: true}
	return nil
}

// unitModelPrints installs a model command that prints the docstring. A new print
// resets the start record.
func unitModelPrints(ctx context.Context, name string, doc *godog.DocString) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	w.modelStarted[name] = false
	w.models[name] = &unitModel{output: doc.Content + "\n", canStart: true}
	body := "#!/bin/sh\ncat <<'RHINO_BDD_EOF'\n" + doc.Content + "\nRHINO_BDD_EOF\n"
	return w.placeExecutable(w.modelCommandPath(name), body)
}

// unitModelCannotStart leaves the command present but not executable.
func unitModelCannotStart(ctx context.Context, name string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	model, ok := w.models[name]
	if !ok {
		model = &unitModel{}
		w.models[name] = model
	}
	model.canStart = false
	command := w.modelCommandPath(name)
	if strings.Contains(command, "/") {
		return w.writeRepoFile(command, "#!/bin/sh\n")
	}
	w.files.writeFile(path.Join("/scenario/bin", command), "#!/bin/sh\n", 0o644)
	return nil
}

func unitInvoke(ctx context.Context, joined string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	return w.invoke(joined)
}

// unitValidatorCommands maps the validator names the corpus uses to the leaves
// that run them.
var unitValidatorCommands = map[string]string{
	"repo-config":       "repo-config|validate",
	"heading-hierarchy": "md|heading-hierarchy|validate",
	"naming":            "md|naming|validate",
	"metadata":          "metadata|validate",
	"emoji":             "convention|emoji|validate",
}

func unitRunValidator(ctx context.Context, name string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	command, ok := unitValidatorCommands[name]
	if !ok {
		return fmt.Errorf("the corpus names unknown validator %q", name)
	}
	return w.invoke(command)
}

func unitExitCodeIs(ctx context.Context, expected string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	run, err := w.lastRun()
	if err != nil {
		return err
	}
	want, err := parseCode(expected)
	if err != nil {
		return err
	}
	if run.exitCode != want {
		return fmt.Errorf("exit code %d; want %d\nstdout:\n%s\nstderr:\n%s", run.exitCode, want, run.stdout, run.stderr)
	}
	return nil
}

func unitStdoutContains(ctx context.Context, expected string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	run, err := w.lastRun()
	if err != nil {
		return err
	}
	if !strings.Contains(run.stdout, expected) {
		return fmt.Errorf("stdout does not contain %q (exit %d):\n%s", expected, run.exitCode, run.stdout)
	}
	return nil
}

func unitStderrContains(ctx context.Context, expected string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	run, err := w.lastRun()
	if err != nil {
		return err
	}
	if !strings.Contains(run.stderr, expected) {
		return fmt.Errorf("stderr does not contain %q (exit %d):\n%s", expected, run.exitCode, run.stderr)
	}
	return nil
}

// unitDecodeStdout parses the last run's stdout as one JSON object, keeping
// numbers exact so a property compares by its literal text.
func unitDecodeStdout(run unitInvocation) (map[string]json.RawMessage, error) {
	var document map[string]json.RawMessage
	decoder := json.NewDecoder(strings.NewReader(run.stdout))
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("stdout is not one JSON object: %w\n%s", err, run.stdout)
	}
	return document, nil
}

func unitStdoutPropertyIs(ctx context.Context, property, expected string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	run, err := w.lastRun()
	if err != nil {
		return err
	}
	document, err := unitDecodeStdout(run)
	if err != nil {
		return err
	}
	value, ok := document[property]
	if !ok {
		return fmt.Errorf("stdout JSON has no property %q", property)
	}
	if strings.TrimSpace(string(value)) != expected {
		return fmt.Errorf("stdout JSON property %q is %s; want %s", property, value, expected)
	}
	return nil
}

func unitFirstViolationKindIs(ctx context.Context, expected string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	run, err := w.lastRun()
	if err != nil {
		return err
	}
	document, err := unitDecodeStdout(run)
	if err != nil {
		return err
	}
	var violations []struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(document["violations"], &violations); err != nil {
		return fmt.Errorf("stdout JSON has no violations array: %w", err)
	}
	if len(violations) == 0 {
		return fmt.Errorf("stdout JSON reports no violation; want kind %q", expected)
	}
	if violations[0].Kind != expected {
		return fmt.Errorf("first violation kind %q; want %q", violations[0].Kind, expected)
	}
	return nil
}

func unitUnchangedByInspection(ctx context.Context) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	run, err := w.lastRun()
	if err != nil {
		return err
	}
	if !sameSnapshot(run.before, run.after) {
		return errors.New("the run changed the repository")
	}
	return nil
}

func unitGenerationChanges(ctx context.Context) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	run, err := w.lastRun()
	if err != nil {
		return err
	}
	if sameSnapshot(run.before, run.after) {
		return errors.New("the last generation made no repository change")
	}
	return nil
}

// unitTwoRunsIdentical compares the last two invocations' exit codes, stdout, and
// stderr.
func unitTwoRunsIdentical(ctx context.Context) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	if len(w.runs) < 2 {
		return errors.New("fewer than two CLI invocations ran in this scenario")
	}
	first, second := w.runs[len(w.runs)-2], w.runs[len(w.runs)-1]
	if first.exitCode != second.exitCode || first.stdout != second.stdout || first.stderr != second.stderr {
		return fmt.Errorf("runs differ:\nfirst stdout:\n%s\nsecond stdout:\n%s", first.stdout, second.stdout)
	}
	return nil
}

func unitFileStillContains(ctx context.Context, rel, expected string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	content, err := w.readRepoFile(rel)
	if err != nil {
		return err
	}
	if !strings.Contains(content, expected) {
		return fmt.Errorf("%s does not contain %q", rel, expected)
	}
	return nil
}

func unitFileDoesNotContain(ctx context.Context, rel, unexpected string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	content, err := w.readRepoFile(rel)
	if err != nil {
		return err
	}
	if strings.Contains(content, unexpected) {
		return fmt.Errorf("%s contains %q", rel, unexpected)
	}
	return nil
}

// unitFileIsExactly compares the file with the docstring, ignoring one final
// newline that the docstring format does not carry.
func unitFileIsExactly(ctx context.Context, rel string, doc *godog.DocString) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	content, err := w.readRepoFile(rel)
	if err != nil {
		return err
	}
	if strings.TrimSuffix(content, "\n") != strings.TrimSuffix(doc.Content, "\n") {
		return fmt.Errorf("%s is:\n%s\nwant:\n%s", rel, content, doc.Content)
	}
	return nil
}

func unitNoFileAt(ctx context.Context, rel string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	location, err := w.repoPath(rel)
	if err != nil {
		return err
	}
	if _, ok := w.files.lstat(location); ok {
		return fmt.Errorf("%s exists", rel)
	}
	return nil
}

func unitWorkingFileHolds(ctx context.Context, rel, expected string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	content, err := w.readRepoFile(rel)
	if err != nil {
		return err
	}
	if content != expected {
		return fmt.Errorf("%s holds %q; want %q", rel, content, expected)
	}
	return nil
}

func unitIndexHoldsAt(ctx context.Context, expected, rel string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	w.ensureGit()
	blob, ok := w.git.index[path.Clean(rel)]
	if !ok {
		return fmt.Errorf("the index holds no entry at %s", rel)
	}
	if blob.content != expected {
		return fmt.Errorf("index holds %q at %s; want %q", blob.content, rel, expected)
	}
	return nil
}

// unitReceivedArguments compares a fixture child's argument vector with the
// expected one, each argument joined with a single space.
func unitReceivedArguments(ctx context.Context, id, expected string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	record, ok := w.logs[id]
	if !ok {
		return fmt.Errorf("gate %q wrote no argv record; it did not run", id)
	}
	if got := strings.Join(record.argv, " "); got != expected {
		return fmt.Errorf("gate %q received argument vector %q; want %q", id, got, expected)
	}
	return nil
}

func unitReceivedEnvironment(ctx context.Context, id, expected string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	record, ok := w.logs[id]
	if !ok {
		return fmt.Errorf("gate %q wrote no environment record; it did not run", id)
	}
	for _, line := range record.env {
		if line == expected {
			return nil
		}
	}
	return fmt.Errorf("gate %q did not receive environment %q", id, expected)
}

func unitModelWasStarted(ctx context.Context, name string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	if !w.modelStarted[name] {
		return fmt.Errorf("model command %q was not started", name)
	}
	return nil
}

func unitModelWasNotStarted(ctx context.Context, name string) error {
	w, err := unitWorldFrom(ctx)
	if err != nil {
		return err
	}
	if w.modelStarted[name] {
		return fmt.Errorf("model command %q was started", name)
	}
	return nil
}
