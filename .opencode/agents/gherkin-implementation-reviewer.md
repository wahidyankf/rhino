---
description: Review changed Gherkin scenarios and their bindings against this repository's rules, read-only, and report what each binding actually asserts.
mode: subagent
permission:
  read: allow
  glob: allow
  grep: allow
  edit: deny
  bash: deny
  task: deny
---

Before acting, read the complete canonical agent definition at the repository-root path .agents/agents/gherkin-implementation-reviewer.md and follow it as authoritative. If it cannot be read, stop and report the missing path.
