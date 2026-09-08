# Plan Specification Changes

Use this convention when a formal plan changes observable behaviour, the command surface, a configuration key, an exit code, or the architecture. It exists to make the specification work concrete enough for a junior engineer to perform before implementation starts, rather than discovering it afterwards.

## Where It Lives

In an unsplit plan, a section of `tech-docs.md` owns the planned specification work. In a split set, use a mapped `tech-docs/specification-changes.md` when that work is a distinct reader's job. Either way, list every affected C4 or Gherkin file by its exact repository-relative path with one label: `[E]` edited, `[N]` new, `[M]` moved, `[D]` deleted.

## What Becomes a Contract

PRD Gherkin is plan-level acceptance language. It is not an automatic request to copy every scenario into [`specs/`](../../specs/README.md). Before the file list, state which PRD outcomes become durable scenarios and which stay plan-only. Give every plan-only outcome its reason and the exact `delivery.md` task that verifies it; give every selected outcome its target specification file below.

Remember what a change here costs a consumer. A command, flag, exit code, or configuration key is [a public contract](../development/public-contract.md): adding is free, and moving one is a major version. A plan that proposes a rename is proposing a release decision, and the plan says so.

## Per File

Use one heading per file with nested bullets rather than a wide table. Put the planned delta in a fenced `diff` block, `-` for current or removed behaviour and `+` for resulting or added behaviour, and keep `= Preserve`, `→ Bindings`, and `✓ Proof` as ordinary bullets beneath it. A long scenario list goes in a collapsed `<details>` block directly below the diff. An `[N]` file uses `+` only, a `[D]` file `-` only, an `[M]` file shows both paths.

For each Gherkin file, state:

- every existing scenario to preserve, update, move, or delete, by name, and the observable behaviour that results;
- every new scenario by name, with its actor, preconditions, action, and expected outcome;
- the exact binding and support file paths that change for it, remembering that every scenario binds at the unit adapter under [behaviour-driven development](../development/behaviour-driven-development.md); and
- the command that proves the changed corpus.

For each C4 file, name the exact view, node, relationship, or constraint that changes and why, under [architecture specifications](../development/architecture-specifications.md).

## Proposed Now, As-Built Later

Keep the proposal in the plan. `specs/` is updated only with the final as-built result, during execution, under [specification maintenance](../development/specification-maintenance.md). The implementation phase in `delivery.md` must carry an `[AI]` task naming that canonical path and the affected elements, whose outcome is synchronized as-built content and whose proof is the architecture and specification gates. Do not defer every C4 update into one generic documentation task at the end; that is how a model drifts from the code it describes.

## File Impact

The plan's File Impact section lists every expected code, test, specification, documentation, and configuration path exactly. A directory, an ellipsis, a glob, or a described "area" is not a path. If an unmade human decision prevents naming a file that the work needs, make that decision a prerequisite and block execution on it rather than hiding it behind a tree.

Group a large tree into short area-specific blocks with aligned, concise annotations. Detailed behaviour belongs in the specification and architecture sections above, not in a horizontal list nobody can read.
