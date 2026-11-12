// Environment vault rules: the checks that keep a repository's vault manifest
// honest, under environment.vault.
//
// Every rule is network-free and reads names only. The names come from two
// read-only Git queries, the tracked paths and the ignored paths. The one
// file body a rule reads is a tracked workflow, and only for its secret and
// variable references. No ignored file is ever opened.
package application

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// The six vault rule names are public once released, so they are fixed here.
const (
	ruleVaultManifestInvalid         = "vault-manifest-invalid"
	ruleVaultNameGrammar             = "vault-name-grammar"
	ruleVaultUnclassifiedIgnoredFile = "vault-unclassified-ignored-file"
	ruleVaultUnclassifiedTemplate    = "vault-unclassified-template"
	ruleVaultGitHubUnlistedReference = "vault-github-unlisted-reference"
	ruleVaultGitHubSourceUnknown     = "vault-github-source-unknown"
)

// vaultProvider is the only provider the first release names.
const vaultProvider = "bitwarden"

// gitNameTree is the read-only Git name boundary the vault rules hold: the
// paths the index tracks and the paths Git ignores, as names only.
type gitNameTree interface {
	TrackedFiles() ([]string, error)
	IgnoredFiles() ([]string, error)
}

var (
	kebabSegment = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	// vaultFileMode is an octal file mode, written quoted in the manifest.
	vaultFileMode = regexp.MustCompile(`^0[0-7]{3}$`)
	// workflowExpression is one ${{ }} expression of a workflow.
	workflowExpression = regexp.MustCompile(`(?s)\$\{\{(.*?)\}\}`)
	// workflowIf is a bare if: value. GitHub reads it as an expression with no ${{ }}.
	workflowIf = regexp.MustCompile(`(?m)^\s*(?:-\s+)?if:[ \t]*(.+?)[ \t]*$`)
	// workflowName is a key after a dot, or the text inside a quoted bracket key.
	workflowName = regexp.MustCompile(`^[A-Za-z0-9_-]+`)
	// workflowBracketKey is a quoted key in a bracket index, such as secrets['X'].
	// Group 1 is the single-quoted form, group 2 the double-quoted form.
	workflowBracketKey = regexp.MustCompile(`^\[\s*(?:'([A-Za-z0-9_-]+)'|"([A-Za-z0-9_-]+)")\s*\]`)
	// inheritedSecrets is a job that passes every secret on: a block key, or an entry of a flow mapping.
	inheritedSecrets = regexp.MustCompile(`(?m)^\s*(?:-\s+)?secrets:[ \t]*inherit[ \t]*$|[{,]\s*secrets:\s*inherit\s*[,}]`)
)

// vaultFailure is a refusal the vault rules end the run with.
type vaultFailure struct {
	code    domain.ErrorCode
	message string
}

// vaultChecks runs the vault rules when a manifest is declared. An undeclared
// manifest checks nothing and changes nothing. It returns the findings, the
// number of declared files it inspected, and a refusal when a check could not run.
// The phases run in order: decode the manifest, read the Git names, check the
// manifest, derive and check the item names, check the ignored files and the
// templates, then read the workflows.
func vaultChecks(document *config.Document, tree Tree) ([]environmentFinding, int, *vaultFailure) {
	manifest, failure := vaultManifest(document)
	if failure != nil || manifest == nil {
		return nil, 0, failure
	}
	rules, failure := newVaultRules(manifest, tree)
	if failure != nil {
		return nil, 0, failure
	}
	rules.checkManifest()
	rules.checkItems()
	rules.checkIgnored()
	rules.checkTemplates()
	if failure := rules.checkGitHub(); failure != nil {
		return nil, 0, failure
	}
	return rules.findings(), rules.inspected, nil
}

// vaultManifest decodes environment.vault. A manifest that does not decode is a
// refusal, not a finding.
func vaultManifest(document *config.Document) (*config.VaultManifest, *vaultFailure) {
	manifest, err := document.Vault()
	if err != nil {
		return nil, &vaultFailure{code: domain.ConfigUnusable, message: "environment vault: " + err.Error()}
	}
	return manifest, nil
}

