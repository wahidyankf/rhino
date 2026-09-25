# Harden RHINO Validation Edge Cases

Status: Backlog

## Context

[Repo-grounded] Two current validator boundaries produce incorrect answers on valid repository input:

- a harness profile's `fixed` adapter field may name the same native field as its `tier-fields`, so every canonical
  agent that declares no tier silently receives a model or effort, and harness adapter validation reports clean; and
- Mermaid colour and label readers inspect accessibility metadata as diagram content, treating parenthesized
  `accTitle` or `accDescr` prose as a node label and a decimal numeric HTML entity as a colour.

The defects share RHINO's deterministic validation surface, but they do not depend on one another. The plan therefore
keeps one investigation and convergence record while delivering two independently releasable pull requests.

## Decision

[Judgment call] Use one formal plan with two delivery units. Separate plans would repeat the same validation,
specification, and release-readiness framing; one combined code change would make independent defects share a rollback.

Rejected alternatives:

- one pull request for both defects — smaller ceremony, but one regression would block or roll back an unrelated fix;
- two formal plans — strongest isolation, but duplicates setup, convergence, and archival work without improving the
  implementation seams.

## Decision Gate Record

- Pre-write, 2026-09-16: the owner selected one RHINO plan with independent delivery units rather than one combined pull
  request or separate formal plans.
- Post-write, 2026-09-16: after the complete draft and cold-read repairs, the owner approved the plan as written and
  authorized its formal quality gate plus plan-only delivery.
- Re-grounding, 2026-09-25: a documentation audit found the plan grounded on paths that no longer exist and its first
  unit targeting the retired `plan validate` command. The owner chose the minimal resolution: remove that unit,
  re-ground the harness and Mermaid units on `src/v0_4/harnesses.rs` and `src/markdown/mermaid.rs`, and keep the plan in
  backlog.

## Scope

In scope:

- refuse a harness adapter configuration whose direct fields name a declared tier field;
- exclude `accTitle` and `accDescr` metadata from diagram-content inspection;
- distinguish decimal numeric HTML entities from colour literals while preserving entity decoding for visible labels;
- update canonical Gherkin, bindings, and the documentation that describes the changed behaviour.

Out of scope:

- changing existing finding kinds, exit classes, configuration keys, label limits, or colour policy;
- changing unsupported Mermaid syntaxes or adding a general Mermaid parser;
- publishing a release or changing a consumer repository.

## Approach Summary

1. Refuse a direct adapter field that collides with a declared tier field.
2. Classify accessibility metadata and numeric entities before colour and legibility readers inspect diagram source.
3. Deliver and prove each seam independently, then run repository-wide convergence.

## Dependencies

- [Repo-grounded] `specs/` remains the canonical behavior source; its whole corpus is
  `specs/behaviours/v0-4-contract.feature`.
- [Repo-grounded] `cargo xtask test-quick` and `cargo xtask self-validate` are the local completion gates.

## Directory Map

- [Business requirements](brd.md)
- [Product requirements and acceptance criteria](prd.md)
- [Technical design](tech-docs.md)
- [Delivery checklist](delivery.md)
- [Execution learnings](learnings.md)
