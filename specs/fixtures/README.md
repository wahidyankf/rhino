# Retained Fixtures

These files are retained reference inputs for the grouped-v2 contract: consumer configuration, rejection fixtures, and
the command-line conformance manifest. No Go test reads them. The executable corpus in `tests/bdd` builds its own inputs
inline, in one temporary repository per scenario, so a scenario never depends on a file that can drift away from it.

## Directory Map

- [cli-conformance/](cli-conformance/README.md) — the portable command-line interface assertion manifest and the terms
  on which this copy is owned here.
- [v0-4/](v0-4/README.md) — grouped-v2 configuration, adversarial fixtures, and the acceptance manifest for the stable
  contract.