// newVaultRules reads the tracked and ignored names once. A tree without a Git
// name boundary, or a name query that fails, refuses the check.
func newVaultRules(manifest *config.VaultManifest, tree Tree) (*vaultRules, *vaultFailure) {
	names, ok := tree.(gitNameTree)
	if !ok {
		return nil, nameRefusal("this tree has no Git name boundary")
	}
	tracked, err := names.TrackedFiles()
	if err != nil {
		return nil, nameRefusal(err.Error())
	}
	ignored, err := names.IgnoredFiles()
	if err != nil {
		return nil, nameRefusal(err.Error())
	}
	return &vaultRules{
		manifest:  manifest,
		tree:      tree,
		tracked:   tracked,
		ignored:   ignored,
		found:     map[environmentFinding]bool{},
		names:     map[string]bool{},
		inspected: len(manifest.Entries) + len(manifest.Templates),
	}, nil
}

// nameRefusal is the refusal for a check that cannot read the Git names it needs.
func nameRefusal(reason string) *vaultFailure {
	return &vaultFailure{code: domain.RepositoryUnusable, message: "environment vault check refused: " + reason}
}

// findings returns the findings the run collected, in no particular order.
func (r *vaultRules) findings() []environmentFinding {
	out := make([]environmentFinding, 0, len(r.found))
	for finding := range r.found {
		out = append(out, finding)
	}
	return out
}

// vaultRules holds one run's manifest, its Git names, and the findings so far.
// Findings are a set, so a rule that fires twice for one path reports it once.
type vaultRules struct {
	manifest  *config.VaultManifest
	tree      Tree
	tracked   []string
	ignored   []string
	found     map[environmentFinding]bool
	names     map[string]bool
	inspected int
}

func (r *vaultRules) flag(rule, path, key string) {
	r.found[environmentFinding{rule: rule, path: path, key: key, hasKey: key != ""}] = true
}

func (r *vaultRules) invalid(path string) {
	r.flag(ruleVaultManifestInvalid, path, "")
}

// checkManifest applies the shape rules that do not depend on Git: the
// provider, the server, the prefix, the environment set, every entry, and every
// template and GitHub declaration.
func (r *vaultRules) checkManifest() {
	m := r.manifest
	if m.Provider != vaultProvider {
		r.invalid("environment.vault.provider")
	}
	if !strings.HasPrefix(m.Server, "https://") {
		r.invalid("environment.vault.server")
	}
	if !isPrefix(m.Prefix) {
		r.invalid("environment.vault.prefix")
	}
	if !isDistinctKebab(m.Environments) {
		r.invalid("environment.vault.environments")
	}
	for _, pattern := range m.Patterns {
		if !validGlob(pattern) {
			r.flag(ruleVaultManifestInvalid, "environment.vault.patterns", pattern)
		}
	}
	claims := map[string]int{}
	for index, entry := range m.Entries {
		label := entryLabel(index, entry)
		r.checkEntry(label, entry)
		if entry.Path != "" {
			claims[entry.Path]++
		}
	}
	for path, count := range claims {
		if count > 1 {
			r.invalid(path)
		}
	}
	r.checkGlobOverlaps()
	r.checkTemplateRows()
	if m.GitHub != nil {
		r.checkGitHubDeclarations()
	}
}

// entryLabel names an entry in a finding: its path, its consumer, or its index.
func entryLabel(index int, entry config.VaultEntry) string {
	switch {
	case entry.Path != "":
		return entry.Path
	case entry.Consumer != "":
		return entry.Consumer
	default:
		return fmt.Sprintf("environment.vault.entries[%d]", index)
	}
}

