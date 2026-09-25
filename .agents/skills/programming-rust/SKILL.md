---
name: programming-rust
description: >-
  Guides Rust work under the Rust standard: reaching a red that fails on an assertion, resolving borrow checker friction
  through ownership design, replacing each wanted unwrap, and spotting calls that block the async runtime.
when_to_use: >-
  Use when writing, changing, or reviewing Rust code in a crate or workspace, before the first test of the change.
compatibility: Requires a Rust crate or workspace with Cargo, rustfmt, and Clippy.
---

# Rust Programming

Every Rust rule is owned by [Rust Standards](../../../repo-governance/development/quality/stacks/rust-standards.md) and
its
[Code Shape and Security](../../../repo-governance/development/quality/stacks/rust-standards/001-code-shape-and-security.md)
module. [Test-Driven Development](../../../repo-governance/development/test-driven-development.md) and
[Quality Gates](../../../repo-governance/development/quality-gates.md) govern tests and gates,
[Red, Green, Refactor](../../../repo-governance/workflows/red-green-refactor.md) runs each cycle, and
[Developing Applications](../developing-applications/SKILL.md) carries the judgement on layers, errors, logs, and input
that holds in every language. This skill adds only the procedure and judgement of applying them in Rust. Where a
sentence here seems to state a rule, the standard decides.

## Start From the Manifest

Read the lint table the crate inherits, its declared edition, whether its manifest declares it a low-level crate, and
the recorded runtime and serialization choices. Run the format check, Clippy, and the tests on the untouched tree. A
gate already failing before any edit is handled under Preexisting Error Resolution.

## Reach a Red That Counts

A test calling a missing function fails to compile, and `todo!()` compiles but fails by panicking; neither is a red.
Give the function its signature and a body returning a value the assertion rejects, such as an empty collection or an
error variant the test does not expect, then run it. A test expecting a panic suits only a behaviour whose contract is
that panic.

## Settle Borrow Errors Through Ownership

When the borrow checker rejects a design, ask first who owns the data and for how long, and only then consider a copy or
shared mutability.

| The friction                                                          | Usually                                                           |
| --------------------------------------------------------------------- | ----------------------------------------------------------------- |
| a struct holds a reference, and lifetimes spread into every signature | the struct owns the data, or receives it on each call             |
| two parts both want to change one value                               | one part owns and changes it; the other sends a request or result |
| a value is used after it moved into a call                            | the call borrows, or the caller copies a small value on purpose   |
| a reference must outlive the scope that made it                       | the function returns an owned value                               |

Cloning small data at a boundary is a sound answer. A clone added only to silence the compiler inside a loop over large
data is not. Shared ownership with interior mutability is a design decision, stated in a comment where it is introduced,
never a first reflex.

## Replace Each Unwrap You Reach For

When code wants `unwrap()`, choose what the situation actually is:

- the caller can decide, so propagate with `?`;
- the check belongs where the value is created, so build a validated newtype that cannot hold the bad value; or
- a proof already sits nearby, so use `expect()` with a message naming the invariant.

When none of these fits, the failure is a real outcome, and the function returns it.

## Find What Blocks the Runtime

Inside async code, look for calls that wait without yielding: standard-library file or socket operations, a synchronous
lock held while something else waits, a long computation over a large input, and a synchronous client library. Each one
moves as the standard directs.

A test that passes on a multi-threaded runtime can stall on a single-threaded one, because the blocked thread was the
only one. Run async tests on the runtime flavour production uses, so a blocking call surfaces as a failure rather than
as an outage.

## Answering a Lint

Fix a pedantic Clippy finding unless its premise does not hold at that site; the `allow` then names why. Reaching for
`unsafe` in a crate whose manifest does not declare it low-level is a design question for review, never a local fix.

## Before Handing Off

- the format check, Clippy with warnings denied across all targets, and the tests all passed;
- every `expect()` added names its invariant, and no new production path panics on a failure a caller could handle;
- every blocking call inside async code moved off the runtime; and
- each recorded red failed on an assertion about the missing behaviour.
