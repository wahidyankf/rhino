# Pull Request Merge

Merging is an external, irreversible action on the trunk. Its authority comes from the preconditions below rather than from a prompt at the moment of merging. Once all of them hold, merging proceeds by default; a human gate applies only where a plan's own step says so. Preconditions are evaluated per merge, and satisfying them for one pull request says nothing about the next. This authority covers the merge alone: the commits and pushes that built the branch stay separately authorized under [commit authorization](commit-authorization.md).

## Preconditions

All five must hold at the moment of merge:

- **Exact-head quality gate.** The `Quality gate` check from [`pr-quality-gate.yml`](../../.github/workflows/pr-quality-gate.yml) is green for the pull request's current head SHA against its current base. A run against an earlier head or a different base is stale evidence and authorizes nothing.
- **Posted leak review of that same head.** One [leak review](../workflows/pr-leak-review.md) is posted on the pull request, names the head it read, and reports `pass` against the [data-safety convention](public-repository-data-safety.md). Pin the head SHA before reading. A push that moves the head voids the result; review the new head once rather than accumulating a streak of clean runs. Report a finding by category, location, and remediation, never by repeating the value. An inspection nobody posted is not this precondition.
- **Branch currency.** The branch is current with `main`, brought forward by rebase, and GitHub reports no conflict.
- **Conversations.** Every review conversation is resolved, or dismissed by the user.
- **Surface gates.** Every gate the changed behaviour requires has a passing terminal result. When no reachable behaviour changed, say so explicitly rather than leaving the question open.

## What Safety Means Here

A pull request is safe when two of those hold: its diff leaks nothing, and the `Quality gate` check passes on its exact head. Nothing else stands between a change and the trunk. The ruleset requires no approving review, because the sole maintainer cannot approve their own pull request and requiring one would block every merge. No human reads every line before merge, and these preconditions stand in for the reviewer. The other three are integration hygiene, not the safety claim.

This is why the testing practices carry weight rather than ceremony. Whatever the gate does not exercise, nothing checks. [Test-driven development](../development/test-driven-development.md), the [specification corpus](../development/specification-maintenance.md), the unit coverage floor, and the scheduled integration and end-to-end adapters are what give the gate something to fail on. Weakening any of them does not merely lower quality; it silently widens what this convention is willing to call safe.

`main` is also the only branch a release tag may be cut from, so a merge here is the last point at which a defect is cheap. After a tag is published it is never replaced.

## Draft Lifecycle

Open every pull request as a draft with `gh pr create --draft`. Iterate on the branch while it stays a draft, driving the exact-head gate green. Flip it to ready only when the work meets its done definition. Readiness states that the work is finished; it is not a merge authorization and satisfies no precondition.

## Findings and Bypass

A suspected exposure stops merge handling: contain and rotate the credential, then follow the [data-safety convention](public-repository-data-safety.md) for history already written. A green gate, a resolved conversation, or an earlier clean review never authorizes merging a contaminated pull request.

Never bypass a failing or pending gate, an unresolved conversation, or the ruleset. A user may waive a named gate for a named merge; that waiver covers nothing else, and it never covers the leak review. Repairing a gate failure follows [push-hook verification](push-hook-verification.md): fix the cause.
