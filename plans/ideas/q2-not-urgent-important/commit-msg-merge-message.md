# Accept the hook message file Git actually passes for merge commits

The `commit-msg` surface refuses every merge commit: `hook_message_file` accepts only the exact
canonical `COMMIT_EDITMSG` path, but Git passes `MERGE_MSG` (and `SQUASH_MSG`, `COMMIT_EDITMSG`
only for plain commits) as the hook's `$1`. A `git merge` in a gated repository can never complete
its commit locally while this gate is declared.

> Filed 2026-10-25 by a consuming repository under its Upstream Tool Defects standard. First seen by
> that consumer on 2026-10-25, when `git merge origin/main` staged cleanly and the commit-msg hook
> failed. Reproduced on released `v0.15.0` and confirmed present on trunk `f655040` by source
> inspection (no behaviour change between the two on this path).

## Problem and evidence

### Description

The `.husky/commit-msg` adapter runs `rhino gate run --surface commit-msg --message-file "$1"`.
For a merge commit Git supplies `$1` as `<git-dir>/MERGE_MSG` (in a linked worktree,
`<git-dir>/worktrees/<name>/MERGE_MSG`). The gate answers:

```text
rhino: gate `public-safety-commit-message`: message file `.../MERGE_MSG` cannot be read:
the supplied path is not Git's canonical COMMIT_EDITMSG
```

with a configuration-refusal exit (`2`), so `git merge` stops with `Not committing merge` and the
branch cannot be synced locally. The message itself is never scanned; a benign
`Merge branch 'main' into ...` is indistinguishable from a secret-bearing one at this surface.

### Steps to reproduce

1. In any repository declaring the `public-safety-commit-message` gate on the `commit-msg`
   surface, start a merge that does not fast-forward (`git merge origin/main`).
2. Observe the hook failure above; the merge stays staged but uncommitted.
3. Direct probe, no commit involved (from the repository root, `GITDIR=$(git rev-parse --git-dir)`):
   `rhino gate run --surface commit-msg --message-file "$GITDIR/MERGE_MSG"` fails identically,
   as does a `/tmp` copy of the same bytes and a `COMMIT_EDITMSG`-named copy inside the same
   git directory — only the exact canonical path is accepted.

### Expected behaviour

The gate scans the proposed message for every commit kind Git can ask the hook about
(`COMMIT_EDITMSG`, `MERGE_MSG`, `SQUASH_MSG`), or documents that merge commits are out of scope
and says how a gated repository is meant to sync a branch.

### Actual behaviour

`src/runtime/disk.rs` (`hook_message_file`) resolves `git --path-format=absolute --git-path
COMMIT_EDITMSG` and rejects any supplied path that is not byte-identical to it; `src/runtime.rs`
does the same comparison against the in-memory tree's expected path. Both sites emit the
`not Git's canonical COMMIT_EDITMSG` refusal. The integration test at
`tests/integration/policy.rs` enshrines the rejection, so the current suite would fail a fix that
only widens the path check.

### Environment

Consumer on Linux, Git with a linked worktree (also reproducible for plain commits' paths outside
the canonical file — the check is path-shaped, not worktree-shaped). RHINO `v0.15.0` (consumer pin)
and trunk `f655040` carry the same validation.

### Documentation consulted

`rhino gate run --help` (`--message-file`: "Exact commit-msg hook path, read only for that
surface") — "the exact hook path" reads as if whatever Git passes is accepted, which is the
surprise: only one of Git's several hook paths is.

## Why now

A consuming run must sync its delivery branch with `main` before its PR can merge (strict
branch protection). The local sync route is closed by this defect, so every remaining sync on that
run depends on the workaround below until the gate is fixed.

## Prior art

Issue search for `COMMIT_EDITMSG` in this repository returned no open or closed match on
2026-10-25. No duplicate to link.

## Proposed direction

Accept each message-file path Git's `githooks(5)` documents for the `commit-msg` hook
(`COMMIT_EDITMSG`, `MERGE_MSG`, `SQUASH_MSG`, and the `--cleanup` variants' real paths), resolved
the same way the canonical path already is, and scan the bytes the same way. Update the
`policy.rs` integration test to cover a merge-message scan (clean message passes; planted secret
fails) instead of asserting refusal.

## Scope and non-goals

The message scan itself is not in question — only which hook paths reach it. No change to the
secret patterns, no change to any other surface.

## Risks and open questions

- Whether a merge message needs different secret handling than a plain commit message (it
  concatenates conflicted paths and branch names; likely the same patterns apply).
- Whether consumers relying on the refusal (unlikely — it blocks legitimate work) need a
  deprecation note.

## Workaround

Sync the branch server-side (`PUT /repos/{owner}/{repo}/pulls/{n}/update-branch`, i.e. GitHub's
"Update branch" button): the merge commit is created where no local hook runs, and PR CI still
scans the resulting range through the `pull-request` surface bindings. Local `git merge` for sync
stays off the table until this is fixed. No `--no-verify`, no hook bypass.

## Success and promotion signal

A merge commit's message passes the gate when clean and fails it when it carries a planted secret,
on the pin and on trunk, with the `policy.rs` test updated to pin that behaviour.
