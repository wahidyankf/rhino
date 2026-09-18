# How to migrate from v0.3 to v0.4

Use this guide while the v0.4 release candidate is available. The migration is
deliberate: grouped v2 does not alias predecessor keys or commands, and stable
removes the RC-only legacy reader.

## 1. Record the legacy configuration

Keep the existing `repo-config.yml` and record the exact Rhino tag and checksum
that currently validate it. Do not overwrite the legacy file in place before
you have reviewed the new policy ownership.

Run the RC migration leaf against the legacy repository:

```console
$ rhino repo-config migrate
[repo-config-migration] reviewed migration plan for `…`
```

The command prints a plan only. It writes no configuration and a grouped v2
document is refused because it already needs no migration.

## 2. Create a grouped document

Start a new `repo-config.yml` with the grouped schema and add only groups this
repository owns:

```yaml
schema: rhino/repo-config/v2
policies:
  markdown:
    internal-link:
      exclude-sources: []
```

Use the immutable schema artifact URL from the qualified release only for an
editor modeline. Rhino validates local bytes and never fetches that URL.

## 3. Re-express policy by owner

| Legacy concern                                | Grouped v2 owner               | Migration rule                                                                                                           |
| --------------------------------------------- | ------------------------------ | ------------------------------------------------------------------------------------------------------------------------ |
| Legacy gate registry                          | `gates`                        | Declare closed lifecycle membership, typed inputs, argv, and environment projections; do not forward old hook arguments. |
| Harness parity                                | `harness`                      | Define portable requirements and exactly three non-overlapping profiles, then validate before generation.                |
| Frontmatter, links, Mermaid, README presence  | `policies.markdown`            | Opt in to each retained validator with repository-owned scope.                                                           |
| Word budget and directory maps                | `policies.governance`          | Move `count`/`surfaces` to `word-budget` and `trees` to `directory-map`; preserve values and findings.                   |
| Scan exclusions                               | `scan.exclude-directories`     | Preserve the declared directory names; ignore files do not silently change validator scope.                              |
| Vendor terms, layer order, traceability       | `policies.governance`          | Declare roots, vocabulary, categories, artifacts, and relationships without organization defaults.                       |
| License checks                                | `policies.conventions.license` | Declare paths, identifiers, digests, and exact exclusions.                                                               |
| Environment examples, detectors, staged paths | `environment`                  | Name paths and keys, never values; review a plan before `--apply`.                                                       |
| Tool discovery and installation               | `toolchains`                   | Declare no-shell probes and platform provision vectors; provision only with `--apply`.                                   |

The older structure validators, filename, emoji, metadata, and legacy
harness-parity leaves have no grouped-v2 alias. Keep them only as RC legacy
input while you decide their local owner or remove their obsolete policy.

## 4. Validate before adoption

Validate the grouped file locally, then run only the declared v0.4 leaves:

```sh
rhino repo-config validate
rhino gate validate
rhino gate list
```

An omitted group makes its leaf refuse with exit `2`; that is evidence of an
undeclared policy, not a clean result. For operations, review the plan first
and pass `--apply` only at the explicit mutation boundary.

For one semantic gate that checks Conventional Commit messages, retain one
`commit-message` input and one command. Bind it to `hook-message-file` at
`commit-msg`, then to `explicit-range` with `range: explicit` at
`pull-request`; Rhino reads the reviewed non-merge messages from the immutable
`base..head` range for the replay. Do not create a second PR-only command or
pass a mutable hook-file path to pull-request CI.

## 5. Finish before stable

Commit the grouped configuration, its immutable version/checksum pin, and
native validation evidence together. Remove predecessor configuration from the
active path before adopting stable. Do not retain a fallback command, a wrapper
that translates old flags, or a conditional schema reader: stable deliberately
removes that compatibility code.

## Related

- [Configuration](../reference/configuration.md)
- [Grouped v2 Configuration](../reference/v0-4-configuration.md)
- [Command line](../reference/cli.md)
