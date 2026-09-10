# Findings

A finding is one violation of one declared rule. Findings go to stderr, one per
line, sorted, in a fixed shape:

```text
[<category>] <path>[:<line>]: <message>[ (<key>=<value>, ...)]
```

The `[<category>]` prefix is atomic rather than assembled from the command
path, so `grep '\[word-budget\]'` gets that validator's output and nothing
else, however the command was spelled on the way in.

The six commands added in `v0.3.0` report a second shape, frozen by the
contract more than one implementation validates against:

```text
[<category>] <path>:<line>:<column> <rule> <field> <message>
```

`-` stands in for a field the rule does not name, and `1:1` for a finding about
a path rather than a line. The two shapes coexist deliberately: a consumer's
stored output may not change because a new command arrived.

Every finding carries a **kind** — a stable identifier for the rule, not prose.
The kind is what a consumer filters on, so it is never reworded to improve a
message. In text output the kind appears inside the message; in JSON it is the
`kind` field.

## Word budget

| Kind                  | Means                                                          |
| --------------------- | -------------------------------------------------------------- |
| `word-limit-exceeded` | A governed file is longer than the limit its surface declares. |

Details: `words` (what was counted) and `limit` (what was declared). The limit
is inclusive — a file exactly at the declared count passes.

## Directory map

| Kind                    | Means                                                                   |
| ----------------------- | ----------------------------------------------------------------------- |
| `missing-readme`        | A directory inside a mapped tree has no `README.md`.                    |
| `missing-directory-map` | A README exists but has no `## Directory Map` section.                  |
| `missing-map-entry`     | A direct sibling is not listed in the map.                              |
| `invalid-map-entry`     | An entry does not name a direct sibling by a relative path that exists. |

A map entry may address a sibling directly, or a subdirectory through that
subdirectory's own `README.md`. It may not be absolute, carry a scheme, use
`..`, or reach deeper than one sibling: a map is a claim about _this_
directory.

A section with no entries is an empty map, not a missing one — a directory
holding nothing beside its README has a complete map.

## File naming

| Kind              | Means                                                           |
| ----------------- | --------------------------------------------------------------- |
| `invalid-md-name` | A governed file is not named in the style its surface declares. |

Detail: `prefix` on a `path-prefixed` surface — the prefix the file's own
directory encodes to, which is what its name has to begin with.

An exempt file is named by no style at all, so it produces no finding and is
not counted as inspected.

## Front matter

| Kind                        | Means                                                                |
| --------------------------- | -------------------------------------------------------------------- |
| `unterminated-frontmatter`  | A block opens with `---` and never closes.                           |
| `missing-frontmatter-key`   | A key the surface requires is not present.                           |
| `invalid-frontmatter-value` | A value is outside its declared set, or is not an ISO calendar date. |
| `forbidden-frontmatter-key` | A key the surface forbids is present.                                |

Detail: `value` on `invalid-frontmatter-value` — what was written, quotes
removed.

`unterminated-frontmatter` is reported instead of the keys, not alongside them:
everything after the opener reads as front matter to the end of the file, so no
key in it can be trusted. A missing key is reported at line 1, because an
absence has no position of its own.

## Heading hierarchy

| Kind                 | Means                                                                           |
| -------------------- | ------------------------------------------------------------------------------- |
| `missing-h1`         | A governed file declares no level-1 heading, where the repository requires one. |
| `multiple-h1`        | A second level-1 heading appears in a file permitted only one.                  |
| `heading-level-jump` | A heading drops further below its predecessor than the repository allows.       |

Detail: `jump` on `heading-level-jump` — how many levels were skipped.

`missing-h1` is reported at line 1; `multiple-h1` at each extra heading, so a
file with three reports two. The first heading in a document is never a jump: it
establishes the level the rest are measured from.

## README index

| Kind                   | Means                                                     |
| ---------------------- | --------------------------------------------------------- |
| `missing-readme-index` | A directory under a declared tree carries no `README.md`. |

Presence only. This kind and `missing-readme` are different rules a repository
chooses between: the directory-map one also wants a `## Directory Map` section.

## Emoji convention

| Kind                       | Means                                                  |
| -------------------------- | ------------------------------------------------------ |
| `emoji-in-prohibited-file` | A line of a prohibited file holds an emoji code point. |

Detail: `codePoint` — the first offending code point on the line, as `U+XXXX`.

One finding per line rather than per code point: three emoji on one line are one
edit. Only the prohibition is checked, never the permission.

## Internal links

