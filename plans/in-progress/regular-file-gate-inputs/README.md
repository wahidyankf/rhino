# Regular File Gate Inputs

Status: In progress. The owner plan is being prepared for its plan-only delivery; implementation has not started.

RHINO needs an explicit way to pass changed regular files to a formatter while retaining tracked compatibility links as
link data. This plan adds an opt-in input and releases it through the existing delivery route. It owns only RHINO;
consumer adoption is coordinated separately.

Start with [the business need](brd.md), then [the required outcomes](prd.md), [the design](tech-docs.md), and
[the executable delivery record](delivery.md). Capture discoveries in [learnings](learnings.md) as they happen.

## Approach and Dependencies

Add `regular-files` beside `files`, derive its selection from Git destination modes, and retain the existing mutation
snapshot protections. A private pure parser keeps metadata decisions inside the existing coverage denominator. The
design adds no dependency. Git and supported contributor tooling are existing prerequisites.

The selected upstream-defect route places the plan-only pull request before the quality gate. Publish the canonical
catalog correction first, land this plan alone, then adopt that published correction through a separate RHINO PR in the
same checkout. Run the plan quality gate after that adoption lands. Implementation starts after the verdict is recorded
and every blocking row has an owner. Feature code, specifications, schema, documentation, and release preparation form
the next coherent delivery unit. Release Cut runs from clean primary `main` after that unit reaches `origin/main`.

## Decision Record

The pre-write inspection examined the typed configuration, gate resolver, runtime ports, disk mutation runner, scenario
adapters, generated schema, release workflow, and owner governance at baseline `c8d49ff`.

The user selected **a new `regular-files` kind** over an optional selector on `files`. This keeps existing consumers'
selection semantics stable and makes narrower formatter input explicit. The user approved the compatible feature and
release design, then requested its implementation. Open alternatives and discussion were available during the decision.

Settled constraints: Git index and immutable explicit ranges only; unchanged `files`; existing `.paths` projections;
destination modes; a private parser; no dependencies; both mutation surfaces skip their child only when the combined
file selection is empty and the gate declares the new kind. Checks retain their existing invocation behavior.

The separate post-write inspection found a conflict between cleanup-before-archival wording and retaining the one
execution checkout until every delivery unit lands. The owner selected **A: remove the checkout after the archival PR
merges**, retaining one checkout through completion and recording final cleanup proof in that PR. The alternative was
cleanup first with a separate archival route; open alternatives and discussion remained available. This preserves the
one-checkout delivery boundary without claiming cleanup before it occurs.

The portable rule correction has landed in its canonical catalog; [the delivery record](delivery.md) identifies the
published source. This plan lands alone with owner adoption explicitly pending; the separate RHINO adoption PR follows
from the same cleanly synced checkout before the quality gate. The adoption revision remains pending. No product
implementation starts before the plan quality checkpoint.

## Directory Map

- [brd.md](brd.md) explains the need, stakeholders, outcomes, and business risks.
- [prd.md](prd.md) defines observable acceptance criteria.
- [tech-docs.md](tech-docs.md) specifies the design, file impact, tests, and specification changes.
- [delivery.md](delivery.md) owns ordering, live progress, evidence, recovery, and delivery.
- [learnings.md](learnings.md) holds discoveries until each has a durable disposition.
