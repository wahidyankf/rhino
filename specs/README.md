# Specifications

This directory is the canonical statement of RHINO's observable behaviour. Where the documentation and these
specifications disagree, these win, and the disagreement is a bug worth reporting.

`specs/` sits at the repository root rather than under a project directory because this repository is the tool: there is
no second project to disambiguate from.

The behaviour corpus is executable. `go test ./tests/bdd` runs every scenario in `behaviours/` against the built `rhino`
binary. See [tests/bdd/README.md](../tests/bdd/README.md) for how the suite is organised.

## Directory Map

- [architecture.md](architecture.md) — the as-built Go C4 model: context, containers, components, ports, data, exit
  codes, and the test layout.
- [behaviours/](behaviours/README.md) — the Gherkin corpus: the grouped-v2 contract, the validators later releases added
  to it, and the command-line surface.
- [fixtures/](fixtures/README.md) — retained reference inputs for the grouped-v2 contract. The executable corpus does
  not read them.
