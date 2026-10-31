package application

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// gateInputTree is the narrow read boundary only gate input resolution
// holds: the Git index snapshot, the explicit range answers, the fallback
// ref resolution, the commit-message texts, and the hook message file. A
// tree with no Git capability answers the oracle's own default refusals
// rather than widening a boundary the caller never declared.
type gateInputTree interface {
	IndexedFiles() ([]string, error)
	IndexedRegularFiles() ([]string, error)
	ChangedFiles(base, head string) ([]string, error)
	ChangedRegularFiles(base, head string) ([]string, error)
	ResolveGitRef(reference string) (string, error)
	CommitMessages(base, head string) (string, error)
	HookMessageFile(path string) (string, *ReadError)
}

// gateInputsOf answers the boundary when the tree carries it, and the
// oracle's default refusals when it does not.
func gateInputsOf(tree Tree) gateInputTree {
	if inputs, ok := tree.(gateInputTree); ok {
		return inputs
	}
	return noGitTree{}
}

// noGitTree answers every Git-bound question the way the oracle's default
// trait methods do: the tree holds no such boundary.
type noGitTree struct{}

func (noGitTree) IndexedFiles() ([]string, error) {
	return nil, fmt.Errorf("this tree has no Git index snapshot boundary")
}

func (noGitTree) IndexedRegularFiles() ([]string, error) {
	return nil, fmt.Errorf("this tree has no regular-file Git index snapshot boundary")
}

func (noGitTree) ChangedFiles(_, _ string) ([]string, error) {
	return nil, fmt.Errorf("this tree has no changed-file range boundary")
}

func (noGitTree) ChangedRegularFiles(_, _ string) ([]string, error) {
	return nil, fmt.Errorf("this tree has no regular-file changed-file range boundary")
}

func (noGitTree) ResolveGitRef(string) (string, error) {
	return "", fmt.Errorf("this tree has no Git ref-resolution boundary")
}

func (noGitTree) CommitMessages(_, _ string) (string, error) {
	return "", fmt.Errorf("this tree has no commit-message range boundary")
}

func (noGitTree) HookMessageFile(_ string) (string, *ReadError) {
	return "", &ReadError{Kind: "unreadable", Reason: "the Git hook message-file boundary is unavailable"}
}

// gateResolvedInput is one resolved typed input: a file list, a message, a
// commit range, or the repository state.
type gateResolvedInput struct {
	files   []string
	message string
	base    string
	head    string
	root    string
	kind    string
}

// gateResolveRefusal is why one input could not be resolved, in the closed
// vocabulary the run reports.
type gateResolveRefusal struct {
	code   domain.ErrorCode
	reason string
}

func gateRefuse(code domain.ErrorCode, reason string) *gateResolveRefusal {
	return &gateResolveRefusal{code: code, reason: reason}
}

// resolveGateInputs resolves every bound input for one gate on one surface.
// A deleted push range is not a failure: it skips the gate, because there
// is no head to inspect. Anything else unresolvable is a refusal.
func resolveGateInputs(gate config.Gate, binds map[string]config.GateBinding, invocation *Invocation, tree Tree, stdin *string) (map[string]gateResolvedInput, *gateResolveRefusal, bool) {
	inputs := gateInputsOf(tree)
	resolved := make(map[string]gateResolvedInput, len(binds))
	names := make([]string, 0, len(binds))
	for name := range binds {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		binding := binds[name]
		kind := ""
		if input, ok := gate.Inputs[name]; ok {
			kind = input.Kind
		}
		value, refusal, skipped := resolveGateInput(kind, binding, invocation, tree, inputs, stdin)
		if skipped {
			return nil, nil, true
		}
		if refusal != nil {
			return nil, refusal, false
		}
		resolved[name] = value
	}
	return resolved, nil, false
}

