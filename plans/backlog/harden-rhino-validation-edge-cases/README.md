# Harden RHINO Validation Edge Cases

Status: Backlog

## Context

[Repo-grounded] Three current validator boundaries produce incorrect answers on valid repository input:

- plan acceptance identifiers are defined only when a line starts with `Scenario:`, so `Scenario Outline:` definitions
  are invisible to delivery-reference validation;
- harness tier validation returns before inspecting an adapter when the canonical agent declares no tier, so an adapter
  can silently choose a model or effort; and
- Mermaid colour and label readers inspect accessibility metadata as diagram content, treating parenthesized
  `accDescr` prose as a node and a numeric HTML entity as a colour.

The defects share RHINO's deterministic validation surface, but they do not depend on one another. The plan therefore
keeps one investigation and convergence record while delivering three independently releasable pull requests.

## Decision

[Judgment call] Use one formal plan with three delivery units. Separate plans would repeat the same validation,
specification, and release-readiness framing; one combined code change would make independent defects share a rollback.

Rejected alternatives:

- one pull request for all three defects — smaller ceremony, but one regression would block or roll back unrelated
  fixes;
- three formal plans — strongest isolation, but duplicates setup, convergence, and archival work without improving the
  implementation seams.

## Decision Gate Record

- Pre-write, 2026-09-16: the owner selected one RHINO plan with three independent delivery units rather than one
  combined pull request or three formal plans.
- Post-write, 2026-09-16: after the complete draft and cold-read repairs, the owner approved the plan as written and
  authorized its formal quality gate plus plan-only delivery.

## Scope

In scope:

- recognize acceptance identifiers on both `Scenario:` and `Scenario Outline:` declarations;
- reject model or effort projection when the canonical agent declares no tier;
- exclude `accTitle` and `accDescr` metadata from diagram-content inspection;
- distinguish numeric HTML entities from colour literals while preserving entity decoding for visible labels;
- update canonical Gherkin, bindings, the shared plan fixture corpus, and applicable governance contracts.

Out of scope:

- changing existing rule identifiers, exit classes, configuration keys, label limits, or colour policy;
- changing unsupported Mermaid syntaxes or adding a general Mermaid parser;
- publishing a release or changing a consumer repository.

## Approach Summary

1. Extend the plan-criterion grammar and freeze an accepted shared-corpus case.
2. Add a new harness diagnostic for projection without a canonical tier.
3. Classify accessibility metadata before colour and legibility readers inspect diagram source.
4. Deliver and prove each seam independently, then run repository-wide convergence.

## Dependencies

- [Repo-grounded] `specs/` remains the canonical behavior source.
- [Repo-grounded] The shared plan fixture corpus is byte-locked by `SHA256SUMS` and `CORPUS-DIGEST`.
- [Repo-grounded] `cargo xtask test-quick` and `cargo xtask self-validate` are the local completion gates.

## Directory Map

- [Business requirements](brd.md)
- [Product requirements and acceptance criteria](prd.md)
- [Technical design](tech-docs.md)
- [Delivery checklist](delivery.md)
- [Execution learnings](learnings.md)
