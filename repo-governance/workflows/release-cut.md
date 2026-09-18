# Release Cut

A published release is permanent. A tag is never replaced, and a consumer that pinned it must get the same bytes forever. Everything below exists to make a mistake impossible rather than recoverable.

Run this from the **primary checkout on local `main`**, never from a `worktrees/` checkout.

## Preconditions

- The commit to release is already on `origin/main`, reached through a pull request.
- Local `main` equals `origin/main`, reconciled after the last merge by the [integration path](../conventions/integration-path.md) rather than assumed.
- The working tree is clean.
- The quick gate and `cargo xtask schema --check` pass on that exact commit.

## Size Rehearsal

When a candidate exceeds a platform size ceiling, collect all four native measurements before changing any budget.
Run the manual `Release Size Rehearsal` workflow for a default-branch candidate. GitHub does not expose manual dispatch
until that workflow reaches the default branch, so for an unmerged **draft** candidate a maintainer instead applies the
`release-size-rehearsal` label; remove and reapply it to request a fresh head. That label-triggered run checks out the
exact reviewed PR head with the same read-only/no-secret permission. Download its one aggregate artifact and record the
raw executable/archive byte counts with the toolchain and commit. It invokes the same `cargo xtask dist` writer as
release but cannot publish a tag or asset. A partial matrix is evidence of nothing: do not change a ceiling until the
aggregate names all four release targets and a review explains the material change. Then rebuild and rerun the normal
release artifact suite; rehearsal never replaces it.

- `CHANGELOG.md` describes this version, and `README.md` and `docs/` are true to the binary being built.

## Procedure

1. **Verify the commit is reachable from the default branch.** A release describes a commit on `main` or it does not publish.

   ```sh
   git fetch origin && git merge-base --is-ancestor HEAD origin/main
   ```

2. **Confirm the tag does not exist**, locally or on the remote. If it does, stop: choose a new patch version. Never delete or move a tag.

   ```sh
   git tag -l "v<version>" && git ls-remote --tags origin "v<version>"
   ```

3. **Build this platform's archive and stage the grouped schema.** One platform per invocation, deliberately — the release matrix builds each archive on a runner of that architecture rather than cross-compiling, so every published executable has actually started on the operating system it claims. `cargo xtask dist` stages the checked-in v2 schema only after proving it matches the typed model.

   ```sh
   ./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo xtask dist
   ```

4. **Record the digests.** Only through the task; never by hand. The manifest covers every archive and the staged v2 schema.

   ```sh
   ./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo xtask checksums
   ```

5. **Verify embedded identity.** Each executable's `version --json` must report the tag and the commit exactly. A mismatch means the archive was built from something other than what is being tagged.

6. **Screen, then tag and push the tag.** The tag name, any annotation, and the notes the release workflow generates from merged pull requests are published with the release. Screen them first, and tag only on exit `0`:

   ```sh
   gh api repos/<owner>/<repo>/releases/generate-notes -f tag_name=v<version> --jq .body > local-tmp/release-notes.md
   scripts/public-safety/public-safety.sh --surface release --text "v<version>" --file local-tmp/release-notes.md
   ```

   Pass an annotation as one more `--text`. The release workflow builds and publishes every platform archive, the
   grouped v2 schema, and `checksums.txt`.

7. **Verify the published release** before telling anyone it exists: an archive per supported platform, the grouped
   v2 schema, a `checksums.txt` covering every release asset, and downloaded archive/schema bytes whose digests match.

## If Something Is Wrong

A defect becomes a **new patch version and a new pin**. Never replace a tag, never re-upload an asset, and never weaken checksum verification to accommodate a bad artifact. A consumer bootstrap that fails closed on a mismatch is behaving correctly; the fix is upstream of it.

## Related

- [The public contract](../development/public-contract.md)
- [Integration path](../conventions/integration-path.md)
