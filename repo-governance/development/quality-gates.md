# Quality Gates

Three checks, the first of which runs before the other two.

## Public Safety

[`scripts/public-safety/`](../../scripts/public-safety/README.md) runs first on `commit-msg`, `pre-commit`, `pre-push`,
and in CI. It fails fast on purpose: a gate that screened for prohibited material after a formatter had rewritten the
tree would be screening the wrong bytes. Its own case suite is offline, uses a stub scanner, and runs on every one of
those surfaces. The pinned scanner it bootstraps is verified by digest, and its detection is proved by a canary before
any repository material is read.

## The Quick Gate

`./scripts/test-quick.sh` is what the `pre-push` hook runs through `repo-config.yml`, so it holds only what is fast
enough to run on every push:

- `gofmt -l cmd internal` prints nothing;
- `go vet ./...` is clean;
- `go test -race -count=1 ./...` passes, with a race in this tool treated as a finding rather than a flake. That run
  includes the behaviour corpus: `go test ./tests/bdd` executes every scenario in `specs/behaviours` against the built
  binary, and fails on any skipped scenario;
- deterministic-core coverage (domain, application, and config) meets its pinned floor;
- the built binary answers the public contract on a fixture.

## The Full Gate

`./scripts/test.sh` runs the quick gate, then `go test -race -count=1 ./tests/bdd/` so the behaviour corpus also runs
under the race detector on its own, then the public-safety wrapper's suite, then `go tool govulncheck ./...`. The
vulnerability tool is pinned in `go.mod`, so it needs no install step, but it needs network access to the vulnerability
database. The full gate is not a hook. It runs in the [Main Test workflow](../../.github/workflows/main-test.yml) on
every push to `main`, on `ubuntu-24.04` and `macos-15`.

## Where Each Gate Runs

- **Commit and push:** the local hooks, which call `rhino gate run` with the surface that `repo-config.yml` declares.
- **Pull request:** the [PR Quality Gate](../../.github/workflows/pr-quality-gate.yml), with public safety, the quick
  gate, and the pinned HIPPO bootstrap. Its aggregate `Quality gate` job is the only required check. A skipped or
  cancelled dependency is a failure there, because a skipped required check never reports at all.
- **Every push to `main`:** [Main Test](../../.github/workflows/main-test.yml), with the full gate on both operating
  systems, a Linux release build, and a macOS rebuild whose checksums must match the Linux ones byte for byte.
- **Daily:** the [Scheduled](../../.github/workflows/scheduled.yml) quick gate, unfiltered.
- **Tag:** the [Release workflow](../../.github/workflows/release.yml), which refuses a tag whose commit has no passing
  Main Test run before it builds or publishes anything.

All Linux workflows pin `ubuntu-24.04`, and the macOS legs pin `macos-15`. A `-latest` label moves under a release, so
none is used.

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
