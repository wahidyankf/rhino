# Plan Execution

Use this only after an explicit direction to execute one formal plan.

## Start

1. Select one plan in `backlog/` or `in-progress/`. A quality-gate verdict is optional manual history: execution starts
   with or without one and never runs the gate itself. A missing verdict is recorded as
   `plan-quality-gate: not run (manual-only)`. For a bug-fix plan, an adopted Upstream Tool Defects standard remains
   available direction when explicitly invoked.
2. Enter the plan's worktree before any file or Git mutation, initializing if new, under
   [the integration path](../../conventions/integration-path.md). Executing from the primary checkout is forbidden. Pass
   its sync gate there, and read the whole incoming diff against the plan when the sync adds commits. Before starting
   plan execution or creating a new worktree, sync the execution checkout from the latest origin/main.
3. If the plan is queued, move it to `plans/in-progress/<slug>/` with its status and both stage maps, in one change.
   Never copy it.
4. Read `learnings.md` before touching anything. Mirror every unchecked executable delivery item into the
   [task list](../../conventions/task-tracking.md), preserving wording, owner label, and references, ordered by the
   dependency graph rather than list position. Leave dormant recovery items dormant. See
   [Dependency Graph and Recovery](../../conventions/plans/011-dependency-graph-and-recovery.md).

## Execute

1. Work ready items concurrently. Dispatch every item whose dependencies are terminal, up to the concurrency cap,
   reusing the plan's one worktree per repository; a failure blocks only its downstream. Stop at any pending `[HUMAN]`
   input. Pass every phase checkpoint before the next phase starts. A more optimal order found mid-run revises
   `delivery.md` first, keeping acceptance criteria and the end result; a material change re-passes the quality gate.
2. Resolve each item atomically once its stated outcome and stated proof both hold: tick the box, then record a dated
   result saying what proved it, what changed, and what surprised — one step, never a batch of ticks, since a tick alone
   never says what it found. Every behaviour-changing item names its establishing layer under
   [verification routing](../../development/planning-capabilities/006-verification-routing.md).
3. Keep the plan and task list synchronized, activating a conditional item on its trigger. Add a discovered task to both
   lists only when it serves an outcome the plan already has, labelled and explained.
4. Capture learnings as they happen, never reconstructing them at the end. Search
   [`plans/ideas/`](../../../plans/ideas/README.md) for overlap: merge into an existing brief, or write one distinct
   brief and link it.
5. Land work in delivery units through [worktree to pull request](../maintenance/worktree-to-pull-request.md), reusing
   the plan's one worktree throughout. Independent units land in any order, each refreshed with its gates re-run, per
   [Delivery Seams and Ownership](../../development/planning-capabilities/005-delivery-seams-and-ownership.md). A unit
   is complete at pull-request merge, not when its code is written.
6. Run each unit's required automation and record evidence
   [data safety](../../conventions/public-repository-data-safety.md) permits.
7. Apply every applicable repository rule; neither a plan nor a task list expands authority. A failing gate stops the
   line: repair the cause, update `delivery.md` and `learnings.md`, then resume.
8. Fix what fails, including pre-existing failures. A check red before this plan started is red because of this plan's
   work by the time it ships. Pre-existing is an explanation, not an exemption.

## Complete and Archive

1. Run [plan execution check](plan-execution-check.md) once every substantive item is terminal, and record its verdict.
   It alone closes the plan; the quality gate judges plans, not delivered work. Never require a full E2E run as a
   completion condition for plan execution or a plan phase; full E2E is a separately batched set. Archival is blocked,
   not warned, by an unresolved criterion, an unproven cleanup, or an unrouted learning.
2. Tear down what the plan created — its worktree, local branch, and that branch on `origin` — once every delivery unit
   has landed. A failed run keeps its worktree and records why. Record proof of each removal.
3. Give every dormant conditional a dated, evidenced `Not triggered` disposition; never claim unexecuted work as
   executed. The plan stays in progress while any required outcome, activated conditional, gate, or human action
   remains.
4. Archive through the sequence
   [knowledge capture and archival](../../conventions/plans/008-knowledge-capture-and-archival.md) fixes, using the
   final checkpoint's local date for the README `Completed` field and the destination folder alike. Refuse an existing
   destination: never merge, overwrite, or add a suffix. Committing and pushing need separate
   [authorization](../../conventions/commit-authorization.md).

## Recovery

Pause, resume, and budget rules live in [Execution Recovery](plan-execution-recovery.md).
