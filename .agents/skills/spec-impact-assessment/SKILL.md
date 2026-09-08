---
name: spec-impact-assessment
description: Assess specs/behaviours and specs/architecture.md for impact before changing anything, and record a verified no-op rather than churning an unaffected specification.
---

# Specification Impact Assessment

Required before **every** change to this repository, not only before a behaviour change. The rules are in [specification maintenance](../../../repo-governance/development/specification-maintenance.md); this file is how the assessment is performed and recorded.

## Assess

1. List the `.feature` files whose subject could plausibly be touched. Read them — the scenarios, not the filenames.
2. Read `specs/architecture.md` and identify whether any boundary, responsibility, or dependency direction moves.
3. Decide, per surface, one of: **changed**, or **verified no-op**.

A verified no-op names what was read and why the change does not reach it. "No specification changes needed" with nothing behind it is not an assessment.

## When a behaviour changes

1. Write or edit the Gherkin **first**.
2. Bind it at the unit adapter.
3. Run it and confirm it fails **because the behaviour is absent** — not because the binding is missing, the fixture is wrong, or the crate does not compile. A red for the wrong reason proves nothing.
4. Write production code until it passes.
5. Bind the remaining applicable adapters, or record an exemption.

## Exemptions

Every scenario binds at the unit adapter; there is no unit exemption. An integration or end-to-end exemption names the **concrete boundary** that layer cannot reach and the alternative proof covering it. Difficulty, runtime, flakiness, and cost are never boundaries.

## Refusals

- Never edit a scenario because a change happened nearby.
- Never write a placeholder, no-op, or outcome-table binding.
- Never widen a coverage exclusion to accommodate a new module.

Changed Gherkin or changed bindings get [the review](../../../repo-governance/workflows/gherkin-implementation-review.md) before completion.
