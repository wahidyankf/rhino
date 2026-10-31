package bdd_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

// binding pairs a step expression with its handler. The table is the only
// registry: the suite registers it with godog, and the compliance test matches
// the same patterns against the corpus.
type binding struct {
	pattern string
	handler any
}

// bindings returns every step definition the corpus uses, grouped by what each
// step does: prepare the repository, act, or assert on the result.
func bindings() []binding {
	return []binding{
		// Preparing the repository.
		{`^the configuration file is this text:$`, configurationFileText},
		{`^the repository contains:$`, repositoryContains},
		{`^file "([^"]+)" contains this Markdown:$`, fileContainsText},
		{`^the repository contains an executable "([^"]+)"$`, repositoryContainsExecutable},
		{`^the repository contains an executable "([^"]+)" with the body:$`, repositoryContainsExecutableWithBody},
		{`^the repository contains a symbolic link at "([^"]+)"$`, repositoryContainsDanglingLink},
		{`^the repository contains a symbolic link at "([^"]+)" to "([^"]+)" inside it$`, repositoryContainsLinkInside},
		{`^the repository contains a symbolic link at "([^"]+)" to a directory outside it holding "([^"]+)"$`, repositoryContainsLinkOutside},
		{`^file "([^"]+)" cannot be opened$`, fileCannotBeOpened},
		{`^the working tree path "([^"]+)" is a symbolic link to "([^"]+)"$`, workingTreePathIsLink},
		{`^the standard input is:$`, standardInputIs},
		{`^the Git index selects the regular file "([^"]+)"$`, gitIndexSelectsFile},
		{`^the Git index selects no regular files$`, gitIndexSelectsNone},
		{`^the Git index stages "([^"]+)" with the content "([^"]+)"$`, gitIndexStages},
		{`^the explicit range selects the regular file "([^"]+)"$`, explicitRangeSelectsFile},
		{`^the explicit range selects no regular files$`, explicitRangeSelectsNone},
		{`^the explicit range changes "([^"]+)" to "([^"]+)"$`, explicitRangeChanges},

		// Declared children and commands.
		{`^the declared mutation runs against its isolated snapshot$`, declaredMutationRunsInSnapshot},
		{`^the declared mutation rewrites "([^"]+)" to "([^"]+)" in its (isolated snapshot|verify-clean replay)$`, declaredMutationRewrites},
		{`^the gate "([^"]+)" exits (\d+)$`, gateExits},
		{`^the gate "([^"]+)" interrupts the CLI with signal "([A-Z]+)"$`, gateInterruptsCLI},
		{`^the model command "([^"]+)" prints:$`, modelCommandPrints},
		{`^the model command "([^"]+)" cannot be started$`, modelCommandCannotStart},

		// Acting.
		{`^I invoke the CLI with "([^"]*)"$`, invokeCLI},
		{`^I run the "([^"]+)" validator$`, runNamedValidator},

		// Asserting on the last run.
		{`^the exit code is (\d+)$`, exitCodeIs},
		{`^stdout contains "(.*)"$`, stdoutContains},
		{`^stderr contains "(.*)"$`, stderrContains},
		{`^stdout JSON property "([^"]+)" is (-?\d+)$`, stdoutJSONPropertyIs},
		{`^the first stdout JSON violation kind is "([^"]+)"$`, firstViolationKindIs},
		{`^the repository is unchanged by the inspection$`, repositoryUnchangedByInspection},
		{`^the last adapter generation changes the repository$`, lastGenerationChangesRepository},
		{`^the last adapter generation makes no repository change$`, lastGenerationMakesNoChange},
		{`^the two runs are byte-identical$`, lastTwoRunsIdentical},
		{`^the file "([^"]+)" still contains "([^"]+)"$`, fileStillContains},
		{`^the generated adapter at "([^"]+)" contains "([^"]+)"$`, generatedAdapterContains},
		{`^the generated adapter at "([^"]+)" does not contain "([^"]+)"$`, generatedAdapterDoesNotContain},
		{`^the generated adapter at "([^"]+)" is exactly:$`, generatedAdapterIsExactly},
		{`^no generated adapter exists at "([^"]+)"$`, noGeneratedAdapter},
		{`^the working tree file "([^"]+)" holds "([^"]+)"$`, workingTreeFileHolds},
		{`^the Git index holds "([^"]+)" at "([^"]+)"$`, gitIndexHoldsAt},
		{`^the gate "([^"]+)" received the argument vector "([^"]*)"$`, gateReceivedArguments},
		{`^the gate "([^"]+)" received the environment projection "([^"]+)"$`, gateReceivedEnvironment},
		{`^the model command "([^"]+)" was started$`, modelCommandWasStarted},
		{`^the model command "([^"]+)" was not started$`, modelCommandWasNotStarted},
	}
}

