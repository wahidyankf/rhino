# Configuration

RHINO v0.4 reads one grouped configuration from `repo-config.yml`. Every policy
is explicit: an omitted group declares no policy, and an unknown group or key
is a configuration error. RHINO supplies no repository-specific defaults.

```yaml
schema: rhino/repo-config/v2
repository: {}
scan: {}
harness: {}
policies:
  markdown: {}
  governance: {}
  conventions: {}
  plans: {}
environment: {}
toolchains: {}
gates: {}
extensions: {}
```

Only `schema` is required. The other groups are optional and closed. An empty
group is an explicit owner boundary; omitting it says that this repository
declares no policy in that area.

## Groups

| Group                   | Purpose                                                                  |
| ----------------------- | ------------------------------------------------------------------------ |
| `repository` and `scan` | Repository-wide declarations and traversal scope.                        |
| `harness`               | Portable requirements and exactly three opaque adapter profiles.         |
| `policies.markdown`     | Opt-in frontmatter, internal-link, Mermaid, and README-index policy.     |
| `policies.governance`   | Configured vendor, layer, and traceability policy.                       |
| `policies.conventions`  | Configured license placement, identifiers, and digests.                  |
| `policies.plans`        | Reserved closed policy owner.                                            |
| `environment`           | Declared example/target pairs, detectors, and staged-path policy.        |
| `toolchains`            | Declared probes and explicit provision vectors.                          |
| `gates`                 | Closed lifecycle membership, typed inputs, and declared argv projection. |
| `extensions`            | Opaque, namespaced mappings that Rhino preserves without interpreting.   |

The [grouped v2 reference](./v0-4-configuration.md) defines each supported
group, generated schema, offline modeline behavior, and operation boundary.

## Schema and modelines

The checked-in schema is Draft 2020-12 and derives from the same typed Rust
model that parses `repo-config.yml`. Rhino reads local bytes and never fetches
a modeline URL at runtime. A consumer pins an immutable release schema URL for
editor use; the modeline does not grant network authority to the binary.

```yaml
schema: rhino/repo-config/v2
```

In Rhino source, `cargo xtask schema` regenerates the artifact and
`cargo xtask schema --check` proves it has not drifted. Consumers validate their
local configuration with `rhino repo-config validate`.

## Release-candidate migration boundary

During the v0.4 release candidate, the predecessor schemas
`rhino/repo-config/v1` and `ose/repo-config/v2` remain readable solely as
legacy input. They do not alias to grouped v2 keys or commands. Run
`rhino repo-config migrate` against a legacy file to print a reviewed migration
plan, then create and validate the grouped document explicitly.

The RC-only reader is removed before stable. New adopters must start with
`rhino/repo-config/v2`; existing adopters should follow
[the v0.3-to-v0.4 migration guide](../how-to/migrate-to-v0-4.md).

## Related

- [Grouped v2 Configuration](./v0-4-configuration.md)
- [Command line](./cli.md)
- [Migrate to v0.4](../how-to/migrate-to-v0-4.md)
