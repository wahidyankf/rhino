# Exit codes

RHINO uses three exit codes and no others. A caller reads the code without
knowing which subcommand produced it.

| Code | Meaning                                             | What to do                                          |
| ---- | --------------------------------------------------- | --------------------------------------------------- |
| `0`  | Checked and clean                                   | Nothing. The run happened and found nothing.        |
| `1`  | The repository violates its declared policy         | Read the findings on stderr and fix the repository. |
| `2`  | The invocation, root, or configuration was unusable | Fix the command or `repo-config.yml`. Nothing ran.  |

The distinction that matters is between `1` and `2`. **Exit `1` always means
the repository broke a rule it declared for itself.** Exit `2` means RHINO
never got as far as forming an opinion — the command was misspelled, the root
had no configuration, the configuration could not be parsed, or a selected file
could not be read.

A gate that treats every non-zero code the same will tell a maintainer to fix a
document when the real problem is a typo in a flag.

## Which situations produce which code

`0` — every declared surface was inspected and nothing violated policy. A
surface that matched no files still exits `0`; the run reports what it scanned
so an empty match is visible rather than silent.

`1` — one or more findings. Findings go to stderr, one per line, each prefixed
with the command category. stdout still carries the summary.

`2` — any of:

- an unrecognized command or option;
- a flag a leaf does not accept, such as `--harness` on a Markdown command;
- `--output` with a value other than `text` or `json`;
- a `--root` that holds no `repo-config.yml`;
- a `repo-config.yml` that is missing, unreadable, declares no schema, declares
  an unrecognized schema, is malformed, or is semantically inconsistent;
- a `--file` or `--directory` that names something RHINO cannot read.

On exit `2` stdout is **empty**. A caller parsing stdout can rely on it holding
a result or holding nothing, never a summary of a run that did not happen.

## Reading the code in a script

```sh
rhino governance word-budget validate
case $? in
  0) ;;
  1) echo "policy violation; see stderr" >&2; exit 1 ;;
  2) echo "RHINO could not run; check the invocation and repo-config.yml" >&2; exit 2 ;;
esac
```

## Related

- [How to respond to an exit code](../how-to/respond-to-exit-codes.md)
- [What a finding means](../explanation/what-a-finding-means.md)
- [JSON output](./json-output.md), where `exitCode` is also a field
