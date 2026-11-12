// Package vault is the shared step registry for the two environment vault
// corpora: specs/behaviours/environment-vault-validation.feature, which holds
// the manifest rules, and specs/behaviours/environment-vault.feature, which
// holds the commands.
//
// Every adapter binds these step texts once, here, and supplies only a Driver:
// the unit adapter builds the repository in memory, the integration adapter in
// a real temporary Git repository, and the command-line adapter in the
// scenario repository the built binary runs in. This package is test support.
// No product package imports it.
package vault

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"gopkg.in/yaml.v3"
)

// Driver is one scenario's repository and the command under test, as one
// adapter builds them. Every method receives the scenario context.
type Driver interface {
	// Declare writes the repository's environment configuration and makes the
	// repository a Git repository.
	Declare(ctx context.Context, configuration string) error
	// Track adds a tracked file with content.
	Track(ctx context.Context, path, content string) error
	// Ignore adds an untracked file that Git ignores, with content.
	Ignore(ctx context.Context, path, content string) error
	// Validate runs env validate, as JSON when asJSON is set.
	Validate(ctx context.Context, asJSON bool) (Result, error)
	// Session sets the vault session the next command meets.
	Session(ctx context.Context, state Session, server string) error
	// Login adds one login item, with the password, under name. A name held
	// already is held twice, which is how an ambiguous name is built.
	Login(ctx context.Context, name, password string) error
	// Generation adds one attachment named file, holding content, to the
	// secure note called name, which it creates when absent.
	Generation(ctx context.Context, name, file, content string) error
	// Local writes a repository file that the repository does not track, with
	// content and the default mode a checkout gives it.
	Local(ctx context.Context, path, content string) error
	// Chmod sets the permission bits of a repository file the scenario holds,
	// as four octal digits.
	Chmod(ctx context.Context, path, mode string) error
	// Command runs one env command with its arguments, and records the
	// repository and the vault before and after the run.
	Command(ctx context.Context, arguments []string) (Result, error)
	// LocalFile reports whether a repository file exists, its content, and its
	// permission bits as four octal digits.
	LocalFile(ctx context.Context, path string) (LocalFile, error)
	// Items returns every vault item held under the exact name, in creation order.
	Items(ctx context.Context, name string) ([]ItemView, error)
	// Close removes the scenario's fixture.
	Close() error
}

// Session is the state of the vault session a scenario sets.
type Session string

// The four session states a scenario can set.
const (
	SessionUnlocked    Session = "unlocked"
	SessionNoKey       Session = "no-key"
	SessionLocked      Session = "locked"
	SessionOtherServer Session = "other-server"
)

// otherServer is the server a session points at when a scenario sets the
// session to another server than the one the repository declares.
const otherServer = "https://other.example.invalid"

// LocalFile is one repository file as a scenario sees it.
type LocalFile struct {
	Exists  bool
	Content string
	Mode    string
}

// ItemView is one vault item as a scenario reads it from the provider's state,
// never from a command's output. Kind is login or note, Password the login's
// value, History its retired passwords, and Attachments the content of each
// attachment by file name.
type ItemView struct {
	Kind        string
	Password    string
	History     []string
	Attachments map[string]string
}

// Result is one command run: its exit code, both of its streams, and every
// process the run started, in order.
type Result struct {
	ExitCode  int
	Stdout    string
	Stderr    string
	Processes []Process
	// LocalBefore and LocalAfter fingerprint the repository's files either
	// side of the run; VaultBefore and VaultAfter the vault's state.
	LocalBefore string
	LocalAfter  string
	VaultBefore string
	VaultAfter  string
}

// Process is one program a run started and its arguments, as the adapter saw
// them. A unit run records the Git name queries it asked; a real run records
// the git child the shim logged.
type Process struct {
	Program string
	Args    []string
}

// readOnlyGitVerbs are the Git verbs a validation run may start. Every name
// query is ls-files, and no other verb reads the repository without a write.
var readOnlyGitVerbs = map[string]bool{"ls-files": true}

// gitVerb returns the Git subcommand in argv, skipping the `-C <dir>` pair an
// adapter prefixes, or an empty string when none is present.
func gitVerb(args []string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == "-C" {
			i++
			continue
		}
		return args[i]
	}
	return ""
}

// Binding pairs one step expression with its handler, in the form godog
// registers.
type Binding struct {
	Pattern string
	Handler any
}

