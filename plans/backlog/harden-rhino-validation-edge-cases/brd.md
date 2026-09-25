# Business Requirements: Harden RHINO Validation Edge Cases

## Business Goal

Make RHINO's validator results trustworthy at two already-observed boundaries: a valid construct must not be reported as
invalid, and an undeclared adapter decision must not pass silently.

## Affected Roles

- Repository maintainers rely on a clean run to mean the declared contracts were actually checked.
- Harness maintainers need canonical tier absence to prevent vendor-specific model choices.
- Documentation authors need accessible Mermaid metadata and encoded labels to remain valid input.

## Desired Outcomes

- [Repo-grounded] Harness adapter validation refuses a configuration that would give a no-tier canonical agent an
  adapter model or effort.
- [Repo-grounded] Mermaid inspection distinguishes metadata, visible labels, entities, and colour declarations.
- Each correction is independently reviewable, reversible, and green on RHINO's complete gates.

## Success Measures

- Every acceptance criterion in `prd.md` has passing executable evidence.
- The changed Gherkin scenarios pass at unit, integration, and end-to-end boundaries.
- `cargo xtask test-quick` and `cargo xtask self-validate` exit `0` after every delivery unit.

## Non-Goals

- Assigning tiers to canonical agents that intentionally declare none.
- Weakening Mermaid accessibility, legibility, or palette rules.
- Coordinating downstream adoption or cutting a release.

## Business Risks

- An over-broad Mermaid exclusion could hide real colour or label findings.
- A tier-field refusal could reject a configuration a consumer relies on; the refusal names the profile so the fix is
  one configuration edit.
- Combining delivery units would increase rollback scope; the plan explicitly keeps them separate.
