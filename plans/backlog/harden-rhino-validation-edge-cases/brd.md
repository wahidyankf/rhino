# Business Requirements: Harden RHINO Validation Edge Cases

## Business Goal

Make RHINO's validator results trustworthy at three already-observed grammar boundaries: a valid construct must not be
reported as invalid, and an undeclared adapter decision must not pass silently.

## Affected Roles

- Repository maintainers rely on a clean run to mean the declared contracts were actually checked.
- Plan authors need `Scenario Outline:` criteria to trace to delivery items.
- Harness maintainers need canonical absence to prevent vendor-specific model choices.
- Documentation authors need accessible Mermaid metadata and encoded labels to remain valid input.

## Desired Outcomes

- [Repo-grounded] Plan validation recognizes both accepted Gherkin scenario declaration forms.
- [Repo-grounded] Harness parity reports a stable finding when a no-tier canonical agent gains adapter model or effort.
- [Repo-grounded] Mermaid inspection distinguishes metadata, visible labels, entities, and colour declarations.
- Each correction is independently reviewable, reversible, and green on RHINO's complete gates.

## Success Measures

- Every acceptance criterion in `prd.md` has passing executable evidence.
- The three focused Gherkin corpora pass at unit, integration, and end-to-end boundaries where applicable.
- The shared plan corpus checksum verifies after its accepted case is added.
- `cargo xtask test-quick` and `cargo xtask self-validate` exit `0` after every delivery unit.

## Non-Goals

- Redesigning the plan validator contract.
- Assigning tiers to canonical agents that intentionally declare none.
- Weakening Mermaid accessibility, legibility, or palette rules.
- Coordinating downstream adoption or cutting a release.

## Business Risks

- A grammar expansion without a shared fixture could leave another implementation behind.
- An over-broad Mermaid exclusion could hide real colour or label findings.
- Combining delivery units would increase rollback scope; the plan explicitly keeps them separate.
