// Package application owns the invocation contract: parsing, dispatch, and outcomes.
package application

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wahidyankf/rhino/internal/domain"
)

// Format re-exports the domain format at the consumer boundary.
type Format = domain.Format

const (
	Text = domain.Text
	JSON = domain.JSON
)

// Accepts names one flag a leaf may take.
type Accepts int

const (
	AcceptFile Accepts = iota
	AcceptDirectory
	AcceptBackupDirectory
	AcceptApply
	AcceptForce
	AcceptSurface
	AcceptMessageFile
	AcceptPushUpdatesStdin
	AcceptRange
	AcceptJSON
)

// Leaf is one command path plus its output category and flags.
type Leaf struct {
	Path     []string
	Category string
	Accepts  []Accepts
	Summary  string
}

// Leaves is the closed command table. Adding is free; moving is not.
var Leaves = []Leaf{
	{Path: []string{"repo-config", "validate"}, Category: "repo-config", Accepts: []Accepts{}, Summary: "Check that repo-config.yml is complete and usable."},
	{Path: []string{"gate", "run"}, Category: "gate", Accepts: []Accepts{AcceptSurface, AcceptMessageFile, AcceptPushUpdatesStdin, AcceptRange}, Summary: "Run the gates the declared surface selects, in declaration order."},
	{Path: []string{"gate", "list"}, Category: "gate-list", Accepts: []Accepts{}, Summary: "List the declared v0.4 lifecycle gates for each closed surface."},
	{Path: []string{"gate", "validate"}, Category: "gate-validate", Accepts: []Accepts{}, Summary: "Validate v0.4 lifecycle membership and pull-request composition."},
	{Path: []string{"governance", "vendor", "validate"}, Category: "vendor", Accepts: []Accepts{}, Summary: "Check declared roots for declared vendor terms and exact exceptions."},
	{Path: []string{"governance", "layers", "validate"}, Category: "layers", Accepts: []Accepts{}, Summary: "Check the declared governance layer and category structure."},
	{Path: []string{"governance", "traceability", "validate"}, Category: "traceability", Accepts: []Accepts{}, Summary: "Check declared artifacts and their declared local links."},
	{Path: []string{"governance", "quality-gates", "validate"}, Category: "quality-gates", Accepts: []Accepts{}, Summary: "Check the declared quality-gate layout, headings, verdicts, and cycle ceiling."},
	{Path: []string{"governance", "word-budget", "validate"}, Category: "word-budget", Accepts: []Accepts{}, Summary: "Check every declared surface against its declared word budget."},
	{Path: []string{"governance", "directory-map", "validate"}, Category: "directory-map", Accepts: []Accepts{AcceptDirectory}, Summary: "Check that every mapped tree's READMEs list their siblings."},
	{Path: []string{"harness", "adapters", "validate"}, Category: "harness-adapters-validate", Accepts: []Accepts{}, Summary: "Validate canonical harness adapters without writing them."},
	{Path: []string{"harness", "adapters", "generate"}, Category: "harness-adapters-generate", Accepts: []Accepts{}, Summary: "Generate canonical harness adapters in one declared transaction."},
	{Path: []string{"env", "backup"}, Category: "environment-backup", Accepts: []Accepts{AcceptBackupDirectory}, Summary: "Copy declared environment targets into an explicit backup destination."},
	{Path: []string{"env", "validate"}, Category: "environment-validate", Accepts: []Accepts{}, Summary: "Validate declared environment policy without reporting values."},
	{Path: []string{"env", "init"}, Category: "environment-init", Accepts: []Accepts{AcceptApply}, Summary: "Initialize declared environment targets only with explicit authorization."},
	{Path: []string{"env", "restore"}, Category: "environment-restore", Accepts: []Accepts{AcceptBackupDirectory, AcceptForce}, Summary: "Restore declared environment targets from an explicit backup directory."},
	{Path: []string{"toolchain", "validate"}, Category: "toolchain-validate", Accepts: []Accepts{}, Summary: "Probe declared toolchains without installing or reporting their output."},
	{Path: []string{"toolchain", "provision"}, Category: "toolchain-provision", Accepts: []Accepts{AcceptApply}, Summary: "Provision declared toolchains only with explicit authorization."},
	{Path: []string{"md", "frontmatter", "validate"}, Category: "frontmatter", Accepts: []Accepts{}, Summary: "Check every declared surface's front matter against its declared schema."},
	{Path: []string{"md", "heading-hierarchy", "validate"}, Category: "heading-hierarchy", Accepts: []Accepts{}, Summary: "Check every declared surface's heading structure."},
	{Path: []string{"metadata", "validate"}, Category: "metadata", Accepts: []Accepts{}, Summary: "Check every declared surface against its path-selected metadata schema."},
	{Path: []string{"md", "internal-link", "validate"}, Category: "internal-link", Accepts: []Accepts{}, Summary: "Check that every local Markdown link resolves inside the repository."},
	{Path: []string{"md", "mermaid", "validate"}, Category: "mermaid", Accepts: []Accepts{AcceptFile}, Summary: "Check Mermaid diagrams for label length and colour contrast."},
	{Path: []string{"md", "naming", "validate"}, Category: "naming", Accepts: []Accepts{}, Summary: "Check every declared surface's filenames against its declared style."},
	{Path: []string{"md", "readme-index", "validate"}, Category: "readme-index", Accepts: []Accepts{}, Summary: "Check that every directory in a declared tree carries a README."},
	{Path: []string{"convention", "emoji", "validate"}, Category: "emoji", Accepts: []Accepts{}, Summary: "Check that no declared file carries an emoji code point."},
	{Path: []string{"convention", "license", "validate"}, Category: "license", Accepts: []Accepts{}, Summary: "Check configured license paths, identifiers, and digests."},
	{Path: []string{"version"}, Category: "version", Accepts: []Accepts{AcceptJSON}, Summary: "Report this build's release identity."},
}

