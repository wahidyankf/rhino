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
	// A workflow reference is a secrets or vars name. The leading class keeps
	// a longer dotted name, such as github.secrets.x, from reading as one.
	workflowReference = regexp.MustCompile(`(?:^|[^A-Za-z0-9_.])(secrets|vars)\.([A-Za-z_][A-Za-z0-9_]*)`)
)

// vaultFailure is a refusal the vault rules end the run with.
type vaultFailure struct {
	code    domain.ErrorCode
	message string
}

// vaultChecks runs the vault rules when a manifest is declared. An undeclared
// manifest checks nothing and changes nothing. It returns the findings, the
// number of declared files it inspected, and a refusal when a check could not run.
func vaultChecks(document *config.Document, tree Tree) ([]environmentFinding, int, *vaultFailure) {
	manifest, err := document.Vault()
	if err != nil {
		return nil, 0, &vaultFailure{code: domain.ConfigUnusable, message: "environment vault: " + err.Error()}
	}
	if manifest == nil {
		return nil, 0, nil
	}
	names, ok := tree.(gitNameTree)
	if !ok {
		return nil, 0, &vaultFailure{code: domain.RepositoryUnusable,
			message: "environment vault check refused: this tree has no Git name boundary"}
	}
	tracked, err := names.TrackedFiles()
	if err != nil {
		return nil, 0, &vaultFailure{code: domain.RepositoryUnusable,
			message: "environment vault check refused: " + err.Error()}
	}
	ignored, err := names.IgnoredFiles()
	if err != nil {
		return nil, 0, &vaultFailure{code: domain.RepositoryUnusable,
			message: "environment vault check refused: " + err.Error()}
	}
	rules := &vaultRules{
		manifest: manifest,
		tree:     tree,
		tracked:  tracked,
		ignored:  ignored,
		found:    map[environmentFinding]bool{},
		names:    map[string]bool{},
	}
	rules.inspected = len(manifest.Entries) + len(manifest.Templates)
	rules.checkManifest()
	rules.checkItems()
	rules.checkIgnored()
	rules.checkTemplates()
	if failure := rules.checkGitHub(); failure != nil {
		return nil, 0, failure
	}
	findings := make([]environmentFinding, 0, len(rules.found))
	for finding := range rules.found {
		findings = append(findings, finding)
	}
	return findings, rules.inspected, nil
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
	for index, template := range m.Templates {
		if !isExactPath(template.Path) {
			r.invalid(fmt.Sprintf("environment.vault.templates[%d]", index))
		}
	}
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

// checkEntry applies the rules of one entry's class and form.
func (r *vaultRules) checkEntry(label string, entry config.VaultEntry) {
	switch entry.Class {
	case "vault":
		r.checkVaultEntry(label, entry)
	case "derived", "generated":
		if entry.Path == "" || !validGlob(entry.Path) || entry.Reason == "" || entry.By == "" {
			r.invalid(label)
		}
	case "machine-local":
		if entry.Path == "" || !validGlob(entry.Path) || entry.Reason == "" {
			r.invalid(label)
		}
	default:
		r.invalid(label)
	}
}

// checkVaultEntry applies the rules of a vault entry: an exact path for a file
// or an env file, no path for a resident item, and unique keys in an env entry.
func (r *vaultRules) checkVaultEntry(label string, entry config.VaultEntry) {
	switch entry.Form {
	case "env":
		if !isExactPath(entry.Path) || entry.App == "" || entry.Env == "" || len(entry.Keys) == 0 || hasInvalidKey(entry.Keys) {
			r.invalid(label)
		}
	case "file":
		if !isExactPath(entry.Path) || entry.App == "" || entry.Env == "" || entry.Secret == "" {
			r.invalid(label)
		}
	case "resident":
		if entry.Path != "" || entry.App == "" || entry.Env == "" || entry.Secret == "" {
			r.invalid(label)
		}
	default:
		r.invalid(label)
	}
}

// checkGitHubDeclarations applies the shape rules of the GitHub block: every
// listed secret names a source and every listed name is non-empty.
func (r *vaultRules) checkGitHubDeclarations() {
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
		if !strings.HasSuffix(path, ".example") || r.templated(path) || r.claimed(path) {
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
	listedSecrets := map[string]bool{}
	for _, name := range github.Builtin {
		listedSecrets[name] = true
	}
	for _, secret := range github.Secrets {
		listedSecrets[secret.Name] = true
	}
	listedVariables := map[string]bool{}
	for _, variable := range github.Variables {
		listedVariables[variable.Name] = true
	}
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
		for _, match := range workflowReference.FindAllStringSubmatch(text, -1) {
			kind, name := match[1], match[2]
			listed := (kind == "secrets" && listedSecrets[name]) || (kind == "vars" && listedVariables[name])
			if !listed {
				r.flag(ruleVaultGitHubUnlistedReference, path, kind+"."+name)
			}
		}
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
	if !strings.ContainsAny(declared, "*?[") {
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

// isPrefix accepts a two-segment namespace of kebab-case segments, such as a
// namespace and a repository name.
func isPrefix(prefix string) bool {
	segments := strings.Split(prefix, "/")
	if len(segments) != 2 {
		return false
	}
	return kebabSegment.MatchString(segments[0]) && kebabSegment.MatchString(segments[1])
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
