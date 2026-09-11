# RHINO documentation

RHINO checks that a repository still matches the policy it wrote down — word
budgets, directory maps, internal links, diagram legibility, and one canonical
instruction body kept in parity across every coding harness the repository
declares.

## Start here

New to RHINO? [Validate your first
repository](./tutorials/validate-your-first-repository.md) takes about ten
minutes and ends with a finding you caused on purpose and then fixed.

Already know what you want to do? Jump to the [how-to guides](./how-to/README.md).

## Find the right kind of help

This documentation follows the [Diátaxis framework](https://diataxis.fr/), so
you can pick material that matches the question you have right now.

## Directory Map

- [Tutorials](./tutorials/README.md) — you are new and want to learn by doing.
- [How-to guides](./how-to/README.md) — you have a specific goal and need the steps.
- [Reference](./reference/README.md) — you need an exact command, flag, exit code, finding kind, or configuration key.
- [Explanation](./explanation/README.md) — you want to understand why RHINO is built this way.

The distinction that matters most: a **tutorial** teaches you something you did
not know how to want, while a **how-to guide** helps you do something you
already decided to do. If a page feels like the wrong shape for your question,
another section probably has the right one.

## The short version

- **What it is** — a standalone Rust CLI. No daemon, no network access, no
  subprocesses, and nothing written to the repository it inspects.
- **What it does** — reads your `repo-config.yml`, walks the repository, and
  reports every place the tree disagrees with what you declared.
- **How you use it** — `rhino <domain> <validator> validate`, in a gate or by
  hand.
- **What it will not do** — supply a value you did not declare. There is no
  default word limit, no default palette, and no default harness roster.

## Specifications

The [specifications tree](../specs/README.md) is canonical.
[`specs/architecture.md`](../specs/architecture.md) holds the as-built C4 model,
and [`specs/behaviours/`](../specs/behaviours/README.md) holds the executable
Gherkin corpus every test adapter runs. Where this documentation and the corpus
disagree about observable behaviour, the corpus wins — and that disagreement is
a bug worth reporting.

## Project context

RHINO is one of the **OSE Code Repositories** — the repositories
[Open Sharia Enterprise](https://github.com/wahidyankf/ose-public) is built and
maintained in. The public ones are:

| Repository                                                 | What it does                          |
| ---------------------------------------------------------- | ------------------------------------- |
| [`ose-public`](https://github.com/wahidyankf/ose-public)   | The OSE product platform and research |
| **`rhino`**                                                | Repository hygiene — this repository  |
| [`hippo`](https://github.com/wahidyankf/hippo)             | Host resource coordination            |
| [`beaver-nest`](https://github.com/wahidyankf/beaver-nest) | An independent family product         |

The set also contains repositories that are not public. This table lists what a
reader can actually open; a name nobody outside the organisation can reach is
not navigation, and publishing one says something about a private repository
that its owner did not publish.

RHINO supplies repository hygiene to the rest of the set and is being adopted
across them; it consumes [HIPPO](https://github.com/wahidyankf/hippo) for local
resource coordination. Nothing else crosses.

**That name is navigation, not coupling.** Each is developed, versioned, and
released independently — no shared version number, no shared release cadence, no
monorepo, and no parent repository above them. Membership means only that a
reader who finds one can find the others. RHINO is usable entirely on its own
and has no OSE-specific values compiled into it.
