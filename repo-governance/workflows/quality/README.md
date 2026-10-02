# Quality Workflows

Every quality gate with its propagation, and every single-pass review.

## Directory Map

- [Docs propagation](docs-propagation.md) — the only writer of every document edit, run automatically with each change a
  document describes.
- [Docs quality gate](docs-quality-gate.md) — a read-only audit of documents, handing findings to propagation and
  auditing again until two consecutive audits are clean. Explicit request or release only.
- [Gherkin implementation review](gherkin-implementation-review.md) — semantic review of what a binding actually
  asserts.
- [Harness parity verification](harness-parity-verification.md) — the read-only audit of the coding-harness contract.
- [Harness propagation](harness-propagation.md) — one canonical edit, every adapter reconciled with it.
- [Plan quality gate](plan-quality-gate.md) — a semantic verdict on one plan's readiness. Explicit request only.
- [PR leak review](pr-leak-review.md) — the private review before every push and the posted, current-head review a merge
  requires.
- [PR leak review modules](pr-leak-review/README.md) — leak classes, the push review, and enforcement, in reading order.
- [Red, green, refactor](red-green-refactor.md) — the executable form of the test-driven cycle.
- [Rules propagation](rules-propagation.md) — the sole writer of every rule edit, stopping at this repository's
  boundary.
- [Rules quality gate](rules-quality-gate.md) — a read-only semantic verdict on one rule state.
