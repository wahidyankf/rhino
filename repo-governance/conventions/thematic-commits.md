# Thematic Commits

One commit carries one theme: a single coherent change with a single reason to exist.

## Requirements

- Split unrelated work into separate commits even when it lands in one pull request. A reader bisecting history should
  never have to untangle two purposes from one commit.
- Keep a change internally complete. Source, its tests, its specification, and its documentation belong in the same
  commit when leaving one of them out would make the tree inconsistent at that commit.
- Follow Conventional Commits. `commitlint` enforces the type and shape; it cannot tell whether the subject is true.
- Keep the message brief. Name the change in the subject. Write the body for someone who has the diff and wants the
  reason: state what was wrong and why this change was needed, including a rejected alternative when it matters.
- Record a failure the change is a response to, including one a gate reported, so the reason survives longer than the
  memory of it.

## Related

- [Pull request boundaries](pull-request-boundaries.md) — where commits become a delivery unit.
- [Data safety](public-repository-data-safety.md) — inspect the message too, not only the diff.
