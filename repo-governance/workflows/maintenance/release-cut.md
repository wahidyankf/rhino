# Release Cut

Published releases are permanent: never replace tags; pinned consumers must receive identical bytes forever.

Run this from the **primary checkout on local `main`**, never from a `worktrees/` checkout.

## Preconditions

- Release authorization comes from either the adopted
  [Upstream Tool Defects](../../development/upstream-tool-defects.md) standard for a merged defect fix, or explicit
  owner approval for a compatible feature release.
- The commit to release is already on `origin/main`, reached through a pull request.
- Local `main` equals `origin/main`, reconciled after the last merge by the
  [integration path](../../conventions/integration-path.md) rather than assumed.
- The working tree is clean.
- `./scripts/test.sh` passes on that exact commit, and the Main Test run for it has passed on both operating systems.
  The release workflow refuses a tag without that run.
- `CHANGELOG.md` describes this version, and a [docs quality gate](../quality/docs-quality-gate.md) verdict on subject
  `all`, run before the release-prep pull request, is recorded. A documented contract the binary breaks is fixed in code
  before the tag.

## Size Rehearsal

When a candidate exceeds a platform size ceiling, collect all four native measurements before changing any budget. Run
the manual `Release Size Rehearsal` workflow for a default-branch candidate. For an unmerged **draft** candidate, a
maintainer applies the `release-size-rehearsal` label; remove and reapply it to request a fresh head. The run builds
each archive with the release archiver and cannot publish a tag or asset. Download its aggregate artifact and record the
byte counts with the toolchain and commit. A partial matrix is evidence of nothing: do not change a ceiling until the
aggregate names all four release targets and a review explains the material change. Rehearsal never replaces the normal
release artifact suite.

## Procedure

1. **Verify the commit is reachable from the default branch.**

   ```sh
   git fetch origin && git merge-base --is-ancestor HEAD origin/main
   ```

2. **Confirm the tag does not exist**, locally or on the remote. If it does, stop: choose a new patch version. Never
   delete or move a tag.

   ```sh
   git tag -l "v<version>" && git ls-remote --tags origin "v<version>"
   ```

3. **Confirm the Main Test run passed for this commit.** The count must be at least `1`.

   ```sh
   gh api "repos/{owner}/{repo}/actions/workflows/main-test.yml/runs?head_sha=$(git rev-parse HEAD)&status=success" \
     --jq .total_count
   ```

4. **Build the archives and verify the set.** The script refuses a dirty checkout or a commit that is not HEAD. Use the
   version without any prerelease suffix: the script refuses `v0.5.0-rc.1` with exit `1`. It writes four archives and
   `checksums.txt`.

   ```sh
   ./hippo run --class ephemeral --resource-tier standard --disk-path . -- \
     ./scripts/build-release.sh "v<version>" "$(git rev-parse HEAD)" dist
   (cd dist && sha256sum -c checksums.txt)
   ```

   On macOS, use `shasum -a 256 -c checksums.txt`.

5. **Verify embedded identity.** Unpack the archive for this host and ask the executable for its version. A mismatch
   means the archive was not built from the tagged commit.

   ```sh
   tar -xzf dist/rhino-x86_64-unknown-linux-gnu.tar.gz -C dist &&
     test "$(dist/rhino version)" = "v<version>"
   ```

6. **Write the release notes** following the [release notes standard](release-notes.md). Screen the final text before
   the tag exists. Exit `0` is required.

   ```sh
   scripts/public-safety/public-safety.sh --surface release --text "v<version>" --text "$(cat <notes-file>)"
   ```

   Keep `<notes-file>` outside the repository.

7. **Tag and push the tag.** Only after step 6 exits `0`. The release workflow checks the tag's commit against `main`
   and the Main Test run, builds and verifies the four archives, the grouped v2 schema, and `checksums.txt`, and
   publishes them with GitHub's generated notes.

   ```sh
   git tag -a "v<version>" -m "v<version>" && git push origin "v<version>"
   ```

8. **Replace the generated notes.** Right after the workflow publishes, replace the body with the notes from step 6. The
   generated body is a placeholder until this step runs.

   ```sh
   gh release edit "v<version>" --notes-file <notes-file>
   ```

9. **Verify the published release** before telling anyone it exists: download the four archives and `checksums.txt`, run
   `sha256sum -c checksums.txt`, and compare the grouped schema with the tagged commit, as the release notes say.

## If Something Is Wrong

A defect becomes a **new patch version and a new pin**. Never replace a tag, never re-upload an asset, and never weaken
checksum verification to accommodate a bad artifact. A consumer bootstrap that fails closed on a mismatch is behaving
correctly; the fix is upstream of it.

## Related

- [Release notes standard](release-notes.md)
- [Quality gates](../../development/quality-gates.md)
- [The public contract](../../development/public-contract.md)
- [Integration path](../../conventions/integration-path.md)
