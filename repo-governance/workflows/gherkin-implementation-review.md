# Gherkin Implementation Review

Use this workflow after adding or materially changing a `.feature` file, a behaviour adapter, an exemption, or a coverage mechanism, and before declaring the work complete.

Its purpose is semantic review. The static behaviour check proves a binding exists; it cannot prove the binding implements the behaviour.

## Inputs

- the `specs/behaviours/` corpus;
- the unit, integration, and end-to-end adapters;
- [behaviour-driven development](../development/behaviour-driven-development.md), [specification maintenance](../development/specification-maintenance.md), and [end-to-end testing](../development/end-to-end-testing.md); and
- the changed scope, or the whole corpus when a full audit is requested.

## Procedure

Inspect one scenario at a time. Do not substitute scenario counts, a grep heuristic, or a green test run for the inspection.

1. Inventory the corpus and expand every `Scenario Outline` example into its executable scenarios.
2. Create one review row per expanded scenario and applicable adapter. Record feature, scenario, adapter, binding location, and one of `PASS`, `EXEMPT`, or `FAIL`.
3. For each non-exempt row, trace the whole Given–When–Then path:
   - **Given** establishes the stated precondition through a real fixture or an injected double, in a root the test created.
   - **When** invokes the production subject or the boundary the scenario names.
   - **Then** reads independent observable evidence produced by that invocation.
4. Mark `FAIL` when any step is empty or a no-op; returns or stores a literal success sentinel; selects success from an expected-outcome table; asserts something the subject cannot fail; copies the expected value into the value later asserted; or asserts on a tree the subject never read. A literal `true` is still a failure when a helper performed the action first — the value `Then` consumes must derive from independently observed evidence.
5. For each exemption, verify it is scenario-level, its comment uses the canonical format, and its reason names a **concrete boundary** that layer cannot reach plus the alternative proof covering it. Difficulty, runtime, flakiness, and cost are never boundaries. **A unit exemption always fails.** Remove any layer-specific no-op branch for an exempt scenario, so accidental execution fails rather than reporting false proof.
6. Run the affected adapters. A test failure, a `FAIL` row, an invalid exemption, or a missing row blocks completion.
7. Store a requested audit as a non-authoritative report under ignored `local-tmp/`, with corpus totals, every row, findings and fixes, the exemption inventory, the commands run, and the results.

## Verification

Run the quick gate, then confirm the row count equals the expanded scenario count multiplied by its applicable adapters — exempt rows stay explicit and count as `EXEMPT`, they are never omitted.

Re-read the final diff for placeholder patterns after the fixes. The static check rejects known forms; this inspection is authoritative.

## Recovery

If a row cannot pass, keep it `FAIL` and fix the production seam or the adapter. Use a higher-layer exemption only when that layer fundamentally cannot express the scenario and an unexempted named layer proves the omitted concern. Never use an exemption to finish faster, avoid cost, quarantine a flake, or defer implementation.
