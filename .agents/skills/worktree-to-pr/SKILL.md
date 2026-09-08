---
name: worktree-to-pr
description: Take a change from a task worktree to merged on main through a pull request, the only path this repository's ruleset allows.
---

# Worktree to Pull Request

`main` refuses direct pushes for every actor, with no bypass. This is the procedure that gets a change there. The rules behind each step live in [the workflow](../../../repo-governance/workflows/worktree-to-pull-request.md); this file is the sequence.

## Once per task

```sh
git -C <repo> worktree add worktrees/<name> -b worktree/<name> origin/main
cd worktrees/<name>
./hippo run --class ephemeral --disk-path . -- npm ci
```

`npm ci` installs the hooks. Skip it and the worktree pushes unverified work with nothing local saying so.

One worktree per task, reused for every delivery unit. A second `git worktree add` for the same task is a defect.

## Per delivery unit

1. `git fetch origin && git rebase origin/main`. Never auto-stash, never auto-resolve. Read the whole incoming diff when the sync brings commits in.
2. Commit thematically, with authorization.
3. `git push`. The `pre-push` hook runs `cargo xtask test-quick` under the HIPPO guard. Exit `75` means retry the same invocation, never bypass.
4. `gh pr create --draft --base main --title "<type>: <subject>" --body-file <path>`.
5. `gh pr checks <number>` — at most once every three minutes. Use the interval for independent work.
6. Repair a failing job at its cause. Never weaken the gate.
7. `gh pr ready <number>` **re-triggers the whole gate** — the workflow listens for `ready_for_review`. Wait for that run, not the previous one.
8. Verify the five merge preconditions, including the data-safety review of the exact head, then `gh pr merge <number> --rebase`.
9. Next unit starts at step 1, in the same worktree.

## When the last unit has landed

Confirm nothing is unpushed and nothing is running, then remove the worktree, delete the local branch, and delete it on `origin`. Keep a worktree whose run failed, and say so.

## Refusals

- Never `git push` to `main`, and never `--no-verify` without explicit permission for that operation.
- Never merge on a stale gate run, an earlier head, or a different base.
- Never open a second pull request from one branch.
