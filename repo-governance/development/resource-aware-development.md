# Resource-Aware Development

Heavy local work runs under the checksum-pinned [`./hippo`](../../hippo) wrapper, which arbitrates contention on one shared workstation across every repository the maintainer runs.

```sh
./hippo run --class ephemeral --disk-path . -- cargo xtask test-quick
```

The `pre-push` hook already does this. RHINO has no exemption from the guard and does not grant itself one.

## Exit Codes

- **`75`** — the host was busy. Retry that same invocation once the condition clears. Never bypass it, never duplicate it into a second queued attempt, and never change its class to get through.
- **`73`** — insufficient storage. Clean up, then retry.
- **`78`** — the request cannot be satisfied as configured. Replan the work rather than reshaping the invocation until it is admitted.

Recovery and status commands run directly, unguarded, because a guard that blocks the tool used to diagnose the guard is a deadlock.

## Enforcement

The `pre-push` hook covers the push path. It does not cover an agent typing a build command directly, and that gap is not theoretical: an unguarded fan-out in a sibling repository drove the shared workstation into severe memory pressure and forced a restart. HIPPO cannot defer or shed work it was never told about, so the host reached critical pressure while the scheduler still reported `normal` — a failure that is unrecoverable after the fact rather than merely slow.

[`.claude/hooks/require-hippo-boundary.sh`](../../.claude/hooks/require-hippo-boundary.sh) therefore refuses a compute-bearing command carrying no outer boundary, before the process spawns. It decides only *whether* a boundary is present, never *which* class is right; class choice needs intent and stays a judgment. Claude Code and Codex both bind it, and the script is byte-identical to the copy every other consuming repository carries — one file, so a hardening fix cannot land in one copy and quietly miss the rest.

Verbs match only in command position, so `rg 'cargo build'` is not refused. A guard that blocks ordinary searching is one that gets switched off, and a switched-off guard protects nothing anywhere.

Two gaps stay open and recorded. A verb reached through an interpreter or a Makefile target is not in command position and passes; text matching cannot close that in principle. And enforcement reaches only what a harness exposes.

## Not in CI

The [pull-request gate](../../.github/workflows/pr-quality-gate.yml) does not wrap its jobs in `./hippo`. A dedicated ephemeral runner has no contention to arbitrate, so the wrapper could only add a download, a checksum verification, and an exit-`75` path that cannot occur. The guard stays where the contention is real.

The gate does verify that the wrapper still resolves and runs, on a clean machine, whenever the wrapper or its lock changes — that is the one condition a local hook cannot observe.

## Related

- [Quality gates](quality-gates.md)