// resolveGateInput resolves one bound input. An unknown kind/source pair is
// an invalid typed binding: the declaration names a combination the
// vocabulary never allows, and dispatch must refuse it rather than guess.
func resolveGateInput(kind string, binding config.GateBinding, invocation *Invocation, tree Tree, inputs gateInputTree, stdin *string) (gateResolvedInput, *gateResolveRefusal, bool) {
	incomplete := func(reason string) *gateResolveRefusal {
		return gateRefuse(domain.ArgsIncomplete, reason)
	}
	notCommits := func() *gateResolveRefusal {
		return gateRefuse(domain.ArgsUnrecognized, "requires hexadecimal commit IDs for `--base` and `--head`")
	}
	git := func(reason string) *gateResolveRefusal {
		return gateRefuse(domain.RepositoryUnusable, reason)
	}
	switch {
	case kind == "files" && binding.Source == "checkout":
		return gateResolvedInput{files: tree.Files(), kind: "files"}, nil, false
	case kind == "files" && binding.Source == "explicit-range":
		if invocation.Base == nil || invocation.Head == nil {
			return gateResolvedInput{}, incomplete("requires both `--base` and `--head` for an explicit file range"), false
		}
		base, head := *invocation.Base, *invocation.Head
		if !isCommit(base) || !isCommit(head) {
			return gateResolvedInput{}, notCommits(), false
		}
		paths, err := inputs.ChangedFiles(base, head)
		if err != nil {
			return gateResolvedInput{}, git("cannot read changed files for the explicit range: " + err.Error()), false
		}
		return gateResolvedInput{files: paths, kind: "files"}, nil, false
	case kind == "regular-files" && binding.Source == "git-index":
		paths, err := inputs.IndexedRegularFiles()
		if err != nil {
			return gateResolvedInput{}, git("cannot resolve regular files from the Git index: " + err.Error()), false
		}
		return gateResolvedInput{files: paths, kind: "files"}, nil, false
	case kind == "regular-files" && binding.Source == "explicit-range":
		if invocation.Base == nil || invocation.Head == nil {
			return gateResolvedInput{}, incomplete("requires both `--base` and `--head` for an explicit regular-file range"), false
		}
		base, head := *invocation.Base, *invocation.Head
		if !isCommit(base) || !isCommit(head) {
			return gateResolvedInput{}, notCommits(), false
		}
		paths, err := inputs.ChangedRegularFiles(base, head)
		if err != nil {
			return gateResolvedInput{}, git("cannot read regular files for the explicit range: " + err.Error()), false
		}
		return gateResolvedInput{files: paths, kind: "files"}, nil, false
	case kind == "repository-state" && binding.Source == "checkout":
		return gateResolvedInput{root: tree.Root(), kind: "state"}, nil, false
	case kind == "commit-message" && binding.Source == "hook-message-file":
		if invocation.MessageFile == nil {
			return gateResolvedInput{}, incomplete("requires `--message-file` for a commit-message input"), false
		}
		path := *invocation.MessageFile
		message, readErr := inputs.HookMessageFile(path)
		if readErr != nil {
			switch readErr.Kind {
			case "missing":
				return gateResolvedInput{}, gateRefuse(domain.FileMissing, "message file `"+path+"` does not exist"), false
			case "not-text":
				return gateResolvedInput{}, gateRefuse(domain.FileUnreadable, "message file `"+path+"` is not text"), false
			default:
				return gateResolveInputFallback(readErr, path)
			}
		}
		return gateResolvedInput{message: message, kind: "message"}, nil, false
	case kind == "commit-message" && binding.Source == "explicit-range":
		if invocation.Base == nil || invocation.Head == nil {
			return gateResolvedInput{}, incomplete("requires both `--base` and `--head` for an explicit commit range"), false
		}
		base, head := *invocation.Base, *invocation.Head
		if !isCommit(base) || !isCommit(head) {
			return gateResolvedInput{}, notCommits(), false
		}
		messages, err := inputs.CommitMessages(base, head)
		if err != nil {
			return gateResolvedInput{}, git("cannot read commit messages for the explicit range: " + err.Error()), false
		}
		return gateResolvedInput{message: messages, kind: "message"}, nil, false
	case kind == "commit-range" && binding.Source == "explicit-range":
		if invocation.Base == nil || invocation.Head == nil {
			return gateResolvedInput{}, incomplete("requires both `--base` and `--head` for an explicit commit range"), false
		}
		base, head := *invocation.Base, *invocation.Head
		if !isCommit(base) || !isCommit(head) {
			return gateResolvedInput{}, notCommits(), false
		}
		return gateResolvedInput{base: base, head: head, kind: "range"}, nil, false
	case kind == "commit-range" && binding.Source == "push-updates":
		if !invocation.PushUpdatesStdin {
			return gateResolvedInput{}, incomplete("requires `--push-updates-stdin` for a push-update range"), false
		}
		text := ""
		if stdin != nil {
			text = *stdin
		}
		parsed, reason := parsePushRange(text, binding.Fallback)
		if reason != "" {
			return gateResolvedInput{}, gateRefuse(domain.InputUnreadable, reason), false
		}
		if parsed.deleted {
			return gateResolvedInput{}, nil, true
		}
		base := parsed.base
		if parsed.newRef {
			resolved, err := inputs.ResolveGitRef(parsed.fallback)
			if err != nil {
				return gateResolvedInput{}, git("cannot resolve push fallback `" + parsed.fallback + "`: " + err.Error()), false
			}
			base = resolved
		}
		return gateResolvedInput{base: base, head: parsed.head, kind: "range"}, nil, false
	case kind == "files" && binding.Source == "git-index":
		paths, err := inputs.IndexedFiles()
		if err != nil {
			return gateResolvedInput{}, git("cannot resolve the Git index: " + err.Error()), false
		}
		return gateResolvedInput{files: paths, kind: "files"}, nil, false
	default:
		return gateResolvedInput{}, gateRefuse(domain.ConfigUnusable, "has an invalid typed input binding"), false
	}
}

// gateResolveInputFallback maps the hook-file read fault that is neither
// missing nor non-text to its unreadable refusal.
func gateResolveInputFallback(readErr *ReadError, path string) (gateResolvedInput, *gateResolveRefusal, bool) {
	return gateResolvedInput{}, gateRefuse(domain.FileUnreadable, "message file `"+path+"` cannot be read: "+readErr.Reason), false
}

