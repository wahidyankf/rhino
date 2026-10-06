# Check Mermaid diagram syntax in `md mermaid validate`

`md mermaid validate` passes a flowchart or sequence diagram that Mermaid's own parser rejects, because it never checks
whether a diagram parses.

> Filed 2026-10-06 by a consuming repository under its Upstream Tool Defects standard. First seen by that consumer on
> 2026-07-22, when a deliberately corrupted diagram passed its gate. Reproduced on released `v0.11.0`
> (`db711617a95ed67700f42d6652c5a9c271a166e6`) and on trunk `6d31682fe724e3a5331a13af94b40d0cc85ce160`.

## Problem and evidence

### Description

The command reports `checked N diagrams, no findings` and exits `0` for a diagram that `mermaid.parse` rejects. It does
inspect each block: a diagram with an overlong label or an unlisted type exits `1`. A malformed diagram is
indistinguishable from a well-formed one in its output.

The help text promises only "label length and colour contrast", so no stated contract is broken. The surprise is
elsewhere. Consumers wire the command in as their Mermaid gate, and the reference speaks of types "RHINO can parse",
which reads as if a parsed type has been checked for syntax. A broken diagram ships green and renders on the host as an
error box.

### Steps to reproduce

From an empty directory, with a `rhino` binary on `PATH`, create `repo-config.yml`:

```yaml
schema: rhino/repo-config/v2
repository: {}
policies:
  markdown:
    mermaid:
      authoring-rule: rendered
      node-label-graphemes: 20
      edge-label-graphemes: 20
      fill-colors: ["#FFFFFF", "#0173B2"]
      edge-colors: ["#000000", "#0173B2"]
      text-colors: ["#000000"]
      require-default-class: true
      allowed-types: [flowchart, sequenceDiagram]
```

Create four Markdown files, each holding one fenced `mermaid` block with this body:

- `good.md`, a well-formed control:

  ```text
  flowchart TD
      accTitle: Good diagram
      accDescr: Two nodes joined by one edge.
      A[Good node] --> B[Also good]

      classDef default fill:#FFFFFF,stroke:#000000,color:#000000
  ```

- `broken-flowchart.md`, a malformed shape and an unclosed brace after a valid edge:

  ```text
  flowchart TD
      accTitle: Broken diagram
      accDescr: A malformed shape and an unclosed brace follow a valid edge.
      A[Good node] --> B[Also good]
      C[[[malformed shape --> D
      E{{unclosed --> F

      classDef default fill:#FFFFFF,stroke:#000000,color:#000000
  ```

- `broken-flowchart-2.md`, an unclosed bracket, an invalid arrow, and an unclosed `subgraph`:

  ```text
  flowchart TD
      accTitle: Broken diagram two
      accDescr: An unclosed bracket, a bogus arrow, and an unclosed subgraph.
      A[Unclosed --> B
      B ~~~> C
      subgraph S1
      C --> D

      classDef default fill:#FFFFFF,stroke:#000000,color:#000000
  ```

- `broken-sequence.md`, a message with a malformed arrow:

  ```text
  sequenceDiagram
      accTitle: Broken sequence
      accDescr: A message with a malformed arrow.
      participant A
      participant B
      A -x-> B: hello
  ```

Then run:

```sh
rhino md mermaid validate; echo "exit $?"
for f in good.md broken-flowchart.md broken-flowchart-2.md broken-sequence.md; do
  rhino md mermaid validate --file "$f"; echo "exit $?"
done
```

Two controls show the command is not inert. In a second directory with the same configuration, a flowchart whose node
label is 51 graphemes long, and a `pie` diagram, which the configuration does not list, each exit `1`.

### Expected behaviour

Each broken diagram is reported, with a finding naming the file and the line where parsing failed, and the command exits
`1`. `mermaid@11.15.0`'s `mermaid.parse`, run on the four bodies, accepts `good.md` and rejects the other three:

```text
REJECT broken-flowchart.md - Parse error on line 5:
REJECT broken-flowchart-2.md - Parse error on line 11:
REJECT broken-sequence.md - Parse error on line 6:
```

### Actual behaviour

Identical on `v0.11.0` and on trunk:

```text
[mermaid] checked 4 diagrams, no findings
exit 0
```

