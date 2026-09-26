# How to narrow a run to one file or tree

You want RHINO's answer about part of a repository — one diagram you are
editing, one documentation tree, another checkout — without waiting for or
reading the whole thing.

Narrowing asks a smaller question. It never gives a quieter answer to the same
one: a narrowed run reports every finding it inspects.

## One file, or several

`--file` replaces the declared surface of `md mermaid validate`, the only
command that accepts it:

```sh
rhino md mermaid validate --file docs/architecture.md
```

It is repeatable, and the paths are inspected together:

```sh
rhino md mermaid validate --file docs/one.md --file docs/two.md
```

Any other command refuses it with exit `2` rather than ignoring it:

```console
$ rhino md internal-link validate --file docs/README.md
rhino: `--file` is not accepted by `md internal-link validate`
```

Paths are **repository-relative**. An absolute path is refused, because the
answer would then depend on where the command ran, and so is a path whose `..`
segments climb out of the root:

```console
$ rhino md mermaid validate --file /etc/hosts
rhino: `--file /etc/hosts` is absolute, and a selection is relative to the repository root
$ rhino md mermaid validate --file docs/../../notes.md
rhino: `--file docs/../../notes.md` escapes the repository root
```

A path that passes through a symbolic link is refused the same way, with exit
`2` and `rhino.path.escapes-root`, even when the link's target lies inside the
root: a `--file` or `--directory` selection never follows a link. Only a
declared surface glob follows one, and only to a target inside the root; see
[why RHINO validation only reads](../explanation/reading-only.md#what-it-cannot-reach).

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

## One grouped directory tree

`governance directory-map validate` reads the grouped governance policy and can
inspect one declared directory tree at a time.

`--directory` replaces the declared trees for directory-map validation:

```sh
rhino governance directory-map validate --directory docs
```

It **replaces** rather than adds. A path that is not a directory is refused
with exit `2` rather than reported as a clean walk of zero directories, because
naming nothing is a mistake in the invocation, not a fact about the repository:

```console
$ rhino governance directory-map validate --directory README.md
rhino: [directory-map] README.md is not a directory in this repository
```

## A different repository entirely

`--root` points RHINO at another checkout. It may appear before or after the
command path:

```sh
rhino --root ../other-repo md internal-link validate
rhino md internal-link validate --root ../other-repo
```

The selected root brings its own `repo-config.yml`, its own surfaces, and its
own scan exclusions. A root holding no configuration is exit `2`.

`--root` names the repository rather than a selection inside it, so any
directory is accepted, `..` included. The containment rule applies to the
`--file`, `--directory`, and `--dir` values chosen inside that root.

## What narrowing does not change

The exit code contract, the finding shape, the output format, and the rules
themselves. Only the set of things inspected changes — and the run still
reports how many that was, so a narrowed run that matched nothing is visible
rather than silent.

## Related

- [Command line](../reference/cli.md)
- [Exit codes](../reference/exit-codes.md)
