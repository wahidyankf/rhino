# Delivery Record

## Execution Checkout

Use the one contained checkout `worktrees/regular-file-gate-inputs/`, initially on `worktree/regular-file-gate-inputs`.
The delivery orchestrator owns branch transitions, commits, PRs, merges, releases, and reconciliation. Reuse this
directory across delivery units. Production and test/config changes go to `swe-*` executors; the maker owns initial plan
authorship, independent checkers judge, and separate fixers repair frozen rows.

Baseline inspected: `c8d49ff`. The setup manager installed locked contributor tooling and activated hooks with a
supported Node runtime. The baseline quick gate passed before plan authorship: 156 library/unit-adapter tests, 19
behavior-coverage checks, 99.24% line coverage. These are baseline results, not feature evidence.

The user approved implementation and release of the additive input contract. The owner plan directs no consumer edits.
Read `learnings.md` on resume. All commands below are prescribed work unless a dated result explicitly says they ran.

## Delivery Units

- **U0 — Owner plan.** Owner: RHINO planning. Outcome: these six documents and the stage map land alone on
  `origin/main`. Proof: merged plan PR and ancestry check. Rollback: a dedicated revert PR; no product state changes.
- **U0A — Published governance adoption.** Owner: RHINO rules adoption, delivered separately from this product plan.
  Outcome: RHINO explicitly copies the published canonical catalog correction after U0 lands. Its own delivery records
  the exact mapping, source revision, bindings, and checks. Proof consumed here: merged adoption PR, verified published
  source identity, and primary `main` reconciliation. Rollback: a dedicated adoption revert PR; no product state
  changes.
- **U1 — Feature and release preparation.** Owner: RHINO implementation/release. Outcome: new kind, safe selection,
  empty-mutation handling, regressions, as-built specs, generated schema, docs, next minor version, and note baseline.
  Proof: required local/hosted gates and merged feature PR. Rollback before publication: dedicated revert PR restoring
  the whole unit; after publication, retain immutable assets and ship a new patch.
- **U2 — Execution closure and archival.** Owner: RHINO plan execution. Outcome: routed knowledge, execution verdict,
  expendable-artifact cleanup, and dated archive delivered through the owner route. Retain the one checkout until this
  archival PR merges, then remove it and its task branches. Record the terminal removal proof in that PR.

The canonical catalog correction is published first at P2A. U0 then lands alone with owner governance adoption pending;
U0A follows from a clean sync of the same checkout. Only then does the Plan Quality Gate run under the selected Upstream
Tool Defects route. This does not skip the gate: no implementation starts until its verdict is recorded and every open
blocking finding has an owner. Do not combine U0 with U0A, stack their PRs, create a second checkout, or stash the plan
to deliver adoption ahead of it.

## Pause Safety

At a pause, record current commit/branch, last completed item and evidence, next ready item, active failed gate, frozen
ledger identity, and any active child/workflow. Keep raw outputs under ignored `local-tmp/`, sanitize durable evidence,
and preserve the failed checkout. Do not duplicate this checklist in a scratch status file. A tick and its dated result
are one update, performed only after the stated proof succeeds.

## Command Environment

Use the supported installed contributor Node runtime for hooks; do not alter package constraints to work around a broken
host runtime. Define these shell helpers in the execution session, not in repository configuration:

```sh
S() { ./hippo run --class ephemeral --resource-tier standard --disk-path . -- "$@"; }
L() { ./hippo run --class ephemeral --resource-tier light --disk-path . -- "$@"; }
H() { ./hippo run --class ephemeral --resource-tier heavy --disk-path . -- "$@"; }
W() { ./hippo run --class transactional --resource-tier standard --disk-path . -- "$@"; }
```

Every named helper invocation below includes that HIPPO boundary. Exit `124`, `125`, and `2` follow the owner's
Resource-Aware Development instructions; never retry as a bypass. Git/GitHub delivery commands use existing safety hooks
and the exact-head leak review. Hosted full E2E remains scheduled and is never a phase completion requirement.

## Parallelization Model

The owner permits at most three delegated agents. Use fewer when the parent orchestration consumes shared slots or
resource admission requires it. Source and test changes share files, so T1–T4 run serially in the one checkout.
Read-only reviewers may run concurrently against one frozen revision. Documentation review and code review can read the
same stable candidate; their fixers serialize because edits would change that candidate.