// scenario is the state one scenario carries through the godog context.
type scenario struct {
	driver Driver
	server string
	runs   []Result
}

type scenarioKey struct{}

// WithDriver carries one scenario's driver through the godog context.
func WithDriver(ctx context.Context, driver Driver) context.Context {
	return context.WithValue(ctx, scenarioKey{}, &scenario{driver: driver})
}

func scenarioFrom(ctx context.Context) (*scenario, error) {
	current, ok := ctx.Value(scenarioKey{}).(*scenario)
	if !ok || current == nil {
		return nil, errors.New("no vault scenario in context")
	}
	return current, nil
}

// Bindings returns every step the environment vault corpus uses, each once.
func Bindings() []Binding {
	return []Binding{
		{`^the repository declares this environment configuration:$`, declareConfiguration},
		{`^the repository tracks "([^"]+)" with the content "([^"]*)"$`, trackFile},
		{`^the repository ignores "([^"]+)" with the content "([^"]*)"$`, ignoreFile},
		{`^I run environment validation$`, runValidation},
		{`^I run environment validation in (text|json) format$`, runValidationIn},
		{`^the command exits (\d+)$`, commandExits},
		{`^the output names "([^"]*)"$`, outputNames},
		{`^the output does not name "([^"]*)"$`, outputDoesNotName},
		{`^the summary line is "([^"]*)"$`, summaryLineIs},
		{`^no run output contains "([^"]*)"$`, noRunOutputContains},
		{`^the run started only read-only Git queries$`, startedOnlyReadOnlyQueries},
		{`^the run started only the Bitwarden CLI$`, startedOnlyBitwarden},
		{`^the vault session (is unlocked for the declared server|has no key|is locked|points at another server)$`, setSession},
		{`^the vault holds a login item "([^"]*)" with the password "([^"]*)"$`, addLogin},
		{`^the vault holds the secure note "([^"]*)" with the attachment "([^"]*)" holding the content "([^"]*)"$`, addGeneration},
		{`^the repository holds the local file "([^"]+)" with the content "([^"]*)"$`, writeLocal},
		{`^the repository holds the local file "([^"]+)" with this content:$`, writeLocalDocument},
		{`^the repository gives the local file "([^"]+)" the mode "([0-7]{4})"$`, giveLocalMode},
		{`^I run environment (pull|push|diff)(?: with (force|overwrite))?(?: in (text|json) format)?$`, runEnvironment},
		{`^I run environment (pull|push|diff) for the entry "([^"]+)"$`, runEnvironmentForEntry},
		{`^the local file "([^"]+)" holds the content "([^"]*)"$`, localHoldsContent},
		{`^the local file "([^"]+)" holds this content:$`, localHoldsDocument},
		{`^the local file "([^"]+)" does not exist$`, localAbsent},
		{`^the local file "([^"]+)" has the mode "([0-7]{4})"$`, localHasMode},
		{`^the local files are unchanged by the last run$`, localFilesUnchanged},
		{`^the vault is unchanged by the last run$`, vaultUnchanged},
		{`^the vault holds the login item "([^"]*)" with the password "([^"]*)"$`, vaultHoldsLogin},
		{`^the login item "([^"]*)" keeps the password "([^"]*)" in its history$`, loginKeepsHistory},
		{`^the secure note "([^"]*)" holds the attachment "([^"]*)"$`, noteHoldsAttachment},
		{`^the secure note "([^"]*)" holds the attachment "([^"]*)" with the content "([^"]*)"$`, noteHoldsContent},
		{`^the secure note "([^"]*)" does not hold the attachment "([^"]*)"$`, noteLacksAttachment},
	}
}

func declareConfiguration(ctx context.Context, doc *godog.DocString) error {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return err
	}
	var document struct {
		Environment struct {
			Vault struct {
				Server string `yaml:"server"`
			} `yaml:"vault"`
		} `yaml:"environment"`
	}
	if err := yaml.Unmarshal([]byte(doc.Content), &document); err != nil {
		return fmt.Errorf("the declared configuration does not parse: %w", err)
	}
	current.server = document.Environment.Vault.Server
	return current.driver.Declare(ctx, doc.Content)
}

func trackFile(ctx context.Context, path, content string) error {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return err
	}
	return current.driver.Track(ctx, path, content)
}

func ignoreFile(ctx context.Context, path, content string) error {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return err
	}
	return current.driver.Ignore(ctx, path, content)
}

func runValidation(ctx context.Context) error {
	return validate(ctx, false)
}

