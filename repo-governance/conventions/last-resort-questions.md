# Last-Resort Questions

Ask the user for information, clarification, confirmation, or a decision only after exhausting safe, reasonable, in-scope ways to proceed independently.

## Before Asking

- Read the applicable instructions, governance, repository files, history, configuration, current state, and available tool output.
- Run read-only diagnostics and use repository evidence to resolve the uncertainty.
- Make a bounded assumption when it is reversible, low risk, and unlikely to diverge from the user's intent — and state it.
- Try viable in-scope alternatives, and continue every piece of independent work that does not depend on the missing answer.
- Never ask the user to repeat something already provided or discoverable here.

## When Asking Is Necessary

Ask only when progress needs information or authority available nowhere else, and proceeding by assumption would materially risk incorrect work, harm, an irreversible or external side effect, a rule violation, or a meaningfully different outcome.

Ask the fewest, most concise questions that unblock the work. State the blocker, what was already checked or attempted, and why no safe default remains.

## What This Does Not Waive

This convention does not waive an explicit authorization requirement and does not permit unauthorized action. Where a rule reserves a decision for the user — [commit and push](commit-authorization.md), a hook bypass under [push-hook verification](push-hook-verification.md), a `[HUMAN]` step in a plan — obtain it after completing every independent prerequisite, not instead of them.

Equally, do not manufacture work to avoid a question that genuinely has to be asked.
