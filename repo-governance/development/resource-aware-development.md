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

## Not in CI

The [pull-request gate](../../.github/workflows/pr-quality-gate.yml) does not wrap its jobs in `./hippo`. A dedicated ephemeral runner has no contention to arbitrate, so the wrapper could only add a download, a checksum verification, and an exit-`75` path that cannot occur. The guard stays where the contention is real.

The gate does verify that the wrapper still resolves and runs, on a clean machine, whenever the wrapper or its lock changes — that is the one condition a local hook cannot observe.

## Related

- [Quality gates](quality-gates.md)