// runValidationIn runs the validation in the named output format: text, or
// json for --output json.
func runValidationIn(ctx context.Context, format string) error {
	return validate(ctx, format == "json")
}

// startedOnlyReadOnlyQueries holds the last run to its process record: it
// started at least the two Git name queries, and every process it started was
// a git invocation with a read-only verb. A program name or verb is printed;
// no argument value is.
//
// What this step proves, per adapter:
//   - Unit: the in-memory tree records the argv its two name queries hard-code,
//     so the verb here is the verb that double names. No process starts in the
//     unit layer, so this step does not show that any process ran.
//   - Integration and command line: the git child the PATH shim logged. The
//     shim catches only a program started by bare name through PATH. A program
//     started by absolute path is not logged, and no traffic is observed.
//
// Proofs that stand beside this step, and do not depend on it:
//   - internal/adapters/runtime/read_boundary_test.go pins the Git verbs that
//     disk.go may start, and which functions may write.
//   - internal/bootstrap/boundary_test.go pins the validator cores' imports, the
//     binary's dependency graph to hold no network package, and process
//     spawning to the runtime adapter.
func startedOnlyReadOnlyQueries(ctx context.Context) error {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return err
	}
	last, err := current.lastRun()
	if err != nil {
		return err
	}
	if len(last.Processes) < 2 {
		return fmt.Errorf("the last run recorded %d processes, want at least the two Git name queries", len(last.Processes))
	}
	for _, process := range last.Processes {
		if process.Program != "git" {
			return fmt.Errorf("the last run started %q, which is not a Git query", process.Program)
		}
		if verb := gitVerb(process.Args); !readOnlyGitVerbs[verb] {
			return fmt.Errorf("the last run started git %q, which is not a read-only query", verb)
		}
	}
	return nil
}

// startedOnlyBitwarden holds the last run to its process record: it started at
// least one process, and every process it started was the Bitwarden CLI. A
// program name is printed; no argument value is.
//
// What this step proves, per adapter:
//   - Unit: the in-memory double records each provider call it answers, as the
//     program bw. No process starts in the unit layer.
//   - Integration and command line: the bw child the fake logged. The fake is
//     reached only by its bare name through PATH, so a bw started by absolute
//     path is not logged.
func startedOnlyBitwarden(ctx context.Context) error {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return err
	}
	last, err := current.lastRun()
	if err != nil {
		return err
	}
	if len(last.Processes) == 0 {
		return errors.New("the last run recorded no process, so the claim cannot be checked")
	}
	for _, process := range last.Processes {
		if process.Program != "bw" {
			return fmt.Errorf("the last run started %q, which is not the Bitwarden CLI", process.Program)
		}
	}
	return nil
}

func validate(ctx context.Context, asJSON bool) error {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return err
	}
	result, err := current.driver.Validate(ctx, asJSON)
	if err != nil {
		return err
	}
	current.runs = append(current.runs, result)
	return nil
}

// lastRun returns the most recent command run, or an error when none ran.
func (s *scenario) lastRun() (Result, error) {
	if len(s.runs) == 0 {
		return Result{}, errors.New("no command has run in this scenario")
	}
	return s.runs[len(s.runs)-1], nil
}

func commandExits(ctx context.Context, code string) error {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return err
	}
	want, err := strconv.Atoi(code)
	if err != nil {
		return fmt.Errorf("exit code %q is not a number: %w", code, err)
	}
	last, err := current.lastRun()
	if err != nil {
		return err
	}
	if last.ExitCode != want {
		return fmt.Errorf("the last run exited %d, want %d; stderr: %s", last.ExitCode, want, last.Stderr)
	}
	return nil
}

func outputNames(ctx context.Context, text string) error {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return err
	}
	last, err := current.lastRun()
	if err != nil {
		return err
	}
	if !strings.Contains(last.Stdout, text) && !strings.Contains(last.Stderr, text) {
		return fmt.Errorf("the last run's output does not name %q", text)
	}
	return nil
}

// noRunOutputContains fails without echoing the value it searched for, so a
// failing run never prints a marker value the scenario was proving absent.
func noRunOutputContains(ctx context.Context, value string) error {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return err
	}
	if len(current.runs) == 0 {
		return errors.New("no command has run, so the absence of a value proves nothing")
	}
	for index, run := range current.runs {
		if strings.Contains(run.Stdout, value) || strings.Contains(run.Stderr, value) {
			return fmt.Errorf("run %d of %d prints the value it must not print", index+1, len(current.runs))
		}
	}
	return nil
}

