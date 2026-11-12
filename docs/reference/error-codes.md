# Error codes

Under `--output json` every refusal writes one document to standard error:

```json
{
  "schemaVersion": 1,
  "error": { "code": "rhino.gate.child-not-found", "message": "…" }
}
```

`schemaVersion` is `1`. `error.code` is drawn from the closed vocabulary below and is stable: a caller may branch on it.
`error.message` is written for a person and may be reworded in any release, so a caller must not match on it.

The exit status stays inside [its own closed vocabulary](./exit-codes.md); the code says which of the situations behind
a status occurred. Under the default text output the same refusal is one `rhino: …` line carrying the message alone. A
surface with no gates is not a refusal: `gate run` on it exits `0`. Under `--output json` it reports `"result":"clean"`;
under text output it prints nothing.

## The vocabulary

Every code is `rhino.<area>.<reason>`. RHINO emits no code outside this list.

- `rhino.args.unrecognized` — An unrecognized command, option, or option value, such as a `--surface` this build does
  not know or a `--base` that is not a commit ID.
- `rhino.args.incomplete` — An option the command needs is absent, an option is missing its value or the partner it
  needs, or two options contradict each other.
- `rhino.input.unreadable` — Standard input was selected and could not be read, or does not hold what the command reads
  from it.
- `rhino.repository.unusable` — The selected root is missing or is not a directory, or cannot answer what the command
  needs from it, such as a Git index or commit range.
- `rhino.config.unusable` — `repo-config.yml` is missing, unreadable, malformed, or inconsistent, such as a declared
  path or pattern that is absolute or climbs out of the root.
- `rhino.config.undeclared` — The configuration omits the group or policy the command needs.
- `rhino.path.escapes-root` — A `--file` or `--directory` selection is absolute, climbs out of the selected root, or
  passes through a symbolic link; a `--dir` or `--entry` is absolute or climbs out; a declared surface glob reaches a
  symbolic link that leads outside the root; or a declared environment example or staging allowance is not an exact
  repository-relative path.
- `rhino.file.missing` — A named file or directory does not exist.
- `rhino.file.unreadable` — A file RHINO had to read exists and could not be read, or lies behind a symbolic link RHINO
  does not follow; a declared surface glob reaches a symbolic link that loops or leads to nothing; or a file
  `governance quality-gates validate` was reading vanished.
- `rhino.file.exists` — A file that had to be created is already there.
- `rhino.file.unwritable` — A file that had to be written could not be written, including one whose path passes through
  a symbolic link.
- `rhino.gate.child-not-found` — A declared gate child was not found.
- `rhino.gate.child-not-executable` — A declared gate child was found and could not be executed.
- `rhino.gate.child-refused` — A declared gate child was refused for any other reason.
- `rhino.gate.undeclared` — A selected gate declares no executable command.
- `rhino.toolchain.failed` — A declared toolchain operation did not complete. An applied provision child that cannot be
  found or executed exits `127` or `126` under this code.
- `rhino.toolchain.undeclared` — A declared toolchain has no provision declaration for this platform.
- `rhino.harness.refused` — A declared harness operation was refused. A `harness adapters generate` that has written its
  adapters and still finds a generated marker it does not own is refused with this code too, and its message names each
  marker.
- `rhino.vault.unavailable` — The vault cannot be used: no session key is set, the vault is locked, it serves another
  server, a `bw` call does not complete, or an attachment cannot be staged in a temporary file for the provider. The
  `bw` executable is not on the path or cannot be executed; that exits `127` or `126`, and the other causes exit `2`. A
  session check that fails runs before any write, so it writes nothing. A call that fails during apply can leave earlier
  writes in place; see [findings](findings.md#vault-transfer).
- `rhino.vault.refused` — The vault answered something this build does not understand, or a value cannot be read or
  written safely: a `bw status`, `bw list items`, `bw get item`, `bw create item`, or `bw create attachment` answer that
  does not decode, a `bw get item` answer that is `null`, an attachment download that is not exactly one regular file, a
  login whose password the vault holds as null, a login value with a line break under `env pull`, or an uploaded
  generation that does not read back. A session check that refuses leaves the vault's state as it was. A refusal raised
  during apply can leave earlier writes in place; see [findings](findings.md#vault-transfer) and the vault how-to.

Error codes are public identifiers, and a caller may branch on one. Adding a code moves the minor version, because a
caller may treat the vocabulary as closed. Renaming or removing a code moves the contract: it needs a major version, or
a minor version while RHINO stays in `0.x`.

## Related

- [Exit codes](./exit-codes.md)
- [JSON output](./json-output.md)
