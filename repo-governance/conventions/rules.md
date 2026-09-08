# Rules

A rule is a repository statement that directs or constrains a decision, behaviour, standard, or procedure within a stated scope. It tells a person or an agent what is required, prohibited, expected, recommended, or permitted.

## Interpretation

- **Must** and **must not** identify mandatory requirements and prohibitions.
- **Should** and **should not** identify expected behaviour; deviating requires a stated reason.
- **May** identifies permission, never an obligation.
- A direct imperative — "run the check" — is mandatory unless its context explicitly makes it optional.

A rule's scope may name people, agents, files, components, or tasks directly, or inherit it from the document containing the rule. Rationale, explanation, and examples support a rule but create no additional rule unless they carry their own direction or constraint.

## Authority

Every rule has exactly one canonical source. Its level and precedence come from that source under the [governance hierarchy](../README.md). A reference at a point of use links to the canonical rule rather than restating it, because a restatement is a second copy that will eventually disagree.

Creating, moving, changing, or removing a rule invokes [rules propagation](../workflows/rules-propagation.md), whether or not it was asked for. A rule change is ready when propagation returns `PASS_NO_CHANGE` or `PASS_CHANGED`.

The read-only [rules quality gate](../workflows/rules-quality-gate.md) and the [rules grooming](../workflows/rules-grooming.md) sweep each run only on explicit direction, and neither writes.

## Related

- [Governance continuity](../principles/governance-continuity.md)
