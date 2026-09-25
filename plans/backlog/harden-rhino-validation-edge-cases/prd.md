# Product Requirements: Harden RHINO Validation Edge Cases

## Product Overview

[Repo-grounded] RHINO already generates and validates harness adapters and validates Mermaid accessibility and
legibility. This plan closes two edge cases without changing the commands, configuration schema, finding kinds, or exit
classes.

## Personas

- A harness maintainer preserving canonical tier intent across adapters.
- A documentation author writing accessible Mermaid diagrams.
- A repository maintainer interpreting RHINO's findings as deterministic evidence.

## User Stories

- As a harness maintainer, I want a profile that fixes a tier field to be refused so adapters cannot invent a model or
  effort for an agent that declares no tier.
- As a documentation author, I want accessibility prose and HTML entities parsed by their declared roles so valid
  diagrams do not produce false findings.

## Product Scope

In scope: the behaviors below, their refusals and findings, canonical specifications, bindings, and repository gates.
Out of scope: new CLI or configuration surface, new Mermaid syntaxes, consumer rollout, and release publication.

## Acceptance Criteria

```gherkin
Feature: Validator edge-case correctness

  Scenario: [AC-01] An adapter cannot fix a declared tier field
    Given a harness profile's agent adapter declares tier fields and a direct field with the same native name
    When harness adapter validation or generation reads the configuration
    Then it exits 2 naming the profile before any adapter is written

  Scenario: [AC-02] A no-tier agent with no tier projection remains valid
    Given a canonical agent declares no tier and its profile fixes no tier field
    When harness adapters are generated and then validated
    Then the agent's adapter carries neither tier field and validation is clean

  Scenario: [AC-03] Accessibility metadata is not diagram content
    Given a rendered Mermaid diagram has label-like prose in accTitle or single-line or braced accDescr metadata
    When Mermaid validation checks the diagram
    Then the accessibility metadata produces no node-label or colour finding

  Scenario: [AC-04] Numeric HTML entities are not colour literals
    Given a Mermaid visible label contains a decimal or hexadecimal numeric HTML entity
    When Mermaid validation checks the diagram
    Then the entity is decoded for label measurement without producing a colour finding

  Scenario: [AC-05] Real diagram content remains enforced
    Given a Mermaid diagram declares a real colour outside classDef and an overlong visible label
    When Mermaid validation checks the diagram
    Then both existing findings remain observable

  Scenario: [AC-06] Every delivery unit is independently releasable
    Given one of the two validator corrections is the only unit added to current main
    When the changed scenarios and repository completion gates run
    Then every applicable check exits successfully before that unit merges
```

## Product Risks

- The tier-field refusal must not apply to a profile that declares no tier fields.
- Accessibility block exclusion must end at the correct brace and must not suppress later diagram statements.
- Entity-aware colour scanning must still reject actual three-, six-, or eight-digit colour literals.
