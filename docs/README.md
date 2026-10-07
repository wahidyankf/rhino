# RHINO documentation

RHINO checks that a repository still matches the policy it wrote down — word budgets, directory maps, internal links,
diagram legibility, and one canonical instruction body kept in parity across every coding harness the repository
declares.

## Start here

New to RHINO? [Validate your first v0.4 repository](./tutorials/validate-your-first-v0-4-repository.md) starts with a
grouped policy and ends with a finding you caused on purpose and then fixed.

Already know what you want to do? Jump to the [how-to guides](./how-to/README.md).

## Find the right kind of help

This documentation follows the [Diátaxis framework](https://diataxis.fr/), so you can pick material that matches the
question you have right now.

## Directory Map

- [Tutorials](./tutorials/README.md) — you are new and want to learn by doing.
- [How-to guides](./how-to/README.md) — you have a specific goal and need the steps.
- [Reference](./reference/README.md) — you need an exact command, flag, exit code, finding kind, or configuration key.
- [Explanation](./explanation/README.md) — you want to understand why RHINO is built this way.

The distinction that matters most: a **tutorial** teaches you something you did not know how to want, while a **how-to
guide** helps you do something you already decided to do. If a page feels like the wrong shape for your question,
another section probably has the right one.

## The short version

- **What it is** — a standalone Rust CLI. No daemon or network access; tree validation spawns no subprocess and writes
  nothing to the repository it inspects.
- **What it does** — reads your `repo-config.yml`, walks the repository, and reports every place the tree disagrees with
  what you declared.
- **How you use it** — `rhino <domain> <validator> validate`, in a gate or by hand.
- **What it will not do** — supply a value you did not declare. There is no default word limit, no default palette, and
  no default harness roster.

## Specifications

The [specifications tree](../specs/README.md) is canonical. [`specs/architecture.md`](../specs/architecture.md) holds
the as-built C4 model, and [`specs/behaviours/`](../specs/behaviours/README.md) holds the executable Gherkin corpus
every test adapter runs. Where this documentation and the corpus disagree about observable behaviour, the corpus wins —
and that disagreement is a bug worth reporting.

## Project context

RHINO is one of the seven **`ose-projects`** repositories — the repositories
[Open Sharia Enterprise](https://github.com/wahidyankf/ose-public) is built and maintained in. Each entry gives the
repository's role, then how it relates to RHINO:

- **[`rhino`](https://github.com/wahidyankf/rhino)** — repository hygiene. This repository.
- [`hippo`](https://github.com/wahidyankf/hippo) — host resource coordination. Upstream consumption both ways: RHINO
  guards its local compute with a pinned HIPPO release, and HIPPO's contributor gates pin RHINO releases.
- [`ose-public`](https://github.com/wahidyankf/ose-public) — the OSE product platform and research. Upstream consumption
  both ways: it pins RHINO releases, and RHINO pins its FERRET release.
- _(unnamed, private)_ — authorized operations. Upstream consumption: it pins RHINO releases.
- [`beaver-nest`](https://github.com/wahidyankf/beaver-nest) — an independent family product. Upstream consumption: it
  pins RHINO releases.
- [`ose-rules`](https://github.com/wahidyankf/ose-rules) — the reference catalog of governance, planning, agent, and
  skill artifacts. Knowledge sharing: RHINO adopts its artifacts by explicit one-off copy and owns each copy. It also
  pins RHINO releases.
- [`py-typekit`](https://github.com/wahidyankf/py-typekit) — typed functional primitives for Python. Upstream
  consumption: it pins RHINO releases.

The private one stays unnamed on purpose: a name nobody outside the organisation can reach is not navigation, and
publishing one says something about a private repository that its owner did not publish.

**That label is navigation, not coupling.** `ose-projects` is a routing label only — not an organisation, a parent
repository, a parity group, or a shared release. The seven are developed, versioned, gated, and released independently,
with no shared version number, release cadence, or monorepo. Membership obliges each member only to name the others, so
a reader who finds one can find the other six, and nothing more. RHINO is usable entirely on its own and has no
OSE-specific values compiled into it.