// stepWorld resolves the scenario state a step needs.
func stepWorld(ctx context.Context) (*world, error) {
	return worldFrom(ctx)
}

// tableRows returns the data rows of a two-column table with a path/content
// header, so a malformed table fails instead of being read as data.
func tableRows(table *godog.Table) ([][2]string, error) {
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

func configurationFileText(ctx context.Context, doc *godog.DocString) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	w.config = doc.Content
	return w.writeRepoFile("repo-config.yml", doc.Content)
}

func repositoryContains(ctx context.Context, table *godog.Table) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	rows, err := tableRows(table)
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

func fileContainsText(ctx context.Context, path string, doc *godog.DocString) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	return w.writeRepoFile(path, doc.Content)
}

func repositoryContainsExecutable(ctx context.Context, path string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	return w.writeRepoFile(path, "#!/bin/sh\nexit 0\n")
}

// repositoryContainsExecutableWithBody installs an executable whose body is the
// docstring, so a scenario controls what the program prints and how long it runs.
func repositoryContainsExecutableWithBody(ctx context.Context, path string, doc *godog.DocString) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	return w.writeRepoFile(path, doc.Content)
}

// repositoryContainsDanglingLink places a link whose target does not exist.
func repositoryContainsDanglingLink(ctx context.Context, path string) error {
	return createLink(ctx, path, "missing-target")
}

func repositoryContainsLinkInside(ctx context.Context, path, target string) error {
	return createLink(ctx, path, target)
}

// repositoryContainsLinkOutside places a directory outside the repository that
// holds one file, and links the repository path to it by absolute path.
func repositoryContainsLinkOutside(ctx context.Context, path, file string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	held := filepath.Join(w.outside, filepath.FromSlash(path))
	if err := os.MkdirAll(held, 0o755); err != nil {
		return fmt.Errorf("create outside directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(held, file), []byte("# Held outside the repository\n"), 0o644); err != nil {
		return fmt.Errorf("write outside file: %w", err)
	}
	return createLink(ctx, path, held)
}

// createLink replaces whatever is at a repository path with a symbolic link.
func createLink(ctx context.Context, path, target string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	location, err := w.repoPath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(location), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}
	if err := os.RemoveAll(location); err != nil {
		return fmt.Errorf("clear %s: %w", path, err)
	}
	return os.Symlink(target, location)
}

// fileCannotBeOpened makes an existing file unreadable to this process.
func fileCannotBeOpened(ctx context.Context, path string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	if _, err := w.repoPath(path); err != nil {
		return err
	}
	if err := w.writeRepoFile(path, "# Sealed\n"); err != nil {
		return err
	}
	location, _ := w.repoPath(path)
	return os.Chmod(location, 0o000)
}

// workingTreePathIsLink replaces a working-tree path with a relative link.
func workingTreePathIsLink(ctx context.Context, path, target string) error {
	return createLink(ctx, path, target)
}

func standardInputIs(ctx context.Context, doc *godog.DocString) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	text := doc.Content
	w.stdin = &text
	return nil
}

func gitIndexSelectsFile(ctx context.Context, path string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	content, err := w.workingContent(path)
	if err != nil {
		return err
	}
	return w.indexSet(ctx, path, content)
}

