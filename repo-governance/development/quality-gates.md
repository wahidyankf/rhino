# Quality Gates

The gates follow the [Test and Gate Contract](quality/checks/test-and-gate-contract.md). Each local surface holds one
kind of work (C6), and the pull-request gate holds every other check (C8).

## Public Safety

[`scripts/public-safety/`](../../scripts/public-safety/README.md) runs at `pre-push` over the pushed range, and at the
pull-request gate over the tree, the range, and the messages (C11). It runs before history leaves the machine. Its own
case suite is offline, uses a stub scanner, and runs in CI only: on each pull request, and in the full gate that CI runs
on each push to `main`. The pinned scanner it bootstraps is verified by digest, and its detection is proved by a canary
before any repository material is read.

## Local Hooks

Each hook calls `rhino gate run` with its surface, and `repo-config.yml` declares what each surface holds. The allowed
lists live under `gates.wiring.local-surfaces`, and `rhino gate wiring validate` refuses an entry outside them (W07):

- `pre-commit` runs the staged-file formatter only (`format-staged`);
- `commit-msg` runs the commit-message format lint only (`commit-message`);
- `pre-push` runs the leak scan over the pushed range only (`public-safety-range`).

Every other check runs at the pull-request gate, which replays the hooks.

## The Quick Gate

`./scripts/test-quick.sh` is the `test:quick` target. It runs these checks in order, and a race in this tool is a
finding rather than a flake:

- `gofmt -l cmd internal` prints nothing;
- `go vet ./...` is clean;
- `go test -race -count=1` passes over the unit packages: `cmd`, `internal`, and `scripts/release-archive`;
- unit coverage meets its pinned floor, read from that unit run;
- the built binary prints the `version --json` shape with `schemaVersion` 1 and reports one emoji finding on a fixture;
- `rhino behaviour coverage validate` passes. It is static, and it runs no scenario.

## The Integration and End-to-End Targets

`test:integration` runs `go test -race -count=1 ./tests/bdd/...`. Every scenario in `specs/behaviours` runs against the
built binary in a subprocess, and a skipped scenario fails the run. `test:e2e` runs the end-to-end suite under
`tests/e2e/`. Neither target is part of the quick gate. The pull-request gate runs `test:integration` on every pull
request (C17). The scheduled surface runs `test:e2e`, so no pull request waits on it.

## The Full Gate

`./scripts/test.sh` runs the quick gate, the integration and end-to-end targets, the public-safety wrapper's suite, and
then `go tool govulncheck ./...`. The vulnerability tool is pinned in `go.mod`, so it needs no install step, but it
needs network access to the vulnerability database. The full gate is not a hook. It runs in the
[Main Test workflow](../../.github/workflows/main-test.yml) on every push to `main`, on both operating systems pinned
below.

## Where Each Gate Runs

- **Commit and push:** the local hooks above.
- **Pull request:** the [PR Quality Gate](../../.github/workflows/pr-quality-gate.yml), which runs the `pull-request`
  surface: the quick gate, the integration target, and every document and repository check. Its aggregate `Quality gate`
  job is the only required check. A skipped or cancelled dependency is a failure there, because a skipped required check
  never reports at all.
- **Every push to `main`:** [Main Test](../../.github/workflows/main-test.yml), with the full gate on both operating
  systems, a Linux release build, and a macOS rebuild whose checksums must match the Linux ones byte for byte.
- **Daily:** the [Scheduled](../../.github/workflows/scheduled.yml) workflow, which runs the `scheduled` surface
  unfiltered: the quick gate and the end-to-end target.
- **Tag:** the [Release workflow](../../.github/workflows/release.yml), which refuses a tag whose commit has no passing
  Main Test run before it builds or publishes anything.

Every job pins a fixed runner label: `ubuntu-24.04`, `ubuntu-24.04-arm`, `macos-15`, or `macos-15-intel`. A `-latest`
label moves under a release, so none is used.

## Portability

The full gate runs on both operating systems a consumer runs rhino on. A script or test that depends on GNU-only flags,
Linux-only paths, or a `/tmp` that is a symbolic link on macOS fails there. Use `shasum -a 256` where `sha256sum` is
absent, as the release build does, and use the Go archiver rather than the host `tar` for release bytes.

## Test Isolation

A test never reads or writes outside a root it created. Each case gets its own temporary directory or repository, and
removes it. There is no shared fixture directory, because a shared fixture is a test that passes because of another
test.

## Related

- [Release cut](../workflows/maintenance/release-cut.md)
- [Release notes standard](../workflows/maintenance/release-notes.md)
- [Resource-aware development](resource-aware-development.md)
