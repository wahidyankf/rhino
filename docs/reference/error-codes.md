# Error codes

Under `--output json` every refusal writes one document to standard error:

```json
{
  "schemaVersion": 1,
  "error": { "code": "rhino.gate.child-not-found", "message": "…" }
}
```

`schemaVersion` is `1`. `error.code` is drawn from the closed vocabulary below
and is stable: a caller may branch on it. `error.message` is written for a
person and may be reworded in any release, so a caller must not match on it.

The exit status stays inside [its own closed vocabulary](./exit-codes.md); the
code says which of the situations behind a status occurred. Under the default
text output the same refusal is one `rhino: …` line carrying the message alone.
A surface with no gates is not a refusal: `gate run` on it reports `clean` and
exits `0`.

## The vocabulary

Every code is `rhino.<area>.<reason>`. RHINO emits no code outside this table.

| Code                              | Meaning                                                                                                                                                                                                                                                      |
| --------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `rhino.args.unrecognized`         | An unrecognized command, option, or option value, such as a `--surface` this build does not know or a `--base` that is not a commit ID.                                                                                                                      |
| `rhino.args.incomplete`           | An option the command needs is absent, an option is missing its value or the partner it needs, or two options contradict each other.                                                                                                                         |
| `rhino.input.unreadable`          | Standard input was selected and could not be read, or does not hold what the command reads from it.                                                                                                                                                          |
| `rhino.repository.unusable`       | The selected root is missing or is not a directory, or cannot answer what the command needs from it, such as a Git index or commit range.                                                                                                                    |
| `rhino.config.unusable`           | `repo-config.yml` is missing, unreadable, malformed, or inconsistent, such as a declared path or pattern that is absolute or climbs out of the root.                                                                                                         |
| `rhino.config.undeclared`         | The configuration omits the group or policy the command needs.                                                                                                                                                                                               |
| `rhino.path.escapes-root`         | A `--file` or `--directory` selection is absolute, climbs out of the selected root, or passes through a symbolic link; a `--dir` is absolute or climbs out; or a declared environment example or staging allowance is not an exact repository-relative path. |
| `rhino.file.missing`              | A named file or directory does not exist.                                                                                                                                                                                                                    |
| `rhino.file.unreadable`           | A file RHINO had to read exists and could not be read, or lies behind a symbolic link, which RHINO never follows.                                                                                                                                            |
| `rhino.file.exists`               | A file that had to be created is already there.                                                                                                                                                                                                              |
| `rhino.file.unwritable`           | A file that had to be written could not be written, including one whose path passes through a symbolic link.                                                                                                                                                 |
| `rhino.gate.child-not-found`      | A declared gate child was not found.                                                                                                                                                                                                                         |
| `rhino.gate.child-not-executable` | A declared gate child was found and could not be executed.                                                                                                                                                                                                   |
| `rhino.gate.child-refused`        | A declared gate child was refused for any other reason.                                                                                                                                                                                                      |
| `rhino.gate.undeclared`           | A selected gate declares no executable command.                                                                                                                                                                                                              |
| `rhino.toolchain.failed`          | A declared toolchain operation did not complete.                                                                                                                                                                                                             |
| `rhino.toolchain.undeclared`      | A declared toolchain has no provision declaration for this platform.                                                                                                                                                                                         |
| `rhino.harness.refused`           | A declared harness operation was refused.                                                                                                                                                                                                                    |

Adding a code is a breaking change for a caller that treats the vocabulary as
closed, so it moves the minor version while RHINO stays in `0.x`.

## Related

- [Exit codes](./exit-codes.md)
- [JSON output](./json-output.md)
