# Command line

Eight commands. Every one reads `repo-config.yml` from the repository root
except `version`, which reports the build.

```console
$ rhino --help
rhino -- Repository Hygiene & INtegration Orchestrator

Every value it enforces is declared in the repository's repo-config.yml.

Commands:
  rhino repo-config validate                  Check that repo-config.yml is complete and usable.
  rhino governance word-budget validate       Check every declared surface against its declared word budget.
  rhino governance directory-map validate     Check that every mapped tree's READMEs list their siblings.
  rhino harness parity validate               Reconcile the canon against every declared coding harness.
  rhino md internal-link validate             Check that every local Markdown link resolves inside the repository.
  rhino md mermaid validate                   Check Mermaid diagrams for label length and colour contrast.
  rhino md word-count inspect                 Report a file's word count. Never reports findings.
  rhino version                               Report this build's release identity.

Options:
  --root <path>          The repository to inspect. Defaults to the working directory.
  --output <text|json>   How to render the result. Defaults to text.
  --quiet, --verbose, --no-color
                         Accepted everywhere; presentation only.
  --file <path>          Inspect these paths instead of the declared surface.
                         Repeatable; `-` reads standard input.
  --directory <path>     Inspect this directory instead of the declared trees.
  --harness <name>       Reconcile this harness instead of the whole roster.
  --json                 Shorthand for --output json.

Exit codes:
  0  checked and clean
  1  the repository violates its declared policy
  2  the invocation, root, or configuration was unusable
```

`--help` works at every level of the path, and the help you get is scoped to
the path you asked for: `rhino md --help` lists the three Markdown commands and
the options those commands accept, not all eight and not every option.

## Commands

### `rhino repo-config validate`

Checks that `repo-config.yml` is present, declares a schema this build
understands, parses, and is internally consistent. Run it first when anything
else exits `2`.

```console
$ rhino repo-config validate
[repo-config] checked 1 configuration file, no findings
```

### `rhino governance word-budget validate`

Counts words in every file matching a declared surface and reports the ones
over their declared limit. A clean run lists every path it read, because a
surface that matched nothing and a surface that matched everything both pass —
and only the listing tells them apart.

```console
$ rhino governance word-budget validate
[word-budget] checked 2 files, no findings
[word-budget] scanned AGENTS.md
[word-budget] scanned README.md
```

Accepts `--directory`? No. Accepts `--file`? No — use
`rhino md word-count inspect` to count one file.

### `rhino governance directory-map validate`

Within each declared tree, checks that every directory carries a `README.md`
with a `## Directory Map` section listing each direct sibling exactly once by a
resolvable relative path.

```console
$ rhino governance directory-map validate
[directory-map] checked 7 directories, no findings
```

`--directory <path>` inspects that one directory tree instead of the declared
ones. It replaces the declared trees rather than adding to them, and a path
that is not a directory is refused with exit `2`.

### `rhino harness parity validate`

Reconciles one canonical instruction body, one canonical skill set, and one
canonical agent set against every coding harness the repository declares. It
also reports a digest over everything it read, so a caller can tell "nothing
changed" from "nothing was checked".

```console
$ rhino harness parity validate
[harness-parity] checked 0 harnesses, no findings
[harness-parity] canon 0 harnesses, 0 skills, 0 agents, 0 reconciled capability declarations
[harness-parity] digest d9ba55f3310c2c8ea440e5c137eab428b4cb8f54365d6edc846c01b8ce038404
```

`--harness <name>` reconciles one declared harness instead of the roster. A
name the repository does not declare is refused with exit `2` — narrowing is a
smaller question, never a quieter answer to the same one.

### `rhino md internal-link validate`

Checks that every local Markdown link resolves to something inside the
repository. Fragments and links with a scheme are skipped: they address
something RHINO is not looking at.

```console
$ rhino md internal-link validate
[internal-link] checked 109 links, no findings
```

### `rhino md mermaid validate`

Checks every Mermaid diagram for label length and for colour drawn from the
declared palette.

```console
$ rhino md mermaid validate
[mermaid] checked 3 diagrams, no findings
```

`--file <path>` inspects those paths instead of the declared surface. It is
repeatable, and `-` reads a diagram from standard input.

### `rhino md word-count inspect`

Reports a file's word count. It **never** reports findings, so it never exits
`1`; it is the tool you reach for when you want the number rather than the
judgement.

```console
$ rhino md word-count inspect --file README.md
[word-count] checked 1 file, no findings
[word-count] README.md: 1593 words
```

A word is a run of letters, marks, and numbers, optionally joined by an
apostrophe, hyphen, or underscore to another such run — so `can't-stop` counts
as one word, not three.

### `rhino version`

```console
$ rhino version
0.1.0

$ rhino version --json
{"schemaVersion":1,"version":"0.1.0","commit":"5ef5ff83ef15eb45885210b9a6fa91419d1c6004"}
```

The commit is embedded at build time, so the hash above is the revision that
built the binary this page was written against and yours will differ. A build
made outside a repository reports forty zeros rather than lying about which
revision it is.

## Options

| Option                  | Accepted by                                    | Meaning                                                                                    |
| ----------------------- | ---------------------------------------------- | ------------------------------------------------------------------------------------------ |
| `--root <path>`         | every command                                  | The repository to inspect. Defaults to the working directory.                              |
| `--output <text\|json>` | every command                                  | How to render the result. Defaults to `text`.                                              |
| `--quiet`               | every command                                  | Presentation only. Accepted and ignored.                                                   |
| `--verbose`             | every command                                  | Presentation only. Accepted and ignored.                                                   |
| `--no-color`            | every command                                  | Presentation only. Accepted and ignored.                                                   |
| `--file <path>`         | `md mermaid validate`, `md word-count inspect` | Inspect these paths instead of the declared surface. Repeatable; `-` reads standard input. |
| `--directory <path>`    | `governance directory-map validate`            | Inspect this directory instead of the declared trees.                                      |
| `--harness <name>`      | `harness parity validate`                      | Reconcile this harness instead of the whole roster.                                        |
| `--json`                | `version`                                      | Shorthand for `--output json`.                                                             |

Two things about this table are worth stating plainly.

**`--json` is a `version` shorthand and nothing more.** Every other command
takes `--output json`. `rhino --help` lists the union of every option any
command accepts, so it shows `--json` without that qualification; the scoped
help — `rhino governance word-budget validate --help` — shows only what that
command actually takes, and is the one to trust.

**A flag a command does not accept is an error, not a no-op.**

```console
$ rhino governance word-budget validate --json
rhino: `--json` is not accepted by `governance word-budget validate`
```

`--root` may appear before or after the command path. It may not leave the
repository: a path containing `..` is refused.

## Related

- [Exit codes](./exit-codes.md)
- [Configuration](./configuration.md)
- [JSON output](./json-output.md)