// checkEntry applies the rules of one entry's class and form, and the file mode
// when one is declared. Each class refuses every field it never reads.
func (r *vaultRules) checkEntry(label string, entry config.VaultEntry) {
	if entry.Mode != "" && !vaultFileMode.MatchString(entry.Mode) {
		r.invalid(label)
	}
	switch entry.Class {
	case "vault":
		r.checkVaultEntry(label, entry)
	case "derived", "generated":
		if entry.Path == "" || !validGlob(entry.Path) || entry.Reason == "" || entry.By == "" || !onlyFields(entry, "path", "reason", "by") {
			r.invalid(label)
		}
	case "machine-local":
		if entry.Path == "" || !validGlob(entry.Path) || entry.Reason == "" || !onlyFields(entry, "path", "reason") {
			r.invalid(label)
		}
	default:
		r.invalid(label)
	}
}

// entryFields maps each manifest field of an entry to whether the entry sets it.
func entryFields(entry config.VaultEntry) map[string]bool {
	return map[string]bool{
		"path":     entry.Path != "",
		"form":     entry.Form != "",
		"app":      entry.App != "",
		"env":      entry.Env != "",
		"secret":   entry.Secret != "",
		"keys":     len(entry.Keys) > 0,
		"mode":     entry.Mode != "",
		"reason":   entry.Reason != "",
		"by":       entry.By != "",
		"consumer": entry.Consumer != "",
	}
}

// onlyFields reports whether an entry sets no field outside allowed. A class
// never reads the fields it does not name, so the manifest refuses them rather
// than ignore them silently.
func onlyFields(entry config.VaultEntry, allowed ...string) bool {
	permitted := map[string]bool{}
	for _, name := range allowed {
		permitted[name] = true
	}
	for name, set := range entryFields(entry) {
		if set && !permitted[name] {
			return false
		}
	}
	return true
}

// checkVaultEntry applies the rules of a vault entry: an exact path for a file
// or an env file, no path for a resident item, unique keys in an env entry, and
// only the fields its form reads.
func (r *vaultRules) checkVaultEntry(label string, entry config.VaultEntry) {
	switch entry.Form {
	case "env":
		// An env entry reads its items from keys, so an entry-level secret would be ignored.
		if !isExactPath(entry.Path) || entry.App == "" || entry.Env == "" || len(entry.Keys) == 0 ||
			hasInvalidKey(entry.Keys) || !onlyFields(entry, "path", "form", "app", "env", "keys", "mode") {
			r.invalid(label)
		}
	case "file":
		// A file entry names one secret, so keys would be ignored.
		if !isExactPath(entry.Path) || entry.App == "" || entry.Env == "" || entry.Secret == "" ||
			!onlyFields(entry, "path", "form", "app", "env", "secret", "mode") {
			r.invalid(label)
		}
	case "resident":
		if entry.App == "" || entry.Env == "" || entry.Secret == "" ||
			!onlyFields(entry, "form", "app", "env", "secret", "consumer") {
			r.invalid(label)
		}
	default:
		r.invalid(label)
	}
}

// checkGitHubDeclarations applies the shape rules of the GitHub block: every
// listed secret names a source and every listed name is non-empty.
func (r *vaultRules) checkGitHubDeclarations() {
	for _, glob := range r.manifest.GitHub.Workflows {
		if !validGlob(glob) {
			r.flag(ruleVaultManifestInvalid, "github.workflows", glob)
		}
	}
	for _, secret := range r.manifest.GitHub.Secrets {
		if secret.Name == "" || secret.Source.App == "" || secret.Source.Env == "" || secret.Source.Secret == "" {
			r.flag(ruleVaultManifestInvalid, "github.secrets", secret.Name)
		}
	}
	for _, variable := range r.manifest.GitHub.Variables {
		if variable.Name == "" {
			r.invalid("github.variables")
		}
	}
}

