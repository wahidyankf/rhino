# Consumer Correctness

When design options differ, choose the one that most helps the consumer be correct.

Consumers cannot read maintainer intent, so ambiguity becomes misuse and misuse becomes incidents. A terse or ambiguous
surface that lets misuse pass quietly is the wrong choice even when it is shorter.

## Requirements

- Name the cause and point to the fix. A diagnostic identifies what went wrong and where, and says what to do about it,
  placed as [errors and diagnostics](../conventions/command-line-interface/006-errors-and-diagnostics.md) requires.
- Keep names exact across surfaces. The word in the finding is the word in the configuration, the error code, and the
  document, so a consumer can match them without guessing, as [naming](../development/code-clarity.md) requires.
- Refuse loudly rather than absorb quietly. Invalid input is refused with a typed error where it arrives, and a check
  that cannot run reports that it could not. Silence is never evidence of success.

## What This Does Not Require

- It does not move advice into the diagnosis line. Errors and diagnostics keeps the fix on a following advice line so a
  machine matching on the message cannot mistake it for the diagnosis.
- It does not name a fix the tool cannot know. Where the remedy depends on which of two correct things the author meant,
  the message states what is wrong and what it depends on, per
  [message discipline](../conventions/plan-validator-contract/002-rule-identifiers.md); naming one fix there would
  mislead.
- It does not end the declared harness-callback exemption. A program meeting all four exemption conditions stays exempt;
  its silence is declared and recorded, not quiet misuse.

## Enforcement

Unenforced by decision. No mechanical check can judge which of two candidate surfaces helps a consumer be correct, so
review applies this principle. Its instances stay covered where they live: diagnostics shape, naming, and boundary
refusal each carry their own gate.

## Related

- [Errors and diagnostics](../conventions/command-line-interface/006-errors-and-diagnostics.md)
- [Code clarity](../development/code-clarity.md)
- [Type and boundary safety](../development/quality/code/type-and-boundary-safety.md)
- [Vision](../vision/README.md)