```text
P0 -> P1 -> P2A -> P2 -> P3 -> P4 -> G0 -> Q0 -> T0
T0 -> T1-RED -> T1-GREEN -> T1-REFACTOR -> T2-RED
T2-RED -> T2-GREEN -> T2-REFACTOR -> T3-RED
T3-RED -> T3-GREEN -> T3-REFACTOR -> T4 -> T5 -> D0 -> D1
D1 -> V0 -> V1 -> V2 -> R0 -> R1 -> R2 -> R3 -> R4
R4 -> C0 -> C1 -> C3 -> C2 -> C4 -> C5
```

Each arrow reads the predecessor's output: published canonical source before final plan checks, the plan-only merge
before clean-checkout governance adoption, adopted governance before plan quality review, accepted plan before
implementation, typed model before selection wiring, selection before empty behavior, completed behavior before
docs/release, merged candidate before tag, and published proof before closure. T4's failing host regression is authored
during T2-RED, before production metadata selection; T4 records its final green and preservation proof. C3 removes
expendable artifacts and prepares the final PR cleanup procedure before C2's terminal execution review, which verifies
all cleanup already due. The one checkout, required evidence, and prepared PR body remain available while later delivery
needs them. C4 verifies the actual archival PR carries that procedure. C5 is the terminal cleanup node, dependent on
every delivery node; it removes the retained checkout and task branches after the archival merge. It records evidence in
the merged archival PR and makes no further archived-plan edit. Recovery nodes activate only on their declared trigger.

## Phase 0 — Plan-Only Delivery

- [x] [AI] P0 — Inspect owner instructions and baseline implementation; paths: the source, specs, schema, and workflows
      listed in `tech-docs.md`; proof: `git rev-parse --short HEAD` returns `c8d49ff`, and the design records the
      inspected symbols and existing regression names. Acceptance: AC-RF-01, AC-RF-07.
  > 2026-10-15: read the canonical maker, planning/plan conventions, typed input model, Tree methods, disk queries,
  > mutation branches, adapters, schema checks, C4 model, and release workflow. No product files changed.
- [x] [AI] P1 — Close the pre-write decision record in this plan's `README.md`; proof: the selected additive kind,
      approved design, sources, compatibility, and release scope are recorded. Acceptance: AC-RF-01, AC-RF-07, AC-RF-11.
  > 2026-10-15: carried forward the user's actual selected new-kind option and implementation instruction. No renewed
  > approval or unstated policy choice was invented.
