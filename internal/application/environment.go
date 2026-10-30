// Environment operations: declared local setup and redacted validation.
//
// Validate is read-only: it opens only declared detector sources and reports
// only names, never values. Init, backup, and restore hold the narrow
// EnvironmentStore boundary; a nil store fails closed, so validators and
// dry-run planners never write.
package application

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// EnvironmentFile is one declared target with its desired bytes.
type EnvironmentFile struct {
	Path     string
	Contents string
}

// EnvironmentRestoreFile is one declared target with its backup bytes.
// Replace authorizes overwriting an existing target.
type EnvironmentRestoreFile struct {
	Path     string
	Contents string
	Replace  bool
}

// EnvironmentStore is the narrow write boundary only init, backup, and
// restore hold. Validators never receive it.
type EnvironmentStore interface {
	Create(root string, files []EnvironmentFile) error
	Restore(root string, files []EnvironmentRestoreFile) error
}

// indexedTree is the narrow read boundary only the staging guard holds: the
// Git index snapshot, never the mutable working tree.
type indexedTree interface {
	IndexedFiles() ([]string, error)
}

type environmentExample struct {
	source string
	target string
}

type stagedPolicy struct {
	forbidden []string
	allowed   []string
}

type environmentContract struct {
	path string
	keys []string
}

type environmentDetector struct {
	language string
	paths    []string
}

type environmentAllowlist struct {
	rule   string
	path   string
	key    string
	reason string
	owner  string
	expiry string
}

type environmentPolicy struct {
	examples   []environmentExample
	staged     *stagedPolicy
	contracts  []environmentContract
	detectors  []environmentDetector
	allowlists []environmentAllowlist
}

func environmentValidate(document *config.Document, tree Tree, format Format) Outcome {
	policy, refusal := decodeEnvironment(document)
	if refusal != nil {
		return refuseEnvironment(format, refusal)
	}
	if err := validateDetectorPolicy(policy); err != nil {
		return RefuseWith(format, domain.ConfigUnusable, "environment detector policy: "+err.Error())
	}
	var inspected int
	findings := map[environmentFinding]bool{}
	if policy.staged != nil {
		for _, path := range policy.staged.allowed {
			if !isExactPath(path) {
				return RefuseWith(format, domain.PathEscapesRoot,
					"environment staging guard allows only exact repository-relative paths")
			}
		}
		forbidden, allowed, err := compileStaged(policy.staged)
		if err != nil {
			return RefuseWith(format, domain.ConfigUnusable, "environment staging guard: "+err.Error())
		}
		indexed, ok := tree.(indexedTree)
		if !ok {
			return RefuseWith(format, domain.RepositoryUnusable,
				"environment staging guard refused: this tree has no Git index snapshot boundary")
		}
		staged, err := indexed.IndexedFiles()
		if err != nil {
			return RefuseWith(format, domain.RepositoryUnusable,
				"environment staging guard refused: "+err.Error())
		}
		inspected += len(staged)
		for _, path := range staged {
			if matchAny(forbidden, path) && !matchAny(allowed, path) {
				findings[environmentFinding{rule: "staged-environment-file", path: path}] = true
			}
		}
	}
	declared := map[[2]string]bool{}
	declaredKeys := map[string]bool{}
	for _, contract := range policy.contracts {
		for _, key := range contract.keys {
			declared[[2]string{contract.path, key}] = true
			declaredKeys[key] = true
		}
	}
	readKeys := map[string]bool{}
	for _, detector := range policy.detectors {
		for _, path := range detector.paths {
			inspected++
			source, readErr := tree.Read(path)
			if readErr != nil {
				switch readErr.Kind {
				case "missing":
					findings[environmentFinding{rule: "declared-source-unread", path: path}] = true
					continue
				case "not-text":
					return RefuseWith(format, domain.FileUnreadable,
						"environment detector source `"+path+"` holds no text")
				default:
					return RefuseWith(format, domain.FileUnreadable,
						"environment detector source `"+path+"` is unreadable: "+readErr.Reason)
				}
			}
			keys, dynamic := detectorAccesses(detector.language, source)
			for _, key := range keys {
				readKeys[key] = true
				if !declaredKeys[key] {
					findings[environmentFinding{rule: "read-but-undeclared", path: path, key: key, hasKey: true}] = true
				}
			}
			if dynamic {
				findings[environmentFinding{rule: "unsupported-dynamic-access", path: path, key: "<dynamic>", hasKey: true}] = true
			}
		}
	}
	for pair := range declared {
		if !readKeys[pair[1]] {
			findings[environmentFinding{rule: "declared-but-unread", path: pair[0], key: pair[1], hasKey: true}] = true
		}
	}
	ordered := make([]environmentFinding, 0, len(findings))
	for finding := range findings {
		if finding.rule == "staged-environment-file" || !allowlisted(policy, finding) {
			ordered = append(ordered, finding)
		}
	}
	sort.Slice(ordered, func(i, j int) bool { return compareEnvironmentFindings(ordered[i], ordered[j]) < 0 })
	return renderEnvironmentFindings(ordered, inspected, format)
}

