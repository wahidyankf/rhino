Feature: Repository configuration contract

  Every value RHINO enforces arrives from the consuming repository's
  repo-config.yml. There is no default for any of them, so a configuration
  RHINO cannot understand is a fault to report rather than a gap to fill in.

  Scenario: A well-formed configuration validates
    Given the repository declares a complete configuration
    When I run the "repo-config" validator
    Then the exit code is 0

  Scenario: A configuration declaring none of the optional sections is complete
    Given the repository declares a complete configuration
    When I run the "repo-config" validator
    Then the exit code is 0
    And there are no violations

  Scenario: An absent configuration file is a configuration fault
    Given the repository has no configuration file
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr names the missing configuration file

  Scenario: A blank line does not end the search for the schema comment
    Given the configuration file is this text:
      """

      # schema: rhino/repo-config/v1
      md-mermaid:
        node-label-graphemes: 32
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "missing field `edge-label-graphemes`"

  Scenario: An unrecognized schema identifier is refused
    Given the repository declares the schema "rhino/repo-config/v99"
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr names the unrecognized schema

  Scenario: The predecessor schema spelling is accepted as an alias
    Given the repository declares the schema "rhino-cli/repo-config/v1"
    When I run the "repo-config" validator
    Then the exit code is 0

  Scenario: An unknown key inside an owned section is refused
    Given the repository declares a complete configuration
    And the configuration adds the unknown key "node-label-grapheme" to "md-mermaid"
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr names the unknown key and its position

  Scenario: An unknown top-level section is ignored
    Given the repository declares a complete configuration
    And the configuration adds the top-level section "env-contract"
    When I run the "repo-config" validator
    Then the exit code is 0

  Scenario: A missing required key is refused
    Given the repository declares a complete configuration
    And the configuration omits "md-mermaid.edge-label-graphemes"
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr names the missing key

  Scenario: A value of the wrong type is refused
    Given the repository declares a complete configuration
    And the configuration sets "md-mermaid.node-label-graphemes" to "thirty-two"
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr names the offending key and its position

  Scenario Outline: A declared path may not escape the repository root
    Given the repository declares a complete configuration
    And the configuration sets "harness-parity.canonical.skills-root" to "<path>"
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr names the offending key and its position

    Examples:
      | path                |
      | ../outside          |
      | /absolute           |
      | rules/../../outside |

  Scenario: An empty harness roster is a legal declaration
    Given the repository declares a configuration with an empty harness roster and no canonical skill or agent root
    When I run the "repo-config" validator
    Then the exit code is 0

  Scenario: An empty harness roster alongside a declared canonical root is refused
    Given the repository declares a configuration with an empty harness roster and a canonical skills root
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr names the canon that has nowhere to be reconciled

  Scenario: An empty harness roster alongside a required MCP server is refused
    Given the repository declares a configuration with an empty harness roster and no canonical skill or agent root
    And a required MCP server is declared
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "required-mcp"
    And stderr contains "declared alongside an empty harness roster"

  Scenario: A repository whose harnesses reach no capability server is legal
    Given the repository declares a complete configuration
    And no capability server is required and no harness declares a capability file
    When I run the "repo-config" validator
    Then the exit code is 0

  Scenario: A required server no harness declares a capability file for is refused
    Given the repository declares a complete configuration
    And no harness declares a capability file
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "declares no capability file"

  Scenario: A capability file no required server reads is refused
    Given the repository declares a complete configuration
    And no capability server is required
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "no required server names anything to find in it"

  Scenario: A repository whose harnesses express no canonical agents is legal
    Given the repository declares a complete configuration
    And no canonical agents are declared and no harness expresses them
    When I run the "repo-config" validator
    Then the exit code is 0

  Scenario: A canonical agents root no harness expresses is refused
    Given the repository declares a complete configuration
    And no harness declares an agent adapter
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "declares no agent adapter"

  Scenario: An agent adapter with no canonical agents root is refused
    Given the repository declares a complete configuration
    And no canonical agents are declared
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "no `agents-root` names what it would express"

  Scenario: A skill adapter with no canonical skills root is refused
    Given the repository declares a complete configuration
    And harness "beta" declares a command directory
    And no canonical skills are declared
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "no `skills-root` names what it would express"

  Scenario: Canonical agents whose permission fields are unnamed are refused
    Given the repository declares a complete configuration
    And the configuration omits "harness-parity.canonical.declaration"
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "granting and denying nothing"

  Scenario: A declaration shape with no canonical agents to describe is refused
    Given the repository declares a complete configuration
    And the canonical agents are gone and only their declaration shape remains
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "no canonical agent to read"

  Scenario Outline: A canonical root and the route its adapters carry are one declaration
    Given the repository declares a complete configuration
    And the configuration omits "<key>"
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "<reason>"

    Examples:
      | key                                  | reason                            |
      | harness-parity.canonical.agent-route | required alongside `agents-root`  |
      | harness-parity.canonical.skill-route | required alongside `skills-root`  |

  Scenario: A translation naming no capability is refused
    Given the repository declares a complete configuration
    And a harness translates a capability it does not name
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "conditional translation naming no capability"

  Scenario: A translation naming an undeclared capability is refused
    Given the repository declares a complete configuration
    And a harness translates a capability the repository never declared
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "nothing would ever trigger this translation"

  Scenario: A validator refuses to run against a configuration it cannot read
    Given the repository declares a complete configuration
    And the configuration adds the unknown key "trees-list" to "governance-directory-map"
    When I run the "directory-map" validator
    Then the exit code is 2
    And no directories were inspected

  Scenario: A configuration that declares no schema is refused
    Given the configuration file is this text:
      """

      # written by hand
      md-mermaid:
        node-label-graphemes: 32
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr names the missing schema declaration

  Scenario: A configuration whose first content is not a comment declares no schema
    Given the configuration file is this text:
      """
      md-mermaid:
        node-label-graphemes: 32
      # schema: rhino/repo-config/v1
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr names the missing schema declaration

  Scenario: A configuration file that cannot be read is refused
    Given the repository declares a complete configuration
    And the configuration file cannot be read
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr names the unreadable configuration file

  Scenario Outline: A declared harness roster requires the keys that reconcile it
    Given the repository declares a complete configuration
    And the configuration omits "<key>"
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr names the missing key

    Examples:
      | key                                   |
      | harness-parity.canonical.skills-root  |
      | harness-parity.canonical.agents-root  |

  Scenario: A complete tier mapping carries both of its fields
    Given the repository declares a complete configuration
    And the repository maps harness "codex" tier "plan" to model "gpt-5.6-sol" at effort "high"
    And the repository maps harness "codex" tier "execution" to model "gpt-5.6-terra" at effort "xhigh"
    When I run the "repo-config" validator
    Then the exit code is 0

  Scenario: An unmapped tier is an answer rather than an omission
    Given the repository declares a complete configuration
    And the repository maps harness "codex" tier "plan" to model "gpt-5.6-sol" at effort "high"
    When I run the "repo-config" validator
    Then the exit code is 0
    And no output names an optional section

  Scenario Outline: A broken tier mapping is refused before anything is generated
    Given the repository declares a complete configuration
    And the repository declares the model-tier mapping "<mapping>"
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "<reason>"

    Examples:
      | mapping                                          | reason                          |
      | {}                                               | declares no harness             |
      | {codex: {}}                                      | declares no tier                |
      | {codex: {plan: null}}                            | declares no model or effort     |
      | {codex: {plan: {model: gpt-5.6-sol}}}            | declares no effort              |
      | {codex: {plan: {effort: high}}}                  | declares no model               |
      | {codex: {plan: {model: '', effort: high}}}        | declares an empty model         |
      | {codex: {plan: {model: gpt-5.6-sol, effort: ''}}} | declares an empty effort        |

  Scenario: A tier mapping keyed by an unsupported harness is refused
    Given the repository declares a complete configuration
    And the repository declares the model-tier mapping "{gemini: {plan: {model: gpt-5.6-sol, effort: high}}}"
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "gemini"

  Scenario: A tier mapping keyed by a tier outside the closed set is refused
    Given the repository declares a complete configuration
    And the repository declares the model-tier mapping "{codex: {architect: {model: gpt-5.6-sol, effort: high}}}"
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "architect"

  Scenario: A mapping keyed by an agent name is not a tier mapping
    Given the repository declares a complete configuration
    And the repository declares the model-tier mapping "{codex: {plan-checker: {model: gpt-5.6-sol, effort: high}}}"
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "plan-checker"