// checkItems derives every vault item name from the entries and checks it
// against the grammar: the closed environment set and kebab-case segments. A
// name two entries both derive is reported as a second claim on one name.
func (r *vaultRules) checkItems() {
	for index, entry := range r.manifest.Entries {
		if entry.Class != "vault" {
			continue
		}
		label := entryLabel(index, entry)
		switch entry.Form {
		case "env":
			for _, key := range entry.Keys {
				if key.Key == "" {
					continue
				}
				segment := key.Secret
				if segment == "" {
					segment = deriveSecretSegment(key.Key)
				}
				app := key.App
				if app == "" {
					app = entry.App
				}
				r.item(label, app, entry.Env, segment)
			}
		case "file", "resident":
			r.item(label, entry.App, entry.Env, entry.Secret)
		}
	}
}

// item checks one derived name. An entry missing a segment is a manifest
// finding already, so a name is derived only when every segment is present.
func (r *vaultRules) item(label, app, env, secret string) {
	if app == "" || env == "" || secret == "" {
		return
	}
	name := itemName(r.manifest.Prefix, app, env, secret)
	if r.names[name] {
		r.flag(ruleVaultManifestInvalid, label, name)
	}
	r.names[name] = true
	if !kebabSegment.MatchString(app) || !kebabSegment.MatchString(secret) || !kebabSegment.MatchString(env) ||
		!contains(r.manifest.Environments, env) {
		r.flag(ruleVaultNameGrammar, label, name)
	}
}

// checkIgnored reports every ignored file a declared pattern matches when no
// entry classifies it. Only the name is read; the file is never opened.
func (r *vaultRules) checkIgnored() {
	for _, path := range r.ignored {
		if matchAny(r.manifest.Patterns, path) && !r.claimed(path) {
			r.flag(ruleVaultUnclassifiedIgnoredFile, path, "")
		}
	}
}

// checkTemplates reports every tracked example template that neither a
// templates row nor an entry names.
func (r *vaultRules) checkTemplates() {
	for _, path := range r.tracked {
		if !strings.HasSuffix(path, ".example") || r.templated(path) || r.namesExactly(path) {
			continue
		}
		r.flag(ruleVaultUnclassifiedTemplate, path, "")
	}
}

// checkGitHub reads each tracked workflow the manifest names and reports every
// secret or variable it references that the manifest does not list. It then
// checks that every listed secret's source names a derived item.
func (r *vaultRules) checkGitHub() *vaultFailure {
	github := r.manifest.GitHub
	if github == nil {
		return nil
	}
	listedSecrets, listedVariables := listedNames(github)
	for _, path := range r.tracked {
		if !matchAny(github.Workflows, path) {
			continue
		}
		text, readErr := r.tree.Read(path)
		if readErr != nil {
			return &vaultFailure{code: domain.FileUnreadable,
				message: "environment vault workflow `" + path + "` could not be read: " + readErr.Reason}
		}
		r.inspected++
		r.checkWorkflow(path, text, listedSecrets, listedVariables)
	}
	for _, secret := range github.Secrets {
		source := secret.Source
		if source.App == "" || source.Env == "" || source.Secret == "" {
			continue
		}
		if !r.names[itemName(r.manifest.Prefix, source.App, source.Env, source.Secret)] {
			r.flag(ruleVaultGitHubSourceUnknown, "github.secrets", secret.Name)
		}
	}
	return nil
}

// claimed reports whether an entry's path, exact or a glob, covers path.
func (r *vaultRules) claimed(path string) bool {
	for _, entry := range r.manifest.Entries {
		if entry.Path != "" && claims(entry.Path, path) {
			return true
		}
	}
	return false
}

// templated reports whether a templates row names path exactly.
func (r *vaultRules) templated(path string) bool {
	for _, template := range r.manifest.Templates {
		if template.Path == path {
			return true
		}
	}
	return false
}

// claims reports whether one declared path covers path: an exact match, or a
// match of the declared glob.
func claims(declared, path string) bool {
	if !isGlob(declared) {
		return declared == path
	}
	ok, err := Match(declared, path)
	return err == nil && ok
}

// validGlob reports whether a declared pattern compiles and stays inside the repository.
func validGlob(pattern string) bool {
	_, err := Match(pattern, pattern)
	return err == nil
}

