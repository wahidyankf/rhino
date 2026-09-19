# 🦏 RHINO

**Repository Hygiene & INtegration Orchestrator** checks that a repository
matches the policy it explicitly declares. It ships no repository, organization,
harness, tree, or threshold defaults.

[![Quality gate](https://github.com/wahidyankf/rhino/actions/workflows/pr-quality-gate.yml/badge.svg)](https://github.com/wahidyankf/rhino/actions/workflows/pr-quality-gate.yml)
[![Rust](https://img.shields.io/badge/rust-1.85-000000)](https://www.rust-lang.org/)
[![Platforms](https://img.shields.io/badge/platforms-macOS%20%7C%20Linux-lightgrey)](#install)
[![License](https://img.shields.io/badge/license-MIT-blue)](./LICENSE)

RHINO validates declared Markdown, governance, lifecycle, harness, environment,
and toolchain policy. Validators read local files only: they do not write,
spawn, or connect to a network. Declared gates and explicit operations use
separate, narrow boundaries.

## Highlights

- **No policy of its own.** Every enforced value comes from `repo-config.yml`.
- **Closed grouped configuration.** `rhino/repo-config/v2` rejects unknown core
  groups and keys instead of treating a typo as silent policy.
- **Explicit operations.** Adapter generation, environment setup, toolchain
  provision, and gate execution need their own declared authority.
- **Stable exit meanings.** `1` means a declared policy found a violation; `2`
  means the invocation, configuration, or boundary was unusable.
- **Machine-readable results.** `--output json` keeps programmatic callers out
  of human prose.

## Install

Download a published, immutable release tag and verify its checksum before
extracting it. The qualified v0.4 release carries one checksum manifest for all
four supported archives.

```sh
VERSION=REPLACE_WITH_QUALIFIED_TAG
TARGET=aarch64-apple-darwin
BASE="https://github.com/wahidyankf/rhino/releases/download/$VERSION"

curl -fsSLO "$BASE/rhino-$TARGET.tar.gz"
curl -fsSLO "$BASE/checksums.txt"
shasum -a 256 --ignore-missing -c checksums.txt
tar -xzf "rhino-$TARGET.tar.gz"
```

Use only a tag that the release page lists as qualified. The full procedure,
including the Linux checksum alternative and PATH installation, is in
[How to install a pinned release](./docs/how-to/install-a-pinned-release.md).

## Quick start

Create a grouped configuration and declare only the policy your repository
owns. This minimal example adopts internal-link validation:

```yaml
schema: rhino/repo-config/v2
policies:
  markdown:
    internal-link:
      exclude-sources: []
```

Validate the configuration, then run its declared validator:

```console
$ rhino repo-config validate
$ rhino md internal-link validate
```

An omitted policy group makes the leaf that needs it exit `2`; it is not a
clean result. Add Markdown, governance, gate, harness, environment, and
toolchain groups only when the repository can name their scope and owner.

## Lifecycle and operations

`gate list` shows declared membership across the seven closed lifecycle
surfaces. `gate validate` checks membership and pull-request composition before
`gate run --surface <name>` dispatches a declared typed argv projection. A gate
cannot replace its command with arguments after `--`.

Environment initialization and toolchain provision plan first and require
`--apply` for a declared mutation. Environment backup and restore require a
repository-relative `--dir`; restore additionally requires `--force` before it
replaces an existing target. Harness adapter generation validates the complete
three-profile projection before it replaces declared adapter families and exact
instruction-adapter files.

## v0.3 to v0.4 migration

Stable v0.4 accepts only the grouped schema. It rejects predecessor schemas and
does not provide aliases, compatibility wrappers, or a migration command.
Create an explicit grouped configuration and validate it locally before using
the stable binary.

[How to migrate from v0.3 to v0.4](./docs/how-to/migrate-to-v0-4.md) gives the
owner-by-owner mapping and verification sequence.

## Documentation

The documentation follows [Diátaxis](https://diataxis.fr/).

| Section                                     | Use it when                                                  |
| ------------------------------------------- | ------------------------------------------------------------ |
| [Tutorials](./docs/tutorials/README.md)     | You are learning the product surface.                        |
| [How-to guides](./docs/how-to/README.md)    | You have a concrete maintenance goal.                        |
| [Reference](./docs/reference/README.md)     | You need an exact command, flag, code, or configuration key. |
| [Explanation](./docs/explanation/README.md) | You need the design reasoning.                               |

The [specifications tree](./specs/README.md) is canonical. If documentation and
the executable Gherkin corpus disagree, the corpus wins.

## Project status

RHINO is a pre-1.0 product. The planned v0.4 release is breaking by design:
stable accepts the grouped contract only, rather than carrying an unbounded
compatibility layer. Published tags and assets are immutable.

Contributor rules for maintainers and automated agents start at
[AGENTS.md](./AGENTS.md). External contributions are currently closed while the
engineering patterns stabilize.

## Project context

RHINO is part of the Open Sharia Enterprise repository group and remains usable
on its own. Its membership is navigation, not product coupling: repositories
choose their own policy, version, release cadence, and configuration.
