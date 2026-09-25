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

1. `worktree/harden-rhino-validation-edge-cases-tiers`
2. `worktree/harden-rhino-validation-edge-cases-mermaid`

Before provisioning, verify the primary checkout and remote:

```bash
cd ~/ose-projects/rhino
test "$(git branch --show-current)" = main
test -z "$(git status --porcelain)"
git fetch origin --prune
git merge --ff-only origin/main
git worktree add worktrees/harden-rhino-validation-edge-cases \
  -b worktree/harden-rhino-validation-edge-cases-tiers origin/main
```

Reuse the worktree after each merged unit: reconcile `main`, then create the next branch from current `origin/main` in
the same clean directory. Retain landed local unit branches until final cleanup; do not reset them. Never create a
second worktree for this plan.

## Delivery Units

| Unit | Outcome                                            | Branch                                                | Rollback                                            |
| ---- | -------------------------------------------------- | ----------------------------------------------------- | --------------------------------------------------- |
| 1    | Tier-field collision refusal                       | `worktree/harden-rhino-validation-edge-cases-tiers`   | Revert harness behavior, source, bindings, and docs |
| 2    | Mermaid metadata and numeric-entity classification | `worktree/harden-rhino-validation-edge-cases-mermaid` | Revert Mermaid behavior, source, bindings, and docs |

Every unit is independently green and releasable. The worktree-to-PR workflow owns commit, push, draft PR, exact-head
quality, leak review, rebase merge, main reconciliation, and branch cleanup. Fix every gate failure at its cause,
including a pre-existing failure encountered in scope; never bypass a hook.

## Phase 0: Environment Setup and Baseline

- [ ] `[AI]` Provision the declared worktree with the exact `git worktree add` command above; acceptance: the worktree
      is registered once on the Unit 1 branch at `origin/main`. `[AC-06]`
- [ ] `[AI]` Export and verify `HIPPO_DEFAULT_CONFIG` with the exact commands above; acceptance: it resolves to the
      readable primary-checkout policy file before the first guarded command. `[AC-06]`
- [ ] `[AI]` In the worktree, run `./hippo run --class transactional --resource-tier standard --disk-path . -- npm ci`; acceptance: dependencies
      install, hooks activate, and `git status --porcelain` is empty. `[AC-06]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo xtask test-quick`; acceptance: the baseline quick
      gate exits `0`. `[AC-06]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo xtask self-validate`; acceptance: the baseline
      CI surface exits `0`. `[AC-06]`
- [ ] `[AI]` Move `plans/backlog/harden-rhino-validation-edge-cases/` to
      `plans/in-progress/harden-rhino-validation-edge-cases/` with `git mv`; acceptance: one in-progress copy exists and
      no backlog copy exists. `[AC-06]`
- [ ] `[AI]` Update `plans/backlog/README.md` and `plans/in-progress/README.md`; acceptance: only the in-progress index
      links the active plan. `[AC-06]`

### Phase 0 Gate

- [ ] `[AI]` Run `git status --short` and record the baseline plus the plan activation paths in this file; acceptance:
      no unowned path is present. `[AC-06]`

> **Pause Safety**: one clean worktree exists on a green baseline and the plan is in progress. Safe to stop. To resume:
> `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo xtask self-validate`.

## Phase 1: Tier-Field Collision Refusal — Unit 1

- [ ] `[AI]` Confirm the clean reused worktree is on `worktree/harden-rhino-validation-edge-cases-tiers` at
      `origin/main`; acceptance: `git rev-list --left-right --count HEAD...origin/main` prints `0 0`. `[AC-06]`
- [ ] `[AI]` **CHARACTERIZE**: add `A no-tier agent renders no tier field` to `specs/behaviours/v0-4-contract.feature`,
      `tests/unit/bindings.rs`, `tests/integration/bindings.rs`, and `tests/e2e/bindings.rs`; run
      `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo test --test unit`; acceptance: the
      scenario passes before production code changes, and any step added to `tests/support/binding.rs` is recorded
      here. `[AC-02]`