// Invocation is one runnable request.
type Invocation struct {
	Category         string
	Root             *string
	Format           Format
	Files            []string
	Directory        *string
	BackupDirectory  *string
	Surface          *string
	MessageFile      *string
	PushUpdatesStdin bool
	Base             *string
	Head             *string
	Apply            bool
	Force            bool
}

// Help is a usage answer.
type Help struct{ Text string }

// Parsed is either a run or help.
type Parsed struct {
	Run  *Invocation
	Help *Help
}

// Refusal is why an invocation cannot run.
type Refusal struct {
	Code    domain.ErrorCode
	Message string
	Format  Format
}

func (r *Refusal) Error() string { return "rhino: " + r.Message }

func partialFormat(output *string, jsonFlag bool) Format {
	if jsonFlag || (output != nil && *output == "json") {
		return JSON
	}
	return Text
}

// Parse implements the command line. Flags may appear before or after the
// command path; an unknown flag, command, or flag-for-leaf refuses with 2.
func Parse(args []string) (Parsed, *Refusal) {
	var path []string
	var wantsHelp, wantsVersion bool
	var root, format, directory, backupDir, surface, messageFile, base, head *string
	var files []string
	var pushStdin, apply, force, jsonFlag bool

	set := func(name string, held **string, rest []string, i *int, seen Format) *Refusal {
		*i++
		if *i >= len(rest) {
			return &Refusal{Code: domain.ArgsIncomplete, Message: "`" + name + "` needs a value", Format: seen}
		}
		value := rest[*i]
		*held = &value
		return nil
	}

	for i := 0; i < len(args); i++ {
		seen := partialFormat(format, jsonFlag)
		arg := args[i]
		switch arg {
		case "--help", "-h":
			wantsHelp = true
		case "--version", "-V":
			wantsVersion = true
		case "--root":
			if refusal := set("--root", &root, args, &i, seen); refusal != nil {
				return Parsed{}, refusal
			}
		case "--output":
			if refusal := set("--output", &format, args, &i, seen); refusal != nil {
				return Parsed{}, refusal
			}
		case "--directory":
			if refusal := set("--directory", &directory, args, &i, seen); refusal != nil {
				return Parsed{}, refusal
			}
		case "--dir":
			if refusal := set("--dir", &backupDir, args, &i, seen); refusal != nil {
				return Parsed{}, refusal
			}
		case "--surface":
			if refusal := set("--surface", &surface, args, &i, seen); refusal != nil {
				return Parsed{}, refusal
			}
		case "--message-file":
			if refusal := set("--message-file", &messageFile, args, &i, seen); refusal != nil {
				return Parsed{}, refusal
			}
		case "--push-updates-stdin":
			pushStdin = true
		case "--base":
			if refusal := set("--base", &base, args, &i, seen); refusal != nil {
				return Parsed{}, refusal
			}
		case "--head":
			if refusal := set("--head", &head, args, &i, seen); refusal != nil {
				return Parsed{}, refusal
			}
		case "--apply":
			apply = true
		case "--force":
			force = true
		case "--":
			path = append(path, args[i+1:]...)
			i = len(args)
		case "--file":
			var held *string
			if refusal := set("--file", &held, args, &i, seen); refusal != nil {
				return Parsed{}, refusal
			}
			files = append(files, *held)
		case "--json":
			jsonFlag = true
		case "--quiet", "--verbose", "--no-color":
		default:
			if strings.HasPrefix(arg, "-") {
				seenNow := partialFormat(format, jsonFlag)
				return Parsed{}, &Refusal{Code: domain.ArgsUnrecognized, Message: "unrecognized option `" + arg + "`", Format: seenNow}
			}
			path = append(path, arg)
		}
	}

	if len(path) > 0 && path[0] == "help" {
		path = path[1:]
		wantsHelp = true
	}
	if wantsVersion && !wantsHelp {
		path = []string{"version"}
	}

	if wantsHelp {
		text, ok := HelpFor(path)
		if !ok {
			seen := partialFormat(format, jsonFlag)
			return Parsed{}, &Refusal{Code: domain.ArgsUnrecognized, Message: unrecognized(path), Format: seen}
		}
		return Parsed{Help: &Help{Text: text}}, nil
	}

	var leaf *Leaf
	for i := range Leaves {
		if equalPath(Leaves[i].Path, path) {
			leaf = &Leaves[i]
			break
		}
	}
	if leaf == nil {
		seen := partialFormat(format, jsonFlag)
		return Parsed{}, &Refusal{Code: domain.ArgsUnrecognized, Message: unrecognized(path), Format: seen}
	}

	formatGiven := format != nil
	resolved := Text
	if format != nil {
		switch *format {
		case "text":
			resolved = Text
		case "json":
			resolved = JSON
		default:
			seen := partialFormat(nil, jsonFlag)
			return Parsed{}, &Refusal{Code: domain.ArgsUnrecognized,
				Message: "`--output " + *format + "` is not a format this build writes; expected `text` or `json`",
				Format:  seen}
		}
	}

	accepted := map[Accepts]bool{}
	for _, flag := range leaf.Accepts {
		accepted[flag] = true
	}
	checks := []struct {
		given bool
		flag  Accepts
		name  string
	}{
		{len(files) > 0, AcceptFile, "--file"},
		{directory != nil, AcceptDirectory, "--directory"},
		{backupDir != nil, AcceptBackupDirectory, "--dir"},
		{surface != nil, AcceptSurface, "--surface"},
		{messageFile != nil, AcceptMessageFile, "--message-file"},
		{pushStdin, AcceptPushUpdatesStdin, "--push-updates-stdin"},
		{base != nil || head != nil, AcceptRange, "--base/--head"},
		{jsonFlag, AcceptJSON, "--json"},
		{apply, AcceptApply, "--apply"},
		{force, AcceptForce, "--force"},
	}
	for _, check := range checks {
		if check.given && !accepted[check.flag] {
			return Parsed{}, &Refusal{Code: domain.ArgsUnrecognized,
				Message: "`" + check.name + "` is not accepted by `" + strings.Join(leaf.Path, " ") + "`",
				Format:  resolved}
		}
	}

	if directory != nil {
		if reason := notRepositoryRelative(*directory); reason != "" {
			return Parsed{}, &Refusal{Code: domain.PathEscapesRoot,
				Message: "`--directory " + *directory + "` " + reason, Format: resolved}
		}
	}
	for _, selected := range files {
		if selected == "-" {
			continue
		}
		if reason := notRepositoryRelative(selected); reason != "" {
			return Parsed{}, &Refusal{Code: domain.PathEscapesRoot,
				Message: "`--file " + selected + "` " + reason, Format: resolved}
		}
	}
	if jsonFlag && formatGiven && resolved == Text {
		return Parsed{}, &Refusal{Code: domain.ArgsIncomplete,
			Message: "`--json` and `--output text` ask for different things", Format: resolved}
	}
	if (base != nil) != (head != nil) {
		return Parsed{}, &Refusal{Code: domain.ArgsIncomplete,
			Message: "`--base` and `--head` must be supplied together", Format: resolved}
	}
	if leaf.Category == "gate" {
		selected := ""
		if surface != nil {
			selected = *surface
		}
		if messageFile != nil && selected != "commit-msg" {
			return Parsed{}, &Refusal{Code: domain.ArgsIncomplete,
				Message: "`--message-file` is accepted only with `--surface commit-msg`", Format: resolved}
		}
		if pushStdin && selected != "pre-push" {
			return Parsed{}, &Refusal{Code: domain.ArgsIncomplete,
				Message: "`--push-updates-stdin` is accepted only with `--surface pre-push`", Format: resolved}
		}
		if base != nil && selected != "pull-request" {
			return Parsed{}, &Refusal{Code: domain.ArgsIncomplete,
				Message: "`--base` and `--head` are accepted only with `--surface pull-request`", Format: resolved}
		}
	}

	effective := resolved
	if jsonFlag {
		effective = JSON
	}
	return Parsed{Run: &Invocation{
		Category: leaf.Category, Root: root, Format: effective, Files: files,
		Directory: directory, BackupDirectory: backupDir, Surface: surface,
		MessageFile: messageFile, PushUpdatesStdin: pushStdin,
		Base: base, Head: head, Apply: apply, Force: force,
	}}, nil
}

