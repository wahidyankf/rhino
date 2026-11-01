# Stable Behaviours

The stable corpus is the grouped-v2 contract, the validators that later releases added to it, and the command-line
surface. Every scenario runs through one binding: the `tests/bdd` suite drives the built `rhino` binary as a child
process, in an isolated temporary repository, and asserts on its exit code, its streams, its JSON, and the repository
state it leaves behind. No scenario is skipped. A scenario that cannot run hermetically is reported with a proposed,
reviewed exemption.

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
