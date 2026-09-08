# Changelog

All notable changes to RHINO are recorded here. This project follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html), and released tags
are immutable — a published release is never rebuilt or replaced.

Entries describe what a consumer can observe: commands, flags, exit codes,
finding kinds, configuration keys, and output. They are not a commit list. For
the commits behind any release, see its
[comparison on GitHub](https://github.com/wahidyankf/rhino/releases).

## [v0.1.3] — 2026-09-08

### Added

- **A harness's own settings can be a prohibited instruction source.**
  `harness-parity.prohibited-instruction-fields` names a configuration file, its
  format, and one top-level key that may not carry always-on instructions — the
  same rule as "no second canonical instruction body", written in the vendor's
  syntax rather than as a path. The file stays legitimate; only the key is
  prohibited. A key nobody wrote, and a key written as an empty list, empty
  table, empty string, or `null`, are both answers rather than violations; a
  file that cannot be read in its declared format is reported, because a source
  that cannot be ruled out has not been ruled out. Optional: a repository that
  declares none is unaffected, and every existing configuration keeps working
  unchanged.

## [v0.1.2] — 2026-09-08

### Fixed

- **`harness parity validate` no longer reads the whole repository.** It read
  every file the walk listed, then used the content of almost none of them:
  every rule it applies is about the canonical instruction body, a file under a
  declared canonical or adapter root, a harness's capability declaration, a
  path a prohibited glob names, or an import — and only Markdown can express an
  import. Everything else was opened and discarded as soon as it turned out not
  to be text, which on a working tree means compiled artifacts, dialyzer
  tables, and database files. On a 4,649-file repository the command now reads
  **210 files and 1.9 MB instead of 4,649 and 98.9 MB**, and falls from
  **182 ms to 35 ms**. Output is unchanged: every command's JSON is
  byte-identical to `v0.1.1` on that repository.

### Changed

- **An unreadable file stops the run only if the run was going to read it.**
  Previously any file `harness parity validate` could not open was exit `2`,
  including files no rule it applies is about. A file matching a prohibited
  glob still stops the run whatever its kind, because that rule is about the
  path and the run has to be able to say what is in it.

### Added

- **The canonical instruction body and its adapter need not be Markdown.** They
  were always allowed to be anything the repository declared; nothing proved
  it, and narrowing what gets read is exactly the change that could have
  broken it silently.

## [v0.1.1] — 2026-09-08

### Fixed

- **Listing one directory no longer walks the whole repository.** The
  filesystem tree used the port's default `children`, which derives the answer
  from a full recursive walk — correct for any implementation and affordable
  only for the in-memory one. `governance directory-map validate` asks that
  question once per mapped directory, so on a real repository it walked the
  tree once per question and its cost grew with the repository rather than with
  what was being inspected. On a 49-map tree the command fell from **1,065 ms
  to 87 ms**, and the six-command gate that repository runs from **1,150 ms to
  178 ms**. Output is unchanged: every command's JSON is byte-identical to
  `v0.1.0` across three real repositories.

## [v0.1.0] — 2026-09-08

The first release, so everything below is new.

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
- **What counts as a word is declared, not assumed.** `governance-word-budget`
  takes a `count` of `letters-and-digits` or `whitespace-separated`. The two
  repositories being reconciled here already disagree, by 156 words on the same
  file, so either choice made silently would enforce a budget nobody set.
- **A capability's executable vector is read in every spelling.** The whole
  command line is what two harnesses agree on; whether it is split across
  `command` and `args` or written as one array is that vendor's syntax.
- **Quoted link syntax is not a link.** Inside a fenced block or an inline code
  span, `[name](name)` is characters a document is talking about rather than a
  destination it points at.
- **A route is compared as a sentence.** Runs of whitespace collapse on both
  sides before an adapter's route is matched against the canonical one, so an
  adapter a Markdown formatter wrapped is not drift.
- **The canon's own field names are configuration too.** A repository says
  which field of a canonical agent holds what it may do, what it may not, and
  how it must behave, and which fields every such agent must carry outright.
  RHINO ships no name for any of them, because a list read under a name nobody
  wrote comes back empty rather than missing: a guess that missed would fire no
  translation, compare nothing, and report the repository clean.
- **Only pairs are required.** A repository with harnesses may have no canonical
  skills, no canonical agents, and no capability server. What it may not do is
  declare half of anything: a root without its route or its declaration shape, an
  adapter contract without a root, or a capability server without the
  per-harness declarations
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
- **The release identity is spelled the way the tag is.** `version` reports
  `vX.Y.Z`, and `version --json` emits exactly
  `{"schemaVersion":1,"version":"vX.Y.Z","commit":"<40 hex>"}`, so a lock file
  holding a tag compares against it without reformatting. A build made outside a
  repository reports forty zeros for the commit rather than claiming a revision.
- **Read-only by construction.** The product writes nothing to the repository it
  inspects, starts no subprocess, and opens no network connection — including
  loopback. Each of those is held by a boundary-policy test rather than by
  convention.

[v0.1.3]: https://github.com/wahidyankf/rhino/releases/tag/v0.1.3
[v0.1.2]: https://github.com/wahidyankf/rhino/releases/tag/v0.1.2
[v0.1.1]: https://github.com/wahidyankf/rhino/releases/tag/v0.1.1
[v0.1.0]: https://github.com/wahidyankf/rhino/releases/tag/v0.1.0
