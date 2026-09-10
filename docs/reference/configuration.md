# Configuration

Every value RHINO enforces arrives from `repo-config.yml` at the repository
root. **The tool ships no default for any of them.** A repository that declares
nothing gets a configuration error, never a borrowed assumption from whichever
repository the validator grew up in.

## Two schemas

A build understands two, and a document declares exactly one of them.

```yaml
# schema: rhino/repo-config/v1
```

```yaml
schema: ose/repo-config/v2
```

They are told apart by _where_ the declaration is: v1 in a leading comment, v2
in a leading key. No document can be read as both, and one carrying both is a
stated fault rather than a coin toss. `rhino-cli/repo-config/v1` is accepted as
a predecessor spelling. Anything else is refused with exit `2`, naming what was
declared and what this build understands.

They are not versions of one document. v1 describes Markdown and harness
hygiene and serves the thirteen commands that shipped before `v0.3.0` plus
`metadata validate`; v2 describes portable governance and its gates and serves
the five commands `v0.3.0` adds. A command reaching for the schema it was not
given refuses by name rather than guessing:

```console
$ rhino plan validate
[plan] repo-config.yml: line 1: this rule is part of `ose/repo-config/v2`, and this repository declares `rhino/repo-config/v1`
```

Everything from here to [`scan`](#scan) describes v1. The v2 sections are
[at the end](#ose-repo-config-v2).

Sections are named after the command path that reads them, with spaces replaced
by hyphens, so a reader can find a section from a command and back again.
**Unknown sections are ignored** — another tool may keep its policy in the same
file. **Unknown keys inside a section RHINO owns are refused**, because there a
typo is a policy that silently does nothing.

Six sections are required. Seven — `md-naming`, `md-frontmatter`,
`md-heading-hierarchy`, `md-readme-index`, `convention-emoji`, `metadata`, and
`model-tiers` — are **optional**, and each is marked as such below. Omitting one is not a gap to be
filled in: the command that reads it exits `2` naming the missing section, so
"this validator has no policy here" and "this validator has a default policy
here" can never be confused.

## A complete file

This is RHINO's own configuration, plus the optional sections it does not
declare — so the block below is the whole schema rather than the whole file:

```yaml
# schema: rhino/repo-config/v1

governance-word-budget:
  count: letters-and-digits
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

# Optional, from here to the end of this comment block.
md-naming:
  surfaces:
    - glob: "docs/**/*.md"
      style: kebab-case
    - glob: "vault/**/*.md"
      style: path-prefixed
      separator: "__"
  exempt: ["**/README.md"]

md-frontmatter:
  surfaces:
    - glob: "docs/**/*.md"
      require: [title, category]
      enum:
        category: [tutorial, how-to, reference, explanation]
      iso-date: [last_updated]
    - glob: "specs/**/*.md"
      require: []
      forbid: [updated]

md-heading-hierarchy:
  surfaces:
    - glob: "docs/**/*.md"
  single-h1: true
  max-level-jump: 1

md-readme-index:
  trees:
    - path: docs

convention-emoji:
  prohibited:
    - glob: "**/*.json"
    - glob: "**/*.toml"
# Optional sections end here.

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
| `count`    | yes      | What this repository means by a word.             |
| `surfaces` | yes      | Ordered list of governed globs and their budgets. |

`count` is `letters-and-digits` or `whitespace-separated`, and there is no
default because two real repositories already disagree:

| Rule                   | A word is                                                                    | `**bold** https://x.dev/a` |
| ---------------------- | ---------------------------------------------------------------------------- | -------------------------- |
| `letters-and-digits`   | a run of letters, marks and digits, joined by `'`, `-` or `_` to another run | 5                          |
| `whitespace-separated` | anything between two runs of whitespace                                      | 2                          |

One repository's `AGENTS.md` is 749 words under the second rule and 905 under
the first. Picking either would enforce a budget that repository never set.

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

## `md-frontmatter`

**Optional.** Undeclared, `md frontmatter validate` exits `2` naming the
section.

| Key        | Required | Meaning                                  |
| ---------- | -------- | ---------------------------------------- |
| `surfaces` | yes      | Ordered globs. Last matching entry wins. |

Each surface takes `glob` and `require`, and optionally `enum`, `iso-date`, and
`forbid`.

| Surface key | Required              | Meaning                                         |
| ----------- | --------------------- | ----------------------------------------------- |
| `glob`      | yes                   | Which files this schema governs.                |
| `require`   | yes, and may be empty | Keys that must be present.                      |
| `enum`      | no                    | Keys whose value must come from a declared set. |
| `iso-date`  | no                    | Keys whose value must be an ISO calendar date.  |
| `forbid`    | no                    | Keys that may not appear.                       |

`forbid` is what lets a repository keep a rule RHINO knows nothing about —
"this tree does not carry a date" — without RHINO having to know what the rule
is for.

A block that opens and never closes is reported as itself rather than as a set
of missing keys: everything after the opener reads as front matter to the end of
the file, so no key in it can be trusted, and naming the keys would send a
maintainer to the wrong line.

Only top-level keys are read. A key's indented sequence items and nested
mappings belong to that key, so a nested `updated:` does not trip a rule about
the document's own keys. `enum` and `iso-date` fire only on a key that is
present; absence is `require`'s business, and reporting it twice would make one
fault read as two.

## `md-heading-hierarchy`

**Optional.** Undeclared, `md heading-hierarchy validate` exits `2` naming the
section.

| Key              | Required | Meaning                                                    |
| ---------------- | -------- | ---------------------------------------------------------- |
| `surfaces`       | yes      | Globs whose heading structure is governed.                 |
| `single-h1`      | yes      | Whether a governed file holds exactly one level-1 heading. |
| `max-level-jump` | yes      | How far a heading may drop below the one before it.        |

Each surface takes a `glob` and nothing else: this section's two rules are
stated once for all of them, so nothing depends on which surface matched.

`single-h1` has no default because a repository whose documents are sections of
a larger whole legitimately has none. `max-level-jump: 1` is the strict reading;
a larger number is a repository that has decided otherwise.

A `#` inside a fenced block is a shell comment, a Markdown example, or a colour
literal — not a heading. A run of hashes is a heading only when a space follows
it, so `#hashtag` is a word. The first heading in a document establishes the
level the rest are measured from: it cannot drop below something that is not
there, and a document that opens deep is `single-h1`'s business or nobody's.

## `md-internal-link`

| Key               | Required | Meaning                           |
| ----------------- | -------- | --------------------------------- |
| `exclude-sources` | no       | Globs excluded as link _sources_. |

Link syntax inside a fenced block or an inline code span is not a link. A
document explaining a convention writes `` `[name](name)` `` and means the
characters; reporting it would accuse the document of a broken link it never
made.

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

## `md-naming`

**Optional.** Undeclared, `md naming validate` exits `2` naming the section.

| Key        | Required              | Meaning                                  |
| ---------- | --------------------- | ---------------------------------------- |
| `surfaces` | yes                   | Ordered globs. Last matching entry wins. |
| `exempt`   | yes, and may be empty | Globs no style applies to.               |

Each surface takes `glob` and `style`, plus `separator` when the style is
`path-prefixed`.

| Style           | A filename is                                                                      |
| --------------- | ---------------------------------------------------------------------------------- |
| `kebab-case`    | lowercase alphanumeric runs joined by single hyphens                               |
| `path-prefixed` | the file's own directory path, encoded, then the separator, then a kebab-case name |

`separator` is required by `path-prefixed` and refused by `kebab-case`. Either
mismatch is exit `2`: a separator nothing consults reads as a rule that is in
force.

The encoded prefix is derived from the path being walked, so a repository
declares the separator and nothing else. Each directory level below the
surface's root contributes one encoded segment and the levels are joined by a
hyphen. A segment's hyphen-separated words each contribute two characters, or
one character and an underscore when the word is a single character:

| Directory below the root   | Encoded prefix |
| -------------------------- | -------------- |
| `people`                   | `pe`           |
| `docs/tutorials`           | `do-tu`        |
| `f-sharp`                  | `f_sh`         |
| `2025-annual-review`       | `20anre`       |
| `jobs/north-branch/events` | `jo-nobr-ev`   |

The surface's root is the fixed leading path of its glob: everything before the
first segment carrying a metacharacter. `vault/**/*.md` roots at `vault`, so
`vault/people/pe__ada-lovelace.md` is correctly named and `va-pe__…` is not.
Deriving the root from the glob rather than asking for it separately means the
two cannot disagree: a repository that re-rooted its surface re-rooted its
prefixes with it. A file directly at the root has no directory to encode, so it
carries a kebab-case name and no separator.

An exemption wins over every surface. A repository saying "not this file" has
said so about all of them at once.

## `md-readme-index`

**Optional.** Undeclared, `md readme-index validate` exits `2` naming the
section.

| Key     | Required | Meaning                                                |
| ------- | -------- | ------------------------------------------------------ |
| `trees` | yes      | The trees in which every directory must hold a README. |

Each tree takes a `path`, spelled exactly as `governance-directory-map` spells
it, because it is the same kind of statement.

This is deliberately weaker than `governance-directory-map`, which wants the
README **and** a `## Directory Map` section listing every direct sibling. Both
exist so a repository can say which rung it is on: one with a hundred READMEs
and no map sections can adopt this today and the stronger rule when it has
written them, instead of choosing between a hundred findings and no check at
all.

Presence, not readability. A README that is there and cannot be opened is
present, and telling a maintainer to write a file that already exists would be
the wrong instruction.

## `harness-parity`

| Key                              | Required                | Meaning                                                             |
| -------------------------------- | ----------------------- | ------------------------------------------------------------------- |
| `canonical.instruction`          | yes                     | The one always-on instruction body.                                 |
| `canonical.instruction-adapter`  | no                      | A file that may contain only the import of the instruction.         |
| `canonical.skills-root`          | no                      | Where canonical skills live.                                        |
| `canonical.skill-route`          | with `skills-root`      | What a wrapper carries in place of the skill.                       |
| `canonical.agents-root`          | no                      | Where canonical agents live.                                        |
| `canonical.agent-route`          | with `agents-root`      | What an adapter carries in place of the prompt.                     |
| `canonical.declaration`          | with `agents-root`      | Which field of a canonical agent holds which permission.            |
| `harnesses`                      | yes, and may be empty   | The coding harnesses to reconcile.                                  |
| `prohibited-instruction-sources` | yes                     | Globs that may not be always-on instruction sources.                |
| `prohibited-instruction-fields`  | no                      | Fields of a harness's own settings that may not carry instructions. |
| `capabilities`                   | yes, and may be empty   | The vocabulary an agent may draw capabilities from.                 |
| `constraints`                    | yes, and may be empty   | The vocabulary an agent may draw constraints from.                  |
| `required-mcp`                   | with every `capability` | The server every harness must declare identically.                  |

Each entry of `prohibited-instruction-fields` takes `file`, `format` (`json`
or `toml`), and `field` — a key read at the document's top level. The file
itself is never prohibited; it is that harness's legitimate settings. What is
prohibited is one key inside it carrying always-on instructions, which is the
same competing source as a second `CLAUDE.md` written in the vendor's own
syntax. A key nobody wrote, and a key written as an empty list, empty table,
empty string, or `null`, are both answers rather than violations. A file RHINO
cannot read in its declared format is reported instead: a source that cannot be
ruled out has not been ruled out.

`required-mcp` takes `name`, `command`, and `args`. RHINO looks for the server
by **name**, wherever the vendor nests it, and compares the whole executable
vector — `command` followed by `args` — however that vendor splits it. One
harness writes a scalar `command` beside an `args` array; another writes the
entire vector as `command`. Both name the same command line, and pinning either
split would put a vendor's syntax back in the binary.

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

## The canonical declaration

A canonical agent writes down what it may do, what it may not, and how it must
behave. The three lists have names, and **the names are yours** — one repository
writes `requires:` where another writes `capabilities:`.

| Key       | Required | Meaning                                                    |
| --------- | -------- | ---------------------------------------------------------- |
| `grants`  | yes      | The field naming what the agent may do.                    |
| `denials` | yes      | The field naming what it may not.                          |
| `limits`  | yes      | The field naming how it must behave.                       |
| `fixed`   | no       | Fields every canonical agent must carry, and their values. |

```yaml
declaration:
  grants: requires
  denials: denies
  limits: constraints
  fixed: { mode: subagent }
```

This is required alongside `agents-root` rather than defaulted, and the reason
is worth stating plainly: **a list read under a name nobody wrote comes back
empty, not missing**. A validator that chose these names itself would find no
capabilities on any agent, fire no translation, compare nothing, and report the
repository clean — a false pass rather than a false finding, and the only kind
of defect a gate cannot survive.

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

`closed` permits everything the contract itself names — the route field, the
identity and fixed fields, and **every field a translation is about**. A closed
set that excluded the last of those would report the very field the adapter is
obliged to declare.

Routes are compared as sentences, not as bytes: runs of whitespace collapse to
one space on both sides first. A route is prose, every Markdown formatter wraps
prose, and where a line breaks is not something an adapter can be said to have
got wrong.

The check is **non-weakening, not equality**. An adapter may grant more than the
canon requires; it may not grant less, and it may not grant what the canon
denies. A capability the canon never asked for triggers nothing at all.

## `convention-emoji`

**Optional.** Undeclared, `convention emoji validate` exits `2` naming the
section.

| Key          | Required | Meaning                                          |
| ------------ | -------- | ------------------------------------------------ |
| `prohibited` | yes      | Globs in which an emoji code point is a finding. |

Each entry takes a `glob`.

Only the prohibition is expressible, and deliberately: whether an emoji belongs
in a particular sentence is a judgement about meaning, and a validator that
guessed at it would be reporting on taste. Where an emoji is forbidden — a
configuration file, a shell script, a data document a machine reads — the
question has one answer.

This is the one section whose command reads files no Markdown corpus contains,
so it walks the repository itself. One finding per line rather than per code
point: three emoji on one line are one edit.

The code-point blocks are narrower than "every symbol". An em dash, an arrow, a
copyright sign, and a plus-minus sign all live just outside them, because every
one of those appears in ordinary prose that this validator must not accuse.

## `metadata`

**Optional.** The surfaces whose front matter is held to a canonical schema, and
which schema each one uses.

| Key        | Required | Meaning                                        |
| ---------- | -------- | ---------------------------------------------- |
| `surfaces` | yes      | Ordered list; the last matching `glob` wins.   |
| `glob`     | yes      | Which files this row selects.                  |
| `schema`   | yes      | `governance`, `workflow`, `skill`, or `agent`. |

```yaml
metadata:
  surfaces:
    - glob: "repo-governance/**/*.md"
      schema: governance
    - glob: ".agents/agents/*.md"
      schema: agent
```

The schema is selected by path rather than declared in the document, because the
path already supplies every identity the schema omits. A document under two rows
is held to the last one, the same way word budgets resolve.

## `model-tiers`

**Optional.** The same portable tier map v2 declares, available to v1 so the
`agent` metadata schema can check that a tier a document names is one this
repository declared. The tiers are a closed set: `ultra`, `plan`, `execution`,
`fast`.

## `scan`

| Key                   | Required | Meaning                                  |
| --------------------- | -------- | ---------------------------------------- |
| `exclude-directories` | yes      | Directory _names_, matched at any depth. |

Filesystem links are always skipped regardless of this list, because following
one can escape the repository.

## `ose/repo-config/v2`

The portable governance schema. Six top-level keys, in this order, of which
`schema`, `visibility`, and `gates` are required:

```yaml
schema: ose/repo-config/v2
visibility: private
governance:
  local-categories:
    - development/practice
model-tiers:
  claude:
    ultra: { model: claude-opus, effort: high }
gates:
  - id: hygiene
    kind: check
    run:
      - ./gates/hygiene.sh
    surfaces:
      - commit-msg
      - pre-commit
      - pre-push
extensions:
  acme/anything: {}
```

**Order is checked.** A document whose keys arrive out of that order is refused,
because a schema that tolerated any order would make two repositories that
declare the same policy look different to a reader.

### `visibility`

`public` or `private`. Not inferred: an offline validator cannot learn hosted
visibility from a remote or a network call, and guessing would mean a repository
was screened or not screened depending on how it was cloned.

A `public` repository must declare a gate whose id is `public-safety`, and it
must be first at every surface it runs at. A gate that scans for prohibited
material second has already let something else touch the publication surface.

### `governance`

Optional, and keeps only `local-categories`: the categories beyond the five
canonical layers (`conventions`, `development`, `principles`, `vision`,
`workflows`) that this repository's governance tree may hold. Declared in
ascending order, and each one must name a directory that exists.

### `model-tiers`

Optional. Maps a portable tier to a harness's model and effort, so an agent
definition can name a tier rather than a vendor's model string. The tiers are a
closed set — `ultra`, `plan`, `execution`, `fast` — and a mapping keyed by an
agent name is refused: model and effort resolve by tier, never by artifact. A
tier declared with nothing under it is a refusal, not an empty map.

### `gates`

The ordered registry `gate run` dispatches. Each entry keeps exactly `id`,
`kind`, `run`, and `surfaces`, in that order.

| Key        | Meaning                                                                           |
| ---------- | --------------------------------------------------------------------------------- |
| `id`       | The name reported when this gate runs. Unique.                                    |
| `kind`     | `check` or `mutation`. A `mutation` may only run at `pre-commit`.                 |
| `run`      | The argument vector, started directly. No shell, so nothing here is interpolated. |
| `surfaces` | Where it runs: `commit-msg`, `pre-commit`, `pre-push`, `ci`, in that order.       |

Every one of the three hook surfaces must be covered by at least one gate. `ci`
is the only optional surface, which is why it is declared last.

A `mutation` restricted to `pre-commit` is the whole reason `kind` exists: a
gate that rewrites files during `pre-push` would push bytes nobody reviewed.

### `extensions`

Optional, and the escape hatch that keeps the rest of the schema closed. Each
key owns a mapping and RHINO reads no meaning inside one; a key whose value is
not a mapping is refused. Namespacing the key (`vendor/name`) is the convention
that keeps two tools out of each other's way, not something this build checks.

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
