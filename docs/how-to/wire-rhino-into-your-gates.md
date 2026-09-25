# How to wire RHINO into your gates

Use grouped v2 to declare a lifecycle once, validate it before dispatch, and
give every child an explicit typed input and argv projection. A hook or CI job
calls one RHINO surface; it does not duplicate the registry or append a command
after `--`.

## 1. Declare the lifecycle

`gates` owns closed lifecycle membership: `pre-commit`, `commit-msg`,
`pre-push`, `pull-request`, `main`, `scheduled`, and `manual`. Each entry has a
semantic ID, typed inputs, a direct executable-and-argv command, and declared
`run-on` bindings. Pull-request composition is explicitly `exact` or
`at-least`; an at-least-only gate needs its own reason.

Start with `gate validate` while writing the declaration:

```console
$ rhino gate validate
```

The command refuses an incomplete lifecycle before any child process starts.
Use `gate list` to inspect the resolved membership for each surface.

## 2. Call one surface from each caller

```sh
#!/bin/sh
# .husky/pre-commit or .git/hooks/pre-commit
set -eu
rhino gate run --surface pre-commit
```

The declared gate receives only the inputs its configuration binds. Do not add
`-- "$@"`: v0.4 rejects arguments after `--` so a hook cannot replace a typed
command with unreviewed argv.

A pull-request caller supplies its immutable range only when the declared
surface binds that input:

```sh
rhino gate run --surface pull-request --base "$BASE_SHA" --head "$HEAD_SHA"
```

For `commit-msg`, pass Git's hook path with `--message-file`. Rhino confirms it
is the current worktree's canonical `COMMIT_EDITMSG` file before reading it;
this works in linked worktrees and refuses another external path. For
`pre-push`, pass update records only with `--push-updates-stdin`. The command
refuses the wrong surface/input combination with exit `2`.

## 3. Keep mutations explicit

A mutation pairs local `apply-index` behavior with CI `verify-clean` behavior.
RHINO refuses to substitute the mutable working tree for the declared index or
disposable replay boundary. Environment initialization and toolchain provision
likewise require their own reviewed plan and explicit `--apply` authorization.

## 4. Handle results at the caller

| Exit code | Caller action                                                  |
| --------- | -------------------------------------------------------------- |
| `0`       | Continue; the selected surface completed cleanly.              |
| `1`       | Stop; a declared child reported a repository finding.          |
| `2`       | Stop; fix the invocation, configuration, or declared boundary. |
| `126`     | Stop; a declared gate child could not be executed.             |
| `127`     | Stop; a declared gate child was not found.                     |
| `128+N`   | Stop; RHINO was ended by signal `N`.                           |

RHINO prints only a gate identifier and sanitized status. It never repeats a
child stream that could contain material unsafe to publish.

## Related

- [Grouped v2 Configuration](../reference/v0-4-configuration.md)
- [Command line](../reference/cli.md)
- [How to respond to exit codes](./respond-to-exit-codes.md)
