# Delegated-Agent Concurrency

At most **three** delegated agents run at once. The main thread is the `+1`: it takes no slot and stays active while the
three run, so a session holds four agents at most.

## Requirements

- The count covers every delegated agent alive in the session, foreground or background, at any depth: an agent that a
  delegated agent spawns takes a slot of its own.
- It binds in every harness whose session can spawn delegated agents, whatever that harness calls them: the Claude Code
  `Agent` tool, Codex spawned agents, OpenCode `task`, and Command Code `agent` alike.
- Work beyond the cap waits until a running agent returns; it is never launched over the cap. When a slot frees and
  independent work is waiting, the next agent launches, because the cap limits the instantaneous count rather than the
  batch total.
- Only a plan or the user changes the cap. An agent never raises it on its own judgement, least of all because early
  agents finished quickly, and runs below it under budget, runner, or machine pressure.

## Why

Each concurrent agent spends independently against the same token quota and rate limits, and overshooting produces
retries that cascade until the batch runs slower than it would have serially. The workstation is shared too:
[resource-aware development](resource-aware-development.md) arbitrates the compute those agents start, but not the
agents themselves.

## Enforcement

Unenforced by decision. No hook or harness setting caps the count, and none is to be added: the maintainer declined
mechanical enforcement, so review verifies it.

## Related

- [Task tracking](../conventions/task-tracking.md)
- [Coding-harness contract](../conventions/coding-harness-contract.md)
