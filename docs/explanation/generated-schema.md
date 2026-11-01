# Why the Grouped Schema Is Generated

The grouped v2 configuration was derived from one typed model, so an editor's structural feedback and Rhino's structural
reader shared one source of truth instead of two hand-maintained descriptions that can drift. The Draft 2020-12 artifact
stays checked in beside the model it came from.

There is no schema generator in the Go tree: the generator left with the Rust implementation, and regenerating the
artifact from the Go model (`internal/adapters/config`, YAML only) is tracked work. Until that lands, schema edits have
no proving step, so do not edit the checked-in JSON by hand. Runtime validation reads local configuration bytes only. A
modeline helps an editor find a schema but never authorizes Rhino to fetch one.

## Dependency Decision

A hand-written JSON artifact was rejected because it creates a second schema to maintain; the checked-in artifact keeps
that property until a supported generator takes over. The generator's dependencies left with the Rust implementation,
and the shipped Go binary depends only on the modules `go.mod` pins.
