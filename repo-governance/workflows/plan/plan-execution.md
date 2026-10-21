# Plan Execution

Use this only after an explicit direction to execute one formal plan.

## Start

1. Select one plan in `backlog/` or `in-progress/`. A quality-gate verdict is optional history, never run here; missing
   records as `plan-quality-gate: not run (manual-only)`. Upstream Tool Defects stays available on explicit invocation.
   Resolve declared dependencies per [Plan-Order Graph](../../conventions/plans/012-plan-order-graph.md): each
   satisfied, or stop at the unmet one; a mid-run graph change in a reviewed merged PR is authoritative — adapt.
2. Enter the plan's worktree before any file or Git mutation, initializing if new, under
   [the integration path](../../conventions/integration-path.md) — never the primary checkout. Pass its sync gate,
   reading incoming diff. Sync from latest `origin/main` first. Then prove access from the plan's scope — least
   privilege, infra secrets only for infra scope, portable versus machine-bound per need; missing access stops the run.
3. Promote a queued plan by its own pull request moving only it to `plans/in-progress/<slug>/` with status, stage maps,
   graph node, before Phase 0. Worktree from that fresh trunk; the promotion branch dies at merge.
4. Read `learnings.md` first. Mirror every unchecked executable item into the
   [task list](../../conventions/task-tracking.md) — wording, owner, references kept — in dependency order; dormant
   recoveries stay dormant, per
   [Dependency Graph and Recovery](../../conventions/plans/011-dependency-graph-and-recovery.md). Mirror the ledger from
   ignored `local-tmp/` into one per-run Issue as the claim; prove sole ownership from lease, branches, open PRs; a
   stale lease yields only to an explicit caller override. Inputs: `plan`; caps `max-worktrees-per-plan-repo`,
   `max-active-worktrees` (defaults `1`, caller-raised only).

## Execute

1. Work ready items under the worktree lock: dependency-terminal items, one `in_progress` per worktree, every compute
   node through one guard per [Resource-Aware Development](../../development/resource-aware-development.md) — a deferred
   guard queues: retry, never bypass or abandon. Failures block only downstream. Stop at pending `[HUMAN]` input; pass
   every phase checkpoint first. Reorder only by revising `delivery.md`; material changes re-pass the quality gate.
2. Resolve each item atomically on outcome plus proof: tick, then a dated result (proof, change, surprise) — never
   batched ticks. Behaviour changes name their layer per
   [verification routing](../../development/planning-capabilities/006-verification-routing.md). Ship code in batched
   cycles — one RED plus one GREEN per batch, selection frozen — per
   [Batched Cycles](../../development/test-driven-development/001-batched-cycles-in-planned-delivery.md).
3. Keep plan and task list synchronized; discovered tasks join both only for a planned outcome. Capture learnings as
   they happen; search [`plans/ideas/`](../../../plans/ideas/README.md) for overlap to merge or link.
4. Land through [worktree to pull request](../maintenance/worktree-to-pull-request.md) on the plan's one worktree;
   independent units land in any order with gates re-run on refreshed state, per
   [Delivery Seams and Ownership](../../development/planning-capabilities/005-delivery-seams-and-ownership.md). A unit
   completes at merge, not code completion.
5. Run each unit's automation, recording only what [data safety](../../conventions/public-repository-data-safety.md)
   permits; no plan or list expands authority. A failing gate stops the line: repair, update `delivery.md` and
   `learnings.md`, resume. Fix every failure, pre-existing included.

## Status Cadence

Send a five-line heartbeat every **20 minutes** — progress, steps left, blockers, decisions, next `[HUMAN]` timing
(`delivery.md` weights, else `1` each) — plus phase boundaries and terminal states. A queued node is reported, never a
stop. Each heartbeat clears owned subagents and processes of stuck work — `in_progress` with no progress since the last
heartbeat and no guard or error reason: nudge once, escalate if still stuck next.

## Goal and Termination

`pass` is the only success: items terminal, check permits archival, delivery landed, plan in `plans/done/`, worktree and
branches gone, learnings routed. `handed-off`: `[AI]` ready, only a `[HUMAN]` step remains; resume to `pass`. `partial`:
findings remain or work unexecuted; plan stays with blockers and unblockers. `fail`: technical error blocks; both stay.

## Complete and Archive

1. Run [plan execution check](plan-execution-check.md) when substantive items are terminal; its verdict alone permits
   archival. The quality gate judges plans, not work. Full E2E is separately batched.
2. Keep worktree and task branches until every using unit lands; then clean up, recording proof outside `plans/done/` in
   the archival PR body. Completion pends on proof. A failed run keeps its worktree.
3. Give dormant conditionals a dated, evidenced `Not triggered` disposition; never claim unexecuted work. The plan stays
   in progress while any outcome, conditional, gate, or human action remains.
4. Archive through [knowledge capture and archival](../../conventions/plans/008-knowledge-capture-and-archival.md),
   dating README `Completed` and destination alike; refuse an existing destination. Commits and pushes need separate
   [authorization](../../conventions/commit-authorization.md).

## Recovery

Pause, resume, and budget rules live in [Execution Recovery](plan-execution-recovery.md).
