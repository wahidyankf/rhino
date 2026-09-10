# Command line

Nineteen commands. Every one reads `repo-config.yml` from the repository root
except `version`, which reports the build.

Two schemas answer that read. `rhino/repo-config/v1` is declared in a leading
comment and serves the twelve commands that shipped before `v0.3.0`;
`ose/repo-config/v2` is declared in a leading `schema:` key and serves the six
this release adds. A command reaches for exactly one of them and refuses by name
when handed the other, so a repository on either schema is told which commands
it can run rather than getting a default nobody declared. See
[configuration](./configuration.md).

Five of the v1 commands — `md naming`, `md frontmatter`, `md heading-hierarchy`,
`md readme-index`, and `convention emoji` — read a section that is **optional**.
A repository that has not declared the section gets exit `2` naming it, never a
convention RHINO chose on its behalf.

```console
$ rhino --help
rhino -- Repository Hygiene & INtegration Orchestrator

Every value it enforces is declared in the repository's repo-config.yml.

Commands:
  rhino repo-config validate                  Check that repo-config.yml is complete and usable.
  rhino gate run                              Run the gates the declared surface selects, in declaration order.
  rhino governance roots validate             Check the governance layers, their categories, and that each holds something.
  rhino governance companions validate        Check each companion directory's name, index, and ordinals.
  rhino governance instructions validate      Check the canonical instruction spine and the exact import beside it.
  rhino plan validate                         Check plan lifecycle, documents, companions, criteria, and delivery order.
  rhino governance word-budget validate       Check every declared surface against its declared word budget.
  rhino governance directory-map validate     Check that every mapped tree's READMEs list their siblings.
  rhino harness parity validate               Reconcile the canon against every declared coding harness.
  rhino md frontmatter validate               Check every declared surface's front matter against its declared schema.
  rhino md heading-hierarchy validate         Check every declared surface's heading structure.
  rhino metadata validate                     Check every declared surface against its path-selected metadata schema.
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
  --surface <name>       Which surface is dispatching: commit-msg, pre-commit,
                         pre-push, or ci. Arguments after `--` reach every child.
  --json                 Shorthand for --output json.

Exit codes:
  0  checked and clean
  1  the repository violates its declared policy
  2  the invocation, root, or configuration was unusable
  3  a gate child could not be started
```

`--help` works at every level of the path, and the help you get is scoped to
the path you asked for: `rhino md --help` lists the seven Markdown commands and
the options those commands accept, not all nineteen and not every option.

## Commands

### `rhino repo-config validate`

Checks that `repo-config.yml` is present, declares a schema this build
understands, parses, and is internally consistent. Run it first when anything
else exits `2`.

```console
$ rhino repo-config validate
[repo-config] checked 1 configuration file, no findings
```

### `rhino gate run`

Runs the gates the declared surface selects, in the order `gates` declares them,
and stops at the first failure. Each child is started directly — no shell — with
`OSE_GATE_SURFACE` set to the surface that selected it and anything after `--`
appended to its own argument vector. Reads `ose/repo-config/v2`. Requires
`--surface`.

```console
$ rhino gate run --surface pre-commit
[gate] hygiene passed
```

A child that ran and reported something exits `1`:

```console
$ rhino gate run --surface pre-commit
[gate] hygiene failed
[gate] hygiene reported a finding at pre-commit
```

A child that never started exits `3`, because a gate that did not run says
nothing about the repository and reporting that as a finding would let a broken
hook read as a caught violation:

```console
$ rhino gate run --surface pre-commit
[gate] hygiene could not run
rhino: gate `hygiene`: `./gates/hygiene.sh` could not be started: No such file or directory (os error 2)
```

Only the gate identifier and a sanitized status are printed. A child's own
streams are never repeated: a gate exists to look at content that may not be
publishable, and a runner that echoed what it found would publish it on the way
to saying it should not be.

### `rhino governance roots validate`