func equalPath(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func unrecognized(segments []string) string {
	if len(segments) == 0 {
		return "no command given; try `rhino --help`"
	}
	return "unrecognized command `" + strings.Join(segments, " ") + "`"
}

// HelpFor builds usage for a path prefix.
func HelpFor(segments []string) (string, bool) {
	var matching []*Leaf
	for i := range Leaves {
		if hasPrefix(Leaves[i].Path, segments) {
			matching = append(matching, &Leaves[i])
		}
	}
	if len(matching) == 0 {
		return "", false
	}
	var out strings.Builder
	if len(segments) == 0 {
		out.WriteString("rhino -- Repository Hygiene & INtegration Orchestrator\n\n")
		out.WriteString("Every value it enforces is declared in the repository's repo-config.yml.\n\n")
	} else {
		out.WriteString("rhino " + strings.Join(segments, " ") + "\n\n")
	}
	out.WriteString("Commands:\n")
	for _, leaf := range matching {
		out.WriteString(fmt.Sprintf("  rhino %-38s%s\n", strings.Join(leaf.Path, " "), leaf.Summary))
	}
	out.WriteString("\nOptions:\n")
	out.WriteString("  --root <path>          The repository to inspect. Defaults to the working directory.\n")
	out.WriteString("  --output <text|json>   How to render the result. Defaults to text.\n")
	out.WriteString("  -h, --help             Show this help and exit.\n")
	out.WriteString("  -V, --version          Show the release identity and exit.\n")
	out.WriteString("  --quiet, --verbose, --no-color\n")
	out.WriteString("                         Accepted everywhere; presentation only.\n")

	union := map[Accepts]bool{}
	for _, leaf := range matching {
		for _, flag := range leaf.Accepts {
			union[flag] = true
		}
	}
	order := []Accepts{AcceptFile, AcceptDirectory, AcceptBackupDirectory, AcceptApply, AcceptForce, AcceptSurface, AcceptMessageFile, AcceptPushUpdatesStdin, AcceptRange, AcceptJSON}
	texts := map[Accepts]string{
		AcceptFile:             "  --file <path>          Inspect these paths instead of the declared surface.\n                         Repeatable; `-` reads standard input.\n",
		AcceptDirectory:        "  --directory <path>     Inspect this directory instead of the declared trees.\n",
		AcceptBackupDirectory:  "  --dir <path>           Explicit repository-relative backup destination.\n",
		AcceptApply:            "  --apply                Authorize this declared mutation.\n",
		AcceptForce:            "  --force                Replace an existing target after backup planning.\n",
		AcceptSurface:          "  --surface <name>       Which closed lifecycle surface is dispatching.\n",
		AcceptMessageFile:      "  --message-file <path>  Exact commit-msg hook path, read only for that surface.\n",
		AcceptPushUpdatesStdin: "  --push-updates-stdin   Read Git pre-push update records from standard input.\n",
		AcceptRange:            "  --base <sha> --head <sha>\n                         Explicit immutable pull-request range.\n",
		AcceptJSON:             "  --json                 Shorthand for --output json.\n",
	}
	for _, flag := range order {
		if !union[flag] {
			continue
		}
		out.WriteString(texts[flag])
		out.WriteString(scopeNote(matching, flag))
	}
	out.WriteString("\nExit codes:\n  0    checked and clean\n  1    the repository violates its declared policy\n  2    the invocation, root, or configuration was unusable\n  126  a gate child was found and could not be executed\n  127  a gate child was not found\n  128+N  ended by signal N; 130 is an interrupt, 141 a closed pipe\n")
	return out.String(), true
}

func hasPrefix(path []string, prefix []string) bool {
	if len(prefix) > len(path) {
		return false
	}
	for i := range prefix {
		if path[i] != prefix[i] {
			return false
		}
	}
	return true
}

func scopeNote(matching []*Leaf, flag Accepts) string {
	var accepting []string
	for _, leaf := range matching {
		for _, candidate := range leaf.Accepts {
			if candidate == flag {
				accepting = append(accepting, "`"+strings.Join(leaf.Path, " ")+"`")
				break
			}
		}
	}
	if len(accepting) == len(matching) {
		return ""
	}
	sort.Strings(accepting)
	return "  (accepted by " + strings.Join(accepting, ", ") + " only)\n"
}

func notRepositoryRelative(value string) string {
	if strings.TrimSpace(value) == "" {
		return "names nothing"
	}
	if strings.HasPrefix(value, "/") {
		return "is absolute, and a selection is relative to the repository root"
	}
	depth := 0
	for _, segment := range strings.Split(value, "/") {
		switch segment {
		case "..":
			depth--
		case ".", "":
		default:
			depth++
		}
		if depth < 0 {
			return "escapes the repository root"
		}
	}
	return ""
}
