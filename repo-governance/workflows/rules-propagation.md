# Rules Propagation

Apply this workflow automatically whenever a repository [rule](../conventions/rules.md) is created, changed, moved, or deleted, or an explicitly requested [rules quality gate](rules-quality-gate.md) emits `NEEDS_PROPAGATION`. No separate instruction is required. Propagation is the sole writer, never invokes the quality gate, and consumes its frozen ledger when supplied. Edits inside one transaction do not start another.

## It Stops at This Repository's Boundary

These documents were extracted from a sibling repository and adapted. Nothing keeps the copies synchronized, and that is the decision. Propagation carries a rule change through **this** repository — its governance tree, its `AGENTS.md`, its adapters, its configuration — and stops there. It never edits another repository, and a divergence between repositories is not a finding.

The only shared contract is the machine-checked one: `repo-config.yml` and the validator each repository runs.

## Inputs and Transaction

Freeze: the proposed rule and rationale; its intended mandatory, expected, or permitted strength; the people, agents, files, or tasks in scope; known enforcement routes; any supplied gate ledger; the Git revision and dirty paths; pending verification; and authorization. Preserve them through compaction or handoff. A material external input change returns `BLOCKED_INPUT_CHANGED`; it never restarts the transaction.

## Procedure

1. Build one finite ledger from the requested outcome and any supplied gate ledger. Inspect only the affected rule, its points of use, higher authority, and directly overlapping guidance. Record each material gap as `OPEN`, `RESOLVED`, `NOT_APPLICABLE`, or `BLOCKED`. Do not add style preferences, speculative hardening, or machine-owned checks.
2. Before editing, return `BLOCKED_INPUT` for a missing decision or authority and `BLOCKED_CONFLICT` for an irreconcilable higher-authority conflict. Otherwise apply the minimum repair that closes every `OPEN` row:
   - put the shortest actionable form at each applicable point of use in `AGENTS.md`, keeping non-negotiable constraints visible;
   - place outcomes in vision, durable constraints in principles, repository choices in conventions, engineering standards in development, and procedures in workflows;
   - resolve conflicts in the order `vision > principles > conventions > development > workflows`;
   - keep one canonical statement, merge unique meaning, replace copies with concise links, and apply [progressive disclosure](../principles/progressive-disclosure.md);
   - change only stale, misplaced, overlapping, or repeated content the ledger implicates; and
   - name truthful enforcement under [software quality enforcement](../development/software-quality-enforcement.md), adding machinery only for a demonstrated need.
3. Read the repaired surfaces once for semantic closure. Resolve only repair-caused conflicts, under the hierarchy and [minimal sufficiency](../principles/minimal-sufficiency.md). Never broaden the ledger or reopen a settled preference.
4. Run the gate:

   ```sh
   ./hippo run --class ephemeral --disk-path . -- cargo xtask self-validate
   ```

5. On success return `PASS_NO_CHANGE` when no edit was necessary, otherwise `PASS_CHANGED`. For deterministic findings this transaction caused, freeze their exact set, repair mechanically, and rerun step 4 only while the count of failing checks and reported violations strictly decreases and no new failure class appears. Because that measure is nonnegative and decreasing, recovery terminates. Return `BLOCKED_TOOLING` if progress stops, a new or unrelated failure appears, or no verdict can be obtained.

HIPPO recovery is infrastructure handling, not another propagation transaction.

## Terminal Contract

The only results are `PASS_NO_CHANGE`, `PASS_CHANGED`, `BLOCKED_INPUT`, `BLOCKED_CONFLICT`, `BLOCKED_TOOLING`, and `BLOCKED_INPUT_CHANGED`. Propagation repairs every authorized semantic row; anything it cannot decide or verify maps to a specific external blocker. Passing means good enough, not perfect, and authorizes neither commit nor push. With unchanged inputs and repository state, another transaction produces no diff.
