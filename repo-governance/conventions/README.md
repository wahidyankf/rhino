# Conventions

Repository-wide choices, within the [vision](../vision/README.md) and the [principles](../principles/README.md). A convention says how this repository does something where more than one way would have worked.

## Directory Map

- [Coding-harness contract](coding-harness-contract.md) — one instruction body, one prompt per agent, adapters that route rather than copy.
- [Commit authorization](commit-authorization.md) — when a commit or push may happen at all.
- [Directory maps](directory-maps.md) — every governed directory maps its contents, and the word budget.
- [Documentation architecture](documentation-architecture.md) — what belongs in `docs/`, and its Diátaxis categories.
- [File naming](file-naming.md) — lowercase kebab-case, companion directories named after their document, ordinals where there is an order.
- [GitHub polling](github-polling.md) — one status request every three minutes, never a stream.
- [Integration path](integration-path.md) — trunk-based development through a worktree and a pull request.
- [Language](language.md) — English, and which spelling.
- [Last-resort questions](last-resort-questions.md) — exhaust independent progress before asking.
- [Markdown links](markdown-links.md) — internal links resolve, and move with their targets.
- [Markdown visualizations](markdown-visualizations.md) — when to draw one, and the accessible palette.
- [Plan lifecycle](plan-lifecycle.md) — the local additions to the plans convention: idea quadrants, and what this repository requires beyond it.
- [Plan specification changes](plan-specification-changes.md) — how a plan states the specification work before it starts.
- [Plan validator contract](plan-validator-contract.md) — what every plan-structure implementation must accept, reject, and name identically.
- [Plan validator contract modules](plan-validator-contract/README.md) — the four modules that entrypoint indexes.
- [Plans](plans.md) — the canonical plan system: lifecycle, the six documents, delivery, validation, evidence, and archival.
- [Plans convention modules](plans/README.md) — the nine modules that entrypoint indexes, in reading order.
- [Public repository data safety](public-repository-data-safety.md) — what may never be committed here.
- [Pull request body](pull-request-body.md) — what the body carries, and that it never goes stale.
- [Pull request boundaries](pull-request-boundaries.md) — one branch, one pull request, one delivery unit.
- [Pull request merge](pull-request-merge.md) — the five preconditions that authorize a merge.
- [Push-hook verification](push-hook-verification.md) — fix the cause; never bypass unauthorized.
- [Rules](rules.md) — what a rule is, and how must, should, and may are read.
- [Task tracking](task-tracking.md) — granular items kept synchronized with the work.
- [Thematic commits](thematic-commits.md) — one theme per commit, in Conventional Commits form.
- [Working tree](working-tree.md) — what is ignored, and why every tree-walking tool needs telling separately.
