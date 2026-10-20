---
description: Indexes canonical agent and skill sources and repository tool policy
when_to_use: Use when locating canonical definitions or the repository-owned tool endpoint
---

# Canonical Agent Directory

- `agents/` contains canonical agent definitions.
- `skills/` contains canonical skill definitions.
- The `agents/` subtree in this directory (`.agents/agents/<id>/AGENT.md`) is generated Muse adapter output, never
  edited by hand, and no other harness reads it. Canonical agent definitions live in `agents/` at the repository root.
- [`agent-policy.json`](agent-policy.json) declares protected and permitted paths for the repository-owned
  [`agent-policy-hook.sh`](../scripts/agent-policy-hook.sh). Security and resource obligations remain in governance.

See the [harness contract](../repo-governance/conventions/coding-harness-contract.md) for canonical routing.
