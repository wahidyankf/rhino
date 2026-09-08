# Code Clarity

## Phase Separation

Separate setup, validation, decision, mutation, and return phases with blank lines. A reader should be able to see a function's shape before reading its statements.

A formatter is not semantic grouping. `cargo fmt` decides where the line breaks go; it has no opinion about which statements belong together, and a function it has formatted can still be an undifferentiated wall.

## Comments

Comment the non-obvious: a safety invariant, a lifecycle boundary, a decision whose alternative looks equally reasonable, an ordering that matters for a reason the code cannot show. Say why, not what.

Do not narrate line by line. A comment restating the statement below it is noise that ages into a lie the first time the statement changes and the comment does not.

A comment explaining a workaround names what it works around, so the next reader can tell whether it is still needed.

## Naming

Name a thing after what it means to its caller, not after how it is implemented. A validator is named for the rule it enforces, because that is the word in the finding, in the configuration, and in the document a maintainer reads when the finding appears.

## Related

- [Minimal sufficiency](../principles/minimal-sufficiency.md)
