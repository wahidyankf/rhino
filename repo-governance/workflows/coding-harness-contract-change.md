# Coding-Harness Contract Change

Use this workflow whenever repository-owned rules, canonical skills, canonical agents, or their adapters are added, changed, renamed, or removed. The goal is one canonical edit with complete Claude Code, Codex, and OpenCode parity — not three independently maintained trees.

## Prerequisites

- Read the [coding-harness contract](../conventions/coding-harness-contract.md).
- Keep credentials, user-global configuration, local memory, and generated vendor state out of the repository.
- For a rule change, run [rules propagation](rules-propagation.md) first.
- Check current official vendor documentation before changing a native adapter schema. Never infer a field from another harness.

## Procedure

### 1. Edit the canonical source first

- **Rules** — root `AGENTS.md`. Root `CLAUDE.md` stays the exact `@AGENTS.md` import. Add no nested or harness-specific instruction file.
- **Skills** — the complete `.agents/skills/<name>/` bundle, `SKILL.md` and every supporting file. Directory name and front-matter `name` must match.
- **Agents** — `.agents/agents/<name>.md`, including the prompt and its semantic `requires`, `denies`, and `constraints`.

### 2. Reconcile every adapter in the same change

The adapter path, route field, and translation table for each harness are declared in [`repo-config.yml`](../../repo-config.yml). Read them there; this document does not restate the roster.

An adapter carries native identity, mode, and permission metadata plus the fixed route to the canonical file — and nothing else. Never copy the canonical body into an adapter, never append an instruction, never pin a provider model, and never omit a restriction.

Translate canonical capabilities and denials into that harness's **strongest** native control. Where a harness expresses a denial only as an absence, the absence must be complete: a tool left unlisted because it seemed unlikely is a granted capability.

If a semantic capability has no validated mapping, extend the convention and the `harness-parity` declarations before claiming parity. A mapping the validator cannot express is a change to the validator, not a parity claim.

A rename or removal renames or removes every matching adapter and leaves no stale file.

### 3. Update enforcement only when the shape changes

A content-only change that keeps names, descriptions, routes, semantics, and native schemas needs no code change: the validator reads and hashes canonical content dynamically.

When topology, accepted front matter, capability mappings, native schema, or finding behaviour changes:

1. express it in `repo-config.yml`, and stop there if it can be declared;
2. otherwise change the validator at the narrowest responsible layer, against the corpus, following [specification maintenance](../development/specification-maintenance.md); and
3. update `docs/reference/configuration.md` when a declared key or its meaning moves.

This repository builds the validator it runs, so a shape change and its enforcement land together rather than waiting on a release.

### 4. Verify

```sh
./hippo run --class ephemeral --disk-path . -- cargo xtask self-validate
```

Review findings by kind, field, harness, and path. Matching counts are not proof.

## Recovery

Never weaken the validator, remove a denial, or exclude a path to make the gate pass. Restore the missing route or adapter, correct the mapping, and rerun. If a harness cannot express a required capability, stop and record the gap; do not claim three-harness parity until the contract or the supported-harness set changes explicitly.

## Outcome

Complete only when one canonical source remains, every required adapter is current, no stale or extra source exists, the deterministic checks pass, and the diff contains no local or sensitive data.