// isPrefix accepts a namespace of one or more kebab-case segments, such as
// `org/example-repo` or `example-repo`. How many segments a prefix carries is
// the manifest's choice, so the rule does not fix a count.
func isPrefix(prefix string) bool {
	for _, segment := range strings.Split(prefix, "/") {
		if !kebabSegment.MatchString(segment) {
			return false
		}
	}
	return true
}

// isDistinctKebab accepts a non-empty list of unique kebab-case names.
func isDistinctKebab(values []string) bool {
	if len(values) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, value := range values {
		if !kebabSegment.MatchString(value) || seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}

// hasInvalidKey reports whether one env entry names a key blank or declares a
// key twice. Either one leaves no single item per key.
func hasInvalidKey(keys []config.VaultKey) bool {
	seen := map[string]bool{}
	for _, key := range keys {
		if key.Key == "" || seen[key.Key] {
			return true
		}
		seen[key.Key] = true
	}
	return false
}

// deriveSecretSegment is the default secret segment of an env key: the key,
// lowercased, with each underscore written as a hyphen.
func deriveSecretSegment(key string) string {
	return strings.ReplaceAll(strings.ToLower(key), "_", "-")
}

// itemName joins the parts of a vault item name, `<prefix>/<app>/<env>/<secret>`.
func itemName(prefix, app, env, secret string) string {
	return prefix + "/" + app + "/" + env + "/" + secret
}

// isGlob reports whether a declared path is a pattern rather than one exact path.
func isGlob(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

// namesExactly reports whether an entry names path exactly. A glob names the
// files it matches, but a tracked template is named by its own path, so a broad
// glob cannot swallow it.
func (r *vaultRules) namesExactly(path string) bool {
	for _, entry := range r.manifest.Entries {
		if entry.Path == path {
			return true
		}
	}
	return false
}

// checkGlobOverlaps reports an exact entry path that another entry's glob
// covers, so one file would carry two classifications. Two globs that overlap
// each other are not compared; that limit is documented in findings.md.
func (r *vaultRules) checkGlobOverlaps() {
	for _, exact := range r.manifest.Entries {
		if exact.Path == "" || isGlob(exact.Path) {
			continue
		}
		for _, glob := range r.manifest.Entries {
			if glob.Path != "" && isGlob(glob.Path) && claims(glob.Path, exact.Path) {
				r.invalid(exact.Path)
			}
		}
	}
}

// checkTemplateRows checks each templates row: its path is exact, and its for
// names the exact path of an entry.
func (r *vaultRules) checkTemplateRows() {
	entries := map[string]bool{}
	for _, entry := range r.manifest.Entries {
		if entry.Path != "" {
			entries[entry.Path] = true
		}
	}
	for index, template := range r.manifest.Templates {
		if !isExactPath(template.Path) || !isExactPath(template.For) || !entries[template.For] {
			r.invalid(templateLabel(index, template))
		}
	}
}

// templateLabel names a templates row in a finding: its path when the path is
// exact, otherwise its index.
func templateLabel(index int, template config.VaultTemplate) string {
	if isExactPath(template.Path) {
		return template.Path
	}
	return fmt.Sprintf("environment.vault.templates[%d]", index)
}

// listedNames returns the secret and variable names the GitHub block lists,
// with the built-in secrets included.
func listedNames(github *config.VaultGitHub) (map[string]bool, map[string]bool) {
	secrets := map[string]bool{}
	for _, name := range github.Builtin {
		secrets[strings.ToLower(name)] = true
	}
	for _, secret := range github.Secrets {
		secrets[strings.ToLower(secret.Name)] = true
	}
	variables := map[string]bool{}
	for _, variable := range github.Variables {
		variables[strings.ToLower(variable.Name)] = true
	}
	return secrets, variables
}

// checkWorkflow reports, for one tracked workflow, every secrets or vars read the
// manifest does not list. The scanned surface is each ${{ }} expression and each
// bare if: value; the rest of the text, including comments and free text, is not
// read. A read that cannot resolve to one name is reported too: a dynamic index,
// a whole-context read, or an inherited secret set.
func (r *vaultRules) checkWorkflow(path, text string, listedSecrets, listedVariables map[string]bool) {
	for _, body := range workflowBodies(text) {
		for _, read := range workflowReads(body) {
			if read.name == "" || !listedRead(read, listedSecrets, listedVariables) {
				r.flag(ruleVaultGitHubUnlistedReference, path, read.label())
			}
		}
	}
	if inheritedSecrets.MatchString(text) {
		r.flag(ruleVaultGitHubUnlistedReference, path, "secrets: inherit")
	}
}

// listedRead reports whether a literal read names a listed secret or variable.
// Names compare case-insensitively, the safe direction while GitHub's case rule
// for context names is unverified; findings.md records the assumption.
func listedRead(read workflowRead, listedSecrets, listedVariables map[string]bool) bool {
	name := strings.ToLower(read.name)
	if read.kind == "secrets" {
		return listedSecrets[name]
	}
	return listedVariables[name]
}

// workflowRead is one read an expression makes of the secrets or vars context.
// A literal read names its key; a dynamic read indexes with a non-literal
// expression; a whole-context read takes the context itself.
type workflowRead struct {
	kind    string
	name    string
	dynamic bool
}

// label names a read in a finding.
func (read workflowRead) label() string {
	switch {
	case read.name != "":
		return read.kind + "." + read.name
	case read.dynamic:
		return read.kind + "[dynamic]"
	default:
		return read.kind + " (whole context)"
	}
}

// workflowBodies returns the expression bodies a workflow text holds: each
// ${{ }} expression and each bare if: value.
func workflowBodies(text string) []string {
	var bodies []string
	for _, match := range workflowExpression.FindAllStringSubmatch(text, -1) {
		bodies = append(bodies, match[1])
	}
	for _, match := range workflowIf.FindAllStringSubmatch(text, -1) {
		bodies = append(bodies, match[1])
	}
	return bodies
}

// workflowReads returns every read an expression body makes of the secrets or
// vars context. Quoted strings are skipped, and only whole words count, so a
// context named inside a string or inside a longer word is not a read. A word
// after a dot is a property of another value, not a context.
func workflowReads(body string) []workflowRead {
	var reads []workflowRead
	for i := 0; i < len(body); {
		if body[i] == '\'' {
			i = skipQuoted(body, i)
			continue
		}
		if !isWordByte(body[i]) {
			i++
			continue
		}
		start := i
		for i < len(body) && isWordByte(body[i]) {
			i++
		}
		kind := strings.ToLower(body[start:i])
		if (kind != "secrets" && kind != "vars") || (start > 0 && body[start-1] == '.') {
			continue
		}
		reads = append(reads, contextRead(kind, body[i:]))
	}
	return reads
}

// contextRead classifies the text after a secrets or vars word: a dotted key,
// a quoted bracket key, a dynamic bracket index, or nothing at all.
func contextRead(kind, rest string) workflowRead {
	if strings.HasPrefix(rest, ".") {
		if name := workflowName.FindString(rest[1:]); name != "" {
			return workflowRead{kind: kind, name: name}
		}
	}
	if match := workflowBracketKey.FindStringSubmatch(rest); match != nil {
		return workflowRead{kind: kind, name: match[1] + match[2]}
	}
	if strings.HasPrefix(rest, "[") {
		return workflowRead{kind: kind, dynamic: true}
	}
	return workflowRead{kind: kind}
}

// skipQuoted returns the index just past the single-quoted string that opens at
// i. A doubled quote inside the string is an escaped quote. An unterminated
// string runs to the end of the body.
func skipQuoted(body string, i int) int {
	for j := i + 1; j < len(body); j++ {
		if body[j] != '\'' {
			continue
		}
		if j+1 < len(body) && body[j+1] == '\'' {
			j++
			continue
		}
		return j + 1
	}
	return len(body)
}

// isWordByte reports whether a byte can be part of a GitHub expression word.
func isWordByte(c byte) bool {
	return c == '_' || c == '-' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
