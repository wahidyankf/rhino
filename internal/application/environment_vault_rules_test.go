package application_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// vaultBase is a valid manifest with one env entry. Each case changes one part
// of it, so a case reads as the single defect it proves.
const vaultBase = `schema: rhino/repo-config/v2
environment:
  vault:
    provider: bitwarden
    server: https://vault.example.com
    prefix: org/example-repo
    environments: [local, stag, canary, prod]
    entries:
      - path: infra/example/.env.prod
        class: vault
        form: env
        app: infra-example
        env: prod
        keys: [EXAMPLE_TOKEN]
`

// vaultReplace returns vaultBase with one fragment replaced. A fragment that is
// not in the base is a defect in the test table, so the table panics at once.
func vaultReplace(old, replacement string) string {
	if !strings.Contains(vaultBase, old) {
		panic("vault fixture has no fragment " + old)
	}
	return strings.Replace(vaultBase, old, replacement, 1)
}

// vaultCase is one defect or one clean shape, with what it must print.
type vaultCase struct {
	name       string
	config     string
	tracked    map[string]string
	ignored    map[string]string
	unreadable []string
	binary     []string
	wrap       func(*vaultMemoryTree) application.Tree
	exit       int
	stderr     []string
}

// trackedFailureTree answers the tracked-name query with an error.
type trackedFailureTree struct{ *vaultMemoryTree }

func (trackedFailureTree) TrackedFiles() ([]string, error) {
	return nil, errGitQuery
}

// ignoredFailureTree answers the ignored-name query with an error.
type ignoredFailureTree struct{ *vaultMemoryTree }

func (ignoredFailureTree) IgnoredFiles() ([]string, error) {
	return nil, errGitQuery
}

var errGitQuery = errors.New("git query failed")

// withoutNameBoundary is the plain in-memory tree, which has no Git name query.
func withoutNameBoundary(m *vaultMemoryTree) application.Tree { return m.memoryTree }

// workflowsGlob declares the GitHub block with one workflow glob and one listed secret.
const workflowsGlob = `    github:
      workflows: [".github/workflows/*.yml"]
      builtin: [GITHUB_TOKEN]
      secrets:
        - name: EXAMPLE_TOKEN
          source: { app: infra-example, env: prod, secret: example-token }
`

// workflowsWith declares the GitHub block with one workflow glob and no listed
// names, so a case proves the glob alone.
func workflowsWith(glob string) string {
	return vaultReplace("    entries:\n", "    github:\n      workflows: [\""+glob+"\"]\n    entries:\n")
}

