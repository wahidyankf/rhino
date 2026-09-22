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

## The vocabulary

Every code is `rhino.<area>.<reason>`. RHINO emits no code outside this table.

| Code                             | Meaning                                                         |
| -------------------------------- | --------------------------------------------------------------- |
| `rhino.args.unrecognized`        | An unrecognized command, option, or option value.               |
| `rhino.args.incomplete`          | An option that takes a value was given none.                    |
| `rhino.input.unreadable`         | Standard input was selected and could not be read.              |
| `rhino.repository.unusable`      | The selected root is missing, is not a directory, or is not usable. |
| `rhino.config.unusable`          | `repo-config.yml` is missing, unreadable, malformed, or inconsistent. |
| `rhino.config.undeclared`        | The configuration declares no policy for what the command needs. |
| `rhino.path.escapes-root`        | A selected path is absolute or escapes the selected root.        |
| `rhino.file.missing`             | A named file does not exist.                                     |
| `rhino.file.unreadable`          | A named file exists and could not be read.                       |
| `rhino.file.exists`              | A file that had to be created is already there.                  |
| `rhino.gate.child-not-found`     | A declared gate child was not found.                             |
| `rhino.gate.child-not-executable`| A declared gate child was found and could not be executed.       |
| `rhino.gate.child-refused`       | A declared gate child was refused for any other reason.          |
| `rhino.gate.undeclared`          | The selected surface declares no gates.                          |
| `rhino.toolchain.failed`         | A declared toolchain operation did not complete.                 |
| `rhino.toolchain.undeclared`     | The configuration declares no toolchain for the request.         |
| `rhino.harness.refused`          | A declared harness operation was refused.                        |

Adding a code is a breaking change for a caller that treats the vocabulary as
closed, so it moves the minor version while RHINO stays in `0.x`.

## Related

- [Exit codes](./exit-codes.md)
- [JSON output](./json-output.md)
