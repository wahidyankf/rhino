---
description: >-
  Defines the one bounded, advisory contract every quality gate follows: a read-only checker, a frozen ledger, one
  separate writer, at most three cycles, explicit termination, and four verdicts that never block a caller.
when_to_use: >-
  Use when writing, adopting, running, or reviewing a workflow named `<family>-quality-gate`, or when deciding what a
  gate may report, how it ends, and who may start it.
---

# Quality Gate Contract

Every workflow named `<family>-quality-gate` follows this contract. A gate judges what deterministic tooling cannot
prove. It freezes its subject, has a read-only checker audit it, hands the blocking findings to one separate writer, and
audits again, for at most three cycles. Each gate file then states only what is specific to its family.

The writer side is [Sole-Writer Propagation](sole-writer-propagation.md). Every other repeated operation stays under
Bounded Convergence.

This standard implements Explicit Over Implicit, Evidence Over Assertion, and
[Minimal Sufficiency](../../principles/minimal-sufficiency.md).

## Roles

| Role    | Who                                                 | May edit files                    |
| ------- | --------------------------------------------------- | --------------------------------- |
| Gate    | The workflow `<family>-quality-gate`                | No                                |
| Checker | The agent `<family>-checker`                        | No                                |
| Writer  | The workflow `<family>-propagation`                 | Yes, only rows of a frozen ledger |
| Fixer   | The agent `<family>-fixer`, which runs the writer   | Yes, as the writer's executor     |
| Caller  | A person, or a workflow named in the gate's `Entry` | Records the verdict               |

Judging and writing never share a role. A checker that repaired its own findings would audit its own work next cycle.

## Required Sections

Every gate file carries these headings, in this order. Where the repository declares a structural check for gates, it
enforces their presence.

| Heading                     | Holds                                                                       |
| --------------------------- | --------------------------------------------------------------------------- |
| `## Entry`                  | Explicit request, plus the exact list of calling workflows                  |
| `## Inputs`                 | `subject` (required), `mode`, and `max-cycles`; nothing else bounds the run |
| `## Deterministic Boundary` | Table: property → the tool or test that owns it → excluded from findings    |
| `## Cycle`                  | The family's audit lens and its writer, by reference to this contract       |
| `## Termination`            | The termination table of this contract, unchanged                           |
| `## Verdict`                | The four verdicts and what the caller does with each                        |
| `## Ledger`                 | The ledger path pattern under `local-tmp/quality/`                          |

A gate file never restates the sequence, the scoring, or the termination rules in its own words. It links them, so the
contract changes in one place.

## Entry

A gate starts only on an explicit request that names it, or from a workflow listed under its `## Entry` heading. It
never starts because another workflow implies it, and a caller joins that list only when the caller's own text already
names the gate. This repository's listed callers are:

| Gate                | Listed callers              |
| ------------------- | --------------------------- |
| `plan-quality-gate` | `plan-planning`             |
| `docs-quality-gate` | `release-cut`               |
| every other gate    | none; explicit request only |

How this repository maps the rest of the contract is recorded in the [Quality Gate Adapter](quality-gate-adapter.md).

## Modules

1. [Inputs, Scoring, and the Deterministic Boundary](quality-gate-contract/001-inputs-scoring-and-boundary.md)
2. [Sequence and Termination](quality-gate-contract/002-sequence-and-termination.md)
3. [Verdicts, Ledger, and Relations](quality-gate-contract/003-verdicts-ledger-and-relations.md)