type environmentFinding struct {
	rule   string
	path   string
	key    string
	hasKey bool
}

func compareEnvironmentFindings(left, right environmentFinding) int {
	if left.rule != right.rule {
		if left.rule < right.rule {
			return -1
		}
		return 1
	}
	if left.path != right.path {
		if left.path < right.path {
			return -1
		}
		return 1
	}
	switch {
	case left.key < right.key:
		return -1
	case left.key > right.key:
		return 1
	}
	return 0
}

func allowlisted(policy *environmentPolicy, finding environmentFinding) bool {
	if !finding.hasKey {
		return false
	}
	for _, allow := range policy.allowlists {
		if allow.rule == finding.rule && allow.path == finding.path && allow.key == finding.key {
			return true
		}
	}
	return false
}

func renderEnvironmentFindings(findings []environmentFinding, inspected int, format Format) Outcome {
	if len(findings) == 0 {
		if format == JSON {
			return Clean(fmt.Sprintf("{\"command\":\"environment-validate\",\"exitCode\":0,"+
				"\"inspected\":%d,\"schemaVersion\":1}\n", inspected))
		}
		return Clean(fmt.Sprintf("[environment-validate] checked %d declared files, no findings\n", inspected))
	}
	if format == JSON {
		rendered := make([]string, 0, len(findings))
		for _, finding := range findings {
			key := "null"
			if finding.hasKey {
				key = domain.Quote(finding.key)
			}
			rendered = append(rendered, fmt.Sprintf("{\"key\":%s,\"path\":%s,\"rule\":%s}",
				key, domain.Quote(finding.path), domain.Quote(finding.rule)))
		}
		return Outcome{ExitCode: 1, Stdout: fmt.Sprintf(
			"{\"command\":\"environment-validate\",\"exitCode\":1,\"findings\":[%s],\"schemaVersion\":1}\n",
			strings.Join(rendered, ","))}
	}
	var out strings.Builder
	for _, finding := range findings {
		if finding.hasKey {
			fmt.Fprintf(&out, "[environment-validate] %s: %s (%s)\n", finding.path, finding.rule, finding.key)
		} else {
			fmt.Fprintf(&out, "[environment-validate] %s: %s\n", finding.path, finding.rule)
		}
	}
	return Outcome{ExitCode: 1, Stderr: out.String()}
}

func compileStaged(staged *stagedPolicy) (forbidden, allowed []string, err error) {
	check := func(patterns []string) ([]string, error) {
		for _, pattern := range patterns {
			if leavesRootPattern(pattern) {
				return nil, fmt.Errorf("path pattern `%s` must be relative to the repository root", pattern)
			}
			if _, err := Match(pattern, pattern); err != nil {
				return nil, fmt.Errorf("invalid path pattern `%s`: %s", pattern, err.Error())
			}
		}
		return patterns, nil
	}
	if forbidden, err = check(staged.forbidden); err != nil {
		return nil, nil, err
	}
	if allowed, err = check(staged.allowed); err != nil {
		return nil, nil, err
	}
	return forbidden, allowed, nil
}