func vaultCases() []vaultCase {
	entry := func(extra string) string { return vaultBase + extra }
	return []vaultCase{
		// Manifest shape, one defect at a time.
		{name: "provider other than the first release", config: vaultReplace("provider: bitwarden", "provider: aws"), exit: 1,
			stderr: []string{"environment.vault.provider: vault-manifest-invalid"}},
		{name: "server without https", config: vaultReplace("https://vault.example.com", "http://vault.example.com"), exit: 1,
			stderr: []string{"environment.vault.server: vault-manifest-invalid"}},
		{name: "prefix of one segment is accepted", config: vaultReplace("org/example-repo", "example-repo"), exit: 0},
		{name: "prefix of three segments is accepted", config: vaultReplace("org/example-repo", "org/team/example-repo"), exit: 0},
		{name: "prefix with an empty segment", config: vaultReplace("org/example-repo", "org//example-repo"), exit: 1,
			stderr: []string{"environment.vault.prefix: vault-manifest-invalid"}},
		{name: "prefix with a trailing slash", config: vaultReplace("org/example-repo", "org/example-repo/"), exit: 1,
			stderr: []string{"environment.vault.prefix: vault-manifest-invalid"}},
		{name: "prefix segment that is not kebab-case", config: vaultReplace("org/example-repo", "Org/example-repo"), exit: 1,
			stderr: []string{"environment.vault.prefix: vault-manifest-invalid"}},
		{name: "environment set with a duplicate", config: vaultReplace("[local, stag, canary, prod]", "[prod, prod]"), exit: 1,
			stderr: []string{"environment.vault.environments: vault-manifest-invalid"}},
		{name: "empty environment set", config: vaultReplace("[local, stag, canary, prod]", "[]"), exit: 1,
			stderr: []string{"environment.vault.environments: vault-manifest-invalid"}},
		{name: "environment name that is not kebab-case", config: vaultReplace("[local, stag, canary, prod]", "[Prod]"), exit: 1,
			stderr: []string{"environment.vault.environments: vault-manifest-invalid"}},
		{name: "pattern that is absolute", config: vaultReplace("    entries:\n", "    patterns: [\"/abs/**\"]\n    entries:\n"), exit: 1,
			stderr: []string{"environment.vault.patterns: vault-manifest-invalid (/abs/**)"}},

		// Entries.
		{name: "entry class that is unknown", config: vaultReplace("class: vault\n        form: env", "class: other\n        form: env"), exit: 1,
			stderr: []string{"infra/example/.env.prod: vault-manifest-invalid"}},
		{name: "vault entry form that is unknown", config: vaultReplace("form: env", "form: json"), exit: 1,
			stderr: []string{"infra/example/.env.prod: vault-manifest-invalid"}},
		{name: "env entry without keys", config: vaultReplace("keys: [EXAMPLE_TOKEN]", "keys: []"), exit: 1,
			stderr: []string{"infra/example/.env.prod: vault-manifest-invalid"}},
		{name: "env entry with a blank key", config: vaultReplace("keys: [EXAMPLE_TOKEN]", "keys: [{app: infra-example}]"), exit: 1,
			stderr: []string{"infra/example/.env.prod: vault-manifest-invalid"}},
		{name: "env entry with a duplicate key", config: vaultReplace("keys: [EXAMPLE_TOKEN]", "keys: [EXAMPLE_TOKEN, EXAMPLE_TOKEN]"), exit: 1,
			stderr: []string{"infra/example/.env.prod: vault-manifest-invalid"}},
		{name: "env entry whose path is a glob", config: vaultReplace("path: infra/example/.env.prod", "path: infra/*/.env.prod"), exit: 1,
			stderr: []string{"infra/*/.env.prod: vault-manifest-invalid"}},
		{name: "env entry without an app", config: vaultReplace("        app: infra-example\n", ""), exit: 1,
			stderr: []string{"infra/example/.env.prod: vault-manifest-invalid"}},
		{name: "key override whose secret is not kebab-case", config: vaultReplace("keys: [EXAMPLE_TOKEN]", "keys: [{key: EXAMPLE_TOKEN, secret: Example_Token}]"), exit: 1,
			stderr: []string{"infra/example/.env.prod: vault-name-grammar (org/example-repo/infra-example/prod/Example_Token)"}},
		{name: "two entries claim one path", config: entry("      - path: infra/example/.env.prod\n        class: derived\n        reason: r\n        by: t\n"), exit: 1,
			stderr: []string{"infra/example/.env.prod: vault-manifest-invalid"}},
		{name: "file entry without a secret", config: entry("      - path: infra/example/terraform/terraform.tfvars\n        class: vault\n        form: file\n        app: infra-terraform\n        env: prod\n"), exit: 1,
			stderr: []string{"infra/example/terraform/terraform.tfvars: vault-manifest-invalid"}},
		{name: "file entry whose secret is not kebab-case", config: entry("      - path: infra/example/terraform/terraform.tfvars\n        class: vault\n        form: file\n        app: infra-terraform\n        env: prod\n        secret: Tfvars\n"), exit: 1,
			stderr: []string{"infra/example/terraform/terraform.tfvars: vault-name-grammar (org/example-repo/infra-terraform/prod/Tfvars)"}},
		{name: "resident entry with a path", config: entry("      - path: infra/example/resident.env\n        class: vault\n        form: resident\n        app: infra-example\n        env: prod\n        secret: resident-token\n"), exit: 1,
			stderr: []string{"infra/example/resident.env: vault-manifest-invalid"}},
		{name: "resident entry without an app or consumer", config: entry("      - class: vault\n        form: resident\n        env: prod\n        secret: resident-token\n"), exit: 1,
			stderr: []string{"environment.vault.entries[1]: vault-manifest-invalid"}},
		{name: "resident entry with a consumer and no path", config: entry("      - class: vault\n        form: resident\n        app: infra-example\n        env: prod\n        secret: resident-token\n        consumer: scripts/wrapper.mjs\n"), exit: 0},
		{name: "derived entry without a reason", config: entry("      - path: apps/example/gen.json\n        class: derived\n        by: tool\n"), exit: 1,
			stderr: []string{"apps/example/gen.json: vault-manifest-invalid"}},
		{name: "derived entry without a by", config: entry("      - path: apps/example/gen.json\n        class: derived\n        reason: r\n"), exit: 1,
			stderr: []string{"apps/example/gen.json: vault-manifest-invalid"}},
		{name: "derived entry whose path climbs out of the repository", config: entry("      - path: ../gen.json\n        class: derived\n        reason: r\n        by: t\n"), exit: 1,
			stderr: []string{"../gen.json: vault-manifest-invalid"}},
		{name: "machine-local entry without a reason", config: entry("      - path: hippo.local.json\n        class: machine-local\n"), exit: 1,
			stderr: []string{"hippo.local.json: vault-manifest-invalid"}},
		{name: "machine-local entry with a reason", config: entry("      - path: hippo.local.json\n        class: machine-local\n        reason: Capacity differs per machine.\n"), exit: 0},
		{name: "two derived names are the same", config: entry("      - path: infra/example/other.env\n        class: vault\n        form: env\n        app: infra-example\n        env: prod\n        keys: [EXAMPLE_TOKEN]\n"), exit: 1,
			stderr: []string{"infra/example/other.env: vault-manifest-invalid (org/example-repo/infra-example/prod/example-token)"}},
		{name: "app segment that is not kebab-case", config: vaultReplace("app: infra-example", "app: Infra_Example"), exit: 1,
			stderr: []string{"infra/example/.env.prod: vault-name-grammar (org/example-repo/Infra_Example/prod/example-token)"}},
		{name: "entry without environment that derives no name", config: vaultReplace("        env: prod\n", ""), exit: 1,
			stderr: []string{"infra/example/.env.prod: vault-manifest-invalid"}},
		{name: "mode that is not an octal file mode", config: vaultReplace("keys: [EXAMPLE_TOKEN]", "keys: [EXAMPLE_TOKEN]\n        mode: \"0999\""), exit: 1,
			stderr: []string{"infra/example/.env.prod: vault-manifest-invalid"}},
		{name: "mode that is an octal file mode", config: vaultReplace("keys: [EXAMPLE_TOKEN]", "keys: [EXAMPLE_TOKEN]\n        mode: \"0600\""), exit: 0},
		{name: "env entry that names a secret its form ignores", config: vaultReplace("keys: [EXAMPLE_TOKEN]", "secret: unused\n        keys: [EXAMPLE_TOKEN]"), exit: 1,
			stderr: []string{"infra/example/.env.prod: vault-manifest-invalid"}},
		{name: "file entry that names keys its form ignores", config: entry("      - path: infra/example/terraform/terraform.tfvars\n        class: vault\n        form: file\n        app: infra-terraform\n        env: prod\n        secret: tfvars\n        keys: [EXAMPLE_TOKEN]\n"), exit: 1,
			stderr: []string{"infra/example/terraform/terraform.tfvars: vault-manifest-invalid"}},
		{name: "derived entry that names an app its class ignores", config: entry("      - path: apps/example/gen.json\n        class: derived\n        reason: r\n        by: t\n        app: infra-example\n"), exit: 1,
			stderr: []string{"apps/example/gen.json: vault-manifest-invalid"}},
		{name: "exact entry path covered by another entry's glob", config: entry("      - path: infra/example/.env.*\n        class: derived\n        reason: r\n        by: t\n"), exit: 1,
			stderr: []string{"infra/example/.env.prod: vault-manifest-invalid"}},

		// Templates.
		{name: "template row whose path is a glob", config: vaultBase + "    templates:\n      - { path: infra/*/.env.example, for: infra/example/.env.prod }\n", exit: 1,
			stderr: []string{"environment.vault.templates[0]: vault-manifest-invalid"}},
		{name: "tracked example claimed by an entry", config: entry("      - path: infra/example/config.example\n        class: derived\n        reason: r\n        by: t\n"),
			tracked: map[string]string{"infra/example/config.example": "x=1\n"}, exit: 0},
		{name: "tracked example claimed only by a glob entry", config: entry("      - path: infra/example/*.example\n        class: derived\n        reason: r\n        by: t\n"),
			tracked: map[string]string{"infra/example/config.example": "x=1\n"}, exit: 1,
			stderr: []string{"infra/example/config.example: vault-unclassified-template"}},
		{name: "templates row for an entry that exists is accepted", config: vaultBase + "    templates:\n      - { path: infra/example/.env.example, for: infra/example/.env.prod }\n",
			tracked: map[string]string{"infra/example/.env.example": "EXAMPLE_TOKEN=\n"}, exit: 0},
		{name: "templates row for an entry that does not exist", config: vaultBase + "    templates:\n      - { path: infra/example/.env.example, for: infra/example/.env.stag }\n",
			tracked: map[string]string{"infra/example/.env.example": "EXAMPLE_TOKEN=\n"}, exit: 1,
			stderr: []string{"infra/example/.env.example: vault-manifest-invalid"}},

		// Ignored files.
		{name: "ignored file claimed by a glob entry", config: vaultReplace("    entries:\n", "    patterns: [\"**/.secrets/**\"]\n    entries:\n") + "      - path: apps/example/.secrets/*\n        class: generated\n        reason: r\n        by: t\n",
			ignored: map[string]string{"apps/example/.secrets/postgres-password": "marker\n"}, exit: 0},
		{name: "ignored file that no pattern matches", config: vaultReplace("    entries:\n", "    patterns: [\"**/.env*\"]\n    entries:\n"),
			ignored: map[string]string{"build/output.log": "log\n"}, exit: 0},
		{name: "ignored file with no pattern declared", config: vaultBase,
			ignored: map[string]string{"apps/example/.env.dev": "marker\n"}, exit: 0},
		{name: "ignored file whose claiming glob is malformed", config: vaultReplace("    entries:\n", "    patterns: [\"**/.env*\"]\n    entries:\n") + "      - path: apps/[\n        class: derived\n        reason: r\n        by: t\n",
			ignored: map[string]string{"apps/.env.dev": "marker\n"}, exit: 1,
			stderr: []string{"apps/[: vault-manifest-invalid", "apps/.env.dev: vault-unclassified-ignored-file"}},
		{name: "ignored file claimed by an exact entry path", config: vaultReplace("    entries:\n", "    patterns: [\"**/.env*\"]\n    entries:\n"),
			ignored: map[string]string{"infra/example/.env.prod": "marker\n"}, exit: 0},

		// An ignored file is named and never read: an unreadable ignored file and an
		// unreadable tracked template are both still reported as unclassified, where a
		// read of either would refuse the run with exit 2.
		{name: "unreadable ignored and template files are named, never read",
			config:     vaultReplace("    entries:\n", "    patterns: [\"**/.env*\"]\n    entries:\n"),
			tracked:    map[string]string{"infra/example/.env.example": "EXAMPLE_TOKEN=\n"},
			ignored:    map[string]string{"apps/example/.env.dev": "EXAMPLE_TOKEN=placeholder\n"},
			unreadable: []string{"apps/example/.env.dev", "infra/example/.env.example"}, exit: 1,
			stderr: []string{"apps/example/.env.dev: vault-unclassified-ignored-file", "infra/example/.env.example: vault-unclassified-template"}},

		// GitHub workflows.
		{name: "workflow that holds an unlisted secret", config: vaultReplace("    entries:\n", workflowsGlob+"    entries:\n"),
			tracked: map[string]string{".github/workflows/ci.yml": "run: x ${{ secrets.OTHER }}\n"}, exit: 1,
			stderr: []string{".github/workflows/ci.yml: vault-github-unlisted-reference (secrets.OTHER)"}},
		{name: "dotted name is not a workflow reference", config: vaultReplace("    entries:\n", workflowsGlob+"    entries:\n"),
			tracked: map[string]string{".github/workflows/ci.yml": "run: github.secrets.OTHER\n"}, exit: 0},
		{name: "workflow outside the declared globs is not read", config: vaultReplace("    entries:\n", workflowsGlob+"    entries:\n"),
			tracked: map[string]string{"scripts/ci.yml": "run: x ${{ secrets.OTHER }}\n"}, exit: 0},
		{name: "workflow that cannot be read refuses", config: vaultReplace("    entries:\n", workflowsGlob+"    entries:\n"),
			tracked: map[string]string{".github/workflows/ci.yml": "x\n"}, unreadable: []string{".github/workflows/ci.yml"}, exit: 2,
			stderr: []string{"environment vault workflow `.github/workflows/ci.yml` could not be read"}},
		{name: "workflow that holds no text refuses", config: vaultReplace("    entries:\n", workflowsGlob+"    entries:\n"),
			tracked: map[string]string{".github/workflows/ci.yml": "\x00"}, binary: []string{".github/workflows/ci.yml"}, exit: 2,
			stderr: []string{"holds no text"}},
		{name: "GitHub secret whose source names no derived item", config: vaultReplace("    entries:\n", "    github:\n      workflows: [\".github/workflows/*.yml\"]\n      secrets:\n        - name: EXAMPLE_TOKEN\n          source: { app: infra-example, env: prod, secret: missing }\n    entries:\n"), exit: 1,
			stderr: []string{"github.secrets: vault-github-source-unknown (EXAMPLE_TOKEN)"}},
		{name: "GitHub secret without a source", config: vaultReplace("    entries:\n", "    github:\n      secrets:\n        - name: EXAMPLE_TOKEN\n    entries:\n"), exit: 1,
			stderr: []string{"github.secrets: vault-manifest-invalid (EXAMPLE_TOKEN)"}},
		{name: "GitHub secret without a name", config: vaultReplace("    entries:\n", "    github:\n      secrets:\n        - source: { app: infra-example, env: prod, secret: example-token }\n    entries:\n"), exit: 1,
			stderr: []string{"github.secrets: vault-manifest-invalid"}},
		{name: "GitHub variable without a name", config: vaultReplace("    entries:\n", "    github:\n      variables:\n        - note: x\n    entries:\n"), exit: 1,
			stderr: []string{"github.variables: vault-manifest-invalid"}},
		{name: "GitHub block that is empty", config: vaultReplace("    entries:\n", "    github: {}\n    entries:\n"), exit: 0},
		{name: "bracket reference with double quotes that is unlisted", config: vaultReplace("    entries:\n", workflowsGlob+"    entries:\n"),
			tracked: map[string]string{".github/workflows/ci.yml": "run: x ${{ secrets[\"OTHER\"] }}\n"}, exit: 1,
			stderr: []string{".github/workflows/ci.yml: vault-github-unlisted-reference (secrets.OTHER)"}},
		{name: "bracket reference with double quotes that is listed", config: vaultReplace("    entries:\n", workflowsGlob+"    entries:\n"),
			tracked: map[string]string{".github/workflows/ci.yml": "run: x ${{ secrets[\"EXAMPLE_TOKEN\"] }}\n"}, exit: 0},
		{name: "dynamic vars index is reported", config: vaultReplace("    entries:\n", workflowsGlob+"    entries:\n"),
			tracked: map[string]string{".github/workflows/ci.yml": "run: x ${{ vars[inputs.name] }}\n"}, exit: 1,
			stderr: []string{".github/workflows/ci.yml: vault-github-unlisted-reference (vars[dynamic])"}},
		{name: "whole-context read is reported", config: vaultReplace("    entries:\n", workflowsGlob+"    entries:\n"),
			tracked: map[string]string{".github/workflows/ci.yml": "run: x ${{ toJSON(vars) }}\n"}, exit: 1,
			stderr: []string{".github/workflows/ci.yml: vault-github-unlisted-reference (vars (whole context))"}},
		{name: "secrets inherit is reported", config: vaultReplace("    entries:\n", workflowsGlob+"    entries:\n"),
			tracked: map[string]string{".github/workflows/ci.yml": "    secrets: inherit\n"}, exit: 1,
			stderr: []string{".github/workflows/ci.yml: vault-github-unlisted-reference (secrets: inherit)"}},
		{name: "a commented-out reference is reported", config: vaultReplace("    entries:\n", workflowsGlob+"    entries:\n"),
			tracked: map[string]string{".github/workflows/ci.yml": "# run: echo ${{ secrets.OLD }}\n"}, exit: 1,
			stderr: []string{".github/workflows/ci.yml: vault-github-unlisted-reference (secrets.OLD)"}},

		// Workflow globs, scanned surfaces, and field refusals (round three).
		{name: "workflow glob that is absolute", config: workflowsWith("/abs/*.yml"), exit: 1,
			stderr: []string{"github.workflows: vault-manifest-invalid (/abs/*.yml)"}},
		{name: "workflow glob that climbs out of the repository", config: workflowsWith("../x.yml"), exit: 1,
			stderr: []string{"github.workflows: vault-manifest-invalid (../x.yml)"}},
		{name: "workflow glob with an unterminated class", config: workflowsWith("[bad"), exit: 1,
			stderr: []string{"github.workflows: vault-manifest-invalid ([bad)"}},
		{name: "template for that is a glob", config: entry("      - path: infra/*/gen.json\n        class: derived\n        reason: r\n        by: t\n") +
			"    templates:\n      - { path: infra/example/.env.example, for: infra/*/gen.json }\n",
			tracked: map[string]string{"infra/example/.env.example": "EXAMPLE_TOKEN=\n"}, exit: 1,
			stderr: []string{"infra/example/.env.example: vault-manifest-invalid"}},
		{name: "vault env entry that names a reason", config: vaultReplace("keys: [EXAMPLE_TOKEN]", "keys: [EXAMPLE_TOKEN]\n        reason: r"), exit: 1,
			stderr: []string{"infra/example/.env.prod: vault-manifest-invalid"}},
		{name: "vault file entry that names a consumer", config: entry("      - path: infra/example/terraform/terraform.tfvars\n        class: vault\n        form: file\n        app: infra-terraform\n        env: prod\n        secret: tfvars\n        consumer: scripts/x.mjs\n"), exit: 1,
			stderr: []string{"infra/example/terraform/terraform.tfvars: vault-manifest-invalid"}},
		{name: "vault file entry with a mode", config: entry("      - path: infra/example/terraform/terraform.tfvars\n        class: vault\n        form: file\n        app: infra-terraform\n        env: prod\n        secret: tfvars\n        mode: \"0600\"\n"), exit: 0},
		{name: "resident entry with a mode", config: entry("      - class: vault\n        form: resident\n        app: infra-example\n        env: prod\n        secret: resident-token\n        mode: \"0600\"\n"), exit: 1,
			stderr: []string{"environment.vault.entries[1]: vault-manifest-invalid"}},
		{name: "derived entry with a mode", config: entry("      - path: apps/example/gen.json\n        class: derived\n        reason: r\n        by: t\n        mode: \"0600\"\n"), exit: 1,
			stderr: []string{"apps/example/gen.json: vault-manifest-invalid"}},
		{name: "derived entry with a consumer", config: entry("      - path: apps/example/gen.json\n        class: derived\n        reason: r\n        by: t\n        consumer: x.mjs\n"), exit: 1,
			stderr: []string{"apps/example/gen.json: vault-manifest-invalid"}},
		{name: "machine-local entry that names a by", config: entry("      - path: hippo.local.json\n        class: machine-local\n        reason: r\n        by: t\n"), exit: 1,
			stderr: []string{"hippo.local.json: vault-manifest-invalid"}},

		// Workflow scanning: expressions only, whole words only, case-insensitive.
		{name: "string literal that names a context is not a reference", config: vaultReplace("    entries:\n", workflowsGlob+"    entries:\n"),
			tracked: map[string]string{".github/workflows/ci.yml": "run: echo ${{ format('org/secrets-repo') }}\n"}, exit: 0},
		{name: "context word inside a string literal is not a whole-context read", config: vaultReplace("    entries:\n", workflowsGlob+"    entries:\n"),
			tracked: map[string]string{".github/workflows/ci.yml": "run: echo ${{ contains(inputs.x, 'vars') }}\n"}, exit: 0},
		{name: "a context word with a hyphen is not a context", config: vaultReplace("    entries:\n", workflowsGlob+"    entries:\n"),
			tracked: map[string]string{".github/workflows/ci.yml": "run: echo ${{ inputs.secrets-repo }}\n"}, exit: 0},
		{name: "a quoted literal inside an expression is not a reference", config: vaultReplace("    entries:\n", workflowsGlob+"    entries:\n"),
			tracked: map[string]string{".github/workflows/ci.yml": "run: echo ${{ 'secrets.OTHER' }}\n"}, exit: 0},
		{name: "free text outside an expression is not read", config: vaultReplace("    entries:\n", workflowsGlob+"    entries:\n"),
			tracked: map[string]string{".github/workflows/ci.yml": "run: cat docs/secrets.md && node -e @vars.yml\n"}, exit: 0},
		{name: "a context name in another case is reported", config: vaultReplace("    entries:\n", workflowsGlob+"    entries:\n"),
			tracked: map[string]string{".github/workflows/ci.yml": "run: echo ${{ SECRETS.OTHER }}\n"}, exit: 1,
			stderr: []string{".github/workflows/ci.yml: vault-github-unlisted-reference (secrets.OTHER)"}},
		{name: "a listed name in another case is listed", config: vaultReplace("    entries:\n", workflowsGlob+"    entries:\n"),
			tracked: map[string]string{".github/workflows/ci.yml": "run: echo ${{ secrets.example_token }}\n"}, exit: 0},
		{name: "a whole-context read in another case is reported", config: vaultReplace("    entries:\n", workflowsGlob+"    entries:\n"),
			tracked: map[string]string{".github/workflows/ci.yml": "run: echo ${{ TOJSON(Secrets) }}\n"}, exit: 1,
			stderr: []string{".github/workflows/ci.yml: vault-github-unlisted-reference (secrets (whole context))"}},
		{name: "an if condition that reads an unlisted secret is reported", config: vaultReplace("    entries:\n", workflowsGlob+"    entries:\n"),
			tracked: map[string]string{".github/workflows/ci.yml": "if: secrets.OTHER != ''\n"}, exit: 1,
			stderr: []string{".github/workflows/ci.yml: vault-github-unlisted-reference (secrets.OTHER)"}},
		// Whole-manifest and Git-boundary cases.
		{name: "undeclared vault changes nothing", config: "schema: rhino/repo-config/v2\nenvironment: {}\n", exit: 0},
		{name: "vault written as text refuses", config: "schema: rhino/repo-config/v2\nenvironment:\n  vault: marker-7f3a\n", exit: 2,
			stderr: []string{"environment.vault must be a mapping"}},
		{name: "tree without a Git name boundary refuses", config: vaultBase, wrap: withoutNameBoundary, exit: 2,
			stderr: []string{"environment vault check refused: this tree has no Git name boundary"}},
		{name: "tracked-name query that fails refuses", config: vaultBase, wrap: func(m *vaultMemoryTree) application.Tree {
			return trackedFailureTree{m}
		}, exit: 2, stderr: []string{"environment vault check refused: git query failed"}},
		{name: "ignored-name query that fails refuses", config: vaultBase, wrap: func(m *vaultMemoryTree) application.Tree {
			return ignoredFailureTree{m}
		}, exit: 2, stderr: []string{"environment vault check refused: git query failed"}},
	}
}

