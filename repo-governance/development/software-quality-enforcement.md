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

Tree validators read files and write nothing. The validator cores (the domain, application, and config packages) import
no `os`, `os/exec`, `net`, `syscall`, `unsafe`, `plugin`, or cgo package, so they cannot open, write, or start anything
directly. The binary's dependency graph has no `net` package, and no product source imports `net`, `unsafe`, or cgo.
`internal/bootstrap/boundary_test.go` enforces these, with a positive control per finder.

The remaining reach is confined, and each confinement is a test:

- The tree adapter (`disk.go`) starts only read-only Git queries (`diff`, `log`, `rev-parse`), and only its environment
  transaction writes (`internal/adapters/runtime/read_boundary_test.go`).
- `os/exec` is imported only by the runtime adapter; `syscall` only by the CLI adapter, for signals and EPIPE.
- `gate run`, `toolchain`, and `harness adapters generate` start only their declared argv, through their own boundaries.
  Tests: `TestDiskToolchainRunnerRunsDeclaredVectorsDirectly` (`toolchain_test.go`) and
  `TestDiskModelResolverRunsTheDeclaredVector` (`adapters_test.go`).
- `harness adapters validate` never starts the model resolver, even when one is supplied
  (`internal/application/harness_validate_resolver_test.go`).
- Each explicit boundary refuses when missing, never falling back to the read port (`environment_test.go`,
  `toolchain_faults_test.go`, `harness_test.go`).
- A path that leaves the root, or passes through a link, is refused (`disk_links_test.go`, `mutation_test.go`,
  `environment_faults_test.go`).

These are enforced by tests rather than by documentation, because a documented invariant is an intention. The
no-loopback rule is stricter than the integration layer's own boundary, which permits an owned socket — deliberately,
because a validator a maintainer hesitates to point at an unfamiliar tree is a validator that does not get run.

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
