Feature: Quality-gate structure

  A repository that runs bounded quality gates declares where its workflows
  live, which gate families it holds, and the headings, verdicts, and retired
  inputs its gate contract names. RHINO checks only that structure: where each
  gate, propagation, and module sits, which headings each carries, and that no
  gate can run more than three cycles. Whether a gate is well written stays the
  gate's own judgement.

  Background:
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        governance:
          quality-gates:
            root: workflows
            agents: agents
            groups: [plan, quality]
            gate-group: quality
            gate-headings: [Entry, Inputs, Cycle, Verdict]
            verdict-heading: Verdict
            verdicts: [PASS, FAIL]
            retired-inputs: [max-iterations]
            propagation-headings: [Contract, Scope, Executor]
            defaults:
              mode: normal
              max-cycles: 3
            gates:
              - family: plan
      """
    And the repository contains:
      | path                       | content        |
      | workflows/README.md        | # Workflows    |
      | workflows/plan/planning.md | # Planning     |
      | agents/plan-checker.md     | # Plan Checker |
      | agents/plan-fixer.md       | # Plan Fixer   |
    And file "workflows/quality/plan-quality-gate.md" contains this Markdown:
      """
      # Plan Quality Gate

      ## Entry

      Explicit request only.

      ## Inputs

      | Input        | Values     | Default |
      | ------------ | ---------- | ------- |
      | `max-cycles` | 1, 2, or 3 | 3       |

      ## Cycle

      One full audit and one repair.

      ## Verdict

      `PASS` or `FAIL`.

      ## Example Usage

      Run the plan quality gate.
      """
    And file "workflows/quality/plan-propagation.md" contains this Markdown:
      """
      # Plan Propagation

      ## Contract

      The plan family's sole writer.

      ## Scope

      One plan folder.

      ## Executor

      The plan fixer.
      """

  Scenario: A repository whose gates follow the declared structure passes
    When I invoke the CLI with "governance|quality-gates|validate"
    Then the exit code is 0

  Scenario: A repository that declares no quality-gate policy is refused
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        governance: {}
      """
    When I invoke the CLI with "governance|quality-gates|validate"
    Then the exit code is 2
    And stderr contains "policies.governance.quality-gates is not declared"

  Scenario: An entry outside the declared workflow groups is reported (QG01)
    Given the repository contains:
      | path                    | content |
      | workflows/meta/modes.md | # Modes |
    When I invoke the CLI with "governance|quality-gates|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "unexpected-workflow-entry"

  Scenario: A declared family without its gate or propagation is reported (QG02)
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        governance:
          quality-gates:
            root: workflows
            agents: agents
            groups: [plan, quality]
            gate-group: quality
            gate-headings: [Entry, Inputs, Cycle, Verdict]
            verdict-heading: Verdict
            verdicts: [PASS, FAIL]
            retired-inputs: [max-iterations]
            propagation-headings: [Contract, Scope, Executor]
            gates:
              - family: plan
              - family: docs
      """
    And the repository contains:
      | path                   | content        |
      | agents/docs-checker.md | # Docs Checker |
      | agents/docs-fixer.md   | # Docs Fixer   |
    When I invoke the CLI with "governance|quality-gates|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "missing-quality-gate-file"

  Scenario: A gate file outside the gate group is reported (QG03)
    Given the repository contains:
      | path                                | content          |
      | workflows/plan/plan-quality-gate.md | # Misplaced Gate |
    When I invoke the CLI with "governance|quality-gates|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "misplaced-quality-gate-file"

  Scenario: A gate file for an undeclared family is reported (QG03)
    Given file "workflows/quality/ci-propagation.md" contains this Markdown:
      """
      # CI Propagation

      ## Contract

      The CI family's sole writer.

      ## Scope

      The hook wiring.

      ## Executor

      The CI fixer.
      """
    When I invoke the CLI with "governance|quality-gates|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "undeclared-quality-gate-family"

  Scenario: A gate whose required headings are out of order is reported (QG04)
    Given file "workflows/quality/plan-quality-gate.md" contains this Markdown:
      """
      # Plan Quality Gate

      ## Entry

      Explicit request only.

      ## Inputs

      | Input        | Values     | Default |
      | ------------ | ---------- | ------- |
      | `max-cycles` | 1, 2, or 3 | 3       |

      ## Verdict

      `PASS` or `FAIL`.

      ## Cycle

      One full audit and one repair.
      """
    When I invoke the CLI with "governance|quality-gates|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "gate-heading-out-of-order"

  Scenario: A propagation without a required heading is reported (QG05)
    Given file "workflows/quality/plan-propagation.md" contains this Markdown:
      """
      # Plan Propagation

      ## Contract

      The plan family's sole writer.

      ## Scope

      One plan folder.
      """
    When I invoke the CLI with "governance|quality-gates|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "missing-propagation-heading"

  Scenario Outline: A gate that can run beyond three cycles is reported (QG06)
    Given file "workflows/quality/plan-quality-gate.md" contains this Markdown:
      """
      # Plan Quality Gate

      ## Entry

      Explicit request only.

      ## Inputs

      | Input        | Values   | Default   |
      | ------------ | -------- | --------- |
      | `max-cycles` | <values> | <default> |
      | `<extra>`    | any      | none      |

      ## Cycle

      One full audit and one repair.

      ## Verdict

      `PASS` or `FAIL`.
      """
    When I invoke the CLI with "governance|quality-gates|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "<kind>"

    Examples:
      | values     | default | extra          | kind                  |
      | 1 to 5     | 5       | mode           | unbounded-gate-cycles |
      | 1, 2, or 3 | 3       | max-iterations | retired-gate-input    |

  Scenario: A gate verdict outside the declared verdicts is reported (QG07)
    Given file "workflows/quality/plan-quality-gate.md" contains this Markdown:
      """
      # Plan Quality Gate

      ## Entry

      Explicit request only.

      ## Inputs

      | Input        | Values     | Default |
      | ------------ | ---------- | ------- |
      | `max-cycles` | 1, 2, or 3 | 3       |

      ## Cycle

      One full audit and one repair.

      ## Verdict

      `PASS`, `PARTIAL`, or `FAIL`.
      """
    When I invoke the CLI with "governance|quality-gates|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "unknown-gate-verdict"

  Scenario: A declared family without its checker and fixer agents is reported (QG08)
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        governance:
          quality-gates:
            root: workflows
            agents: agents/canonical
            groups: [plan, quality]
            gate-group: quality
            gate-headings: [Entry, Inputs, Cycle, Verdict]
            verdict-heading: Verdict
            verdicts: [PASS, FAIL]
            retired-inputs: [max-iterations]
            propagation-headings: [Contract, Scope, Executor]
            gates:
              - family: plan
      """
    When I invoke the CLI with "governance|quality-gates|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "missing-gate-agent"

  Scenario: A default max-cycles above three is refused before any file is read (QG09)
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        governance:
          quality-gates:
            root: workflows
            agents: agents
            groups: [plan, quality]
            gate-group: quality
            gate-headings: [Entry, Inputs, Cycle, Verdict]
            verdict-heading: Verdict
            verdicts: [PASS, FAIL]
            retired-inputs: [max-iterations]
            propagation-headings: [Contract, Scope, Executor]
            defaults:
              max-cycles: 4
            gates:
              - family: plan
      """
    When I invoke the CLI with "governance|quality-gates|validate"
    Then the exit code is 2
    And stderr contains "defaults.max-cycles"

  Scenario Outline: A declaration the structure check could not apply is refused by repo-config validate
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        governance:
          quality-gates:
            root: <root>
            agents: agents
            groups: <groups>
            gate-group: quality
            gate-headings: <gate-headings>
            verdict-heading: <verdict-heading>
            verdicts: <verdicts>
            retired-inputs: [<retired>]
            propagation-headings: [Contract, Scope, Executor]
            gates: <gates>
      """
    When I invoke the CLI with "repo-config|validate"
    Then the exit code is 2
    And stderr contains "<reason>"

    Examples:
      | root         | groups                | gate-headings           | verdict-heading | verdicts    | retired        | gates                            | reason                          |
      | ../workflows | [plan, quality]       | [Entry, Cycle, Verdict] | Verdict         | [PASS]      | max-iterations | [{family: plan}]                 | exact repository-relative paths |
      | workflows    | [plan]                | [Entry, Cycle, Verdict] | Verdict         | [PASS]      | max-iterations | [{family: plan}]                 | include `gate-group`            |
      | workflows    | [plan, quality, plan] | [Entry, Cycle, Verdict] | Verdict         | [PASS]      | max-iterations | [{family: plan}]                 | include `gate-group`            |
      | workflows    | [plan, quality]       | [Entry, Cycle, Verdict] | Verdict         | [PASS]      | max-iterations | [{family: plan}, {family: plan}] | unique simple name              |
      | workflows    | [plan, quality]       | [Entry, Cycle, Verdict] | Verdict         | [PASS]      | max-iterations | [{family: Plan}]                 | unique simple name              |
      | workflows    | [plan, quality]       | [Entry, Entry, Verdict] | Verdict         | [PASS]      | max-iterations | [{family: plan}]                 | non-empty and unique            |
      | workflows    | [plan, quality]       | [Entry, Cycle, Verdict] | Verdict         | []          | max-iterations | [{family: plan}]                 | non-empty and unique            |
      | workflows    | [plan, quality]       | [Entry, Cycle, Verdict] | Verdict         | [PASS]      | " "            | [{family: plan}]                 | non-empty and unique            |
      | workflows    | [plan, quality]       | [Entry, Cycle, Verdict] | Outcome         | [PASS]      | max-iterations | [{family: plan}]                 | one of `gate-headings`          |

  Scenario: A repository default below three does not lower what a gate may declare
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        governance:
          quality-gates:
            root: workflows
            agents: agents
            groups: [plan, quality]
            gate-group: quality
            gate-headings: [Entry, Inputs, Cycle, Verdict]
            verdict-heading: Verdict
            verdicts: [PASS, FAIL]
            retired-inputs: [max-iterations]
            propagation-headings: [Contract, Scope, Executor]
            defaults:
              max-cycles: 1
            gates:
              - family: plan
      """
    When I invoke the CLI with "governance|quality-gates|validate"
    Then the exit code is 0

  Scenario: A module directory without a sibling entrypoint is reported (QG10)
    Given the repository contains:
      | path                                                     | content |
      | workflows/quality/plan-quality-gate-details/001-steps.md | # Steps |
    When I invoke the CLI with "governance|quality-gates|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "unpaired-workflow-module-directory"

  Scenario: A split workflow that fits its word budget as one file is reported (QG11)
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        governance:
          word-budget:
            count: letters-and-digits
            surfaces:
              - glob: "workflows/**/*.md"
                fail: 50
          quality-gates:
            root: workflows
            agents: agents
            groups: [plan, quality]
            gate-group: quality
            gate-headings: [Entry, Inputs, Cycle, Verdict]
            verdict-heading: Verdict
            verdicts: [PASS, FAIL]
            retired-inputs: [max-iterations]
            propagation-headings: [Contract, Scope, Executor]
            gates:
              - family: plan
      """
    And the repository contains:
      | path                                  | content                                        |
      | workflows/plan/execution.md           | # Execution\n\nSee the modules.                |
      | workflows/plan/execution/README.md    | # Execution Modules\n\n- [Steps](001-steps.md) |
      | workflows/plan/execution/001-steps.md | # Steps\n\nRun each step in order.             |
    When I invoke the CLI with "governance|quality-gates|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "unnecessary-workflow-split"
