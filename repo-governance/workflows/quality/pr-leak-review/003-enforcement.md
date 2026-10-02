# Enforcement

A precondition stated only in prose is a convention someone forgets. The leak review is enforced in three layers; each
fails closed.

## The Screen

[`scripts/public-safety/`](../../../../scripts/public-safety/README.md) runs at the `pre-push` hook and on every
pull-request range. Its `public-safety-range` gate in [`repo-config.yml`](../../../../repo-config.yml) screens the range
commit by commit: each commit's added lines at the line numbers they occupy, its file names, and its message. A merge
contributes what it resolved beyond the automatic merge. Content the range did not add is not screened again, so the
screen binds from adoption onward. The `public-safety-tree` gate still screens the whole tracked tree and the branch
name beside it. Its shapes include absolute home paths, private addresses, and internal hostnames, beside a credential
scanner.

At `pre-push` the range comes from the ref updates Git hands the hook, read through `--push-updates-stdin`; a ref the
remote lacks starts from `refs/remotes/origin/main`. On a pull request it is the review range the replay is given.

The screen matches shapes; the review reads context. Neither replaces the other.

## Hosted Checks

- **Range screen.** The `Quality gate` check from
  [`pr-quality-gate.yml`](../../../../.github/workflows/pr-quality-gate.yml) replays the pull-request surface, including
  the range gate, over the pull request's base-to-head range, because a local hook can be skipped and a hosted check
  cannot.
- **Record check.** [`leak-review.yml`](../../../../.github/workflows/leak-review.yml) runs
  [`record-status.sh`](../../../../scripts/leak-review/README.md) when the pull request changes and when a review is
  submitted. It publishes the `leak-review` commit status on the live head, `success` only when the repository owner's
  latest undismissed review on that head carries a `pass` record naming this repository, the pull request, and the head.
  Posting the record turns it green without a new commit.

Both are required status checks on `main` through its ruleset. A waiver of other gates never covers either.

## This Repository's Decisions

Recorded once; the marker never changes afterward.

| Decision          | Here                                            |
| ----------------- | ----------------------------------------------- |
| Record marker     | `ose-pr-leak-review`                            |
| Reviewer identity | the repository owner                            |
| Required checks   | `Quality gate` (range screen) and `leak-review` |
| Integration path  | pull request; `main` refuses every direct push  |
