# Dependency Selection

A dependency is a permanent obligation: it enters the lock file, the supply-chain gate, the release archive, and every
consumer's trust boundary. Adding one is a decision that needs stating.

## Requirements

Before adding a dependency, record:

- **The need.** What this repository must do that it cannot reasonably do itself. "Convenient" is not a need.
- **The alternatives rejected.** Including doing it by hand, and including the standard library. Name them and say why
  each lost.
- **Evidence of maintenance.** Release cadence, defect response, and whether the module is still owned by someone. An
  archived module is a fork you have not written yet.
- **The owned consequence.** What this repository does when the dependency is abandoned, changes licence, or ships an
  advisory. If the answer is "vendor it", say so before adopting it, not afterwards.

Prefer the standard library, then a small well-maintained module, then a large one. Prefer a module this repository
already depends on over a new one with the same capability.

## Supply Chain

`go tool govulncheck ./...` runs in the full gate. It reports known vulnerabilities in the module's dependencies and the
standard library it links, and a dependency with a reported advisory is not a candidate until that advisory is fixed.
The gate checks nothing else: no licence check, and no check for duplicate module versions. A dependency's licence is a
review question, answered in the change that adds it.

A dependency that imports `unsafe` is not automatically disqualifying, because the rule binds this repository's own code
rather than the ecosystem. It is a consequence to own explicitly, and the record says so. This repository's packages
import no `unsafe`, and no gate checks that yet.

## Removal

Removing a dependency needs no justification beyond the change passing. Prefer it.

## Related

- [Minimal sufficiency](../principles/minimal-sufficiency.md)
- [Software quality enforcement](software-quality-enforcement.md)
