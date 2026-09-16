# Delivery: Harden RHINO Validation Edge Cases

> **Legend** — `[AI]`: an agent performs the step. `[HUMAN]`: only a human can perform it because of unavailable
> credentials, physical action, external authority, or an unresolved decision. Split mixed work into separate items.

## Execution Checkout

Repository path: `~/ose-projects/rhino/`

Worktree path: `~/ose-projects/rhino/worktrees/harden-rhino-validation-edge-cases/`

Delivery mode: `worktree-to-pr`

HIPPO policy source: `~/ose-projects/rhino/hippo.local.json`

The policy file is intentionally ignored and therefore absent from task worktrees. In every new shell, before any
`./hippo run` command in this checklist (including a Pause Safety resume command), run:

```bash
export HIPPO_DEFAULT_CONFIG="$HOME/ose-projects/rhino/hippo.local.json"
test -r "$HIPPO_DEFAULT_CONFIG"
```

Do not fall back to the wrapper's missing worktree-local default and do not change the declared task class.

Use one worktree for the plan and one branch per delivery unit:

1. `worktree/harden-rhino-validation-edge-cases-criteria`
2. `worktree/harden-rhino-validation-edge-cases-tiers`
3. `worktree/harden-rhino-validation-edge-cases-mermaid`

Before provisioning, verify the primary checkout and remote:

```bash
cd ~/ose-projects/rhino
test "$(git branch --show-current)" = main
test -z "$(git status --porcelain)"
git fetch origin --prune
git merge --ff-only origin/main
git worktree add worktrees/harden-rhino-validation-edge-cases \
  -b worktree/harden-rhino-validation-edge-cases-criteria origin/main
```

Reuse the worktree after each merged unit: reconcile `main`, then create the next branch from current `origin/main` in
the same clean directory. Retain landed local unit branches until final cleanup; do not reset them. Never create a
second worktree for this plan.

## Delivery Units

| Unit | Outcome                                            | Branch                                                 | Rollback                                                                 |
| ---- | -------------------------------------------------- | ------------------------------------------------------ | ------------------------------------------------------------------------ |
| 1    | Scenario Outline acceptance definitions            | `worktree/harden-rhino-validation-edge-cases-criteria` | Revert criterion grammar, contract prose, accepted fixture, and bindings |
| 2    | Canonical no-tier projection enforcement           | `worktree/harden-rhino-validation-edge-cases-tiers`    | Revert harness behavior, source, and bindings                            |
| 3    | Mermaid metadata and numeric-entity classification | `worktree/harden-rhino-validation-edge-cases-mermaid`  | Revert Mermaid behavior, source, and bindings                            |

Every unit is independently green and releasable. The worktree-to-PR workflow owns commit, push, draft PR, exact-head
quality, leak review, rebase merge, main reconciliation, and branch cleanup. Fix every gate failure at its cause,
including a pre-existing failure encountered in scope; never bypass a hook.

## Phase 0: Environment Setup and Baseline

- [ ] `[AI]` Provision the declared worktree with the exact `git worktree add` command above; acceptance: the worktree
      is registered once on the Unit 1 branch at `origin/main`. `[AC-08]`
- [ ] `[AI]` Export and verify `HIPPO_DEFAULT_CONFIG` with the exact commands above; acceptance: it resolves to the
      readable primary-checkout policy file before the first guarded command. `[AC-08]`
- [ ] `[AI]` In the worktree, run `./hippo run --class ephemeral --disk-path . -- npm ci`; acceptance: dependencies
      install, hooks activate, and `git status --porcelain` is empty. `[AC-08]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --disk-path . -- cargo xtask test-quick`; acceptance: the baseline quick
      gate exits `0`. `[AC-08]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --disk-path . -- cargo xtask self-validate`; acceptance: the baseline
      CI surface exits `0`. `[AC-08]`