// TestEnvironmentVaultRules holds every vault rule to its stated shape: each
// defect fires its rule on its path, each clean shape stays clean, and each
// boundary failure refuses rather than reporting a tree it never read.
func TestEnvironmentVaultRules(t *testing.T) {
	for _, tc := range vaultCases() {
		t.Run(tc.name, func(t *testing.T) {
			m := &vaultMemoryTree{memoryTree: newMemoryTree("/repository")}
			m.files["repo-config.yml"] = tc.config
			m.tracked = append(m.tracked, "repo-config.yml")
			for path, content := range tc.tracked {
				m.files[path] = content
				m.tracked = append(m.tracked, path)
			}
			for path, content := range tc.ignored {
				m.files[path] = content
				m.ignored = append(m.ignored, path)
			}
			for _, path := range tc.unreadable {
				m.markUnreadable(path)
			}
			for _, path := range tc.binary {
				m.markBinary(path)
			}
			var tree application.Tree = m
			if tc.wrap != nil {
				tree = tc.wrap(m)
			}
			outcome := application.Execute(tree, []string{"env", "validate"}, nil)
			if outcome.ExitCode != tc.exit {
				t.Fatalf("exit = %d, want %d; stdout %q stderr %q", outcome.ExitCode, tc.exit, outcome.Stdout, outcome.Stderr)
			}
			for _, want := range tc.stderr {
				if !strings.Contains(outcome.Stdout+outcome.Stderr, want) {
					t.Fatalf("output does not contain %q:\nstdout %q\nstderr %q", want, outcome.Stdout, outcome.Stderr)
				}
			}
		})
	}
}
