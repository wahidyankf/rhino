# JSON output

Grouped-v2 validation leaves emit the result envelope below. Operations may
emit a command-specific [status document](#operation-status-documents); callers
must validate the selected leaf's contract rather than assuming every command
emits `violations`.

Every command accepts `--output json` and writes **one object on one line**.
Line-delimited rather than pretty-printed, so the output pipes through `grep`
and `jq` alike and so "every stdout line is a result" stays true whatever the
command reports.

```console
$ rhino md internal-link validate --output json
{"schemaVersion":1,"command":"internal-link","exitCode":0,"subject":"link","inspected":1,"scanned":[],"notes":[],"violations":[]}
```

A run with findings, shown wrapped for reading — the real output is one line:

```json
{
  "schemaVersion": 1,
  "command": "mermaid",
  "exitCode": 1,
  "subject": "diagram",
  "inspected": 1,
  "scanned": [],
  "notes": [],
  "violations": [
    {
      "kind": "mermaid-legibility",
      "path": "guides/diagram.md",
      "message": "a node label is longer than the declared limit",
      "line": 5,
      "segment": "node",
      "measured": 56,
      "limit": 32
    },
    {
      "kind": "mermaid-accessibility",
      "path": "guides/diagram.md",
      "message": "fill `#FF0000` is not a declared fill color",
      "line": 6
    }
  ]
}
```

## Validation result fields

| Field           | Type             | Meaning                                                                                                                                            |
| --------------- | ---------------- | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| `schemaVersion` | number           | `1`. Increments only for a breaking change to this shape.                                                                                          |
| `command`       | string           | The category, matching the `[prefix]` in text output.                                                                                              |
| `exitCode`      | number           | `0` or `1`. Mirrors the process exit code.                                                                                                         |
| `subject`       | string           | What was inspected, singular: `file`, `directory`, `link`, `diagram`, `artifact`, `configuration file`, `license file`, or `governance directory`. |
| `inspected`     | number           | How many of that subject were looked at. Reported even when zero.                                                                                  |
| `scanned`       | array of strings | The paths that were read, sorted, where the subject has one.                                                                                       |
| `notes`         | array of strings | Facts the run established that are not findings — a digest, a canon count.                                                                         |
| `violations`    | array of objects | One entry per finding, sorted by path, then line, then kind.                                                                                       |

A command may add named measurements as extra top-level number fields. They are
quantities the run established about the whole inspection, distinct from a
finding's details, which belong to one violation.

### Violation fields

| Field     | Type             | Present                                                |
| --------- | ---------------- | ------------------------------------------------------ |
| `kind`    | string           | always — the stable rule identifier                    |
| `path`    | string           | always — repository-relative                           |
| `message` | string           | always — prose, and the one field that may be reworded |
| `line`    | number           | when the finding has a position                        |
| others    | number or string | per kind; see [Findings](./findings.md)                |

**Filter on `kind`, never on `message`.** The kind is a contract; the message
is prose that exists to be improved.

## What JSON output guarantees

**stdout holds a result or holds nothing.** On exit `2` — a bad invocation, an
unreadable root, an unusable configuration — stdout is empty and the diagnostic
goes to stderr, in both formats. A caller can parse stdout without first
checking whether a run happened.

**Findings are on stderr in text output and inside the object in JSON.** In
JSON, stderr stays empty on exit `0` and `1`.

**Ordering is stable.** `scanned` and `violations` are sorted, so two runs over
one repository produce byte-identical output and a diff between two
repositories is a diff about the repositories.

## Operation status documents

The environment and toolchain operations report what they planned or did as one
object carrying `schemaVersion` `1`, `command`, and `status`. `status` is
`planned` or `completed`:

| Invocation                    | `status`    | Other fields                          |
| ----------------------------- | ----------- | ------------------------------------- |
| `env init`                    | `planned`   | `count`, `targets` (repository paths) |
| `env init --apply`            | `completed` | none                                  |
| `env backup`                  | `planned`   | `destination`                         |
| `env restore`                 | `completed` | none                                  |
| `toolchain provision`         | `planned`   | `count`, `toolchains` (IDs)           |
| `toolchain provision --apply` | `completed` | `count`                               |

```console
$ rhino env init --output json
{"command":"environment-init","count":1,"schemaVersion":1,"status":"planned","targets":[".env.generated"]}
```

## Using the validation result envelope

Count violations by kind across every validator:

```sh
for cmd in "repo-config validate" "md internal-link validate" \
           "md mermaid validate"; do
  rhino $cmd --output json
done | jq -r '.violations[].kind' | sort | uniq -c | sort -rn
```

List the files a validator actually read. `scanned` is filled where the
subject is a file; internal-link counts links, so its `scanned` is empty:

```sh
rhino governance word-budget validate --output json | jq -r '.scanned[]'
```

## Related

- [Findings](./findings.md) for every kind and its details
- [Exit codes](./exit-codes.md)
- [How to consume the JSON output](../how-to/consume-the-json-output.md)