| Kind                               | Means                                               |
| ---------------------------------- | --------------------------------------------------- |
| `internal-link-missing`            | A local link resolves to nothing in the repository. |
| `internal-link-outside-repository` | A local link resolves outside the repository root.  |
| `internal-link-malformed`          | A link's target cannot be interpreted as a path.    |

Fragment-only links (`#section`) and links carrying a scheme (`https:`,
`mailto:`) are skipped rather than reported — reporting on them is what
"internal link" excludes.

## Mermaid

| Kind                    | Means                                                                          |
| ----------------------- | ------------------------------------------------------------------------------ |
| `mermaid-accessibility` | A colour is undeclared, or is set somewhere the declared palette cannot reach. |
| `mermaid-legibility`    | A label segment is longer than the declared limit.                             |

`mermaid-legibility` details: `segment` (`node` or `edge`), `measured`, and
`limit`. A label is measured **as it is seen**: HTML entities are decoded,
markup is stripped, and a `<br>` splits a label into segments measured
separately, because two short lines are legible where one long line is not.

`mermaid-accessibility` covers two rules. Colour may be set only inside a
`classDef`; a `style`, `linkStyle`, or inline colour is reported wherever it
appears. And every colour a `classDef` sets must be drawn from the palette
declared for its role — `fill-colors`, `edge-colors`, or `text-colors`.

## Harness parity

Reconciling the canon against a declared harness.

| Kind                            | Means                                                                                            |
| ------------------------------- | ------------------------------------------------------------------------------------------------ |
| `missing-instruction`           | The declared canonical instruction body is not there.                                            |
| `missing-instruction-adapter`   | The declared adapter is not there.                                                               |
| `invalid-instruction-adapter`   | An adapter contains something other than the import of the canon.                                |
| `unexpected-instruction-source` | A second always-on instruction source competes with the canon.                                   |
| `invalid-skill`                 | A canonical skill has no usable declaration.                                                     |
| `invalid-agent`                 | A canonical agent has no usable declaration.                                                     |
| `unknown-capability`            | An agent names a capability or constraint outside the declared vocabulary.                       |
| `missing-skill-adapter`         | A harness has no wrapper for a canonical skill.                                                  |
| `skill-content-divergence`      | A wrapper's description, route, or declaration has drifted from the skill.                       |
| `unexpected-skill-adapter`      | A harness holds a wrapper no canonical skill asked for.                                          |
| `missing-agent-adapter`         | A harness has no adapter for a canonical agent.                                                  |
| `unexpected-agent-adapter`      | A harness carries an adapter for an agent the canon does not hold.                               |
| `agent-semantic-divergence`     | An adapter's identity, fixed fields, or permissions do not answer the canon.                     |
| `agent-prompt-divergence`       | An adapter does not carry the canonical route.                                                   |
| `divergent-capability`          | A harness's capability declaration is absent, unreadable, or does not match the required server. |

An adapter exists to route to the canon and to do nothing else. That is why
`invalid-instruction-adapter` is not configurable: anything beyond the import is
a second instruction source wearing the adapter's name.

The same holds one level down. An agent adapter carries the declared route and
its own harness's permissions — never a copy of the canonical prompt — so
rewriting the canon is not drift and produces no finding at all. Only the
digest moves.

## Governance roots

| Kind                             | Means                                                                            |
| -------------------------------- | -------------------------------------------------------------------------------- |
| `unknown-governance-layer`       | A directory under the governance root is neither a canonical layer nor declared. |
| `undeclared-governance-category` | A category exists that `governance.local-categories` does not declare.           |
| `unused-local-category`          | A declared local category names no directory.                                    |
| `empty-governed-directory`       | A governed directory holds nothing.                                              |

The five canonical layers are `conventions`, `development`, `principles`,
`vision`, and `workflows`. Anything else is a local category, accepted only
where the repository declared it.

## Companion sets

| Kind                                | Means                                                            |
| ----------------------------------- | ---------------------------------------------------------------- |
| `suffixed-companion-directory`      | A companion directory's name is not exactly its document's stem. |
| `missing-companion-index`           | The document carries no index linking its companions.            |
| `missing-indexed-companion`         | The index links a companion that is not there.                   |
| `unindexed-companion-module`        | A live companion is not in the index.                            |
| `non-contiguous-companion-ordinals` | An ordered set skips or repeats an ordinal.                      |
| `ordinal-in-unordered-set`          | A set with no reading order carries an ordinal prefix anyway.    |
| `unordered-module-in-ordered-set`   | An ordered set holds a companion with no ordinal.                |

## Instruction spine

