# Configuration

RHINO v0.4 reads one grouped configuration from `repo-config.yml`. Every policy is explicit: an omitted group declares
no policy, and an unknown top-level key is a configuration error. Each policy validator refuses an unknown key inside
the group it reads, except the emoji group, which accepts extra keys. The umbrella keys `policies`, `policies.markdown`,
`policies.governance`, `policies.conventions`, and `policies.plans` are not checked at all. The grouped v2 reference
lists each case. RHINO supplies no repository-specific defaults.

Only `schema` is required. The [grouped v2 reference](./v0-4-configuration.md) owns the document skeleton, every key,
the checked-in schema, modelines, and predecessor handling; this page indexes its groups.

## Groups

- `repository` and `scan` — Repository-wide declarations and traversal scope.
- `harness` — Portable requirements and a nonempty list of opaque adapter profiles.
- `policies.markdown` — Opt-in frontmatter, heading-hierarchy, internal-link, metadata, Mermaid, filename, and
  README-index policy. The Mermaid keys are in [Mermaid policy](v0-4-configuration.md#mermaid-policy).
- `policies.governance` — Configured vendor, layer, traceability, word-budget, directory-map, and quality-gate policy.
  The quality-gate keys are in [Quality-gate policy](v0-4-configuration.md#quality-gate-policy).
- `policies.conventions` — Configured license paths, identifiers, digests, exact exclusions, and emoji-prohibited
  surfaces.
- `policies.plans` — Reserved policy owner; no validator reads it.
- `environment` — Declared key contracts, example/target pairs, detectors, allowlists, and staged-path policy.
- `toolchains` — Declared probes and explicit provision vectors.
- `gates` — Closed lifecycle membership, typed inputs, and declared argv projection.
- `extensions` — Opaque, namespaced mappings that Rhino preserves without interpreting.

Consumers validate their local configuration with `rhino repo-config validate`. The
[grouped v2 reference](./v0-4-configuration.md) documents the keys of the groups it covers, the
[modelines](./v0-4-configuration.md#modelines) a consumer pins, and
[predecessor handling](./v0-4-configuration.md#stable-predecessor-handling).

## Related

- [Grouped v2 Configuration](./v0-4-configuration.md)
- [Command line](./cli.md)
- [Migrate to v0.4](../how-to/migrate-to-v0-4.md)
