# Governance Continuity

The rules must survive a coding harness losing its context.

A harness compacts, restarts, or hands off mid-task. When it does, whatever it was holding in memory is gone, and only what is written in the repository remains. A rule that lived only in the conversation is a rule that stops applying at the least convenient moment.

## Requirements

- Every rule has a canonical home in this tree. A rule stated only in a prompt, a commit message, a pull request comment, or a chat is not a rule of this repository.
- Work in progress records enough state to resume: what was decided, what was verified, and what is outstanding. A verification result that cannot be pointed at has to be redone.
- Do not delete or weaken a rule to fit the work in front of you. Changing a rule is its own act, with its own authorization, through [rules propagation](../workflows/rules-propagation.md).
- When context is lost mid-task, re-read the governing documents rather than reconstructing them from memory. Reconstruction produces something plausible, which is worse than knowing you do not know.
- Retain unfamiliar changes under this tree rather than reverting them because they are unrecognized. Unfamiliar is not the same as wrong.

## Related

- [Rules](../conventions/rules.md)
- [Rules propagation](../workflows/rules-propagation.md)
