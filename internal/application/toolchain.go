// Toolchain operations: declared probes and explicit provision vectors.
//
// Validate is read-only apart from the declared probe it runs: it opens no
// configuration beyond the declared toolchains group and never reports tool
// output, only rule names. Provision plans first and only applies with
// --apply, running each declared no-shell vector in declaration order and
// stopping after the first failure. Process spawning stays in the narrow
// ToolchainRunner boundary the process entry point supplies.
package application

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// ToolchainLaunch is one declared probe or provision vector. Like a gate
// launch it carries no hook surface, but unlike one it also carries no
// working directory or inherited environment conversation: the vector is
// exactly what the repository declared, and there is no shell to reinterpret
// it.
type ToolchainLaunch struct {
	Executable string
	Arguments  []string
	// TimeoutSeconds bounds one child; nil leaves no time limit.
	TimeoutSeconds *uint64
}

// ToolchainResult is what one child run produced. Stdout is captured only
// for the declared version parser; operation output never includes it.
type ToolchainResult struct {
	Code   int
	Stdout string
}

// ToolchainRunner is the only capability that starts a declared toolchain
// probe or provision vector. Validators and dry-run planners never receive
// it; tests supply a recorder.
type ToolchainRunner interface {
	Run(launch ToolchainLaunch) (ToolchainResult, error)
}

// closedRunner fails every launch closed, the way the oracle's
// NoToolchainRunner does: a probe records every required toolchain as
// unavailable, and an applied provision refuses that it could not start.
type closedRunner struct{}

func (closedRunner) Run(launch ToolchainLaunch) (ToolchainResult, error) {
	return ToolchainResult{}, fmt.Errorf("this entry point has no toolchain process boundary")
}

type toolchainProvision struct {
	platform   string
	executable string
	arguments  []string
}

type toolchain struct {
	id         string
	executable string
	probe      []string
	parser     *string
	version    *string
	timeout    *uint64
	provision  []toolchainProvision
	required   bool
}

func toolchainValidate(document *config.Document, runner ToolchainRunner, format Format) Outcome {
	toolchains, refusal := decodeToolchains(document)
	if refusal != nil {
		return RefuseWith(format, refusal.Code, refusal.Message)
	}
	if runner == nil {
		runner = closedRunner{}
	}
	type finding struct{ id, rule string }
	var findings []finding
	for _, entry := range toolchains {
		result, err := runner.Run(ToolchainLaunch{
			Executable:     entry.executable,
			Arguments:      append([]string(nil), entry.probe...),
			TimeoutSeconds: entry.timeout,
		})
		if err != nil {
			if entry.required {
				findings = append(findings, finding{entry.id, "unavailable"})
			}
			continue
		}
		if result.Code != 0 {
			if entry.required {
				findings = append(findings, finding{entry.id, "probe-failed"})
			}
			continue
		}
		if !toolchainVersionMatches(entry, result.Stdout) && entry.required {
			findings = append(findings, finding{entry.id, "version-mismatch"})
		}
	}
	if len(findings) == 0 {
		if format == JSON {
			return Clean(fmt.Sprintf("{\"command\":\"toolchain-validate\",\"exitCode\":0,"+
				"\"inspected\":%d,\"schemaVersion\":1}\n", len(toolchains)))
		}
		return Clean(fmt.Sprintf("[toolchain-validate] checked %d declared toolchains, no findings\n",
			len(toolchains)))
	}
	if format == JSON {
		rendered := make([]string, 0, len(findings))
		for _, found := range findings {
			rendered = append(rendered, fmt.Sprintf("{\"rule\":%s,\"toolchain\":%s}",
				domain.Quote(found.rule), domain.Quote(found.id)))
		}
		return Outcome{ExitCode: 1, Stdout: fmt.Sprintf(
			"{\"command\":\"toolchain-validate\",\"exitCode\":1,\"findings\":[%s],\"schemaVersion\":1}\n",
			strings.Join(rendered, ","))}
	}
	var out strings.Builder
	for _, found := range findings {
		fmt.Fprintf(&out, "[toolchain-validate] %s: %s\n", found.id, found.rule)
	}
	return Outcome{ExitCode: 1, Stderr: out.String()}
}

