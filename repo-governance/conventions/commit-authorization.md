# Commit Authorization

Commit and push only when the user has authorized it, when an approved plan step authorizes it, or when an adopted
[upstream tool defects](../development/upstream-tool-defects.md) standard lands a defect's brief or bug-fix plan.

## Ongoing Task Delivery

A single-action grant covers that action once. Authorization to deliver a named task covers its staging, thematic
commits, synchronization, publication, and landing until that task is finished. Do not request permission again for
those routine steps, including `--no-verify` on the task branch under the
[worktree-to-PR exception](push-hook-verification.md#worktree-to-pr-exception);
[authorized task sync](no-destructive-git-operations/001-authorized-task-branch-sync.md) owns the rebase and lease-push
safeguards. Resuming the same unfinished task retains its recorded authority after scope and current state are verified.
It covers no unrelated work, later task, or release; hook bypass only through the
[worktree-to-PR exception](push-hook-verification.md#worktree-to-pr-exception).

## Requirements

- Authorization to make a change is not authorization to commit it. Authorization to commit is not authorization to
  push.
- Authorization does not cross a repository boundary. Permission to commit here says nothing about any other repository,
  including one this repository pins.
- Authorization does not persist beyond the scope it was given for. A grant for one change does not cover the next.
- Merging a pull request is authorized separately, by the [pull-request merge convention](pull-request-merge.md) rather
  than by a prompt at the moment of merging.
- Bypassing a hook is a third, separate permission. See [push-hook verification](push-hook-verification.md); a normal
  push request never authorizes `--no-verify`, except through its
  [worktree-to-PR exception](push-hook-verification.md#worktree-to-pr-exception).
- Publishing a release is separate again, and follows [release cut](../workflows/maintenance/release-cut.md). An adopted
  [upstream tool defects](../development/upstream-tool-defects.md) standard, this repository's or a consumer's,
  authorizes releasing a merged RHINO defect fix.

Choosing how to integrate a change is not authorization either: the [integration path](integration-path.md) says where a
change goes, not whether it may go.

## Related

- [Integration path](integration-path.md)
- [Push-hook verification](push-hook-verification.md)
