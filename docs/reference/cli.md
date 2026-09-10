# Command line

Thirteen commands. Every one reads `repo-config.yml` from the repository root
except `version`, which reports the build.

Five of them — `md naming`, `md frontmatter`, `md heading-hierarchy`,
`md readme-index`, and `convention emoji` — read a section that is **optional**.
A repository that has not declared the section gets exit `2` naming it, never a
convention RHINO chose on its behalf.

```console
$ rhino --help
rhino -- Repository Hygiene & INtegration Orchestrator

Every value it enforces is declared in the repository's repo-config.yml.

Commands:
  rhino repo-config validate                  Check that repo-config.yml is complete and usable.
  rhino governance word-budget validate       Check every declared surface against its declared word budget.
  rhino governance directory-map validate     Check that every mapped tree's READMEs list their siblings.
  rhino harness parity validate               Reconcile the canon against every declared coding harness.
  rhino md frontmatter validate               Check every declared surface's front matter against its declared schema.
  rhino md heading-hierarchy validate         Check every declared surface's heading structure.
  rhino md internal-link validate             Check that every local Markdown link resolves inside the repository.
  rhino md mermaid validate                   Check Mermaid diagrams for label length and colour contrast.
  rhino md naming validate                    Check every declared surface's filenames against its declared style.
  rhino md readme-index validate              Check that every directory in a declared tree carries a README.
  rhino md word-count inspect                 Report a file's word count. Never reports findings.
  rhino convention emoji validate             Check that no declared file carries an emoji code point.
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
the path you asked for: `rhino md --help` lists the seven Markdown commands and
the options those commands accept, not all thirteen and not every option.

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

### `rhino md frontmatter validate`

Checks each governed file's front matter against the schema its surface
declares: which keys must be present, which hold a value from a closed set,
which hold an ISO calendar date, and which may not appear at all.

```console
$ rhino md frontmatter validate
[frontmatter] checked 34 files, no findings
```

The section is optional. Undeclared, the command exits `2` naming
`md-frontmatter`.

Accepts `--file`? No. Accepts `--directory`? No.

### `rhino md heading-hierarchy validate`

Checks each governed file for the number of level-1 headings the repository
permits and for a heading dropping further below its predecessor than the
repository allows. A `#` inside a fenced block is a comment or an example, not a
heading.

```console
$ rhino md heading-hierarchy validate
[heading-hierarchy] checked 34 files, no findings
```

The section is optional. Undeclared, the command exits `2` naming
`md-heading-hierarchy`.

Accepts `--file`? No. Accepts `--directory`? No.

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

### `rhino md naming validate`

Checks each governed file's name against the style its surface declares. Two
styles: `kebab-case`, and `path-prefixed` — the file's own directory path,
encoded, then the declared separator, then a kebab-case content name. The
encoded prefix is derived from the path being walked, so a repository declares
the separator and nothing else.

```console
$ rhino md naming validate
[naming] checked 34 files, no findings
[naming] scanned repo-governance/README.md
```

The section is optional. Undeclared, the command exits `2` naming `md-naming`.

Accepts `--file`? No. Accepts `--directory`? No.

### `rhino md readme-index validate`

Checks that every directory under a declared tree carries a `README.md`.
Presence only, and deliberately weaker than
`governance directory-map validate`: a repository with a hundred READMEs and no
directory-map sections can adopt this rung today and the stronger one when it
has written them.

```console
$ rhino md readme-index validate
[readme-index] checked 22 directories, no findings
```

The section is optional. Undeclared, the command exits `2` naming
`md-readme-index`.

Accepts `--file`? No. Accepts `--directory`? No — a declared tree is the unit.

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

### `rhino convention emoji validate`

Checks that no file matching a declared glob carries an emoji code point. Only
the prohibition is checked, never the permission: whether an emoji belongs in a
particular sentence is a judgement about meaning, and a rule that guessed would
be worse than no rule. This is the one command that reads files a Markdown
corpus never sees, so it walks the repository itself.

```console
$ rhino convention emoji validate
[emoji] checked 9 files, no findings
```

The section is optional. Undeclared, the command exits `2` naming
`convention-emoji`.

Accepts `--file`? No. Accepts `--directory`? No.

### `rhino version`

```console
$ rhino version
v0.1.0

$ rhino version --json
{"schemaVersion":1,"version":"v0.1.0","commit":"d16fc0e7adc652ecdd61766096de0c35ba465cf5"}
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
