# Specification Maintenance

`specs/` is canonical and lives at the repository root, not under a project directory: this repository is the tool, so there is no second project to disambiguate from.

- `specs/behaviours/` holds the Gherkin corpus, one `.feature` file per validator or contract.
- `specs/architecture.md` holds the C4 model. See [architecture specifications](architecture-specifications.md).

## Before Every Change

Assess both surfaces for impact before writing anything. Record a verified no-op rather than churning an unaffected specification: a scenario edited because a change was nearby is a scenario whose history no longer means anything.

"Assess" means read the scenarios that could plausibly be affected and say which and why not — not glance at the file list.

## The Order

1. Write or change the Gherkin scenario first.
2. Bind it at the unit adapter.
3. Run it and prove it fails **for the stated reason** — the behaviour is absent, not the binding is missing or the fixture is wrong. A red for the wrong reason proves nothing about the change you are about to make.
4. Write production code until it passes.
5. Bind the remaining applicable adapters, or record an exemption that meets the bar below.

## Binding Rules

Every scenario binds at the unit adapter. **There is no unit exemption.**

An integration or end-to-end exemption must name the concrete boundary that cannot be reached from that layer, and the alternative proof that covers it. Difficulty, runtime, flakiness, and cost are never boundaries; they are reasons to write the test differently.

A binding may not be a placeholder, a no-op, or an outcome table that asserts the value it was handed. The static behaviour check asserts that every scenario is bound or validly exempt, and that no binding is unused, ambiguous, or duplicated — it exists because an unbound scenario reports nothing, and reporting nothing looks exactly like passing.

## Classification

Classify a test by the strongest real boundary its setup, subject, or assertions touch — never by how much it is allowed to use. End-to-end is defined by observing only the process contract: argv in, exit code and streams out. A test that reaches inside the process is not end-to-end no matter what it spawns.

## Changed Gherkin

A change to a `.feature` file or to an adapter's bindings gets the [Gherkin implementation review](../workflows/gherkin-implementation-review.md) before it merges.

## Related

- [Behaviour-driven development](behaviour-driven-development.md)
- [Test-driven development](test-driven-development.md)
