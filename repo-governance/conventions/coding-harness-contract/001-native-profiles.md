# Native Profiles

This module holds native discovery and enforcement details under the
[Coding-Harness Contract](../coding-harness-contract.md).

## Codex

Codex reads `AGENTS.md` and `.agents/skills/` natively. Its project subagents are standalone TOML files under
`.codex/agents/`, with `name`, `description`, and `developer_instructions`; the generated TOML adapter carries those
fields and routes its instruction field to the complete canonical agent.

That documented adapter schema has no agent-scoped native permission or denial field. Codex therefore receives the full
canonical boundary through `developer_instructions`, but this repository makes no claim that Codex itself hard-enforces
a capability denial. Such a denial is not listed as a native Codex profile requirement. Claude and OpenCode continue to
project every denial their documented formats express.

## Command Code

Command Code discovers root `AGENTS.md` and canonical `.agents/skills/` natively. Add no instruction shim or skill copy.
Generate only `.commandcode/agents/` native leaf adapters, with every documented grant and denial.

Roles declaring `subagent` or nonempty `dispatches`, here
[SWE Orchestrator](../../../.agents/agents/swe-orchestrator.md), are read completely and coordinated in the main session
under their canonical dispatch allowlist. Native leaves cannot dispatch: deny `agent` and `agent_output`. The explicit
selection must equal every canonical leaf, excluding exactly those main-session roles; update it with canonical agent
changes in the same commit and audit coverage after regeneration. Semantic main-session compliance remains unenforced by
decision, with review and the session report carrying it.

Omit tier mappings and model, featureModels, effort, and reasoningEffort pins from this profile, adapters, settings,
global sources, and smoke commands. The active session supplies the model; omitted reasoning fields use the harness
default. Existing harness mappings remain governed by their own profiles.

Keep `.commandcode/settings.local.json` and the complete `.commandcode/taste/` tree local and ignored. Preserve their
contents and active taste learning. Generator ownership is only `.commandcode/agents/`, never local state or settings.