// setSession maps the session phrase onto the session state the driver sets.
func setSession(ctx context.Context, phrase string) error {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return err
	}
	states := map[string]Session{
		"is unlocked for the declared server": SessionUnlocked,
		"has no key":                          SessionNoKey,
		"is locked":                           SessionLocked,
		"points at another server":            SessionOtherServer,
	}
	state, ok := states[phrase]
	if !ok {
		return fmt.Errorf("unknown session phrase %q", phrase)
	}
	server := current.server
	if state == SessionOtherServer {
		server = otherServer
	}
	return current.driver.Session(ctx, state, server)
}

func addLogin(ctx context.Context, name, password string) error {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return err
	}
	return current.driver.Login(ctx, name, password)
}

func addGeneration(ctx context.Context, name, file, content string) error {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return err
	}
	return current.driver.Generation(ctx, name, file, content)
}

func writeLocal(ctx context.Context, path, content string) error {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return err
	}
	return current.driver.Local(ctx, path, content)
}

// writeLocalDocument writes a docstring as a file. A file ends with a newline,
// which a docstring does not carry, so one is added when the text lacks it.
func writeLocalDocument(ctx context.Context, path string, doc *godog.DocString) error {
	content := doc.Content
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return writeLocal(ctx, path, content)
}

// giveLocalMode sets the permission bits of a repository file the scenario holds.
func giveLocalMode(ctx context.Context, path, mode string) error {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return err
	}
	return current.driver.Chmod(ctx, path, mode)
}

// outputDoesNotName asserts that neither stream of the last run names text.
func outputDoesNotName(ctx context.Context, text string) error {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return err
	}
	last, err := current.lastRun()
	if err != nil {
		return err
	}
	if strings.Contains(last.Stdout, text) || strings.Contains(last.Stderr, text) {
		return fmt.Errorf("the last run's output names %q", text)
	}
	return nil
}

// summaryLineIs asserts that one line of the last run's standard output is line.
func summaryLineIs(ctx context.Context, line string) error {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return err
	}
	last, err := current.lastRun()
	if err != nil {
		return err
	}
	for _, got := range strings.Split(last.Stdout, "\n") {
		if got == line {
			return nil
		}
	}
	return fmt.Errorf("the last run's standard output has no summary line %q", line)
}

// runEnvironmentForEntry runs one env command narrowed to one declared entry.
func runEnvironmentForEntry(ctx context.Context, command, entry string) error {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return err
	}
	result, err := current.driver.Command(ctx, []string{"env", command, "--entry", entry})
	if err != nil {
		return err
	}
	current.runs = append(current.runs, result)
	return nil
}

// runEnvironment runs one env command. The option is force or overwrite, or
// empty; the format is text or json, or empty for text.
func runEnvironment(ctx context.Context, command, option, format string) error {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return err
	}
	arguments := []string{"env", command}
	switch option {
	case "force":
		arguments = append(arguments, "--force")
	case "overwrite":
		arguments = append(arguments, "--overwrite")
	}
	if format == "json" {
		arguments = append(arguments, "--output", "json")
	}
	result, err := current.driver.Command(ctx, arguments)
	if err != nil {
		return err
	}
	current.runs = append(current.runs, result)
	return nil
}

func localFile(ctx context.Context, path string) (LocalFile, error) {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return LocalFile{}, err
	}
	return current.driver.LocalFile(ctx, path)
}

func localHoldsContent(ctx context.Context, path, content string) error {
	file, err := localFile(ctx, path)
	if err != nil {
		return err
	}
	if !file.Exists {
		return fmt.Errorf("the local file %q does not exist", path)
	}
	if file.Content != content {
		return fmt.Errorf("the local file %q holds different content than the scenario expects", path)
	}
	return nil
}

// localHoldsDocument compares the file with the docstring's lines. The file
// ends with one newline, which the docstring does not carry, so one trailing
// newline is ignored on both sides.
func localHoldsDocument(ctx context.Context, path string, doc *godog.DocString) error {
	file, err := localFile(ctx, path)
	if err != nil {
		return err
	}
	if !file.Exists {
		return fmt.Errorf("the local file %q does not exist", path)
	}
	if strings.TrimSuffix(file.Content, "\n") != strings.TrimSuffix(doc.Content, "\n") {
		return fmt.Errorf("the local file %q holds different content than the scenario expects", path)
	}
	return nil
}

