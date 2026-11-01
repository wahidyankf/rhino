# Repository Configuration Schemas

These JSON Schema artifacts describe the grouped repository configuration. They are checked in as the source of truth.
There is no schema generator in this tree, so they are maintained by hand; no test or CI step compares them with the
model, and regenerating them from the model is tracked work.

## Directory Map

- [v2 Schema](v2.schema.json) — checked-in Draft 2020-12 schema for `rhino/repo-config/v2`.
- [v2 Producer Example](v2.example.yml) — a source-tree example using the relative schema modeline.
