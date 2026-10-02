# Workflows

Repeatable procedures, within every higher level, in three groups. A workflow may compose other workflows. Where a
workflow and a higher level disagree, the higher level wins and the workflow changes.

Several here carry a terminal contract — a fixed set of results, one of which every run must return. A run that ends
without one of them has not finished.

## Directory Map

- [Plan](plan/README.md) — the plan lifecycle: planning, execution, grooming, and the execution check.
- [Quality](quality/README.md) — every quality gate with its propagation, and every single-pass review.
- [Maintenance](maintenance/README.md) — upkeep and delivery: clean-up, rules grooming, release, and the path to `main`.