- [ ] `[AI]` Move `plans/backlog/harden-rhino-validation-edge-cases/` to
      `plans/in-progress/harden-rhino-validation-edge-cases/` with `git mv`; acceptance: one in-progress copy exists and
      no backlog copy exists. `[AC-08]`
- [ ] `[AI]` Update `plans/backlog/README.md` and `plans/in-progress/README.md`; acceptance: only the in-progress index
      links the active plan. `[AC-08]`
- [ ] `[AI]` Run
      `./hippo run --class ephemeral --disk-path . -- cargo run --quiet --bin rhino -- plan validate`; acceptance: plan
      validation exits `0`. `[AC-08]`

### Phase 0 Gate

- [ ] `[AI]` Run `git status --short` and record the baseline plus the plan activation paths in this file; acceptance:
      no unowned path is present. `[AC-08]`

> **Pause Safety**: one clean worktree exists and the active plan validates. Safe to stop. To resume:
> `./hippo run --class ephemeral --disk-path . -- cargo run --quiet --bin rhino -- plan validate`.

## Phase 1: Acceptance Definition Grammar — Unit 1

- [ ] `[AI]` **RED**: add both the outline-definition and cross-form duplicate scenarios to
      `specs/behaviours/plan-structure.feature`, `tests/unit/bindings.rs`, `tests/integration/bindings.rs`, and
      `tests/e2e/bindings.rs`; run
      `./hippo run --class ephemeral --disk-path . -- cargo test --test unit`; acceptance: the outline citation is
      reported undefined and the expected cross-form duplicate finding is absent, while prior scenarios pass.
      `[AC-01]` `[AC-02]`
- [ ] `[AI]` **GREEN**: edit `src/plan/criteria.rs` so one declaration parser recognizes `Scenario:` and
      `Scenario Outline:` before both definition and duplicate collection; run
      `./hippo run --class ephemeral --disk-path . -- cargo test --test unit`; acceptance: AC-01 passes and the
      cross-form duplicate emits exactly one `PLAN-CRITERION-001`. `[AC-01]` `[AC-02]`
- [ ] `[AI]` **REFACTOR**: keep declaration recognition in one private helper in `src/plan/criteria.rs`; run
      `./hippo run --class ephemeral --disk-path . -- cargo fmt --all --check`, then
      `./hippo run --class ephemeral --disk-path . -- cargo test --test unit`; acceptance: formatting and all unit
      scenarios pass without duplicated prefix logic. `[AC-01]` `[AC-02]`
- [ ] `[AI]` Apply `repo-governance/workflows/rules-propagation.md` to
      `repo-governance/conventions/plan-validator-contract/002-rule-identifiers.md` and
      `repo-governance/conventions/plans/006-structural-validation.md`, then record the workflow verdict under this
      item; acceptance: the verdict is `PASS` or `PASS_NO_CHANGE`, both documents state the two accepted declaration
      forms, and no rule identifier or message changes. `[AC-01]` `[AC-02]`
- [ ] `[AI]` Add the six-document accepted fixture at
      `specs/fixtures/plan-structure/accepted/007-scenario-outline-criterion/plans/backlog/outline-plan/`; acceptance:
      its PRD defines an identifier on `Scenario Outline:` and its delivery checklist cites that identifier. `[AC-01]`
- [ ] `[AI]` Add accepted case 007 to `specs/fixtures/plan-structure/manifest.tsv`; acceptance: the row declares exit
      `0`, no rule identifiers, and the Scenario Outline purpose. `[AC-01]`
- [ ] `[AI]` Regenerate `specs/fixtures/plan-structure/SHA256SUMS` and `CORPUS-DIGEST` with
      `(cd specs/fixtures/plan-structure && { find ./accepted ./rejected -type f -print; printf '%s\n' ./manifest.tsv; }
| LC_ALL=C sort | while IFS= read -r file; do shasum -a 256 "$file"; done > SHA256SUMS && shasum -a 256
SHA256SUMS | awk '{print $1}' > CORPUS-DIGEST)`; acceptance: both files describe the exact current corpus.
      `[AC-01]` `[AC-08]`
