# Progressive Disclosure

A reader should reach the rule that governs what they are about to do in one hop from where they started, without reading the rules that do not.

## Requirements

- Root `AGENTS.md` names each area and links to it. It states no rule of its own, so there is never a question of which copy is authoritative.
- Each level's `README.md` maps its own directory and says what belongs at that level.
- A document covers one reader task. When two tasks share a document, a reader looking for one of them reads both.
- Link to a rule rather than restating it. A restatement is a second copy that will eventually disagree with the first.
- Depth is bounded by usefulness, not by symmetry. A level with one document is fine if that document has a distinct job.

## Why the Word Budget Exists

The [budget](../conventions/directory-maps.md) is this principle made mechanical. A document that outgrows it has usually acquired a second reader task, and splitting it is the repair. Raising the budget instead hides the symptom and the document keeps growing.

The budget is a maximum and never a target. Padding a document toward it is as much a failure as exceeding it.

## Related

- [Directory maps](../conventions/directory-maps.md)
- [Minimal sufficiency](minimal-sufficiency.md)
