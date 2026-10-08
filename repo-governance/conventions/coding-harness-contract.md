# Coding-Harness Contract

Claude Code, Codex, OpenCode, and Command Code must reach the same repository-owned rules, skill procedures, and
custom-agent prompts. Each generated adapter routes to the canonical source. Where a vendor documents a native control
for a canonical capability or denial, the adapter uses that strongest control; vendor system prompts, built-in tools,
models, credentials, local memory, plugins, and approval interfaces remain outside this contract.

Matching file names, paths, or counts is not parity. Parity means each harness reaches the same effective canonical
content, and that every adapter routes to it without adding instructions or weakening a restriction.

## Required Parity

- **Rules**
  - Requirement: Every harness reaches the normalized root `AGENTS.md` body and no other always-on instruction.
  - Deterministic proof: Exact `CLAUDE.md` import, a canonical-content digest, and refusal of every competing source.
- **Skills**
  - Requirement: Every canonical skill, with its complete supporting bundle, is reachable in every harness.
  - Deterministic proof: Adapter coverage where the harness has a native surface, exact route, and a digest of the
    bundle.
- **Agents**
  - Requirement: Every canonical agent is reachable in every harness with the same prompt intent and boundary.
  - Deterministic proof: One native adapter each, apart from the main-session exception in
    [Native Profiles](coding-harness-contract/001-native-profiles.md#command-code); exact route, prompt digest, and
    every documented native control.

An adapter fails parity when it is missing, stale, extra, malformed, routes to the wrong source, copies or extends
canonical instructions, changes effective content, or weakens a denial that its documented native format can express.
Adding, renaming, changing, or removing a canonical skill or agent therefore changes every affected adapter in the same
commit.

## Canonical Sources

- `AGENTS.md` is the only project-rule body. Root `CLAUDE.md` contains only `@AGENTS.md`, apart from a BOM, line-ending
  differences, or surrounding blank lines.
- `.agents/skills/<name>/SKILL.md` and every regular file below it form one canonical skill bundle.
- `.agents/agents/<name>.md` is the only full custom-agent prompt and the only source of its capability boundary.

## No Capability Server

This repository declares no `required-mcp` and no per-harness capability block. It uses no workspace task runner and
needs no credential-free capability vector, and the schema treats the absence as an answer rather than an omission.
Declaring one half without the other is refused, so both stay absent.

Nothing is lost. The parity claim here is one instruction body, one prompt per agent, one bundle per skill, and adapters
that route rather than copy.

## Instruction Boundary

Only root `AGENTS.md` and its exact `CLAUDE.md` import may be always-on. Nested `AGENTS.md`, `AGENTS.override.md`, any
additional `CLAUDE.md`, `.claude/rules/**/*.md`, `.cursorrules`, `**/.windsurfrules`, `**/GEMINI.md`, and
`.github/copilot-instructions.md` are prohibited.

A competing always-on instruction written into a vendor's own settings file is the same violation in a different syntax,
so the prohibited-field list covers those keys too. An absent key and an explicitly empty one are both answers; a
settings file that cannot be parsed in its declared format is reported rather than assumed clean, because a source that
cannot be ruled out has not been ruled out.

`CLAUDE.local.md`, user-global configuration, vendor caches, generated trees, worktrees, credentials, and runtime data
are local concerns and are never inspected.

## Native Profiles

[Native Profiles](coding-harness-contract/001-native-profiles.md) holds Codex's native boundary and Command Code's
discovery, main-session exception, selection, model inheritance, and local-state rules.

## Enforcement

The roster of harnesses, adapter contracts, and capability vocabulary lives in
[`repo-config.yml`](../../repo-config.yml). This document states the requirement, not the roster.

```sh
./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo xtask self-validate
```

`harness adapters validate` is read-only, network-free, path-contained, and deterministic. It normalizes only a BOM and
line endings, sorts ordinally, hashes with SHA-256, and fails on a missing, extra, stale, malformed, divergent, or
semantically weaker adapter. `harness adapters generate` first validates the complete projection in memory, then
atomically replaces only declared adapter roots with catalog and provenance in the same transaction. It cannot prove
that a vendor honoured what it read, so a runtime smoke check stays a separate, human act.

## Repository Policy

Native policy bindings invoke [`scripts/agent-policy-hook.sh`](../../scripts/agent-policy-hook.sh) for this physical
checkout. Claude Code, OpenCode, and Command Code cross-repository operations use neutral destination routing. OpenCode
Serena registration is deferred; Claude Code and Command Code retain semantic integration. Codex retains its existing
guards; new routing and Serena are deferred. Protected-path data lives in
[`.agents/agent-policy.json`](../../.agents/agent-policy.json); semantic indexing exclusions live in the tracked
[Serena project config](../../.serena/project.yml). Shared MCP and capture integrations remain user-global.