- [ ] `[AI]` Run `(cd specs/fixtures/plan-structure && shasum -a 256 -c SHA256SUMS)`; acceptance: exit `0` and accepted
      case 007 produces no finding. `[AC-01]` `[AC-08]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --disk-path . -- cargo xtask test-quick`; acceptance: exit `0`.
      `[AC-08]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --disk-path . -- cargo xtask self-validate`; acceptance: exit `0`.
      `[AC-08]`
- [ ] `[AI]` Commit only Unit 1 plus active-plan progress using a Conventional Commit; acceptance: hooks pass and the
      commit contains no Unit 2 or Unit 3 path. `[AC-01]` `[AC-02]` `[AC-08]`
- [ ] `[AI]` Push the declared Unit 1 branch; acceptance: the pre-push hook passes without bypass and the remote head
      equals the local commit. `[AC-08]`
- [ ] `[AI]` Open the Unit 1 pull request as a draft with a public-safety-screened title and body; acceptance: the PR
      targets `main` and names its proof. `[AC-08]`
- [ ] `[AI]` Mark the Unit 1 PR ready after the diff is final; acceptance: the ready-for-review quality run starts on
      the exact head. `[AC-08]`
- [ ] `[AI]` Post one exact-head leak review after quality passes; acceptance: the posted review names the unchanged
      head and reports no finding. `[AC-08]`
- [ ] `[AI]` Rebase-merge Unit 1 after all five merge preconditions pass; acceptance: GitHub reports the PR merged.
      `[AC-01]` `[AC-02]` `[AC-08]`
- [ ] `[AI]` Reconcile primary `main` and the reused worktree with `origin/main`; acceptance:
      `git rev-list --left-right --count main...origin/main` prints `0 0`. `[AC-08]`

### Phase 1 Gate

- [ ] `[AI]` On reconciled `main`, run
      `(cd specs/fixtures/plan-structure && shasum -a 256 -c SHA256SUMS)`; acceptance: the shared corpus checksum exits
      `0` before Unit 2 starts. `[AC-01]` `[AC-02]` `[AC-08]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --disk-path . -- cargo xtask self-validate` on reconciled `main`;
      acceptance: exit `0` before Unit 2 starts. `[AC-01]` `[AC-02]` `[AC-08]`

> **Pause Safety**: Unit 1 is merged, its branch is terminal, and main accepts Scenario Outline criteria. Safe to stop.
> To resume: `git fetch origin && git rebase origin/main` in the reused worktree.

## Phase 2: Canonical No-Tier Enforcement — Unit 2

- [ ] `[AI]` In the clean reused worktree, run `git fetch origin --prune`, then
      `git switch -c worktree/harden-rhino-validation-edge-cases-tiers origin/main`; acceptance:
      `git rev-list --left-right --count HEAD...origin/main` prints `0 0`. `[AC-08]`
- [ ] `[AI]` **RED**: edit `specs/behaviours/harness-parity.feature`, `tests/unit/bindings.rs`,
      `tests/integration/bindings.rs`, and `tests/e2e/bindings.rs` with examples where a no-tier canonical agent's
      adapter projects only `model` or only `effort`; run
      `./hippo run --class ephemeral --disk-path . -- cargo test --test unit`; acceptance: the new scenario fails
      for both examples because current validation reports no finding. `[AC-03]`
- [ ] `[AI]` **GREEN**: edit `src/harness.rs::tier_projection` so declared tier fields are inspected before canonical
      tier absence returns; emit `tier-projection-without-canonical-tier` for either projected half and run
      `./hippo run --class ephemeral --disk-path . -- cargo test --test unit`; acceptance: both AC-03 examples pass and
      existing tier diagnostics remain unchanged. `[AC-03]`
