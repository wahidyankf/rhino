# Software Quality Enforcement

Before reporting work complete, every gate the change requires has a passing terminal result. Not started, not expected
to pass — passed, with the output.

## Coverage

The policy floor is 99% line coverage over the deterministic core: the domain, application, and config packages, the
exact `-coverpkg` list in `scripts/test-quick.sh`.

The gate pins 99%, so a regression below 99% fails the quick gate. The total was 99.8% when this text was written, so
new deterministic-core code needs its own tests to keep the floor.

The policy names two declared exclusions, but the Go gate declares none. A package outside that list, such as the
command adapter, is simply not measured.

The floor is a floor. Lowering it, widening an exclusion, or excluding a module that became hard to test is a change to
what this repository is willing to ship, not a test-infrastructure adjustment.

## Product Invariants Enforced by Tests

Tree-validation commands are read-only, network-free, process-free, and path-contained:

- it opens no socket, including loopback;
- they spawn no child process;
- they write nothing into the tree they inspect; and
- it follows no path outside the declared root.

`gate run` may start only a declared argv child through its launcher boundary. `toolchain validate` and
`toolchain provision` may start only declared typed argv through their separate runner boundary; probes never install or
report output, and provision stops at its first failed child. `harness adapters` `generate` may replace only declared
adapter roots through its adapter-store boundary, after the complete desired projection has passed loss validation in
memory. It may start only a repository-declared `model-resolve` argument vector through its separate model-resolver
boundary, with no shell and a bounded timeout; when that boundary is missing or its child fails, generation renders the
last recorded or fallback model and warns. `harness adapters validate` never starts it. Every other boundary rejects
being missing; none may fall back to the validator's ordinary read port. No product command opens a socket.

These are enforced by tests rather than by documentation, because a documented invariant is an intention. The
no-loopback rule is stricter than the integration layer's own boundary, which permits an owned socket — deliberately,
because a validator a maintainer hesitates to point at an unfamiliar tree is a validator that does not get run.

The repository's packages import no `unsafe`, but no gate checks that yet, so review confirms it.
`go tool govulncheck ./...` runs in the full gate, and a vulnerability it reports is a defect to fix rather than a rule
to relax.

## Prohibited Data Enforced by a Gate

[Public repository data safety](../conventions/public-repository-data-safety.md) used to rest entirely on review. It now
has a gate: `scripts/public-safety/` runs first on every hook surface and in CI, screening credential patterns with a
scanner pinned by digest and generic private-metadata shapes with a tracked pattern set.

Say what it proves and no more. It proves that no pattern it holds appeared in what it was handed, and that the scanner
it used detected a credential generated moments earlier without reproducing it. It does not prove that a value is safe
to publish: which values are _deliberately_ public is judgement, and the review that convention requires is not
replaced.

## No Superficial Satisfaction

Never make a check pass by weakening it. Deleting an assertion, widening an exclusion, marking a test ignored, catching
an error to reach a return, or asserting something the code cannot fail are all the same move, and all of them leave the
repository reporting a guarantee it no longer has.

When a gate fails, trace it to the earliest responsible cause and fix that. When the cause is that the gate was wrong,
change the gate deliberately, in its own commit, with the reason stated — not inside the change that was inconvenienced
by it.

## Manual Evidence

The gate cannot prove that a published command works. Before changing `docs/`, run the command against the current build
and paste what it printed. See [documentation architecture](../conventions/documentation-architecture.md).

## Related

- [Quality gates](quality-gates.md)
- [The public contract](public-contract.md)