- [ ] `[AI]` **RED**: add the outline `A direct adapter field cannot name a declared tier field`, with `fixed` `model`
      and `fixed` `effort` examples, to the same four files; run
      `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo test --test unit`; acceptance: both
      examples fail because validation currently exits `0`. `[AC-01]`
- [ ] `[AI]` **GREEN**: edit `validate_adapter` in `src/v0_4/harnesses.rs` so the declared `tier-fields` names join the
      repeated-field set checked against `identity`, `fixed`, `lists`, and translation fields; run
      `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo test --test unit`; acceptance: both
      AC-01 examples exit `2` naming the profile, AC-02 still passes, and existing harness scenarios are unchanged.
      `[AC-01]` `[AC-02]`
- [ ] `[AI]` **REFACTOR**: keep the collision check in one pass over the adapter's field names in
      `src/v0_4/harnesses.rs`; run
      `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo fmt --all --check`, then
      `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo test --test unit`; acceptance:
      formatting and all unit scenarios pass. `[AC-01]` `[AC-02]`
- [ ] `[AI]` Apply `repo-governance/workflows/docs-propagation.md` to `docs/reference/v0-4-configuration.md` and
      `CHANGELOG.md`; acceptance: the configuration reference states the refusal and the changelog records it under an
      unreleased entry. `[AC-01]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo xtask test-quick`;
      acceptance: exit `0`. `[AC-06]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo xtask self-validate`;
      acceptance: exit `0`. `[AC-06]`
- [ ] `[AI]` Commit only Unit 1 plus plan progress using a Conventional Commit; acceptance: hooks pass and no Unit 2
      path is present. `[AC-01]` `[AC-02]` `[AC-06]`
- [ ] `[AI]` Push the Unit 1 branch; acceptance: the pre-push hook passes and the remote head equals the local commit.
      `[AC-06]`
- [ ] `[AI]` Open the Unit 1 PR as a draft; acceptance: it targets `main` and records the unit's proof. `[AC-06]`
- [ ] `[AI]` Mark the Unit 1 PR ready only after the diff is final; acceptance: the exact-head ready-for-review quality
      run starts. `[AC-06]`
- [ ] `[AI]` Post one exact-head leak review; acceptance: it names the unchanged head and reports no finding. `[AC-06]`
- [ ] `[AI]` Rebase-merge Unit 1 after all five preconditions pass; acceptance: GitHub reports the PR merged. `[AC-01]`
      `[AC-02]` `[AC-06]`
- [ ] `[AI]` Reconcile primary `main` and the reused worktree with `origin/main`; acceptance:
      `git rev-list --left-right --count main...origin/main` prints `0 0`. `[AC-06]`

### Phase 1 Gate

- [ ] `[AI]` Run `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo xtask self-validate` on
      reconciled main; acceptance: exit `0` before Unit 2 starts. `[AC-01]` `[AC-02]` `[AC-06]`

> **Pause Safety**: Unit 1 is merged and no-tier canon cannot acquire adapter policy. Safe to stop. To resume:
> `git fetch origin && git rebase origin/main` in the reused worktree.

## Phase 2: Mermaid Source Classification — Unit 2

- [ ] `[AI]` In the clean reused worktree, run `git fetch origin --prune`, then
      `git switch -c worktree/harden-rhino-validation-edge-cases-mermaid origin/main`; acceptance:
      `git rev-list --left-right --count HEAD...origin/main` prints `0 0`. `[AC-06]`
- [ ] `[AI]` **CHARACTERIZE**: add `Real diagram content remains enforced` to `specs/behaviours/v0-4-contract.feature`,
      `tests/unit/bindings.rs`, `tests/integration/bindings.rs`, and `tests/e2e/bindings.rs`; run
      `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo test --test unit`; acceptance: both
      existing finding kinds are observable before production code changes. `[AC-05]`
