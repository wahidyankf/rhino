# Exit codes

RHINO reports from one closed vocabulary. Every status below means the same thing whichever subcommand produced it, and
RHINO never invents a status outside this set.

| Code    | Meaning                                             | What to do                                          |
| ------- | --------------------------------------------------- | --------------------------------------------------- |
| `0`     | Checked and clean                                   | Nothing. The run happened and found nothing.        |
| `1`     | A finding, a vault finding, or a failed gate child  | Read the findings or gate output and fix the cause. |
| `2`     | The invocation, root, or configuration was unusable | Fix the command or `repo-config.yml`. Nothing ran.  |
| `126`   | A required child was found but cannot be executed   | Fix that child's permissions. See below.            |
| `127`   | A required child was not found                      | Fix that child's path or install it. See below.     |
| `128+N` | Ended by signal `N`; `130`, `143`, or `141`         | Nothing ran to completion. See below.               |

The distinction that matters is between `1` and `2`. **Exit `1` means a finding: a validator reported that the
repository breaks a rule its configuration declares, or `env pull`, `env push`, or `env diff` reported a vault
finding.** `gate run` also exits `1` when a declared child fails, which is the one way `1` can come from a command's
exit status rather than from a rule RHINO checked. Exit `2` means RHINO never got as far as forming an opinion — the
command was misspelled, the root had no configuration, the configuration could not be parsed, or a selected file could
not be read.

A gate that treats every non-zero code the same will tell a maintainer to fix a document when the real problem is a typo
in a flag.

`126` and `127` draw the same line one step further out, and only three required children can produce them: a gate child
under `gate run`, the Bitwarden CLI under `env pull`, `env push`, or `env diff`, and a provision child under
`toolchain provision --apply`. A child that ran and reported something says one thing about the repository; a child that
never started says nothing at all, and reporting the second as the first would let a broken hook read as a caught
violation. They are the statuses a shell already reports for these two situations, so a caller needs no RHINO-specific
vocabulary to act on them.

## Which situations produce which code

`0` — every declared surface was inspected and nothing violated policy. A surface that matched no files still exits `0`;
the run reports what it scanned so an empty match is visible rather than silent.

`1` — one or more findings. Findings go to stderr, one per line, each prefixed with the command category. stdout still
carries the summary.

`1` from `gate run` — a check child that exits non-zero or ends by a signal, or a mutation child that exits non-zero. On
the `pull-request` surface, a mutation gate's `verify-clean` replay also exits `1` when the replay changes a selected
file. On `pre-commit`, the `apply-index` run applies such a change and the gate passes with exit `0`. For each failure,
RHINO writes `[gate] <id> failed` to stdout and one `reported a finding` line to stderr, and never repeats the child's
output. Under `--output json` the result reports one finding for the gate.

`1` from `env pull`, `env push`, or `env diff` — one or more vault findings, such as a vault item that is missing, or a
local file that differs from the vault. Each finding names an entry path or a key, never a value, and `env pull` writes
nothing for a flagged entry. A vault session, server, manifest, or answer RHINO does not understand exits `2`, never
`1`.

`2` — any of:

- an unrecognized command or option;
- no command at all, which prints one line on stderr pointing to `rhino --help`;
- a flag a leaf does not accept, such as `--dir` on a Markdown command;
- `--output` with a value other than `text` or `json`;
- a `--root` that holds no `repo-config.yml`;
- a `repo-config.yml` that is missing, unreadable, declares no schema, declares an unrecognized schema, is malformed, or
  is semantically inconsistent;
- a group or policy the command needs that the configuration omits;
- a `--file` or `--directory` that does not exist, cannot be read, or leaves the repository root, including through a
  symbolic link;
- a file the run has to read that exists and cannot be opened;
- a symbolic link a declared surface glob reaches that leads outside the repository root, loops back into itself, or
  leads to nothing;
- a file the run has to write that cannot be written;
- a `harness adapters generate` that wrote its own adapters and then finds a generated marker it does not own
  (`rhino.harness.refused`); validation reports the same marker as a finding, `1`, and writes nothing;
- a vault command whose session is missing, whose vault is locked, or whose server is not the configured one
  (`rhino.vault.unavailable`), or whose vault answer it does not understand (`rhino.vault.refused`);
- a vault manifest that breaks its rules, an `--entry` that names no declared vault entry, or a local env line that
  cannot be read;
- a local file that holds no UTF-8 text, which `env pull`, `env push`, and `env diff` refuse even with `--force`;
- a gate child that was refused for any reason other than the two below, including a mutation child that ends by a
  signal;
- an uncaught panic. RHINO has no internal-error code and does not recover from a panic: the Go runtime ends the process
  with status `2`, prints the panic message and a goroutine trace on stderr, and writes no error document, even under
  `--output json`.

`126` — a required child exists and could not be executed, because it is not executable. The required children are:

- a gate child in the selected surface's sequence, whose `command.executable` is not executable, under `gate run`;
- the Bitwarden CLI (`bw`), under `env pull`, `env push`, or `env diff`;
- a provision child declared for `toolchain provision --apply`.

`127` — a required child from the same three sources could not be found at all.

For a gate child, the gates before it in declaration order already ran; the ones after it did not. For a provision
child, the children declared before it already ran. `env` refuses before any vault write, so a `bw` that cannot start at
that check changes neither the repository nor the vault. A `bw` that disappears mid-run can exit `127` or `126` after
earlier writes, because a vault run is not transactional. The error code under `--output json` does not change with
these statuses: a gate child reports `rhino.gate.child-not-found` or `rhino.gate.child-not-executable`, a `bw` reports
`rhino.vault.unavailable`, and a provision child reports `rhino.toolchain.failed`. A `toolchain validate` probe that
cannot start is not a required child; it stays a finding, `1`.

`128+N` — RHINO was ended by signal `N` and reports `128+N`, never a value from the result range. `130` is `SIGINT`,
`143` is `SIGTERM`, and `141` is a closed pipe: a reader closed stdout or stderr, which the operating system reports as
a broken pipe. A run that any of them ends has reported nothing. A gate child that ends by a signal is never reported as
`128+N`: a check child is `1`, and a mutation child is `2`, as above.

Whenever RHINO refuses, stdout is **empty**. A caller parsing stdout can rely on it holding a result or holding nothing,
never a summary of a run that did not happen. Under `--output json` the refusal is a single document on stderr:

```json
{
  "schemaVersion": 1,
  "error": { "code": "rhino.gate.child-not-found", "message": "…" }
}
```

The `code` is namespaced and stable; the `message` is for a human and is not.

## Reading the code in a script

```sh
status=0
rhino governance word-budget validate || status=$?
case $status in
  0) ;;
  1) echo "policy violation; see stderr" >&2; exit 1 ;;
  2) echo "RHINO could not run; check the invocation and repo-config.yml" >&2; exit 2 ;;
  *) exit "$status" ;;
esac
```

The last arm passes `126`, `127`, and `128+N` through unchanged rather than translating them.

## Related

- [How to respond to an exit code](../how-to/respond-to-exit-codes.md)
- [What a finding means](../explanation/what-a-finding-means.md)
- [JSON output](./json-output.md), where `exitCode` is also a field
