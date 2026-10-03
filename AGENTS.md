# RHINO Rules

RHINO is a repository-hygiene validator. Every rule lives in [`repo-governance/`](repo-governance/README.md), stated
once.

## The Product

- Ship no repository-specific default, repository, harness, or organization name in `src/`; the
  [vision](repo-governance/vision/README.md) draws the policy/behaviour line.
- Exit codes, `version --json`, commands, flags, and configuration keys are
  [a public contract](repo-governance/development/public-contract.md); moving is major-version work.
- Tree validators are read-only, network-free, process-free, and path-contained. Declared gate children, toolchain
  probes/provision, and adapter generation use separate narrow boundaries. `#![forbid(unsafe_code)]` and those
  boundaries are [enforced by tests](repo-governance/development/software-quality-enforcement.md).

## Specifications

- `specs/` is canonical. Assess [behaviours and architecture](repo-governance/development/specification-maintenance.md)
  before every change; record a verified no-op over churn.
- Gherkin first, prove the [red for the stated reason](repo-governance/development/test-driven-development.md), then
  implement. Write scenarios and bindings under [BDD](repo-governance/development/behaviour-driven-development.md); keep
  [the C4 model](repo-governance/development/architecture-specifications.md) true.
- Every scenario binds at unit adapter without exemption;
  [review changed Gherkin](repo-governance/workflows/quality/gherkin-implementation-review.md).

## Testing

- `cargo xtask test-quick` is [the quick gate](repo-governance/development/quality-gates.md); integration and
  [end-to-end](repo-governance/development/end-to-end-testing.md) never run in a hook.
- The 99% coverage floor and two declared exclusions are
  [not negotiable](repo-governance/development/software-quality-enforcement.md).
- Guard local compute with [`./hippo`](repo-governance/development/resource-aware-development.md), never bypassed: `124`
  read its reason, `125` replan or drain, `2` fix the call. Route
  [HIPPO and FERRET defects](repo-governance/development/upstream-tool-defects.md) to their owners.

## Change Discipline

- [Understand, reuse, minimize, verify](repo-governance/principles/minimal-sufficiency.md). A new dependency carries
  [its own record](repo-governance/development/dependency-selection.md).
- Keep `README.md`, `docs/`, and `CHANGELOG.md`
  [true to the binary](repo-governance/workflows/quality/docs-propagation.md) under
  [Diátaxis](repo-governance/conventions/documentation-architecture.md). Follow
  [code clarity](repo-governance/development/code-clarity.md), [English](repo-governance/conventions/language.md),
  [Markdown](repo-governance/conventions/markdown-line-length.md),
  [links](repo-governance/conventions/markdown-links.md), and
  [directory maps](repo-governance/conventions/directory-maps.md).
- Dispatch coding work to fitting `swe-*` agents per [SWE Delegation](repo-governance/conventions/swe-delegation.md).
- Cap [delegated agents](repo-governance/development/delegated-agent-concurrency.md) at three. Track work in
  [granular items](repo-governance/conventions/task-tracking.md) and a written progress record, preserve rules through
  [compaction](repo-governance/principles/governance-continuity.md),
  [ask last](repo-governance/conventions/last-resort-questions.md), and name files in
  [lowercase kebab-case](repo-governance/conventions/file-naming.md).
- Plans are working records under [`plans/`](plans/README.md), never architecture: the
  [plans convention](repo-governance/conventions/plans.md),
  [local additions](repo-governance/conventions/plan-lifecycle.md),
  [specification changes](repo-governance/conventions/plan-specification-changes.md), and
  [validator contract](repo-governance/conventions/plan-validator-contract.md) structure answers to.
- [Quality gates](repo-governance/development/workflow/quality-gate-contract.md) need explicit request or a listed
  caller; the [execution check](repo-governance/workflows/plan/plan-execution-check.md) blocks archival.

## Version Control

- `main` refuses direct pushes. Work only at `{repository location}/worktrees/<name>/`, never a sibling `*-worktrees/`
  path, and integrate by [pull request](repo-governance/workflows/maintenance/worktree-to-pull-request.md) under
  [the integration path](repo-governance/conventions/integration-path.md), one
  [delivery unit](repo-governance/conventions/pull-request-boundaries.md) each, with an accurate
  [body](repo-governance/conventions/pull-request-body.md) and
  [merge preconditions](repo-governance/conventions/pull-request-merge.md).
- Make [thematic commits](repo-governance/conventions/thematic-commits.md) when
  [authorized](repo-governance/conventions/commit-authorization.md). Never commit prohibited
  [data](repo-governance/conventions/public-repository-data-safety.md); [public safety](scripts/public-safety/README.md)
  gates every surface first without bypass; before each push the
  [leak review](repo-governance/workflows/quality/pr-leak-review.md) reads every outgoing commit, and every merge needs
  its posted exact-head `pass` (`leak-review` status). Fix
  [hook failures](repo-governance/conventions/push-hook-verification.md) at the cause. Keep
  [working tree](repo-governance/conventions/working-tree.md) clean and
  [poll GitHub](repo-governance/conventions/github-polling.md) every three minutes.
- Cut releases only through [release cut](repo-governance/workflows/maintenance/release-cut.md); never replace a tag.

## Harnesses

- Claude Code, Codex, and OpenCode reach the same rules under
  [the contract](repo-governance/conventions/coding-harness-contract.md);
  [change](repo-governance/workflows/quality/harness-propagation.md) and
  [verify](repo-governance/workflows/quality/harness-parity-verification.md) it through its workflows.
- [Rule](repo-governance/conventions/rules.md) changes automatically run
  [propagation](repo-governance/workflows/quality/rules-propagation.md); grooming needs explicit request.