- [ ] `[AI]` **RED**: add the outlines `Accessibility metadata is not diagram content`, with `accTitle`, single-line
      `accDescr`, and braced `accDescr` examples, and `Numeric HTML entities are not colour literals`, with decimal
      `&#128640;` and hexadecimal `&#x1F680;` examples, to the same four files; run
      `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo test --test unit`; acceptance: every
      metadata example and the decimal-entity example fail with the current false finding, and the result for the
      hexadecimal example is recorded here. `[AC-03]` `[AC-04]`
- [ ] `[AI]` **GREEN**: edit `colors`, `contains_color`, and `legibility` in `src/markdown/mermaid.rs` to skip
      single-line and braced accessibility metadata and to ignore valid numeric entity spans during colour-token
      scanning; run
      `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo test --test unit`; acceptance: every
      AC-03 and AC-04 example passes, AC-05 still reports both existing findings, and `accessibility` still reads the
      metadata. `[AC-03]` `[AC-04]` `[AC-05]`
- [ ] `[AI]` **REFACTOR**: centralize line-role and entity-span recognition in private helpers in
      `src/markdown/mermaid.rs`; run
      `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo fmt --all --check`, then
      `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo test --test unit`; acceptance: all
      Mermaid scenarios pass with no duplicated state tracking. `[AC-03]` `[AC-04]` `[AC-05]`
- [ ] `[AI]` Apply `repo-governance/workflows/docs-propagation.md` to `docs/reference/findings.md` and `CHANGELOG.md`;
      acceptance: the Mermaid section states that accessibility metadata and numeric entities are not inspected as
      content, and the changelog records the fix. `[AC-03]` `[AC-04]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo xtask test-quick`;
      acceptance: exit `0`. `[AC-06]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo xtask self-validate`;
      acceptance: exit `0`. `[AC-06]`
- [ ] `[AI]` Commit only Unit 2 plus plan progress using a Conventional Commit; acceptance: hooks pass and the commit
      contains only Unit 2 plus plan-record paths. `[AC-03]` `[AC-04]` `[AC-05]` `[AC-06]`
- [ ] `[AI]` Push the Unit 2 branch; acceptance: the pre-push hook passes and the remote head equals the local commit.
      `[AC-06]`
- [ ] `[AI]` Open the Unit 2 PR as a draft; acceptance: it targets `main` and records the unit's proof. `[AC-06]`
- [ ] `[AI]` Mark the Unit 2 PR ready only after the diff is final; acceptance: the exact-head ready-for-review quality
      run starts. `[AC-06]`
- [ ] `[AI]` Post one exact-head leak review; acceptance: it names the unchanged head and reports no finding. `[AC-06]`
- [ ] `[AI]` Rebase-merge Unit 2 after all five preconditions pass; acceptance: GitHub reports the PR merged. `[AC-03]`
      `[AC-04]` `[AC-05]` `[AC-06]`
- [ ] `[AI]` Reconcile primary `main` and the reused worktree with `origin/main`; acceptance:
      `git rev-list --left-right --count main...origin/main` prints `0 0`. `[AC-06]`

### Phase 2 Gate

- [ ] `[AI]` Run `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo xtask self-validate` on
      reconciled main; acceptance: exit `0` and both units are independently present. `[AC-03]` `[AC-04]` `[AC-05]`
      `[AC-06]`

> **Pause Safety**: all code units are merged and main is green. Safe to stop. To resume:
> `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo xtask self-validate`.

## Phase 3: Convergence and Execution Review

- [ ] `[AI]` Run `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo test --test integration`;
      acceptance: the integration adapter passes every changed scenario. `[AC-01]` `[AC-02]` `[AC-03]` `[AC-04]`
      `[AC-05]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo test --test e2e`;
      acceptance: the end-to-end adapter passes every changed scenario. `[AC-01]` `[AC-02]` `[AC-03]` `[AC-04]` `[AC-05]`
