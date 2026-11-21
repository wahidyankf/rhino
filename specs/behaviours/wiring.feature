Feature: Gate wiring

  A repository that declares gates in its registry must call each surface a gate runs on from a hook or a workflow
  step, and a local hook must not hide a failure. RHINO checks the wiring between the registry, the hook directory, and
  the workflow directory: which surfaces are called, by which file, and whether each local surface holds only the
  entries its allowed list names. Whether a gate is correct stays the gate's own judgement.

  Background:
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        composition:
          pull-request:
            relation: exact
        entries:
          - id: format-staged
            command:
              executable: formatter
            run-on:
              pre-commit: {}
              pull-request: {}
          - id: commit-message-lint
            command:
              executable: linter
            run-on:
              commit-msg: {}
              pull-request: {}
          - id: leak-scan
            command:
              executable: scanner
            run-on:
              pre-push: {}
              pull-request: {}
        wiring:
          hooks:
            directory: .husky
          ci:
            directory: .github/workflows
      """
    And the repository contains:
      | path                 | content                                                              |
      | .husky/pre-commit    | exec go run ./cmd/rhino gate run --surface pre-commit                |
      | .husky/commit-msg    | exec go run ./cmd/rhino gate run --surface commit-msg --message-file "$1" |
      | .husky/pre-push      | exec go run ./cmd/rhino gate run --surface pre-push --push-updates-stdin |
    And file ".github/workflows/pr-quality-gate.yml" contains this Markdown:
      """
      on:
        pull_request:
      jobs:
        quality-gate:
          runs-on: ubuntu-24.04
          steps:
            - run: go run ./cmd/rhino gate run --surface pull-request
      """

  # Behaviour: RB-10
  Scenario: Every surface with entries is called by a hook or a workflow step (W01 passes)
    When I invoke the CLI with "gate|wiring|validate"
    Then the exit code is 0
    And stdout contains "no findings"

  Scenario: A surface with entries that nothing calls is reported (W01)
    Given file ".husky/pre-push" contains this Markdown:
      """
      exec echo "pushed"
      """
    When I invoke the CLI with "gate|wiring|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "W01"

  Scenario: A hook call names only surfaces the registry declares (W02 passes)
    When I invoke the CLI with "gate|wiring|validate"
    Then the exit code is 0

  Scenario: A hook that calls a surface the registry does not declare is reported (W02)
    Given file ".husky/pre-rebase" contains this Markdown:
      """
      exec go run ./cmd/rhino gate run --surface main
      """
    When I invoke the CLI with "gate|wiring|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "W02"

  Scenario: A pull-request surface that a workflow step calls passes (W03 passes)
    When I invoke the CLI with "gate|wiring|validate"
    Then the exit code is 0

  Scenario: A pull-request surface that no workflow step calls is reported (W03)
    Given file ".github/workflows/pr-quality-gate.yml" contains this Markdown:
      """
      on:
        pull_request:
      jobs:
        quality-gate:
          runs-on: ubuntu-24.04
          steps:
            - run: echo "checked"
      """
    When I invoke the CLI with "gate|wiring|validate"
    Then the exit code is 1
    And stderr contains "W03"

  Scenario: A local surface with a hook file of its own name passes (W04 passes)
    When I invoke the CLI with "gate|wiring|validate"
    Then the exit code is 0

  Scenario: A local surface whose hook file is missing is reported (W04)
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        composition:
          pull-request:
            relation: exact
        entries:
          - id: format-staged
            command:
              executable: formatter
            run-on:
              pre-commit: {}
              pull-request: {}
          - id: commit-message-lint
            command:
              executable: linter
            run-on:
              commit-msg: {}
              pull-request: {}
          - id: leak-scan
            command:
              executable: scanner
            run-on:
              pre-push: {}
              pull-request: {}
        wiring:
          hooks:
            directory: .git-hooks
          ci:
            directory: .github/workflows
      """
    When I invoke the CLI with "gate|wiring|validate"
    Then the exit code is 1
    And stderr contains "W04"

  Scenario: A step that calls gate run without a suppressed failure passes (W05 passes)
    When I invoke the CLI with "gate|wiring|validate"
    Then the exit code is 0

  Scenario: A hook that suppresses its gate failure is reported (W05)
    Given file ".husky/pre-push" contains this Markdown:
      """
      exec go run ./cmd/rhino gate run --surface pre-push --push-updates-stdin || true
      """
    When I invoke the CLI with "gate|wiring|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "W05"

  Scenario: A workflow step that continues on error after gate run is reported (W05)
    Given file ".github/workflows/pr-quality-gate.yml" contains this Markdown:
      """
      on:
        pull_request:
      jobs:
        quality-gate:
          runs-on: ubuntu-24.04
          steps:
            - run: go run ./cmd/rhino gate run --surface pull-request
              continue-on-error: true
      """
    When I invoke the CLI with "gate|wiring|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "W05"

  Scenario: A workflow step that installs the release its lock names passes (W06 passes)
    Given file "hippo.lock" contains this Markdown:
      """
      version=v0.13.4
      commit=007084b3382f1d7d2dc1373458364de9dcfa0a0b
      """
    And file ".github/workflows/pr-quality-gate.yml" contains this Markdown:
      """
      on:
        pull_request:
      jobs:
        quality-gate:
          runs-on: ubuntu-24.04
          steps:
            - run: gh release download v0.13.4 --repo wahidyankf/hippo
            - run: go run ./cmd/rhino gate run --surface pull-request
      """
    When I invoke the CLI with "gate|wiring|validate"
    Then the exit code is 0

  Scenario: A workflow step that installs a release other than its lock names is reported (W06)
    Given file "hippo.lock" contains this Markdown:
      """
      version=v0.13.4
      commit=007084b3382f1d7d2dc1373458364de9dcfa0a0b
      """
    And file ".github/workflows/pr-quality-gate.yml" contains this Markdown:
      """
      on:
        pull_request:
      jobs:
        quality-gate:
          runs-on: ubuntu-24.04
          steps:
            - run: gh release download v0.12.0 --repo wahidyankf/hippo
            - run: go run ./cmd/rhino gate run --surface pull-request
      """
    When I invoke the CLI with "gate|wiring|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "W06"

  Scenario: A local surface whose entries are on its declared allowed list passes (W07 passes)
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        composition:
          pull-request:
            relation: exact
        entries:
          - id: format-staged
            command:
              executable: formatter
            run-on:
              pre-commit: {}
              pull-request: {}
          - id: commit-message-lint
            command:
              executable: linter
            run-on:
              commit-msg: {}
              pull-request: {}
          - id: leak-scan
            command:
              executable: scanner
            run-on:
              pre-push: {}
              pull-request: {}
          - id: public-scan
            command:
              executable: public-scanner
            run-on:
              pre-push: {}
              pull-request: {}
        wiring:
          hooks:
            directory: .husky
          ci:
            directory: .github/workflows
          local-surfaces:
            pre-push: [leak-scan, public-scan]
      """
    When I invoke the CLI with "gate|wiring|validate"
    Then the exit code is 0

  Scenario: A local surface that holds an entry outside its allowed list is reported (W07)
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        composition:
          pull-request:
            relation: exact
        entries:
          - id: format-staged
            command:
              executable: formatter
            run-on:
              pre-commit: {}
              pull-request: {}
          - id: commit-message-lint
            command:
              executable: linter
            run-on:
              commit-msg: {}
              pull-request: {}
          - id: leak-scan
            command:
              executable: scanner
            run-on:
              pre-push: {}
              pull-request: {}
          - id: public-scan
            command:
              executable: public-scanner
            run-on:
              pre-push: {}
              pull-request: {}
        wiring:
          hooks:
            directory: .husky
          ci:
            directory: .github/workflows
      """
    When I invoke the CLI with "gate|wiring|validate|--output|json"
    Then the exit code is 1
    And stdout contains "public-scan"
    And the first stdout JSON violation kind is "W07"

  Scenario: A repository with no hosted workflows declares a reason and passes
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries: []
        wiring:
          hooks:
            directory: .git-hooks
          ci:
            none: This repository hosts no workflows.
      """
    When I invoke the CLI with "gate|wiring|validate"
    Then the exit code is 0

  # Behaviour: RB-75
  Scenario: A repository that declares no wiring is refused
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries: []
      """
    When I invoke the CLI with "gate|wiring|validate|--output|json"
    Then the exit code is 2
    And stderr contains "rhino.config.undeclared"

  Scenario: A workflow directory declared together with a reason of no workflows is refused
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries: []
        wiring:
          hooks:
            directory: .husky
          ci:
            directory: .github/workflows
            none: This repository hosts no workflows.
      """
    When I invoke the CLI with "gate|wiring|validate|--output|json"
    Then the exit code is 2
    And stderr contains "rhino.config.unusable"
    And stderr contains "gates.wiring.ci"

  Scenario: A declared no-workflows reason with no text is refused
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries: []
        wiring:
          hooks:
            directory: .husky
          ci:
            none: ""
      """
    When I invoke the CLI with "gate|wiring|validate|--output|json"
    Then the exit code is 2
    And stderr contains "rhino.config.unusable"
    And stderr contains "gates.wiring.ci.none"
