Feature: Environment vault manifest validation

  A repository declares its secret-bearing files, its templates, and the secrets its workflows read under
  environment.vault. env validate reports what that manifest leaves unclassified. It reads names from Git. From tracked workflows it reads only the
  secret and variable references in their expressions and if: values. It never opens an ignored file.

  Scenario: Reject an unclassified ignored secret file
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: org/example-repo
          environments: [local, stag, canary, prod]
          patterns:
            - "**/.env*"
          entries:
            - path: infra/example/.env.prod
              class: vault
              form: env
              app: infra-example
              env: prod
              keys: [EXAMPLE_TOKEN]
      """
    And the repository ignores "apps/example/.env.dev" with the content "EXAMPLE_TOKEN=placeholder"
    When I run environment validation
    Then the command exits 1
    And the output names "apps/example/.env.dev"
    And the output names "vault-unclassified-ignored-file"

  Scenario: Accept an ignored secret file that an entry classifies
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: org/example-repo
          environments: [local, stag, canary, prod]
          patterns:
            - "**/.env*"
          entries:
            - path: infra/example/.env.prod
              class: vault
              form: env
              app: infra-example
              env: prod
              keys: [EXAMPLE_TOKEN]
      """
    And the repository ignores "infra/example/.env.prod" with the content "EXAMPLE_TOKEN=placeholder"
    When I run environment validation
    Then the command exits 0

  Scenario: Reject an unclassified tracked template
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
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
      """
    And the repository tracks "infra/example/.env.example" with the content "EXAMPLE_TOKEN="
    When I run environment validation
    Then the command exits 1
    And the output names "infra/example/.env.example"
    And the output names "vault-unclassified-template"

  Scenario: Accept a tracked template that a templates row names
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
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
          templates:
            - path: infra/example/.env.example
              for: infra/example/.env.prod
      """
    And the repository tracks "infra/example/.env.example" with the content "EXAMPLE_TOKEN="
    When I run environment validation
    Then the command exits 0

  Scenario: Reject a tracked template that only a glob entry covers
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
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
            - path: infra/example/*.example
              class: derived
              reason: Documented placeholder, not a secret.
              by: infra/example/README.md
      """
    And the repository tracks "infra/example/.env.example" with the content "EXAMPLE_TOKEN="
    When I run environment validation
    Then the command exits 1
    And the output names "infra/example/.env.example"
    And the output names "vault-unclassified-template"

  Scenario: Reject an exact entry path that another entry's glob covers
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
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
            - path: infra/example/.env.*
              class: derived
              reason: Covers every environment file.
              by: infra/example/README.md
      """
    When I run environment validation
    Then the command exits 1
    And the output names "infra/example/.env.prod"
    And the output names "vault-manifest-invalid"

  Scenario: Reject a templates row whose for names no entry
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
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
          templates:
            - path: infra/example/.env.example
              for: infra/example/.env.stag
      """
    And the repository tracks "infra/example/.env.example" with the content "EXAMPLE_TOKEN="
    When I run environment validation
    Then the command exits 1
    And the output names "infra/example/.env.example"
    And the output names "vault-manifest-invalid"

  Scenario: Reject a templates row whose for is a glob
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: org/example-repo
          environments: [local, stag, canary, prod]
          entries:
            - path: infra/*/gen.json
              class: derived
              reason: Generated from the vault.
              by: scripts/generate.mjs
          templates:
            - path: infra/example/.env.example
              for: infra/*/gen.json
      """
    And the repository tracks "infra/example/.env.example" with the content "EXAMPLE_TOKEN="
    When I run environment validation
    Then the command exits 1
    And the output names "infra/example/.env.example"
    And the output names "vault-manifest-invalid"

  Scenario: Reject a workflow secret reference that the manifest does not list
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
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
          github:
            workflows: [".github/workflows/*.yml"]
            builtin: [GITHUB_TOKEN]
            secrets:
              - name: EXAMPLE_TOKEN
                source: { app: infra-example, env: prod, secret: example-token }
      """
    And the repository tracks ".github/workflows/release.yml" with the content "run: echo ${{ secrets.OTHER_TOKEN }}"
    When I run environment validation
    Then the command exits 1
    And the output names ".github/workflows/release.yml"
    And the output names "secrets.OTHER_TOKEN"

  Scenario: Reject a workflow variable reference that the manifest does not list
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
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
          github:
            workflows: [".github/workflows/*.yml"]
            builtin: [GITHUB_TOKEN]
            secrets:
              - name: EXAMPLE_TOKEN
                source: { app: infra-example, env: prod, secret: example-token }
            variables:
              - name: EXAMPLE_LIVE_ENABLED
                note: Plain switch.
      """
    And the repository tracks ".github/workflows/release.yml" with the content "run: echo ${{ vars.OTHER_SWITCH }}"
    When I run environment validation
    Then the command exits 1
    And the output names ".github/workflows/release.yml"
    And the output names "vars.OTHER_SWITCH"

  Scenario: Accept workflow references that the manifest lists or declares as built in
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
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
          github:
            workflows: [".github/workflows/*.yml"]
            builtin: [GITHUB_TOKEN]
            secrets:
              - name: EXAMPLE_TOKEN
                source: { app: infra-example, env: prod, secret: example-token }
            variables:
              - name: EXAMPLE_LIVE_ENABLED
                note: Plain switch.
      """
    And the repository tracks ".github/workflows/release.yml" with the content "run: echo ${{ secrets.EXAMPLE_TOKEN }} ${{ secrets.GITHUB_TOKEN }} ${{ vars.EXAMPLE_LIVE_ENABLED }}"
    When I run environment validation
    Then the command exits 0

  Scenario: Accept a bracket workflow reference that the manifest lists
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
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
          github:
            workflows: [".github/workflows/*.yml"]
            builtin: [GITHUB_TOKEN]
            secrets:
              - name: EXAMPLE_TOKEN
                source: { app: infra-example, env: prod, secret: example-token }
      """
    And the repository tracks ".github/workflows/release.yml" with the content "run: echo ${{ secrets['EXAMPLE_TOKEN'] }}"
    When I run environment validation
    Then the command exits 0

  Scenario Outline: Reject a workflow reference the manifest cannot resolve to one listed name
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
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
          github:
            workflows: [".github/workflows/*.yml"]
            builtin: [GITHUB_TOKEN]
            secrets:
              - name: EXAMPLE_TOKEN
                source: { app: infra-example, env: prod, secret: example-token }
      """
    And the repository tracks ".github/workflows/release.yml" with the content "<content>"
    When I run environment validation
    Then the command exits 1
    And the output names ".github/workflows/release.yml"
    And the output names "<finding>"
    And the output names "vault-github-unlisted-reference"

    Examples:
      | content                                 | finding                 |
      | run: echo ${{ secrets['OTHER_TOKEN'] }} | secrets.OTHER_TOKEN       |
      | run: echo ${{ vars['OTHER_SWITCH'] }}   | vars.OTHER_SWITCH         |
      | run: echo ${{ secrets[matrix.target] }} | secrets[dynamic]          |
      | run: echo ${{ vars[env.NAME] }}         | vars[dynamic]             |
      | run: echo ${{ toJSON(secrets) }}        | secrets (whole context)   |
      | secrets: inherit                        | secrets: inherit          |

  Scenario Outline: Reject a workflow glob that is not a relative pattern
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
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
          github:
            workflows: ["<glob>"]
            builtin: [GITHUB_TOKEN]
      """
    When I run environment validation
    Then the command exits 1
    And the output names "github.workflows"
    And the output names "<glob>"
    And the output names "vault-manifest-invalid"

    Examples:
      | glob       |
      | /abs/*.yml |
      | ../x.yml   |
      | [bad       |

  Scenario: Accept workflow text that names a secret outside an expression
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
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
          github:
            workflows: [".github/workflows/*.yml"]
            builtin: [GITHUB_TOKEN]
            secrets:
              - name: EXAMPLE_TOKEN
                source: { app: infra-example, env: prod, secret: example-token }
      """
    And the repository tracks ".github/workflows/release.yml" with the content "run: cat docs/secrets.md && node -e @vars.yml"
    When I run environment validation
    Then the command exits 0

  Scenario: Accept a context name inside a string literal of an expression
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
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
          github:
            workflows: [".github/workflows/*.yml"]
            builtin: [GITHUB_TOKEN]
            secrets:
              - name: EXAMPLE_TOKEN
                source: { app: infra-example, env: prod, secret: example-token }
      """
    And the repository tracks ".github/workflows/release.yml" with the content "run: echo ${{ format('org/secrets-repo') }} ${{ contains(inputs.x, 'vars') }}"
    When I run environment validation
    Then the command exits 0

  Scenario: Reject a workflow reference written in another letter case
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
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
          github:
            workflows: [".github/workflows/*.yml"]
            builtin: [GITHUB_TOKEN]
            secrets:
              - name: EXAMPLE_TOKEN
                source: { app: infra-example, env: prod, secret: example-token }
      """
    And the repository tracks ".github/workflows/release.yml" with the content "run: echo ${{ SECRETS.OTHER_TOKEN }}"
    When I run environment validation
    Then the command exits 1
    And the output names ".github/workflows/release.yml"
    And the output names "secrets.OTHER_TOKEN"
    And the output names "vault-github-unlisted-reference"

  Scenario: Reject a workflow if condition that reads an unlisted secret
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
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
          github:
            workflows: [".github/workflows/*.yml"]
            builtin: [GITHUB_TOKEN]
            secrets:
              - name: EXAMPLE_TOKEN
                source: { app: infra-example, env: prod, secret: example-token }
      """
    And the repository tracks ".github/workflows/release.yml" with the content "if: secrets.OTHER_TOKEN != ''"
    When I run environment validation
    Then the command exits 1
    And the output names ".github/workflows/release.yml"
    And the output names "secrets.OTHER_TOKEN"
    And the output names "vault-github-unlisted-reference"

  Scenario: Reject an item name whose environment is outside the declared set
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: org/example-repo
          environments: [local, stag, canary, prod]
          entries:
            - path: infra/example/.env.dev
              class: vault
              form: env
              app: infra-example
              env: dev
              keys: [EXAMPLE_TOKEN]
      """
    When I run environment validation
    Then the command exits 1
    And the output names "infra/example/.env.dev"
    And the output names "vault-name-grammar"

  Scenario: Reject an item name whose secret segment is not kebab-case
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
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
              keys: [Example.Token]
      """
    When I run environment validation
    Then the command exits 1
    And the output names "infra/example/.env.prod"
    And the output names "vault-name-grammar"

  Scenario: Reject a manifest entry that has no reason
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: org/example-repo
          environments: [local, stag, canary, prod]
          entries:
            - path: apps/example/.secrets/postgres-password
              class: generated
              by: scripts/run-local.mjs
      """
    When I run environment validation
    Then the command exits 1
    And the output names "apps/example/.secrets/postgres-password"
    And the output names "vault-manifest-invalid"

  Scenario: Reject a vault env entry that names a reason
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
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
              reason: Not read by an env entry.
      """
    When I run environment validation
    Then the command exits 1
    And the output names "infra/example/.env.prod"
    And the output names "vault-manifest-invalid"

  Scenario: Reject a vault file entry that names a consumer
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: org/example-repo
          environments: [local, stag, canary, prod]
          entries:
            - path: infra/example/terraform/terraform.tfvars
              class: vault
              form: file
              app: infra-example
              env: prod
              secret: tfvars
              consumer: scripts/apply.mjs
      """
    When I run environment validation
    Then the command exits 1
    And the output names "infra/example/terraform/terraform.tfvars"
    And the output names "vault-manifest-invalid"

  Scenario: Reject a resident entry that sets a mode
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: org/example-repo
          environments: [local, stag, canary, prod]
          entries:
            - class: vault
              form: resident
              app: infra-example
              env: prod
              secret: resident-token
              mode: "0600"
      """
    When I run environment validation
    Then the command exits 1
    And the output names "environment.vault.entries[0]"
    And the output names "vault-manifest-invalid"

  Scenario: Reject a derived entry that names a consumer
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: org/example-repo
          environments: [local, stag, canary, prod]
          entries:
            - path: apps/example/gen.json
              class: derived
              reason: Generated from the vault.
              by: scripts/generate.mjs
              consumer: scripts/apply.mjs
      """
    When I run environment validation
    Then the command exits 1
    And the output names "apps/example/gen.json"
    And the output names "vault-manifest-invalid"

  Scenario: Reject a machine-local entry that names a by
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: org/example-repo
          environments: [local, stag, canary, prod]
          entries:
            - path: hippo.local.json
              class: machine-local
              reason: Written by this machine only.
              by: scripts/setup.mjs
      """
    When I run environment validation
    Then the command exits 1
    And the output names "hippo.local.json"
    And the output names "vault-manifest-invalid"

  Scenario: Accept each field a class reads
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
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
              mode: "0600"
            - path: infra/example/terraform/terraform.tfvars
              class: vault
              form: file
              app: infra-example
              env: prod
              secret: tfvars
              mode: "0600"
            - class: vault
              form: resident
              app: infra-example
              env: prod
              secret: resident-token
              consumer: scripts/apply.mjs
            - path: apps/example/gen.json
              class: derived
              reason: Generated from the vault.
              by: scripts/generate.mjs
            - path: hippo.local.json
              class: machine-local
              reason: Written by this machine only.
      """
    When I run environment validation
    Then the command exits 0

  Scenario: Reject a GitHub secret whose source names no entry
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
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
          github:
            workflows: [".github/workflows/*.yml"]
            secrets:
              - name: EXAMPLE_TOKEN
                source: { app: infra-example, env: stag, secret: example-token }
      """
    When I run environment validation
    Then the command exits 1
    And the output names "EXAMPLE_TOKEN"
    And the output names "vault-github-source-unknown"

  Scenario Outline: Print no value on success
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: org/example-repo
          environments: [local, stag, canary, prod]
          patterns:
            - "**/.env*"
          entries:
            - path: infra/example/.env.prod
              class: vault
              form: env
              app: infra-example
              env: prod
              keys: [EXAMPLE_TOKEN]
      """
    And the repository ignores "infra/example/.env.prod" with the content "EXAMPLE_TOKEN=marker-7f3a"
    When I run environment validation in <format> format
    Then the command exits 0
    And no run output contains "marker-7f3a"

    Examples:
      | format |
      | text   |
      | json   |

  Scenario Outline: Print no value on a finding in an ignored file
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: org/example-repo
          environments: [local, stag, canary, prod]
          patterns:
            - "**/.env*"
          entries:
            - path: infra/example/.env.prod
              class: vault
              form: env
              app: infra-example
              env: prod
              keys: [EXAMPLE_TOKEN]
      """
    And the repository ignores "apps/example/.env.dev" with the content "EXAMPLE_TOKEN=marker-7f3a"
    When I run environment validation in <format> format
    Then the command exits 1
    And the output names "apps/example/.env.dev"
    And no run output contains "marker-7f3a"

    Examples:
      | format |
      | text   |
      | json   |

  Scenario Outline: Print no value on a finding
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
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
          github:
            workflows: [".github/workflows/*.yml"]
            builtin: [GITHUB_TOKEN]
            secrets:
              - name: EXAMPLE_TOKEN
                source: { app: infra-example, env: prod, secret: example-token }
      """
    And the repository tracks ".github/workflows/release.yml" with the content "run: echo marker-7f3a ${{ secrets.OTHER_TOKEN }}"
    When I run environment validation in <format> format
    Then the command exits 1
    And the output names "secrets.OTHER_TOKEN"
    And no run output contains "marker-7f3a"

    Examples:
      | format |
      | text   |
      | json   |

  Scenario Outline: Print no value on a refusal
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: [marker-7f3a]
          server: https://vault.example.com
          prefix: org/example-repo
          environments: [local, stag, canary, prod]
      """
    When I run environment validation in <format> format
    Then the command exits 2
    And the output names "environment.vault.provider"
    And no run output contains "marker-7f3a"

    Examples:
      | format |
      | text   |
      | json   |

  Scenario: Validation starts only read-only Git queries
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: org/example-repo
          environments: [local, stag, canary, prod]
          patterns:
            - "**/.env*"
          entries:
            - path: infra/example/.env.prod
              class: vault
              form: env
              app: infra-example
              env: prod
              keys: [EXAMPLE_TOKEN]
      """
    And the repository ignores "infra/example/.env.prod" with the content "EXAMPLE_TOKEN=placeholder"
    When I run environment validation
    Then the command exits 0
    And the run started only read-only Git queries
