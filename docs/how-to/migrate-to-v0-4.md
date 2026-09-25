# How to migrate from v0.3 to v0.4

Use this guide to convert a predecessor configuration before using stable v0.4.
Stable accepts only grouped v2; it does not alias predecessor keys or commands,
read predecessor configuration, or provide a migration command.

## 1. Record the legacy configuration

Keep the existing `repo-config.yml` and record the exact Rhino tag and checksum
that last validated it. Do not overwrite the legacy file in place before you
have reviewed the new policy ownership.

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

Use the immutable schema artifact URL from the release you pin
(`rhino-repo-config-v2.schema.json`) only for an editor modeline. Rhino validates local bytes and never fetches that URL.

## 3. Re-express policy by owner

| Legacy concern                                                                    | Grouped v2 owner           | Migration rule                                                                                                           |
| --------------------------------------------------------------------------------- | -------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| Legacy gate registry                                                              | `gates`                    | Declare closed lifecycle membership, typed inputs, argv, and environment projections; do not forward old hook arguments. |
| Harness parity                                                                    | `harness`                  | Define portable requirements and exactly three non-overlapping profiles, then validate before generation.                |
| Frontmatter, heading hierarchy, links, metadata, Mermaid, naming, README presence | `policies.markdown`        | Opt in to each retained validator with repository-owned scope.                                                           |
| Word budget and directory maps                                                    | `policies.governance`      | Move `count`/`surfaces` to `word-budget` and `trees` to `directory-map`; preserve values and findings.                   |
| Scan exclusions                                                                   | `scan.exclude-directories` | Preserve the declared directory names; ignore files do not silently change validator scope.                              |
| Vendor terms, layer order, traceability                                           | `policies.governance`      | Declare roots, vocabulary, categories, artifacts, and relationships without organization defaults.                       |
| License checks and emoji prohibition                                              | `policies.conventions`     | Declare license paths, identifiers, digests, exclusions, and emoji-prohibited surfaces.                                  |
| Environment examples, detectors, staged paths                                     | `environment`              | Name paths and keys, never values; review a plan before `--apply`.                                                       |
| Tool discovery and installation                                                   | `toolchains`               | Declare no-shell probes and platform provision vectors; provision only with `--apply`.                                   |

The retired structure validators, `md word-count inspect`, and legacy
harness-parity leaves have no grouped-v2 alias. Retire their obsolete local
policy during conversion; stable does not register those commands.

## 4. Validate before adoption

Validate the grouped file locally, then run only the declared v0.4 leaves:

```sh
rhino repo-config validate
rhino gate validate
rhino gate list
```

An omitted group makes its leaf refuse with exit `2`; that is evidence of an
undeclared policy, not a clean result. Leaving `gates` out makes both commands
above exit `2`. An explicitly empty group, such as `gates: {}`, is a
declaration and reports clean. For operations, review the plan first and pass
`--apply` only at the explicit mutation boundary.

For one semantic gate that checks Conventional Commit messages, retain one
`commit-message` input and one command. Bind it to `hook-message-file` at
`commit-msg`, then to `explicit-range` with `range: explicit` at
`pull-request`; Rhino reads the reviewed non-merge messages from the immutable
`base..head` range for the replay. Do not create a second PR-only command or
pass a mutable hook-file path to pull-request CI.

## 5. Finish with stable

Commit the grouped configuration, its immutable version/checksum pin, and
native validation evidence together. Remove predecessor configuration from the
active path before adopting stable. Do not retain a fallback command, a wrapper
that translates old flags, or a conditional schema reader: stable deliberately
has no compatibility path.

## Related

- [Configuration](../reference/configuration.md)
- [Grouped v2 Configuration](../reference/v0-4-configuration.md)
- [Command line](../reference/cli.md)
