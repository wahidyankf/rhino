package config_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/config"
)

// vaultDocument wraps one environment.vault body in a grouped configuration.
func vaultDocument(t *testing.T, vault string) *config.Document {
	t.Helper()
	document, err := config.Parse("schema: rhino/repo-config/v2\nenvironment:\n  vault:\n" + vault)
	if err != nil {
		t.Fatalf("configuration refused: %v", err)
	}
	return document
}

func TestVaultIsUndeclaredWithoutTheKey(t *testing.T) {
	cases := map[string]string{
		"no environment group": "schema: rhino/repo-config/v2\n",
		"empty environment":    "schema: rhino/repo-config/v2\nenvironment: {}\n",
		"null vault":           "schema: rhino/repo-config/v2\nenvironment:\n  vault:\n",
		"scalar environment":   "schema: rhino/repo-config/v2\nenvironment: text\n",
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			document, err := config.Parse(text)
			if err != nil {
				t.Fatalf("configuration refused: %v", err)
			}
			manifest, err := document.Vault()
			if err != nil || manifest != nil {
				t.Fatalf("Vault() = %v, %v; want undeclared", manifest, err)
			}
		})
	}
}

func TestVaultDecodesEveryContractField(t *testing.T) {
	document := vaultDocument(t, `
          provider: bitwarden
          server: https://vault.example.com
          prefix: org/example-repo
          environments: [local, prod]
          patterns: ["**/.env*"]
          entries:
            - path: infra/example/.env.prod
              class: vault
              form: env
              app: infra-example
              env: prod
              mode: "0600"
              keys:
                - TAILSCALE_AUTH_KEY
                - { key: CLOUDFLARE_API_TOKEN, app: infra-edge, secret: edge-dns, field: password }
            - path: infra/example/terraform/terraform.tfvars
              class: vault
              form: file
              app: infra-terraform
              env: prod
              secret: tfvars
            - class: vault
              form: resident
              app: ose-example-app
              env: stag
              secret: cosign-password
              consumer: scripts/example-wrapper.mjs
            - path: infra/example/ansible/inventory.ini
              class: derived
              reason: Rendered from addresses.
              by: infra/example/terraform/outputs.tf
          templates:
            - path: infra/example/ansible/.env.example
              for: infra/example/.env.prod
          github:
            workflows: [".github/workflows/*.yml"]
            builtin: [GITHUB_TOKEN]
            secrets:
              - name: EXAMPLE_COSIGN_PASSWORD
                source: { app: ose-example-app, env: stag, secret: cosign-password }
            variables:
              - name: EXAMPLE_LIVE_ENABLED
                note: Plain switch.
`)
	manifest, err := document.Vault()
	if err != nil {
		t.Fatalf("Vault() refused: %v", err)
	}
	want := &config.VaultManifest{
		Provider:     "bitwarden",
		Server:       "https://vault.example.com",
		Prefix:       "org/example-repo",
		Environments: []string{"local", "prod"},
		Patterns:     []string{"**/.env*"},
		Entries: []config.VaultEntry{
			{
				Path: "infra/example/.env.prod", Class: "vault", Form: "env", App: "infra-example", Env: "prod", Mode: "0600",
				Keys: []config.VaultKey{
					{Key: "TAILSCALE_AUTH_KEY"},
					{Key: "CLOUDFLARE_API_TOKEN", App: "infra-edge", Secret: "edge-dns", Field: "password"},
				},
			},
			{Path: "infra/example/terraform/terraform.tfvars", Class: "vault", Form: "file", App: "infra-terraform", Env: "prod", Secret: "tfvars"},
			{Class: "vault", Form: "resident", App: "ose-example-app", Env: "stag", Secret: "cosign-password", Consumer: "scripts/example-wrapper.mjs"},
			{Path: "infra/example/ansible/inventory.ini", Class: "derived", Reason: "Rendered from addresses.", By: "infra/example/terraform/outputs.tf"},
		},
		Templates: []config.VaultTemplate{{Path: "infra/example/ansible/.env.example", For: "infra/example/.env.prod"}},
		GitHub: &config.VaultGitHub{
			Workflows: []string{".github/workflows/*.yml"},
			Builtin:   []string{"GITHUB_TOKEN"},
			Secrets: []config.VaultGitHubSecret{{
				Name:   "EXAMPLE_COSIGN_PASSWORD",
				Source: config.VaultSource{App: "ose-example-app", Env: "stag", Secret: "cosign-password"},
			}},
			Variables: []config.VaultGitHubVariable{{Name: "EXAMPLE_LIVE_ENABLED", Note: "Plain switch."}},
		},
	}
	if !reflect.DeepEqual(manifest, want) {
		t.Fatalf("Vault() =\n%#v\nwant\n%#v", manifest, want)
	}
}

func TestVaultReadsAnEmptyGitHubGroupAsDeclared(t *testing.T) {
	manifest, err := vaultDocument(t, "          provider: bitwarden\n          github: {}\n").Vault()
	if err != nil {
		t.Fatalf("Vault() refused: %v", err)
	}
	if manifest.GitHub == nil || len(manifest.GitHub.Secrets) != 0 {
		t.Fatalf("github group = %#v; want a declared empty group", manifest.GitHub)
	}
}

