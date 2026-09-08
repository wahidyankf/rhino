# Software Quality Enforcement

Before reporting work complete, every gate the change requires has a passing terminal result. Not started, not expected to pass — passed, with the output.

## Coverage

Line coverage over the validator modules stays at or above 99%. `src/main.rs` and the concrete filesystem adapter are the only declared exclusions, and `README.md` says what they are and why a unit test may not reach them.

The floor is a floor. Lowering it, widening an exclusion, or excluding a module that became hard to test is a change to what this repository is willing to ship, not a test-infrastructure adjustment.

## Product Invariants Enforced by Tests

The product is read-only, network-free, process-free, and path-contained:

- it opens no socket, including loopback;
- it spawns no child process;
- it writes nothing into the tree it inspects; and
- it follows no path outside the declared root.

These are enforced by tests rather than by documentation, because a documented invariant is an intention. The no-loopback rule is stricter than the integration layer's own boundary, which permits an owned socket — deliberately, because a validator a maintainer hesitates to point at an unfamiliar tree is a validator that does not get run.

`#![forbid(unsafe_code)]` stays. `cargo deny check` gates advisories, licences, and duplicate sources, and a new advisory is a defect to fix rather than a rule to relax.

## No Superficial Satisfaction

Never make a check pass by weakening it. Deleting an assertion, widening an exclusion, marking a test ignored, catching an error to reach a return, or asserting something the code cannot fail are all the same move, and all of them leave the repository reporting a guarantee it no longer has.

When a gate fails, trace it to the earliest responsible cause and fix that. When the cause is that the gate was wrong, change the gate deliberately, in its own commit, with the reason stated — not inside the change that was inconvenienced by it.

## Manual Evidence

The gate cannot prove that a published command works. Before changing `docs/`, run the command against the current build and paste what it printed. See [documentation architecture](../conventions/documentation-architecture.md).

## Related

- [Quality gates](quality-gates.md)
- [The public contract](public-contract.md)
