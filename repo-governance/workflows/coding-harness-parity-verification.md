# Coding-Harness Parity Verification

Use this workflow to evaluate whether Claude Code, Codex, and OpenCode currently satisfy the [coding-harness contract](../conventions/coding-harness-contract.md). It is read-only. Use the [contract change workflow](coding-harness-contract-change.md) when remediation is requested.

## Prerequisites

- Read the contract, and keep its repository-owned scope distinct from vendor prompts, models, credentials, plugins, and user-global configuration.
- Record the current revision and `git status --short`. Existing changes are user-owned and are not modified by verification.

## Procedure

### 1. Establish the baseline

Record the revision, the worktree status, and the declared roster from `repo-config.yml`. Any later repository change invalidates results taken from this baseline.

### 2. Review the canonical topology

Confirm the intended sources and adapters before interpreting the gate:

- root `AGENTS.md` and the exact root `CLAUDE.md` import;
- every `.agents/skills/<name>/` bundle and its adapter on each harness that has a native skill surface; and
- every `.agents/agents/<name>.md` and its one adapter per harness.

Do not infer parity from matching names or counts. Content digests, routes, native permissions, denials, and constraints are authoritative.

### 3. Run the deterministic proof

```sh
./hippo run --class ephemeral --disk-path . -- cargo xtask self-validate
```

Record the exit status, the contract digest, and the reconciled harness, skill, and agent counts. On failure, review every finding by kind, field, harness, and path rather than stopping at the summary.

Because this repository builds the validator it runs, a parity verdict here is a claim about the working tree at that revision — including any unreleased validator change it contains. Say which revision when reporting.

### 4. Bound runtime claims

Recording that a vendor CLI is installed is an availability smoke check. It is not proof that vendor discovery, model behaviour, or user-global integrations honour the repository contract. Report runtime discovery as separately verified, not assessed, unavailable, or failed — never folded into the parity verdict.

### 5. Report

- **pass** — the deterministic gate succeeded at the recorded baseline;
- **fail** — any contract finding exists;
- **blocked** — the gate could not execute or required evidence was unreadable;
- **not assessed** — verification was not run.

Include the commands, the digest and counts, the worktree state, the runtime-discovery scope, and unresolved risks. Never broaden a repository-parity pass into a claim about excluded vendor or local state.

## Recovery

Preserve the findings and use the contract change workflow. Never weaken the validator, remove a denial, or exclude a path to obtain a pass. Rerun from a newly recorded baseline after remediation.
