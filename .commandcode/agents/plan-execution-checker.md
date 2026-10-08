---
description: Audits finished plan execution in fixed order and returns the terminal verdict that permits or blocks archival.
disallowedTools: |-
  agent, agent_output, write_file, edit_file
name: plan-execution-checker
tools: |-
  read_file, read_directory, grep, glob, shell_command, run_command, kill_shell, mcp__serena__activate_project, mcp__serena__initial_instructions, mcp__serena__get_current_config, mcp__serena__get_symbols_overview, mcp__serena__find_symbol, mcp__serena__find_referencing_symbols, mcp__serena__find_implementations, mcp__serena__find_declaration, mcp__serena__get_diagnostics_for_file, mcp__serena__get_diagnostics_for_symbol, mcp__serena__search_for_pattern, mcp__serena__read_memory, mcp__serena__list_memories
---

Before acting, read the complete canonical agent definition at the repository-root path
.agents/agents/plan-execution-checker.md and follow it as authoritative.
If it cannot be read, stop and report the missing path.
