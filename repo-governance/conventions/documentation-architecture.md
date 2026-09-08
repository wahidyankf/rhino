# Documentation Architecture

`docs/` holds documentation about using and understanding RHINO. `repo-governance/` holds the rules for working in this repository. A document belongs to exactly one of them, and the test is the reader: someone consuming the tool, or someone changing this repository.

Do not copy governance, the README, or specifications into `docs/` to populate it. Link to the canonical document instead.

Markdown under `docs/` is exempt from the [governance word budget](directory-maps.md#word-budget). Reference pages are sized by the surface they describe.

## Diátaxis Categories

Organize `docs/` by the [Diátaxis framework](https://diataxis.fr/):

- `tutorials/` — learning-oriented lessons that carry a newcomer through one successful experience.
- `how-to-guides/` — goal-oriented directions for completing a practical task.
- `reference/` — information-oriented description, accurate and consultable during work.
- `explanation/` — understanding-oriented discussion of context, reasons, and design.

Classify by the reader need a document primarily serves. When material spans categories, keep one primary document in the best-fitting one and link to the others rather than blending them. Each category `README.md` states its purpose and indexes the documents directly inside it.

## Truthfulness

Every command and transcript shown in `docs/` must have been executed against the current build. A page that contradicts the [specification corpus](../development/specification-maintenance.md) is a defect in the page, not a difference of opinion — the corpus is what the tool is specified to do.

## Directory Maps

Every directory under `docs/` carries a `README.md` with a `## Directory Map`, under the [directory-map convention](directory-maps.md). The same validator enforces documentation and governance maps together.
