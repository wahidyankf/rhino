# Rules Quality Gate

Run only when the user explicitly names this gate or unambiguously directs its semantic audit. Do not infer authorization from a rule change, a review request, propagation, or another workflow.

Produce one read-only semantic verdict for one proposed or effective rule state. This workflow never edits rules and never starts another gate run. [Rules propagation](rules-propagation.md) is the sole writer and the mandatory continuation for any non-passing finding.

## Sufficiency and Ownership

A passing rule is good enough for the stated need, scope, and known risk — not perfect, exhaustive, or future-proof. Do not raise findings for wording preference, speculative cases, optional explanation, or automation with no demonstrated need. Apply [minimal sufficiency](../principles/minimal-sufficiency.md).

This gate owns semantic rule quality. Deterministic tooling owns machine-decidable checks — links, directory maps, word budgets, Mermaid, harness parity. Do not manually reproduce, sample, or second-guess them; consume their result only where this workflow requires effective-state verification.

For a deterministic check proposed but not yet implemented, proposal mode verifies only that its ownership, executable delivery, and proof obligation are explicit. Effective mode requires the target to exist and pass. Never simulate a future tool.

## Modes, Snapshot, and Ledger

Run in exactly one mode:

- `PROPOSAL` compares the requested outcome with current effective rules, before edits.
- `EFFECTIVE` evaluates the repository after propagation edits.

Freeze the mode, requested outcome and rationale, intended strength, scope and consumers, proposed move or deletion, relevant canonical sources, enforcement route, Git revision, and dirty paths. A material external change returns `BLOCKED_INPUT_CHANGED`; it never restarts the gate.

Audit without editing. Record a finite ledger of `ID`, canonical source, material semantic gap, required resolution, evidence, and status — `OPEN`, `RESOLVED`, `NOT_APPLICABLE`, or `BLOCKED`. Admit only a rule violation, or a gap making the outcome unsafe, contradictory, undiscoverable, or materially ambiguous. `NOT_APPLICABLE` requires evidence. Preserve everything through compaction under [governance continuity](../principles/governance-continuity.md).

## Semantic Audit

Inspect only the affected rule, its point-of-use routes, relevant higher authority, and directly overlapping guidance. Decide whether:

1. the need, outcome, and rationale are concrete enough to evaluate;
2. `must`, `should`, or `may` expresses the intended strength;
3. scope, trigger, action or prohibition, boundaries, and necessary exceptions are explicit;
4. the canonical level is correct and no lower rule conflicts with higher authority;
5. one canonical source owns the meaning while concise point-of-use links make it discoverable without duplication;
6. each enforcement claim names a truthful class and route, with required evidence where automation cannot decide;
7. the instruction survives compaction and handoff at every entry point;
8. a reasonable reader can act without inventing policy; and
9. a move or deletion preserves unique intent and updates affected consumers.

## Results and Mandatory Handoff

In `PROPOSAL` mode, return `PASS_NO_CHANGE` when current effective meaning already satisfies the request; otherwise emit `NEEDS_PROPAGATION` with the ledger, evidence, and any required external decision.

In `EFFECTIVE` mode, run:

```sh
./hippo run --class ephemeral --disk-path . -- cargo xtask self-validate
```

Return `PASS_EFFECTIVE` only when the ledger is clear and tooling passes; otherwise emit `NEEDS_PROPAGATION`.

`NEEDS_PROPAGATION` is a non-terminal handoff, never a blocked result. The caller must immediately run propagation with the frozen outcome, ledger, and evidence, then report only propagation's terminal result. This gate can therefore end only in `PASS_NO_CHANGE` or `PASS_EFFECTIVE`. It never ends blocked, repairs rules, reruns itself, or authorizes commit or push.
