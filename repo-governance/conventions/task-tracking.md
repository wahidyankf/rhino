# Task Tracking

Represent repository work as a granular task list, and keep that list synchronized with the actual state of the work.

## Requirements

- Create or refresh the list before beginning execution.
- Split work into small, concrete items with one observable outcome each. Split again when an item combines distinct
  actions or outcomes.
- For new or changed behaviour and for bug fixes, represent each
  [red–green–refactor](../workflows/quality/red-green-refactor.md) increment as separate RED, GREEN, and REFACTOR items.
  Preserve the test path, the expected behavioural RED reason before implementation, and the final GREEN and REFACTOR
  results. A pure refactor follows the green-baseline and characterization rule in
  [test-driven development](../development/test-driven-development.md).
- Include discovery, implementation, validation, documentation, and delivery items when they are in scope. Never hide
  required work inside a broad item.
- Record each item as pending, in progress, completed, or blocked, and keep at most one in progress unless work
  genuinely proceeds in parallel.
- Update status promptly when work starts, completes, becomes blocked, or returns for revision. Add, split, merge, or
  reorder items when scope or understanding changes.
- Mark an item completed only after its stated outcome is achieved and its verification succeeded. A failing check means
  the item stays open.
- Before reporting completion, reconcile the whole list against the repository state. Finish what remains or say plainly
  what does not and why.
- Carry the list and its accurate status across context compaction or handoff, under
  [governance continuity](../principles/governance-continuity.md).

## Written Progress Record

A session can break off, and the live list goes with it, so the work also keeps a written record a new session resumes
from. Inside plan-mediated work that record is the plan's `delivery.md`, under its
[delivery contract](plans/004-delivery-contract.md#single-progress-surface).

Outside a plan, a progress file in `local-tmp/`, the [scratch directory](working-tree.md#ignored), is the written
record. Open it before the task's first action; record the goal, every active rule decision, and each item with its
status; and update it as items resolve, so a session that breaks off resumes from it. It stays until the whole task has
ended, delivery and cleanup in every repository included, and is then removed under
[dev artifact clean-up](../workflows/maintenance/dev-artifact-clean-up.md).

## New Direction Mid-Task

New, follow-on, or changed direction reaches the list before it reaches the work. Read it against every open item first:
some are now wrong, some are superseded, some are unaffected, and the new direction is usually more than one item.
Record that reconciliation, then continue.

Acting first and updating afterwards produces a list describing the task as it was requested rather than as it is being
performed, which is the state the list exists to prevent. The reconciliation is also where a contradiction between old
and new direction becomes visible; carrying both silently resolves it by accident.

## Concurrent Ownership

Another task may be creating, changing, or removing rules under `repo-governance/` at the same time. Refresh that state
before relying on or editing it. Treat unfamiliar concurrent changes as another task's work: preserve and reconcile
around them rather than reverting them or reporting them as an error.

A task list records intended work. It grants no authorization for a commit, a push, or a merge; those stay under
[commit authorization](commit-authorization.md) and the [merge preconditions](pull-request-merge.md).