- [ ] `[AI]` **REFACTOR**: express canonical absent, mapped, and unmapped states as one exhaustive branch structure in
      `src/harness.rs`; run `./hippo run --class ephemeral --disk-path . -- cargo fmt --all --check`, then
      `./hippo run --class ephemeral --disk-path . -- cargo test --test unit`; acceptance: AC-03 and AC-04 pass without
      duplicate scalar reads. `[AC-03]` `[AC-04]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --disk-path . -- cargo xtask test-quick`; acceptance: exit `0`.
      `[AC-08]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --disk-path . -- cargo xtask self-validate`; acceptance: exit `0`.
      `[AC-08]`
- [ ] `[AI]` Commit only Unit 2 plus plan progress using a Conventional Commit; acceptance: hooks pass and no Unit 3
      path is present. `[AC-03]` `[AC-04]` `[AC-08]`
- [ ] `[AI]` Push the Unit 2 branch; acceptance: the pre-push hook passes and the remote head equals the local commit.
      `[AC-08]`
- [ ] `[AI]` Open the Unit 2 PR as a draft; acceptance: it targets `main` and records the unit's proof. `[AC-08]`
- [ ] `[AI]` Mark the Unit 2 PR ready only after the diff is final; acceptance: the exact-head ready-for-review quality
      run starts. `[AC-08]`
- [ ] `[AI]` Post one exact-head leak review; acceptance: it names the unchanged head and reports no finding. `[AC-08]`
- [ ] `[AI]` Rebase-merge Unit 2 after all five preconditions pass; acceptance: GitHub reports the PR merged. `[AC-03]`
      `[AC-04]` `[AC-08]`
- [ ] `[AI]` Reconcile primary `main` and the reused worktree with `origin/main`; acceptance:
      `git rev-list --left-right --count main...origin/main` prints `0 0`. `[AC-08]`

### Phase 2 Gate

- [ ] `[AI]` Run `./hippo run --class ephemeral --disk-path . -- cargo xtask self-validate` on reconciled main;
      acceptance: exit `0` before Unit 3 starts. `[AC-03]` `[AC-04]` `[AC-08]`

> **Pause Safety**: Unit 2 is merged and no-tier canon cannot acquire adapter policy. Safe to stop. To resume:
> `git fetch origin && git rebase origin/main` in the reused worktree.

## Phase 3: Mermaid Source Classification — Unit 3

- [ ] `[AI]` In the clean reused worktree, run `git fetch origin --prune`, then
      `git switch -c worktree/harden-rhino-validation-edge-cases-mermaid origin/main`; acceptance:
      `git rev-list --left-right --count HEAD...origin/main` prints `0 0`. `[AC-08]`
- [ ] `[AI]` **CHARACTERIZE**: add the preservation scenario for a real out-of-class colour and overlong visible label
      to `specs/behaviours/mermaid-legibility.feature`, `tests/unit/bindings.rs`, `tests/integration/bindings.rs`, and
      `tests/e2e/bindings.rs`; run `./hippo run --class ephemeral --disk-path . -- cargo test --test unit`; acceptance:
      both existing finding kinds are observable before production code changes. `[AC-07]`
- [ ] `[AI]` **RED**: in `specs/behaviours/mermaid-legibility.feature`, `tests/unit/bindings.rs`,
      `tests/integration/bindings.rs`, and `tests/e2e/bindings.rs`, add metadata examples for `accTitle`, single-line
      `accDescr`, and braced `accDescr`, plus visible-label examples for decimal `&#128640;` and hexadecimal `&#x1F680;`;
      run
      `./hippo run --class ephemeral --disk-path . -- cargo test --test unit`; acceptance: the new scenarios fail with
      the current false node-label or colour findings for every example. `[AC-05]` `[AC-06]`
