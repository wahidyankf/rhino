package config

import (
	"fmt"
	"sort"
)

// VaultManifest is the decoded environment.vault group. Decoding checks shape
// only: a mapping where a mapping belongs, a string or a list of strings where
// those belong, and no key the contract does not name. The rules that make a
// manifest valid, and every check that reads Git, belong to the validator,
// which reports a broken rule as a finding and never as a refusal.
type VaultManifest struct {
	Provider     string
	Server       string
	Prefix       string
	Environments []string
	Patterns     []string
	Entries      []VaultEntry
	Templates    []VaultTemplate
	GitHub       *VaultGitHub
}

// VaultEntry is one classification: a file whose secrets live in the vault, or
// a file another tool derives, generates, or keeps machine-local.
type VaultEntry struct {
	Path     string
	Class    string
	Form     string
	App      string
	Env      string
	Secret   string
	Mode     string
	Reason   string
	By       string
	Consumer string
	Keys     []VaultKey
}

// VaultKey is one key of an env entry and its optional per-key overrides.
type VaultKey struct {
	Key    string
	App    string
	Secret string
	Field  string
}

// VaultTemplate names one tracked template and the entry it stands for.
type VaultTemplate struct {
	Path string
	For  string
}

// VaultGitHub declares the workflow references a repository may hold.
type VaultGitHub struct {
	Workflows []string
	Builtin   []string
	Secrets   []VaultGitHubSecret
	Variables []VaultGitHubVariable
}

// VaultGitHubSecret is one listed GitHub secret and the vault item it comes from.
type VaultGitHubSecret struct {
	Name   string
	Source VaultSource
}

// VaultSource names a vault item by the parts its name is built from.
type VaultSource struct {
	App    string
	Env    string
	Secret string
}

// VaultGitHubVariable is one listed GitHub variable.
type VaultGitHubVariable struct {
	Name string
	Note string
}

// Vault decodes environment.vault. An absent or null key is undeclared, so a
// repository without a vault manifest is validated exactly as before.
func (d *Document) Vault() (*VaultManifest, error) {
	environment, ok := d.Environment()
	if !ok {
		return nil, nil
	}
	raw, present := environment["vault"]
	if !present || raw == nil {
		return nil, nil
	}
	return decodeVault(raw)
}

func decodeVault(raw any) (*VaultManifest, error) {
	fields, err := newVaultFields(raw, "environment.vault",
		"provider", "server", "prefix", "environments", "patterns", "entries", "templates", "github")
	if err != nil {
		return nil, err
	}
	manifest := &VaultManifest{
		Provider:     fields.text("provider"),
		Server:       fields.text("server"),
		Prefix:       fields.text("prefix"),
		Environments: fields.texts("environments"),
		Patterns:     fields.texts("patterns"),
	}
	if manifest.Entries, err = decodeVaultEntries(fields); err != nil {
		return nil, err
	}
	if manifest.Templates, err = decodeVaultTemplates(fields); err != nil {
		return nil, err
	}
	if raw, present := fields.present("github"); present {
		if manifest.GitHub, err = decodeVaultGitHub(raw); err != nil {
			return nil, err
		}
	}
	if err := fields.failure(); err != nil {
		return nil, err
	}
	return manifest, nil
}

