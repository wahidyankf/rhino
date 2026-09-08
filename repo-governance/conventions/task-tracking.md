# Task Tracking

Represent repository work as a granular task list, and keep that list synchronized with the actual state of the work.

## Requirements

- Create or refresh the list before beginning execution.
- Split work into small, concrete items with one observable outcome each. Split again when an item combines distinct actions or outcomes.
- For new or changed behaviour and for bug fixes, represent each [red–green–refactor](../workflows/red-green-refactor.md) increment as separate RED, GREEN, and REFACTOR items. Preserve the test path, the expected behavioural RED reason before implementation, and the final GREEN and REFACTOR results. A pure refactor follows the green-baseline and characterization rule in [test-driven development](../development/test-driven-development.md).
- Include discovery, implementation, validation, documentation, and delivery items when they are in scope. Never hide required work inside a broad item.
- Record each item as pending, in progress, completed, or blocked, and keep at most one in progress unless work genuinely proceeds in parallel.
- Update status promptly when work starts, completes, becomes blocked, or returns for revision. Add, split, merge, or reorder items when scope or understanding changes.
- Mark an item completed only after its stated outcome is achieved and its verification succeeded. A failing check means the item stays open.
- Before reporting completion, reconcile the whole list against the repository state. Finish what remains or say plainly what does not and why.
- Carry the list and its accurate status across context compaction or handoff, under [governance continuity](../principles/governance-continuity.md).

## Concurrent Ownership

Another task may be creating, changing, or removing rules under `repo-governance/` at the same time. Refresh that state before relying on or editing it. Treat unfamiliar concurrent changes as another task's work: preserve and reconcile around them rather than reverting them or reporting them as an error.

A task list records intended work. It grants no authorization for a commit, a push, or a merge; those stay under [commit authorization](commit-authorization.md) and the [merge preconditions](pull-request-merge.md).