Checks that every directory under `repo-governance/` is a canonical layer or a
category the repository declared under `governance.local-categories`, and that
no governed directory is empty. Reads `ose/repo-config/v2`. Finding kinds:
`unknown-governance-layer`, `undeclared-governance-category`,
`unused-local-category`, `empty-governed-directory`.

```console
$ rhino governance roots validate
[governance-roots] checked 2 directories, no findings
```

### `rhino governance companions validate`

Checks each companion set: a suffix-free directory named after the document it
carries, an index in that document linking every companion, no live sibling the
index omits, and contiguous ordinals when the set is ordered. Reads
`ose/repo-config/v2`. Finding kinds: `suffixed-companion-directory`,
`missing-companion-index`, `missing-indexed-companion`,
`unindexed-companion-module`, `non-contiguous-companion-ordinals`,
`ordinal-in-unordered-set`, `unordered-module-in-ordered-set`.

```console
$ rhino governance companions validate
[governance-companions] checked 0 companion sets, no findings
```

### `rhino governance instructions validate`

Checks that `AGENTS.md` opens with the five spine sections in one order and that
`CLAUDE.md`, if it exists, is exactly `@AGENTS.md`. An absent `CLAUDE.md` is
legal: a repository with no Claude surface has no adapter to keep honest. Reads
`ose/repo-config/v2`. Finding kinds: `missing-canonical-instruction`,
`missing-spine-section`, `misordered-spine-section`,
`interrupted-instruction-spine`, `inexact-instruction-import`.

```console
$ rhino governance instructions validate
[governance-instructions] checked 2 instructions, no findings
```

### `rhino plan validate`

Checks the shape of every plan under `plans/`: the lifecycle root and slug form,
the six required documents, exactly one technical shape, companion ordinals,
acceptance identifiers declared in `prd.md` and referenced by `delivery.md`, and
the delivery order. `plans/done/` and `plans/ideas/` are held to their lifecycle
form and nothing else. Reads `ose/repo-config/v2`. Twenty rule identifiers, in
five families: `PLAN-LIFECYCLE-001` to `-005`, `PLAN-DOCUMENT-001` to `-003`,
`PLAN-COMPANION-001` to `-006`, `PLAN-CRITERION-001` and `-002`, and
`PLAN-DELIVERY-001` to `-004`. The identifiers are frozen in a contract more
than one implementation validates against, so a rule whose meaning changes gets
a new identifier rather than a new definition.

Structure only. Whether the writing is clear or the approach is sound is not
visible in the shape, and a rule that guessed at it would be enforced
confidently in cases nobody considered.

```console
$ rhino plan validate
[plan] checked 1 plan, no findings
```

### `rhino metadata validate`

Checks each declared surface's front matter against the schema its path selects:
`governance`, `workflow`, `skill`, or `agent`. Reads `metadata` in
`rhino/repo-config/v1`. Twenty-three finding kinds, all prefixed `metadata-`;
[findings](./findings.md) lists them.

```console
$ rhino metadata validate
[metadata] checked 1 file, no findings
[metadata] scanned repo-governance/conventions/file-naming.md
```

```console
$ rhino metadata validate
[metadata] checked 1 file, 1 finding
[metadata] scanned repo-governance/conventions/file-naming.md
[metadata] repo-governance/conventions/file-naming.md:3:1 metadata-unknown-key name is not a key this artifact's schema declares; the path already supplies every identity the schema omits
```

That `path:line:column kind field message` line is the structural diagnostic
format, and it is deliberately not the format the validators that shipped before
`v0.3.0` use: a consumer's stored output may not change because a new command
arrived.

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
| `--surface <name>`      | `gate run`                                     | Which surface is dispatching: `commit-msg`, `pre-commit`, `pre-push`, or `ci`. Required.   |
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

**`--` belongs to `gate run` alone.** Everything after it is appended to every
child's own argument vector, which is how a hook forwards the arguments Git gave
it. Nothing is interpreted by a shell on the way.

## Related

- [Exit codes](./exit-codes.md)
- [Configuration](./configuration.md)
- [JSON output](./json-output.md)
