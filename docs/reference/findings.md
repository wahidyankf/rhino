# Findings

This page records the stable finding kinds for grouped-v2 validation leaves.
Command-specific operation status is defined by [the command reference](./cli.md)
and the executable v0.4 corpus.

A finding is one violation of one declared rule. Findings go to stderr, one per
line, sorted, in a fixed shape:

```text
[<category>] <path>[:<line>]: <message>[ (<key>=<value>, ...)]
```

The `[<category>]` prefix is atomic rather than assembled from the command
path, so `grep '\[word-budget\]'` gets that validator's output and nothing
else, however the command was spelled on the way in.

`metadata validate` reports a second shape, the one it has used since
`v0.3.0`:

```text
[<category>] <path>:<line>:<column> <kind>[ <field>] <message>
```

The field is left out when the rule names none, and `1:1` stands for a finding
about the file rather than one of its lines. The two shapes coexist
deliberately: a consumer's stored output may not change because a new command
arrived.

`harness adapters validate`, `env validate`, and `toolchain validate` report a
third, shorter shape, described in [their own sections](#harness-adapters).

Every finding carries a **kind** — a stable identifier for the rule, not prose.
The kind is what a consumer filters on, so it is never reworded to improve a
message. In JSON it is the `kind` field of each violation. In text, the second
shape prints it after the position; the first shape prints only the message, so
filter on the kind through `--output json`.

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

| Kind                 | Means                                                                             |
| -------------------- | --------------------------------------------------------------------------------- |
| `invalid-md-name`    | A governed file is not named in the style its surface declares.                   |
| `fragmented-md-name` | A governed file's name ends in `-part-<n>`, `-continuation-<n>`, or `-continued`. |

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

| Kind                         | Means                                                                           |
| ---------------------------- | ------------------------------------------------------------------------------- |
| `missing-readme-index`       | A declared tree has no `README.md`.                                             |
| `missing-readme-index-child` | A tree declaring `require-direct-children` has a direct child its README omits. |
| `missing-readme-annotation`  | A declared annotation file is absent, or does not contain its declared text.    |

Detail: `child` on `missing-readme-index-child` — the child the index does not
link.

This validator and the directory map are different rules a repository chooses
between: the directory-map one also wants a `## Directory Map` section.

## Emoji convention

| Kind                       | Means                                                  |
| -------------------------- | ------------------------------------------------------ |
| `emoji-in-prohibited-file` | A line of a prohibited file holds an emoji code point. |

Detail: `codePoint` — the first offending code point on the line, as `U+XXXX`.

One finding per line rather than per code point: three emoji on one line are one
edit. Only the prohibition is checked, never the permission.

## Internal links

| Kind                               | Means                                                                                                            |
| ---------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| `internal-link-missing`            | A local link resolves to nothing in the repository.                                                              |
| `internal-link-outside-repository` | A local link resolves outside the repository root, or passes through a symbolic link, which RHINO never follows. |
| `internal-link-malformed`          | A link's target cannot be interpreted as a path.                                                                 |

Fragment-only links (`#section`) and links carrying a scheme (`https:`,
`mailto:`) are skipped rather than reported — reporting on them is what
"internal link" excludes.

## Mermaid

| Kind                     | Means                                                                                                        |
| ------------------------ | ------------------------------------------------------------------------------------------------------------ |
| `mermaid-accessibility`  | A colour is undeclared, misplaced, or too low in contrast, or an accessible title or description is missing. |
| `mermaid-legibility`     | A label segment is longer than the declared limit.                                                           |
| `diagram-authoring-rule` | A repository declaring `authoring-rule: plain-text` carries a Mermaid diagram.                               |

`mermaid-legibility` details: `segment` (`node` or `edge`), `measured`, and
`limit`. A label is measured **as it is seen**: HTML entities are decoded,
markup is stripped, and a `<br>` splits a label into segments measured
separately, because two short lines are legible where one long line is not.

`mermaid-accessibility` covers colour and description. Colour may be set only
inside a `classDef`; a `style`, `linkStyle`, or inline colour is reported
wherever it appears. Every colour a `classDef` sets must be a six-digit hex
value such as `#0173B2`, drawn from the palette declared for its role —
`fill-colors`, `edge-colors`, or `text-colors`. A class that sets a `fill` must
also set a `stroke` that differs from the fill, so the shape keeps a visible
boundary, and a text `color`, so its label does not inherit an unknown one. A
class that sets only a text colour is reported, because nothing makes it
legible; a `stroke` alone is a valid outline-only class. Text on a fill must
meet the normal-text contrast threshold of 4.5:1. That finding carries two
details: `ratio`, the measured contrast to two decimals, and `threshold`,
`4.5`. Contrast is measured only when both the fill and the text colour are
declared, so an undeclared colour is reported once. Under
`authoring-rule: rendered`, a diagram that declares no `accTitle` or no
`accDescr` is reported too.

## License

| Kind                          | Means                                                      |
| ----------------------------- | ---------------------------------------------------------- |
| `missing-license`             | A declared license path holds no file.                     |
| `license-identifier-mismatch` | A declared identifier does not appear in the license.      |
| `license-digest-mismatch`     | The license's SHA-256 digest is not the declared `sha256`. |

Detail: `identifier` on `license-identifier-mismatch` — the identifier that is
missing.

## Vendor terms

| Kind                    | Means                                                                    |
| ----------------------- | ------------------------------------------------------------------------ |
| `forbidden-vendor-term` | A declared forbidden term appears in a file outside its exact exception. |

Detail: `term` — the forbidden term found.

## Governance layers

| Kind                             | Means                                                  |
| -------------------------------- | ------------------------------------------------------ |
| `missing-governance-layer`       | A layer the declared order names is absent.            |
| `unexpected-governance-layer`    | An entry under the layer root is not a declared layer. |
| `missing-governance-category`    | A category declared for a layer is absent.             |
| `unexpected-governance-category` | A directory inside a layer is not declared for it.     |

## Traceability

| Kind                                | Means                                                            |
| ----------------------------------- | ---------------------------------------------------------------- |
| `missing-traceability-artifact`     | A declared artifact's path holds no file.                        |
| `missing-traceability-relationship` | A declared relationship has no local link from source to target. |

Detail: `target` on `missing-traceability-relationship` — the artifact the
source does not link.

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

## Harness adapters

`harness adapters validate` compares every declared adapter with the bytes
generation would write. It reports one line per path on stderr, in the shape
`[harness-adapters] <path>: <kind>`; under `--output json` the `findings` array
on stdout carries the same `<path>: <kind>` strings.

| Kind                 | Means                                                                       |
| -------------------- | --------------------------------------------------------------------------- |
| `missing-adapter`    | An adapter generation would write is absent.                                |
| `divergent-adapter`  | An adapter exists and differs from what generation would write.             |
| `stale-adapter`      | A file under a generated family root is not one generation would write.     |
| `non-text-adapter`   | An adapter path holds bytes that are not text.                              |
| `unreadable-adapter` | An adapter path cannot be read; the line ends with the reason after a `: `. |

`harness adapters generate` writes exactly the files that clear every kind.

## Environment

`env validate` reports `[environment-validate] <path>: <kind>` on stderr,
followed by ` (<key>)` when the finding is about one key. Under
`--output json` each entry of `findings` carries `rule`, `path`, and `key`. A key
is named; a value never is.

| Kind                         | Means                                                                                                                                                                     |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `declared-but-unread`        | A key a contract declares is read by no detected source.                                                                                                                  |
| `read-but-undeclared`        | A detected source reads a key no contract declares.                                                                                                                       |
| `declared-source-unread`     | A path a detector declares does not exist. One that exists and cannot be read, or lies behind a symbolic link, refuses the run with exit `2` and `rhino.file.unreadable`. |
| `unsupported-dynamic-access` | A detected source reads a key whose name is computed, reported as `<dynamic>`.                                                                                            |
| `staged-environment-file`    | An indexed path matches a forbidden staging pattern and no allowed one.                                                                                                   |

An allowlist entry can suppress every kind except `staged-environment-file`.

## Toolchains

`toolchain validate` reports `[toolchain-validate] <id>: <kind>` on stderr for
each required toolchain that fails its probe. Under `--output json` each entry
of `findings` carries `toolchain` and `rule`. The probe's own output is never
reported.

| Kind               | Means                                                            |
| ------------------ | ---------------------------------------------------------------- |
| `unavailable`      | The probe could not be started.                                  |
| `probe-failed`     | The probe ran and exited non-zero.                               |
| `version-mismatch` | The probe's parsed output does not match the declared `version`. |

A toolchain that is not `required` reports nothing.

## Gates

`gate run` reports no findings of its own. It reports which gate ran and
whether it passed, and the exit code carries the verdict: `1` when a child
reported something, `127` when a child was not found, and `126` when one was
found and could not be executed. A child's own
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
- **A fenced block whose diagram declaration RHINO cannot parse.** Reporting on
  syntax the tool does not understand would be reporting on its own ignorance.
- **A surface, tree, or roster that matched nothing.** Zero is a real answer.
  The run reports what it inspected so zero stays visible.

## Related

- [What a finding means](../explanation/what-a-finding-means.md)
- [Exit codes](./exit-codes.md)
- [JSON output](./json-output.md)
