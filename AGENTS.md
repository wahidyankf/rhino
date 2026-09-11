# RHINO Contributor Rules

RHINO is a generic repository-hygiene validator that [owns no repository's answers](repo-governance/vision/README.md). This file is an index; every rule lives in [`repo-governance/`](repo-governance/README.md), stated once.

## The Product

- Ship no default a repository could reasonably decide differently, and name no repository, harness, or organization in `src/`; the [vision](repo-governance/vision/README.md) draws the policy/behaviour line.
- Exit codes, `version --json`, commands, flags, and configuration keys are [a public contract](repo-governance/development/public-contract.md): adding is free, moving is a major version.
- Read-only, network-free, process-free, path-contained, `#![forbid(unsafe_code)]` — [enforced by tests](repo-governance/development/software-quality-enforcement.md), not documentation.

## Specifications

- `specs/` is canonical. Assess [behaviours and architecture](repo-governance/development/specification-maintenance.md) before every change; record a verified no-op over churn.
- Gherkin first, prove the [red for the stated reason](repo-governance/development/test-driven-development.md), then implement. Write scenarios and bindings under [BDD](repo-governance/development/behaviour-driven-development.md); keep [the C4 model](repo-governance/development/architecture-specifications.md) true.
- Every scenario binds at the unit adapter, with no unit exemption. Changed Gherkin gets [the review](repo-governance/workflows/gherkin-implementation-review.md).

## Testing

- `cargo xtask test-quick` is [the quick gate](repo-governance/development/quality-gates.md); integration and [end-to-end](repo-governance/development/end-to-end-testing.md) never run in a hook. [`repo-config.yml`](repo-config.yml) declares which gates run at which moment; `cargo xtask self-validate` dispatches the `ci` surface.
- The 99% coverage floor and its two declared exclusions are [not negotiable](repo-governance/development/software-quality-enforcement.md).
- Guard heavy local work with [`./hippo`](repo-governance/development/resource-aware-development.md): `75` retry, `73` clean up, `78` replan, never bypass.

## Change Discipline

- [Understand, reuse, minimize, verify](repo-governance/principles/minimal-sufficiency.md). A new dependency carries [its own record](repo-governance/development/dependency-selection.md).
- Keep `README.md`, `docs/`, and `CHANGELOG.md` true to the binary under [Diátaxis](repo-governance/conventions/documentation-architecture.md). Follow [code clarity](repo-governance/development/code-clarity.md), [English](repo-governance/conventions/language.md), [Mermaid](repo-governance/conventions/markdown-visualizations.md), [links](repo-governance/conventions/markdown-links.md), and [directory maps](repo-governance/conventions/directory-maps.md).
- Track work in [granular items](repo-governance/conventions/task-tracking.md), preserve rules through [compaction](repo-governance/principles/governance-continuity.md), [ask last](repo-governance/conventions/last-resort-questions.md), and name files in [lowercase kebab-case](repo-governance/conventions/file-naming.md).
- Plans are working records under [`plans/`](plans/README.md), never architecture: the [plans convention](repo-governance/conventions/plans.md) with its [modules](repo-governance/conventions/plans/README.md), the [local additions](repo-governance/conventions/plan-lifecycle.md), [specification changes](repo-governance/conventions/plan-specification-changes.md), and [the validator contract](repo-governance/conventions/plan-validator-contract.md) structure answers to.
- Seven workflows cover that lifecycle, each [reachable](repo-governance/development/planning-capabilities.md) and indexed in [`workflows/`](repo-governance/workflows/README.md). The [quality gate](repo-governance/workflows/plan-quality-gate.md) needs an explicit request; the [execution check](repo-governance/workflows/plan-execution-check.md) blocks archival.

## Version Control

- `main` refuses direct pushes for everyone. Work in `worktrees/<name>/` and integrate by [pull request](repo-governance/workflows/worktree-to-pull-request.md) under [the integration path](repo-governance/conventions/integration-path.md), one [delivery unit](repo-governance/conventions/pull-request-boundaries.md) each, with an accurate [body](repo-governance/conventions/pull-request-body.md) and all five [merge preconditions](repo-governance/conventions/pull-request-merge.md).
- Make [thematic commits](repo-governance/conventions/thematic-commits.md) when [authorized](repo-governance/conventions/commit-authorization.md). Never commit prohibited [data](repo-governance/conventions/public-repository-data-safety.md); [public safety](scripts/public-safety/README.md) gates every surface first and has no bypass, and still cannot judge what is deliberately public. Fix [hook failures](repo-governance/conventions/push-hook-verification.md) at the cause. Keep [the working tree](repo-governance/conventions/working-tree.md) clean and [poll GitHub](repo-governance/conventions/github-polling.md) every three minutes.
- Cut releases only through [the release workflow](repo-governance/workflows/release-cut.md); never replace a tag.

## Harnesses

- Claude Code, Codex, and OpenCode reach the same rules under [the contract](repo-governance/conventions/coding-harness-contract.md); [change](repo-governance/workflows/coding-harness-contract-change.md) and [verify](repo-governance/workflows/coding-harness-parity-verification.md) it through its workflows.
- A [rule](repo-governance/conventions/rules.md) change runs [propagation](repo-governance/workflows/rules-propagation.md) automatically; [the quality gate](repo-governance/workflows/rules-quality-gate.md) and [grooming](repo-governance/workflows/rules-grooming.md) need an explicit request.
