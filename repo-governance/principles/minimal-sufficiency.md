# Minimal Sufficiency

Understand before adding, reuse before writing, and stop at the smallest change that is verified.

## Requirements

- Read the existing implementation, specification, and rule before proposing a new one. A change built on a guess about how something works is a change that has to be made twice.
- Prefer extending or correcting what exists to introducing a parallel mechanism. Two mechanisms for one job is a defect regardless of how good either is.
- Stop when the change is verified and complete. Speculative generality, unused configuration, and hardening for a case nobody has is cost with no owner.
- A change is complete when its own proof passes, not when it feels finished.

## Dependencies

A new dependency needs four things stated before it is added: the need it meets, the alternatives rejected and why, evidence that it is maintained and that its defects get answered, and an owned consequence — who deals with it when it breaks or is abandoned.

`cargo deny` gates advisories, licences, and duplicate sources, but it cannot tell whether a dependency should exist. That judgement is this rule's.

## What This Is Not

It is not an argument for small diffs. A change is minimal when nothing in it is unnecessary, not when it is short. A large change whose parts must land together is minimal; a one-line change that adds an unused option is not.

It is also not a reason to leave a known defect unfixed inside work that touches it. Minimality bounds scope creep, not correctness.

## Related

- [Progressive disclosure](progressive-disclosure.md)
- [Dependency selection](../development/dependency-selection.md)
