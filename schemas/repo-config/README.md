# Repository Configuration Schemas

These JSON Schema artifacts describe the grouped repository configuration. Run
`cargo xtask schema` to regenerate them, or `cargo xtask schema --check` to
prove the checked-in bytes still derive from Rhino's model. Do not edit a
generated JSON file by hand.

## Directory Map

- [v2 Schema](v2.schema.json) — generated Draft 2020-12 schema for `rhino/repo-config/v2`.
- [v2 Producer Example](v2.example.yml) — a source-tree example using the relative schema modeline.