func leavesRootPattern(pattern string) bool {
	if strings.HasPrefix(pattern, "/") {
		return true
	}
	for _, segment := range strings.Split(pattern, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func matchAny(patterns []string, path string) bool {
	for _, pattern := range patterns {
		if ok, err := Match(pattern, path); err == nil && ok {
			return true
		}
	}
	return false
}

// refuseEnvironment renders a decode refusal for the requested format.
func refuseEnvironment(format Format, refusal *Refusal) Outcome {
	return Refused(domain.Body(format, refusal.Code, refusal.Message))
}

// decodeEnvironment projects the declared environment group. Unknown keys
// refuse: a policy the tool silently ignored would be a check that never ran.
func decodeEnvironment(document *config.Document) (*environmentPolicy, *Refusal) {
	mapping, ok := document.Environment()
	if !ok {
		return nil, &Refusal{Code: domain.ConfigUndeclared, Message: "environment section is not declared"}
	}
	policy := &environmentPolicy{}
	for key := range mapping {
		switch key {
		case "examples", "staged", "contracts", "detectors", "allowlists":
		default:
			return nil, &Refusal{Code: domain.ConfigUnusable,
				Message: fmt.Sprintf("environment names unknown key `%s`", key)}
		}
	}
	var err error
	if policy.examples, err = decodeEnvironmentExamples(mapping); err != nil {
		return nil, &Refusal{Code: domain.ConfigUnusable, Message: err.Error()}
	}
	if policy.staged, err = decodeStagedPolicy(mapping); err != nil {
		return nil, &Refusal{Code: domain.ConfigUnusable, Message: err.Error()}
	}
	if policy.contracts, err = decodeEnvironmentContracts(mapping); err != nil {
		return nil, &Refusal{Code: domain.ConfigUnusable, Message: err.Error()}
	}
	if policy.detectors, err = decodeEnvironmentDetectors(mapping); err != nil {
		return nil, &Refusal{Code: domain.ConfigUnusable, Message: err.Error()}
	}
	if policy.allowlists, err = decodeEnvironmentAllowlists(mapping); err != nil {
		return nil, &Refusal{Code: domain.ConfigUnusable, Message: err.Error()}
	}
	return policy, nil
}

func decodeEnvironmentExamples(mapping map[string]any) ([]environmentExample, error) {
	raw, present := mapping["examples"]
	if !present {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("environment.examples names no examples")
	}
	examples := make([]environmentExample, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("environment.examples names no source and target")
		}
		for key := range entry {
			if key != "source" && key != "target" {
				return nil, fmt.Errorf("environment.examples names unknown key `%s`", key)
			}
		}
		source, ok := entry["source"].(string)
		if !ok {
			return nil, fmt.Errorf("environment.examples names no source and target")
		}
		target, ok := entry["target"].(string)
		if !ok {
			return nil, fmt.Errorf("environment.examples names no source and target")
		}
		examples = append(examples, environmentExample{source: source, target: target})
	}
	return examples, nil
}

func decodeEnvironmentStrings(mapping map[string]any, key string) ([]string, error) {
	raw, present := mapping[key]
	if !present {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("environment.%s names no paths", key)
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		value, ok := environmentScalar(item)
		if !ok {
			return nil, fmt.Errorf("environment.%s names no paths", key)
		}
		values = append(values, value)
	}
	return values, nil
}

// environmentScalar reads one declared scalar the way the oracle does: plain
// strings stay strings, and an unquoted ISO date stays its calendar text
// rather than becoming a timestamp. Anything else is not a name.
func environmentScalar(raw any) (string, bool) {
	switch value := raw.(type) {
	case string:
		return value, true
	case time.Time:
		return value.Format("2006-01-02"), true
	default:
		return "", false
	}
}

func decodeStagedPolicy(mapping map[string]any) (*stagedPolicy, error) {
	raw, present := mapping["staged"]
	if !present {
		return nil, nil
	}
	entry, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("environment.staged names no staged policy")
	}
	for key := range entry {
		if key != "forbidden" && key != "allowed" {
			return nil, fmt.Errorf("environment.staged names unknown key `%s`", key)
		}
	}
	staged := &stagedPolicy{}
	var err error
	if staged.forbidden, err = decodeEnvironmentStrings(entry, "forbidden"); err != nil {
		return nil, err
	}
	if staged.allowed, err = decodeEnvironmentStrings(entry, "allowed"); err != nil {
		return nil, err
	}
	return staged, nil
}

