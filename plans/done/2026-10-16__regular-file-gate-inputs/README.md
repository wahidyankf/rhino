# Regular File Gate Inputs

Status: Completed.

Completed: 2026-10-16 (Asia/Jakarta).

Substantive delivery is complete: the plan, governance adoption, and feature are merged, and `v0.13.0` is published and
verified. C0 acceptance reconciliation and C1 learning dispositions are recorded. The Plan Quality Gate remains
terminally blocked because structural tooling is unavailable; it found no semantic issues, and its unmet structural
proof remains owned by the Q0 gate caller under the advisory contract. T0 returned no-change. C3's eligible cleanup and
prepared archival PR body are complete. C2 permits archival with zero blocking findings. This dated archive records
substantive completion and readiness; C4's final merge/reconciliation proof and C5's actual cleanup proof remain
assigned to the archival PR body after merge.

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

The portable rule correction landed in its canonical catalog, and this plan, the separate RHINO governance adoption, and
the feature delivery have merged; [the delivery record](delivery.md) records their revisions and evidence. The Plan
Quality Gate returned `BLOCKED` because its structural validator is unavailable, after finding no semantic issues. The
Q0 gate caller owns the unmet structural proof. Under the existing advisory contract, the approved work continued at T0,
which returned no-change. T1–T5, D0/D1, V0/V1, V2, and R0–R4 are complete. Release verification confirms `v0.13.0`
matches its commit and assets. C0 acceptance reconciliation, C1 learning dispositions, and C3's eligible cleanup and
prepared PR procedure are recorded. C2 permits archival. This change prepares C4's dated archive; the archival PR body
records its final delivery proof and C5's actual cleanup after merge, without another archived-plan edit.

## Directory Map

- [brd.md](brd.md) explains the need, stakeholders, outcomes, and business risks.
- [prd.md](prd.md) defines observable acceptance criteria.
- [tech-docs.md](tech-docs.md) specifies the design, file impact, tests, and specification changes.
- [delivery.md](delivery.md) owns ordering, live progress, evidence, recovery, and delivery.
- [learnings.md](learnings.md) holds discoveries until each has a durable disposition.
