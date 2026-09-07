# Configuration

Every value RHINO enforces arrives from `repo-config.yml` at the repository
root. **The tool ships no default for any of them.** A repository that declares
nothing gets a configuration error, never a borrowed assumption from whichever
repository the validator grew up in.

The first line declares the schema, in a comment:

```yaml
# schema: rhino/repo-config/v1
```

`rhino-cli/repo-config/v1` is accepted as a predecessor spelling. Anything else
is refused with exit `2`, naming what was declared and what this build
understands.

Sections are named after the command path that reads them, with spaces replaced
by hyphens, so a reader can find a section from a command and back again.
**Unknown sections are ignored** — another tool may keep its policy in the same
file. **Unknown keys inside a section RHINO owns are refused**, because there a
typo is a policy that silently does nothing.

## A complete file

This is RHINO's own configuration, and the whole schema:

```yaml
# schema: rhino/repo-config/v1

governance-word-budget:
  surfaces:
    - glob: "AGENTS.md"
      fail: 1200
      warn: 1000
    - glob: "README.md"
      fail: 1600
      warn: 1400

governance-directory-map:
  trees:
    - path: docs
    - path: specs

md-internal-link:
  exclude-sources: []

md-mermaid:
  node-label-graphemes: 32
  edge-label-graphemes: 24
  fill-colors: ["#0173B2", "#DE8F05", "#029E73", "#CA9161"]
  edge-colors: ["#0173B2", "#000000"]
  text-colors: ["#000000", "#FFFFFF"]

harness-parity:
  canonical:
    instruction: AGENTS.md
    instruction-adapter: CLAUDE.md
  harnesses: []
  prohibited-instruction-sources:
    - "**/GEMINI.md"
    - "**/.cursorrules"
  capabilities: []
  constraints: []

scan:
  exclude-directories:
    - .git
    - node_modules
    - target
```

## `governance-word-budget`

| Key        | Required | Meaning                                           |
| ---------- | -------- | ------------------------------------------------- |
| `surfaces` | yes      | Ordered list of governed globs and their budgets. |

Each surface takes `glob` and `fail`, and optionally `warn` and `target`.
`fail` is inclusive: a file exactly at the declared count passes, and strictly
above it is a finding.

**The list is ordered and the last matching entry wins**, which is how a
specific surface overrides a general tree:

```yaml
surfaces:
  - glob: "docs/**/*.md"
    fail: 900
  - glob: "docs/reference/**/*.md"
    fail: 2500
```

## `governance-directory-map`

| Key     | Required | Meaning                                     |
| ------- | -------- | ------------------------------------------- |
| `trees` | yes      | The directory trees whose maps are checked. |

Each tree takes a `path`. Every directory at or under it must carry a
`README.md` with a `## Directory Map` section.

## `md-internal-link`

| Key               | Required | Meaning                           |
| ----------------- | -------- | --------------------------------- |
| `exclude-sources` | no       | Globs excluded as link _sources_. |

Files under an excluded glob are not read for links. They remain valid link
_targets_ — excluding a source says "do not check the links in this file", not
"this file does not exist".

## `md-mermaid`

| Key                    | Required | Meaning                                           |
| ---------------------- | -------- | ------------------------------------------------- |
| `node-label-graphemes` | yes      | Longest legible node or state label segment.      |
| `edge-label-graphemes` | yes      | Longest legible edge or transition label segment. |
| `fill-colors`          | yes      | Every fill a `classDef` may set.                  |
| `edge-colors`          | yes      | Every stroke a `classDef` may set.                |
| `text-colors`          | yes      | Every text colour a `classDef` may set.           |

Limits are per segment, not per label: a `<br>` splits a label into lines a
reader sees separately.

## `harness-parity`

| Key                              | Required                | Meaning                                                     |
| -------------------------------- | ----------------------- | ----------------------------------------------------------- |
| `canonical.instruction`          | yes                     | The one always-on instruction body.                         |
| `canonical.instruction-adapter`  | no                      | A file that may contain only the import of the instruction. |
| `canonical.skills-root`          | no                      | Where canonical skills live.                                |
| `canonical.skill-route`          | with `skills-root`      | What a wrapper carries in place of the skill.               |
| `canonical.agents-root`          | no                      | Where canonical agents live.                                |
| `canonical.agent-route`          | with `agents-root`      | What an adapter carries in place of the prompt.             |
| `harnesses`                      | yes, and may be empty   | The coding harnesses to reconcile.                          |
| `prohibited-instruction-sources` | yes                     | Globs that may not be always-on instruction sources.        |
| `capabilities`                   | yes, and may be empty   | The vocabulary an agent may draw capabilities from.         |
| `constraints`                    | yes, and may be empty   | The vocabulary an agent may draw constraints from.          |
| `required-mcp`                   | with every `capability` | The server every harness must declare identically.          |

`harnesses` is required even when empty, and empty is a legal, explicit
declaration. The difference between "this repository has no coding harnesses"
and "I forgot to configure harnesses" has to stay visible.