func localAbsent(ctx context.Context, path string) error {
	file, err := localFile(ctx, path)
	if err != nil {
		return err
	}
	if file.Exists {
		return fmt.Errorf("the local file %q exists, so the run wrote or kept it", path)
	}
	return nil
}

func localHasMode(ctx context.Context, path, mode string) error {
	file, err := localFile(ctx, path)
	if err != nil {
		return err
	}
	if !file.Exists {
		return fmt.Errorf("the local file %q does not exist", path)
	}
	if file.Mode != mode {
		return fmt.Errorf("the local file %q has mode %s, want %s", path, file.Mode, mode)
	}
	return nil
}

// snapshotChanged reports whether a run changed the snapshot it was taken
// around. An empty snapshot means the driver recorded none, which proves nothing.
func snapshotChanged(before, after string) (bool, error) {
	if before == "" || after == "" {
		return false, errors.New("the run recorded no snapshot, so the claim cannot be checked")
	}
	return before != after, nil
}

func localFilesUnchanged(ctx context.Context) error {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return err
	}
	last, err := current.lastRun()
	if err != nil {
		return err
	}
	changed, err := snapshotChanged(last.LocalBefore, last.LocalAfter)
	if err != nil {
		return err
	}
	if changed {
		return errors.New("the last run changed a repository file")
	}
	return nil
}

func vaultUnchanged(ctx context.Context) error {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return err
	}
	last, err := current.lastRun()
	if err != nil {
		return err
	}
	changed, err := snapshotChanged(last.VaultBefore, last.VaultAfter)
	if err != nil {
		return err
	}
	if changed {
		return errors.New("the last run changed the vault")
	}
	return nil
}

// items returns the vault's view of every item under name, from the driver's
// provider state.
func items(ctx context.Context, name string) ([]ItemView, error) {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return nil, err
	}
	return current.driver.Items(ctx, name)
}

// singleLogin returns the one login item under name, or an error naming how
// many items hold the name.
func singleLogin(ctx context.Context, name string) (ItemView, error) {
	held, err := items(ctx, name)
	if err != nil {
		return ItemView{}, err
	}
	if len(held) != 1 || held[0].Kind != "login" {
		return ItemView{}, fmt.Errorf("the vault holds %d items under the name %q, want one login item", len(held), name)
	}
	return held[0], nil
}

func vaultHoldsLogin(ctx context.Context, name, password string) error {
	item, err := singleLogin(ctx, name)
	if err != nil {
		return err
	}
	if item.Password != password {
		return fmt.Errorf("the login item %q holds a different password than the scenario expects", name)
	}
	return nil
}

func loginKeepsHistory(ctx context.Context, name, password string) error {
	item, err := singleLogin(ctx, name)
	if err != nil {
		return err
	}
	for _, retired := range item.History {
		if retired == password {
			return nil
		}
	}
	return fmt.Errorf("the login item %q does not keep the expected retired password in its history", name)
}

// singleNote returns the one secure note under name.
func singleNote(ctx context.Context, name string) (ItemView, error) {
	held, err := items(ctx, name)
	if err != nil {
		return ItemView{}, err
	}
	if len(held) != 1 || held[0].Kind != "note" {
		return ItemView{}, fmt.Errorf("the vault holds %d items under the name %q, want one secure note", len(held), name)
	}
	return held[0], nil
}

func noteHoldsAttachment(ctx context.Context, name, file string) error {
	note, err := singleNote(ctx, name)
	if err != nil {
		return err
	}
	if _, ok := note.Attachments[file]; !ok {
		return fmt.Errorf("the secure note %q does not hold the attachment %q", name, file)
	}
	return nil
}

func noteHoldsContent(ctx context.Context, name, file, content string) error {
	note, err := singleNote(ctx, name)
	if err != nil {
		return err
	}
	held, ok := note.Attachments[file]
	if !ok {
		return fmt.Errorf("the secure note %q does not hold the attachment %q", name, file)
	}
	if held != content {
		return fmt.Errorf("the attachment %q holds different content than the scenario expects", file)
	}
	return nil
}

func noteLacksAttachment(ctx context.Context, name, file string) error {
	note, err := singleNote(ctx, name)
	if err != nil {
		return err
	}
	if _, ok := note.Attachments[file]; ok {
		return fmt.Errorf("the secure note %q holds the attachment %q, want none", name, file)
	}
	return nil
}
