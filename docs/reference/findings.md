# Findings

A finding is one violation of one declared rule. Findings go to stderr, one per
line, sorted, in a fixed shape:

```text
[<category>] <path>[:<line>]: <message>[ (<key>=<value>, ...)]
```

The `[<category>]` prefix is atomic rather than assembled from the command
path, so `grep '\[word-budget\]'` gets that validator's output and nothing
else, however the command was spelled on the way in.

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

## Not findings

Some situations look like findings and are not.

- **A file that vanishes between the walk and the read.** The repository
  changed under the run; reporting it would blame the maintainer for a race.
- **A file that exists and cannot be opened.** That is exit `2`, not a finding.
  Reporting it as missing would tell a maintainer to write a file that is
  already there.
- **A file that opens and holds no text.** An image or an archive is not an
  instruction, a skill, an agent, or a capability declaration. Only `harness
parity validate` meets one, because it is the only walk that reads every file
  rather than every file of a declared kind.
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
