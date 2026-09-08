# Quality Gates

Two gates, distinguished by what they can afford to run.

## The Quick Gate

`cargo xtask test-quick` is what the `pre-push` hook runs, so it holds only what is fast enough to run on every push:

- `cargo fmt --all --check`
- `cargo clippy --all-targets --all-features -- -D warnings`
- the unit adapter under the line-coverage floor, measured in the same execution rather than a second one, because two runs can disagree and the number that gates must be the number the passing run produced
- the static behaviour-coverage check, which executes no scenario and asserts that every scenario is bound or validly exempt
- `cargo xtask self-validate` last, so a repository that is out of date cannot mask a product that is broken

Integration and end-to-end adapters never run in a hook and never in the quick gate. They run on a schedule. See [end-to-end testing](end-to-end-testing.md).

## The Full Gate

The scheduled workflow runs every adapter, including the slow ones. It is not a required status check, because a required check that takes minutes on every push trains people to work around it.

## The Server Gate

[`pr-quality-gate.yml`](../../.github/workflows/pr-quality-gate.yml) mirrors the hook contract for pull requests, and is a superset of it: it adds `cargo deny check` and a repository-wide Prettier pass that `lint-staged` structurally cannot perform, because `lint-staged` only ever sees staged paths.

Its aggregate `Quality gate` job is the only required check. Individual jobs may legitimately skip, and a skipped required check never reports at all, so merge protection points at the aggregate. The aggregate treats a skipped or cancelled dependency as failure, and that refusal is proved rather than assumed.

## Tooling and Hooks

Install locked tooling with `npm ci`. That is also what activates the hooks, so a fresh checkout — including a new worktree — has no enforcement until it runs.

Hooks enforce Conventional Commits, staged formatting, and the quick gate before push. A failing hook is repaired at its cause under [push-hook verification](../conventions/push-hook-verification.md).

## Test Isolation

A test never reads or writes outside a root it created. The unit adapter runs against an in-memory tree; the integration and end-to-end adapters build a temporary tree per case and remove it. There is no shared fixture directory, because a shared fixture is a test that passes because of another test.

## Related

- [Software quality enforcement](software-quality-enforcement.md)
- [Resource-aware development](resource-aware-development.md)
