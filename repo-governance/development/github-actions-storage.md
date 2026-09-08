# GitHub Actions Storage

This repository runs on the free allowance and stays there. The budget is `$0`, and it is a constraint on design rather than an aspiration.

## Requirements

- Keep artifact and log retention short. Set `retention-days` explicitly on any uploaded artifact; the default is longer than anything here needs.
- Upload an artifact only when something reads it. A release archive is read by the release; a log a human might open once is not an artifact.
- Keep caches small and scoped. A cache key that never hits is storage spent for nothing, and a cache that grows without bound eventually evicts the ones that work.
- Prefer recomputing a cheap thing over caching it. A Rust build cache is worth having; a `node_modules` cache for two dev dependencies is not.
- Watch the total. When storage approaches the allowance, reduce retention or stop producing an artifact — never move to a paid tier silently.

## Runner Time

Runner minutes on a public repository are unmetered, which is why the [pull-request gate](../../.github/workflows/pr-quality-gate.yml) can afford to be a superset of the hooks. That does not license a gate that takes long enough to be worked around; the quick gate stays quick because a slow one gets bypassed.

## Related

- [Quality gates](quality-gates.md)
