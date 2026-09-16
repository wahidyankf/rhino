# Technical Design: Harden RHINO Validation Edge Cases

## Current Boundaries

| Concern                 | Current behavior                                                         | Required behavior                                                                                          |
| ----------------------- | ------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------- |
| Acceptance definitions  | `src/plan/criteria.rs` accepts only trimmed lines starting `Scenario:`   | Parse `Scenario:` and `Scenario Outline:` declarations, then extract identifiers from the declaration line |
| No-tier canonical agent | `src/harness.rs::tier_projection` returns when `expected.tier` is absent | Inspect declared tier fields; reject adapter model or effort when canonical tier is absent                 |
| Mermaid metadata        | Colour and legibility readers iterate every diagram line                 | Classify `accTitle` and single-line or braced `accDescr` as metadata before content inspection             |
| Numeric entities        | `contains_color` sees `#123;` as a three-digit colour                    | Exclude valid numeric entity spans from colour-token recognition while label decoding remains unchanged    |

All four statements are [Repo-grounded] in current `main` at plan authoring time.

## Design Decisions

### Acceptance grammar

Use an explicit declaration-prefix parser rather than substring matching. The parser returns the remainder only for
`Scenario:` and `Scenario Outline:` after indentation, so comments, prose, `Examples:`, and step text remain inert.
Both definition collection and duplicate detection consume that one parser; the RED scenarios for both behaviors land
before its first production edit.

### Tier absence

Keep the existing exemption for harness adapters with no declared `tier-fields`. When fields exist, canonical absence
means both mapped fields must be absent. A projection produces the new stable finding kind
`tier-projection-without-canonical-tier`; existing mapped and unmapped-tier findings keep their meanings.

### Mermaid source roles

Classify accessibility metadata once and share that classification with colour and legibility readers. A braced
`accDescr` block includes its opener, body, and matching close. Accessibility-presence validation still reads those
lines; diagram-content validators do not. Colour recognition skips complete decimal forms such as `&#128640;` and
hexadecimal forms such as `&#x1F680;`, but continues scanning the rest of the same line for actual colour tokens.

## Specification Changes

AC-01 through AC-07 become durable behavior in the three files below. AC-08 stays plan-level because independent
release safety is a delivery property; the per-unit commit, focused gate, complete gate, exact-head PR, merge, and
reconciled-main tasks in Phases 1–3 prove it.

### `specs/behaviours/plan-structure.feature` `[E]`

```diff
- Acceptance definitions are recognized only on Scenario: lines.
+ A Scenario Outline: line defines its acceptance identifier.
+ One identifier on Scenario: and Scenario Outline: is a duplicate definition.
```

- New `A scenario outline defines its acceptance identifier`: a plan author supplies a conforming six-document plan,
  validation follows a delivery citation, and no undefined-identifier finding remains. `[AC-01]`
- New `Scenario and outline definitions share duplicate detection`: a plan author defines one identifier through both
  declaration forms, validation runs, and exactly one `PLAN-CRITERION-001` is reported. `[AC-02]`
- Preserve the existing `An acceptance identifier defined twice is a finding` and
  `A delivery item citing an undefined acceptance identifier is a finding` scenarios; every other existing scenario
  remains unchanged.
- Bindings: `tests/unit/bindings.rs`, `tests/integration/bindings.rs`, and `tests/e2e/bindings.rs`.
- Support: no edit; the existing generic file fixture in `tests/support/plan.rs` accepts the synthetic Markdown input.
- Proof: `./hippo run --class ephemeral --disk-path . -- cargo test --test unit`, then the integration and end-to-end
  commands in Phase 4.

### `specs/behaviours/harness-parity.feature` `[E]`

```diff
- Canonical tier absence returns before adapter model or effort is inspected.
+ A no-tier canonical agent rejects either projected adapter tier field.
```

- New outline `A no-tier canonical agent cannot project an adapter tier field`: a harness maintainer provides a
  canonical agent with no tier and examples for `model` and `effort`, parity validation runs, and each example reports
  one `tier-projection-without-canonical-tier` finding. `[AC-03]`
- Preserve `A canonical agent declaring no tier has nothing to project`,
  `A harness that declares no tier fields is not held to the mapping`, and all existing mapped, unmapped, partial, and
  drift scenarios unchanged. `[AC-04]`
- Bindings: `tests/unit/bindings.rs`, `tests/integration/bindings.rs`, and `tests/e2e/bindings.rs`.
- Support: no edit; the existing generic projection steps in `tests/support/harness.rs` select model and effort
  independently.