func decodeVaultEntries(parent *vaultFields) ([]VaultEntry, error) {
	items, err := parent.items("entries")
	if err != nil {
		return nil, err
	}
	entries := make([]VaultEntry, 0, len(items))
	for index, item := range items {
		path := fmt.Sprintf("environment.vault.entries[%d]", index)
		fields, err := newVaultFields(item, path, "path", "class", "form", "app", "env", "secret",
			"mode", "reason", "by", "consumer", "keys")
		if err != nil {
			return nil, err
		}
		entry := VaultEntry{
			Path: fields.text("path"), Class: fields.text("class"), Form: fields.text("form"),
			App: fields.text("app"), Env: fields.text("env"), Secret: fields.text("secret"),
			Mode: fields.text("mode"), Reason: fields.text("reason"), By: fields.text("by"),
			Consumer: fields.text("consumer"),
		}
		if entry.Keys, err = decodeVaultKeys(fields, path); err != nil {
			return nil, err
		}
		if err := fields.failure(); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// decodeVaultKeys reads an entry's keys: each is a bare key name, or a mapping
// that may override the app, the secret, and the hidden field of that one key.
func decodeVaultKeys(parent *vaultFields, path string) ([]VaultKey, error) {
	items, err := parent.items("keys")
	if err != nil {
		return nil, err
	}
	var keys []VaultKey
	for index, item := range items {
		if name, ok := item.(string); ok {
			keys = append(keys, VaultKey{Key: name})
			continue
		}
		keyPath := fmt.Sprintf("%s.keys[%d]", path, index)
		if _, isMapping := item.(map[string]any); !isMapping {
			return nil, fmt.Errorf("%s must be a key name or a mapping", keyPath)
		}
		fields, err := newVaultFields(item, keyPath, "key", "app", "secret", "field")
		if err != nil {
			return nil, err
		}
		keys = append(keys, VaultKey{
			Key: fields.text("key"), App: fields.text("app"),
			Secret: fields.text("secret"), Field: fields.text("field"),
		})
		if err := fields.failure(); err != nil {
			return nil, err
		}
	}
	return keys, nil
}

func decodeVaultTemplates(parent *vaultFields) ([]VaultTemplate, error) {
	items, err := parent.items("templates")
	if err != nil {
		return nil, err
	}
	templates := make([]VaultTemplate, 0, len(items))
	for index, item := range items {
		fields, err := newVaultFields(item, fmt.Sprintf("environment.vault.templates[%d]", index), "path", "for")
		if err != nil {
			return nil, err
		}
		templates = append(templates, VaultTemplate{Path: fields.text("path"), For: fields.text("for")})
		if err := fields.failure(); err != nil {
			return nil, err
		}
	}
	return templates, nil
}

func decodeVaultGitHub(raw any) (*VaultGitHub, error) {
	fields, err := newVaultFields(raw, "environment.vault.github", "workflows", "builtin", "secrets", "variables")
	if err != nil {
		return nil, err
	}
	github := &VaultGitHub{Workflows: fields.texts("workflows"), Builtin: fields.texts("builtin")}
	if github.Secrets, err = decodeVaultGitHubSecrets(fields); err != nil {
		return nil, err
	}
	if github.Variables, err = decodeVaultGitHubVariables(fields); err != nil {
		return nil, err
	}
	if err := fields.failure(); err != nil {
		return nil, err
	}
	return github, nil
}

func decodeVaultGitHubSecrets(parent *vaultFields) ([]VaultGitHubSecret, error) {
	items, err := parent.items("secrets")
	if err != nil {
		return nil, err
	}
	secrets := make([]VaultGitHubSecret, 0, len(items))
	for index, item := range items {
		path := fmt.Sprintf("environment.vault.github.secrets[%d]", index)
		fields, err := newVaultFields(item, path, "name", "source")
		if err != nil {
			return nil, err
		}
		secret := VaultGitHubSecret{Name: fields.text("name")}
		if raw, present := fields.present("source"); present {
			if secret.Source, err = decodeVaultSource(raw, path+".source"); err != nil {
				return nil, err
			}
		}
		if err := fields.failure(); err != nil {
			return nil, err
		}
		secrets = append(secrets, secret)
	}
	return secrets, nil
}

func decodeVaultSource(raw any, path string) (VaultSource, error) {
	fields, err := newVaultFields(raw, path, "app", "env", "secret")
	if err != nil {
		return VaultSource{}, err
	}
	source := VaultSource{App: fields.text("app"), Env: fields.text("env"), Secret: fields.text("secret")}
	return source, fields.failure()
}

func decodeVaultGitHubVariables(parent *vaultFields) ([]VaultGitHubVariable, error) {
	items, err := parent.items("variables")
	if err != nil {
		return nil, err
	}
	variables := make([]VaultGitHubVariable, 0, len(items))
	for index, item := range items {
		fields, err := newVaultFields(item, fmt.Sprintf("environment.vault.github.variables[%d]", index), "name", "note")
		if err != nil {
			return nil, err
		}
		variables = append(variables, VaultGitHubVariable{Name: fields.text("name"), Note: fields.text("note")})
		if err := fields.failure(); err != nil {
			return nil, err
		}
	}
	return variables, nil
}

// vaultFields reads one mapping of the vault manifest. The first wrong shape
// is kept and returned by failure, so each field reads in one expression, and
// the message names the field and never its value.
type vaultFields struct {
	path    string
	mapping map[string]any
	err     error
}

// newVaultFields accepts a mapping that holds only the named keys. An unknown
// key refuses: a policy the tool silently ignored would be a check that never ran.
func newVaultFields(raw any, path string, allowed ...string) (*vaultFields, error) {
	mapping, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a mapping", path)
	}
	permitted := make(map[string]bool, len(allowed))
	for _, key := range allowed {
		permitted[key] = true
	}
	var unknown []string
	for key := range mapping {
		if !permitted[key] {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("%s names unknown key `%s`", path, unknown[0])
	}
	return &vaultFields{path: path, mapping: mapping}, nil
}

// failure returns the first wrong shape a read found, or nil.
func (f *vaultFields) failure() error { return f.err }

// present returns a key's value when the key is set to something other than null.
func (f *vaultFields) present(key string) (any, bool) {
	raw, ok := f.mapping[key]
	return raw, ok && raw != nil
}

func (f *vaultFields) fail(key, shape string) {
	if f.err == nil {
		f.err = fmt.Errorf("%s.%s must be %s", f.path, key, shape)
	}
}

// text reads a string, or the empty string when the key is absent or null.
func (f *vaultFields) text(key string) string {
	raw, ok := f.present(key)
	if !ok {
		return ""
	}
	value, isText := raw.(string)
	if !isText {
		f.fail(key, "a string")
	}
	return value
}

// texts reads a list of strings, or nil when the key is absent or null.
func (f *vaultFields) texts(key string) []string {
	items, err := f.items(key)
	if err != nil {
		return nil
	}
	var values []string
	for _, item := range items {
		value, isText := item.(string)
		if !isText {
			f.fail(key, "a list of strings")
			return nil
		}
		values = append(values, value)
	}
	return values
}

// items reads a list, or nil when the key is absent or null.
func (f *vaultFields) items(key string) ([]any, error) {
	raw, ok := f.present(key)
	if !ok {
		return nil, nil
	}
	items, isList := raw.([]any)
	if !isList {
		f.fail(key, "a list")
		return nil, f.err
	}
	return items, nil
}
