Feature: Rhino v0.4 contracts

  The v0.4 surface is repository-neutral and closed. These scenarios name the
  first observable contracts before their implementation exists; each one is
  expected to be RED until its owning delivery phase supplies the behavior.

  Scenario: A grouped v0.4 configuration is accepted
    Given the configuration file is this text:
      """
      # yaml-language-server: $schema=schemas/repo-config/v2.schema.json
      schema: rhino/repo-config/v2
      repository: {}
      scan: {}
      harness: {}
      policies:
        markdown: {}
        governance: {}
        conventions: {}
        plans: {}
      environment: {}
      toolchains: {}
      gates: {}
      extensions: {}
      """
    When I run the "repo-config" validator
    Then the exit code is 0

  Scenario: An unknown grouped-core key is refused
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      repository: {}
      unknown-core: {}
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "unknown core key"

  Scenario: An extension owner outside the portable namespace is refused
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      extensions:
        Invalid_owner: {}
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "extension owner"

  Scenario: A legacy configuration emits an RC-only reviewed migration plan
    Given the repository declares a complete configuration
    When I invoke the CLI with "repo-config|migrate"
    Then the exit code is 0
    And stdout contains "reviewed migration plan"
    And stdout contains "deleted before stable"

  Scenario: A grouped configuration does not require migration
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      """
    When I invoke the CLI with "repo-config|migrate"
    Then the exit code is 2
    And stderr contains "does not require migration"

  Scenario: Lifecycle identities are listed declaratively
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates: {}
      """
    When I invoke the CLI with "gate|list"
    Then the exit code is 0

  Scenario: Each declared lifecycle membership uses one closed surface
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: lifecycle-check
            type: check
            run-on:
              pre-commit: {}
              commit-msg: {}
              pre-push: {}
              pull-request: {}
              main: {}
              scheduled: {}
              manual: {}
        composition:
          pull-request:
            relation: exact
      """
    When I invoke the CLI with "gate|list"
    Then the exit code is 0
    And stdout contains "pre-commit: lifecycle-check"
    And stdout contains "commit-msg: lifecycle-check"
    And stdout contains "pre-push: lifecycle-check"
    And stdout contains "pull-request: lifecycle-check"
    And stdout contains "main: lifecycle-check"
    And stdout contains "scheduled: lifecycle-check"
    And stdout contains "manual: lifecycle-check"

  Scenario: A legacy ci lifecycle membership is refused
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: no-legacy-ci
            type: check
            run-on:
              ci: {}
      """
    When I invoke the CLI with "repo-config|validate"
    Then the exit code is 2
    And stderr contains "ci"

  Scenario: Pull-request exact composition includes every local gate
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: local-check
            type: check
            run-on:
              pre-commit: {}
        composition:
          pull-request:
            relation: exact
      """
    When I invoke the CLI with "gate|validate"
    Then the exit code is 2
    And stderr contains "missing from pull-request composition"

  Scenario: Pull-request at-least extras name their direct reason
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: local-check
            type: check
            run-on:
              pre-commit: {}
              pull-request: {}
          - id: pr-only-check
            type: check
            run-on:
              pull-request: {}
        composition:
          pull-request:
            relation: at-least
      """
    When I invoke the CLI with "gate|validate"
    Then the exit code is 2
    And stderr contains "needs its own non-empty reason"

  Scenario: Pull-request at-least extras retain their direct reason
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: local-check
            type: check
            run-on:
              pre-commit: {}
              pull-request: {}
          - id: pr-only-check
            type: check
            run-on:
              pull-request:
                reason: runs only in the disposable pull-request replay
        composition:
          pull-request:
            relation: at-least
      """
    When I invoke the CLI with "gate|validate"
    Then the exit code is 0

  Scenario: Duplicate semantic lifecycle IDs are refused
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: repeated-check
            type: check
            run-on:
              main: {}
          - id: repeated-check
            type: check
            run-on:
              scheduled: {}
      """
    When I invoke the CLI with "repo-config|validate"
    Then the exit code is 2
    And stderr contains "duplicated"

  Scenario: Generic gate inputs are validated before dispatch
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: state-check
            type: check
            inputs:
              repository: { kind: repository-state }
            command:
              executable: ./rhino
              args:
                - { literal: repo-config }
                - { literal: validate }
                - { input: repository.root, expand: single }
            run-on:
              manual:
                bind:
                  repository: { source: checkout }
      """
    When I invoke the CLI with "gate|validate"
    Then the exit code is 0

  Scenario: Tool-named input selectors are refused
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: files-check
            type: check
            inputs:
              files: { kind: files }
            command:
              executable: ./rhino
              args:
                - { input: files.paths, expand: repeat }
            run-on:
              manual:
                bind:
                  files: { source: nx-affected }
      """
    When I invoke the CLI with "gate|validate"
    Then the exit code is 2
    And stderr contains "nx-affected"

  Scenario: Typed argv and environment projections name resolved range fields
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: range-check
            type: check
            inputs:
              range: { kind: commit-range }
            command:
              executable: npm
              args:
                - { literal: exec }
                - { literal: -- }
                - { literal: nx }
                - { literal: affected }
              environment:
                NX_BASE: { input: range.base }
                NX_HEAD: { input: range.head }
            run-on:
              pre-push:
                bind:
                  range: { source: push-updates, fallback: refs/remotes/origin/main }
              pull-request:
                bind:
                  range: { source: explicit-range, range: explicit }
        composition:
          pull-request:
            relation: exact
      """
    When I invoke the CLI with "gate|validate"
    Then the exit code is 0

  Scenario: A pre-push range declares its new-ref fallback
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: range-check
            type: check
            inputs:
              range: { kind: commit-range }
            command:
              executable: runner
              args:
                - { input: range.base, expand: single }
            run-on:
              pre-push:
                bind:
                  range: { source: push-updates, fallback: refs/remotes/origin/main }
              pull-request:
                bind:
                  range: { source: explicit-range, range: explicit }
        composition:
          pull-request:
            relation: exact
      """
    When I invoke the CLI with "gate|validate"
    Then the exit code is 0

  Scenario: Unresolved typed input projections are refused
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: unresolved-input
            type: check
            inputs:
              repository: { kind: repository-state }
            command:
              executable: ./rhino
              args:
                - { input: missing.root, expand: single }
            run-on:
              manual:
                bind:
                  repository: { source: checkout }
      """
    When I invoke the CLI with "gate|validate"
    Then the exit code is 2
    And stderr contains "does not declare"

  Scenario: Typed bindings reject a source for the wrong input kind
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: wrong-source
            type: check
            inputs:
              files: { kind: files }
            command:
              executable: ./rhino
              args:
                - { input: files.paths, expand: repeat }
            run-on:
              manual:
                bind:
                  files: { source: hook-message-file }
      """
    When I invoke the CLI with "gate|validate"
    Then the exit code is 2
    And stderr contains "not a compatible source"

  Scenario: Surface membership cannot replace a declared command
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: replacement-command
            type: check
            run-on:
              manual:
                command:
                  executable: not-allowed-here
      """
    When I invoke the CLI with "gate|validate"
    Then the exit code is 2
    And stderr contains "unknown field `command`"

  Scenario: A mutation gate declares paired local and pull-request behavior
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: format-markdown
            type: mutation
            mutation:
              local: apply-index
              ci: verify-clean
            command:
              executable: formatter
              args:
                - { literal: --write }
            run-on:
              pre-commit: {}
              pull-request: {}
        composition:
          pull-request:
            relation: exact
      """
    When I invoke the CLI with "gate|validate"
    Then the exit code is 0

  Scenario: A mutation gate cannot omit its pull-request replay contract
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: incomplete-mutation
            type: mutation
            mutation:
              local: apply-index
            command:
              executable: formatter
            run-on:
              pre-commit: {}
              pull-request: {}
        composition:
          pull-request:
            relation: exact
      """
    When I invoke the CLI with "gate|validate"
    Then the exit code is 2
    And stderr contains "missing field `ci`"

  Scenario: A mutation refuses a mutable-working-tree fallback
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: safe-mutation
            type: mutation
            mutation:
              local: apply-index
              ci: verify-clean
            command:
              executable: formatter
            run-on:
              pre-commit: {}
              pull-request: {}
        composition:
          pull-request:
            relation: exact
      """
    When I invoke the CLI with "gate|run|--surface|pre-commit"
    Then the exit code is 3
    And stderr contains "no index mutation boundary"

  Scenario: Harness adapters have one canonical lifecycle
    Given the repository declares a complete configuration
    When I invoke the CLI with "harness|adapters|validate"
    Then the exit code is 0

  Scenario: Environment backup starts with an explicit destination plan
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      environment: {}
      """
    When I invoke the CLI with "env|backup|--dir|safe-backups"
    Then the exit code is 0

  Scenario: Environment backup refuses a destination outside its repository
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      environment: {}
      """
    When I invoke the CLI with "env|backup|--dir|../outside"
    Then the exit code is 2
    And stderr contains "backup destination leaves the repository root"

  Scenario: Text and JSON gate listings share a result envelope
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates: {}
      """
    When I invoke the CLI with "gate|list|--output|json"
    Then the exit code is 0
    And stdout JSON property "schemaVersion" is 1

  Scenario: Retired gate aliases are unknown
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates: {}
      """
    When I invoke the CLI with "gate|emit"
    Then the exit code is 2
    And stderr contains "unrecognized command"

  Scenario: Retired gate audit alias is unknown
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates: {}
      """
    When I invoke the CLI with "gate|audit"
    Then the exit code is 2
    And stderr contains "unrecognized command"

  Scenario: Conventional Commit input follows the commit-message lifecycle
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: conventional-commit
            type: check
            inputs:
              message: { kind: commit-message }
            command:
              executable: ./check-conventional-commit
              args:
                - { input: message.text, expand: single }
            run-on:
              commit-msg:
                bind:
                  message: { source: hook-message-file }
              pull-request:
                bind:
                  message: { source: explicit-range, range: explicit }
        composition:
          pull-request:
            relation: exact
      """
    When I invoke the CLI with "gate|validate"
    Then the exit code is 0

  Scenario: Lifecycle composition is checked before a gate runs
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates: {}
      """
    When I invoke the CLI with "gate|validate|--output|json"
    Then the exit code is 0

  Scenario: Pull-request replay receives an explicit immutable range
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates: {}
      """
    When I invoke the CLI with "gate|run|--surface|pull-request|--base|aaaaaaaa|--head|bbbbbbbb"
    Then the exit code is 0

  Scenario: Canonical harness adapters generate as one transaction
    Given the repository declares a complete configuration
    When I invoke the CLI with "harness|adapters|generate"
    Then the exit code is 0

  Scenario: Grouped harness adapters refuse an inherited profile selector
    Given the repository declares a complete configuration
    When I invoke the CLI with "harness|adapters|validate|--harness|alpha"
    Then the exit code is 2
    And stderr contains "--harness"

  Scenario: Canonical adapters generate once and then become a deterministic no-op
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      harness:
        requirements:
          capabilities: [read]
          grants: [files-read]
          denials: [network]
          constraints: [offline]
          routes: [canonical-import]
          identities: [reviewer]
        profiles:
          - id: alpha
            root: adapters/alpha
            supports:
              capabilities: [read]
              grants: [files-read]
              denials: [network]
              constraints: [offline]
              routes: [canonical-import]
              identities: [reviewer]
          - id: beta
            root: adapters/beta
            supports:
              capabilities: [read]
              grants: [files-read]
              denials: [network]
              constraints: [offline]
              routes: [canonical-import]
              identities: [reviewer]
          - id: gamma
            root: adapters/gamma
            supports:
              capabilities: [read]
              grants: [files-read]
              denials: [network]
              constraints: [offline]
              routes: [canonical-import]
              identities: [reviewer]
      """
    And the repository contains:
      | path                           | content               |
      | AGENTS.md                      | Canonical instruction |
      | .agents/agents/reviewer.md     | Canonical agent       |
      | .agents/skills/review/SKILL.md | Canonical skill       |
    When I invoke the CLI with "harness|adapters|generate"
    Then the exit code is 0
    And the last adapter generation changes the repository
    When I invoke the CLI with "harness|adapters|generate"
    Then the exit code is 0
    And the last adapter generation makes no repository change
    And the two runs are byte-identical

  Scenario: An unrepresentable canonical requirement refuses before adapter generation
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      harness:
        requirements:
          capabilities: [write]
        profiles:
          - id: alpha
            root: adapters/alpha
            supports: {capabilities: [write]}
          - id: beta
            root: adapters/beta
            supports: {capabilities: []}
          - id: gamma
            root: adapters/gamma
            supports: {capabilities: [write]}
      """
    And the repository contains:
      | path                         | content               |
      | AGENTS.md                    | Canonical instruction |
      | .agents/agents/writer.md     | Canonical agent       |
      | .agents/skills/write/SKILL.md | Canonical skill      |
    When I invoke the CLI with "harness|adapters|generate"
    Then the exit code is 2
    And stderr contains "profile `beta` cannot represent required capability `write`"
    And the repository is unchanged by the inspection

  Scenario: Toolchain provisioning requires an explicit apply contract
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      toolchains: {}
      """
    When I invoke the CLI with "toolchain|provision|--apply"
    Then the exit code is 0

  Scenario: Toolchain provisioning plans before an explicit mutation
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      toolchains: {}
      """
    When I invoke the CLI with "toolchain|provision"
    Then the exit code is 0
    And stdout contains "planned 0"

  Scenario: Toolchain validation reports a required unavailable probe without output bytes
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      toolchains:
        entries:
          - id: unavailable
            executable: unavailable-tool
            probe: [--version]
            required: true
      """
    When I invoke the CLI with "toolchain|validate"
    Then the exit code is 1
    And stderr contains "unavailable"

  Scenario: Curated environment detection accepts a declared Rust key
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      environment:
        contracts:
          - path: .env.example
            keys: [RUST_KEY]
        detectors:
          - language: rust
            paths: [src/main.rs]
      """
    And the repository contains:
      | path        | content                            |
      | src/main.rs | std::env::var("RUST_KEY");         |
    When I invoke the CLI with "env|validate"
    Then the exit code is 0

  Scenario: Curated environment detection redacts unsupported dynamic access
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      environment:
        detectors:
          - language: rust
            paths: [src/main.rs]
      """
    And the repository contains:
      | path        | content                                  |
      | src/main.rs | std::env::var(dynamic_name); SYNTHETIC=x |
    When I invoke the CLI with "env|validate"
    Then the exit code is 1
    And stderr contains "unsupported-dynamic-access"

  Scenario: Environment validation reports an explicitly empty policy clean
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      environment: {}
      """
    When I invoke the CLI with "env|validate"
    Then the exit code is 0

  Scenario: Environment initialization refuses a policy with no targets
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      environment: {}
      """
    When I invoke the CLI with "env|init|--apply"
    Then the exit code is 2
    And stderr contains "declares no example targets"

  Scenario: License policy is repository-configured rather than OSE-defined
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        conventions:
          license:
            paths:
              - path: LICENSE
                identifiers: ["License-Identifier: MIT"]
      """
    And the repository contains:
      | path    | content                         |
      | LICENSE | License-Identifier: MIT\nSynthetic |
    When I invoke the CLI with "convention|license|validate"
    Then the exit code is 0

  Scenario: License policy reports only its configured identifier mismatch
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        conventions:
          license:
            paths:
              - path: LICENSE
                identifiers: ["License-Identifier: MIT"]
      """
    And the repository contains:
      | path    | content                            |
      | LICENSE | License-Identifier: Other\nSynthetic |
    When I invoke the CLI with "convention|license|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "license-identifier-mismatch"

  Scenario: License policy reports a configured digest mismatch
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        conventions:
          license:
            paths:
              - path: LICENSE
                sha256: 0000000000000000000000000000000000000000000000000000000000000000
      """
    And the repository contains:
      | path    | content                         |
      | LICENSE | License-Identifier: MIT\nSynthetic |
    When I invoke the CLI with "convention|license|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "license-digest-mismatch"

  Scenario: README index requires only declared direct-child links and annotations
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        markdown:
          readme-index:
            trees:
              - path: docs
                require-direct-children: true
                annotations:
                  - path: README.md
                    text: Synthetic index
      """
    And the repository contains:
      | path           | content                                |
      | docs/README.md | # Docs\n\nSynthetic index\n\n- [Guide](guide.md) |
      | docs/guide.md  | # Guide                                |
    When I invoke the CLI with "md|readme-index|validate"
    Then the exit code is 0

  Scenario: README index reports an omitted declared direct child
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        markdown:
          readme-index:
            trees:
              - path: docs
                require-direct-children: true
      """
    And the repository contains:
      | path           | content    |
      | docs/README.md | # Docs     |
      | docs/guide.md  | # Guide    |
    When I invoke the CLI with "md|readme-index|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "missing-readme-index-child"

  Scenario: README index excludes only a declared direct-child subtree
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        markdown:
          readme-index:
            trees:
              - path: docs
                require-direct-children: true
                exclusions: [generated]
      """
    And the repository contains:
      | path                    | content     |
      | docs/README.md          | # Docs      |
      | docs/generated/item.md  | # Generated |
    When I invoke the CLI with "md|readme-index|validate"
    Then the exit code is 0

  Scenario: README index refuses an escaping exclusion
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        markdown:
          readme-index:
            trees:
              - path: docs
                exclusions: [../outside]
      """
    And the repository contains:
      | path           | content |
      | docs/README.md | # Docs  |
    When I invoke the CLI with "md|readme-index|validate"
    Then the exit code is 2
    And stderr contains "exact paths"

  Scenario: Vendor policy permits only an exact declared vocabulary exception
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        governance:
          vendor:
            roots: [docs]
            excluded-binding-roots: [docs/bindings]
            forbidden-terms: [ExampleVendor]
            vocabulary-exceptions:
              - term: ExampleVendor
                paths: [docs/README.md]
      """
    And the repository contains:
      | path                | content                 |
      | docs/README.md      | ExampleVendor reference |
      | docs/bindings/a.txt | ExampleVendor binding   |
    When I invoke the CLI with "governance|vendor|validate"
    Then the exit code is 0

  Scenario: Vendor policy reports an undeclared forbidden term
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        governance:
          vendor:
            roots: [docs]
            forbidden-terms: [ExampleVendor]
      """
    And the repository contains:
      | path           | content                 |
      | docs/README.md | ExampleVendor reference |
    When I invoke the CLI with "governance|vendor|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "forbidden-vendor-term"

  Scenario: Vendor policy refuses an escaping exception path
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        governance:
          vendor:
            roots: [docs]
            forbidden-terms: [ExampleVendor]
            vocabulary-exceptions:
              - term: ExampleVendor
                paths: [../outside]
      """
    And the repository contains:
      | path           | content                 |
      | docs/README.md | ExampleVendor reference |
    When I invoke the CLI with "governance|vendor|validate"
    Then the exit code is 2
    And stderr contains "exact repository-relative paths"

  Scenario: Layer policy validates only the declared layer order and categories
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        governance:
          layers:
            root: policy
            order: [foundation, practice]
            categories:
              foundation: [shared]
              practice: [guides]
      """
    And the repository contains:
      | path                              | content |
      | policy/foundation/shared/item.md  | # Item  |
      | policy/practice/guides/item.md    | # Item  |
    When I invoke the CLI with "governance|layers|validate"
    Then the exit code is 0

  Scenario: Layer policy reports a missing declared category
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        governance:
          layers:
            root: policy
            order: [foundation]
            categories:
              foundation: [shared]
      """
    And the repository contains:
      | path                          | content |
      | policy/foundation/README.md   | # Item  |
    When I invoke the CLI with "governance|layers|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "missing-governance-category"

  Scenario: Layer policy reports an undeclared layer
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        governance:
          layers:
            root: policy
            order: [foundation]
            categories:
              foundation: [shared]
      """
    And the repository contains:
      | path                              | content |
      | policy/foundation/shared/item.md  | # Item  |
      | policy/extra/local/item.md        | # Item  |
    When I invoke the CLI with "governance|layers|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "unexpected-governance-layer"

  Scenario: Layer policy refuses an escaping category
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        governance:
          layers:
            root: policy
            order: [foundation]
            categories:
              foundation: [../outside]
      """
    When I invoke the CLI with "governance|layers|validate"
    Then the exit code is 2
    And stderr contains "simple names"

  Scenario: Traceability policy validates a declared artifact relationship
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        governance:
          traceability:
            artifacts:
              - id: requirements
                path: requirements.md
              - id: design
                path: design.md
            relationships:
              - from: design
                to: requirements
      """
    And the repository contains:
      | path            | content                                    |
      | requirements.md | # Requirements                             |
      | design.md       | # Design\n\n[Requirements](requirements.md) |
    When I invoke the CLI with "governance|traceability|validate"
    Then the exit code is 0

  Scenario: Traceability policy reports an omitted declared relationship
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        governance:
          traceability:
            artifacts:
              - id: requirements
                path: requirements.md
              - id: design
                path: design.md
            relationships:
              - from: design
                to: requirements
      """
    And the repository contains:
      | path            | content          |
      | requirements.md | # Requirements   |
      | design.md       | # Design         |
    When I invoke the CLI with "governance|traceability|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "missing-traceability-relationship"

  Scenario: Traceability policy reports a missing declared artifact
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        governance:
          traceability:
            artifacts:
              - id: requirements
                path: requirements.md
      """
    When I invoke the CLI with "governance|traceability|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "missing-traceability-artifact"

  Scenario: Frontmatter policy accepts a declared required key
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        markdown:
          frontmatter:
            surfaces:
              - glob: notes/**/*.md
                require: [title]
      """
    And the repository contains:
      | path           | content                         |
      | notes/entry.md | ---\ntitle: Entry\n---\n\n# Entry |
    When I invoke the CLI with "md|frontmatter|validate"
    Then the exit code is 0

  Scenario: Frontmatter policy reports an absent declared key
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        markdown:
          frontmatter:
            surfaces:
              - glob: notes/**/*.md
                require: [title]
      """
    And the repository contains:
      | path           | content            |
      | notes/entry.md | ---\n---\n\n# Entry |
    When I invoke the CLI with "md|frontmatter|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "missing-frontmatter-key"

  Scenario: Internal-link policy accepts a declared local target
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        markdown:
          internal-link:
            exclude-sources: []
      """
    And the repository contains:
      | path          | content                 |
      | docs/readme.md | [Guide](guide.md)       |
      | docs/guide.md  | # Guide                 |
    When I invoke the CLI with "md|internal-link|validate"
    Then the exit code is 0

  Scenario: Internal-link policy reports a missing local target
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        markdown:
          internal-link:
            exclude-sources: []
      """
    And the repository contains:
      | path          | content              |
      | docs/readme.md | [Missing](guide.md)  |
    When I invoke the CLI with "md|internal-link|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "internal-link-missing"

  Scenario: Mermaid policy accepts a declared plain-text repository with no diagrams
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        markdown:
          mermaid:
            authoring-rule: plain-text
            node-label-graphemes: 32
            edge-label-graphemes: 24
            fill-colors: []
            edge-colors: []
            text-colors: []
      """
    And the repository contains:
      | path        | content |
      | docs/a.md   | # Text  |
    When I invoke the CLI with "md|mermaid|validate"
    Then the exit code is 0

  Scenario: Mermaid policy reports a diagram forbidden by its authoring rule
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        markdown:
          mermaid:
            authoring-rule: plain-text
            node-label-graphemes: 32
            edge-label-graphemes: 24
            fill-colors: []
            edge-colors: []
            text-colors: []
      """
    And the repository contains:
      | path        | content                           |
      | docs/a.md   | ```mermaid\nflowchart LR\nA-->B\n``` |
    When I invoke the CLI with "md|mermaid|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "diagram-authoring-rule"
