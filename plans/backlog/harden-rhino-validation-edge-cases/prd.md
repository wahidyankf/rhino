# Product Requirements: Harden RHINO Validation Edge Cases

## Product Overview

[Repo-grounded] RHINO already validates plan acceptance traceability, harness tier projection, and Mermaid accessibility
and legibility. This plan closes three edge cases without changing the commands, configuration schema, or exit classes.

## Personas

- A plan author using parameterized Gherkin acceptance criteria.
- A harness maintainer preserving canonical tier intent across adapters.
- A documentation author writing accessible Mermaid diagrams.
- A repository maintainer interpreting RHINO's findings as deterministic evidence.

## User Stories

- As a plan author, I want a `Scenario Outline:` acceptance identifier to be a valid definition so delivery references
  do not fail incorrectly.
- As a harness maintainer, I want a no-tier canonical agent to forbid adapter model or effort so adapters cannot invent
  policy.
- As a documentation author, I want accessibility prose and HTML entities parsed by their declared roles so valid
  diagrams do not produce false findings.

## Product Scope

In scope: the three behaviors below, their stable diagnostics, canonical specifications, bindings, fixtures, and
repository gates. Out of scope: new CLI or configuration surface, new Mermaid syntaxes, consumer rollout, and release
publication.

## Acceptance Criteria

```gherkin
Feature: Validator edge-case correctness

  Scenario: [AC-01] A scenario outline defines its acceptance identifier
    Given a conforming plan whose product requirements define an identifier on a Scenario Outline line
    When plan validation checks a delivery item citing that identifier
    Then acceptance traceability passes without an undefined-identifier finding

  Scenario: [AC-02] Scenario and outline definitions share duplicate detection
    Given a plan defines the same acceptance identifier once on a Scenario line and once on a Scenario Outline line
    When plan validation checks the plan
    Then it reports exactly one PLAN-CRITERION-001 finding for that identifier

  Scenario: [AC-03] An adapter cannot project a tier the canonical agent does not declare
    Given a canonical agent declares no tier and its harness adapter declares a model or effort
    When harness-parity validation checks the adapter
    Then it reports one tier-projection-without-canonical-tier finding

  Scenario: [AC-04] A no-tier agent with no projection remains valid
    Given a canonical agent declares no tier and its harness adapter declares neither model nor effort
    When harness-parity validation checks the adapter
    Then the adapter passes tier projection validation

  Scenario: [AC-05] Accessibility metadata is not diagram content
    Given a rendered Mermaid diagram has label-like prose in accTitle or single-line or braced accDescr metadata
    When Mermaid legibility validation checks the diagram
    Then the accessibility metadata produces no node-label or colour finding

  Scenario: [AC-06] Numeric HTML entities are not colour literals
    Given a Mermaid visible label contains a decimal or hexadecimal numeric HTML entity
    When Mermaid accessibility and legibility validation check the diagram
    Then the entity is decoded for label measurement without producing a colour finding

  Scenario: [AC-07] Real diagram content remains enforced
    Given a Mermaid diagram declares a real colour outside classDef and an overlong visible label
    When Mermaid accessibility and legibility validation check the diagram
    Then both existing findings remain observable

  Scenario: [AC-08] Every delivery unit is independently releasable
    Given one of the three validator corrections is the only unit added to current main
    When the focused corpus and repository completion gates run
    Then every applicable check exits successfully before that unit merges
```

## Product Risks

- `Scenario Outline:` recognition must not treat prose or examples as definitions.
- The new tier finding must not apply to a harness that declares no tier fields.
- Accessibility block exclusion must end at the correct brace and must not suppress later diagram statements.
- Entity-aware colour scanning must still reject actual three-, six-, or eight-digit colour literals.