func TestVaultReadsAMissingFieldAsEmpty(t *testing.T) {
	manifest, err := vaultDocument(t, "          provider:\n          entries:\n            - class: vault\n              form: env\n              keys: [A]\n").Vault()
	if err != nil {
		t.Fatalf("Vault() refused: %v", err)
	}
	if manifest.Provider != "" || manifest.GitHub != nil || manifest.Entries[0].Path != "" {
		t.Fatalf("manifest = %#v; want empty fields and no GitHub group", manifest)
	}
}

// TestVaultRefusesAScalarInPlaceOfTheMapping proves a vault written as text is
// refused as a mapping, and the refusal does not print the text.
func TestVaultRefusesAScalarInPlaceOfTheMapping(t *testing.T) {
	document, err := config.Parse("schema: rhino/repo-config/v2\nenvironment:\n  vault: marker-7f3a\n")
	if err != nil {
		t.Fatalf("configuration refused before the vault decode: %v", err)
	}
	_, err = document.Vault()
	if err == nil || !strings.Contains(err.Error(), "environment.vault must be a mapping") {
		t.Fatalf("Vault() error = %v, want a mapping refusal", err)
	}
	if strings.Contains(err.Error(), "marker-7f3a") {
		t.Fatalf("refusal printed a value: %v", err)
	}
}

// TestVaultRefusesWrongShapes proves each shape a decode cannot read is a
// refusal, and that the refusal names the field and never a value.
func TestVaultRefusesWrongShapes(t *testing.T) {
	cases := []struct {
		name  string
		vault string
		want  string
	}{
		{"unknown top key", "          nope: x\n", "environment.vault names unknown key `nope`"},
		{"provider is a list", "          provider: [marker-7f3a]\n", "environment.vault.provider must be a string"},
		{"environments is text", "          environments: local\n", "environment.vault.environments must be a list"},
		{"environments holds a number", "          environments: [1]\n", "environment.vault.environments must be a list of strings"},
		{"patterns holds a map", "          patterns: [{a: b}]\n", "environment.vault.patterns must be a list of strings"},
		{"entries is text", "          entries: x\n", "environment.vault.entries must be a list"},
		{"entry is text", "          entries: [x]\n", "environment.vault.entries[0] must be a mapping"},
		{"entry unknown key", "          entries:\n            - path: a\n              nope: x\n", "environment.vault.entries[0] names unknown key `nope`"},
		{"entry path is a list", "          entries:\n            - path: [a]\n", "environment.vault.entries[0].path must be a string"},
		{"keys is text", "          entries:\n            - keys: x\n", "environment.vault.entries[0].keys must be a list"},
		{"key is a number", "          entries:\n            - keys: [1]\n", "environment.vault.entries[0].keys[0] must be a key name or a mapping"},
		{"key mapping unknown key", "          entries:\n            - keys: [{key: A, nope: x}]\n", "environment.vault.entries[0].keys[0] names unknown key `nope`"},
		{"key mapping holds a list", "          entries:\n            - keys: [{key: [A]}]\n", "environment.vault.entries[0].keys[0].key must be a string"},
		{"templates is text", "          templates: x\n", "environment.vault.templates must be a list"},
		{"template is text", "          templates: [x]\n", "environment.vault.templates[0] must be a mapping"},
		{"template unknown key", "          templates: [{path: a, nope: x}]\n", "environment.vault.templates[0] names unknown key `nope`"},
		{"template path is a list", "          templates: [{path: [a]}]\n", "environment.vault.templates[0].path must be a string"},
		{"github is text", "          github: x\n", "environment.vault.github must be a mapping"},
		{"github unknown key", "          github: {nope: x}\n", "environment.vault.github names unknown key `nope`"},
		{"github workflows is text", "          github: {workflows: x}\n", "environment.vault.github.workflows must be a list"},
		{"github builtin holds a map", "          github: {builtin: [{a: b}]}\n", "environment.vault.github.builtin must be a list of strings"},
		{"github secrets is text", "          github: {secrets: x}\n", "environment.vault.github.secrets must be a list"},
		{"github secret is text", "          github: {secrets: [x]}\n", "environment.vault.github.secrets[0] must be a mapping"},
		{"github secret unknown key", "          github: {secrets: [{name: A, nope: x}]}\n", "environment.vault.github.secrets[0] names unknown key `nope`"},
		{"github secret name is a list", "          github: {secrets: [{name: [A]}]}\n", "environment.vault.github.secrets[0].name must be a string"},
		{"source is text", "          github: {secrets: [{name: A, source: x}]}\n", "environment.vault.github.secrets[0].source must be a mapping"},
		{"source unknown key", "          github: {secrets: [{name: A, source: {nope: x}}]}\n", "environment.vault.github.secrets[0].source names unknown key `nope`"},
		{"variables is text", "          github: {variables: x}\n", "environment.vault.github.variables must be a list"},
		{"variable is text", "          github: {variables: [x]}\n", "environment.vault.github.variables[0] must be a mapping"},
		{"variable unknown key", "          github: {variables: [{name: A, nope: x}]}\n", "environment.vault.github.variables[0] names unknown key `nope`"},
		{"variable note is a list", "          github: {variables: [{name: A, note: [x]}]}\n", "environment.vault.github.variables[0].note must be a string"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document, err := config.Parse("schema: rhino/repo-config/v2\nenvironment:\n  vault:\n" + tc.vault)
			if err != nil {
				t.Fatalf("configuration refused before the vault decode: %v", err)
			}
			_, err = document.Vault()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Vault() error = %v, want it to contain %q", err, tc.want)
			}
			if strings.Contains(err.Error(), "marker-7f3a") {
				t.Fatalf("refusal printed a value: %v", err)
			}
		})
	}
}
