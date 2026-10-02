# Stable Behaviours

The stable corpus is the grouped-v2 contract and the validators that later releases added to it. Every scenario binds at
the unit, integration, and E2E boundary. It declares only policy owned by `rhino/repo-config/v2`; predecessor schemas
survive only as rejection fixtures.

## Directory Map

- [v0-4-contract.feature](v0-4-contract.feature) — configuration, policy, lifecycle, harness, environment, toolchain,
  and stable predecessor-rejection contracts.
- [quality-gates.feature](quality-gates.feature) — the quality-gate structure checks QG01–QG11 under
  `policies.governance.quality-gates`.