func decodeEnvironmentContracts(mapping map[string]any) ([]environmentContract, error) {
	raw, present := mapping["contracts"]
	if !present {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("environment.contracts names no contracts")
	}
	contracts := make([]environmentContract, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("environment.contracts names no contracts")
		}
		for key := range entry {
			if key != "path" && key != "keys" {
				return nil, fmt.Errorf("environment.contracts names unknown key `%s`", key)
			}
		}
		path, ok := entry["path"].(string)
		if !ok {
			return nil, fmt.Errorf("environment.contracts names no path")
		}
		keys, err := decodeEnvironmentStrings(entry, "keys")
		if err != nil {
			return nil, err
		}
		contracts = append(contracts, environmentContract{path: path, keys: keys})
	}
	return contracts, nil
}

func decodeEnvironmentDetectors(mapping map[string]any) ([]environmentDetector, error) {
	raw, present := mapping["detectors"]
	if !present {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("environment.detectors names no detectors")
	}
	detectors := make([]environmentDetector, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("environment.detectors names no detectors")
		}
		for key := range entry {
			if key != "language" && key != "paths" {
				return nil, fmt.Errorf("environment.detectors names unknown key `%s`", key)
			}
		}
		language, ok := entry["language"].(string)
		if !ok {
			return nil, fmt.Errorf("environment.detectors names no language")
		}
		switch language {
		case "rust", "type-script", "f-sharp", "go", "terraform", "ansible":
		default:
			return nil, fmt.Errorf("environment.detectors names unknown language `%s`", language)
		}
		paths, err := decodeEnvironmentStrings(entry, "paths")
		if err != nil {
			return nil, err
		}
		detectors = append(detectors, environmentDetector{language: language, paths: paths})
	}
	return detectors, nil
}

func decodeEnvironmentAllowlists(mapping map[string]any) ([]environmentAllowlist, error) {
	raw, present := mapping["allowlists"]
	if !present {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("environment.allowlists names no allowlists")
	}
	allowlists := make([]environmentAllowlist, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("environment.allowlists names no allowlists")
		}
		for key := range entry {
			switch key {
			case "rule", "path", "key", "reason", "owner", "expiry":
			default:
				return nil, fmt.Errorf("environment.allowlists names unknown key `%s`", key)
			}
		}
		allow := environmentAllowlist{}
		var okRule, okPath, okKey, okReason, okOwner, okExpiry bool
		allow.rule, okRule = entry["rule"].(string)
		allow.path, okPath = entry["path"].(string)
		allow.key, okKey = entry["key"].(string)
		allow.reason, okReason = entry["reason"].(string)
		allow.owner, okOwner = entry["owner"].(string)
		allow.expiry, okExpiry = environmentScalar(entry["expiry"])
		if !okRule || !okPath || !okKey || !okReason || !okOwner || !okExpiry {
			return nil, fmt.Errorf("environment.allowlists names no rule, path, key, reason, owner, and expiry")
		}
		allowlists = append(allowlists, allow)
	}
	return allowlists, nil
}

// validateDetectorPolicy refuses a policy that names no exact checkable
// surface, before any file is opened.
func validateDetectorPolicy(policy *environmentPolicy) error {
	pairs := map[[2]string]bool{}
	for _, contract := range policy.contracts {
		if !isExactPath(contract.path) {
			return fmt.Errorf("contract paths must be exact repository-relative files")
		}
		for _, key := range contract.keys {
			if !isEnvironmentKey(key) || pairs[[2]string{contract.path, key}] {
				return fmt.Errorf("contract keys must be unique valid environment names")
			}
			pairs[[2]string{contract.path, key}] = true
		}
	}
	seen := map[[2]string]bool{}
	for _, detector := range policy.detectors {
		if len(detector.paths) == 0 {
			return fmt.Errorf("each detector requires one or more exact source paths")
		}
		for _, path := range detector.paths {
			if !isExactPath(path) || seen[[2]string{detector.language, path}] {
				return fmt.Errorf("detector source paths must be exact and unique per language")
			}
			seen[[2]string{detector.language, path}] = true
		}
	}
	rows := map[[3]string]bool{}
	for _, allow := range policy.allowlists {
		if strings.TrimSpace(allow.rule) == "" || !isExactPath(allow.path) ||
			!isEnvironmentKey(allow.key) || strings.TrimSpace(allow.reason) == "" ||
			strings.TrimSpace(allow.owner) == "" || !isISODate(allow.expiry) ||
			rows[[3]string{allow.rule, allow.path, allow.key}] {
			return fmt.Errorf("allowlists require unique exact rule, path, key, reason, owner, and ISO expiry")
		}
		rows[[3]string{allow.rule, allow.path, allow.key}] = true
	}
	return nil
}

