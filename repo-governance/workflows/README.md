# Workflows

Repeatable procedures, within every higher level. A workflow may compose other workflows. Where a workflow and a higher level disagree, the higher level wins and the workflow changes.

Several here carry a terminal contract — a fixed set of results, one of which every run must return. A run that ends without one of them has not finished.

## Directory Map

- [Coding-harness contract change](coding-harness-contract-change.md) — one canonical edit, every adapter reconciled with it.
- [Coding-harness parity verification](coding-harness-parity-verification.md) — the read-only audit of the same contract.
- [Gherkin implementation review](gherkin-implementation-review.md) — semantic review of what a binding actually asserts.
- [Dev artifact clean-up](dev-artifact-clean-up.md) — removing the artifacts one piece of work created, and nothing else.
- [PR leak review](pr-leak-review.md) — the posted, current-head review a merge requires.
- [Plan backlog grooming](plan-backlog-grooming.md) — whether each queued plan is still true of this repository.
- [Plan execution](plan-execution.md) — executing one formal plan while keeping its records true.
- [Plan execution check](plan-execution-check.md) — the fixed-order review that stands between finished work and archival.
- [Plan ideas grooming](plan-ideas-grooming.md) — one disposition, with a reason, for every brief.
- [Plan planning](plan-planning.md) — authoring the six documents between two decision gates.
- [Plan quality gate](plan-quality-gate.md) — a semantic verdict on one plan's readiness. Explicit request only.
- [Red, green, refactor](red-green-refactor.md) — the executable form of the test-driven cycle.
- [Release cut](release-cut.md) — building, checksumming, and tagging a release that can never be replaced.
- [Rules grooming](rules-grooming.md) — a sweep for volume carrying no obligation. Never writes.
- [Rules propagation](rules-propagation.md) — the sole writer of every rule edit, stopping at this repository's boundary.
- [Rules quality gate](rules-quality-gate.md) — a read-only semantic verdict on one rule state.
- [Worktree to pull request](worktree-to-pull-request.md) — the only path from a change to `main`.
