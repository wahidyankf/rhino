# Push-Hook Verification

Do not pass `--no-verify` to `git commit` or `git push` unless the user explicitly authorizes bypassing hooks for that
specific operation, or the push takes the [worktree-to-PR exception](#worktree-to-pr-exception) on a task-owned branch
that lands through [worktree to pull request](../workflows/maintenance/worktree-to-pull-request.md).

## What the Hooks Run

[Public safety](public-repository-data-safety.md) runs first on all three, and every hook sets `-e` so a finding ends
the sequence instead of being printed above a successful commit. Its exit `1` is a finding and its exit `2` is a scan it
could not trust; both block, and neither has a bypass.

- `commit-msg` screens the message, then runs `commitlint`, enforcing Conventional Commits.
- `pre-commit` screens the staged content and names, then runs `lint-staged`, which formats staged files only.
- `pre-push` screens the tracked tree and the branch name, then each pushed commit's additions, names, and message, then
  runs `cargo xtask test-quick` under the pinned HIPPO guard. It is the mechanical half of the
  [push review](../workflows/quality/pr-leak-review/002-push-review.md), never the whole of it. Exit `124` means a limit
  stopped the work, and the reason on stderr says which: retry that same invocation once the condition clears, unless it
  stopped before any check started and takes the one-push exception in
  [Capacity-Held Hook](push-hook-verification/001-capacity-held-hook.md).

## Requirements

- Treat authorization to push and authorization to bypass hooks as separate permissions. A normal push request does not
  authorize `--no-verify`.
- Obtain explicit permission that names or clearly includes the bypass, and do not carry it into a later operation or a
  broader scope. The [worktree-to-PR exception](#worktree-to-pr-exception) is the only standing permission; it needs no
  per-instance prompt.
- When a hook fails, preserve its output, identify the failing check, and reproduce the failure without bypassing
  verification.
- Trace it to the earliest responsible code, test, configuration, or dependency and fix that. A gate that fails because
  a file it names was renamed is telling you the rename is incomplete, not that the gate is wrong.
- Never disable, remove, mute, weaken, or superficially satisfy a hook or its checks to make an operation pass.
- Rerun the failed check after the fix, then use a normal verified operation.
- Convenience, time pressure, repeated failure, or difficulty diagnosing the cause never justify a bypass.
- If the root cause cannot be fixed within the authorized scope, report the evidence and the blocker. Ask only under
  [last-resort questions](last-resort-questions.md).
- Even when a bypass is authorized, disclose which safeguards are skipped and any unresolved failure before proceeding.

## Worktree-to-PR Exception

The maintainer pre-authorizes `--no-verify` on `git commit` and `git push` for a task-owned branch that lands through
[worktree to pull request](../workflows/maintenance/worktree-to-pull-request.md), with no per-instance prompt, when
every condition below holds. This is the only standing exception to the requirements above. It supersedes the earlier
HIPPO-congestion-only pre-authorization: a hook whose contract the pull-request gate mirrors does not need to run
locally first, because the gate refuses the head when the hook would have.

- The branch is a task-owned worktree branch whose only route to `main` is its pull request. Direct pushes to `main`
  stay outside this exception: explicit per-instance authorization still applies, strongly discouraged except for manual
  override.
- No reported finding is skipped: a hook that ran and reported a violation is fixed at its cause. The public-safety
  screens have no bypass — the tree and branch screens plus each pushed commit's additions, names, and message are
  screened and exit `0`, with no push-review finding. The `commit-msg` hook stays enabled.
- Formatting is done by hand or editor; the pull-request gate verifies it.
- The bypass cites the screen evidence in the push and PR notes, and never carries into a later operation.
- The branch does not merge until the pull-request gate passes on the exact pushed head. A later push replaces that head
  and owes a fresh pass; nothing about this exception waives it.

Enforcement: gated — [`pr-quality-gate.yml`](../../.github/workflows/pr-quality-gate.yml) mirrors the workstation's hook
contracts as merge-blocking checks, so a bypassed hook moves its failure into the gate, where it blocks the merge;
review verifies the cited screen evidence against the gate result.

## Deliberately Failing Fixtures

Constructing a commit that is _meant_ to fail — to prove a gate refuses it — still requires the bypass permission above,
and requires that the fixture live on a throwaway branch which never merges. The bypass manufactures a failure rather
than avoiding one, and it is disclosed either way.

## Related

- [Commit authorization](commit-authorization.md)
- [Quality gates](../development/quality-gates.md)