- [x] [AI] P2A — Confirm the separately delivered governance correction is published by its canonical catalog; proof:
      record the actual published catalog commit and its reachability from that catalog's `origin/main`. Owner adoption
      remains explicitly pending until G0, after the plan-only merge. This is an externally delivered prerequisite, not
      authority for this product plan to edit another repository. Acceptance: AC-RF-12.
  > 2026-10-15: [catalog PR 116](https://github.com/wahidyankf/ose-rules/pull/116) merged at 12:52:04 UTC. Published
  > source `769d6d21f3f48947d17410a1dcb10b5d3f9bd27a` is reachable from its `origin/main`; the explicit
  > `git merge-base --is-ancestor` check exited 0. Read the five canonical source documents and their generated
  > provenance entries at that commit. Owner adoption remains pending at G0; no RHINO governance files were copied.
- [x] [AI] P2 — After P2A, finalize the six documents and the post-write gate; paths: this plan folder and
      `plans/in-progress/README.md`; command: `S cargo xtask self-validate`; proof: exit 0 and a separate dated
      post-write decision record below, retaining the explicit pending G0 obligation. Acceptance: AC-RF-12.
  > 2026-10-15: scoped Prettier formatting of the six plan documents and stage map exited 0. On unchanged baseline
  > `c8d49ffc92a53719f084adde5b022cb1fa5564ba` with this seven-file plan draft, `S cargo xtask self-validate` exited 0;
  > every declared gate reported passed. Local evidence is `local-tmp/regular-file-gate-inputs/plan-final-format.log`
  > and `plan-final-self-validate.log`, with corresponding `.exit` files recording 0. The post-write decision below
  > remains closed; G0 adoption is still pending. No product or governance implementation was added.
- [x] [AI] P3 — Run Docs Propagation for the plan-only change, preserving maker/checker/fixer responsibilities; paths:
      the six plan docs and stage map; command: `S cargo xtask self-validate`; proof: terminal propagation status and
      exit 0, with any unexercised execution commands explicitly identified as prescribed steps. Acceptance: AC-RF-12.
  > 2026-10-15: final frozen-input Docs Propagation returned `no-change`; review of all seven scoped Markdown files
  > found no stale claim or navigation defect. No content correction was needed, so P2's final-input
  > `S cargo xtask self-validate` result and scoped formatting results remain the applicable checks: exit 0, with all 19
  > declared gates passed. Their logs and exit statuses are listed under P2. Implementation, feature-verification,
  > release, and publishing commands below remain prescribed work and were not exercised.
- [x] [AI] P4 — Deliver U0 through Worktree to Pull Request with required public safety, exact-head fresh-context leak
      review, review, and CI; paths: plan and stage map only; proof: merged PR,
      `git merge-base --is-ancestor <plan-commit> origin/main` exit 0, then primary
      `git rev-list --left-right --count HEAD...origin/main` reports `0 0`. Include no governance-adoption changes in
      this PR; retain G0 as a separate delivery. Acceptance: AC-RF-12.
  > 2026-10-15: [plan PR 154](https://github.com/wahidyankf/rhino/pull/154) merged at 15:02:34 UTC as
  > `11da192fa4b012d9aa58bec92a6ad6feca85f703`. Reviewed head: `6aa3343`; reviewed and landed trees both equal
  > `0cec5f70b82e136b9e58561b014b7f4336968012`. The plan commit is an ancestor of `origin/main` (check exit 0); primary
  > `main` matched `origin/main` at `0 0`.

Post-write decision gate — 2026-10-15: complete-draft inspection found the cleanup/archive ordering conflict. The owner
selected A: keep the one checkout through the archival PR merge, then remove it and its task branches. Record final
evidence in that PR. The competing cleanup-first route was rejected because it would require another archival route. The
catalog correction publishes before final plan checks and the plan-only merge; RHINO adopts it afterward through its own
separate PR, before the Plan Quality Gate. This ordering lets each transaction land from the same cleanly synced
checkout. The material branch is closed, source publication is proven, and P2's final deterministic checks passed for
the original seven-file plan input; P3's frozen-input review returned `no-change` on that same input. Subsequent
delivery and the later Q0 verdict are recorded below. Q0 returned `BLOCKED` because structural tooling is unavailable;
the semantic audit found no issues. The Q0 gate caller owns the unmet structural proof. Under the existing advisory
contract, approved work continued after T0 returned no-change. T1–T5 implementation is complete, and the current phase
is as-built documentation and release preparation. The structural proof remains unmet.

Governance source: `769d6d21f3f48947d17410a1dcb10b5d3f9bd27a` (P2A). RHINO adoption merged as
`5a3fb75b50113ed94d8a19aa78239c0a275bfe8e` (G0); the adoption proof follows below.

The source inspection covered `repo-governance/conventions/structure/plans/014-dependency-graph-and-recovery.md`,
`repo-governance/development/agents/planning-capabilities/007-execution-review.md`,
`repo-governance/workflows/plan/plan-execution-check.md`, `repo-governance/workflows/plan/plan-execution.md`, and
`.agents/agents/plan-execution-checker.md` at that exact published revision. The five changed generated provenance
surfaces agree on the canonical execution-checker digest
`9ce2714d431316526d9be40e46ea5ed4375e92fdde7bcd7e1490afe72bb45cb9`. These are source observations, not a claim that
RHINO's differently located governance or generated bindings have already adopted them.

## Phase 1 — Governance Adoption and Plan Quality

P4 and G0 are complete. Q0 returned a terminal `BLOCKED` verdict because structural tooling is unavailable. Its semantic
audit found no issues; the missing structural proof is assigned to the Q0 gate caller. Under the existing advisory
contract, T0 returned no-change and the approved work proceeded to T1–T5 while that proof remains unmet.

- [x] [AI] G0 — From the same now-clean task checkout, fetch and sync from refreshed `origin/main`, inspect the incoming
      plan merge, then deliver the separately owned U0A governance-adoption PR from P2A's actual published catalog SHA.
      Proof: its exact source mapping and generated bindings verify, owner checks and required review/CI pass, adoption
      merges, `git merge-base --is-ancestor <adoption-commit> origin/main` exits 0, and primary
      `git rev-list --left-right --count HEAD...origin/main` reports `0 0`. Record the published source/adoption
      revisions here only after these proofs exist. Do not run Q0 before G0. Acceptance: AC-RF-12.

  > 2026-10-15: [adoption PR 155](https://github.com/wahidyankf/rhino/pull/155) merged at 15:52:33 UTC as
  > `5a3fb75b50113ed94d8a19aa78239c0a275bfe8e`, adopting catalog source `769d6d21f3f48947d17410a1dcb10b5d3f9bd27a`.
  > Reviewed head: `495a485`; reviewed and landed trees both equal `ff3e7377c37773a47ed947502562896c79b49bae`. Ready CI
  > run `38763091941` passed all jobs; all 19 native gates passed on the adoption candidate. Fresh merge leak review
  > passed all three counts at 0, with zero final API threads and the reviewed head, base, text, and clean state
  > unchanged. The adoption commit is an ancestor of `origin/main` (check exit 0). Primary `main` and `origin/main` both
  > resolved to this merge, with divergence `0 0`.

  > After the merge, the same task checkout was clean-synced to `5a3fb75` on
  > `worktree/regular-file-inputs-implementation`; its incoming tree diff against the reviewed adoption candidate was
  > empty. No second checkout was created.

- [ ] [AI] Q0 — After G0, sync this same task checkout from refreshed `origin/main`, inspect the incoming adoption diff,
      and run Plan Quality Gate on `plans/in-progress/regular-file-gate-inputs` with `mode normal`, `max-cycles 3`;
      proof: structural checks exit 0, a frozen checker ledger and separate fixer outcomes exist, and a terminal verdict
      is recorded here. Assign each open blocking row to a concrete owner before T0. Acceptance: AC-RF-12.

plan-quality-gate: BLOCKED (unavailable structural tooling; 1 cycle; 0 semantic findings; entry GREEN; structural
subcheck unavailable; exit not due). The Q0 gate caller owns the unmet structural proof; under the advisory contract,
approved work continued at T0, which returned no-change. The unmet proof remains with the Q0 gate caller.

## Phase 2 — Test-First Implementation

Q0 is terminally `BLOCKED`; its missing structural proof is owned by the gate caller. Under the existing advisory
contract, approved work continued after T0 returned no-change. T1–T5 implementation is complete. Historical RED and
refactor chronology are recorded beneath each item. No general validator implementation is in scope, and the structural
proof remains unmet. Every RED names an intended behavioral failure and must compile/run far enough to establish it; a
typo, undefined Gherkin step, or missing fixture is not the stated red. Keep focused outputs with a revision and
timestamp.

- [x] [AI] T0 — Start Rules Propagation for the changed gate acceptance rule before product edits; paths:
      `src/v0_4/config.rs`, `src/v0_4/gates.rs`, and affected existing rule statements; proof: a placement record
      freezes additive opt-in, unchanged legacy behavior, empty mutation scope, and enforcement at config/runtime tests.
      Record terminal status, including `no-change` for governance when existing rules suffice. Acceptance: AC-RF-01,
      AC-RF-07.
  > 2026-10-15: Rules placement returned `no-change`: this opt-in product capability adds no contributor obligation.
  > Existing [public-contract rules](../../../repo-governance/development/public-contract.md),
  > [software-quality enforcement](../../../repo-governance/development/software-quality-enforcement.md),
  > [specification maintenance](../../../repo-governance/development/specification-maintenance.md), and
  > [architecture specifications](../../../repo-governance/development/architecture-specifications.md) cover its
  > compatibility, read-only boundaries, and spec relationships. No governance or harness edit is required. The full
  > placement proof is in `local-tmp/regular-file-gate-inputs/t0-rules-placement.md`.
- [x] [AI] T1-RED — Write configuration acceptance/refusal Gherkin first in `specs/behaviours/regular-files.feature`;
      add its README map entry and unit/integration/E2E bindings; tests first in `src/v0_4/config.rs` and
      `tests/v0_4_schema.rs`; commands: `S cargo test --lib regular_files`, `S cargo test --test unit`,
      `S cargo test --test v0_4_schema`; proof: intended new-kind acceptance cases fail because the current kind is
      unknown; unsupported-source tests retain explicit expected refusal. Acceptance: AC-RF-01, AC-RF-02, AC-RF-10.
  > Initial unit RED (UTC timestamp unavailable; see `local-tmp/regular-file-gate-inputs/implementation-evidence.md`):
  > `S cargo test --test unit -- --nocapture` returned 101. Of 224 expanded cases, 222 passed; the new Git-index and
  > explicit-range acceptance cases failed because `regular-files` was still an unknown variant. The typed model and
  > external-schema acceptance REDs also reached their intended schema/reader rejection. All three acceptance REDs were
  > resolved by T1-GREEN below.
- [x] [AI] T1-GREEN — Add the typed kind, `.paths` validation, compatible sources, and generated schema; paths:
      `src/v0_4/config.rs`, `schemas/repo-config/v2.schema.json`; commands: `S cargo xtask schema`, then the T1-RED
      commands; proof: all focused cases pass with legacy cases unchanged. Acceptance: AC-RF-01, AC-RF-02, AC-RF-10.
  > The unit, typed model, and external-schema acceptance REDs reached the intended unknown-kind/schema-rejection
  > failures. Unsupported-source refusal examples were added after the kind became readable; they passed as GREEN
  > characterization and are not claimed as RED cases.
- [x] [AI] T1-REFACTOR — Remove duplication in the new validation branches without changing contract; tests first:
      T1-RED paths; production: `src/v0_4/config.rs`; commands: T1-RED commands plus `L cargo fmt --all --check`; proof:
      pass with no new acceptance or projection difference. Acceptance: AC-RF-01, AC-RF-02, AC-RF-10.
  > Shared `.paths` field validation was factored across `Files` and `RegularFiles` in the T1 GREEN change; focused
  > acceptance tests and formatter checks passed afterward. There was no separate post-GREEN refactor run.
- [x] [AI] T2-RED — Extend the new feature with selection/projection scenarios and concrete shared fixture support;
      tests first: `src/runtime/git_paths.rs` byte fixtures, `src/runtime.rs` port/memory tests, `src/v0_4/gates.rs`
      resolver tests, and the real nine-link preservation tests in `tests/integration/policy.rs`. Use minimal compiling
      entry points/default refusal where needed before implementation. Commands: `S cargo test --lib regular_files`,
      `S cargo test --test unit`, `S cargo test --test integration regular_files`; proof: acceptance fails on missing
      regular-selection capability or incorrect eligibility, and the host regression cannot format the selected source
      beside links. The tests expose that behavior, not an unrelated setup error. Acceptance: AC-RF-03, AC-RF-04,
      AC-RF-05, AC-RF-08, AC-RF-09.
- [x] [AI] T2-GREEN — Implement the private parser, default-refusing ports, explicit memory selections/rerooting, disk
      raw-metadata queries, and resolution through `ResolvedInput::Files`; paths: `src/runtime/git_paths.rs`,
      `src/runtime.rs`, `src/runtime/disk.rs`, `src/v0_4/gates.rs`; command: the three T2-RED commands; proof:
      destination mode, snapshot, projection, malformed metadata, and real index-preservation tests pass. Acceptance:
      AC-RF-03, AC-RF-04, AC-RF-05, AC-RF-08, AC-RF-09.
- [x] [AI] T2-REFACTOR — Move the unchanged pure path-containment helper into the private parser sibling and simplify
      metadata plumbing; tests first: T2's paths; production: `src/runtime/git_paths.rs`, `src/runtime/disk.rs`;
      commands: T2-RED commands plus `L cargo fmt --all --check`; proof: unchanged passing behavior, no
      filesystem/process access in parser unit tests, and no new coverage exclusion. Acceptance: AC-RF-03, AC-RF-09.
  > The parser RED was one valid-record byte case; the other seven matrix cases were compile-checked before
  > implementation, not run as RED. The helper relocation occurred during GREEN, before parser GREEN, rather than as a
  > post-GREEN refactor. The complete parser matrix and focused adapters passed afterward; chronology is recorded in
  > `local-tmp/regular-file-gate-inputs/implementation-evidence.md`.
- [x] [AI] T3-RED — Write both empty-surface scenarios, mixed-input nonempty case, and empty-check case in the new
      feature; tests first: `src/v0_4/gates.rs`, all declared bindings/drivers/support paths in `tech-docs.md`;
      commands: `S cargo test --lib regular_files`, `S cargo test --test unit`; proof: empty mutations expose an
      attempted port/child invocation instead of skip-success; mixed and check cases assert actual invocation.
      Acceptance: AC-RF-06, AC-RF-07.
- [x] [AI] T3-GREEN — Add the combined-empty-selection short circuit only for mutations declaring the new kind;
      production: `src/v0_4/gates.rs`; command: both T3-RED commands; proof: both surfaces skip the child, while mixed,
      legacy files-only, and check cases preserve their invocation behavior. Acceptance: AC-RF-06, AC-RF-07.
- [x] [AI] T3-REFACTOR — Simplify the new skip predicate without broadening it; tests first: T3's paths; production:
      `src/v0_4/gates.rs`; commands: T3-RED commands plus `L cargo fmt --all --check`; proof: unchanged passing results.
      Acceptance: AC-RF-06, AC-RF-07.
  > The empty legacy `Files`-only mutation characterization was added after GREEN; its test confirms both mutation ports
  > still run with empty selections. The full focused result and chronology are in the ignored implementation evidence.
- [x] [AI] T4 — Complete host-boundary GREEN/characterization evidence in `tests/integration/policy.rs` and
      `tests/e2e/policy.rs`, with driver/support changes already specified; commands:
      `S cargo test --test integration regular_files`, `S cargo test --test integration disk_mutation`,
      `S cargo test --test e2e regular_files`; proof: the T2 host red is now green, all nine links retain raw
      targets/modes, partial staging and unrelated bytes survive, immutable-head selection ignores working types,
      type/mode/rename/unmerged cases pass, and legacy local link refusal remains. Acceptance: AC-RF-03, AC-RF-04,
      AC-RF-06, AC-RF-07, AC-RF-08, AC-RF-09.
- [x] [AI] T5 — Run the unchanged quick gate and exact schema check; candidate paths: all U1 code, tests, schema, and
      fixtures; commands: `H cargo xtask test-quick`, `S cargo xtask schema --check`; proof: both exit 0 with coverage
      at least 99% and unchanged exclusions. Acceptance: AC-RF-01 through AC-RF-10.

## Phase 3 — As-Built Contracts and Release Preparation

T5 passed. D0/D1 content work is updated, and the independent architecture Final Review passed. The independent
implementation review found all 36 expanded scenario rows passing with no open HIGH or new material findings; its
remaining MEDIUM advisory is the historical TDD/helper chronology recorded above. The final quick gate passed at
2026-10-15 17:15:05–17:15:23 UTC: 171 library tests, the unit corpus, and 19 coverage contracts; line coverage was
99.12%, with the existing floor and exclusions unchanged. The exact schema check also passed. The full
`cargo xtask self-validate` is deferred until the immutable candidate is committed; the coordinating caller will run it
once for D0/D1 verification and V1 entry. The example producer passed the focused `cargo test --test v0_4_schema` parity
test (5/5, exit 0 at 2026-10-15 17:21:10–17:21:15 UTC), but the consumer-specific formatter lifecycle commands in the
how-to remain unexercised. CHANGELOG and release metadata stay with the assigned releaser.

- [ ] [AI] D0 — Synchronize `specs/architecture.md` Components/Boundaries with the runtime parser and Git adapter, plus
      their gate-resolution relationship; run Gherkin Implementation Review for every new expanded scenario at each
      applicable adapter; commands: `S cargo test --test coverage`, `S cargo xtask self-validate`; proof: semantic
      review rows contain no FAIL, every scenario binds at unit, current corpus remains, and commands exit 0.
      Acceptance: AC-RF-01 through AC-RF-10.
- [ ] [AI] D1 — Run Docs Propagation over the complete U1 change; paths: `README.md`, `CHANGELOG.md`,
      `docs/reference/v0-4-configuration.md`, `docs/how-to/wire-rhino-into-your-gates.md`, schema example, and as-built
      specs; command: `S cargo xtask self-validate`; proof: terminal status, canonical reference/example consistency,
      safe commands exercised or marked unexercised, and exit 0. Acceptance: AC-RF-01, AC-RF-02, AC-RF-06, AC-RF-07,
      AC-RF-10.
- [ ] [AI] V0 — Verify next unused minor tag locally/remotely and prepare expected `0.13.0`; paths: `Cargo.toml`,
      `Cargo.lock`, `CHANGELOG.md`, `.github/workflows/release.yml`; commands: `git tag -l 'v0.13.0'` and
      `git ls-remote --tags origin 'v0.13.0'`; proof: neither names an existing tag, version files agree, and the
      workflow explicitly uses `v0.12.0` as the new version's notes baseline. If occupied, stop this item and record the
      next unused compatible version before editing its baseline. Acceptance: AC-RF-11.
- [ ] [AI] V1 — Run Docs Quality Gate with subject `all`, mode `normal`, max-cycles `3`, before the release-prep PR;
      paths: its complete docs/spec scope; proof: terminal verdict recorded with separate fixer outcomes and every open
      blocking row owned. Fix any documented behavior the binary violates before release. Acceptance: AC-RF-11.
- [ ] [AI] V2 — Deliver U1 through Worktree to Pull Request after current-main sync; paths: complete coherent U1
      surface; commands: `H cargo xtask test-quick`, `S cargo xtask schema --check`, affected focused integration/E2E
      commands from T4; proof: pass on the candidate, independent review, exact-head fresh-context leak review, required
      hosted CI, merged PR, task ancestry on refreshed `origin/main`, and primary `main...origin/main` equals `0 0`.
      Acceptance: AC-RF-01 through AC-RF-12.

## Phase 4 — Release Cut

Blocked by V2. Release authorization is already provided; do not ask again solely because publication matters.

- [ ] [AI] R0 — Admit the exact merged release revision from clean primary `main`; paths: release manifests/schema;
      commands: `git status --porcelain`, `git merge-base --is-ancestor HEAD origin/main`,
      `git rev-list --left-right --count HEAD...origin/main`, `H cargo xtask test-quick`,
      `S cargo xtask schema --check`; proof: clean tree, ancestry exit 0, `0 0`, and both gates pass on that revision.
      Acceptance: AC-RF-11, AC-RF-12.
- [ ] [AI] R1 — Build/check the local release archive and schema using Release Cut; paths: ignored `dist/` assets;
      commands: `H cargo xtask dist`, `S cargo xtask checksums`, `S cargo test --package xtask` with declared release
      version/commit environment; proof: local native archive/schema/checksum tests pass and unpacked `version --json`
      identifies the intended version and commit. Acceptance: AC-RF-11.
- [ ] [AI] R2 — Generate the exact release-note preview from prior stable `v0.12.0`, screen notes and the chosen tag
      through `scripts/public-safety/public-safety.sh --surface release`, and publish the tag through Release Cut;
      proof: tag absent before creation, screen exit 0, tag points to the exact release commit, release workflow starts.
      Keep exact commands and sanitized results here; do not commit raw generated notes. Acceptance: AC-RF-11.
- [ ] [AI] R3 — Observe all four native release matrix legs and aggregate publication; paths: workflow run and assets;
      proof: successful provenance, native archive identity/artifact tests for every target, aggregate release suite,
      four archives, schema, and `checksums.txt`. Poll hosted state at the owner's three-minute interval. Acceptance:
      AC-RF-11.
- [ ] [AI] R4 — Download published archives/schema/checksums into ignored scratch storage and verify every asset digest;
      execute the matching native archive's `version --json`; proof: all manifest comparisons pass and downloaded native
      identity matches the tag/commit, with the other native identities established by R3. Hand the verified release
      URL, version, commit, schema, and checksums to the coordinating caller. Acceptance: AC-RF-11.

## Phase 5 — Substantive Reconciliation and Archival Admission

Blocked by R4; release verification precedes substantive reconciliation. C3's eligible cleanup and prepared final PR
procedure precede C2's terminal execution review. Final checkout and task-branch cleanup remain C5.

- [ ] [AI] C0 — Reconcile every acceptance criterion and dormant recovery trigger against recorded evidence; paths: this
      plan's `delivery.md`; proof: AC-RF-01 through AC-RF-12 each has a met or justified inapplicable disposition,
      substantive tasks through R4 are terminal, and no unowned blocking gate finding remains. The final archival merge
      and checkout-removal proofs remain explicitly assigned to C4 and C5. Acceptance: AC-RF-12.
- [ ] [AI] C1 — Resolve every `learnings.md` entry to one durable owner or a reasoned discard; search `plans/ideas/`
      before adding a follow-up; proof: no pending disposition remains and all named owners exist. Acceptance: AC-RF-12.
- [ ] [AI] C3 — Clean expendable task-owned scratch and release downloads, and record the final checkout/branch removal
      obligation and prepared archival PR body text before execution review; paths: only owned ignored artifacts and
      this delivery record; proof: cleanup command results identify removed artifacts, required evidence and the
      prepared PR body remain available, unrelated work remains, and the recorded body text names C5's post-merge
      procedure and required proof. C4 will verify the actual opened PR carries this text. Retain the one checkout and
      task branches while U2 needs them; do not delete an artifact still required by C2 or C4. Acceptance: AC-RF-12.
- [ ] [AI] C2 — After C0, C1, and C3, run Plan Execution Check in its fixed order; paths: the six plan documents and
      their cited implementation/evidence; proof: terminal execution verdict recorded here verifies substantive work,
      acceptance evidence, resolved learnings, all cleanup already due, and the recorded C5 procedure for checkout/task
      branches retained through U2's merge under the adopted ordering. Archival requires a permitting verdict; the
      deferred C5 removal is not claimed complete. Acceptance: AC-RF-12.

plan-execution-check: pending.

### Conditional Recovery

- [ ] [AI] X1 — Trigger: an owned pre-publication candidate cannot pass its required gates; decision owner: delivery
      orchestrator. Procedure: preserve failure evidence and checkout, diagnose the cause, repair owned code or revert
      the complete unshipped delivery unit through a dedicated PR; never suppress a check or drop links. Proof: the
      required gate exits 0 on the repaired/reverted candidate and disposition names the revision. Acceptance: AC-RF-07,
      AC-RF-12.
- [ ] [AI] X2 — Trigger: a published asset fails verification; decision owner: release orchestrator. Procedure: stop
      verified-release handoff, preserve the immutable tag/assets, diagnose and prepare a new patch version, repeat the
      owner's release-prep/Review/Release Cut sequence, then verify new assets. Proof: the replacement version's
      downloaded bytes and identities pass; the previous assets remain unchanged. Acceptance: AC-RF-11.
- [ ] [AI] X3 — Trigger: Rules/Docs/Plan Quality Gate reaches its declared ceiling with blocking rows; decision owner:
      gate caller. Procedure: freeze terminal verdict, assign each row a plan item, idea brief, or issue; do not extend
      the cycle budget or let the maker repair frozen findings. Proof: every row has an owner and execution records
      whether its affected outcome can proceed. Acceptance: AC-RF-12.

At reconciliation, every dormant X item receives a dated evidenced `Not triggered` disposition and stays unticked; an
activated item records the actual procedure and proof. No dormant trigger is silently treated as executed.

## Phase 6 — Archival

Blocked by C0–C3 and a permitting execution verdict. This is separate from substantive completion.

- [ ] [AI] C4 — Move the six-document folder into the collision-refusing local completion-date path under `plans/done/`,
      update both stage maps and live inbound links, set README `Completed`, and deliver U2 through the owner PR route;
      command: `H cargo xtask test-quick` from the archived state; proof: no destination existed, gate exit 0, final
      delivery reaches `origin/main`, primary divergence is `0 0`, and the actual opened archival PR body carries C3's
      prepared C5 post-merge cleanup procedure and required proof before merge. The recorded execution verdict must
      permit this ordering. Acceptance: AC-RF-12.

### Terminal Cleanup After the Archival Merge

- [ ] [AI] C5 — After C4's PR merges, remove this task's checkout and local/remote task branches under Dev Artifact
      Clean-Up; proof: verify no unpushed work or running process remains first, then record absence from
      `git worktree list`, local branch listing, and remote branch listing in the merged archival PR. Preserve unrelated
      worktrees and branches. Recheck primary `main...origin/main` equals `0 0`. Do not edit the archived plan merely to
      add these results; the PR is the final evidence surface. Acceptance: AC-RF-12.
