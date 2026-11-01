# Why the Grouped Schema Is Checked In

The grouped v2 configuration schema was derived from one typed model while the Rust implementation had a generator.
`cargo xtask schema` wrote the Draft 2020-12 artifact from that model, and `cargo xtask schema --check` proved that the
checked-in bytes still matched it. The artifact stayed checked in beside the model, so an editor's structural feedback
and Rhino's structural reader were built on one description, not two hand-maintained descriptions that can drift.

There is no schema generator in the Go tree. The generator left with the Rust implementation, so the artifact is
maintained by hand, and no test or CI step compares it with the Go configuration reader in `internal/adapters/config`,
which checks configuration with hand-written rules. Regenerating the artifact from a typed model is future work. Until
that lands, a schema edit has no proving step. Runtime validation reads local configuration bytes only and never reads
the artifact, so a hand edit cannot change what Rhino accepts. A modeline helps an editor find a schema but never
authorizes Rhino to fetch one.

## Dependency Decision

Deriving the artifact from the model, rather than hand-writing a second JSON schema, was chosen because a hand-written
artifact creates a second description to maintain. The Rust generator's dependencies, including `schemars`, left with
the Rust implementation, and the shipped Go binary depends only on the modules `go.mod` pins. The Go rewrite therefore
returned the artifact to the state that decision rejected, and it stays hand-maintained until a generator returns.
