---
name: release-cut
description: Build, checksum, and tag a RHINO release from the primary checkout, where a published tag is permanent and is never replaced.
---

# Release Cut

A published release is permanent. A consumer pins it by version, commit, and checksum, and must get the same bytes forever. The rules are in [the workflow](../../../repo-governance/workflows/release-cut.md); this file is the sequence.

Run from the **primary checkout on local `main`**, never from a `worktrees/` checkout.

## Preconditions

- The commit is already on `origin/main`, reached through a pull request.
- Local `main` equals `origin/main`, working tree clean.
- `CHANGELOG.md` describes this version, and `README.md` and `docs/` are true to the binary being built.

## Sequence

```sh
git fetch origin && git merge-base --is-ancestor HEAD origin/main
git tag -l "v<version>"; git ls-remote --tags origin "v<version>"
./hippo run --class ephemeral --disk-path . -- cargo xtask dist
./hippo run --class ephemeral --disk-path . -- cargo xtask checksums
```

The ancestry check is what enforces "a release describes a commit reachable from the default branch". The tag lookups must both come back empty: if either finds the tag, **stop** and choose a new patch version.

`cargo xtask dist` builds one platform per invocation, deliberately — each published archive is built on a runner of its own architecture, so every executable has actually started on the operating system it claims.

Then confirm each executable's `version --json` reports the tag and commit exactly, tag, push the tag, and verify the published release: an archive per supported platform, a `checksums.txt` covering all of them, and a downloaded archive whose digest matches.

## Refusals

- Never delete, move, or replace a tag.
- Never re-upload an asset to an existing release.
- Never weaken checksum verification to accommodate a bad artifact. A consumer bootstrap failing closed on a mismatch is behaving correctly.
- Never record digests by hand.

A defect becomes a new patch version and a new pin.
