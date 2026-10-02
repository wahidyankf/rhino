# Push Review

The merge review guards what lands on `main`, but a pushed branch is already on the remote. Every push is therefore
reviewed before it leaves, privately, with nothing posted.

## Range

For each ref the push updates, the range runs from the remote's current tip of that ref to the local head. A ref the
remote lacks starts from `origin/main`, the same fallback the `pre-push` range gate declares.

## Sequence

1. **List the range's commits.** Every commit in the range, oldest first, including merges.
2. **Run the repository's screen on the range.** The `pre-push` hook runs it commit by commit per
   [Enforcement](003-enforcement.md); a scan error blocks exactly as a finding does.
3. **Read every commit.** Each commit's added lines, its file names, its message, and the ref name. A merge contributes
   what it resolved beyond the automatic merge.
4. **Judge against the [leak classes](001-leak-classes.md).** No candidate is copied into notes, commands, or logs.
5. **Decide.** No finding: push. Any finding: do not push; remediate, then start again from step 1.

The hook passing is step 2, not the review. A clean screen says no shape matched; only reading says nothing real is
there.

## Remediation

Before the push, the fix is to the history, not the tree. A later commit that deletes the value does not pass, because
the commit that added it would still be published. Rewrite the unpushed commits so that none carries the value: amend
the latest commit, or rebuild the range without it.

| Class                            | Replace the value with                                                      |
| -------------------------------- | --------------------------------------------------------------------------- |
| `secret_or_private_value`        | a reference to environment or secret storage; rotate it if it left the host |
| `protected_environment_property` | an environment variable declared in the committed template                  |
| `machine_specific_absolute_path` | a repository-relative path, a documented placeholder, or a `~/` path        |

After the push, the value is disclosed. Stop, rotate any credential, and report it to the repository owner. Rewriting
published history requires the owner's explicit approval under
[no destructive Git operations](../../../conventions/no-destructive-git-operations.md); correcting the tree alone is
never the whole remedy.
