# Technical Design: Harden RHINO Validation Edge Cases

## Current Boundaries

| Concern                 | Current behavior                                                                                                                                                                                                 | Required behavior                                                                                       |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| No-tier canonical agent | `src/v0_4/harnesses.rs::validate_adapter` checks `identity`, `fixed`, and `lists` for repeated fields but not against `tier-fields`, so `render_adapter` writes a fixed `model` or `effort` into a no-tier agent | Refuse any direct or translation field that names a declared tier field, before any adapter is written  |
| Mermaid metadata        | `src/markdown/mermaid.rs::colors` and `::legibility` iterate every diagram line, `accTitle` and `accDescr` included                                                                                              | Classify `accTitle` and single-line or braced `accDescr` as metadata before content inspection          |
| Numeric entities        | `src/markdown/mermaid.rs::contains_color` reads the digits of `&#123;` or `&#128640;` as a colour                                                                                                                | Exclude valid numeric entity spans from colour-token recognition while label decoding remains unchanged |

All three statements are [Repo-grounded] in `main` at `b691fae` (`v0.6.0`), each reproduced with the release binary:

- With `fixed: { model: sonnet }` beside `tier-fields: { model: model, effort: effort }` and no canonical agent declaring
  a tier, `harness adapters generate` wrote `model: sonnet` into every agent adapter and `harness adapters validate`
  exited `0`. With one tiered agent present, generation instead refused late with ``adapter output repeats field `model` ``.
- `accTitle:`, single-line `accDescr:`, and braced `accDescr` prose containing a parenthesized clause each produced a
  `mermaid-legibility` node-label finding, and a label containing `&#123;` produced "color is declared outside a
  classDef". A hexadecimal entity such as `&#x1F680;` did not, because `x` ends the digit run.

A hand-edited adapter that adds a model is already reported as `divergent-adapter`, because validation compares every
adapter with the bytes generation would write; the configuration path above is the remaining way to project a tier.

## Design Decisions

### Tier fields

Add the declared `tier-fields` model and effort names to the set `validate_adapter` already uses for repeated direct
fields, so `identity`, `fixed`, `lists`, or a translation naming either one refuses the configuration with exit `2` and
names the profile. [Judgment call] A configuration refusal, not a new finding kind: the fault is in `repo-config.yml`,
the neighbouring field-collision rule already refuses, and the refusal fires before any write in both `validate` and
`generate`. A profile that declares no `tier-fields` is unaffected.

### Mermaid source roles

Classify accessibility metadata once and share that classification with the colour and legibility readers. A braced
`accDescr` block includes its opener, body, and matching close. `accessibility` still reads those lines to decide
presence; diagram-content validators do not. Colour recognition skips complete decimal forms such as `&#128640;` and
hexadecimal forms such as `&#x1F680;`, but continues scanning the rest of the same line for actual colour tokens.

## Specification Changes

AC-01 through AC-05 become durable behavior in `specs/behaviours/v0-4-contract.feature`, the whole stable corpus. AC-06
stays plan-level because independent release safety is a delivery property; the per-unit commit, focused gate, complete
gate, exact-head PR, merge, and reconciled-main tasks in Phases 1–2 prove it.

### `specs/behaviours/v0-4-contract.feature` `[E]`

```diff
- A profile may fix a native field that its tier fields also project.
+ A direct adapter field naming a declared tier field refuses the configuration.
- Colour and label readers inspect accessibility metadata as diagram content.
- A numeric entity beginning with # can be classified as a colour.
+ accTitle, single-line accDescr, and braced accDescr are metadata, not diagram content.
+ Complete decimal and hexadecimal numeric entities are excluded from colour-token recognition.
```

- New outline `A direct adapter field cannot name a declared tier field`: a harness maintainer declares `tier-fields`
  and, per example, a `fixed` `model` or `effort`; validation runs, exits `2`, and names the profile. `[AC-01]`
- New `A no-tier agent renders no tier field`: a canonical agent declares no tier and its profile fixes no tier field;
  generation then validation leaves neither field in its adapter and exits `0`. `[AC-02]`
- New outline `Accessibility metadata is not diagram content`: a documentation author supplies examples for `accTitle`,
  single-line `accDescr`, and braced `accDescr` containing parenthesized or colour-like prose; validation runs, and the
  metadata produces no node-label or colour finding. `[AC-03]`
- New outline `Numeric HTML entities are not colour literals`: a visible label contains decimal `&#128640;` or
  hexadecimal `&#x1F680;`; validation runs, and each entity remains decoded for measurement without a colour finding.
  `[AC-04]`
- New `Real diagram content remains enforced`: a real out-of-class colour and overlong visible label are validated, and
  both existing findings remain observable. `[AC-05]`
- Preserve every existing harness-adapter and Mermaid scenario unchanged, including
  `Typed harness profiles render native agent and skill adapters`, `Agent adapters project a declared canonical list`, and both `Mermaid policy`
  scenarios.
- Bindings: `tests/unit/bindings.rs`, `tests/integration/bindings.rs`, and `tests/e2e/bindings.rs`.
- Steps: `tests/support/binding.rs` holds the shared step vocabulary; the RED item records any step it adds.
- Proof: `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo test --test unit`, then the
  integration and end-to-end commands in Phase 3.

### Architecture Disposition

`specs/architecture.md` is a verified no-op: both changes stay inside existing validator components, so no C4 view,
node, relationship, or constraint changes.

## File-Impact Analysis

```text
CHANGELOG.md                                                               [E] Record both corrections
docs/reference/
├── findings.md                                                            [E] Mermaid metadata and entity handling
└── v0-4-configuration.md                                                  [E] Tier-field collision refusal
specs/behaviours/v0-4-contract.feature                                     [E] Tier-field and Mermaid behavior
src/
├── markdown/mermaid.rs                                                    [E] Classify metadata and entities
└── v0_4/harnesses.rs                                                      [E] Refuse tier-field collisions
tests/
├── e2e/bindings.rs                                                        [E] Register new behavior scenarios
├── integration/bindings.rs                                                [E] Register new behavior scenarios
└── unit/bindings.rs                                                       [E] Register new behavior scenarios
```

### More Detail

`tests/support/binding.rs` is a [G] grounding reference: an edit is made only when a new scenario needs a step the
shared vocabulary lacks, and the RED item records it.

## Dependencies

No new crate or external service is required. Existing Rust standard-library parsing, `unicode-segmentation`, and the
BDD adapters are sufficient.

## Rollback

Each delivery unit has one branch and pull request. Revert only the unit's specification, source, binding, and
documentation changes. The other unit remains valid because no source or test dependency crosses their seams.