| Kind                            | Means                                                                    |
| ------------------------------- | ------------------------------------------------------------------------ |
| `missing-canonical-instruction` | There is no `AGENTS.md`.                                                 |
| `missing-spine-section`         | One of the five spine sections is absent.                                |
| `misordered-spine-section`      | A spine section is written before one the order puts ahead of it.        |
| `interrupted-instruction-spine` | A repository-specific section divides the spine instead of following it. |
| `inexact-instruction-import`    | `CLAUDE.md` is not exactly `@AGENTS.md`.                                 |

An absent `CLAUDE.md` is not a finding. A repository with no Claude surface has
no adapter to keep honest, and inventing one would activate a harness the
repository never declared.

## Plan structure

Twenty rules in five families, each identified rather than described:

| Family            | Identifiers  | About                                                          |
| ----------------- | ------------ | -------------------------------------------------------------- |
| `PLAN-LIFECYCLE-` | `001`–`005`  | The root, the slug form, dating, and a slug in two roots.      |
| `PLAN-DOCUMENT-`  | `001`–`003`  | The six required documents and exactly one technical shape.    |
| `PLAN-COMPANION-` | `001`–`006`  | The `tech-docs/` set: naming, ordinals, index, and coverage.   |
| `PLAN-CRITERION-` | `001`, `002` | Acceptance identifiers: unique in `prd.md`, referenced onward. |
| `PLAN-DELIVERY-`  | `001`–`004`  | Numbered phases, executor labels, and archival last.           |

The identifiers are frozen. A rule whose meaning changes gets a new identifier
rather than a new definition, because more than one implementation reports
these and a consumer compares them by equality.

## Metadata

Twenty-three kinds, all prefixed `metadata-`. They divide into what the front
matter is:

| Group      | Kinds                                                                                                                                                        |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Shape      | `metadata-frontmatter-malformed`, `metadata-frontmatter-unterminated`                                                                                        |
| Keys       | `metadata-required-key-missing`, `metadata-unknown-key`, `metadata-duplicate-key`, `metadata-key-order`                                                      |
| Values     | `metadata-empty-value`, `metadata-null-value`, `metadata-scalar-required`, `metadata-folded-scalar-required`                                                 |
| Arrays     | `metadata-array-required`, `metadata-array-empty`, `metadata-array-duplicate`, `metadata-array-order`                                                        |
| Identity   | `metadata-name-format`, `metadata-name-path-mismatch`                                                                                                        |
| Prose      | `metadata-description-form`, `metadata-description-length`, `metadata-when-to-use-length`, `metadata-when-to-use-sentences`, `metadata-routing-not-distinct` |
| Vocabulary | `metadata-capability-unknown`, `metadata-tier-unknown`                                                                                                       |

The schema a document is held to is selected by its path, not declared in the
document: the path already supplies every identity the schema omits.

## Gates

`gate run` reports no findings of its own. It reports which gate ran and
whether it passed, and the exit code carries the verdict: `1` when a child
reported something, `3` when a child could not be started. A child's own
streams are never repeated.

## Not findings

Some situations look like findings and are not.

- **A file that vanishes between the walk and the read.** The repository
  changed under the run; reporting it would blame the maintainer for a race.
- **A file that exists and cannot be opened.** That is exit `2`, not a finding.
  Reporting it as missing would tell a maintainer to write a file that is
  already there — but only for a file the command was going to read. A file
  outside every declared root, glob, and document kind is one no rule is about,
  and refusing the run over it would make a repository's build directory able
  to stop its documentation from being checked.
- **A file that opens and holds no text.** An image or an archive is not an
  instruction, a skill, an agent, or a capability declaration. Only `harness
parity validate` meets one, because it is the only walk that reads files that
  are not Markdown: a prohibited glob names a path whatever its kind.
- **A non-Markdown file containing the canonical import.** Source, fixtures,
  and data are not always-on instructions to any harness — the code that
  implements this check has to contain the route to look for it. Files
  prohibited by _name_ are still reported whatever their kind.
- **A `README.md` in a canonical root or a harness's adapter directory.** It is
  an index of what lives there, not a declaration of anything.
- **An adapter granting more than the canon requires.** The contract is
  non-weakening, not equality. A harness's own defaults are not the canon's
  business; granting _less_, or granting what the canon denies, is.
- **A fenced example or code span quoting the import.** A page documenting the
  adapter is not one. A document that ends inside an unclosed fence gets no
  such benefit.
- **A fenced block whose diagram declaration RHINO cannot parse.** Reporting on
  syntax the tool does not understand would be reporting on its own ignorance.
- **A surface, tree, or roster that matched nothing.** Zero is a real answer.
  The run reports what it inspected so zero stays visible.

## Related

- [What a finding means](../explanation/what-a-finding-means.md)
- [Exit codes](./exit-codes.md)
- [JSON output](./json-output.md)
