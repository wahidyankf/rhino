# Command line

Every command except `version` and `help` reads `repo-config.yml` from the selected repository root.
`rhino/repo-config/v2` is the active grouped v0.4 contract. An omitted policy group is refused by the leaf that needs
it; RHINO does not invent a policy or route an old command through a replacement.

## Grouped v0.4 leaves

- **Configuration** — `repo-config validate`
- **Lifecycle gates** — `gate list`, `gate validate`, `gate run --surface <name>`
- **Harness adapters** — `harness adapters validate`, `harness adapters generate`
- **Environment** — `env validate`, `env init --apply`, `env backup --dir <path>`, `env restore --dir <path> [--force]`
- **Toolchains** — `toolchain validate`, `toolchain provision --apply`
- **Markdown** — `md frontmatter validate`, `md heading-hierarchy validate`, `md internal-link validate`,
  `md mermaid validate`, `md naming validate`, `md readme-index validate`
- **Metadata** — `metadata validate`
- **Governance** — `governance word-budget validate`, `governance directory-map validate`, `governance vendor validate`,
  `governance layers validate`, `governance traceability validate`, `governance quality-gates validate`
- **Conventions** — `convention emoji validate`, `convention license validate`
- **Build identity** — `version`, `version --json`
- **Help** — `help`, `help <command path>`

`gate run` accepts only a declared closed surface: `pre-commit`, `commit-msg`, `pre-push`, `pull-request`, `main`,
`scheduled`, or `manual`. Its configuration declares every typed input, argv token, and environment projection.
Arguments after `--` are refused, so a caller cannot replace or extend a declared command.

Tree validators read files, write nothing, start no process, and use no network. Adapter generation, declared gate
execution, environment operations, and toolchain provision use their own explicit boundaries.

## Options

- `-h`, `--help`
  - Accepted by: Every command
  - Meaning: Print help for the command path given, on stdout, and exit `0`.
- `-V`, `--version`
  - Accepted by: Every command
  - Meaning: Print the release identity, as `version` does, and exit `0`.
- `--root <path>`
  - Accepted by: Every command
  - Meaning: Select a repository root.
- `--output <text\|json>`
  - Accepted by: Every command
  - Meaning: Select text or JSON rendering.
- `--quiet`, `--verbose`, `--no-color`
  - Accepted by: Every command
  - Meaning: Presentation-only compatibility options.
- `--file <path>`
  - Accepted by: `md mermaid validate`
  - Meaning: Repeatable repository-relative input; `-` reads standard input.
- `--directory <path>`
  - Accepted by: `governance directory-map validate`
  - Meaning: Replace the legacy directory-map tree selection.
- `--dir <path>`
  - Accepted by: `env backup`, `env restore`
  - Meaning: Required repository-relative backup directory.
- `--apply`
  - Accepted by: `env init`, `toolchain provision`
  - Meaning: Authorize the declared mutation after planning.
- `--force`
  - Accepted by: `env restore`
  - Meaning: Authorize replacement after a recoverable backup plan.
- `--surface <name>`
  - Accepted by: `gate run`
  - Meaning: Select one closed lifecycle surface.
- `--message-file <path>`
  - Accepted by: `gate run` at `commit-msg`
  - Meaning: Pass Git's current `COMMIT_EDITMSG` hook path only.
- `--push-updates-stdin`
  - Accepted by: `gate run` at `pre-push`
  - Meaning: Read Git update records from standard input, and hand them to each gate child.
- `--base <sha> --head <sha>`
  - Accepted by: `gate run` at `pull-request`
  - Meaning: Supply the immutable pull-request range.
- `--json`
  - Accepted by: `version`
  - Meaning: Shorthand for `--output json`.

A flag a leaf does not accept is an invocation error. Repository-relative paths cannot be absolute or escape the
selected root.

## Exit codes

| Code    | Meaning                                                                    |
| ------- | -------------------------------------------------------------------------- |
| `0`     | The command completed cleanly.                                             |
| `1`     | A declared policy found a violation.                                       |
| `2`     | The invocation, root, configuration, or declared boundary was unusable.    |
| `126`   | A declared gate child could not be executed. Only `gate run` returns it.   |
| `127`   | A declared gate child was not found. Only `gate run` returns it.           |
| `128+N` | RHINO was ended by signal `N`; `130` is an interrupt, `141` a closed pipe. |

## Related

- [Configuration](./configuration.md)
- [Grouped v2 Configuration](./v0-4-configuration.md)
- [Migrate to v0.4](../how-to/migrate-to-v0-4.md)
- [Exit codes](./exit-codes.md)
