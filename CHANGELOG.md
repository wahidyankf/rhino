# Changelog

All notable changes to RHINO are recorded here. This project follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html), and released tags
are immutable — a published release is never rebuilt or replaced.

Entries describe what a consumer can observe: commands, flags, exit codes,
finding kinds, configuration keys, and output. They are not a commit list. For
the commits behind any release, see its
[comparison on GitHub](https://github.com/wahidyankf/rhino/releases).

## [Unreleased]

The first release. Nothing is published yet, so everything below is new.

### Added

- **Eight commands.** `repo-config validate`, `governance word-budget validate`,
  `governance directory-map validate`, `md internal-link validate`,
  `md mermaid validate`, `md word-count inspect`, `harness parity validate`, and
  `version`.
- **A configuration contract, `repo-config.yml`.** Declared by a
  `# schema: rhino/repo-config/v1` comment before any content, with
  `rhino-cli/repo-config/v1` accepted as a predecessor alias. RHINO holds no
  default for anything it enforces: a missing required key, an unknown key
  inside a section RHINO owns, a value of the wrong type, or a declared path
  that escapes the repository root is refused rather than filled in.
- **Word budgets.** Ordered `glob`/`fail`/`warn` surfaces, last match winning,
  so a specific file can override the tree it sits in.
- **Directory maps.** Within a declared tree, every directory carries a
  `README.md` whose `## Directory Map` section names each direct sibling exactly
  once by a resolvable relative path.
- **Internal links.** Every relative Markdown link resolves, with a configurable
  list of source globs excluded from being checked — excluded as sources only,
  never removed from the set of valid targets.
- **Mermaid legibility.** Grapheme limits for node and edge labels, and declared
  palettes for fills, edges, and text, so a diagram cannot reach a colour the
  repository never approved.
- **Harness parity.** One canonical instruction body, one canonical skill bundle
  per skill, one canonical agent prompt per agent, reconciled against a declared
  roster of harnesses. A digest over the canonical files is reported on every
  run.
- **Adapters route and translate; they never copy.** Each harness declares where
  its adapters live (as a `{name}` pattern, so a file per document and a
  directory per document are both expressible), what document format they use,
  where the route sentence lives, which fields must match the canon, which are
  fixed, which are forbidden, and how each canonical capability becomes a
  permission in that harness's own vocabulary. The check is non-weakening rather
  than equality: an adapter may grant more than the canon requires, never less,
  and never what the canon denies. Semantic drift is reported apart from a
  wrong route, because they are different problems.
- **Only pairs are required.** A repository with harnesses may have no canonical
  skills, no canonical agents, and no capability server. What it may not do is
  declare half of anything: a root without its route, an adapter contract
  without a root, or a capability server without the per-harness declarations
  that satisfy it. All three are still refused alongside an *empty* roster,
  where they would reconcile nothing.
- **No harness is named in the binary.** The roster is configuration; a further
  harness is one more entry rather than a new release.
- **A three-value exit contract.** `0` clean, `1` the repository violates its
  declared policy, `2` the invocation or the configuration was wrong and nothing
  was checked. On `2` standard output is empty.
- **What was inspected, always.** Every command reports how many files,
  directories, links, diagrams, or harnesses it looked at, whether or not it
  found anything, so a clean run and a run that matched nothing are never the
  same output.
- **Narrowing.** `--file`, `--directory`, and `--harness` ask a smaller question
  without giving a quieter answer; `--file -` reads a document from standard
  input.
- **Machine-readable output.** `--output json` emits one `schemaVersion: 1`
  document per run, carrying the same exit code as the human report. `--json` is
  a shorthand on `version` alone; every other command refuses it rather than
  accept two spellings of the same thing.
- **Some things are deliberately not findings.** A file that vanished mid-run, a
  file that holds no text rather than text RHINO dislikes, a non-Markdown file
  containing the canonical import, a fenced example quoting it, and a `README.md`
  index in an adapter directory are all left alone. Each was a false positive a
  real repository produced.
- **Read-only by construction.** The product writes nothing to the repository it
  inspects, starts no subprocess, and opens no network connection — including
  loopback. Each of those is held by a boundary-policy test rather than by
  convention.

[Unreleased]: https://github.com/wahidyankf/rhino/commits/main
