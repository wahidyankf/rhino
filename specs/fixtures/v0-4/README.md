# Rhino v0.4 Fixtures

Frozen Phase 2 contracts live here. They describe public behavior and synthetic repository state only; no consumer
configuration, workstation path, secret, or executable output belongs in this tree.

## Directory Map

- [Acceptance Manifest](acceptance-manifest.tsv) — every non-Delete legacy capability mapped to fixture IDs, ACs, owner,
  and delivery phase.
- [Phase 2 RED Fixtures](phase-2-red-fixtures.json) — typed projection, snapshot/replay, external Nx, harness, and
  safe-operation inputs that later GREEN phases must satisfy.
- [Consumer v2 Configuration](consumer-repo-config.yml) — a synthetic consumer fixture whose modeline pins an
  immutable release schema rather than asking runtime to fetch one.
