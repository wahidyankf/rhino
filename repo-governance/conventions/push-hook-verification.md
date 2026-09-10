# Push-Hook Verification

Do not pass `--no-verify` to `git commit` or `git push` unless the user explicitly authorizes bypassing hooks for that specific operation.

## What the Hooks Run

[Public safety](public-repository-data-safety.md) runs first on all three, and every hook sets `-e` so a finding ends the sequence instead of being printed above a successful commit. Its exit `1` is a finding and its exit `2` is a scan it could not trust; both block, and neither has a bypass.

- `commit-msg` screens the message, then runs `commitlint`, enforcing Conventional Commits.
- `pre-commit` screens the staged content and names, then runs `lint-staged`, which formats staged files only.
- `pre-push` screens the tracked tree and the branch name, then runs `cargo xtask test-quick` under the pinned HIPPO guard. Exit `75` means the host was busy: retry that same invocation once the condition clears, never bypass it.

## Requirements

- Treat authorization to push and authorization to bypass hooks as separate permissions. A normal push request does not authorize `--no-verify`.
- Obtain explicit permission that names or clearly includes the bypass, and do not carry it into a later operation or a broader scope.
- When a hook fails, preserve its output, identify the failing check, and reproduce the failure without bypassing verification.
- Trace it to the earliest responsible code, test, configuration, or dependency and fix that. A gate that fails because a file it names was renamed is telling you the rename is incomplete, not that the gate is wrong.
- Never disable, remove, mute, weaken, or superficially satisfy a hook or its checks to make an operation pass.
- Rerun the failed check after the fix, then use a normal verified operation.
- Convenience, time pressure, repeated failure, or difficulty diagnosing the cause never justify a bypass.
- If the root cause cannot be fixed within the authorized scope, report the evidence and the blocker. Ask only under [last-resort questions](last-resort-questions.md).
- Even when a bypass is authorized, disclose which safeguards are skipped and any unresolved failure before proceeding.

## Deliberately Failing Fixtures

Constructing a commit that is _meant_ to fail — to prove a gate refuses it — still requires the bypass permission above, and requires that the fixture live on a throwaway branch which never merges. The bypass manufactures a failure rather than avoiding one, and it is disclosed either way.

## Related

- [Commit authorization](commit-authorization.md)
- [Quality gates](../development/quality-gates.md)