func gitIndexSelectsNone(ctx context.Context) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	return w.indexReset(ctx)
}

func gitIndexStages(ctx context.Context, path, content string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	return w.indexSet(ctx, path, content)
}

func explicitRangeSelectsFile(ctx context.Context, path string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	content, err := w.workingContent(path)
	if err != nil {
		return err
	}
	return w.rangeSet(ctx, path, content)
}

// explicitRangeSelectsNone leaves the head at the base commit, so the range
// changes nothing.
func explicitRangeSelectsNone(ctx context.Context) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	return w.ensureGit(ctx)
}

func explicitRangeChanges(ctx context.Context, path, content string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	return w.rangeSet(ctx, path, content)
}

// declaredMutationRunsInSnapshot makes every declared mutation child refuse to
// run in the live repository. Such a child exits 0 only inside a snapshot.
func declaredMutationRunsInSnapshot(ctx context.Context) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	w.snapshotOn = true
	return nil
}

// declaredMutationRewrites installs the declared mutation command as a fixture
// that writes content to a path in its working directory. The mode word says
// which replay the scenario means; both replays run the same declared command.
func declaredMutationRewrites(ctx context.Context, path, content, _ string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	w.snapshotOn = true
	gate, err := w.mutationGate()
	if err != nil {
		return err
	}
	prologue, err := w.childPrologue()
	if err != nil {
		return err
	}
	dir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(path)))
	body := "#!/bin/sh\n" + prologue + "mkdir -p " + shellQuote(dir) + "\n" +
		"printf '%s' " + shellQuote(content) + " > " + shellQuote(path) + "\n"
	if err := w.placeExecutable(gate.Command.Executable, body); err != nil {
		return err
	}
	return w.trackDeclaredChild(ctx, gate.Command.Executable, true)
}

// trackDeclaredChild commits a repository-relative child so the index snapshot
// and the replay checkouts, which hold only committed content, can start it. A
// bare name on PATH lives outside the repository and needs no tracking.
func (w *world) trackDeclaredChild(ctx context.Context, command string, staged bool) error {
	if !strings.Contains(command, "/") {
		return nil
	}
	return w.trackExecutable(ctx, filepath.ToSlash(filepath.Clean(command)), staged)
}

// childPrologue makes a fixture child exit 1 when it runs in the live
// repository. Only a declared mutation's child uses it, because a mutation
// must never change the working tree it was asked to check.
func (w *world) childPrologue() (string, error) {
	if !w.snapshotOn {
		return "", nil
	}
	live, err := filepath.EvalSymlinks(w.repo)
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	return "if [ \"$(pwd -P)\" = " + shellQuote(live) + " ]; then\n" +
		"  echo 'declared mutation ran in the live repository' >&2\n  exit 1\nfi\n", nil
}

// gateExits installs a gate child that records its argv, its environment, and
// its working directory, then exits with the declared status.
func gateExits(ctx context.Context, id string, code string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	gate, err := w.gateByID(id)
	if err != nil {
		return err
	}
	prologue, err := w.childPrologue()
	if err != nil {
		return err
	}
	log := shellQuote(w.childLog(id))
	body := "#!/bin/sh\n" + prologue +
		": > " + log + ".argv\n" +
		"for argument in \"$@\"; do\n  printf '%s\\n' \"$argument\" >> " + log + ".argv\ndone\n" +
		"env > " + log + ".env\n" +
		"pwd -P > " + log + ".cwd\n" +
		"exit " + code + "\n"
	if err := w.placeExecutable(gate.Command.Executable, body); err != nil {
		return err
	}
	if gate.Type != "mutation" {
		return nil
	}
	return w.trackDeclaredChild(ctx, gate.Command.Executable, false)
}

