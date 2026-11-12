// Package vault is the shared step registry for the environment vault
// validation corpus, specs/behaviours/environment-vault-validation.feature.
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
	// Close removes the scenario's fixture.
	Close() error
}

// Result is one command run: its exit code, both of its streams, and every
// process the run started, in order.
type Result struct {
	ExitCode  int
	Stdout    string
	Stderr    string
	Processes []Process
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
		{`^no run output contains "([^"]*)"$`, noRunOutputContains},
		{`^the run started only read-only Git queries$`, startedOnlyReadOnlyQueries},
	}
}

func declareConfiguration(ctx context.Context, doc *godog.DocString) error {
	current, err := scenarioFrom(ctx)
	if err != nil {
		return err
	}
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
