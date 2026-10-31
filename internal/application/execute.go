// Package application also owns execution dispatch.
package application

import (
	"fmt"
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
}

// GateLaunch is what one gate child is asked to do: the declared vector,
// the repository root it runs in, the surface that selected it, and the
// projected environment. It mirrors the oracle's Launch: surface and
// environment travel with the vector instead of being inherited by
// accident.
type GateLaunch struct {
	Argv        []string
	Directory   string
	Surface     string
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
		report := NewReport("repo-config", "configuration file")
		report.Inspected = 1
		return report.Render(invocation.Format)
	case "gate-list":
		return gateList(document, invocation.Format)
	case "gate-validate":
		return gateValidate(document, invocation.Format)
	case "gate":
		return gateRun(document, invocation, current, stdin, launcher)
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
		return provisionToolchains(document, runner, invocation.Apply, invocation.Format)
	case "harness-adapters-validate":
		return harnessValidate(document, current, invocation.Format)
	case "harness-adapters-generate":
		return harnessGenerate(document, current, adapters, models, invocation.Format)
	default:
		owner, ok := categoryOwner[invocation.Category]
		if !ok {
			return reportRefused(invocation.Category, invocation.Format,
				"the grouped v0.4 configuration declares no command in this stable release")
		}
		if !document.PolicyOwner(owner) {
			report := NewReport(invocation.Category, "configuration file")
			report.Refusal = strptr(owner + " is not declared, and RHINO holds no default for it")
			report.RefusalCode = domain.ConfigUndeclared
			return report.Render(invocation.Format)
		}
		return notYetPorted(invocation.Category, invocation.Format)
	}
}

// categoryOwner maps validator categories to their stable policy owners.
var categoryOwner = map[string]string{
	"frontmatter":               "policies.markdown.frontmatter",
	"heading-hierarchy":         "policies.markdown.heading-hierarchy",
	"internal-link":             "policies.markdown.internal-link",
	"metadata":                  "policies.markdown.metadata",
	"mermaid":                   "policies.markdown.mermaid",
	"naming":                    "policies.markdown.naming",
	"readme-index":              "policies.markdown.readme-index",
	"license":                   "policies.conventions.license",
	"emoji":                     "policies.conventions.emoji",
	"vendor":                    "policies.governance.vendor",
	"layers":                    "policies.governance.layers",
	"traceability":              "policies.governance.traceability",
	"quality-gates":             "policies.governance.quality-gates",
	"word-budget":               "policies.governance.word-budget",
	"directory-map":             "policies.governance.directory-map",
	"environment-backup":        "policies.environment.backup",
	"environment-validate":      "policies.environment.validate",
	"environment-init":          "policies.environment.init",
	"environment-restore":       "policies.environment.restore",
	"toolchain-validate":        "policies.toolchains.validate",
	"toolchain-provision":       "policies.toolchains.provision",
	"harness-adapters-validate": "policies.harness.adapters",
	"harness-adapters-generate": "policies.harness.adapters",
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

func gateRun(document *config.Document, invocation *Invocation, tree Tree, stdin *string, launcher Launcher) Outcome {
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
	for _, gate := range selected {
		if gate.Executable == "" {
			return gateRefusal(invocation.Format, 2, domain.GateUndeclared, &report,
				"gate `"+gate.ID+"` declares no executable command")
		}
		argv, err := gate.Argv()
		if err != nil {
			return gateRefusal(invocation.Format, 2, domain.ConfigUnusable, &report,
				"gate `"+gate.ID+"`: "+err.Error())
		}
		environment, err := gate.Environ()
		if err != nil {
			return gateRefusal(invocation.Format, 2, domain.ConfigUnusable, &report,
				"gate `"+gate.ID+"`: "+err.Error())
		}
		if launcher == nil {
			return gateRefusal(invocation.Format, 2, domain.GateChildRefused, &report,
				"gate `"+gate.ID+"`: process dispatch is not wired in this invocation")
		}
		launched := launcher.Launch(GateLaunch{
			Argv:        argv,
			Directory:   tree.Root(),
			Surface:     surface,
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
			return gateOutcome(invocation.Format, 1, completed, report.String(),
				"[gate] "+gate.ID+" reported a finding at "+surface+"\n")
		default:
			report.WriteString("[gate] " + gate.ID + " passed\n")
			completed = append(completed, gate.ID)
		}
	}
	// Gate children run with no standard input in this port: the launcher
	// passes the vector, the working directory, the selecting surface, and
	// the projected environment, so stdin stays with the document
	// validators that read it. Only stdin forwarding is out of scope.
	_ = stdin
	if invocation.Format == JSON {
		quoted := make([]string, 0, len(completed))
		for _, id := range completed {
			quoted = append(quoted, domain.Quote(id))
		}
		return Outcome{ExitCode: 0, Stdout: fmt.Sprintf(
			"{\"schemaVersion\":1,\"command\":[\"gate\",\"run\"],\"result\":\"clean\",\"surface\":%s,\"completed\":[%s]}\n",
			domain.Quote(surface), strings.Join(quoted, ","))}
	}
	return Clean(report.String())
}

func gateOutcome(format Format, code int, completed []string, report, stderr string) Outcome {
	if format == JSON {
		quoted := make([]string, 0, len(completed))
		for _, id := range completed {
			quoted = append(quoted, domain.Quote(id))
		}
		return Outcome{ExitCode: code, Stdout: fmt.Sprintf(
			"{\"schemaVersion\":1,\"command\":[\"gate\",\"run\"],\"result\":\"findings\",\"completed\":[%s]}\n",
			strings.Join(quoted, ",")),
			Stderr: "[gate] " + strings.TrimRight(stderr, "\n") + "\n"}
	}
	return Outcome{ExitCode: code, Stdout: report, Stderr: stderr}
}

func gateRefusal(format Format, status int, code domain.ErrorCode, report *strings.Builder, message string) Outcome {
	if format == JSON {
		return Outcome{ExitCode: status, Stderr: fmt.Sprintf(
			"{\"schemaVersion\":1,\"error\":{\"code\":%s,\"message\":%s}}\n",
			domain.Quote(code.String()), domain.Quote(message))}
	}
	return Outcome{ExitCode: status, Stdout: report.String(), Stderr: "rhino: " + message + "\n"}
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

func notYetPorted(category string, format Format) Outcome {
	report := Refuse(category, "the "+category+" validator core is not yet ported in this Go build")
	report.RefusalCode = domain.ConfigUnusable
	return report.Render(format)
}

func strptr(value string) *string { return &value }
