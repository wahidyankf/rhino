# Dependency Selection

A dependency is a permanent obligation: it enters the lock file, the supply-chain gate, the release archive, and every consumer's trust boundary. Adding one is a decision that needs stating.

## Requirements

Before adding a dependency, record:

- **The need.** What this repository must do that it cannot reasonably do itself. "Convenient" is not a need.
- **The alternatives rejected.** Including doing it by hand, and including the standard library. Name them and say why each lost.
- **Evidence of maintenance.** Release cadence, defect response, and whether the crate is still owned by someone. An archived crate is a fork you have not written yet.
- **The owned consequence.** What this repository does when the dependency is abandoned, changes licence, or ships an advisory. If the answer is "vendor it", say so before adopting it, not afterwards.

Prefer the standard library, then a small well-maintained crate, then a large one. Prefer a crate this repository already depends on over a new one with the same capability.

## Supply Chain

`cargo deny check` gates advisories, licences, and duplicate sources. A dependency that cannot pass it is not a candidate.

`unsafe` in a transitive dependency is not automatically disqualifying — `#![forbid(unsafe_code)]` binds this crate, not the ecosystem — but it is a consequence to own explicitly, and the record says so.

## Removal

Removing a dependency needs no justification beyond the change passing. Prefer it.

## Related

- [Minimal sufficiency](../principles/minimal-sufficiency.md)
- [Software quality enforcement](software-quality-enforcement.md)
