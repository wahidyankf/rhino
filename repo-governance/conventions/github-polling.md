# GitHub Polling

When monitoring a GitHub operation or waiting for GitHub state to change, issue at most one status request every three minutes. Keep a minimum of 180 seconds between repeated queries in the same monitoring loop.

The limit applies whatever the tool: GitHub CLI, the API, a browser, or an integration. Poll in discrete requests; never stream. A watch command, a follow or tail of a running log, an automatic refresh, or any long-lived connection that reports as state changes is prohibited regardless of how little traffic it appears to send, because its request rate is not the caller's to control. Prefer an event-driven wait where one exists.

A single user-requested lookup is not polling. It becomes polling when a second status request is made while waiting for the same operation. After the first lookup, wait the full interval before querying again, even when completion is expected sooner.

Use the interval. A pull-request gate here takes minutes, and the wait is time to advance independent work, not time to spend asking whether it finished.
