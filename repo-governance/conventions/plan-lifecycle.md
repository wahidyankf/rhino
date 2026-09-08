# Plan Lifecycle

A plan is a working record for proposed or delivered work. It lives in exactly one stage under [`plans/`](../../plans/README.md), and it moves through them: `ideas/`, `backlogs/`, `in-progress/`, `done/`.

Plans propose. [`specs/`](../../specs/README.md) is as-built truth, and execution updates every affected specification alongside the implementation under [specification maintenance](../development/specification-maintenance.md).

## Authorization

Writing a plan into this repository requires an explicit user request. Designing an approach in conversation, or in a harness planning mode, authorizes neither the folder nor the commit. See [commit authorization](commit-authorization.md).

## Scope

This repository plans only work it can deliver alone. Work that spans repositories is planned where it is coordinated and arrives here as its own change with its own evidence — [rules propagation](../workflows/rules-propagation.md) explains why a rule, or a plan, that crosses a boundary must be decided again on the other side.

## Ideas

Store a rough two-pager at `plans/ideas/<quadrant>/<slug>.md`, choosing q1–q4 from dated evidence of urgency and importance. Search first and consolidate overlap. Exclude file-level design, Gherkin, and delivery checklists; those belong to a formal plan.

## Formal Plans

Use `plans/backlogs/<slug>/` when queued, `plans/in-progress/<slug>/` when active, and `plans/done/YYYY-MM-DD__<slug>/` when complete, all kebab-case. Every folder contains:

- `README.md` — status, context, scope, approach, dependencies, navigation;
- `brd.md` — goal, roles, outcomes, non-goals, risks;
- `prd.md` — personas, stories, Gherkin acceptance criteria, scope, risks;
- `delivery.md` — ordered tasks, ownership, proof, checkpoints;
- `learnings.md` — approach and transient observations; and
- exactly one technical shape.

That shape is a single `tech-docs.md`, or `tech-docs/README.md` with mapped companions. Keep one document while it stays coherent; split when distinct responsibilities each deserve their own reading order; collapse a fragment with no distinct job. Never keep both shapes, and never pre-create an empty companion. Length is a review signal, never a requirement. Follow [minimal sufficiency](../principles/minimal-sufficiency.md).

Split companions carry a two-digit reading-order prefix such as `01-config-schema.md`, with `README.md` first. Renumber on insertion and order every map by number.

Plans carry no word limit, but they obey [directory maps](directory-maps.md), [Mermaid](markdown-visualizations.md), and [data safety](public-repository-data-safety.md). Write for a junior. Name every affected path exactly, labelled `[E]` edited, `[N]` new, `[M]` moved, or `[D]` deleted; a directory or a glob is not a path.

PRD Gherkin accepts the plan rather than the corpus. Which of it becomes a durable scenario is decided under [plan specification changes](plan-specification-changes.md).

## Delivery Ownership

Every executable checkbox carries its acceptance labels and one owner: `[AI]` for work inside available authority and tools, `[HUMAN]` only for a decision, credential, physical action, or external authority no agent has. Prefer `[AI]`; never use `[HUMAN]` to defer a settled decision or postpone discovery. Split a mixed task. Each task names its input, action, outcome, and proof. End every phase with a blocking checkpoint.

A checkbox that ships code expresses its [red-green-refactor](../workflows/red-green-refactor.md) cycle as three separate RED, GREEN, and REFACTOR checkboxes, each naming the exact test path, the command, and the expected failure or pass. Never combine the cycle into one checkbox or into prose.

Give every recovery task an explicit trigger and leave it dormant until it fires; at reconciliation it receives a dated, evidenced `Not triggered` disposition rather than a checkmark.

When execution may change a repository rule, `delivery.md` carries an `[AI]` task applying [rules propagation](../workflows/rules-propagation.md) and recording its terminal result, which may be `PASS_NO_CHANGE`.

## Transitions

Move a folder, never copy it; update status and both stage indexes in the same change. Refuse an existing dated destination outright. Archive only after acceptance, verification, learnings, and conditional items are reconciled, then run the repository gate. [Plan execution](../workflows/plan-execution.md) owns the procedure.
