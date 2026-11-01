# Repository Configuration Schemas

These JSON Schema artifacts describe the grouped repository configuration for editors. They are maintained by hand:
there is no schema generator in this tree, and no test or CI step compares them with the Go configuration reader in
`internal/adapters/config`, which decides what Rhino accepts. Regenerating them is future work.

## Directory Map

- [v2 Schema](v2.schema.json) — checked-in Draft 2020-12 schema for `rhino/repo-config/v2`.
- [v2 Producer Example](v2.example.yml) — a source-tree example using the relative schema modeline.