// projectGateCommand projects the executable argv and environment from the
// resolved inputs. A literal travels untouched; a typed item expands one
// value for single and every value for repeat.
func projectGateCommand(executable string, args []config.GateArg, environment map[string]config.GateEnv, inputs map[string]gateResolvedInput) ([]string, map[string]string, string) {
	argv := []string{executable}
	for _, argument := range args {
		switch {
		case argument.HasLiteral && !argument.HasInput && argument.Expand == "":
			argv = append(argv, argument.Literal)
		case !argument.HasLiteral && argument.HasInput && (argument.Expand == "single" || argument.Expand == "repeat"):
			values, reason := referencedValues(inputs, argument.Input)
			if reason != "" {
				return nil, nil, reason
			}
			if argument.Expand == "single" {
				if len(values) != 1 {
					return nil, nil, "`" + argument.Input + "` is not one value for `expand: single`"
				}
				argv = append(argv, values[0])
				continue
			}
			argv = append(argv, values...)
		default:
			return nil, nil, "has an invalid argv item after validation"
		}
	}
	names := make([]string, 0, len(environment))
	for name := range environment {
		names = append(names, name)
	}
	sort.Strings(names)
	projected := make(map[string]string, len(environment))
	for _, name := range names {
		entry := environment[name]
		values, reason := referencedValues(inputs, entry.Input)
		if reason != "" {
			return nil, nil, reason
		}
		if len(values) != 1 {
			return nil, nil, "`" + entry.Input + "` cannot project a sequence into environment variable `" + name + "`"
		}
		projected[name] = values[0]
	}
	return argv, projected, ""
}

// referencedValues reads one `input.field` reference from the resolved
// inputs: repository root, file paths, commit-message text, or a range side.
func referencedValues(inputs map[string]gateResolvedInput, reference string) ([]string, string) {
	name, field, ok := strings.Cut(reference, ".")
	if !ok {
		return nil, "`" + reference + "` is not an input field reference"
	}
	input, ok := inputs[name]
	if !ok {
		return nil, "`" + reference + "` was not resolved for this surface"
	}
	switch input.kind {
	case "files":
		if field == "paths" {
			return append([]string(nil), input.files...), ""
		}
	case "message":
		if field == "text" {
			return []string{input.message}, ""
		}
	case "range":
		if field == "base" {
			return []string{input.base}, ""
		}
		if field == "head" {
			return []string{input.head}, ""
		}
	case "state":
		if field == "root" {
			return []string{input.root}, ""
		}
	}
	return nil, "`" + reference + "` is not available on its resolved input"
}

// isCommit reports whether value is a hexadecimal commit ID: 7 to 64
// hexadecimal characters, the range Git accepts for an unambiguous object.
func isCommit(value string) bool {
	if len(value) < 7 || len(value) > 64 {
		return false
	}
	for i := 0; i < len(value); i++ {
		character := value[i]
		if character >= '0' && character <= '9' || character >= 'a' && character <= 'f' || character >= 'A' && character <= 'F' {
			continue
		}
		return false
	}
	return true
}

// isZero reports whether value is a non-empty all-zero object ID: the push
// record marker for a created or deleted ref.
func isZero(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] != '0' {
			return false
		}
	}
	return true
}

// pushRange is one parsed pre-push update record.
type pushRange struct {
	base     string
	head     string
	fallback string
	newRef   bool
	deleted  bool
}

// parsePushRange parses the single pre-push update record. A normal update
// carries both sides; a new ref resolves its base through the declared
// fallback; a deleted ref skips the range gate.
func parsePushRange(stdin string, fallback *string) (pushRange, string) {
	// Mirror the oracle's `lines`: a final newline terminates the record
	// without opening a second one, while any other blank line is a second
	// record. An input of only `"\n"` is one empty record, not no record.
	var records []string
	if stdin != "" {
		split := strings.Split(stdin, "\n")
		if strings.HasSuffix(stdin, "\n") {
			split = split[:len(split)-1]
		}
		for _, line := range split {
			records = append(records, strings.TrimSuffix(line, "\r"))
		}
	}
	if len(records) == 0 {
		return pushRange{}, "received no pre-push update record"
	}
	if len(records) > 1 {
		return pushRange{}, "received multiple pre-push update records; select one declared ref first"
	}
	fields := strings.Fields(records[0])
	if len(fields) != 4 {
		return pushRange{}, "received an invalid pre-push update record"
	}
	local, remote := fields[1], fields[3]
	if isZero(local) {
		return pushRange{deleted: true}, ""
	}
	if !isCommit(local) {
		return pushRange{}, "received a push update without a hexadecimal local commit ID"
	}
	if isZero(remote) {
		if fallback == nil {
			return pushRange{}, "received a new push ref without a declared fallback ref"
		}
		return pushRange{head: local, fallback: *fallback, newRef: true}, ""
	}
	if !isCommit(remote) {
		return pushRange{}, "received a push update without a hexadecimal remote commit ID"
	}
	return pushRange{base: remote, head: local}, ""
}