- [ ] `[AI]` **GREEN**: edit `src/markdown/mermaid.rs` to classify single-line and braced accessibility metadata and to
      ignore valid numeric entity spans during colour-token scanning; run
      `./hippo run --class ephemeral --disk-path . -- cargo test --test unit`; acceptance: every AC-05 and AC-06
      example passes, AC-07 still reports both existing findings, and accessibility-presence checks still read the
      metadata. `[AC-05]` `[AC-06]` `[AC-07]`
- [ ] `[AI]` **REFACTOR**: centralize line-role and entity-span recognition in private helpers; run
      `./hippo run --class ephemeral --disk-path . -- cargo fmt --all --check`, then
      `./hippo run --class ephemeral --disk-path . -- cargo test --test unit`; acceptance: all Mermaid scenarios pass
      with no duplicated state tracking. `[AC-05]` `[AC-06]` `[AC-07]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --disk-path . -- cargo xtask test-quick`; acceptance: exit `0`.
      `[AC-08]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --disk-path . -- cargo xtask self-validate`; acceptance: exit `0`.
      `[AC-08]`
- [ ] `[AI]` Commit only Unit 3 plus plan progress using a Conventional Commit; acceptance: hooks pass and the commit
      contains only Unit 3 plus plan-record paths. `[AC-05]` `[AC-06]` `[AC-07]` `[AC-08]`
- [ ] `[AI]` Push the Unit 3 branch; acceptance: the pre-push hook passes and the remote head equals the local commit.
      `[AC-08]`
- [ ] `[AI]` Open the Unit 3 PR as a draft; acceptance: it targets `main` and records the unit's proof. `[AC-08]`
- [ ] `[AI]` Mark the Unit 3 PR ready only after the diff is final; acceptance: the exact-head ready-for-review quality
      run starts. `[AC-08]`
- [ ] `[AI]` Post one exact-head leak review; acceptance: it names the unchanged head and reports no finding. `[AC-08]`
- [ ] `[AI]` Rebase-merge Unit 3 after all five preconditions pass; acceptance: GitHub reports the PR merged. `[AC-05]`
      `[AC-06]` `[AC-07]` `[AC-08]`
- [ ] `[AI]` Reconcile primary `main` and the reused worktree with `origin/main`; acceptance:
      `git rev-list --left-right --count main...origin/main` prints `0 0`. `[AC-08]`

### Phase 3 Gate

- [ ] `[AI]` Run `./hippo run --class ephemeral --disk-path . -- cargo xtask self-validate` on reconciled main;
      acceptance: exit `0` and all three units are independently present. `[AC-05]` `[AC-06]` `[AC-07]` `[AC-08]`

> **Pause Safety**: all code units are merged and main is green. Safe to stop. To resume:
> `./hippo run --class ephemeral --disk-path . -- cargo xtask self-validate`.

## Phase 4: Convergence and Execution Review

- [ ] `[AI]` Run `./hippo run --class ephemeral --disk-path . -- cargo test --test integration`; acceptance: the
      integration adapter passes every changed scenario. `[AC-01]` `[AC-02]` `[AC-03]` `[AC-04]` `[AC-05]` `[AC-06]`
      `[AC-07]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --disk-path . -- cargo test --test e2e`; acceptance: the end-to-end
      adapter passes every changed scenario. `[AC-01]` `[AC-02]` `[AC-03]` `[AC-04]` `[AC-05]` `[AC-06]` `[AC-07]`
- [ ] `[AI]` Run `repo-governance/workflows/gherkin-implementation-review.md` for the three changed feature files and
      store its row ledger at `local-tmp/harden-rhino-validation-edge-cases-gherkin-review.tsv`; acceptance: every
      expanded scenario has a `PASS` or justified `EXEMPT` row for unit, integration, and end-to-end. `[AC-08]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --disk-path . -- cargo xtask test-quick`; acceptance: exit `0` on the
      exact final main tree. `[AC-08]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --disk-path . -- cargo xtask self-validate`; acceptance: exit `0` on the
      exact final main tree. `[AC-08]`
