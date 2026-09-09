# Worktree to Pull Request

The procedure for getting a change from nothing to `main`. `main` refuses direct pushes for every actor, so this is the only path.

## Once Per Task

```sh
git -C <repo> worktree add worktrees/<name> -b worktree/<name> origin/main
cd worktrees/<name>
./hippo run --class ephemeral --disk-path . -- npm ci
```

`npm ci` is what installs the hooks. A worktree that skipped it pushes unverified work, and nothing local will say so.

Provision **one** worktree per task and reuse it for every delivery unit the task produces. A second `git worktree add` for the same work is a defect.

## Per Delivery Unit

1. **Sync.** `git fetch origin && git rebase origin/main`. Never auto-stash and never auto-resolve; an unclean tree or a conflict stops the work. When the sync brings in commits, read the whole incoming diff and reconcile the task against it before continuing.
2. **Commit** in [thematic commits](../conventions/thematic-commits.md), with authorization under [commit authorization](../conventions/commit-authorization.md).
3. **Push.** The `pre-push` hook runs the quick gate under the HIPPO guard. Exit `75` means retry that same invocation, never bypass.
4. **Open as a draft.**

   ```sh
   gh pr create --draft --base main --title "<type>: <subject>" --body-file <path>
   ```

   The body carries what [the body convention](../conventions/pull-request-body.md) requires, and is rewritten on every push that moves the head.

5. **Wait.** Poll at most once every three minutes under [GitHub polling](../conventions/github-polling.md). Use the interval for independent work.

   ```sh
   gh pr checks <number>
   ```

6. **Repair at the cause.** A failing job is fixed where it broke, under [push-hook verification](../conventions/push-hook-verification.md). Never weaken the gate.
7. **Ready, then merge.** Flip to ready only when the work is finished, then verify all five [merge preconditions](../conventions/pull-request-merge.md) — including the data-safety review of the exact head — and merge.

   ```sh
   gh pr merge <number> --rebase
   ```

   Rebase keeps history linear, which the ruleset requires.

8. **Next unit** starts at step 1 in the same worktree.

## When the Last Unit Has Landed

Run [dev artifact clean-up](dev-artifact-clean-up.md). It removes the worktree, both copies of the branch, and the build output this work produced, then reconciles the primary checkout.

## Related

- [Integration path](../conventions/integration-path.md)
- [Pull request boundaries](../conventions/pull-request-boundaries.md)
