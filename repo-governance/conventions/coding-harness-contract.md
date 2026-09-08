# Coding-Harness Contract

Claude Code, Codex, and OpenCode must reach the same repository-owned rules, the same skill procedures, the same custom-agent intent, and the same safety boundaries. Vendor system prompts, built-in tools, models, credentials, local memory, plugins, and approval interfaces are outside the claim.

Matching file names, paths, or counts is not parity. Parity means each harness reaches the same effective canonical content, and that every adapter routes to it without adding instructions or weakening a restriction.

## Required Parity

| Concern | Requirement                                                                                    | Deterministic proof                                                                               |
| ------- | ---------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| Rules   | Every harness reaches the normalized root `AGENTS.md` body and no other always-on instruction. | Exact `CLAUDE.md` import, a canonical-content digest, and refusal of every competing source.      |
| Skills  | Every canonical skill, with its complete supporting bundle, is reachable in every harness.     | Adapter coverage where the harness has a native surface, exact route, and a digest of the bundle. |
| Agents  | Every canonical agent is reachable in every harness with the same prompt intent and boundary.  | One native adapter each, exact route, prompt digest, and equivalent grants, denials, and limits.  |

An adapter fails parity when it is missing, stale, extra, malformed, routes to the wrong source, copies or extends canonical instructions, changes effective content, or weakens a denial. Adding, renaming, changing, or removing a canonical skill or agent therefore changes every affected adapter in the same commit.

## Canonical Sources

- `AGENTS.md` is the only project-rule body. Root `CLAUDE.md` contains only `@AGENTS.md`, apart from a BOM, line-ending differences, or surrounding blank lines.
- `.agents/skills/<name>/SKILL.md` and every regular file below it form one canonical skill bundle.
- `.agents/agents/<name>.md` is the only full custom-agent prompt and the only source of its capability boundary.

## No Capability Server

This repository declares no `required-mcp` and no per-harness capability block. It uses no workspace task runner and needs no credential-free capability vector, and the schema treats the absence as an answer rather than an omission. Declaring one half without the other is refused, so both stay absent.

Nothing is lost. The parity claim here is one instruction body, one prompt per agent, one bundle per skill, and adapters that route rather than copy.

## Instruction Boundary

Only root `AGENTS.md` and its exact `CLAUDE.md` import may be always-on. Nested `AGENTS.md`, `AGENTS.override.md`, any additional `CLAUDE.md`, `.claude/rules/**/*.md`, `.cursorrules`, `**/.windsurfrules`, `**/GEMINI.md`, and `.github/copilot-instructions.md` are prohibited.

A competing always-on instruction written into a vendor's own settings file is the same violation in a different syntax, so the prohibited-field list covers those keys too. An absent key and an explicitly empty one are both answers; a settings file that cannot be parsed in its declared format is reported rather than assumed clean, because a source that cannot be ruled out has not been ruled out.

`CLAUDE.local.md`, user-global configuration, vendor caches, generated trees, worktrees, credentials, and runtime data are local concerns and are never inspected.

## Enforcement

The roster of harnesses, adapter contracts, and capability vocabulary lives in [`repo-config.yml`](../../repo-config.yml). This document states the requirement, not the roster.

```sh
./hippo run --class ephemeral --disk-path . -- cargo xtask self-validate
```

The parity validator is read-only, network-free, path-contained, and deterministic. It normalizes only a BOM and line endings, sorts ordinally, hashes with SHA-256, and fails on a missing, extra, stale, malformed, divergent, or semantically weaker adapter. It cannot prove that a vendor honoured what it read, so a runtime smoke check stays a separate, human act.