// provisionToolchains plans each supported provision vector by default.
// --apply is the explicit confirmation that starts the declared no-shell
// children in declaration order.
func provisionToolchains(document *config.Document, runner ToolchainRunner, apply bool, format Format) Outcome {
	toolchains, refusal := decodeToolchains(document)
	if refusal != nil {
		return RefuseWith(format, refusal.Code, refusal.Message)
	}
	if runner == nil {
		runner = closedRunner{}
	}
	platform := runtime.GOOS
	type declared struct {
		entry  toolchain
		vector toolchainProvision
	}
	var planned []declared
	for _, entry := range toolchains {
		if len(entry.provision) == 0 {
			continue
		}
		matched := -1
		for i, vector := range entry.provision {
			if vector.platform == platform {
				matched = i
				break
			}
		}
		if matched < 0 {
			return RefuseWith(format, domain.ToolchainUndeclared,
				fmt.Sprintf("toolchain `%s` has no provision declaration for `%s`", entry.id, platform))
		}
		planned = append(planned, declared{entry, entry.provision[matched]})
	}
	if !apply {
		ids := make([]string, 0, len(planned))
		for _, item := range planned {
			ids = append(ids, item.entry.id)
		}
		if format == JSON {
			return Clean(fmt.Sprintf("{\"command\":\"toolchain-provision\",\"count\":%d,"+
				"\"schemaVersion\":1,\"status\":\"planned\",\"toolchains\":[%s]}\n",
				len(planned), quoteList(ids)))
		}
		return Clean(fmt.Sprintf("[toolchain-provision] planned %d declared toolchains; pass --apply to run\n",
			len(planned)))
	}
	completed := 0
	for _, item := range planned {
		result, err := runner.Run(ToolchainLaunch{
			Executable:     item.vector.executable,
			Arguments:      append([]string(nil), item.vector.arguments...),
			TimeoutSeconds: item.entry.timeout,
		})
		if err != nil {
			return RefuseWith(format, domain.ToolchainFailed,
				fmt.Sprintf("toolchain `%s` could not start: %s", item.entry.id, err.Error()))
		}
		if result.Code != 0 {
			return RefuseWith(format, domain.ToolchainFailed,
				fmt.Sprintf("toolchain `%s` provision failed with status %d", item.entry.id, result.Code))
		}
		completed++
	}
	if format == JSON {
		return Clean(fmt.Sprintf("{\"command\":\"toolchain-provision\",\"count\":%d,"+
			"\"schemaVersion\":1,\"status\":\"completed\"}\n", completed))
	}
	return Clean(fmt.Sprintf("[toolchain-provision] completed %d declared toolchains\n", completed))
}

func toolchainVersionMatches(entry toolchain, stdout string) bool {
	if entry.version == nil {
		return true
	}
	if entry.parser == nil {
		return false
	}
	var found string
	switch *entry.parser {
	case "first-line":
		if line, _, ok := strings.Cut(stdout, "\n"); ok {
			found = strings.TrimSpace(line)
		} else {
			found = strings.TrimSpace(stdout)
		}
	case "first-token":
		found = firstToken(stdout)
	}
	return found == *entry.version
}

func firstToken(stdout string) string {
	fields := strings.Fields(stdout)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func validToolchainExecutable(executable string) bool {
	return strings.TrimSpace(executable) != "" &&
		!strings.HasPrefix(executable, "-") &&
		!strings.ContainsAny(executable, "\x00\n\r")
}

func validToolchainPlatform(platform string) bool {
	if platform == "" {
		return false
	}
	for _, character := range platform {
		if character >= 'a' && character <= 'z' {
			continue
		}
		if character >= '0' && character <= '9' {
			continue
		}
		if character == '-' {
			continue
		}
		return false
	}
	return true
}

func validateToolchainPolicy(toolchains []toolchain) error {
	seen := map[string]bool{}
	for _, entry := range toolchains {
		if strings.TrimSpace(entry.id) == "" || seen[entry.id] {
			return fmt.Errorf("identifiers must be unique and non-empty")
		}
		seen[entry.id] = true
		if !validToolchainExecutable(entry.executable) {
			return fmt.Errorf("`%s` has an invalid executable", entry.id)
		}
		if len(entry.probe) == 0 {
			return fmt.Errorf("`%s` declares no probe argv", entry.id)
		}
		if (entry.version == nil) != (entry.parser == nil) {
			return fmt.Errorf("`%s` must declare both parser and version, or neither", entry.id)
		}
		if entry.version != nil && strings.TrimSpace(*entry.version) == "" {
			return fmt.Errorf("`%s` has an empty version constraint", entry.id)
		}
		if entry.timeout != nil && *entry.timeout == 0 {
			return fmt.Errorf("`%s` has a zero timeout", entry.id)
		}
		platforms := map[string]bool{}
		for _, vector := range entry.provision {
			if !validToolchainPlatform(vector.platform) ||
				!validToolchainExecutable(vector.executable) ||
				platforms[vector.platform] {
				return fmt.Errorf("`%s` has a duplicate or invalid provision declaration", entry.id)
			}
			platforms[vector.platform] = true
		}
	}
	return nil
}

// decodeToolchains projects the declared toolchains group. Unknown keys
// refuse: a policy the tool silently ignored would be a check that never ran.
func decodeToolchains(document *config.Document) ([]toolchain, *Refusal) {
	mapping, ok := document.Toolchains()
	if !ok {
		return nil, &Refusal{Code: domain.ConfigUndeclared, Message: "toolchains section is not declared"}
	}
	for key := range mapping {
		if key != "entries" {
			return nil, &Refusal{Code: domain.ConfigUnusable,
				Message: fmt.Sprintf("toolchains names unknown key `%s`", key)}
		}
	}
	raw, present := mapping["entries"]
	if !present || raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, &Refusal{Code: domain.ConfigUnusable, Message: "toolchains.entries names no entries"}
	}
	entries := make([]toolchain, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, &Refusal{Code: domain.ConfigUnusable, Message: "toolchains.entries names no entries"}
		}
		decoded, err := decodeToolchainEntry(entry)
		if err != nil {
			return nil, &Refusal{Code: domain.ConfigUnusable, Message: err.Error()}
		}
		entries = append(entries, decoded)
	}
	if err := validateToolchainPolicy(entries); err != nil {
		return nil, &Refusal{Code: domain.ConfigUnusable, Message: "toolchain policy: " + err.Error()}
	}
	return entries, nil
}

