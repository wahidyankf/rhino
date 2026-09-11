# Plan Lifecycle — Local Rules

The plan contract itself is the [plans convention](plans.md) and its [modules](plans/README.md): the lifecycle folders, the six documents, the technical shape, the delivery grammar, structural validation, evidence, and archival. That document is portable on purpose and names no repository.

This file holds the part that is RHINO's, and would not be true of another repository. Where the two touch the same subject, the convention is the rule and this file adds to it; it never contradicts it.

## A Plan Proposes; `specs/` Records

[`specs/`](../../specs/README.md) is the as-built description of the binary this repository builds. Where a plan and a specification disagree the specification is right, and execution updates every affected specification alongside the implementation under [specification maintenance](../development/specification-maintenance.md).

## Authorization

Writing a plan into this repository requires an explicit user request. Designing an approach in conversation, or in a harness planning mode, authorizes neither the folder nor the commit that would carry it. See [commit authorization](commit-authorization.md).

## Scope

This repository plans only work it can deliver alone. Work that spans repositories is planned where it is coordinated and arrives here as its own change with its own evidence — [rules propagation](../workflows/rules-propagation.md) gives the reason: something that crosses a boundary is decided again on the other side, or it arrives without the argument that justified it.

## Ideas Are Filed by Quadrant

A rough two-pager lives at `plans/ideas/<quadrant>/<slug>.md`, with q1 to q4 chosen from dated evidence of urgency and importance rather than from impression. Search the quadrants first and consolidate overlap.

The convention leaves the internal shape of the idea folder to the repository. This is the shape RHINO chose, and it is the one place the two differ.

## Delivery Items That Ship Code

A checkbox that ships code states its [red-green-refactor](../workflows/red-green-refactor.md) cycle as three checkboxes — RED, GREEN, REFACTOR — each naming the test path, the command, and the failure or pass expected. Never one checkbox, and never prose.

That is stricter than the convention requires, and it is stricter because this repository is the one that implements the validators the convention is checked with. A cycle recorded as a single item cannot show that the test failed first, which is the only part of it worth recording.

## Rule Changes Found During Execution

Where execution may change a repository rule, `delivery.md` carries an `[AI]` task applying [rules propagation](../workflows/rules-propagation.md) and recording its terminal result, which may be `PASS_NO_CHANGE`.

## Which Gherkin Becomes Durable

The Gherkin in `prd.md` accepts the plan, not the corpus. Which of it becomes a scenario under `specs/behaviours/` is decided under [plan specification changes](plan-specification-changes.md), and a plan that adds none says so rather than leaving the question open.
