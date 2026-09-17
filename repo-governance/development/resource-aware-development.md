# Resource-Aware Development

Local compute runs under the checksum-pinned [`./hippo`](../../hippo) wrapper, which arbitrates contention on one shared workstation across every repository the maintainer runs.

```sh
./hippo run --class ephemeral --resource-tier heavy --disk-path . -- cargo xtask test-quick
```

The `pre-push` hook already does this. RHINO has no exemption from the guard and does not grant itself one.

Use `light` for narrow static checks, `standard` for ordinary checks and writers, and `heavy` for complete gates, full builds, full suites, and release assets. A schema-3 waiter keeps one FIFO identity and launches its payload at most once. `hippo.identity.json` labels RHINO in live status and bounded history; `--tag checkout=worktree --tag plan=<slug>` adds privacy-safe per-run context without changing the repository source.

## Exit Codes

- **`75`** — retry only when a new schema-1 receipt proves `never-started`; pressure-shed, storage-shed, `started-safety-stop`, and child-owned `75` require payload-specific recovery.
- **`76`** — never retry. Inspect `./hippo status`, drain or upgrade the incompatible peer, then retry the original command. A legacy client without distinct exit `76` can still report the mismatch as `75`, but has no qualifying receipt.
- **`73`** — insufficient storage. Clean up, then retry.
- **`78`** — the request cannot be satisfied as configured. Replan the work rather than reshaping the invocation until it is admitted.
- **`1`** — diagnose malformed shared state or Hippo-owned post-launch cleanup; never classify it as capacity.

Child codes pass through, including `75` and `76`; task-failed evidence without a new `never-started` receipt keeps them child-owned. Recovery and status commands run directly, unguarded, because a guard that blocks the tool used to diagnose the guard is a deadlock.

Use `./hippo status`, `./hippo watch --source rhino`, and `./hippo history --since 30d --source rhino` from either the primary checkout or a contained `worktrees/<task>` checkout. If that worktree has no ignored `hippo.local.json`, its wrapper uses the primary checkout's copy. Every checkout uses the shared default state root; set `HIPPO_ROOT` only for isolated tests.
Raw evidence rolls for seven days and compacted daily summaries roll for 30 days under byte caps, so the shared log cannot grow without bound.

`hippo.lock` pins executable identity, not governance semantics. Before changing consumer behavior, read the Hippo repository at the commit in `hippo.lock`, especially its exit-code and recovery references, then reconcile this rule, Gherkin, and harness checks with the capabilities that commit actually provides. Never infer capability from SemVer ordering or copy a release number into the rule.

## Enforcement

The `pre-push` hook covers the push path. It does not cover an agent typing a build command directly, and that gap is not theoretical: an unguarded fan-out in a sibling repository drove the shared workstation into severe memory pressure and forced a restart. HIPPO cannot defer or shed work it was never told about, so the host reached critical pressure while the scheduler still reported `normal` — a failure that is unrecoverable after the fact rather than merely slow.

[`.claude/hooks/require-hippo-boundary.sh`](../../.claude/hooks/require-hippo-boundary.sh) therefore refuses a compute-bearing command carrying no outer boundary, before the process spawns. It decides only _whether_ a boundary is present, never _which_ class is right; class choice needs intent and stays a judgment. Claude Code and Codex both bind it, and the script is byte-identical to the copy every other consuming repository carries — one file, so a hardening fix cannot land in one copy and quietly miss the rest.

Verbs match only in command position, so `rg 'cargo build'` is not refused. A guard that blocks ordinary searching is one that gets switched off, and a switched-off guard protects nothing anywhere.

Two gaps stay open and recorded. A verb reached through an interpreter or a Makefile target is not in command position and passes; text matching cannot close that in principle. And enforcement reaches only what a harness exposes.

## Not in CI

The [pull-request gate](../../.github/workflows/pr-quality-gate.yml) does not wrap its jobs in `./hippo`. A dedicated ephemeral runner has no contention to arbitrate, so the wrapper could only add a download, a checksum verification, and an exit-`75` path that cannot occur. The guard stays where the contention is real.

The gate does verify that the wrapper still resolves and runs, on a clean machine, whenever the wrapper or its lock changes — that is the one condition a local hook cannot observe.

## Related

- [Quality gates](quality-gates.md)
