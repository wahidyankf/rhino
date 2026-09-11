# Plan Execution

Use this only after an explicit direction to execute one formal plan. Its job is to keep the plan's records true while the work happens.

## Start

1. Select one plan in `backlog/` or `in-progress/`. Require a current `PASS` from a [plan quality gate](plan-quality-gate.md) run the user explicitly directed. Authority to execute is not authority to run that gate: with no current `PASS`, stop and say so.
2. Enter the plan's worktree before any file or Git mutation, initializing it if new, under [the integration path](../conventions/integration-path.md). Executing from the primary checkout is forbidden. Pass its sync gate there, and read the whole incoming diff against the plan when the sync adds commits.
3. If the plan is queued, move it to `plans/in-progress/<slug>/` with its status and both stage maps, in one change. Never copy it.
4. Read `learnings.md` before touching anything. Mirror every unchecked executable delivery item into the [task list](../conventions/task-tracking.md), preserving wording, owner label, order, and references. Leave dormant recovery items dormant.

## Execute

1. Work in order, one active item at a time unless two are genuinely independent. Stop at any pending `[HUMAN]` input. Pass every phase checkpoint before the next phase starts.
2. Resolve each item atomically, once its stated outcome and its stated proof both hold: tick the box, then record a dated result saying what proved it, what changed, and what was surprising. One step, not a batch of ticks at the end of a session — a tick says an action happened and does not say what it found. Every behaviour-changing item names the layer that establishes it under [verification routing](../development/planning-capabilities/006-verification-routing.md).
3. Keep the plan and the task list synchronized, activating a conditional item when its trigger fires. Add a discovered task to both lists only when it serves an outcome the plan already has; label it and explain it.
4. Capture learnings as they happen rather than reconstructing them at the end. Search [`plans/ideas/`](../../plans/ideas/README.md) for overlap and either merge into an existing brief or write one distinct brief and link it.
5. Land work in delivery units through [worktree to pull request](worktree-to-pull-request.md), reusing the plan's one worktree for every unit. A unit is complete when its pull request has merged, not when its code is written.
6. Run the required automation for each unit and record evidence [data safety](../conventions/public-repository-data-safety.md) permits.
7. Apply every applicable repository rule; a plan expands no authority, and neither does a task list. A failing gate stops the line: repair the cause, update `delivery.md` and `learnings.md`, then resume.
8. Fix what fails, including what was already failing. A check that was red before this plan started is red because of this plan's work by the time it ships. Pre-existing is an explanation, not an exemption.

## Complete and Archive

1. Run [plan execution check](plan-execution-check.md) once every substantive item is terminal, and record its verdict. Archival is blocked, not warned, by an unresolved criterion, an unproven cleanup, or an unrouted learning.
2. Require an explicit direction for a fresh completion run of the quality gate, and continue only on `PASS`. Do not start it from here.
3. Tear down what the plan created — its worktree, its local branch, and that branch on `origin` — once every delivery unit that used the worktree has landed. A failed run keeps its worktree and records why. Record proof of each removal.
4. Give every dormant conditional a dated, evidenced `Not triggered` disposition; never claim execution of something that never ran. The plan stays in progress while any required outcome, activated conditional, gate, or human action remains.
5. Archive through the sequence [knowledge capture and archival](../conventions/plans/008-knowledge-capture-and-archival.md) fixes, using the final checkpoint's local date for both the README `Completed` field and the destination folder. Refuse an existing destination: never merge, overwrite, or add a suffix. Committing and pushing need their own [authorization](../conventions/commit-authorization.md).

## Recovery

At any pause the plan itself carries enough state to resume: the current checkout, the last terminal gate, the next unresolved item, and any bounded budget already partly spent. A resumed session continues a budget; it never resets one.

Interrupted work stays accurately in progress and resumes only after a directed fresh quality-gate `PASS`. If archival verification fails, restore the folder, its status, and both maps. Never leave a plan split across two stages, and never archive incomplete work.
