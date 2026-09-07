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
    Then the exit code is 1
    And the only violation starts with "root-instructions.md: missing-instruction:"

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

  Scenario: A README in the canonical agents root is an index, not an agent
    Given a valid one-skill one-agent one-capability harness contract
    And an index README sits in the canonical agents root
    When I inspect harness parity
    Then harness-parity validation succeeds with 3 harnesses, 1 skill, 1 agent, and 3 reconciled capability declarations

  Scenario: A README in a harness's agent directory is an index, not an adapter
    Given a valid one-skill one-agent one-capability harness contract
    And an index README sits in every harness agent directory
    When I inspect harness parity
    Then harness-parity validation succeeds with 3 harnesses, 1 skill, 1 agent, and 3 reconciled capability declarations

  Scenario: A source file containing the import is not an instruction source
    Given the repository declares no instruction adapter
    And a valid one-skill one-agent one-capability harness contract
    And a source file contains the canonical import
    When I inspect harness parity
    Then there are no violations

  Scenario: A prohibited name competes with the canon whatever the file kind
    Given the repository declares no instruction adapter
    And a valid one-skill one-agent one-capability harness contract
    And a file with a prohibited name is not Markdown
    When I inspect harness parity
    Then the harness-parity violations include "unexpected-instruction-source"

  Scenario: A documentation page showing the import in a fenced example is not a source
    Given the repository declares no instruction adapter
    And a valid one-skill one-agent one-capability harness contract
    And a documentation page shows the canonical import inside a fenced example
    When I inspect harness parity
    Then harness-parity validation succeeds

  Scenario: A documentation page showing the import in an inline code span is not a source
    Given the repository declares no instruction adapter
    And a valid one-skill one-agent one-capability harness contract
    And a documentation page shows the canonical import in an inline code span
    When I inspect harness parity
    Then harness-parity validation succeeds

  Scenario: A code span containing a backtick is read to its matching close
    Given the repository declares no instruction adapter
    And a valid one-skill one-agent one-capability harness contract
    And a documentation page shows the canonical import in a code span containing a backtick
    When I inspect harness parity
    Then harness-parity validation succeeds

  Scenario: A stray backtick does not hide a competing instruction source
    Given the repository declares no instruction adapter
    And a valid one-skill one-agent one-capability harness contract
    And a documentation page carries a stray backtick before the canonical import
    When I inspect harness parity
    Then the harness-parity violations include "unexpected-instruction-source"

  Scenario: An unclosed fence does not hide a competing instruction source
    Given the repository declares no instruction adapter
    And a valid one-skill one-agent one-capability harness contract
    And a documentation page hides the canonical import behind an unclosed fence
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
    Then the only violation starts with "adapters/beta/commands/tidy.md: skill-content-divergence: `description` is not the canonical description"

  Scenario: A skill wrapper may not grow a body of its own
    Given a valid one-skill one-agent one-capability harness contract
    And harness "beta" declares a command directory
    And the skill wrapper for "beta" has an extra body
    When I inspect harness parity
    Then the only violation starts with "adapters/beta/commands/tidy.md: skill-content-divergence: `body` is not the canonical route"

  Scenario: A skill wrapper may declare nothing beyond its route
    Given a valid one-skill one-agent one-capability harness contract
    And harness "beta" declares a command directory
    And the skill wrapper for "beta" declares more than its route
    When I inspect harness parity
    Then the only violation starts with "adapters/beta/commands/tidy.md: skill-content-divergence: `name` is beyond"

  Scenario: A wrapper no canonical skill asked for is reported
    Given a valid one-skill one-agent one-capability harness contract
    And harness "beta" declares a command directory
    And an unexpected skill wrapper exists for "beta"
    When I inspect harness parity
    Then the harness-parity violation for "adapters/beta/commands/ghost.md" is "unexpected-skill-adapter"

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
    And the agent adapter for "beta" weakens a denied capability
    When I inspect harness parity
    Then the harness-parity violation for "adapters/alpha/agents/reviewer.md" is "agent-prompt-divergence"
    And the harness-parity violation for "adapters/beta/agents/reviewer.md" is "agent-semantic-divergence"

  Scenario: An adapter must grant what the canon requires
    Given a valid one-skill one-agent one-capability harness contract
    And the agent adapter for "beta" withholds a required capability
    When I inspect harness parity
    Then the harness-parity violation for "adapters/beta/agents/reviewer.md" is "agent-semantic-divergence"

  Scenario: An adapter may grant more than the canon requires
    Given a valid one-skill one-agent one-capability harness contract
    And the canonical agent requires less than its adapters grant
    When I inspect harness parity
    Then harness-parity validation succeeds with 3 harnesses, 1 skill, 1 agent, and 3 reconciled capability declarations

  Scenario: A capability the canon never took on obliges an adapter nothing
    Given a valid one-skill one-agent one-capability harness contract
    And the canonical agent requires less and no adapter answers for it
    When I inspect harness parity
    Then harness-parity validation succeeds with 3 harnesses, 1 skill, 1 agent, and 3 reconciled capability declarations

  Scenario: A repository with harnesses and no canonical agents reconciles none
    Given a valid one-skill one-agent one-capability harness contract
    And no canonical agents are declared and no harness expresses them
    When I inspect harness parity
    Then harness-parity validation succeeds with 3 harnesses, 1 skill, 0 agent, and 3 reconciled capability declarations

  Scenario: A harness may write its grants as a list or as one line
    Given a valid one-skill one-agent one-capability harness contract
    And the agent adapter for "alpha" lists its grants
    When I inspect harness parity
    Then harness-parity validation succeeds with 3 harnesses, 1 skill, 1 agent, and 3 reconciled capability declarations

  Scenario: An adapter must name the agent it stands for
    Given a valid one-skill one-agent one-capability harness contract
    And the agent adapter for "gamma" names another agent
    When I inspect harness parity
    Then the only violation starts with "adapters/gamma/agents/reviewer.toml: agent-semantic-divergence: `name` is not the canonical name"

  Scenario Outline: An adapter must answer a capability in its own vocabulary
    Given a valid one-skill one-agent one-capability harness contract
    And the agent adapter for "<harness>" grants nothing answering a capability
    When I inspect harness parity
    Then the harness-parity violation for "<path>" is "agent-semantic-divergence"

    Examples:
      | harness | path                              |
      | alpha   | adapters/alpha/agents/reviewer.md |
      | beta    | adapters/beta/agents/reviewer.md  |

  Scenario: An adapter must grant every member a capability translates to
    Given a valid one-skill one-agent one-capability harness contract
    And the agent adapter for "alpha" withholds a required capability
    When I inspect harness parity
    Then the only violation starts with "adapters/alpha/agents/reviewer.md: agent-semantic-divergence: `tools` does not grant `Grep`"

  Scenario: An adapter may not declare a field its harness forbids
    Given a valid one-skill one-agent one-capability harness contract
    And the agent adapter for "gamma" declares a field its harness forbids
    When I inspect harness parity
    Then the harness-parity violation for "adapters/gamma/agents/reviewer.toml" is "agent-semantic-divergence"

  Scenario: An adapter may not change a field its harness fixes
    Given a valid one-skill one-agent one-capability harness contract
    And the agent adapter for "gamma" changes a field its harness fixes
    When I inspect harness parity
    Then the harness-parity violation for "adapters/gamma/agents/reviewer.toml" is "agent-semantic-divergence"

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

  Scenario: An adapter may not ignore a declared constraint
    Given a valid one-skill one-agent one-capability harness contract
    And the agent adapter for "alpha" ignores a declared constraint
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

  Scenario: A repository that requires no capability server reconciles none
    Given a valid one-skill one-agent one-capability harness contract
    And no capability server is required and no harness declares a capability file
    When I inspect harness parity
    Then harness-parity validation succeeds with 3 harnesses, 1 skill, 1 agent, and 0 reconciled capability declarations

  Scenario: A file that is not text is not an instruction source
    Given a valid one-skill one-agent one-capability harness contract
    And file "logo.png" holds bytes that are not text
    When I inspect harness parity
    Then there are no violations

  Scenario: A file that cannot be opened still stops the run
    Given a valid one-skill one-agent one-capability harness contract
    And file "notes.md" cannot be opened
    When I inspect harness parity
    Then the exit code is 2

  Scenario: An unusable prohibited-source glob stops the instruction check
    Given a valid one-skill one-agent one-capability harness contract
    And a file imports the canonical instruction body
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

  Scenario Outline: A capability may be written as one command vector
    Given a valid one-skill one-agent one-capability harness contract
    And harness "<harness>" writes its capability as one command vector
    When I inspect harness parity
    Then harness-parity validation succeeds

    Examples:
      | harness |
      | alpha   |
      | gamma   |

  Scenario: A closed adapter still declares the fields its translations name
    Given every harness closes its agent adapter declaration
    And a valid one-skill one-agent one-capability harness contract
    When I inspect harness parity
    Then harness-parity validation succeeds

  Scenario: A route template written with uneven spacing is the same route
    Given the repository writes its route template with uneven spacing
    And a valid one-skill one-agent one-capability harness contract
    When I inspect harness parity
    Then harness-parity validation succeeds

  Scenario Outline: A route wrapped across lines is the same route
    Given a valid one-skill one-agent one-capability harness contract
    And the agent adapter for "<harness>" wraps its route across lines
    When I inspect harness parity
    Then harness-parity validation succeeds

    Examples:
      | harness |
      | alpha   |
      | beta    |

  Scenario Outline: The canon declares its permissions under names the repository chose
    Given the canonical agents declare their permissions under "<spelling>" names
    And a valid one-skill one-agent one-capability harness contract
    When I inspect harness parity
    Then harness-parity validation succeeds

    Examples:
      | spelling  |
      | ordinary  |
      | alternate |

  Scenario Outline: Drift is still caught when the canon renames its permission fields
    Given the canonical agents declare their permissions under "alternate" names
    And a valid one-skill one-agent one-capability harness contract
    And the agent adapter for "<harness>" weakens a denied capability
    When I inspect harness parity
    Then the harness-parity violation for "<path>" is "agent-semantic-divergence"

    Examples:
      | harness | path                              |
      | alpha   | adapters/alpha/agents/reviewer.md |
      | beta    | adapters/beta/agents/reviewer.md  |

  Scenario: A constraint is still enforced when the canon renames its fields
    Given the canonical agents declare their permissions under "alternate" names
    And a valid one-skill one-agent one-capability harness contract
    And the agent adapter for "alpha" ignores a declared constraint
    When I inspect harness parity
    Then the harness-parity violations include "agent-semantic-divergence"

  Scenario: A canonical agent must carry the field its declaration fixes
    Given a valid one-skill one-agent one-capability harness contract
    And the canonical agent omits the field every declaration must carry
    When I inspect harness parity
    Then the harness-parity violations include "invalid-agent: `mode` must be declared as `subagent`"

  Scenario: A canonical agent without a declaration is invalid
    Given a valid one-skill one-agent one-capability harness contract
    And the canonical agent carries no declaration
    When I inspect harness parity
    Then the harness-parity violation for "canon/agents/reviewer.md" is "invalid-agent"

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
    Then the only violation starts with "canon/skills/tidy/SKILL.md: invalid-skill:"

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
    And harness-parity validation succeeds with 0 harnesses, 0 skill, 0 agent, and 0 reconciled capability declarations
