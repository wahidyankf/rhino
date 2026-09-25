# Docs Quality Gate

Run only when someone explicitly names this gate or directs its documentation audit, or when [release cut](release-cut.md) runs it with scope `all`. A change or a [docs propagation](docs-propagation.md) run never authorizes it alone.

Produce one read-only verdict with a finite ledger of stale, obsolete, misplaced, and unreadable documents. This gate never edits a document and never starts another gate run. Docs propagation is the only writer and the mandatory continuation for any finding.

## Inputs and Snapshot

- `scope` — `change`, the documents one change affects, or `all`, the whole document set docs propagation defines.
- `change` — the revision range or working-tree change; required when `scope` is `change`.

Freeze the scope, the Git revision, and the uncommitted paths. A material external change returns `INPUT_CHANGED` with the ledger kept; it never restarts the gate.

## Audit

Under `change`, audit the documents the change touches and every document citing what it changed; under `all`, the whole set. Decide for each document whether:

1. every claim is true to the implementation, and every command shown was run or is marked not exercised, under [truthfulness](../conventions/documentation-architecture.md#truthfulness);
2. it still describes something the repository has; if not, it is obsolete and its resolution is removal;
3. each fact has one home, a summary sits above its detail under [progressive disclosure](../principles/progressive-disclosure.md), and a `docs/` page serves one Diátaxis category under [documentation architecture](../conventions/documentation-architecture.md);
4. a newcomer learns from the opening what it is and why it matters, and finds the next step, judged by reading, never by a score;
5. under `all`, or when setup changed, a reader with no prior context can follow the setup exactly as written from a clean checkout, each step marked smooth, frustrating, or blocking; and
6. it agrees with `specs/`, which is canonical.

## Ledger

Record the ledger under `local-tmp/docs-quality-gate/`. Each row names the document, the gap, the required resolution — update, move, or remove — the evidence, and a status: `OPEN`, `RESOLVED`, `NOT_APPLICABLE` with evidence, or `BLOCKED`. Admit only a document that is wrong, obsolete, unreachable, or unusable by a newcomer; wording preference is not a finding, under [minimal sufficiency](../principles/minimal-sufficiency.md).

Formatting, links, directory maps, and word budgets belong to deterministic checks. Consume their result rather than repeating them:

```sh
./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo xtask self-validate
```

## Results and Handoff

Return `PASS` only when the ledger is clear and the checks pass; otherwise return `NEEDS_PROPAGATION` with the ledger. A finding only the owner can decide, such as a specification that disagrees with the implementation, is asked through [grill-me](../../.agents/skills/grill-me/SKILL.md).

`NEEDS_PROPAGATION` is a handoff, never a blocked result: the caller runs docs propagation with the ledger without another request, then reports propagation's result.

**Recorded choice: verdict only.** The gate does not audit again after propagation, matching the [rules quality gate](rules-quality-gate.md); a second audit needs a second request. The rejected alternative, repairing to zero findings, repeats the audit while open findings strictly decrease and costs an audit per round. A verdict authorizes no commit or push.

## At Release Cut

[Release cut](release-cut.md) runs the gate with scope `all` twice. The first run happens before the release-prep pull request opens, and that pull request resolves its ledger, so each repair's new prose is audited before the merge rather than one run at a time after it. The second run, on the exact commit to tag, is the precondition.

The gate runs the binary, so it is also a behaviour audit. A row where the document states the intended contract and the binary disagrees is a code defect: fix it through its own pull request before the tag, never by documenting the defect as current behaviour.

## Why It Runs on Request

Judging whether a document is still true, still needed, and still readable is a reading task. Wired into every change, it produces noise nobody reads or a pass nobody earned; propagation already refreshes each change.