**Nothing is required by the roster; everything is required by its pair.** A
repository with harnesses may have no canonical skills, no canonical agents, and
no capability server — each is a thing a repository may genuinely not have, and
insisting on any of them would be asserting one repository's arrangement as
everyone's. `skills-root`, `agents-root`, and `required-mcp` are still
**refused** alongside an _empty_ roster: a canon with nowhere to be reconciled
is a reconciliation that was silently skipped, not a clean pass.

What each does require is the rest of its own declaration:

| Declare this   | And you must also declare                              |
| -------------- | ------------------------------------------------------ |
| `agents-root`  | `agent-route`, and an `agent-adapter` on every harness |
| `skills-root`  | `skill-route`                                          |
| `required-mcp` | a `capability` on every harness                        |

Each half alone is a rule nothing enforces: a root with no route leaves every
adapter's body unchecked, an adapter contract with no root behind it reconciles
nothing, and a server no harness is checked against enforces nothing.

Each harness takes:

| Key             | Required            | Meaning                                             |
| --------------- | ------------------- | --------------------------------------------------- |
| `name`          | yes                 | How the harness is named in output and `--harness`. |
| `agent-adapter` | with `agents-root`  | How this harness expresses the canonical agents.    |
| `skill-adapter` | no                  | How it expresses the canonical skills.              |
| `capability`    | with `required-mcp` | `file` and `format` (`toml` or `json`).             |

## Adapter contracts

An adapter is a **route**, not a copy. It points at the canonical document and
translates the canon's capabilities into the permission vocabulary its own
harness understands — which is what every coding harness actually does, and why
a schema that made adapters repeat the canonical prompt could describe none of
them.

| Key            | Required | Meaning                                                           |
| -------------- | -------- | ----------------------------------------------------------------- |
| `path`         | yes      | Where the adapter lives, with `{name}` standing for the document. |
| `format`       | yes      | `front-matter` or `toml`.                                         |
| `route-field`  | yes      | `body`, or the name of a field holding the route.                 |
| `identity`     | no       | Adapter field to the canonical property it must equal.            |
| `fixed`        | no       | Fields that must hold exactly this value.                         |
| `absent`       | no       | Fields that may not appear at all.                                |
| `closed`       | no       | The declaration may carry nothing beyond the rules above.         |
| `translations` | no       | How a canonical capability becomes this harness's own permission. |

`path` is a pattern rather than a directory and an extension because both
shapes are real: a file per document (`.claude/agents/{name}.md`) and a
directory per document (`.claude/skills/{name}/SKILL.md`).

Each translation says **when** it applies and **what** it then obliges:

| Key                            | Meaning                                                     |
| ------------------------------ | ----------------------------------------------------------- |
| `when`                         | `always`, `requires`, `denies`, or `constrains`.            |
| `capability`                   | The canonical name that triggers it. Omitted for `always`.  |
| `field`                        | The adapter field the obligation is about.                  |
| `members` / `absent-members`   | Members the field must, or must not, contain.               |
| `member-prefix`                | At least one member beginning with this.                    |
| `entries`                      | Keys the field must map to exactly these values.            |
| `entry-prefix` / `entry-value` | At least one key beginning with this, mapped to that value. |

A field read for members takes a sequence as its items and a scalar as its
comma-separated parts, so a harness writing `tools: Read, Grep` and one writing
a list are read the same way.

The check is **non-weakening, not equality**. An adapter may grant more than the
canon requires; it may not grant less, and it may not grant what the canon
denies. A capability the canon never asked for triggers nothing at all.

## `scan`

| Key                   | Required | Meaning                                  |
| --------------------- | -------- | ---------------------------------------- |
| `exclude-directories` | yes      | Directory _names_, matched at any depth. |

Filesystem links are always skipped regardless of this list, because following
one can escape the repository.

## When it is wrong

`rhino repo-config validate` is the command that tells you. Every configuration
fault is exit `2` and names a line. Replace the empty roster in the file above
with a harness, and leave the canonical roots out:

```yaml
harnesses:
  - name: claude
    agent-adapter:
      path: ".claude/agents/{name}.md"
      format: front-matter
      route-field: body
    capability:
      file: .mcp.json
      format: json
```

```console
$ rhino repo-config validate
[repo-config] repo-config.yml: line 31: agent-adapter: `claude` declares an agent adapter, but no `agents-root` names what it would express
```

Line 31 is the `harnesses:` line. A key that is absent has no line of its own,
so the fault is reported against the roster that holds the half that is
present — which is the line you would have to look at to decide whether to add
the root or drop the contract. Validation stops at the first semantic fault, so
fix this one and run it again: the `capability` block above will then be refused
in its turn, because nothing declares `required-mcp`.

None of them degrade to a partial run. A validator that skipped a tree because
its configuration was malformed would report a clean repository that was never
checked.

## Related

- [Command line](./cli.md)
- [Exit codes](./exit-codes.md)
- [Why RHINO holds no defaults](../explanation/holding-no-defaults.md)
