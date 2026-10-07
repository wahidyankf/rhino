# Conventions

Repository-wide choices, within the [vision](../vision/README.md) and the [principles](../principles/README.md). A
convention says how this repository does something where more than one way would have worked.

## Directory Map

- [Bug reports](bug-reports.md) — the duplicate search before filing, and what a report carries, version and commit
  included.
- [Coding-harness contract](coding-harness-contract.md) — one instruction body, one prompt per agent, adapters that
  route rather than copy.
- [Command-line interface](command-line-interface.md) — the two-layer contract a command-line tool presents to its
  callers: one closed exit vocabulary, and a body that says what happened.
- [Command-line interface modules](command-line-interface/README.md) — the eight modules that entrypoint indexes.
- [Commit authorization](commit-authorization.md) — when a commit or push may happen at all.
- [Directory maps](directory-maps.md) — every governed directory maps its contents, and the word budget.
- [Documentation architecture](documentation-architecture.md) — what belongs in `docs/`, and its Diátaxis categories.
- [File naming](file-naming.md) — lowercase kebab-case, companion directories named after their document, ordinals where
  there is an order.
- [GitHub polling](github-polling.md) — one status request every three minutes, never a stream.
- [Integration path](integration-path.md) — trunk-based development through a worktree and a pull request.
- [Language](language.md) — English for repository artifacts and clear writing for documents and agent replies.
- [Last-resort questions](last-resort-questions.md) — exhaust independent progress before asking.
- [Markdown links](markdown-links.md) — internal links resolve, and move with their targets.
- [Markdown line length](markdown-line-length.md) — 120 characters per line, tables and code included.
- [Markdown visualizations](markdown-visualizations.md) — when to draw one, and why it is ASCII.
- [No destructive Git operations](no-destructive-git-operations.md) — approval for any Git command that destroys work or
  history.
- [Plan lifecycle](plan-lifecycle.md) — the local additions to the plans convention: idea quadrants, and what this
  repository requires beyond it.
- [Plan specification changes](plan-specification-changes.md) — how a plan states the specification work before it
  starts.
- [Plan validator contract](plan-validator-contract.md) — what every plan-structure implementation must accept, reject,
  and name identically.
- [Plan validator contract modules](plan-validator-contract/README.md) — the four modules that entrypoint indexes.
- [Plans](plans.md) — the canonical plan system: lifecycle, the six documents, delivery, validation, evidence, and
  archival.
- [Plans convention modules](plans/README.md) — the ten modules that entrypoint indexes, in reading order.
- [Public repository data safety](public-repository-data-safety.md) — what may never be committed here.
- [Pull request body](pull-request-body.md) — what the body carries, and that it never goes stale.
- [Pull request boundaries](pull-request-boundaries.md) — one branch, one pull request, one delivery unit.
- [Pull request merge](pull-request-merge.md) — the five preconditions that authorize a merge.
- [PR review agent procedures](pr-review-agent-procedures/README.md) — the procedure moved out of the
  `pr-review-checker` definition to fit its word budget.
- [Push-hook verification](push-hook-verification.md) — fix the cause; never bypass unauthorized.
- [Rules](rules.md) — what a rule is, and how must, should, and may are read.
- [Structure](structure/README.md) — where each kind of artifact adopted from the shared catalog lives.
- [SWE agent procedures](swe-agent-procedures/README.md) — the procedure sections moved out of three `swe-*` agent
  definitions to fit their word budget.
- [SWE delegation](swe-delegation.md) — coding work goes to the fitting `swe-*` agent, with three exceptions.
- [Task tracking](task-tracking.md) — granular items kept synchronized with the work.
- [Thematic commits](thematic-commits.md) — one theme per commit, in Conventional Commits form.
- [Working tree](working-tree.md) — what is ignored, and why every tree-walking tool needs telling separately.
