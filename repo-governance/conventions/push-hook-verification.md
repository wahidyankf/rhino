# Push-Hook Verification

Do not pass `--no-verify` to `git commit` or `git push` unless the user explicitly authorizes bypassing hooks for that
specific operation, or the push takes the [worktree-to-PR exception](#worktree-to-pr-exception) on a task-owned branch
that lands through [worktree to pull request](../workflows/maintenance/worktree-to-pull-request.md).

## What the Hooks Run

[Public safety](public-repository-data-safety.md) runs first on all three, and every hook sets `-e` so a finding ends
the sequence instead of passing silently above a successful commit. Exit `1` is a finding, exit `2` an untrusted scan;
both block, and neither has a bypass.

- `commit-msg` screens the message, then runs `commitlint`, enforcing Conventional Commits.
- `pre-commit` screens the staged content and names, then runs the `format-staged` gate, which runs Prettier on the
  staged Markdown, JSON, and YAML files only.
- `pre-push` screens the tracked tree and the branch name, then each pushed commit's additions, names, and message, then
  runs the rest of its declared surface, which includes the quick gate, `./scripts/test-quick.sh`, under the pinned
  HIPPO guard. It is the mechanical half of the [push review](../workflows/quality/pr-leak-review/002-push-review.md),
  never the whole of it. Exit `124` means a limit stopped the work, and the reason on stderr says which: retry that same
  invocation once the condition clears, or take the [worktree-to-PR exception](#worktree-to-pr-exception) when its
  conditions hold — a hook held back before any check started holds no finding to fix.

## Requirements

- Push authorization and hook-bypass authorization are separate permissions: a normal push request does not authorize
  `--no-verify`.
- Obtain explicit permission naming or clearly including the bypass; never carry it into a later operation or a broader
  scope. The [worktree-to-PR exception](#worktree-to-pr-exception) is the only standing permission; it needs no
  per-instance prompt.
- When a hook fails, preserve its output, identify the failing check, and reproduce the failure without bypassing.
- Trace it to the earliest responsible code, test, configuration, or dependency and fix that: a gate failing on a file
  it names is reporting an incomplete rename, not a wrong gate.
- Never disable, remove, mute, weaken, or superficially satisfy a hook or its checks to make an operation pass.
- Rerun the failed check after the fix, then use a normal verified operation.
- Convenience, time pressure, repeated failure, or difficulty diagnosing the cause never justify a bypass.
- If the root cause cannot be fixed within the authorized scope, report the evidence and the blocker, asking only under
  [last-resort questions](last-resort-questions.md).
- Even with an authorized bypass, disclose skipped safeguards and any unresolved failure before proceeding.

## Worktree-to-PR Exception

The maintainer pre-authorizes `--no-verify` on `git commit` and `git push` for a task-owned branch that lands through
[worktree to pull request](../workflows/maintenance/worktree-to-pull-request.md) with no per-instance prompt, when every
condition below holds. This is the only standing exception to the requirements above, and its single canonical
statement. When the pull-request gate mirrors the skipped hook's deterministic checks, pushing with `--no-verify` and
letting CI run the gate is sound — preferable under HIPPO contention (deferral or failure, exit `124`/`125`
never-started, abnormal latency, or a wait). A mirrored hook need not run locally first: the gate refuses the head when
the hook would have.

- The branch is task-owned with its pull request as the only route to `main`. Direct pushes to `main` stay outside this
  exception: explicit per-instance authorization still applies, strongly discouraged except for manual override.
- No reported finding is skipped: a hook that reported a violation is fixed at its cause. The public-safety screens have
  no bypass — tree, branch, and every pushed commit's additions, names, and message are screened and exit `0`, with no
  push-review finding. The `commit-msg` hook stays enabled.
- Formatting is done by hand or editor; the pull-request gate verifies it.
- The bypass cites the receipt and screen evidence in the push and PR notes, and never carries into a later operation.
- The branch does not merge until the pull-request gate passes on the exact pushed head. A later push replaces that head
  and owes a fresh pass; nothing about this exception waives it.

Enforcement: gated — [`pr-quality-gate.yml`](../../.github/workflows/pr-quality-gate.yml) mirrors the workstation's hook
contracts as merge-blocking checks: a bypassed hook fails in the gate instead, blocking the merge; review verifies the
cited receipt and screen evidence against the gate result.

## Deliberately Failing Fixtures

A commit _meant_ to fail — proving a gate refuses it — still requires the bypass permission above, on a throwaway branch
that never merges. It manufactures a failure rather than avoiding one, disclosed either way.

## Related

- [Commit authorization](commit-authorization.md)
- [Quality gates](../development/quality-gates.md)