// isRelativeFile answers whether a path names a file inside the root: no
// emptiness, no leading slash, no dot segments, no smuggled separators.
// Unlike an exact path it may carry glob metacharacters; the environment
// leaves never match globs, they only refuse them elsewhere.
func isRelativeFile(path string) bool {
	if strings.TrimSpace(path) == "" || strings.HasPrefix(path, "/") {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." || strings.ContainsRune(part, '\\') {
			return false
		}
	}
	return true
}

func isEnvironmentKey(key string) bool {
	if key == "" {
		return false
	}
	head := key[0]
	if !(head == '_' || head >= 'a' && head <= 'z' || head >= 'A' && head <= 'Z') {
		return false
	}
	for i := 1; i < len(key); i++ {
		c := key[i]
		if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// detectorAccesses reads one declared source for the curated reader of its
// language, reporting only names. A computed key stays dynamic, which is a
// redacted finding rather than a reported value.
func detectorAccesses(language, source string) ([]string, bool) {
	switch language {
	case "rust":
		return quotedCalls(source, []string{"std::env::var(", "std::env::var_os("})
	case "type-script":
		return typescriptAccesses(source)
	case "f-sharp":
		return quotedCalls(source, []string{"Environment.GetEnvironmentVariable("})
	case "go":
		return quotedCalls(source, []string{"os.Getenv(", "os.LookupEnv(", "os.LookupEnv,"})
	case "terraform":
		return quotedCalls(source, []string{"get_env(", "env("})
	case "ansible":
		return ansibleAccesses(source)
	default:
		return nil, false
	}
}

func quotedCalls(source string, markers []string) ([]string, bool) {
	var keys []string
	seen := map[string]bool{}
	dynamic := false
	for _, marker := range markers {
		remainder := source
		for {
			index := strings.Index(remainder, marker)
			if index < 0 {
				break
			}
			after := strings.TrimLeft(remainder[index+len(marker):], " \t\r\n")
			key, rest, ok := quotedValue(after)
			if !ok {
				dynamic = true
				if len(after) > 0 {
					remainder = after[1:]
				} else {
					remainder = ""
				}
				continue
			}
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
			remainder = rest
		}
	}
	sort.Strings(keys)
	return keys, dynamic
}

// quotedValue splits one quoted literal from the text after it.
func quotedValue(value string) (string, string, bool) {
	if len(value) < 1 || (value[0] != '\'' && value[0] != '"') {
		return "", value, false
	}
	quote := value[0]
	rest := value[1:]
	end := strings.IndexByte(rest, quote)
	if end < 0 {
		return "", value, false
	}
	return rest[:end], rest[end+1:], true
}

func typescriptAccesses(source string) ([]string, bool) {
	keys, dynamic := quotedCalls(source, []string{"process.env["})
	seen := map[string]bool{}
	for _, key := range keys {
		seen[key] = true
	}
	remainder := source
	for {
		index := strings.Index(remainder, "process.env.")
		if index < 0 {
			break
		}
		after := remainder[index+len("process.env."):]
		length := 0
		for length < len(after) && isKeyByte(after[length]) {
			length++
		}
		if length == 0 {
			dynamic = true
			if len(after) > 0 {
				remainder = after[1:]
			} else {
				remainder = ""
			}
			continue
		}
		key := after[:length]
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
		remainder = after[length:]
	}
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimLeft(line, " \t\r")
		colon := strings.IndexByte(trimmed, ':')
		if colon < 0 {
			continue
		}
		key := strings.TrimRight(trimmed[:colon], " \t\r")
		if key != "" && isUpperKey(key) && !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys, dynamic
}

func isKeyByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func isUpperKey(key string) bool {
	for i := 0; i < len(key); i++ {
		c := key[i]
		if !(c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}

func ansibleAccesses(source string) ([]string, bool) {
	var keys []string
	seen := map[string]bool{}
	dynamic := false
	remainder := source
	for {
		index := strings.Index(remainder, "lookup(")
		if index < 0 {
			break
		}
		after := strings.TrimLeft(remainder[index+len("lookup("):], " \t\r\n")
		lookup, rest, ok := quotedValue(after)
		if !ok {
			dynamic = true
			if len(after) > 0 {
				remainder = after[1:]
			} else {
				remainder = ""
			}
			continue
		}
		if lookup != "env" {
			remainder = rest
			continue
		}
		trimmed := strings.TrimLeft(rest, " \t\r\n")
		if !strings.HasPrefix(trimmed, ",") {
			dynamic = true
			remainder = trimmed
			continue
		}
		afterComma := strings.TrimLeft(trimmed[1:], " \t\r\n")
		key, rest, ok := quotedValue(afterComma)
		if !ok {
			dynamic = true
			remainder = afterComma
			continue
		}
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
		remainder = rest
	}
	sort.Strings(keys)
	return keys, dynamic
}

// environmentInit plans the declared example copies, and writes them only
// with explicit authorization. A plan never reaches the write boundary.
func environmentInit(document *config.Document, tree Tree, store EnvironmentStore, apply bool, format Format) Outcome {
	policy, refusal := decodeEnvironment(document)
	if refusal != nil {
		return refuseEnvironment(format, refusal)
	}
	if len(policy.examples) == 0 {
		return RefuseWith(format, domain.ConfigUndeclared, "environment policy declares no example targets")
	}
	files := make([]EnvironmentFile, 0, len(policy.examples))
	seen := map[string]bool{}
	for _, example := range policy.examples {
		if !isRelativeFile(example.source) || !isRelativeFile(example.target) {
			return RefuseWith(format, domain.PathEscapesRoot, "environment example leaves the repository root")
		}
		if example.source == example.target {
			return RefuseWith(format, domain.ConfigUnusable, "environment example source and target are the same")
		}
		if seen[example.target] {
			return RefuseWith(format, domain.ConfigUnusable,
				"environment target `"+example.target+"` is declared more than once")
		}
		seen[example.target] = true
		if tree.Exists(example.target) {
			return RefuseWith(format, domain.FileExists,
				"environment target `"+example.target+"` already exists")
		}
		contents, readErr := tree.Read(example.source)
		if readErr != nil {
			switch readErr.Kind {
			case "missing":
				return RefuseWith(format, domain.FileMissing,
					"environment example `"+example.source+"` is missing")
			default:
				return RefuseWith(format, domain.FileUnreadable,
					"environment example `"+example.source+"` is unreadable")
			}
		}
		files = append(files, EnvironmentFile{Path: example.target, Contents: contents})
	}
	if !apply {
		targets := make([]string, 0, len(files))
		for _, file := range files {
			targets = append(targets, file.Path)
		}
		if format == JSON {
			return Clean(fmt.Sprintf("{\"command\":\"environment-init\",\"count\":%d,\"schemaVersion\":1,"+
				"\"status\":\"planned\",\"targets\":[%s]}\n",
				len(files), quoteList(targets)))
		}
		return Clean(fmt.Sprintf("[environment-init] planned %d declared targets; pass --apply to write\n", len(files)))
	}
	if store == nil {
		return RefuseWith(format, domain.FileUnwritable,
			"environment initialization refused: this entry point has no environment write boundary")
	}
	written := make([]string, 0, len(files))
	for _, file := range files {
		written = append(written, file.Path)
	}
	if err := store.Create(tree.Root(), files); err != nil {
		return RefuseWith(format, domain.FileUnwritable,
			"environment initialization refused: "+err.Error())
	}
	if format == JSON {
		return Clean(fmt.Sprintf("{\"command\":\"environment-init\",\"count\":%d,\"schemaVersion\":1,"+
			"\"status\":\"completed\",\"targets\":[%s]}\n",
			len(written), quoteList(written)))
	}
	return Clean("[environment-init] created declared targets\n")
}

// environmentBackup copies declared targets into an explicit destination.
// An explicitly empty policy is a valid zero-file plan.
func environmentBackup(document *config.Document, tree Tree, store EnvironmentStore,
	destination *string, format Format) Outcome {
	policy, refusal := decodeEnvironment(document)
	if refusal != nil {
		return refuseEnvironment(format, refusal)
	}
	if destination == nil {
		return RefuseWith(format, domain.ArgsIncomplete, "environment backup requires --dir")
	}
	if leavesRootPattern(*destination) || strings.TrimSpace(*destination) == "" {
		return RefuseWith(format, domain.PathEscapesRoot, "backup destination leaves the repository root")
	}
	files := make([]EnvironmentFile, 0, len(policy.examples))
	for _, example := range policy.examples {
		if !isRelativeFile(example.target) {
			return RefuseWith(format, domain.PathEscapesRoot, "environment example leaves the repository root")
		}
		contents, readErr := tree.Read(example.target)
		if readErr != nil {
			switch readErr.Kind {
			case "missing":
				return RefuseWith(format, domain.FileMissing,
					"environment target `"+example.target+"` is missing")
			default:
				return RefuseWith(format, domain.FileUnreadable,
					"environment target `"+example.target+"` is unreadable")
			}
		}
		files = append(files, EnvironmentFile{Path: backupPath(*destination, example.target), Contents: contents})
	}
	written := make([]string, 0, len(files))
	for _, file := range files {
		written = append(written, file.Path)
	}
	if len(files) > 0 {
		if store == nil {
			return RefuseWith(format, domain.FileUnwritable,
				"environment backup refused: this entry point has no environment write boundary")
		}
		if err := store.Create(tree.Root(), files); err != nil {
			return RefuseWith(format, domain.FileUnwritable,
				"environment backup refused: "+err.Error())
		}
	}
	if format == JSON {
		return Clean(fmt.Sprintf("{\"command\":\"environment-backup\",\"count\":%d,\"destination\":%s,"+
			"\"files\":[%s],\"schemaVersion\":1,\"status\":\"completed\"}\n",
			len(written), domain.Quote(*destination), quoteList(written)))
	}
	return Clean(fmt.Sprintf("[environment-backup] completed %d declared files into %s\n", len(written), *destination))
}

func backupPath(destination, target string) string {
	trimmed := strings.Trim(destination, "/")
	if trimmed == "" || trimmed == "." {
		return target
	}
	return trimmed + "/" + target
}

// environmentRestore copies declared targets back from an explicit backup
// directory. A conflicting target is never overwritten without --force.
func environmentRestore(document *config.Document, tree Tree, store EnvironmentStore,
	directory *string, force bool, format Format) Outcome {
	policy, refusal := decodeEnvironment(document)
	if refusal != nil {
		return refuseEnvironment(format, refusal)
	}
	if directory == nil {
		return RefuseWith(format, domain.ArgsIncomplete, "environment restore requires --dir")
	}
	if leavesRootPattern(*directory) || strings.TrimSpace(*directory) == "" {
		return RefuseWith(format, domain.PathEscapesRoot, "backup destination leaves the repository root")
	}
	files := make([]EnvironmentRestoreFile, 0, len(policy.examples))
	for _, example := range policy.examples {
		if !isRelativeFile(example.target) {
			return RefuseWith(format, domain.PathEscapesRoot, "environment example leaves the repository root")
		}
		source := backupPath(*directory, example.target)
		contents, readErr := tree.Read(source)
		if readErr != nil {
			switch readErr.Kind {
			case "missing":
				return RefuseWith(format, domain.FileMissing,
					"environment backup `"+source+"` is missing")
			default:
				return RefuseWith(format, domain.FileUnreadable,
					"environment backup `"+source+"` is unreadable")
			}
		}
		if tree.Exists(example.target) && !force {
			return RefuseWith(format, domain.FileExists,
				"environment target `"+example.target+"` already exists; use --force")
		}
		files = append(files, EnvironmentRestoreFile{Path: example.target, Contents: contents, Replace: force})
	}
	written := make([]string, 0, len(files))
	for _, file := range files {
		written = append(written, file.Path)
	}
	if store == nil {
		return RefuseWith(format, domain.FileUnwritable,
			"environment restore refused: this entry point has no environment write boundary")
	}
	if err := store.Restore(tree.Root(), files); err != nil {
		return RefuseWith(format, domain.FileUnwritable,
			"environment restore refused: "+err.Error())
	}
	if format == JSON {
		return Clean(fmt.Sprintf("{\"command\":\"environment-restore\",\"count\":%d,\"schemaVersion\":1,"+
			"\"status\":\"completed\",\"targets\":[%s]}\n",
			len(written), quoteList(written)))
	}
	return Clean("[environment-restore] restored declared targets\n")
}

func quoteList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, domain.Quote(value))
	}
	return strings.Join(quoted, ",")
}