- [ ] `[AI]` Run `repo-governance/workflows/gherkin-implementation-review.md` for the scenarios this plan added to
      `specs/behaviours/v0-4-contract.feature` and store its row ledger at
      `local-tmp/harden-rhino-validation-edge-cases-gherkin-review.tsv`; acceptance: every expanded scenario has a
      `PASS` or justified `EXEMPT` row for unit, integration, and end-to-end. `[AC-06]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo xtask test-quick`;
      acceptance: exit `0` on the exact final main tree. `[AC-06]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo xtask self-validate`;
      acceptance: exit `0` on the exact final main tree. `[AC-06]`
- [ ] `[AI]` Run `repo-governance/workflows/plan-execution-check.md` against Phases 0–3 and record the verdict in this
      file; acceptance: it reports `PASS` with every AC terminal before knowledge capture. `[AC-06]`

### Phase 3 Gate

- [ ] `[AI]` Verify AC-01 through AC-06 each cite terminal evidence and no delivery unit or PR remains open;
      acceptance: the execution review permits knowledge capture. `[AC-06]`

> **Pause Safety**: substantive implementation is terminal and reviewable on main. Safe to stop. To resume:
> `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo xtask self-validate`.

## Phase 4: Knowledge Capture

- [ ] `[AI]` Apply the generalizability, secret/sensitivity, and repository-relevance gates to every `learnings.md`
      entry; acceptance: each entry has one safe owner or a discard reason. `[AC-06]`
- [ ] `[AI]` Route each surviving entry to exactly one durable owner, filing code/test follow-up work as a separate
      backlog plan rather than editing it inline; acceptance: `learnings.md` records every terminal destination.
      `[AC-06]`
- [ ] `[AI]` If execution produced no generalizable entry, write
      `No generalizable learnings — all observations were specific to these two corrected validator boundaries.`;
      acceptance: the running log has an explicit terminal state. `[AC-06]`

### Phase 4 Gate

- [ ] `[AI]` Read every `learnings.md` entry and verify it is promoted, filed, or discarded with reason; acceptance: no
      unresolved entry remains. `[AC-06]`

> **Pause Safety**: every learning is terminal and substantive work remains green. Safe to stop. To resume:
> `rg -n '^## Learning|Status|No generalizable' plans/in-progress/harden-rhino-validation-edge-cases/learnings.md`.

## Plan Archival

- [ ] `[AI]` Verify all substantive items are complete and the execution-check verdict is `PASS`. `[AC-06]`
- [ ] `[AI]` Move the plan to `plans/done/YYYY-MM-DD__harden-rhino-validation-edge-cases/` with the actual completion
      date; acceptance: exactly one done copy exists and no in-progress copy remains. `[AC-06]`
- [ ] `[AI]` Update `plans/in-progress/README.md`, `plans/done/README.md`, and every live reference; acceptance: no live
      link names the former in-progress path. `[AC-06]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo xtask test-quick`; acceptance: exit `0` from the
      archived state. `[AC-06]`
- [ ] `[AI]` Run `./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo xtask self-validate`; acceptance: exit `0` from
      the archived state. `[AC-06]`
- [ ] `[AI]` Run `git diff --check`; acceptance: exit `0` from the archived state. `[AC-06]`
- [ ] `[AI]` Commit the archival transaction using a Conventional Commit; acceptance: hooks pass and only archive/index
      paths are present. `[AC-06]`
- [ ] `[AI]` Push the archival branch; acceptance: the pre-push hook exits `0` and the remote head equals the local
      commit. `[AC-06]`
- [ ] `[AI]` Open the archival PR as a draft; acceptance: it targets `main` and records archived-state proof. `[AC-06]`
- [ ] `[AI]` Mark the archival PR ready only after the diff is final; acceptance: exact-head quality starts. `[AC-06]`
- [ ] `[AI]` Post one exact-head leak review for the archival PR; acceptance: it names the unchanged head and reports no
      finding. `[AC-06]`
- [ ] `[AI]` Rebase-merge the archival PR after all five preconditions pass; acceptance: GitHub reports it merged.
      `[AC-06]`
- [ ] `[AI]` Run dev artifact clean-up from the primary checkout; acceptance: the plan worktree and task branches are
      absent, `main` equals `origin/main`, and no untracked secret or unrelated artifact was removed. `[AC-06]`