Each `--file` run prints `[mermaid] checked 1 diagram, no findings` and `exit 0`, broken or not. The controls print:

```text
[mermaid] long-label.md:7: a node label is longer than the declared limit (segment=node, measured=51, limit=20)
exit 1
[mermaid] bad-type.md:4: diagram type `pie` is not in the declared allowed types, so this repository does not render it
exit 1
```

### Environment

macOS on arm64. The released `v0.11.0` archive and a `cargo build --release` of trunk. The parser comparison used
`mermaid@11.15.0` under `jsdom` in Node.js 24.

### Documentation consulted

- `rhino --help`: "Check Mermaid diagrams for label length and colour contrast."
- [Findings](../../../docs/reference/findings.md#mermaid) lists accessibility, legibility, and authoring-rule findings,
  and says "a listed type RHINO cannot parse still receives the checks that need no grammar".
- The module comment in [`src/markdown/mermaid.rs`](../../../src/markdown/mermaid.rs) says which syntaxes RHINO "can
  parse" is a property of the tool, meaning read well enough to measure labels and colours.

The behaviour matches the help text. It does not match what "can parse" suggests, or the role consumers give the
command.

## Why now

The consumer that found this mandates Mermaid across its documentation, plans, and rules, and gates every commit and
pull request on this command. Its plans cite the gate as the Mermaid-correctness check, and a diagram-remediation
backlog it intends to verify with the gate could go green with broken diagrams. Nothing is blocked today, because a
separate parse step works, so this is important rather than urgent.

## Prior art

Duplicate check, run 2026-10-06 against `wahidyankf/rhino`:

- Issues, all states: none exist in the repository (`is:issue` returns 0).
- Pull requests: none open. Searching all pull requests for `mermaid`, `syntax`, and `parse` finds closed work only.
  [#95](https://github.com/wahidyankf/rhino/pull/95) added `exclude`, `require-default-class`, `allowed-types`, theme,
  and canvas checks, not syntax.
- In-flight plans: [`harden-rhino-validation-edge-cases`](../../backlog/harden-rhino-validation-edge-cases/README.md)
  hardens Mermaid metadata and entity handling and lists "adding a general Mermaid parser" as a non-goal. It is related,
  not a duplicate. `plans/in-progress/` holds no plan.
- Idea briefs: none existed in any quadrant.

References, all accessed 2026-10-06:

- Mermaid's [usage documentation](https://mermaid.js.org/config/usage.html) documents
  `mermaid.parse(text, parseOptions)`, which validates a diagram without rendering it.
- [mermaid-cli](https://github.com/mermaid-js/mermaid-cli) is the maintained headless renderer, so a diagram it cannot
  render is caught outside a browser.

## Proposed direction

- Add a parse stage to the command, distinct from the accessibility and legibility checks, with its own finding kind, so
  the two failure classes stay separately reportable.
- Per diagram type, either implement a real grammar or report clearly that the type was not syntax-checked. Never let a
  pass imply a syntax check that did not run.
- If a real parser is out of reach for a Rust binary that spawns no processes, state the limit in the help text and the
  findings reference so consumers know to add a parse step of their own.

## Scope and non-goals

In scope: syntax checking for the types RHINO already reads, at least `flowchart` and `sequenceDiagram`; its finding
kind; and the help text and reference.

Out of scope: rendering diagrams to images, and changing the existing label-length, palette, contrast, and type checks.

## Risks and open questions

- Mermaid's grammar is large and changes with each release. A hand-written grammar drifts from the renderer, and a
  bundled JavaScript parser conflicts with the validators being process-free.
- Which Mermaid version a consumer renders with differs per host, so "parses" needs a named reference version.
- A consumer corpus may hold diagrams that pass today and fail a real parse, so the stage may need to be opt-in until
  consumers have measured and repaired.

## Workaround

Run `mermaid.parse` or `mermaid-cli` over the corpus as a separate gate step beside `md mermaid validate`.

## Success and promotion signal

Success: the three broken diagrams above exit `1` with a parse finding each, the good diagram and both controls behave
as today, and the help text states exactly what is checked.

Promote when the owner decides between a real grammar, a declared limit, or an opt-in external parser, which is the open
design question above.
