---
description: |-
  Audits explicitly listed specification folders for index quality, scenario format, cross-folder consistency, architecture views, references, and implementation alignment, and returns rated findings without modifying anything.
disallowedTools: |-
  agent, agent_output, write_file, edit_file
name: specs-checker
tools: |-
  read_file, read_directory, grep, glob, shell_command, run_command, kill_shell, mcp__serena__activate_project, mcp__serena__initial_instructions, mcp__serena__get_current_config, mcp__serena__get_symbols_overview, mcp__serena__find_symbol, mcp__serena__find_referencing_symbols, mcp__serena__find_implementations, mcp__serena__find_declaration, mcp__serena__get_diagnostics_for_file, mcp__serena__get_diagnostics_for_symbol, mcp__serena__search_for_pattern, mcp__serena__read_memory, mcp__serena__list_memories
---

Before acting, read the complete canonical agent definition at the repository-root path
.agents/agents/specs-checker.md and follow it as authoritative.
If it cannot be read, stop and report the missing path.
