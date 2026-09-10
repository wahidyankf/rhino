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

  # -- ose/repo-config/v2 -------------------------------------------------------
  #
  # v2 is a second schema beside v1 rather than a replacement. Five consumers
  # run v1 today, and a reader that stopped understanding it would turn every
  # one of their recorded vectors into a configuration error and call that
  # preservation. The two are told apart without ambiguity: v1 declares its
  # schema in a leading comment, v2 in a leading `schema` key.

  Scenario: A v2 configuration carrying only its required keys validates
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A v2 configuration carrying every optional section validates
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: public
      governance:
        local-categories:
          - development/example
      model-tiers:
        codex:
          execution:
            model: gpt-5.6-terra
            effort: xhigh
      gates:
        - id: public-safety
          kind: check
          run:
            - ./gates/public-safety.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
            - ci
      extensions:
        rhino:
          diagram-mode: ascii
      """
    When I run the "repo-config" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A v1 comment and a v2 key in one document is not one document
    Given the configuration file is this text:
      """
      # schema: rhino/repo-config/v1
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "declares two schemas"

  Scenario Outline: A v2 document missing one of its required keys is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    And the v2 configuration omits "<key>"
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "<reason>"

    Examples:
      | key        | reason                            |
      | visibility | visibility: the schema requires it |
      | gates      | gates: the schema requires it     |

  Scenario: A v2 document declaring an empty gates list is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates: []
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "gates: declares no gate"

  Scenario Outline: A v2 visibility outside the closed pair is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: <visibility>
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "visibility: is neither public nor private"

    Examples:
      | visibility |
      | internal   |
      | Public     |
      | ''         |

  Scenario: A v2 top-level key the schema does not keep is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      provenance: hand-written
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "provenance: is not a key this schema keeps"

  Scenario: A v2 top-level key out of canonical order is refused
    Given the configuration file is this text:
      """
      visibility: private
      schema: ose/repo-config/v2
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "schema: is out of canonical key order"

  Scenario: An optional v2 section placed after gates is out of order
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      governance:
        local-categories:
          - development/example
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "governance: is out of canonical key order"

  Scenario Outline: An optional v2 section declared with nothing in it is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      <section>
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "<reason>"

    Examples:
      | section                     | reason                                         |
      | governance: {}              | governance: is declared with nothing in it     |
      | model-tiers: {}             | model-tiers: is declared with nothing in it    |
      | governance: {local-categories: []} | local-categories: is declared with nothing in it |

  Scenario: An empty v2 extensions section is refused rather than ignored
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      extensions: {}
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "extensions: is declared with nothing in it"

  Scenario: A v2 extension payload core does not understand is still accepted
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      extensions:
        beaver:
          nest-depth: 4
          moods:
            - busy
      """
    When I run the "repo-config" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A v2 extension profile that is not a namespace is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      extensions:
        rhino: ascii
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "rhino: is not an extension namespace"

  Scenario: A v2 gate entry key the contract does not keep is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
          timeout: 30
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "timeout: is not a key a gate entry keeps"

  Scenario: A v2 gate entry whose keys are out of canonical order is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          run:
            - ./gates/hygiene.sh
          kind: check
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "kind: is out of canonical key order"

  Scenario Outline: A v2 gate kind outside the closed pair is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: <kind>
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "kind: is neither check nor mutation"

    Examples:
      | kind      |
      | inspect   |
      | Check     |
      | ''        |

  Scenario: A v2 gate identifier that is not lowercase-hyphen is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: Hygiene_Gate
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "id: is not a lowercase-hyphen identifier"

  Scenario: Two v2 gates sharing one identifier are refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
        - id: hygiene
          kind: check
          run:
            - ./gates/again.sh
          surfaces:
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "id: hygiene is declared twice"

  Scenario: A v2 run written as a shell string is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run: ./gates/hygiene.sh --all
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "run: is not an argument vector"

  Scenario: A v2 run holding no argument is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run: []
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "run: is declared with nothing in it"

  Scenario: A v2 run whose argument holds a space is an ordinary argument
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
            - --title
            - two words
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A v2 gate declaring no surface is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces: []
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "surfaces: is declared with nothing in it"

  Scenario: A v2 surface outside the closed set is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
            - post-merge
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "post-merge: is not a surface this schema keeps"

  Scenario: A v2 gate naming one surface twice is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "pre-commit: is declared twice"

  Scenario: A v2 surface list out of canonical order is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - pre-commit
            - commit-msg
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "commit-msg: is out of canonical surface order"

  Scenario Outline: A v2 mutation mapped away from pre-commit is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
        - id: formatter
          kind: mutation
          run:
            - ./gates/format.sh
          surfaces:
            - <surface>
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "surfaces: a mutation may run only at pre-commit"

    Examples:
      | surface    |
      | commit-msg |
      | pre-push   |
      | ci         |

  Scenario: A v2 mutation confined to pre-commit is accepted
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
        - id: formatter
          kind: mutation
          run:
            - ./gates/format.sh
          surfaces:
            - pre-commit
      """
    When I run the "repo-config" validator
    Then the exit code is 0
    And there are no violations

  Scenario Outline: A v2 document leaving a Git hook surface unmanned is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    And the v2 configuration drops the surface "<surface>"
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "<surface>: no gate runs at this surface"

    Examples:
      | surface    |
      | commit-msg |
      | pre-commit |
      | pre-push   |

  Scenario: A remote-free repository mans pre-push with a reject gate
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
        - id: no-remote
          kind: check
          run:
            - ./gates/reject-push.sh
          surfaces:
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A v2 document declaring no ci surface is complete
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 0
    And no output names an optional section

  Scenario Outline: A public v2 repository that does not run public-safety first is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: public
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - <surface>
        - id: public-safety
          kind: check
          run:
            - ./gates/public-safety.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
            - ci
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "<surface>: public-safety does not run first at this surface"

    Examples:
      | surface    |
      | commit-msg |
      | pre-commit |
      | pre-push   |
      | ci         |

  Scenario: A public v2 repository declaring no public-safety gate is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: public
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "gates: declares no public-safety gate"

  Scenario: A private v2 repository needs no public-safety gate
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A public v2 repository running public-safety first everywhere is accepted
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: public
      gates:
        - id: public-safety
          kind: check
          run:
            - ./gates/public-safety.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
            - ci
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - pre-commit
            - ci
      """
    When I run the "repo-config" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A v2 model-tier section carries the same closed contract as v1
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      model-tiers:
        gemini:
          plan:
            model: gpt-5.6-sol
            effort: high
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "gemini"

  Scenario: A v2 local category outside governed path naming is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      governance:
        local-categories:
          - Development/Example
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "Development/Example: is not a governed layer and category"

  Scenario: A v2 local category list out of sorted order is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      governance:
        local-categories:
          - development/example
          - conventions/example
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "conventions/example: is out of sorted order"

  Scenario: A v2 local category named twice is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      governance:
        local-categories:
          - development/example
          - development/example
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "development/example: is declared twice"

  Scenario: The v2 reader refuses a schema identifier it does not know
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v3
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "ose/repo-config/v3"

  Scenario: A v2 repository declaring no section for a command is refused by the same rule v1 uses
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "word-budget" validator
    Then the exit code is 2
    And stderr contains "the section is not declared, and RHINO holds no default for it"

  Scenario: A v2 visibility written with nothing after it is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility:
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "visibility: is neither public nor private"

  Scenario: A v2 governance section that is not a section is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      governance: local
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "governance: keeps only `local-categories`"

  Scenario: A v2 line that names no key at all is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      provenance
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "is not a key this schema keeps"

  Scenario: A v2 line whose key is empty is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      : hand-written
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "is not a key this schema keeps"

  Scenario: A v2 local category list written inline is read the same way
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      governance: {local-categories: [development/example, workflows/example]}
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A v2 top-level key declared twice is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      visibility: public
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "visibility: is declared twice"

  Scenario: A v2 local category naming no layer is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      governance:
        local-categories:
          - example
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "example: is not a governed layer and category"

  Scenario: A v2 tier mapped to nothing is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      model-tiers:
        codex:
          plan:
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "declares no model or effort"

  Scenario: A v2 gate entry omitting one of its keys is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "surfaces: the gate entry contract requires it"

  # -- v2 carries the validator sections too ------------------------------------
  #
  # v2 arrived carrying only what a gate runner needs, so a repository that
  # declared it to get gate dispatch gave up every command that reads a surface
  # list. That partition is invisible from inside the release: the repository
  # that notices is the first one to want both, and by then a tag exists. The
  # sections below are the same sections v1 declares, read by the same code, so
  # one rule keeps one implementation and a repository states its policy once.

  Scenario: A v2 document declaring a metadata surface is held to it
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      metadata:
        surfaces:
          - glob: "repo-governance/**/*.md"
            schema: governance
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    And file "repo-governance/conventions/file-naming.md" contains this Markdown:
      """
      ---
      description: Defines the durable repository rule for naming one governed file.
      when_to_use: >-
        Use when creating or renaming a governed Markdown document.
      ---

      # File Naming
      """
    When I run the "metadata" validator
    Then the exit code is 0
    And 1 files were inspected

  Scenario: A v2 document declaring a word budget is held to it
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      governance-word-budget:
        count: letters-and-digits
        surfaces:
          - glob: "repo-governance/**/*.md"
            fail: 5
            warn: 3
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    And file "repo-governance/conventions/file-naming.md" contains 40 words
    When I run the "word-budget" validator
    Then the exit code is 1

  Scenario: A v2 document declaring an emoji prohibition is held to it
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      convention-emoji:
        prohibited:
          - glob: "**/*.md"
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    And file "repo-governance/conventions/file-naming.md" contains this Markdown:
      """
      # File Naming 🚀
      """
    When I run the "emoji" validator
    Then the exit code is 1

  Scenario: A v2 validator section written out of canonical order is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      convention-emoji:
        prohibited:
          - glob: "**/*.md"
      metadata:
        surfaces:
          - glob: "repo-governance/**/*.md"
            schema: governance
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "metadata"

  Scenario: A v2 validator section declared with nothing in it is refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      metadata:
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "is declared with nothing in it"

  Scenario: A v2 top-level key outside the schema is still refused
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      md-spelling:
        surfaces:
          - glob: "**/*.md"
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr contains "md-spelling"
