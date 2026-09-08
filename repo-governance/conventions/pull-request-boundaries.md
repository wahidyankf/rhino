# Pull Request Boundaries

One branch, one pull request, one delivery unit. Never open two pull requests from one branch, and never drive one pull request from two branches.

A **delivery unit** is the contiguous run of work ending where what has accumulated is an independently shippable increment. The unit, not the phase or the task, becomes a pull request.

## Where a Pull Request Opens

- At a delivery boundary, not at every phase. Work inside a unit still passes its own checkpoint but opens nothing, and pushing the branch for durability opens nothing either.
- A plan declares its delivery boundaries and its work location — the `worktrees/<name>/` checkout its execution uses — when it is written. The last change-producing phase is always a boundary, or the plan's final work never reaches `main`. A setup phase that produces nothing reviewable is never a boundary; move anything reviewable it acquired into the first real unit.
- Independent work delivers separately. Group only along a dependency chain: folding two independent pieces together to reduce the count re-serializes work that was independent.
- A ready increment is never held back to batch it with later work.

## The Boundary Test

A point is a delivery boundary when all four hold:

- **Coherent** — the increment is a complete unit of meaning: a validator, a schema section, a governance rule. Not half a refactor.
- **Green alone** — every applicable gate passes on the increment by itself.
- **Releasable** — the exact resulting `main` state satisfies the rule below.
- **Reviewable whole** — a reviewer can judge it without reading work that does not exist yet.

Anything failing one is intermediate: a schema field nothing reads, a helper the next step consumes, a fixture the next step asserts on.

## Where to Draw the Seam

Split at a cohesive seam — one independently useful purpose a reviewer can hold in mind, which this repository can build, verify, ship, and roll back without an unmerged sibling. Line and file counts never create or erase a boundary: a large unit is right when its parts must land together, and a small diff carrying two unrelated purposes still splits.

Keep everything the unit needs to stay internally consistent inside it: source, tests, the [specification corpus](../development/specification-maintenance.md), documentation, governance text, configuration, and release support. A directory boundary is not by itself a seam, and unrelated purposes stay apart even when they touch one file.

Units land sequentially in the one worktree the work provisions: land a unit, sync from `origin/main`, then begin the next. Each is reviewed against a base that already contains its dependencies, never stacked on an unmerged sibling.

## Releasable State

`main` is what a release tag is cut from, so merge a unit only when the exact resulting `main` state could be tagged immediately. A new validator, output field, or configuration key that is incomplete must be inert — unreferenced by the schema, unreachable from the CLI — rather than half-wired. Nothing is allowed to land on the promise that the next pull request will make it safe.

Public surfaces bind harder than internal ones. Anything a consumer repository can depend on — the configuration schema, exit codes, output shapes, the released archive layout — lands complete or not at all, because a consumer pins a version and cannot see the pull request that was going to finish it.

## Enforcement

Unenforced by tooling, by decision. No diff-size check can tell whether a seam is natural or a resulting state is releasable. The `Quality gate` check and the [merge preconditions](pull-request-merge.md) supply supporting evidence; the judgement stays with the author, recorded in the [body](pull-request-body.md).