// gateInterruptsCLI installs a gate child that sends a signal to the RHINO
// process that launched it and then waits, so RHINO's own signal handling, not
// the child's status, decides the exit code.
func gateInterruptsCLI(ctx context.Context, id, signal string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	gate, err := w.gateByID(id)
	if err != nil {
		return err
	}
	return w.placeExecutable(gate.Command.Executable, "#!/bin/sh\nkill -s "+signal+" $PPID\nsleep 10\n")
}

// modelCommandPath finds the path the configuration names for a model command,
// so the fixture is written where the declared command will look.
func (w *world) modelCommandPath(name string) string {
	pattern := regexp.MustCompile(`[A-Za-z0-9_./-]*` + regexp.QuoteMeta(name) + `[A-Za-z0-9_./-]*`)
	if match := pattern.FindString(w.config); match != "" {
		return match
	}
	return name
}

func (w *world) modelMarker(name string) string {
	return filepath.Join(w.logs, "model-"+name+".started")
}

// modelCommandPrints installs a model command that records that it started and
// prints the docstring. A new print resets the start record.
func modelCommandPrints(ctx context.Context, name string, doc *godog.DocString) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	marker := shellQuote(w.modelMarker(name))
	if err := os.RemoveAll(w.modelMarker(name)); err != nil {
		return fmt.Errorf("reset start record: %w", err)
	}
	body := "#!/bin/sh\n: > " + marker + "\ncat <<'RHINO_BDD_EOF'\n" + doc.Content + "\nRHINO_BDD_EOF\n"
	return w.placeExecutable(w.modelCommandPath(name), body)
}

// modelCommandCannotStart leaves the command file present but not executable.
func modelCommandCannotStart(ctx context.Context, name string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	command := w.modelCommandPath(name)
	body := "#!/bin/sh\n: > " + shellQuote(w.modelMarker(name)) + "\n"
	if err := w.placeExecutable(command, body); err != nil {
		return err
	}
	location := filepath.Join(w.bin, command)
	if strings.Contains(command, "/") {
		location, err = w.repoPath(command)
		if err != nil {
			return err
		}
	}
	return os.Chmod(location, 0o644)
}

func invokeCLI(ctx context.Context, joined string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	return w.invoke(ctx, joined)
}

// validatorCommands maps the validator names the corpus uses to the leaves
// that run them.
var validatorCommands = map[string]string{
	"repo-config":       "repo-config|validate",
	"heading-hierarchy": "md|heading-hierarchy|validate",
	"naming":            "md|naming|validate",
	"metadata":          "metadata|validate",
	"emoji":             "convention|emoji|validate",
}

func runNamedValidator(ctx context.Context, name string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	command, ok := validatorCommands[name]
	if !ok {
		return fmt.Errorf("the corpus names unknown validator %q", name)
	}
	return w.invoke(ctx, command)
}

func exitCodeIs(ctx context.Context, expected string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	run, err := w.lastRun()
	if err != nil {
		return err
	}
	want, err := strconv.Atoi(expected)
	if err != nil {
		return err
	}
	if run.exitCode != want {
		return fmt.Errorf("exit code %d; want %d\nstdout:\n%s\nstderr:\n%s", run.exitCode, want, run.stdout, run.stderr)
	}
	return nil
}

func stdoutContains(ctx context.Context, expected string) error {
	w, err := stepWorld(ctx)
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

func stderrContains(ctx context.Context, expected string) error {
	w, err := stepWorld(ctx)
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

// decodeStdout parses the last run's stdout as one JSON object, keeping numbers
// exact so a property compares by its literal text.
func decodeStdout(run invocation) (map[string]json.RawMessage, error) {
	var document map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader([]byte(run.stdout)))
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("stdout is not one JSON object: %w\n%s", err, run.stdout)
	}
	return document, nil
}

