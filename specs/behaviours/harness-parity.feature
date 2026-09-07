Feature: Coding-harness parity

  A repository declares one canonical instruction body, one canonical skill
  bundle per skill, one canonical agent prompt per agent, and a roster of the
  harnesses that must reach them. RHINO reconciles the canon against every
  declared harness. It knows no harness by name; the roster is data.

  Background:
    Given the repository declares a harness roster of "alpha", "beta", and "gamma"

  Scenario: Canonical instructions, skills, agents, and capability declarations pass
    Given a valid one-skill one-agent one-capability harness contract
    When I inspect harness parity
    Then harness-parity validation succeeds with 3 harnesses, 1 skill, 1 agent, and 3 reconciled capability declarations

  Scenario: The canonical instruction body must exist
    Given a valid one-skill one-agent one-capability harness contract
    And the canonical instruction body is absent
    When I inspect harness parity
    Then the only violation starts with "root-instructions.md: missing-instruction:"

  Scenario: A declared instruction adapter must exist
    Given a valid one-skill one-agent one-capability harness contract
    And the declared instruction adapter is absent
    When I inspect harness parity
    Then the only violation starts with "adapter-instructions.md: missing-instruction-adapter:"

  Scenario: The instruction adapter may contain only the canonical import
    Given a valid one-skill one-agent one-capability harness contract
    And the declared instruction adapter contains extra instructions
    When I inspect harness parity
    Then the harness-parity violations include "invalid-instruction-adapter"

  Scenario: A declared instruction adapter is optional
    Given the repository declares no instruction adapter
    And a valid one-skill one-agent one-capability harness contract
    When I inspect harness parity
    Then harness-parity validation succeeds

  Scenario: With no adapter declared, no file may import the canonical instructions
    Given the repository declares no instruction adapter
    And a valid one-skill one-agent one-capability harness contract
    And a file imports the canonical instruction body
    When I inspect harness parity
    Then the harness-parity violations include "unexpected-instruction-source"

  Scenario: A harness instruction overlay is a competing source
    Given a valid one-skill one-agent one-capability harness contract
    And harness "gamma" declares an instruction overlay
    When I inspect harness parity
    Then the harness-parity violations include "unexpected-instruction-source"

  Scenario: Additional or nested instruction sources fail
    Given a valid one-skill one-agent one-capability harness contract
    And a nested repository instruction file exists
    When I inspect harness parity
    Then the harness-parity violations include "unexpected-instruction-source"

  Scenario: A harness declaring a command directory needs one thin wrapper per skill
    Given a valid one-skill one-agent one-capability harness contract
    And harness "beta" declares a command directory
    And the skill wrapper for "beta" is missing
    When I inspect harness parity
    Then the harness-parity violations include "missing-skill-adapter"

  Scenario: A harness declaring no command directory needs no wrappers
    Given a valid one-skill one-agent one-capability harness contract
    And harness "beta" declares a command directory
    And harness "alpha" declares no command directory
    When I inspect harness parity
    Then harness-parity validation succeeds

  Scenario: A skill wrapper's description cannot drift from the skill
    Given a valid one-skill one-agent one-capability harness contract
    And harness "beta" declares a command directory
    And the skill wrapper for "beta" has a stale description
    When I inspect harness parity
    Then the only violation starts with "adapters/beta/commands/tidy.md: skill-content-divergence: the wrapper's description"

  Scenario: A skill wrapper may not grow a body of its own
    Given a valid one-skill one-agent one-capability harness contract
    And harness "beta" declares a command directory
    And the skill wrapper for "beta" has an extra body
    When I inspect harness parity
    Then the only violation starts with "adapters/beta/commands/tidy.md: skill-content-divergence: a wrapper may contain"

  Scenario: A skill wrapper must declare the skill it routes to
    Given a valid one-skill one-agent one-capability harness contract
    And harness "beta" declares a command directory
    And the skill wrapper for "beta" declares another name
    When I inspect harness parity
    Then the only violation starts with "adapters/beta/commands/tidy.md: skill-content-divergence: the wrapper declares a name"

  Scenario: Skill bodies and supporting resources affect the contract digest
    Given a valid one-skill one-agent one-capability harness contract
    When I inspect and remember the harness-parity digest
    And I add a canonical skill supporting resource
    And I inspect harness parity again
    Then harness-parity validation succeeds
    And the harness-parity digest changed

  Scenario: Files outside the canon do not affect the contract digest
    Given a valid one-skill one-agent one-capability harness contract
    When I inspect and remember the harness-parity digest
    And I add a file outside the canon
    And I inspect harness parity again
    Then harness-parity validation succeeds
    And the harness-parity digest is unchanged

  Scenario: A canonical skill with no declaration fails
    Given a valid one-skill one-agent one-capability harness contract
    And the canonical skill carries no declaration
    When I inspect harness parity
    Then the only violation starts with "canon/skills/tidy/SKILL.md: invalid-skill: a skill needs a declaration"

  Scenario: A canonical skill with no description fails
    Given a valid one-skill one-agent one-capability harness contract
    And the canonical skill declares no description
    When I inspect harness parity
    Then the only violation starts with "canon/skills/tidy/SKILL.md: invalid-skill: a skill declaration needs both"

  Scenario: A canonical skill must be declared under the directory it lives in
    Given a valid one-skill one-agent one-capability harness contract
    And a canonical skill declares a name that is not its directory
    When I inspect harness parity
    Then the only violation starts with "canon/skills/tidy-again/SKILL.md: invalid-skill: the declared name"

  Scenario: Every canonical agent has one adapter per declared harness
    Given the repository declares a harness roster of "alpha", "beta", and "delta"
    And a valid one-skill one-agent one-capability harness contract
    And the agent adapter for "delta" is missing
    When I inspect harness parity
    Then the harness-parity violations include "missing-agent-adapter"
    And 3 harnesses were inspected

  Scenario: Missing and extra agent adapters fail, each named for what it is
    Given a valid one-skill one-agent one-capability harness contract
    And the agent adapter for "alpha" is missing
    And an unexpected agent adapter exists
    When I inspect harness parity
    Then the harness-parity violation for "adapters/alpha/agents/reviewer.md" is "missing-agent-adapter"
    And the harness-parity violation for "adapters/alpha/agents/ghost.md" is "unexpected-agent-adapter"

  Scenario: Extra prompt content and semantic drift are reported apart
    Given a valid one-skill one-agent one-capability harness contract
    And the agent adapter for "alpha" contains extra prompt instructions
    And the agent adapter for "gamma" weakens a denied capability
    When I inspect harness parity
    Then the harness-parity violation for "adapters/alpha/agents/reviewer.md" is "agent-prompt-divergence"
    And the harness-parity violation for "adapters/gamma/agents/reviewer.md" is "agent-semantic-divergence"

  Scenario: An adapter may not stop denying what the canon denies
    Given a valid one-skill one-agent one-capability harness contract
    And the agent adapter for "beta" stops denying a capability
    When I inspect harness parity
    Then the harness-parity violation for "adapters/beta/agents/reviewer.md" is "agent-semantic-divergence"

  Scenario: An agent may declare only vocabulary the repository declared
    Given a valid one-skill one-agent one-capability harness contract
    And the canonical agent requires a capability outside the declared vocabulary
    When I inspect harness parity
    Then the harness-parity violations include "unknown-capability"

  Scenario: An agent may declare only constraints the repository declared
    Given a valid one-skill one-agent one-capability harness contract
    And the canonical agent declares a constraint outside the declared vocabulary
    When I inspect harness parity
    Then the harness-parity violations include "unknown-capability"

  Scenario: An adapter may not drop a declared constraint
    Given a valid one-skill one-agent one-capability harness contract
    And the agent adapter for "beta" drops a declared constraint
    When I inspect harness parity
    Then the harness-parity violations include "agent-semantic-divergence"

  Scenario: Equivalent required-capability declarations pass in every harness format
    Given a valid one-skill one-agent one-capability harness contract
    And each harness declares the required capability in its own capability format
    When I inspect harness parity
    Then harness-parity validation succeeds

  Scenario: A divergent required-capability declaration fails
    Given a valid one-skill one-agent one-capability harness contract
    And the required-capability command for harness "gamma" diverges
    When I inspect harness parity
    Then the harness-parity violations include "divergent-capability"

  Scenario: A required-capability declaration with a divergent argument vector fails
    Given a valid one-skill one-agent one-capability harness contract
    And the required-capability arguments for harness "gamma" diverge
    When I inspect harness parity
    Then the only violation starts with "adapters/gamma/capabilities.json: divergent-capability:"
    And 2 capability declarations were reconciled

  Scenario: A harness that declares no capability file fails
    Given a valid one-skill one-agent one-capability harness contract
    And the capability declaration for harness "beta" is absent
    When I inspect harness parity
    Then the only violation starts with "adapters/beta/capabilities.json: divergent-capability:"

  Scenario: An unreadable capability declaration fails closed
    Given a valid one-skill one-agent one-capability harness contract
    And the capability declaration for harness "beta" is unreadable
    When I inspect harness parity
    Then the exit code is 2
    And stderr names the unreadable capability file

  Scenario: A roster narrowed by flag still reports every finding it inspects
    Given a valid one-skill one-agent one-capability harness contract
    And the agent adapter for "alpha" is missing
    When I inspect harness parity narrowed to "alpha"
    Then the harness-parity violations include "missing-agent-adapter"
    And 1 harnesses were inspected

  Scenario: Declared excluded directories and filesystem links are not inspected
    Given the repository declares excluded scan directories:
      * build-output
      * dependencies
    And a valid one-skill one-agent one-capability harness contract
    And excluded instruction sources and a linked skill exist
    When I inspect harness parity
    Then harness-parity validation succeeds with 3 harnesses, 1 skill, 1 agent, and 3 reconciled capability declarations

  Scenario: Findings are stable, sorted, and inspection is read-only
    Given a valid one-skill one-agent one-capability harness contract
    And an out-of-order pair of harness-parity violations exists
    When I inspect harness parity twice
    Then harness-parity outputs are identical
    And harness-parity violations are ordinally sorted
    And the repository is unchanged by the inspection

  Scenario: A narrowed inspection refuses a harness the repository does not declare
    Given a valid one-skill one-agent one-capability harness contract
    When I inspect harness parity narrowed to "delta"
    Then the exit code is 2
    And stderr contains "is not a harness this repository declares"

  Scenario: A file that vanishes between the walk and the read is not a finding
    Given a valid one-skill one-agent one-capability harness contract
    And file "root-instructions.md" vanishes between the walk and the read
    When I inspect harness parity
    Then the only violation starts with "root-instructions.md: missing-instruction:"

  Scenario: An unusable prohibited-source glob stops the instruction check
    Given a valid one-skill one-agent one-capability harness contract
    And the repository declares an unusable prohibited instruction source
    When I inspect harness parity
    Then there are no violations

  Scenario: A file directly under the canonical skills root is not a skill
    Given a valid one-skill one-agent one-capability harness contract
    And a loose file sits directly under the canonical skills root
    When I inspect harness parity
    Then harness-parity validation succeeds with 3 harnesses, 1 skill, 1 agent, and 3 reconciled capability declarations

  Scenario Outline: Only Markdown files directly under the agents root are agents
    Given a valid one-skill one-agent one-capability harness contract
    And the repository contains:
      | path       | content     |
      | <path>     | not an agent |
    When I inspect harness parity
    Then harness-parity validation succeeds with 3 harnesses, 1 skill, 1 agent, and 3 reconciled capability declarations

    Examples:
      | path                        |
      | canon/agents/nested/deep.md |
      | canon/agents/notes.txt      |

  Scenario: A canonical agent without a declaration is invalid
    Given a valid one-skill one-agent one-capability harness contract
    And the canonical agent carries no declaration
    When I inspect harness parity
    Then the harness-parity violations include "invalid-agent"

  Scenario: An agent adapter without a declaration is invalid
    Given a valid one-skill one-agent one-capability harness contract
    And the agent adapter for "alpha" carries no declaration
    When I inspect harness parity
    Then the only violation starts with "adapters/alpha/agents/reviewer.md: invalid-agent:"

  Scenario: A skill wrapper without a declaration diverges from the skill
    Given harness "alpha" declares a command directory
    And a valid one-skill one-agent one-capability harness contract
    And the skill wrapper for "alpha" carries no declaration
    When I inspect harness parity
    Then the only violation starts with "adapters/alpha/commands/tidy.md: skill-content-divergence:"

  Scenario Outline: Only an adapter-shaped file under a harness agent directory is an adapter
    Given a valid one-skill one-agent one-capability harness contract
    And the agent directory for "alpha" holds the extra file "<name>"
    When I inspect harness parity
    Then harness-parity validation succeeds with 3 harnesses, 1 skill, 1 agent, and 3 reconciled capability declarations

    Examples:
      | name           |
      | nested/note.md |
      | notes.txt      |

  Scenario: A canonical skill whose front matter is never closed is invalid
    Given a valid one-skill one-agent one-capability harness contract
    And the canonical skill front matter is never closed
    When I inspect harness parity
    Then the harness-parity violations include "invalid-skill"

  Scenario Outline: A capability declaration RHINO cannot read is a divergence
    Given a valid one-skill one-agent one-capability harness contract
    And the capability declaration for harness "alpha" is <shape>
    When I inspect harness parity
    Then the harness-parity violation for "adapters/alpha/capabilities.json" is "divergent-capability"

    Examples:
      | shape                              |
      | not valid in its declared format   |
      | missing the required capability    |

  Scenario: A capability declaration may nest the required server inside a list
    Given a valid one-skill one-agent one-capability harness contract
    And the capability declaration for harness "alpha" is a list of server groups
    When I inspect harness parity
    Then harness-parity validation succeeds with 3 harnesses, 1 skill, 1 agent, and 3 reconciled capability declarations

  Scenario: An empty harness roster reconciles a canon that is not there
    Given the repository declares a configuration with an empty harness roster and no canonical skill or agent root
    And a valid one-skill one-agent one-capability harness contract
    When I inspect harness parity
    Then the exit code is 0
    And 0 harnesses were inspected
