# 🦏 RHINO — Repository Hygiene & INtegration Orchestrator

A configuration-driven repository-hygiene validator. RHINO checks the things a
repository's documentation has to get right — word budgets on governed
instructions, directory maps that match the tree, internal Markdown links that
resolve, Mermaid diagrams that stay legible, and one canonical instruction body
kept in parity across every coding harness the repository declares.

It holds no repository's policy. There is no default word limit, no default tree
list, and no default harness roster compiled into the binary: every value it
enforces arrives from the consuming repository's `repo-config.yml`. The product
is read-only, network-free, and process-free.

## Project status

Early construction. The specification corpus under `specs/behaviours/` is the
canonical statement of behaviour and is being implemented against; where this
README and the corpus disagree, the corpus wins. No release is published yet, so
there is nothing to install.

## Development

```console
$ cargo xtask test-quick
```

Heavy local work runs under the pinned [HIPPO](https://github.com/wahidyankf/hippo)
guard via `./hippo run --class ephemeral --disk-path . -- <command>`.

## License

MIT.