- [ ] `[AI]` Run `repo-governance/workflows/plan-execution-check.md` against Phases 0–4 and record the verdict in this
      file; acceptance: it reports `PASS` with every AC terminal before knowledge capture. `[AC-08]`

### Phase 4 Gate

- [ ] `[AI]` Verify AC-01 through AC-08 each cite terminal evidence and no delivery unit or PR remains open;
      acceptance: the execution review permits knowledge capture. `[AC-08]`

> **Pause Safety**: substantive implementation is terminal and reviewable on main. Safe to stop. To resume:
> `./hippo run --class ephemeral --disk-path . -- cargo xtask self-validate`.

## Phase 5: Knowledge Capture

- [ ] `[AI]` Apply the generalizability, secret/sensitivity, and repository-relevance gates to every `learnings.md`
      entry; acceptance: each entry has one safe owner or a discard reason. `[AC-08]`
- [ ] `[AI]` Route each surviving entry to exactly one durable owner, filing code/test follow-up work as a separate
      backlog plan rather than editing it inline; acceptance: `learnings.md` records every terminal destination.
      `[AC-08]`
- [ ] `[AI]` If execution produced no generalizable entry, write
      `No generalizable learnings — all observations were specific to these three corrected parser boundaries.`;
      acceptance: the running log has an explicit terminal state. `[AC-08]`

### Phase 5 Gate

- [ ] `[AI]` Read every `learnings.md` entry and verify it is promoted, filed, or discarded with reason; acceptance: no
      unresolved entry remains. `[AC-08]`

> **Pause Safety**: every learning is terminal and substantive work remains green. Safe to stop. To resume:
> `rg -n '^## Learning|Status|No generalizable' plans/in-progress/harden-rhino-validation-edge-cases/learnings.md`.

## Plan Archival

- [ ] `[AI]` Verify all substantive items are complete and the execution-check verdict is `PASS`. `[AC-08]`
- [ ] `[AI]` Move the plan to `plans/done/YYYY-MM-DD__harden-rhino-validation-edge-cases/` with the actual completion
      date; acceptance: exactly one done copy exists and no in-progress copy remains. `[AC-08]`
- [ ] `[AI]` Update `plans/in-progress/README.md`, `plans/done/README.md`, and every live reference; acceptance: no live
      link names the former in-progress path. `[AC-08]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --disk-path . -- cargo xtask test-quick`; acceptance: exit `0` from the
      archived state. `[AC-08]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --disk-path . -- cargo xtask self-validate`; acceptance: exit `0` from
      the archived state. `[AC-08]`
- [ ] `[AI]` Run `git diff --check`; acceptance: exit `0` from the archived state. `[AC-08]`
- [ ] `[AI]` Commit the archival transaction using a Conventional Commit; acceptance: hooks pass and only archive/index
      paths are present. `[AC-08]`
- [ ] `[AI]` Push the archival branch; acceptance: the pre-push hook exits `0` and the remote head equals the local
      commit. `[AC-08]`
- [ ] `[AI]` Open the archival PR as a draft; acceptance: it targets `main` and records archived-state proof. `[AC-08]`
- [ ] `[AI]` Mark the archival PR ready only after the diff is final; acceptance: exact-head quality starts. `[AC-08]`
- [ ] `[AI]` Post one exact-head leak review for the archival PR; acceptance: it names the unchanged head and reports no
      finding. `[AC-08]`
- [ ] `[AI]` Rebase-merge the archival PR after all five preconditions pass; acceptance: GitHub reports it merged.
      `[AC-08]`
- [ ] `[AI]` Run dev artifact clean-up from the primary checkout; acceptance: the plan worktree and task branches are
      absent, `main` equals `origin/main`, and no untracked secret or unrelated artifact was removed. `[AC-08]`
