# Behaviour-Driven Development

The corpus in `specs/behaviours/` is the specification. The adapters execute it. Neither is a summary of the other, and where they disagree the corpus is what the tool is specified to do.

## Writing a Scenario

- State a behaviour a consumer could observe, in the vocabulary a consumer uses. "The validator returns `Err`" is an implementation. "A repository that declares nothing is refused with the invocation error code" is a behaviour.
- One behaviour per scenario. A scenario asserting three things fails without saying which.
- `Given` establishes the world, `When` performs exactly one action, `Then` asserts the observable outcome. Do not act in `Then` or assert in `Given`.
- Prefer a `Scenario Outline` over near-duplicate scenarios only when the examples differ in data, not in meaning.
- Write the scenario so it stays true if the implementation is rewritten. That is the test of whether it describes behaviour.

## Bindings

A binding maps one step to code that actually performs it. It may not:

- assert a value it was handed rather than one the subject produced;
- return early on a condition the scenario does not mention;
- be a stub, a `todo`, or an empty body left to be filled in; or
- silently pass when the subject was never invoked.

A step that is genuinely shared belongs in the shared step registry once, not copied per adapter. A step that looks shared but means something different per adapter is two steps with one name, which is worse than duplication.

## The Three Adapters

- **Unit** — in-process, against an in-memory tree. Every scenario binds here.
- **Integration** — a real temporary filesystem, built and removed per case.
- **End-to-end** — a spawned process, observing only argv, exit code, and streams.

## Related

- [Specification maintenance](specification-maintenance.md)
- [End-to-end testing](end-to-end-testing.md)
