# 🦏 RHINO

**Repository Hygiene & INtegration Orchestrator** — keep a repository honest
about the policy it wrote down.

[![Quality gate](https://github.com/wahidyankf/rhino/actions/workflows/pr-quality-gate.yml/badge.svg)](https://github.com/wahidyankf/rhino/actions/workflows/pr-quality-gate.yml)
[![Rust](https://img.shields.io/badge/rust-1.85-000000)](https://www.rust-lang.org/)
[![Platforms](https://img.shields.io/badge/platforms-macOS%20%7C%20Linux-lightgrey)](#-install)
[![License](https://img.shields.io/badge/license-MIT-blue)](./LICENSE)

RHINO is a Rust CLI that checks what a repository's documentation must get
right — word budgets on governed instructions,
directory maps that match the tree, internal Markdown links that resolve,
Mermaid diagrams that stay legible, and one canonical instruction body kept in
parity across every coding harness the repository declares.

```console
$ rhino governance directory-map validate
[directory-map] checked 7 directories, no findings
```

## ✨ Highlights

- **No policy of its own.** There is no default word limit, no default palette,
  and no default harness roster in the binary. Every value it enforces arrives
  from your `repo-config.yml`.
- **Knows no harness by name.** The roster is data. A fourth harness is one more
  entry in your configuration, not a new release of the tool.
- **Two exit codes that mean two things.** `1` is always "your repository
  violates your policy". `2` is always "the invocation or the configuration was
  wrong, and nothing was checked".
- **Says what it looked at.** A glob that matches nothing also reports no
  findings; only the list of scanned paths tells you which happened.
- **Read-only by construction.** It never writes to the repository it inspects,
  never starts a subprocess, and never touches the network.
- **Machine-readable on request.** `--output json` gives the same run as one
  line, with the same exit code.

## 🤔 Why

Documentation rots differently from code. A broken function fails a test; a
directory map that lost a file, a link that lost its target, or an agent prompt
that drifted from its canonical body all keep rendering perfectly. Nothing
announces them. They are found by a reader who trusted them.

The usual fix is a repository-specific script — glob and regex encoding one
repository's conventions, lendable to no other. The next repository writes its
own, and the two disagree quietly.

RHINO separates the checking from the policy. The checks live in one binary; the
values live in each repository's `repo-config.yml`. Four repositories with four
different conventions run the same tool and get four different, correct answers.

Longer version: [Why RHINO exists](./docs/explanation/why-rhino-exists.md).

## 📦 Install

Archives for macOS and Linux on `amd64` and `arm64`:

```sh
BASE=https://github.com/wahidyankf/rhino/releases/download/v0.1.3
curl -fsSLO "$BASE/rhino-aarch64-apple-darwin.tar.gz"
curl -fsSLO "$BASE/checksums.txt"
shasum -a 256 --ignore-missing -c checksums.txt
tar -xzf rhino-aarch64-apple-darwin.tar.gz
```

Verify before extracting. Pin the tag and its SHA-256; never follow `main` at
runtime. See [how to install a pinned
release](./docs/how-to/install-a-pinned-release.md), or build from a checkout
with `cargo install --path .`.

## 🚀 Quick start

Write what your repository enforces:

```sh
cat > repo-config.yml <<'END'
# schema: rhino/repo-config/v1

governance-word-budget:
  count: letters-and-digits
  surfaces:
    - glob: "AGENTS.md"
      fail: 1200

governance-directory-map:
  trees:
    - path: docs

md-internal-link:
  exclude-sources: []

md-mermaid:
  node-label-graphemes: 32
  edge-label-graphemes: 24
  fill-colors: ["#0173B2", "#DE8F05", "#029E73"]
  edge-colors: ["#0173B2", "#000000"]
  text-colors: ["#000000", "#FFFFFF"]

harness-parity:
  canonical:
    instruction: AGENTS.md
  harnesses: []
  prohibited-instruction-sources: []
  capabilities: []
  constraints: []

scan:
  exclude-directories:
    - .git
END
```

Check the configuration itself, then run the validators:

```console
$ rhino repo-config validate
[repo-config] checked 1 configuration file, no findings

$ rhino governance word-budget validate
$ rhino governance directory-map validate
$ rhino md internal-link validate
$ rhino md mermaid validate
$ rhino harness parity validate
```

Six more read an optional section. Undeclared, each exits `2` naming it,
never a convention RHINO chose for you:

```console
$ rhino md naming validate
$ rhino md frontmatter validate
$ rhino md heading-hierarchy validate
$ rhino md readme-index validate
$ rhino convention emoji validate
$ rhino metadata validate
```

`metadata validate` is the one whose schemas RHINO owns. A repository declares
which trees hold governance documents, workflows, skills, and agents; the path
then selects the schema. It reports `path:line:column rule field message`, a
format the validators above deliberately keep out of, so nothing a consumer
already records changes shape.

A repository may instead declare `ose/repo-config/v2`, which adds three
structural commands and an ordered list of gates. It carries the same validator
sections `v1` does, under the same names, so choosing it costs you none of the
commands above; what it adds is the gate registry and the four structural
commands below. The three here read the shared governance contract rather than a
section you write, and a repository still on `v1` is refused rather than held to
a contract it never adopted:

```console
$ rhino governance roots validate
$ rhino governance companions validate
$ rhino governance instructions validate
```

A fourth reads the plans tree:

```console
$ rhino plan validate
```

It holds a formal plan to the shared plan-validator contract: four lifecycle
roots with stage-aware slugs, six documents and exactly one technical shape,
companions numbered contiguously from 001 and indexed in reading order,
acceptance identifiers defined once and cited only where defined, and archival
after the substantive phases. Its diagnostics are
`path:line:column rule field message`, its twenty rule identifiers are stable,
and it judges nothing else: whether a plan is any good is not visible in its
shape.

They check that `repo-governance/` holds only registered categories and no empty
governed directory, that a companion directory is named exactly after its
entrypoint and indexed in reading order, and that `AGENTS.md` carries the five
spine sections in order with `CLAUDE.md` importing it exactly.

Two optional keys shape existing commands under `v2`. `mermaid.authoring-rule`
picks one diagram style per repository — `rendered` requires `accTitle` and
`accDescr` on every diagram, `plain-text` refuses Mermaid outright — and
declaring neither holds a repository to neither. `harness-parity`'s
`agent-adapter.tier-fields` names the two fields a generated adapter projects a
model tier into, so a harness that declares none is not held to the mapping.

`rhino gate run --surface pre-commit` runs the gates that surface
selects, in order, stopping at the first failure. Each child is given the
repository root, the surface in `OSE_GATE_SURFACE`, the hook's arguments, and
its standard input; its own output is never repeated. Exit `3` means a child
never started.

Full walkthrough: [Validate your first
repository](./docs/tutorials/validate-your-first-repository.md).

## ⚙️ How it works

RHINO reads `repo-config.yml` from the repository root, refuses to run if it
cannot understand it, walks the tree once, and reports every place the tree
disagrees with what you declared.

The configuration is a contract rather than a set of hints. A missing required
key is exit `2`, not a fallback; an unknown key inside a section RHINO owns is
exit `2`, not a warning; a declared path that escapes the repository root is
exit `2`. The one thing RHINO will never do is supply a value you did not write
down, because a tool that guessed there would report a clean run on a repository
it never really inspected.

Every command reports what it inspected — how many files, directories, links,
diagrams, or harnesses — whether or not it found anything, so "clean" and
"looked at nothing" are never the same output.

## 📚 Documentation

Full documentation lives in [`docs/`](./docs/README.md) and follows the
[Diátaxis framework](https://diataxis.fr/).

| Section                                     | Use it when                                        |
| ------------------------------------------- | -------------------------------------------------- |
| [Tutorials](./docs/tutorials/README.md)     | You are new and want to learn by doing             |
| [How-to guides](./docs/how-to/README.md)    | You have a specific goal and need the steps        |
| [Reference](./docs/reference/README.md)     | You need an exact command, flag, code, or key      |
| [Explanation](./docs/explanation/README.md) | You want to understand why RHINO is built this way |

The [specifications tree](./specs/README.md) is canonical:
[`specs/architecture.md`](./specs/architecture.md) holds the as-built C4 model
and [`specs/behaviours/`](./specs/behaviours/README.md) holds the executable
Gherkin corpus that every test adapter runs. Where this README and the corpus
disagree, the corpus wins.

## 📋 Project status

Early construction. The behaviour is pinned by an executable specification.
Versions below `1.0.0` may make breaking changes — see the
[changelog](./CHANGELOG.md).

Released tags will be immutable. A published release is never rebuilt or
replaced.

**External contributions are currently closed.** Issues and pull requests from
outside the project are not being accepted while the engineering patterns
stabilize. You are welcome to fork the repository under the MIT license and use
it however you like.

Contributor rules for the maintainer and automated agents start at
[`AGENTS.md`](./AGENTS.md), an index into
[`repo-governance/`](./repo-governance/README.md).

## 🌙 Part of Open Sharia Enterprise

RHINO is one of the **OSE Code Repositories**, alongside `ose-public`, `hippo`,
`beaver-nest`, and repositories that are not public. It supplies their
repository hygiene. The name is navigation, not coupling — see
[project context](./docs/README.md#project-context).

RHINO has no OSE-specific values compiled into it and is designed to be used
entirely on its own — consumers supply their own budgets, trees, palettes, and
harness rosters.

## 🛠️ Development

Source contributors need Rust 1.85 or newer.

```console
$ cargo xtask test-quick
```

That is format, lint, the unit adapter with line coverage, and the static
coverage check — the same gate the pre-push hook runs. Heavy local work runs
under the pinned [HIPPO](https://github.com/wahidyankf/hippo) guard via
`./hippo run --class ephemeral --disk-path . -- <command>`.

Commits follow [Conventional Commits](https://www.conventionalcommits.org/).

Only `main` persists. Work reaches it through a pull request from a branch in
`worktrees/`; direct pushes are refused for every actor, with no bypass. One
aggregate `Quality gate` check, defined in
`.github/workflows/pr-quality-gate.yml`, is required, and it is a superset of
the Git hooks.

### Test topology

One corpus, three executing boundaries, and one static check over the mapping
between them. Each test is classified by the strongest real boundary its setup,
subject, or assertions touch — not by the resources it is permitted to use.

| Target                   | Boundary    | What it touches                                                           |
| ------------------------ | ----------- | ------------------------------------------------------------------------- |
| `cargo test --test unit` | unit        | the corpus against an in-memory tree, entirely in process                 |
| `--test integration`     | integration | the corpus and seven boundary policies against a real temporary directory |
| `--test e2e`             | E2E         | the corpus and one boundary policy against the spawned executable         |
| `--test coverage`        | static      | executes no scenario; reads the corpus and each adapter's registry        |

Every scenario is bound at all three executing boundaries, with **one declared
exemption**: a sentence in the corpus is a claim about behaviour that holds
whether the tree is a map, a directory, or a process's working directory, so a
boundary that could not run one is a boundary at which the claim is untested,
and has to say so.

The exemption is `harness-parity :: A file that vanishes between the walk and
the read is not a finding`, at integration and E2E, because a real directory
cannot be made to drop a file mid-run without a second thread racing the run and
putting the outcome in the scheduler's hands. The unit adapter drives the same
`TreeError::NotFound` through the same port and is not exempt, so the behaviour
is proved, and `Sandbox::build` refuses the precondition outright so the
exemption cannot decay into a step that passes by doing nothing.

`no_leaf_writes_to_the_repository_it_inspects` is deliberately split across two
boundaries: integration proves nothing was written through the repository root,
and only E2E — where the inspected repository is also the process's working
directory — can see a write to a relative path.

Only the unit adapter and the static check run in the quick gate and the push
hook. Integration and E2E never do.

### Coverage

`cargo xtask test-quick` measures unit line coverage in the same execution that
runs the unit adapter, and fails below **99%**. Measured once rather than twice,
because two runs can disagree and the number that gates has to be the number the
passing run produced.

Two modules are excluded from the denominator, and they are the only two:

| Excluded              | Why a unit test may not reach it                                                                       |
| --------------------- | ------------------------------------------------------------------------------------------------------ |
| `src/main.rs`         | the process boundary — arguments, standard input, the working directory, stdout, stderr, the exit code |
| `src/runtime/disk.rs` | the only module that talks to `std::fs`                                                                |

Neither is untested. Both are proved by the integration and E2E adapters running
the whole corpus through them, plus the boundary policies — which is a stronger
claim than a line count, and the reason a number that included them would say
less rather than more.

## 📄 License

RHINO is available under the [MIT License](./LICENSE).
