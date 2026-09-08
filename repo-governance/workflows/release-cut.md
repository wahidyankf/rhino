# Release Cut

A published release is permanent. A tag is never replaced, and a consumer that pinned it must get the same bytes forever. Everything below exists to make a mistake impossible rather than recoverable.

Run this from the **primary checkout on local `main`**, never from a `worktrees/` checkout.

## Preconditions

- The commit to release is already on `origin/main`, reached through a pull request.
- Local `main` equals `origin/main`.
- The working tree is clean.
- The quick gate passes on that exact commit.
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

3. **Build the archive for this platform.** One platform per invocation, deliberately — the release matrix builds each archive on a runner of that architecture rather than cross-compiling, so every published executable has actually started on the operating system it claims.

   ```sh
   ./hippo run --class ephemeral --disk-path . -- cargo xtask dist
   ```

4. **Record the digests.** Only through the task; never by hand.

   ```sh
   ./hippo run --class ephemeral --disk-path . -- cargo xtask checksums
   ```

5. **Verify embedded identity.** Each executable's `version --json` must report the tag and the commit exactly. A mismatch means the archive was built from something other than what is being tagged.

6. **Tag and push the tag.** The release workflow builds and publishes every platform archive plus `checksums.txt`.

7. **Verify the published release** before telling anyone it exists: an archive per supported platform, a `checksums.txt` covering all of them, and a downloaded archive whose digest matches.

## If Something Is Wrong

A defect becomes a **new patch version and a new pin**. Never replace a tag, never re-upload an asset, and never weaken checksum verification to accommodate a bad artifact. A consumer bootstrap that fails closed on a mismatch is behaving correctly; the fix is upstream of it.

## Related

- [The public contract](../development/public-contract.md)
- [Integration path](../conventions/integration-path.md)
