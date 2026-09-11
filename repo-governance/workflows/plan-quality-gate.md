# Plan Quality Gate

Entry is a complete draft whose two [decision gates](../development/planning-capabilities/003-decision-gates.md) have both finished. Run this only when the user names this gate or unambiguously directs its semantic audit. Do not infer authorization from creating, editing, reviewing, or executing a plan, from a harness planning mode, or from another workflow. One instruction may authorize several named checkpoints; otherwise it authorizes one run.

Produce exactly one terminal result — `PASS` or one `BLOCKED_*` variant — for one plan's semantic readiness, at the directed pre-execution, post-material-change, or completion checkpoint. Never recurse or start another run.

## What It Judges

Meaning, consistency, safety, executability, and proof. `PASS` means good enough for the authorized scope, its known risks, and applicable rules — not perfect or future-proof. Do not block on style, speculative hardening, or an improvement that can wait without making execution unsafe or ambiguous. Apply [minimal sufficiency](../principles/minimal-sufficiency.md).

Deterministic tooling owns every machine-decidable check: the plan's own shape under [structural validation](../conventions/plans/006-structural-validation.md), plus links, directory maps, word budgets, Mermaid, and harness parity. Do not reproduce or second-guess them by reading; run them only in verification. Where the plan _delivers_ a check, confirm `delivery.md` has an implementation and a proof task rather than simulating the future tool, which must exist and pass at completion.

## Snapshot and Ledger

Freeze the plan path and stage, Git revision and dirty paths, scope, relevant specification and governance paths, unresolved decisions, and cycle `1`. Preserve them through compaction or handoff under [governance continuity](../principles/governance-continuity.md). A material external input change ends the run as `BLOCKED_INPUT_CHANGED` rather than restarting it.

Audit before editing. Build one finite ledger whose rows carry an ID, canonical rule, location, material gap, required repair, proof, and a status of `OPEN`, `FIXED`, `NOT_APPLICABLE`, or `BLOCKED`. Only a gap that violates a rule, or makes scoped execution unsafe, ambiguous, or unprovable, is a row. A mandatory finding cannot be waived, and `NOT_APPLICABLE` needs evidence.

## Procedure

1. Recursively inventory and read the plan, its assets, relevant implementation and specifications, and the governance it depends on. Do not validate a machine-owned concern.
2. Complete one semantic audit without editing anything. Check the [plans convention](../conventions/plans.md) and its [local additions](../conventions/plan-lifecycle.md) — one stage, required documents, one technical shape, truthful status; a route from BRD and PRD through the technical set to delivery that a junior could follow; necessary, non-placeholder artifacts; architecture, Gherkin, file impact, and dependencies synchronized against [software quality enforcement](../development/software-quality-enforcement.md); ownership, acceptance traceability, RED, GREEN, and REFACTOR tasks, checkpoints, evidence, and recovery; the [specification-change](../conventions/plan-specification-changes.md) contract; and conflicts with current specifications, governance, implementation, or another active plan.
3. Freeze the ledger. Repair only its rows, in dependency and safety order, each closing one `OPEN` row without expanding product scope. A missing decision, missing authority, or irreconcilable rule becomes `BLOCKED`; never invent the answer.
4. Verify semantically in read-only mode, reviewing only repaired meaning and its cross-document effects. Then run the gate:

   ```sh
   ./hippo run --class ephemeral --disk-path . -- cargo xtask test-quick
   ```

5. Return `PASS` when no row is `OPEN` or `BLOCKED`, the gate passes, no new semantic gap appeared, and the snapshot changed only through recorded repairs.
6. Otherwise allow exactly one stabilization cycle: add only repair-caused semantic gaps and deterministic findings, set cycle `2`, repair them once, and repeat step 4. A fixed finding cannot reopen without changed input, which yields `BLOCKED_INPUT_CHANGED`.
7. After cycle `2`, return `PASS` if step 5 now holds. Otherwise return `BLOCKED_NON_CONVERGENT` with the remaining rows and evidence. Do not repair again, restart, or invoke this workflow automatically.

HIPPO recovery required by [resource-aware development](../development/resource-aware-development.md) is infrastructure handling, not another cycle. If the gate cannot reach a deterministic verdict, return `BLOCKED_TOOLING` with its failure evidence; never simulate the check or retry it unbounded.

## Terminal Contract

`PASS` authorizes neither execution nor commit. Every `BLOCKED_*` result names its reason, remaining rows, and the external change required. Resume only when new input and an explicit direction authorize a fresh run. [Plan execution](plan-execution.md) consumes this result and never starts it.
