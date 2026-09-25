# Exit codes

RHINO reports from one closed vocabulary. Every status below means the same
thing whichever subcommand produced it, and RHINO never invents a status
outside this set.

| Code    | Meaning                                             | What to do                                            |
| ------- | --------------------------------------------------- | ----------------------------------------------------- |
| `0`     | Checked and clean                                   | Nothing. The run happened and found nothing.          |
| `1`     | The repository violates its declared policy         | Read the findings on stderr and fix the repository.   |
| `2`     | The invocation, root, or configuration was unusable | Fix the command or `repo-config.yml`. Nothing ran.    |
| `126`   | A gate child was found and could not be executed    | Fix the gate child's permissions. `gate run` only.    |
| `127`   | A gate child was not found                          | Fix the gate's `command.executable`. `gate run` only. |
| `128+N` | Ended by signal `N`                                 | Nothing; `130` is an interrupt, `141` a closed pipe.  |

The distinction that matters is between `1` and `2`. **Exit `1` always means
the repository broke a rule it declared for itself.** Exit `2` means RHINO
never got as far as forming an opinion — the command was misspelled, the root
had no configuration, the configuration could not be parsed, or a selected file
could not be read.

A gate that treats every non-zero code the same will tell a maintainer to fix a
document when the real problem is a typo in a flag.

`126` and `127` draw the same line one step further out, and only `gate run`
can produce them. A gate that ran and reported something says one thing about
the repository; a gate that never started says nothing at all, and reporting
the second as the first would let a broken hook read as a caught violation.
They are the statuses a shell already reports for these two situations, so a
caller needs no RHINO-specific vocabulary to act on them.

## Which situations produce which code

`0` — every declared surface was inspected and nothing violated policy. A
surface that matched no files still exits `0`; the run reports what it scanned
so an empty match is visible rather than silent.

`1` — one or more findings. Findings go to stderr, one per line, each prefixed
with the command category. stdout still carries the summary.

`2` — any of:

- an unrecognized command or option;
- no command at all, which prints one line on stderr pointing to
  `rhino --help`;
- a flag a leaf does not accept, such as `--dir` on a Markdown command;
- `--output` with a value other than `text` or `json`;
- a `--root` that holds no `repo-config.yml`;
- a `repo-config.yml` that is missing, unreadable, declares no schema, declares
  an unrecognized schema, is malformed, or is semantically inconsistent;
- a group or policy the command needs that the configuration omits;
- a `--file` or `--directory` that does not exist, cannot be read, or leaves
  the repository root, including through a symbolic link;
- a file the run has to read that exists and cannot be opened;
- a file the run has to write that cannot be written;
- a gate child that was refused for any reason other than the two below;
- an internal failure, which prints `rhino: internal error:` on stderr.

`126` — `gate run` only: a child in the selected surface's sequence exists and
could not be executed, because its `command.executable` is not executable.

`127` — `gate run` only: a child in the selected surface's sequence could not
be found at all. In both cases the gates before it in declaration order already
ran; the ones after it did not.

`128+N` — RHINO was ended by a signal and reports it by ending the same way,
rather than translating it into a value a caller could not tell apart from a
result. `130` is `SIGINT`, `141` a closed pipe.

Whenever RHINO refuses, stdout is **empty**. A caller parsing stdout can rely
on it holding a result or holding nothing, never a summary of a run that did
not happen. Under `--output json` the refusal is a single document on stderr:

```json
{
  "schemaVersion": 1,
  "error": { "code": "rhino.gate.child-not-found", "message": "…" }
}
```

The `code` is namespaced and stable; the `message` is for a human and is not.

## Reading the code in a script

```sh
rhino governance word-budget validate
case $? in
  0) ;;
  1) echo "policy violation; see stderr" >&2; exit 1 ;;
  *) echo "RHINO could not run; check the invocation and repo-config.yml" >&2; exit 2 ;;
esac
```

## Related

- [How to respond to an exit code](../how-to/respond-to-exit-codes.md)
- [What a finding means](../explanation/what-a-finding-means.md)
- [JSON output](./json-output.md), where `exitCode` is also a field
