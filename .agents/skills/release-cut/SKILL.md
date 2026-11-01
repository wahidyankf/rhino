---
name: release-cut
description: Build, checksum, and tag a RHINO release from the primary checkout, where a published tag is permanent and is never replaced.
when_to_use: >-
  Use when a merged revision on main must become a published RHINO release, once its changelog and docs
  quality gate verdict are recorded.
---

# Release Cut

A published release is permanent. A consumer pins it by version, commit, and checksum, and must get the same bytes
forever. The rules are in [the workflow](../../../repo-governance/workflows/maintenance/release-cut.md); this file is
the sequence.

Run from the **primary checkout on local `main`**, never from a `worktrees/` checkout.

## Preconditions

- The commit is already on `origin/main`, reached through a pull request.
- Local `main` equals `origin/main`, working tree clean.
- `CHANGELOG.md` describes this version, and a
  [docs quality gate](../../../repo-governance/workflows/quality/docs-quality-gate.md) verdict on subject `all` is
  recorded.
- `./scripts/test.sh` passed on the exact commit, and its Main Test run passed on both operating systems.

## Sequence

```sh
git fetch origin && git merge-base --is-ancestor HEAD origin/main
git tag -l "v<version>"; git ls-remote --tags origin "v<version>"
gh api "repos/{owner}/{repo}/actions/workflows/main-test.yml/runs?head_sha=$(git rev-parse HEAD)&status=success" \
  --jq .total_count
./hippo run --class ephemeral --resource-tier standard --disk-path . -- \
  ./scripts/build-release.sh "v<version>" "$(git rev-parse HEAD)" dist
(cd dist && sha256sum -c checksums.txt)
tar -xzf dist/rhino-x86_64-unknown-linux-gnu.tar.gz -C dist &&
  test "$(dist/rhino version)" = "v<version>"
```

The ancestry check is what enforces "a release describes a commit reachable from the default branch". The tag lookups
must both come back empty: if either finds the tag, **stop** and choose a new patch version. The Main Test count must be
at least `1`. On macOS, use `shasum -a 256 -c checksums.txt`.

`scripts/build-release.sh` builds all four archives from one runner: each target is cross-compiled from the same commit
with `CGO_ENABLED=0`, and the Go archiver writes the members with the commit's time, so the bytes depend on the commit
alone. It refuses a dirty checkout or a commit that is not HEAD. The version argument must carry no prerelease suffix,
so a candidate `v0.5.0-rc.1` is built as `v0.5.0`. The `version` check runs the executable out of an unpacked archive,
so it proves the published bytes rather than a copy beside them. The line above checks the Linux archive; on macOS,
unpack the matching Darwin archive instead.

Then write the release notes, screen them with `scripts/public-safety/public-safety.sh --surface release` (exit `0` is
required), tag and push the tag, and verify the published release: the four archives, `checksums.txt`, and the grouped
v2 schema, with a downloaded archive whose digest matches. The release workflow repeats the checksum and version checks
before it publishes, and it refuses a tag whose commit has no passing Main Test run.

## Refusals

- Never delete, move, or replace a tag.
- Never re-upload an asset to an existing release.
- Never weaken checksum verification to accommodate a bad artifact. A consumer bootstrap failing closed on a mismatch is
  behaving correctly.
- Never record digests by hand.

A defect becomes a new patch version and a new pin.
