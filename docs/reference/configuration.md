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
| `canonical.skills-root`          | with a non-empty roster | Where canonical skills live.                                |
| `canonical.agents-root`          | with a non-empty roster | Where canonical agents live.                                |
| `harnesses`                      | yes, and may be empty   | The coding harnesses to reconcile.                          |
| `prohibited-instruction-sources` | yes                     | Globs that may not be always-on instruction sources.        |
| `capabilities`                   | yes, and may be empty   | The vocabulary an agent may draw capabilities from.         |
| `constraints`                    | yes, and may be empty   | The vocabulary an agent may draw constraints from.          |
| `required-mcp`                   | with a non-empty roster | The server every harness must declare identically.          |

`harnesses` is required even when empty, and empty is a legal, explicit
declaration. The difference between "this repository has no coding harnesses"
and "I forgot to configure harnesses" has to stay visible.

**Three keys follow the roster.** `skills-root`, `agents-root`, and
`required-mcp` exist to be reconciled against a harness. They are required
whenever the roster is non-empty and **refused** alongside an empty one — a
canon with nowhere to be reconciled is a reconciliation that was silently
skipped, not a clean pass.

Each harness takes:

| Key                 | Required | Meaning                                             |
| ------------------- | -------- | --------------------------------------------------- |
| `name`              | yes      | How the harness is named in output and `--harness`. |
| `agent-dir`         | yes      | Where this harness keeps its agent adapters.        |
| `agent-extension`   | yes      | The extension those adapters use.                   |
| `command-dir`       | no       | Where this harness keeps skill wrappers.            |
| `capability-file`   | yes      | This harness's capability declaration.              |
| `capability-format` | yes      | `toml` or `json`.                                   |

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
    agent-dir: .claude/agents
    agent-extension: ".md"
    capability-file: .mcp.json
    capability-format: json
```

```console
$ rhino repo-config validate
[repo-config] repo-config.yml: line 31: skills-root: required whenever the harness roster is non-empty
```

Line 31 is the `harnesses:` line. A key that is absent has no line of its own,
so the fault is reported against the roster that made it required — which is the
line you would have to look at to decide whether to add the key or empty the
roster. `agents-root` and `required-mcp` are missing for the same reason;
validation stops at the first semantic fault, so fix this one and run it again.

None of them degrade to a partial run. A validator that skipped a tree because
its configuration was malformed would report a clean repository that was never
checked.

## Related

- [Command line](./cli.md)
- [Exit codes](./exit-codes.md)
- [Why RHINO holds no defaults](../explanation/holding-no-defaults.md)
