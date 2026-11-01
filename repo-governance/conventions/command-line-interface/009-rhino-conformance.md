---
description: >-
  Records the named gaps between this repository's rhino binary and the full-bar tier: the unaccepted color flag, the
  missing tool name in the version output, and the default panic trace.
when_to_use: >-
  Use when deciding whether rhino conforms to a convention clause, or when changing one of the named gaps.
---

# Rhino Conformance Record

Rhino is full bar except for three named gaps. Each one is known, and the binary is kept as it is; none has been
changed. The clauses they break stay as written in the modules above, so this record weakens no rule.

## Named Gaps

- **`--color` is not accepted.** Rhino rejects the flag with exit `2` as an unrecognized option.
  [Terminal and Environment](007-terminal-and-environment.md) requires `--color` to accept exactly `auto`, `always`, and
  `never`.
- **`--version` and `version` print no tool name.** Both print only the version string, such as `v0.0.0-dev`.
  [Arguments and Flags](005-arguments-and-flags.md) requires `--version` to print the tool's name and version. The
  release build stamps the same version string, so release binaries share the gap.
- **An uncaught panic prints a goroutine trace by default.** Rhino installs no top-level handler, so the Go runtime
  prints the trace on standard error. [The Exit Status Contract](001-exit-status-contract.md) requires a top-level
  handler, and [Errors and Diagnostics](006-errors-and-diagnostics.md) forbids a trace by default. The binary documents
  this behaviour in [exit codes](../../../docs/reference/exit-codes.md).

## Related

- [Command-Line Interface](../command-line-interface.md)
