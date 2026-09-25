# Own or Refuse Codex Agent Registrations

A `Rhino generated` region in a Codex `config.toml` that no RHINO release writes can hide unbounded drift; decide
whether RHINO generates Codex agent registrations or reports a generated-region marker it does not own.

Provenance: found on 2026-09-26 during a cross-repository standards adoption.

## Problem and Evidence

- Two consumer repositories kept a `.codex/config.toml` region marked `Rhino generated: codex agents`. No source on
  current `main` writes or reads that marker; a history search of this repository for the string finds nothing.
- The regions held 90 and 54 agent registrations against 5 and 3 agent files. The "do not edit" marker told every
  maintainer the block was machine-owned, so nobody pruned it through several migrations.
- RHINO's Codex agent adapter writes `.codex/agents/<name>.toml` with `catalog.json` and `provenance.json`, and
  `harness adapters validate` reports stale files only under that family root. A file outside every declared family,
  such as `config.toml`, is never inspected, so the stale region passed every check.

## Why Now

The region survived because the marker made it look owned. Every repository that adopts current RHINO adapters on top of
an older layout can inherit the same false claim, and the count gap grows silently.

## Prior Art

- [Codex subagents](https://learn.chatgpt.com/docs/agent-configuration/subagents), accessed 2026-09-26: project agents
  are standalone TOML files under `.codex/agents/`, which Codex loads without a `config.toml` entry; the `[agents]`
  table keeps only global settings such as `enabled` and `max_concurrent_threads_per_session`. Per-agent registrations
  therefore duplicate what discovery already provides.

## Proposed Direction

Two options, not yet decided:

1. **Report a marker RHINO does not own.** A declared surface lets `harness adapters validate` report a region that
   claims RHINO generation but belongs to no declared family, so the maintainer removes it. This fits discovery making
   registrations redundant.
2. **Generate the registrations.** A Codex profile declares a marked `config.toml` region as one more family, and
   `generate` owns it transactionally with the agent files, so the count can never drift.

Option 1 looks smaller and matches current Codex behaviour; option 2 is worth it only if a Codex feature needs a
per-agent registration.

## Scope and Non-Goals

In scope: the chosen check or family, its Gherkin, and the harness reference. Not in scope: rewriting `config.toml`
outside a declared region, other harnesses' configuration files, or migrating any consumer.

## Risks and Open Questions

- The [vision](../../../repo-governance/vision/README.md) forbids built-in repository defaults. Which declared key names
  the file and marker to inspect, so RHINO never hard-codes `config.toml`?
- Is a stale marker a finding (exit `1`) or a refusal (exit `2`)?
- Does any supported Codex version still require per-agent `config.toml` entries? That answer decides between the
  options.

## Success and Promotion Signal

Success: no repository can carry a region labelled as RHINO output that RHINO neither writes nor checks. Promote once the
owner picks an option and the Codex-version question is answered from primary documentation.
