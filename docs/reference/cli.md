# Command line

Every command except `version` reads `repo-config.yml` from the selected
repository root. `rhino/repo-config/v2` is the active grouped v0.4 contract.
An omitted policy group is refused by the leaf that needs it; RHINO does not
invent a policy or route an old command through a replacement.

## Grouped v0.4 leaves

| Area             | Commands                                                                                                  |
| ---------------- | --------------------------------------------------------------------------------------------------------- |
| Configuration    | `repo-config validate`, `repo-config migrate`                                                             |
| Lifecycle gates  | `gate list`, `gate validate`, `gate run --surface <name>`                                                 |
| Harness adapters | `harness adapters validate`, `harness adapters generate`                                                  |
| Environment      | `env validate`, `env init --apply`, `env backup --dir <path>`, `env restore --dir <path> [--force]`       |
| Toolchains       | `toolchain validate`, `toolchain provision --apply`                                                       |
| Markdown         | `md frontmatter validate`, `md internal-link validate`, `md mermaid validate`, `md readme-index validate` |
| Governance       | `governance vendor validate`, `governance layers validate`, `governance traceability validate`            |
| Conventions      | `convention license validate`                                                                             |
| Build identity   | `version`, `version --json`                                                                               |

`repo-config migrate` reads a predecessor configuration only during the release
candidate and prints a reviewed plan. It never rewrites a configuration. A
grouped v2 configuration is refused by that command because it needs no
migration.

`gate run` accepts only a declared closed surface: `pre-commit`, `commit-msg`,
`pre-push`, `pull-request`, `main`, `scheduled`, or `manual`. Its configuration
declares every typed input, argv token, and environment projection. Arguments
after `--` are refused, so a caller cannot replace or extend a declared command.

Tree validators read files, write nothing, start no process, and use no
network. Adapter generation, declared gate execution, environment operations,
and toolchain provision use their own explicit boundaries.

## RC legacy leaves

The following leaves remain readable only with a predecessor configuration
during the release candidate. They are not v0.4 aliases and do not accept a
grouped v2 configuration:

| Area                    | Legacy leaves                                                                                                      |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------ |
| Governance structure    | `governance roots validate`, `governance companions validate`, `governance instructions validate`, `plan validate` |
| Earlier validators      | `md word-count inspect`                                                                                            |
| Earlier harness surface | `harness parity validate`                                                                                          |

Migrate the policy deliberately before stable. The stable release removes the
legacy reader rather than forwarding any of these leaves to a new surface.

## Options

| Option                               | Accepted by                                    | Meaning                                                         |
| ------------------------------------ | ---------------------------------------------- | --------------------------------------------------------------- |
| `--root <path>`                      | Every command                                  | Select a repository root.                                       |
| `--output <text\|json>`              | Every command                                  | Select text or JSON rendering.                                  |
| `--quiet`, `--verbose`, `--no-color` | Every command                                  | Presentation-only compatibility options.                        |
| `--file <path>`                      | `md mermaid validate`, `md word-count inspect` | Repeatable repository-relative input; `-` reads standard input. |
| `--directory <path>`                 | `governance directory-map validate`            | Replace the legacy directory-map tree selection.                |
| `--dir <path>`                       | `env backup`, `env restore`                    | Required repository-relative backup directory.                  |
| `--apply`                            | `env init`, `toolchain provision`              | Authorize the declared mutation after planning.                 |
| `--force`                            | `env restore`                                  | Authorize replacement after a recoverable backup plan.          |
| `--harness <name>`                   | `harness parity validate`                      | Select one predecessor harness during the RC migration window.  |
| `--surface <name>`                   | `gate run`                                     | Select one closed lifecycle surface.                            |
| `--message-file <path>`              | `gate run` at `commit-msg`                     | Supply the exact hook message path.                             |
| `--push-updates-stdin`               | `gate run` at `pre-push`                       | Read Git update records from standard input.                    |
| `--base <sha> --head <sha>`          | `gate run` at `pull-request`                   | Supply the immutable pull-request range.                        |
| `--json`                             | `version`                                      | Shorthand for `--output json`.                                  |

A flag a leaf does not accept is an invocation error. Repository-relative paths
cannot be absolute or escape the selected root.

## Exit codes

| Code | Meaning                                                                   |
| ---- | ------------------------------------------------------------------------- |
| `0`  | The command completed cleanly.                                            |
| `1`  | A declared policy found a violation.                                      |
| `2`  | The invocation, root, configuration, or declared boundary was unusable.   |
| `3`  | A declared gate child could not start. Only `gate run` returns this code. |

## Related

- [Configuration](./configuration.md)
- [Grouped v2 Configuration](./v0-4-configuration.md)
- [Migrate to v0.4](../how-to/migrate-to-v0-4.md)
- [Exit codes](./exit-codes.md)
