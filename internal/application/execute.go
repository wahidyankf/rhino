// Package application also owns execution dispatch.
package application

import (
	"fmt"
	"runtime"
	"sort"
	"strings"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// Version identity, stamped at release time via ldflags.
var (
	Version = "v0.0.0-dev"
	Commit  = "0000000000000000000000000000000000000000"
)

// ReadError is why one file could not be read, declared at this consumer.
// Kind is one of missing, unreadable, not-text, or escapes.
type ReadError struct {
	Kind   string
	Reason string
}

func (e *ReadError) Error() string { return e.Reason }

// Tree is the read-only repository this boundary needs. Disk and memory
// trees both satisfy it implicitly. Existence and link awareness live here,
// at the consuming layer, so a validator that resolves targets never reaches
// past this port into a filesystem adapter.
type Tree interface {
	Root() string
	Files() []string
	Read(path string) (string, *ReadError)
	RootedAt(path string) (Tree, error)
	// Children lists the direct children of a directory as repository-relative
	// paths: the files directly inside it and the subdirectories holding a
	// file at any depth. A subdirectory holding no file is not a child.
	Children(directory string) []string
	// IsDirectory reports whether path names a directory the tree holds, in
	// the file-list sense above: a directory holding no file answers false.
	IsDirectory(path string) bool
	// Exists reports whether path names a file or a directory the tree holds.
	Exists(path string) bool
	// PassesThroughLink reports whether reaching path crosses a filesystem
	// link at any component, the last included. A path behind a link is
	// outside what the repository holds, never missing from it.
	PassesThroughLink(path string) bool
	// Links lists every filesystem link the tree holds, with where its target
	// lies. A surface glob reads this list to decide what it reaches; nothing
	// else follows a link.
	Links() []TreeLink
}

// The three places a link's target can lie.
const (
	LinkWithin     = "within"
	LinkOutside    = "outside"
	LinkUnresolved = "unresolved"
)

// TreeLink is one filesystem link. Target is repository-relative and set only
// when Kind is LinkWithin: an outside target is never named, and an unresolved
// one leads nowhere.
type TreeLink struct {
	Path   string
	Target string
	Kind   string
}

// GateLaunch is what one gate child is asked to do: the declared vector,
// the repository root it runs in, the surface that selected it, the update
// records a push surface hands its children, and the projected
// environment. It mirrors the oracle's Launch: surface, stdin, and
// environment travel with the vector instead of being inherited by
// accident.
type GateLaunch struct {
	Argv        []string
	Directory   string
	Surface     string
	Stdin       *string
	Environment map[string]string
}

// Launcher starts one declared gate child. The process entry point supplies
// the operating-system launcher; tests supply a recorder.
type Launcher interface {
	Launch(launch GateLaunch) Launched
}

// Launched is what one child run produced.
type Launched struct {
	Code          int
	NotFound      bool
	NotExecutable bool
	Refused       string
}

// Execute runs one invocation against a tree.
func Execute(tree Tree, args []string, stdin *string) Outcome {
	return ExecuteWithLauncher(tree, args, stdin, nil)
}

// ExecuteWithLauncher runs one invocation with an explicit child launcher.
func ExecuteWithLauncher(tree Tree, args []string, stdin *string, launcher Launcher) Outcome {
	return ExecuteWithStores(tree, args, stdin, launcher, nil)
}

// ExecuteWithStores runs one invocation with an explicit child launcher and
// environment write boundary. Validators and dry-run planners never reach the
// store; only init, backup, and restore hold it.
func ExecuteWithStores(tree Tree, args []string, stdin *string, launcher Launcher, store EnvironmentStore) Outcome {
	return ExecuteWithToolchains(tree, args, stdin, launcher, store, nil)
}

// ExecuteWithToolchains runs one invocation with an explicit child launcher,
// environment write boundary, and toolchain process boundary. Validators and
// dry-run planners never reach the runner; only toolchain validate and
// applied provision hold it.
func ExecuteWithToolchains(tree Tree, args []string, stdin *string, launcher Launcher, store EnvironmentStore, runner ToolchainRunner) Outcome {
	return ExecuteWithHarness(tree, args, stdin, launcher, store, runner, nil, nil)
}

// ExecuteWithHarness runs one invocation with an explicit child launcher,
// environment write boundary, toolchain process boundary, canonical-adapter
// write boundary, and model-resolution process boundary. Validators and
// dry-run planners never reach the adapter or model boundaries; only
// adapter generation holds them.
func ExecuteWithHarness(tree Tree, args []string, stdin *string, launcher Launcher, store EnvironmentStore, runner ToolchainRunner, adapters AdapterStore, models ModelResolver) Outcome {
	return ExecuteWithMutations(tree, args, stdin, launcher, store, runner, adapters, models, nil)
}

// ExecuteWithMutations runs one invocation with every process and write
// boundary the entry point supplies, including the mutation runner only gate
// dispatch reaches. A nil mutation runner fails every mutation closed.
func ExecuteWithMutations(tree Tree, args []string, stdin *string, launcher Launcher, store EnvironmentStore, runner ToolchainRunner, adapters AdapterStore, models ModelResolver, mutations MutationRunner) Outcome {
	if mutations == nil {
		mutations = closedMutations{}
	}
	parsed, refusal := Parse(args)
	if refusal != nil {
		return Refused(domain.Body(refusal.Format, refusal.Code, refusal.Message))
	}
	if parsed.Help != nil {
		return Clean(parsed.Help.Text)
	}
	invocation := parsed.Run

	current := tree
	if invocation.Root != nil {
		rerooted, err := tree.RootedAt(*invocation.Root)
		if err != nil {
			return RefuseWith(invocation.Format, domain.RepositoryUnusable, err.Error())
		}
		current = rerooted
	}

	if invocation.Category == "version" {
		return version(invocation.Format)
	}

	text, readErr := current.Read(config.Path)
	if readErr != nil {
		var reason string
		switch readErr.Kind {
		case "missing":
			reason = config.Path + ": no configuration file at line 1: RHINO holds no defaults, so every value it enforces has to be declared here"
		case "not-text":
			reason = config.Path + ": holds no text"
		default:
			reason = config.Path + ": cannot be read at line 1: " + readErr.Reason
		}
		return reportRefused(invocation.Category, invocation.Format, reason)
	}
	document, err := config.Parse(text)
	if err != nil {
		return reportRefused(invocation.Category, invocation.Format, err.Error())
	}

	switch invocation.Category {
	case "repo-config":
		if err := checkQualityGatesDeclaration(document); err != nil {
			return reportRefused(invocation.Category, invocation.Format, err.Error())
		}
		report := NewReport("repo-config", "configuration file")
		report.Inspected = 1
		return report.Render(invocation.Format)
	case "gate-list":
		return gateList(document, invocation.Format)
	case "gate-validate":
		return gateValidate(document, invocation.Format)
	case "gate":
		return gateRun(document, invocation, current, stdin, launcher, mutations)
	case "emoji":
		return emojiValidate(document, current, invocation.Format)
	case "license":
		return licenseValidate(document, current, invocation.Format)
	case "naming":
		return namingValidate(document, current, invocation.Format)
	case "frontmatter":
		return frontmatterValidate(document, current, invocation.Format)
	case "heading-hierarchy":
		return headingValidate(document, current, invocation.Format)
	case "internal-link":
		return internalLinkValidate(document, current, invocation.Format)
	case "metadata":
		return metadataValidate(document, current, invocation.Format)
	case "mermaid":
		return mermaidValidate(document, current, invocation.Files, stdin, invocation.Format)
	case "readme-index":
		return readmeValidate(document, current, invocation.Format)
	case "vendor":
		return vendorValidate(document, current, invocation.Format)
	case "layers":
		return layersValidate(document, current, invocation.Format)
	case "traceability":
		return traceabilityValidate(document, current, invocation.Format)
	case "quality-gates":
		return qualityGatesValidate(document, current, invocation.Format)
	case "word-budget":
		return wordBudgetValidate(document, current, invocation.Format)
	case "directory-map":
		return directoryMapValidate(document, current, invocation.Format, invocation.Directory)
	case "environment-backup":
		return environmentBackup(document, current, store, invocation.BackupDirectory, invocation.Format)
	case "environment-validate":
		return environmentValidate(document, current, invocation.Format)
	case "environment-init":
		return environmentInit(document, current, store, invocation.Apply, invocation.Format)
	case "environment-restore":
		return environmentRestore(document, current, store, invocation.BackupDirectory, invocation.Force, invocation.Format)
	case "toolchain-validate":
		return toolchainValidate(document, runner, invocation.Format)
	case "toolchain-provision":
		return provisionToolchains(document, runner, runtime.GOOS, invocation.Apply, invocation.Format)
	case "harness-adapters-validate":
		return harnessValidate(document, current, invocation.Format)
	case "harness-adapters-generate":
		return harnessGenerate(document, current, adapters, models, invocation.Format)
	default:
		// Parse takes every category from its leaf table, and each of those
		// is routed above, so an unrouted category is a broken invariant.
		panic("unrouted command category `" + invocation.Category + "`")
	}
}

// Surfaces is the closed lifecycle surface set.
var Surfaces = []string{"pre-commit", "commit-msg", "pre-push", "pull-request", "main", "scheduled", "manual"}

func gateList(document *config.Document, format Format) Outcome {
	if !document.HasGates() {
		report := NewReport("gate-list", "configuration file")
		report.Refusal = strptr("gates section is not declared, and RHINO holds no default for it")
		report.RefusalCode = domain.ConfigUndeclared
		return report.Render(format)
	}
	if format == JSON {
		var surfaces []string
		for _, surface := range Surfaces {
			ids := document.GateIDsOn(surface)
			quoted := make([]string, 0, len(ids))
			for _, id := range ids {
				quoted = append(quoted, domain.Quote(id))
			}
			surfaces = append(surfaces, fmt.Sprintf("{\"surface\":%s,\"gates\":[%s]}",
				domain.Quote(surface), strings.Join(quoted, ",")))
		}
		return Clean(fmt.Sprintf("{\"schemaVersion\":1,\"command\":[\"gate\",\"list\"],\"result\":\"clean\",\"surfaces\":[%s]}\n",
			strings.Join(surfaces, ",")))
	}
	var out strings.Builder
	for _, surface := range Surfaces {
		ids := document.GateIDsOn(surface)
		listed := "no gates"
		if len(ids) > 0 {
			listed = strings.Join(ids, ", ")
		}
		out.WriteString("[gate] " + surface + ": " + listed + "\n")
	}
	return Clean(out.String())
}

func gateValidate(document *config.Document, format Format) Outcome {
	if !document.HasGates() {
		report := NewReport("gate-validate", "configuration file")
		report.Refusal = strptr("gates section is not declared, and RHINO holds no default for it")
		report.RefusalCode = domain.ConfigUndeclared
		return report.Render(format)
	}
	if format == JSON {
		return Clean("{\"schemaVersion\":1,\"command\":[\"gate\",\"validate\"],\"result\":\"clean\"}\n")
	}
	return Clean("[gate] lifecycle configuration is valid\n")
}

func gateRun(document *config.Document, invocation *Invocation, tree Tree, stdin *string, launcher Launcher, mutations MutationRunner) Outcome {
	if !document.HasGates() {
		report := NewReport("gate", "configuration file")
		report.Refusal = strptr("gates section is not declared, and RHINO holds no default for it")
		report.RefusalCode = domain.ConfigUndeclared
		return report.Render(invocation.Format)
	}
	surface := ""
	if invocation.Surface != nil {
		surface = *invocation.Surface
	}
	known := false
	for _, candidate := range Surfaces {
		if candidate == surface {
			known = true
		}
	}
	if !known {
		code := domain.ArgsIncomplete
		if invocation.Surface != nil {
			code = domain.ArgsUnrecognized
		}
		return RefuseWith(invocation.Format, code,
			"`--surface` must name one of "+strings.Join(Surfaces, ", "))
	}
	selected := document.GatesOn(surface)
	var report strings.Builder
	var completed []string
	var changed []string
	for _, gate := range selected {
		// Inputs resolve before the command is read: a deleted push range
		// skips the gate, and anything unresolvable refuses before any
		// child starts, the way the oracle orders it.
		binds := map[string]config.GateBinding(nil)
		if membership, ok := gate.RunOn[surface]; ok {
			binds = membership.Bind
		}
		resolved, refusal, skipped := resolveGateInputs(gate, binds, invocation, tree, stdin)
		if skipped {
			continue
		}
		if refusal != nil {
			return gateRefused(invocation.Format, 2, refusal.code,
				"gate `"+gate.ID+"`: "+refusal.reason)
		}
		if gate.Executable == "" {
			return gateRefused(invocation.Format, 2, domain.GateUndeclared,
				"gate `"+gate.ID+"` declares no executable command")
		}
		argv, environment, reason := projectGateCommand(gate.Executable, gate.Args, gate.Environment, resolved)
		if reason != "" {
			return gateRefused(invocation.Format, 2, domain.ConfigUnusable,
				"gate `"+gate.ID+"`: "+reason)
		}
		if gate.Kind == "mutation" {
			// A mutation never reaches the ordinary launcher: the mutation
			// boundary owns its snapshot and every write it makes.
			selectedPaths := selectedGatePaths(binds, resolved)
			if hasRegularFilesInput(gate) && len(selectedPaths) == 0 {
				report.WriteString("[gate] " + gate.ID + " passed\n")
				completed = append(completed, gate.ID)
				continue
			}
			mutation := MutationLaunch{
				Argv:          argv,
				Directory:     tree.Root(),
				Environment:   environment,
				SelectedPaths: selectedPaths,
				Surface:       surface,
			}
			if invocation.Head != nil {
				mutation.Revision = *invocation.Head
			}
			var mutated MutationOutcome
			var mutateErr error
			switch surface {
			case "pre-commit":
				mutated, mutateErr = mutations.ApplyIndex(mutation)
			case "pull-request":
				mutated, mutateErr = mutations.VerifyClean(mutation)
			default:
				mutateErr = fmt.Errorf("a grouped mutation may run only at pre-commit or pull-request")
			}
			if mutateErr != nil {
				report.WriteString("[gate] " + gate.ID + " could not run\n")
				return gateRefusal(invocation.Format, 2, domain.GateChildRefused, &report,
					"gate `"+gate.ID+"`: "+mutateErr.Error())
			}
			for _, path := range mutated.Changes {
				report.WriteString("[gate] " + gate.ID + " changed " + path + "\n")
			}
			for _, path := range mutated.Divergences {
				report.WriteString("[gate] " + gate.ID + " preserved unstaged " + path + "\n")
			}
			if mutated.Code != 0 {
				report.WriteString("[gate] " + gate.ID + " failed\n")
				return gateOutcome(invocation.Format, 1, surface, completed, mutated.Changes, report.String(),
					"[gate] "+gate.ID+" reported a finding at "+surface+"\n")
			}
			changed = append(changed, mutated.Changes...)
			report.WriteString("[gate] " + gate.ID + " passed\n")
			completed = append(completed, gate.ID)
			continue
		}
		if launcher == nil {
			return gateRefusal(invocation.Format, 2, domain.GateChildRefused, &report,
				"gate `"+gate.ID+"`: process dispatch is not wired in this invocation")
		}
		launched := launcher.Launch(GateLaunch{
			Argv:        argv,
			Directory:   tree.Root(),
			Surface:     surface,
			Stdin:       stdin,
			Environment: environment,
		})
		switch {
		case launched.NotFound:
			report.WriteString("[gate] " + gate.ID + " could not run\n")
			return gateRefusal(invocation.Format, 127, domain.GateChildNotFound, &report,
				"gate `"+gate.ID+"`: executable was not found")
		case launched.NotExecutable:
			report.WriteString("[gate] " + gate.ID + " could not run\n")
			return gateRefusal(invocation.Format, 126, domain.GateChildNotExecutable, &report,
				"gate `"+gate.ID+"`: executable could not be executed")
		case launched.Refused != "":
			report.WriteString("[gate] " + gate.ID + " could not run\n")
			return gateRefusal(invocation.Format, 2, domain.GateChildRefused, &report,
				"gate `"+gate.ID+"`: "+launched.Refused)
		case launched.Code != 0:
			report.WriteString("[gate] " + gate.ID + " failed\n")
			return gateOutcome(invocation.Format, 1, surface, completed, nil, report.String(),
				"[gate] "+gate.ID+" reported a finding at "+surface+"\n")
		default:
			report.WriteString("[gate] " + gate.ID + " passed\n")
			completed = append(completed, gate.ID)
		}
	}
	if invocation.Format == JSON {
		return Outcome{ExitCode: 0, Stdout: gateRunDocument("clean", 0, surface, completed, changed)}
	}
	return Clean(report.String())
}

// gateRunDocument renders the `gate run` document the JSON reference
// describes: the result, the summary counts (gates passed, paths changed, and
// findings), the changed paths, and the gate IDs that passed. The surface and
// completed keys predate the documented shape and stay for compatibility, so
// the document is a superset of the earlier one rather than a replacement.
func gateRunDocument(result string, code int, surface string, completed, changes []string) string {
	findings := 0
	if code == 1 {
		findings = 1
	}
	return fmt.Sprintf(
		"{\"changes\":%s,\"command\":[\"gate\",\"run\"],\"completed\":%s,\"findings\":[],\"metadata\":{\"gates\":%s},\"result\":%s,\"schemaVersion\":1,\"summary\":{\"changed\":%d,\"findings\":%d,\"scanned\":%d},\"surface\":%s}\n",
		quotedArray(changes), quotedArray(completed), quotedArray(completed), domain.Quote(result),
		len(changes), findings, len(completed), domain.Quote(surface))
}

// selectedGatePaths is the sorted, de-duplicated union of the file paths a
// mutation's Git-index and explicit-range bindings selected. Nothing else
// widens the write set a mutator may touch.
func selectedGatePaths(binds map[string]config.GateBinding, resolved map[string]gateResolvedInput) []string {
	union := map[string]bool{}
	for name, binding := range binds {
		if binding.Source != "git-index" && binding.Source != "explicit-range" {
			continue
		}
		input, ok := resolved[name]
		if !ok || input.kind != "files" {
			continue
		}
		for _, path := range input.files {
			union[path] = true
		}
	}
	paths := make([]string, 0, len(union))
	for path := range union {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// hasRegularFilesInput reports whether a gate declares a regular-files input,
// the one shape whose empty selection passes without starting its mutator.
func hasRegularFilesInput(gate config.Gate) bool {
	for _, input := range gate.Inputs {
		if input.Kind == "regular-files" {
			return true
		}
	}
	return false
}

// quotedArray renders a JSON array of strings, empty as `[]`.
func quotedArray(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, domain.Quote(value))
	}
	return "[" + strings.Join(quoted, ",") + "]"
}

func gateOutcome(format Format, code int, surface string, completed, changes []string, report, stderr string) Outcome {
	if format == JSON {
		return Outcome{ExitCode: code, Stdout: gateRunDocument("findings", code, surface, completed, changes),
			Stderr: strings.TrimRight(stderr, "\n") + "\n"}
	}
	return Outcome{ExitCode: code, Stdout: report, Stderr: stderr}
}

// gateRefused renders a refusal that never reached a verdict: stdout stays
// empty, so a caller parsing the result never mistakes progress for a
// verdict. Used for resolve, command, and projection refusals, which drop
// the accumulated progress the way the oracle does.
func gateRefused(format Format, status int, code domain.ErrorCode, message string) Outcome {
	return Outcome{ExitCode: status, Stderr: domain.Body(format, code, message)}
}

// gateRefusal renders a run that reached a child but never reached a
// verdict. In text mode the completed progress travels on stderr ahead of
// the diagnostic, so stdout still carries no verdict; in JSON mode a caller
// parsing stderr wants one document, not prose before it.
func gateRefusal(format Format, status int, code domain.ErrorCode, progress *strings.Builder, message string) Outcome {
	if format == JSON {
		return Outcome{ExitCode: status, Stderr: domain.Body(format, code, message)}
	}
	return Outcome{ExitCode: status, Stderr: progress.String() + domain.Body(format, code, message)}
}

func version(format Format) Outcome {
	if format == JSON {
		return Clean(fmt.Sprintf("{\"schemaVersion\":1,\"version\":%s,\"commit\":%s}\n",
			domain.Quote(Version), domain.Quote(Commit)))
	}
	return Clean(Version + "\n")
}

func reportRefused(category string, format Format, reason string) Outcome {
	report := Refuse(category, reason)
	return report.Render(format)
}

func strptr(value string) *string { return &value }
