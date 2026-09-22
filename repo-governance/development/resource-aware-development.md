# Resource-Aware Development

Local compute runs under the checksum-pinned [`./hippo`](../../hippo) wrapper, which arbitrates contention on one shared workstation across every repository the maintainer runs.

```sh
./hippo run --class ephemeral --resource-tier heavy --disk-path . -- cargo xtask test-quick
```

The `pre-push` hook already does this. RHINO has no exemption from the guard and does not grant itself one.

Use `light` for narrow static checks, `standard` for ordinary checks and writers, and `heavy` for complete gates, full builds, full suites, and release assets. A schema-3 waiter keeps one FIFO identity and launches its payload at most once. `hippo.identity.json` labels RHINO in live status and bounded history; `--tag checkout=worktree --tag plan=<slug>` adds privacy-safe per-run context without changing the repository source.

## Exit Codes

HIPPO answers with a status and a `hippo: [hippo.area.reason]` line on stderr. Two reasons under one
status can need opposite responses, so read both.

- **`124`** — a limit stopped the work. `hippo.limit.capacity-deferred` retries only when a new
  schema-1 receipt proves `never-started`; `hippo.limit.pressure-shed` and a `started-safety-stop`
  need payload-specific recovery; `hippo.limit.storage-blocked` needs storage cleaned, since waiting
  frees no disk.
- **`125`** — HIPPO started nothing. `hippo.coordination.protocol-mismatch` is never retried: drain
  or upgrade the incompatible peer first. `hippo.policy.replan-required` and the `hippo.config.*`
  reasons mean replanning the work, not reshaping the call until admitted.
- **`2`** — the call itself is unusable; read the diagnostic and fix it.
- **`126`, `127`** — the guarded command cannot be executed, or is not there.
- **`1`** — the work ran and the answer is empty: a result, never a capacity signal.

Child codes pass through, including ones colliding with a status HIPPO uses; only HIPPO's own
failures write that `hippo:` line. Recovery and status commands run unguarded, since a guard
blocking the tool that diagnoses it is a deadlock.

Use `./hippo status`, `./hippo watch --source rhino`, and `./hippo history --since 30d --source rhino` from either the primary checkout or a contained `worktrees/<task>` checkout. If that worktree has no ignored `hippo.local.json`, its wrapper uses the primary checkout's copy. Every checkout uses the shared default state root; set `HIPPO_ROOT` only for isolated tests.
Raw evidence rolls for seven days and compacted daily summaries roll for 30 days under byte caps, so the shared log cannot grow without bound.

`hippo.lock` pins executable identity, not governance semantics. Before changing consumer behavior, read the Hippo repository at the commit in `hippo.lock`, especially its exit-code and recovery references, then reconcile this rule, Gherkin, and harness checks with the capabilities that commit actually provides. Never infer capability from SemVer ordering or copy a release number into the rule.

## Enforcement

The `pre-push` hook covers the push path. It does not cover an agent typing a build command directly, and that gap is not theoretical: an unguarded fan-out in a sibling repository drove the shared workstation into severe memory pressure and forced a restart. HIPPO cannot defer or shed work it was never told about, so the host reached critical pressure while the scheduler still reported `normal` — a failure that is unrecoverable after the fact rather than merely slow.

[`.claude/hooks/require-hippo-boundary.sh`](../../.claude/hooks/require-hippo-boundary.sh) therefore refuses a compute-bearing command carrying no outer boundary, before the process spawns. It decides only _whether_ a boundary is present, never _which_ class is right; class choice needs intent and stays a judgment. Claude Code and Codex both bind it, and the script is byte-identical to the copy every other consuming repository carries — one file, so a hardening fix cannot land in one copy and quietly miss the rest.

Verbs match only in command position, so `rg 'cargo build'` is not refused. A guard that blocks ordinary searching is one that gets switched off, and a switched-off guard protects nothing anywhere.

Two gaps stay open and recorded. A verb reached through an interpreter or a Makefile target is not in command position and passes; text matching cannot close that in principle. And enforcement reaches only what a harness exposes.

## Not in CI

The [pull-request gate](../../.github/workflows/pr-quality-gate.yml) does not wrap its jobs in `./hippo`. A dedicated ephemeral runner has no contention to arbitrate, so the wrapper could only add a download, a checksum verification, and an exit-`124` path that cannot occur. The guard stays where the contention is real.

The gate does verify that the wrapper still resolves and runs, on a clean machine, whenever the wrapper or its lock changes — that is the one condition a local hook cannot observe.

## Related

- [Quality gates](quality-gates.md)
