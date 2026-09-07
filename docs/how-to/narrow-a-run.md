# How to narrow a run to one file or tree

You want RHINO's answer about part of a repository — one diagram you are
editing, one documentation tree, one coding harness — without waiting for or
reading the whole thing.

Narrowing asks a smaller question. It never gives a quieter answer to the same
one: a narrowed run reports every finding it inspects.

## One file, or several

`--file` replaces the declared surface for the commands that read files:

```sh
rhino md mermaid validate --file docs/architecture.md
```

It is repeatable, and the paths are inspected together:

```sh
rhino md mermaid validate --file docs/one.md --file docs/two.md
```

Paths are **repository-relative**. An absolute path is refused, because the
answer would then depend on where the command ran:

```console
$ rhino md mermaid validate --file /etc/hosts
rhino: `--file /etc/hosts` is absolute, and a selection is relative to the repository root
```

## A diagram you have not saved

`-` reads standard input. Pipe in a Markdown document — the fenced block is
what RHINO looks for:

````console
$ printf '```mermaid\nflowchart LR\n    Alpha[a label that is far too long to read comfortably at all]\n```\n' \
    | rhino md mermaid validate --file -
[mermaid] checked 1 diagram, 1 finding
[mermaid] -:3: a node label is longer than the declared limit (segment=node, measured=55, limit=32)
````

The path in the finding is `-`, because that is what you named. This is the
fastest way to check a diagram while you are writing it — no file, no commit,
same rules.

## One directory tree

`--directory` replaces the declared trees for directory-map validation:

```sh
rhino governance directory-map validate --directory docs
```

It **replaces** rather than adds. A path that is not a directory is refused
with exit `2` rather than reported as a clean walk of zero directories, because
naming nothing is a mistake in the invocation, not a fact about the repository:

```console
$ rhino governance directory-map validate --directory README.md
[directory-map] README.md is not a directory in this repository
```

## One coding harness

```sh
rhino harness parity validate --harness claude
```

A name the repository does not declare is refused:

```console
$ rhino harness parity validate --harness nope
[harness-parity] `nope` is not a harness this repository declares
```

## A different repository entirely

`--root` points RHINO at another checkout. It may appear before or after the
command path:

```sh
rhino --root ../other-repo governance word-budget validate
rhino governance word-budget validate --root ../other-repo
```

The selected root brings its own `repo-config.yml`, its own surfaces, and its
own scan exclusions. A root holding no configuration is exit `2`.

`--root` may not leave the repository it names: a path containing `..` inside
the value is refused.

## What narrowing does not change

The exit code contract, the finding shape, the output format, and the rules
themselves. Only the set of things inspected changes — and the run still
reports how many that was, so a narrowed run that matched nothing is visible
rather than silent.

## Related

- [Command line](../reference/cli.md)
- [Exit codes](../reference/exit-codes.md)
