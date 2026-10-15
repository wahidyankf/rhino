# Execution Check

## Entry

Every substantive item in `delivery.md` is terminal. Archival has not started. Post-delivery cleanup may remain
explicitly `pending` under
[Dependency Graph and Recovery](../../conventions/plans/011-dependency-graph-and-recovery.md).

## Sequence

The evaluation runs in this fixed order. The order matters: each step assumes the previous one held, and reordering
produces findings that contradict each other.

1. **Scope.** Did the executed work match the plan's stated scope — nothing quietly added, nothing quietly dropped?
2. **Requirements.** Is every acceptance criterion terminal, and does its evidence actually establish it?
3. **Checklist evidence.** Does every completed item carry a result, and does the result match what the repository now
   contains?
4. **Gates.** Did every declared gate run and return a terminal result, including the ones that returned findings?
5. **Cleanup.** Is cleanup due before archival complete and proven? Are later removals recorded as `pending` in
   `delivery.md` until every delivery unit using the checkout, including archival, has landed?
6. **Knowledge capture.** Is every `learnings.md` entry resolved to a durable owner or discarded with a reason?

## Exit

One archive-readiness verdict is recorded. Archival proceeds only on a verdict that permits it. It does not claim
post-delivery cleanup is complete; the task remains open until actual cleanup and proof are recorded outside
`plans/done/` in the final delivery record (the archival PR body for this route).

## Archival Is Blocked, Not Warned

An unresolved acceptance criterion, unproven cleanup due before archival, or unrouted learning blocks archival. Required
post-delivery cleanup is `pending`, never `met`, and does not block archive readiness. It remains a task obligation
until actual cleanup and proof are recorded in the final delivery record.

This is strict by design. A plan in `plans/done/` records completed substantive execution and archive readiness.
Explicitly pending post-delivery cleanup remains visible in the archived checklist, and the final delivery record must
prove it before the task is complete.

Resolve a blocking item and re-run the check. There is no partial archival and no archival with an unresolved acceptance
criterion, pre-archive cleanup, or unrouted learning.