- Proof: `./hippo run --class ephemeral --disk-path . -- cargo test --test unit`, then the integration and end-to-end
  commands in Phase 4.

### `specs/behaviours/mermaid-legibility.feature` `[E]`

```diff
- Colour and label readers inspect accessibility metadata as diagram content.
- A numeric entity beginning with # can be classified as a colour.
+ accTitle, single-line accDescr, and braced accDescr are metadata, not diagram content.
+ Complete decimal and hexadecimal numeric entities are excluded from colour-token recognition.
```

- New outline `Accessibility metadata is not diagram content`: a documentation author supplies examples for
  `accTitle`, single-line `accDescr`, and braced `accDescr` containing parenthesized or colour-like prose; validation
  runs, and the metadata produces no node-label or colour finding. `[AC-05]`
- New outline `Numeric HTML entities are not colour literals`: a visible label contains decimal `&#128640;` or
  hexadecimal `&#x1F680;`; validation runs, and each entity remains decoded for measurement without a colour finding.
  `[AC-06]`
- New `Real diagram content remains enforced`: a real out-of-class colour and overlong visible label are validated,
  and both existing findings remain observable. `[AC-07]`
- Preserve `A rendered repository requires an accessible title and description`,
  `A rendered diagram with no accessible title is a finding`,
  `A rendered diagram with no accessible description is a finding`, `The braced description form is a description`,
  every label-boundary scenario, and every other existing scenario unchanged.
- Bindings: `tests/unit/bindings.rs`, `tests/integration/bindings.rs`, and `tests/e2e/bindings.rs`.
- Support: no edit; the existing generic diagram fixture in `tests/support/mermaid.rs` accepts each source example.
- Proof: `./hippo run --class ephemeral --disk-path . -- cargo test --test unit`, then the integration and end-to-end
  commands in Phase 4.

### Fixture and Architecture Disposition

- Add `specs/fixtures/plan-structure/accepted/007-scenario-outline-criterion/` as a synthetic six-document accepted
  plan, then update `manifest.tsv`, `SHA256SUMS`, and `CORPUS-DIGEST`. It freezes AC-01 across the shared corpus.
- `specs/architecture.md` is a verified no-op: parser internals change within existing validator components, so no C4
  view, node, relationship, or constraint changes.

## File-Impact Analysis

```text
repo-governance/
└── conventions/
    ├── plan-validator-contract/002-rule-identifiers.md                    [E] Freeze both scenario declaration forms
    └── plans/006-structural-validation.md                                 [E] State the accepted definition grammar
specs/
├── behaviours/
│   ├── harness-parity.feature                                             [E] No-tier projection behavior
│   ├── mermaid-legibility.feature                                        [E] Metadata and numeric-entity behavior
│   └── plan-structure.feature                                             [E] Scenario Outline definition behavior
└── fixtures/plan-structure/
    ├── accepted/007-scenario-outline-criterion/plans/backlog/outline-plan/
    │   ├── README.md                                                      [N] Synthetic fixture overview
    │   ├── brd.md                                                         [N] Synthetic business requirement
    │   ├── delivery.md                                                    [N] Citation of the outline identifier
    │   ├── learnings.md                                                   [N] Required sixth document
    │   ├── prd.md                                                         [N] Scenario Outline definition
    │   └── tech-docs.md                                                   [N] Single technical shape
    ├── CORPUS-DIGEST                                                      [E] Digest of the updated checksum list
    ├── SHA256SUMS                                                         [E] Per-file fixture checksums
    └── manifest.tsv                                                       [E] Accepted case 007
src/
├── harness.rs                                                             [E] Enforce canonical no-tier absence
├── markdown/mermaid.rs                                                    [E] Classify metadata and entities
└── plan/criteria.rs                                                       [E] Parse both scenario declaration forms
tests/
├── e2e/bindings.rs                                                        [E] Register new behavior scenarios
├── integration/bindings.rs                                                [E] Register new behavior scenarios
└── unit/bindings.rs                                                       [E] Register new behavior scenarios
```

### More Detail

`tests/support/plan.rs`, `tests/support/harness.rs`, and `tests/support/mermaid.rs` are [G] grounding references whose
existing generic fixtures cover the planned scenarios. They remain unchanged.

## Dependencies

No new crate or external service is required. Existing Rust standard-library parsing, `unicode-segmentation`, the BDD
adapters, and the shared fixture checksum contract are sufficient.

## Rollback

Each delivery unit has one branch and pull request. Revert only the unit's specification, source, binding, and fixture
changes. The other two units remain valid because no source or test dependency crosses their seams.
