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
    Then harness-parity validation succeeds with 3 harnesses, 1 skill, 1 agent, and 1 capability

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
    And harness "alpha" declares no command directory
    When I inspect harness parity
    Then harness-parity validation succeeds

  Scenario: Skill descriptions and routes cannot drift
    Given a valid one-skill one-agent one-capability harness contract
    And harness "beta" declares a command directory
    And the skill wrapper for "beta" has a stale description and extra body
    When I inspect harness parity
    Then the harness-parity violations include "skill-content-divergence"

  Scenario: Skill bodies and supporting resources affect the contract digest
    Given a valid one-skill one-agent one-capability harness contract
    When I inspect and remember the harness-parity digest
    And I add a canonical skill supporting resource
    And I inspect harness parity again
    Then harness-parity validation succeeds
    And the harness-parity digest changed

  Scenario: Malformed or duplicated canonical skills fail
    Given a valid one-skill one-agent one-capability harness contract
    And a duplicate canonical skill name exists
    When I inspect harness parity
    Then the harness-parity violations include "invalid-skill"

  Scenario: Every canonical agent has one adapter per declared harness
    Given the repository declares a harness roster of "alpha", "beta", and "delta"
    And a valid one-skill one-agent one-capability harness contract
    And the agent adapter for "delta" is missing
    When I inspect harness parity
    Then the harness-parity violations include "missing-agent-adapter"
    And 3 harnesses were inspected

  Scenario: Missing, stale, or extra agent adapters fail
    Given a valid one-skill one-agent one-capability harness contract
    And the agent adapter for "alpha" is missing
    And an unexpected agent adapter exists
    When I inspect harness parity
    Then the harness-parity violations include "missing-agent-adapter"
    And the harness-parity violations include "unexpected-agent-adapter"

  Scenario: Extra agent prompt content or semantic drift fails
    Given a valid one-skill one-agent one-capability harness contract
    And the agent adapter for "alpha" contains extra prompt instructions
    And the agent adapter for "gamma" weakens a denied capability
    When I inspect harness parity
    Then the harness-parity violations include "agent-prompt-divergence"
    And the harness-parity violations include "agent-semantic-divergence"

  Scenario: An agent may declare only vocabulary the repository declared
    Given a valid one-skill one-agent one-capability harness contract
    And the canonical agent requires a capability outside the declared vocabulary
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

  Scenario: An unreadable capability declaration fails closed
    Given a valid one-skill one-agent one-capability harness contract
    And the capability declaration for harness "beta" is unreadable
    When I inspect harness parity
    Then the exit code is 2

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
    Then harness-parity validation succeeds with 3 harnesses, 1 skill, 1 agent, and 1 capability

  Scenario: Findings are stable, sorted, and inspection is read-only
    Given a valid one-skill one-agent one-capability harness contract
    And two sorted harness-parity violations exist
    And I remember the repository snapshot
    When I inspect harness parity twice
    Then harness-parity outputs are identical
    And harness-parity violations are ordinally sorted
    And the repository snapshot is unchanged
