# PR Leak Review

A **leak** is anything in outbound history that a reader of the remote could use to reach an environment or identify the
machine it came from. [Leak Classes](pr-leak-review/001-leak-classes.md) defines the three classes and what is not one.
History is the subject, not the final tree: a value added and later deleted remains in every clone. The review binds
from adoption onward; earlier published history is out of scope.

Adopted from the shared catalog at `f452d6190911f29f5898ecb2e991c89412afa426`, superseding the earlier local rule that
whoever handled the push or the merge performed the review. A fresh-context subagent judges; the harness posts its
verdict as the repository owner, whose records alone count. Self-judged verdicts satisfy nothing, and no human is on the
merge path. [Data safety](../../conventions/public-repository-data-safety.md) stays the stricter owner of what may be
committed at all, and the shared marker name keeps earlier records valid.

## Entry

Two entries share one judgement:

- **Push.** Before any push to `origin`, review the outgoing range privately per
  [Push Review](pr-leak-review/002-push-review.md): nothing is posted, and a finding blocks the push.
- **Merge.** A pull request is open, and no `pass` record from the repository owner exists for its current head.
  `pull-request` (`string`, required): its number or address. It is a
  [merge precondition](../../conventions/pull-request-merge.md): no posted `pass`, no merge.

## Sequence

1. **Pin the head.** The harness resolves the pull request through the GitHub API and records the repository, base
   branch and revision, and head revision. Later steps concern that head alone.
2. **Spawn a fresh-context reviewer.** One subagent for this head alone: no part in authoring the change, no memory of
   an earlier head, read-only, never posting. It receives only the pin, title and body, and the leak classes. A failed
   spawn ends as `failed`; no self-judging fallback, no human substitutes.
3. **Read every commit at that head.** The subagent reads each base-to-head diff — configuration, generated files,
   binary metadata, file names, messages — plus the title and body. A summary or memory is not a reading, and no file is
   skipped because another gate covers it.
4. **Judge candidates against the three [leak classes](pr-leak-review/001-leak-classes.md) and no others.** A candidate
   is a finding only when shape and context show the value is real. Never copy one into notes, commands, or logs.
5. **Return each finding without its value.** The subagent returns the result, the count per class, and each finding's
   class, commit, location, reason, and [remediation](pr-leak-review/002-push-review.md#remediation): never the value,
   nothing else.
6. **Confirm the head before posting.** If the live head differs from the pin, post nothing and end as `stale`.
7. **Post exactly one `COMMENT` review on the pinned head, whatever the result.** The harness posts the subagent's
   verdict unchanged, on behalf of the repository owner, stating all other concerns were out of scope, and carrying this
   record:

   ```text
   <!-- ose-pr-leak-review:v1
   {"repository":"<owner>/<repository>","pull_request":"<number>","base_ref":"<base-branch>",
    "base_sha":"<base-revision>","head_sha":"<reviewed-revision>","result":"pass|findings",
    "counts":{"secret_or_private_value":0,"protected_environment_property":0,
    "machine_specific_absolute_path":0}}
   -->
   ```

8. **Read the review back.** Through the API, confirm the posted review's commit equals the pinned head and its
   repository, pull request, base, head, result, and counts match step 7. Other marker-shaped text is not evidence.
9. **Query the live head once more.** A moved head ends the run as `stale`, its evidence bound to the reviewed head.
   Otherwise wait for the required `leak-review` status on that head to read `success`.

## Exit

Success leaves `result` (`enum`: `pass`, `findings`, `stale`, `failed`) at `pass` or `findings`, with `reviewed-head`
(`string`), `review-id` (`string`), and `counts` (`string`) per class. `pass` means every count is zero; `findings`, any
nonzero count. Only `pass` for the merging head satisfies the precondition.

`stale` means the head moved and authorizes nothing new. `failed` means a spawn, API, posting, read-back, or
authentication error, or an unreadable pin, left no verdict. Neither retries inside the run.

## Example Usage

```text
Run pr-leak-review for the current head of pull request 412.
Run the push leak review for this branch's outgoing commits.
```

## One Head, One Review

A moved head needs one new review from a new subagent. Earlier passes say nothing about the merging head, so the run
neither retries nor waits. An unposted pass counts as no review, so step 7 posts every result.
[Enforcement](pr-leak-review/003-enforcement.md) makes the precondition mechanical.

## Related Workflows

- [Worktree to pull request](../maintenance/worktree-to-pull-request.md) runs the push review before each push and this
  review before each merge.
