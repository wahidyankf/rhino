---
description: >-
  Retired pointer to the canonical worktree-to-PR exception. The one-push rule once stated here is superseded and
  authorizes no bypass on its own.
when_to_use: >-
  Do not apply as an exception. Follow the canonical worktree-to-PR exception in push-hook-verification.md instead.
---

# Capacity-Held Hook

[Back to Push-Hook Verification](../push-hook-verification.md)

This module states no exception of its own. The single canonical statement is the
[worktree-to-PR exception](../push-hook-verification.md#worktree-to-pr-exception): when a task-owned branch lands
through a pull request whose quality gate mirrors the skipped hook's deterministic checks, pushing that branch with
`--no-verify` and letting CI run the gate is sound, and preferable under HIPPO contention. A hook held back by capacity
before any check started holds no finding to fix; its receipt is cited under the canonical exception's evidence clause.
This file is kept only so existing references resolve.
