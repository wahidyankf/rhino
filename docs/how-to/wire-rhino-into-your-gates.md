# How to wire RHINO into your gates

Use grouped v2 to declare a lifecycle once, validate it before dispatch, and give every child an explicit typed input
and argv projection. A hook or CI job calls one RHINO surface; it does not duplicate the registry or append a command
after `--`.

## 1. Declare the lifecycle

`gates` owns closed lifecycle membership: `pre-commit`, `commit-msg`, `pre-push`, `pull-request`, `main`, `scheduled`,
and `manual`. Each entry has a semantic ID, typed inputs, a direct executable-and-argv command, and declared `run-on`
bindings. Pull-request composition is explicitly `exact` or `at-least`; an at-least-only gate needs its own reason.

Start with `gate validate` while writing the declaration:

```console
$ rhino gate validate
```

The command refuses an incomplete lifecycle before any child process starts. Use `gate list` to inspect the resolved
membership for each surface.

Once the hooks and workflows call the surfaces, run `gate wiring validate`. It checks the declaration against those
files: each surface that holds entries is called by a hook or a workflow step, no caller hides a failed `gate run`, and
each local surface holds only the entries its allowed list names. The command reads the `gates.wiring` section, which
names the hook directory and the workflow directory. The rules are listed in
[Findings](../reference/findings.md#gate-wiring).

```console
$ rhino gate wiring validate
```

## 2. Call one surface from each caller

Each caller starts one surface and nothing else. The declaration decides what that surface runs. The local surfaces
(`pre-commit`, `commit-msg`, and `pre-push`) hold only the entries that their allowed list names, and rule W07 refuses
any other entry. Keep the hooks thin for that reason: a hook that runs a test or a scan slows every commit or push.

- `pre-commit` holds staged-file formatting, which rewrites and restages the files.
- `commit-msg` holds the commit-message format lint.
- `pre-push` holds the leak scan over the pushed range.
- `pull-request` holds every other check. A workflow step calls it on each pull request, so the checks run in parallel
  on a runner, not on one author's machine.
- `scheduled` or `main` holds the work that no pull request should wait for, such as an end-to-end suite. A workflow
  with the matching trigger calls it (rule W01).

A caller must not hide a failure. A line that ends the `gate run` call in `|| true`, or a workflow step with
`continue-on-error` around it, is refused (rule W05).

The examples below show the shape of each caller. Replace the executable with the one your repository declares.

```sh
#!/bin/sh
# .husky/pre-commit or .git/hooks/pre-commit
set -eu
rhino gate run --surface pre-commit
```

The declared gate's arguments and environment carry only the inputs its configuration binds. Do not add `-- "$@"`: v0.4
rejects arguments after `--` so a hook cannot replace a typed command with unreviewed argv.

A pull-request caller supplies its immutable range only when the declared surface binds that input:

```sh
rhino gate run --surface pull-request --base "$BASE_SHA" --head "$HEAD_SHA"
```

For `commit-msg`, pass Git's hook path with `--message-file`. Rhino confirms it is the current worktree's canonical
`COMMIT_EDITMSG` file before reading it; this works in linked worktrees and refuses another external path. For
`pre-push`, pass update records only with `--push-updates-stdin`. Rhino reads standard input only when that flag is
given, and then also hands the records, unmodified, to each gate child's standard input. Without it, Rhino never reads
or waits on standard input, and each child's standard input is empty. The command refuses the wrong surface/input
combination with exit `2`.

The complete placement of one repository's gates, with each check's scope and test level, is in
[the gate inventory](../../repo-governance/development/quality/gate-inventory.md) of RHINO itself.

## 3. Select regular Git destinations

Use `regular-files` when a formatter should receive only regular destinations changed in the staged index or an
immutable pull-request range. The selected paths still use the typed `.paths` projection:

```yaml
gates:
  entries:
    - id: format-files
      type: mutation
      inputs:
        files: { kind: regular-files }
      mutation:
        local: apply-index
        ci: verify-clean
      command:
        executable: formatter
        args:
          - { input: files.paths, expand: repeat }
      run-on:
        pre-commit:
          bind:
            files: { source: git-index }
        pull-request:
          bind:
            files: { source: explicit-range, range: explicit }
  composition:
    pull-request:
      relation: exact
```

Replace `formatter` with the repository's declared formatter executable. Git destination modes `100644` and `100755` are
selected; symlinks, gitlinks, and deletions are excluded. If all bound file inputs resolve to an empty selection, this
mutation passes without starting the formatter. The canonical mode and empty-selection rules are in the
[grouped v2 configuration reference](../reference/v0-4-configuration.md#lifecycle-gates).

The matching declaration in `schemas/repo-config/v2.example.yml` ships as the checked-in example; no gate re-proves it
since the schema parity test left with the Rust harness. This guide's consumer-specific YAML and the `gate validate` /
`gate run` lifecycle were not exercised here: RHINO has no consumer formatter or matching gate configuration, so no
formatter child was started.

## 4. Keep mutations explicit

A mutation pairs local `apply-index` behavior with CI `verify-clean` behavior. RHINO refuses to substitute the mutable
working tree for the declared index or disposable replay boundary. Environment initialization and toolchain provision
likewise require their own reviewed plan and explicit `--apply` authorization.

## 5. Handle results at the caller

| Exit code | Caller action                                                                            |
| --------- | ---------------------------------------------------------------------------------------- |
| `0`       | Continue; the selected surface completed cleanly.                                        |
| `1`       | Stop; a validator or a vault command found a violation, or a declared gate child failed. |
| `2`       | Stop; fix the invocation, configuration, or declared boundary.                           |
| `126`     | Stop; a required child (gate child, `bw`, or provision child) could not be executed.     |
| `127`     | Stop; a required child (gate child, `bw`, or provision child) was not found.             |
| `128+N`   | Stop; RHINO was ended by signal `N`.                                                     |

RHINO prints only a gate identifier and sanitized status. It never repeats a child stream that could contain material
unsafe to publish.

## 6. Check behaviour bindings

`behaviour coverage validate` checks that each scenario in the declared corpus has a unit `@scenario` reference, and
that it has an integration and an e2e reference or a valid exemption. It reads the feature files and the test files as
text: it runs no test and starts no process, so it is cheap enough for every pull request. A passing check proves the
bindings exist, not that the tests pass; the test runs prove that.

```console
$ rhino behaviour coverage validate
$ rhino behaviour coverage validate --rules B04,B05,B06,B07,B08
```

The second form runs only the named rules, which suits a spec review before any binding exists. The configuration is
`policies.behaviour`; see [the reference](../reference/v0-4-configuration.md#behaviour-policy) and the
[rules](../reference/findings.md#behaviour-coverage).

## Related

- [Grouped v2 Configuration](../reference/v0-4-configuration.md)
- [Command line](../reference/cli.md)
- [How to respond to exit codes](./respond-to-exit-codes.md)
