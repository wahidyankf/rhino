# Workflows

Repeatable procedures, within every higher level. A workflow may compose other workflows. Where a workflow and a higher level disagree, the higher level wins and the workflow changes.

Several here carry a terminal contract — a fixed set of results, one of which every run must return. A run that ends without one of them has not finished.

## Directory Map

- [Coding-harness contract change](coding-harness-contract-change.md) — one canonical edit, every adapter reconciled with it.
- [Coding-harness parity verification](coding-harness-parity-verification.md) — the read-only audit of the same contract.
- [Gherkin implementation review](gherkin-implementation-review.md) — semantic review of what a binding actually asserts.
- [Plan execution](plan-execution.md) — executing one formal plan while keeping its records true.
- [Plan quality gate](plan-quality-gate.md) — a semantic verdict on one plan's readiness. Explicit request only.
- [Red, green, refactor](red-green-refactor.md) — the executable form of the test-driven cycle.
- [Release cut](release-cut.md) — building, checksumming, and tagging a release that can never be replaced.
- [Rules grooming](rules-grooming.md) — a sweep for volume carrying no obligation. Never writes.
- [Rules propagation](rules-propagation.md) — the sole writer of every rule edit, stopping at this repository's boundary.
- [Rules quality gate](rules-quality-gate.md) — a read-only semantic verdict on one rule state.
- [Worktree to pull request](worktree-to-pull-request.md) — the only path from a change to `main`.
