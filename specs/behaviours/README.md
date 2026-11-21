# Stable Behaviours

The stable corpus is the grouped-v2 contract, the validators that later releases added to it, and the command-line
surface. Every scenario binds at unit level, and at integration and end-to-end level unless it carries a reasoned
exemption. The `tests/bdd` suite drives the built `rhino` binary as a child process, in an isolated temporary
repository, and asserts on its exit code, its streams, its JSON, and the repository state it leaves behind. The
environment-vault feature also binds at the unit adapter (an in-process in-memory tree) and at the integration adapter
(a real temporary Git repository). Those two runners and the CLI runner share one step registry, `tests/bdd/vault`, and
its compliance check fails on any unbound step, unused binding, or ambiguous step. No scenario is skipped. A scenario
that cannot run hermetically is reported with a proposed, reviewed exemption.

The corpus declares only policy owned by `rhino/repo-config/v2`. Predecessor schemas survive only as rejection fixtures.

Run the corpus with `go test ./tests/bdd`.

## Directory Map

- [v0-4-contract.feature](v0-4-contract.feature) — configuration, policy, lifecycle, harness, environment, toolchain,
  and stable predecessor-rejection contracts.
- [quality-gates.feature](quality-gates.feature) — the quality-gate structure checks QG01–QG11 under
  `policies.governance.quality-gates`.
- [regular-files.feature](regular-files.feature) — acceptance scenarios for explicit Git destination-mode input
  selection.
- [cli-surface.feature](cli-surface.feature) — build identity (`version`), the `--root` repository selection, process
  exit codes (126, 127, 130, and 143 for signals), scan exclusion scope, and toolchain output and timeout bounds.
- [environment-vault-validation.feature](environment-vault-validation.feature) — the `environment.vault` rules:
  unclassified ignored files and templates, GitHub workflow references, closed name grammar, the resolution of a per-key
  `env` reference, and no printed value.
- [environment-vault.feature](environment-vault.feature) — `env pull`, `env push`, and `env diff`: restore, seed,
  compare, and overwrite with recovery points; refusal of an unusable vault; exact-name matching; a per-key reference
  that reads another entry's item and that push never writes; and no printed value.
- [wiring.feature](wiring.feature) — `gate wiring validate`: the registry, hook, and workflow wiring rules W01–W07 and
  the `gates.wiring.local-surfaces` allowed list.
- [behaviour-coverage.feature](behaviour-coverage.feature) — `behaviour coverage validate`: the bound corpus rules
  B01–B09, the exemption semantics, the `--rules` selection, and the whole-file glob check.
