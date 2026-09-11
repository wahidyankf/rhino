# Plans

Plans are working records, not architecture. A plan says why work exists, what it will change, and what will prove it done. [`specs/`](../specs/README.md) says what the built binary actually does, and when the two disagree the specification is right and the plan is history.

A plan is optional. Most work here is one delivery unit and needs no folder. A plan earns its cost when the work spans several units or several sessions, or when the decision behind it is one a reader will want the reasoning for later. The rule is the [plans convention](../repo-governance/conventions/plans.md) and the local additions in [plan lifecycle](../repo-governance/conventions/plan-lifecycle.md); the procedure is [plan execution](../repo-governance/workflows/plan-execution.md).

## Stages

```mermaid
flowchart LR
    Ideas["ideas/"] --> Backlog["backlog/"]
    Backlog --> Active["in-progress/"]
    Active --> Done["done/"]

    classDef rough fill:#CA9161,stroke:#000000,color:#000000
    classDef formal fill:#0173B2,stroke:#000000,color:#FFFFFF
    classDef record fill:#029E73,stroke:#000000,color:#000000

    class Ideas rough
    class Backlog,Active formal
    class Done record
```

`ideas/` holds rough two-pagers. `backlog/` and `in-progress/` hold complete formal plans, queued and active. `done/` preserves the delivery record under a completion date.

One folder moves through the stages. Move it, never copy it: two stages holding the same plan is two plans, and one of them is lying.

## What This Repository Plans

Only work it can deliver alone. A plan whose delivery would change another repository is planned where that work is coordinated and arrives here as its own change, with its own evidence, like any other rule that came from elsewhere — see [rules propagation](../repo-governance/workflows/rules-propagation.md). A plan that directed edits into a sibling would be exactly the automatic propagation this repository refuses.

## Directory Map

- [Backlog](backlog/README.md) indexes complete plans that are ready but not started.
- [Done](done/README.md) preserves completed plans as dated delivery records.
- [Ideas](ideas/README.md) indexes rough two-pager briefs by urgency and importance.
- [In progress](in-progress/README.md) holds the plans being executed now.