func decodeToolchainEntry(entry map[string]any) (toolchain, error) {
	for key := range entry {
		switch key {
		case "id", "executable", "probe", "parser", "version", "timeout-seconds", "provision", "required":
		default:
			return toolchain{}, fmt.Errorf("toolchains.entries names unknown key `%s`", key)
		}
	}
	id, ok := toolchainString(entry, "id")
	if !ok {
		return toolchain{}, fmt.Errorf("toolchains.entries names no id")
	}
	executable, ok := toolchainString(entry, "executable")
	if !ok {
		return toolchain{}, fmt.Errorf("toolchains.entries names no executable")
	}
	decoded := toolchain{id: id, executable: executable}
	probe, present, err := toolchainStrings(entry, "probe")
	if err != nil || (present && probe == nil) {
		return toolchain{}, fmt.Errorf("toolchains.entries names no probe")
	}
	decoded.probe = probe
	if raw, present := entry["parser"]; present && raw != nil {
		text, ok := environmentScalar(raw)
		if !ok {
			return toolchain{}, fmt.Errorf("toolchains.entries names no parser")
		}
		switch text {
		case "first-line", "first-token":
			decoded.parser = &text
		default:
			return toolchain{}, fmt.Errorf("toolchains.entries names unknown parser `%s`", text)
		}
	}
	if raw, present := entry["version"]; present && raw != nil {
		text, ok := environmentScalar(raw)
		if !ok {
			return toolchain{}, fmt.Errorf("toolchains.entries names no version")
		}
		decoded.version = &text
	}
	if raw, present := entry["timeout-seconds"]; present && raw != nil {
		seconds, ok := raw.(int)
		if !ok || seconds < 0 {
			return toolchain{}, fmt.Errorf("toolchains.entries names no timeout")
		}
		value := uint64(seconds)
		decoded.timeout = &value
	}
	if raw, present := entry["required"]; present && raw != nil {
		required, ok := raw.(bool)
		if !ok {
			return toolchain{}, fmt.Errorf("toolchains.entries names no required value")
		}
		decoded.required = required
	}
	if raw, present := entry["provision"]; present && raw != nil {
		vectors, err := decodeToolchainProvision(raw)
		if err != nil {
			return toolchain{}, err
		}
		decoded.provision = vectors
	}
	return decoded, nil
}

func decodeToolchainProvision(raw any) ([]toolchainProvision, error) {
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("toolchains.provision names no provision")
	}
	vectors := make([]toolchainProvision, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("toolchains.provision names no provision")
		}
		for key := range entry {
			switch key {
			case "platform", "executable", "arguments":
			default:
				return nil, fmt.Errorf("toolchains.provision names unknown key `%s`", key)
			}
		}
		platform, ok := toolchainString(entry, "platform")
		if !ok {
			return nil, fmt.Errorf("toolchains.provision names no platform")
		}
		executable, ok := toolchainString(entry, "executable")
		if !ok {
			return nil, fmt.Errorf("toolchains.provision names no executable")
		}
		vector := toolchainProvision{platform: platform, executable: executable}
		arguments, present, err := toolchainStrings(entry, "arguments")
		if err != nil || (present && arguments == nil) {
			return nil, fmt.Errorf("toolchains.provision names no arguments")
		}
		vector.arguments = arguments
		vectors = append(vectors, vector)
	}
	return vectors, nil
}

func toolchainString(entry map[string]any, key string) (string, bool) {
	raw, present := entry[key]
	if !present || raw == nil {
		return "", false
	}
	return environmentScalar(raw)
}

// toolchainStrings reads a declared string list the way the oracle does:
// absent stays absent, an explicit empty list stays empty, and anything else
// is not a list of names.
func toolchainStrings(entry map[string]any, key string) ([]string, bool, error) {
	raw, present := entry[key]
	if !present || raw == nil {
		return nil, false, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, true, fmt.Errorf("not a list")
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		value, ok := environmentScalar(item)
		if !ok {
			return nil, true, fmt.Errorf("not a list")
		}
		values = append(values, value)
	}
	return values, true, nil
}