func stdoutJSONPropertyIs(ctx context.Context, property, expected string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	run, err := w.lastRun()
	if err != nil {
		return err
	}
	document, err := decodeStdout(run)
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

func firstViolationKindIs(ctx context.Context, expected string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	run, err := w.lastRun()
	if err != nil {
		return err
	}
	document, err := decodeStdout(run)
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

func repositoryUnchangedByInspection(ctx context.Context) error {
	w, err := stepWorld(ctx)
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

func lastGenerationChangesRepository(ctx context.Context) error {
	w, err := stepWorld(ctx)
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

func lastGenerationMakesNoChange(ctx context.Context) error {
	return repositoryUnchangedByInspection(ctx)
}

// lastTwoRunsIdentical compares the last two invocations' exit codes, stdout,
// and stderr. The JSON contract promises byte-identical output for one state.
func lastTwoRunsIdentical(ctx context.Context) error {
	w, err := stepWorld(ctx)
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

func fileStillContains(ctx context.Context, path, expected string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	content, err := w.readRepoFile(path)
	if err != nil {
		return err
	}
	if !strings.Contains(content, expected) {
		return fmt.Errorf("%s does not contain %q", path, expected)
	}
	return nil
}

func generatedAdapterContains(ctx context.Context, path, expected string) error {
	return fileStillContains(ctx, path, expected)
}

func generatedAdapterDoesNotContain(ctx context.Context, path, unexpected string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	content, err := w.readRepoFile(path)
	if err != nil {
		return err
	}
	if strings.Contains(content, unexpected) {
		return fmt.Errorf("%s contains %q", path, unexpected)
	}
	return nil
}

// generatedAdapterIsExactly compares the file with the docstring, ignoring one
// final newline that the docstring format does not carry.
func generatedAdapterIsExactly(ctx context.Context, path string, doc *godog.DocString) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	content, err := w.readRepoFile(path)
	if err != nil {
		return err
	}
	if strings.TrimSuffix(content, "\n") != strings.TrimSuffix(doc.Content, "\n") {
		return fmt.Errorf("%s is:\n%s\nwant:\n%s", path, content, doc.Content)
	}
	return nil
}

func noGeneratedAdapter(ctx context.Context, path string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	location, err := w.repoPath(path)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(location); err == nil {
		return fmt.Errorf("%s exists", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func workingTreeFileHolds(ctx context.Context, path, expected string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	content, err := w.readRepoFile(path)
	if err != nil {
		return err
	}
	if content != expected {
		return fmt.Errorf("%s holds %q; want %q", path, content, expected)
	}
	return nil
}

func gitIndexHoldsAt(ctx context.Context, expected, path string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	content, err := w.indexBlob(ctx, path)
	if err != nil {
		return err
	}
	if content != expected {
		return fmt.Errorf("index holds %q at %s; want %q", content, path, expected)
	}
	return nil
}

// gateReceivedArguments compares a fixture child's argv record with the
// expected vector. The record holds one argument per line, so an empty record
// is an empty vector, and each argument is joined with a single space.
func gateReceivedArguments(ctx context.Context, id, expected string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	record, err := w.readChildLog(id, ".argv")
	if err != nil {
		return err
	}
	var arguments []string
	if record != "" {
		arguments = strings.Split(strings.TrimSuffix(record, "\n"), "\n")
	}
	if got := strings.Join(arguments, " "); got != expected {
		return fmt.Errorf("gate %q received argument vector %q; want %q", id, got, expected)
	}
	return nil
}

func gateReceivedEnvironment(ctx context.Context, id, expected string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	record, err := w.readChildLog(id, ".env")
	if err != nil {
		return err
	}
	for _, line := range strings.Split(record, "\n") {
		if line == expected {
			return nil
		}
	}
	return fmt.Errorf("gate %q did not receive environment %q", id, expected)
}

func modelCommandWasStarted(ctx context.Context, name string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	if _, err := os.Stat(w.modelMarker(name)); err != nil {
		return fmt.Errorf("model command %q was not started: %w", name, err)
	}
	return nil
}

func modelCommandWasNotStarted(ctx context.Context, name string) error {
	w, err := stepWorld(ctx)
	if err != nil {
		return err
	}
	if _, err := os.Stat(w.modelMarker(name)); err == nil {
		return fmt.Errorf("model command %q was started", name)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
