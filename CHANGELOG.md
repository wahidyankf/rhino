# Changelog

All notable changes to RHINO are recorded here. This project follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html), and released tags
are immutable — a published release is never rebuilt or replaced.

Entries describe what a consumer can observe: commands, flags, exit codes,
finding kinds, configuration keys, and output. They are not a commit list. For
the commits behind any release, see its
[comparison on GitHub](https://github.com/wahidyankf/rhino/releases).

## Unreleased

### Changed — breaking

- **A surface glob follows a symbolic link inside the repository.** A
  word-budget, front-matter, heading-hierarchy, naming, or metadata surface,
  or an emoji prohibition, whose glob passed through a symbolic link matched
  nothing and exited `0`, so the files it was declared to govern were never
  read. A glob now follows a link it reaches when the link's target resolves
  inside the repository root, and reports each file by the path the glob
  matched. A configuration that passed by inspecting nothing can now report
  findings. See
  [surface globs and symbolic links](docs/reference/v0-4-configuration.md#surface-globs-and-symbolic-links).
- **A surface glob refuses a link it cannot follow.** A reached link whose
  target lies outside the root refuses with exit `2` and
  `rhino.path.escapes-root`. One that loops back into a directory it is
  already inside, or whose target does not exist, refuses with exit `2` and
  `rhino.file.unreadable`. A link no surface glob reaches, or one in a
  directory `scan.exclude-directories` names, is neither followed nor
  refused. A glob that matches nothing still exits `0`, and every other read
  still never follows a link.

### Added

- **`marker-surfaces` under `harness`, and the `stale-marker` finding.** An
  optional list of globs naming files outside every adapter family, such as a
  harness settings file. `harness adapters validate` and
  `harness adapters generate` report a matched file holding a
  `Rhino generated` line as `stale-marker` and exit `1`: no RHINO release
  writes that marker, so a region labelled as RHINO output there is owned by
  nothing and could drift unchecked. Generation still writes its adapters
  first. A file under a family root or at an exact adapter path is never
  inspected. Omitted, nothing is inspected, so an existing configuration
  behaves as before. The v2 configuration schema describes the key. See
  [marker surfaces](docs/reference/v0-4-configuration.md#marker-surfaces).

## [v0.6.0] — 2026-09-25

Agent adapters can now carry an agent's skills preload and render a
per-harness agent set, so a repository migrating hand-authored agents to
canonical sources can keep each harness's behaviour. Both keys are optional and
additive. A configuration that declares neither generates byte-identical
adapters. The version moves in the minor position because RHINO is in `0.x`.
See [agent lists and selection](docs/reference/v0-4-configuration.md#agent-lists-and-selection).

This release also brings the binary back in line with its published contract
where the two had drifted. Those corrections change what some invocations
report, so read **Changed — breaking** before upgrading a pinned version.

### Changed — breaking

- **An omitted `gates` group is refused.** `gate list`, `gate validate`, and
  `gate run` now exit `2` with `rhino.config.undeclared` when the
  configuration leaves the `gates` group out, as every other omitted group
  already did. They used to read the omission as an empty lifecycle and report
  clean. `gates: {}` still declares a repository that runs no gate, and still
  exits `0`.
- **Error codes now match their published meanings.** The exit status is
  unchanged in every case below; only `error.code` under `--output json`
  moves. A caller that branches on these codes should check each one.
  - An omitted policy on a tree validator, and an omitted `harness` group,
    report `rhino.config.undeclared`, as `env` and `toolchain` already did.
    They used to report `rhino.config.unusable` and `rhino.harness.refused`.
  - A `--file` or `--directory` selection that does not exist reports
    `rhino.file.missing`. A `--file` that exists and cannot be read still
    reports `rhino.file.unreadable`. Both used to report something else: an
    absent `--file` reported `rhino.file.unreadable`, and an absent
    `--directory` reported `rhino.config.unusable`.
  - A file the run had to read and could not open reports
    `rhino.file.unreadable` rather than `rhino.config.unusable`. This covers
    a Markdown file in the walk, a README a directory map reads, and a path a
    policy declares.
  - `gate run --surface` with an unknown surface reports
    `rhino.args.unrecognized`. Leaving `--surface` out still reports
    `rhino.args.incomplete`.
  - A `gate run` input the invocation cannot supply reports by cause: a
    missing `--base`, `--head`, `--message-file`, or `--push-updates-stdin`
    reports `rhino.args.incomplete`. A `--base` or `--head` that is not a
    commit ID reports `rhino.args.unrecognized`. A missing or unreadable
    message file reports `rhino.file.missing` or `rhino.file.unreadable`. A
    malformed pre-push update record reports `rhino.input.unreadable`. A Git
    index, range, or ref the repository cannot resolve reports
    `rhino.repository.unusable`. All of these used to report
    `rhino.config.unusable`.
  - An `environment.staged` guard in a repository with no Git index reports
    `rhino.repository.unusable` rather than `rhino.config.unusable`.
  - An inconsistent `toolchains` policy reports `rhino.config.unusable`
    rather than `rhino.toolchain.failed`.
  - `env init --apply`, `env backup`, and `env restore` that fail while
    writing, creating, or renaming a file report the new
    `rhino.file.unwritable` rather than `rhino.config.unusable`.

### Added

- **`lists` on an agent adapter.** Maps a native field to the canonical agent
  metadata list `skills`, rendered in authored order: a block sequence in
  `front-matter` format, an array in `toml` format. An agent without the list
  renders no field. Any other canonical list refuses the configuration.
- **`agents` on an agent adapter.** Names the canonical agents that profile
  renders. Its `catalog.json` and `provenance.json` list only those agents,
  `harness adapters validate` reports any other file under the family root as
  `stale-adapter`, and a name with no canonical source refuses before any
  write. Omitted, every canonical agent renders as before.
- The v2 configuration schema describes both keys.
- **`rhino.file.unwritable`.** A new error code: a file that had to be
  written could not be written. Adding a code moves the minor version, because
  a caller may treat the vocabulary as closed. A harness adapter that cannot be
  written still reports `rhino.harness.refused`.

### Fixed

- **A symbolic link is never followed out of the repository.** A tree
  validator could read outside the selected root when a path passed through a
  linked directory, although the walk already skipped links. No path is now
  read through a filesystem link at any component. An internal link whose
  target lies behind one is reported as `internal-link-outside-repository`. A
  declared path behind one is never read. A license path or an environment
  example source behind one refuses the run with exit `2` and
  `rhino.file.unreadable`. An environment target behind one also refuses with
  exit `2`: `rhino.file.unwritable` where the command would write it, and
  `rhino.file.unreadable` where `env backup` would read it. A `--file` or
  `--directory` selection behind one is refused with exit `2` and
  `rhino.path.escapes-root`.
- **`env init` and `toolchain provision` plans honour `--output json`.**
  Without `--apply`, both wrote their text plan line even under
  `--output json`, so a caller piping them into `jq` failed. They now write
  one object on one line, with `status` `planned`, the planned `count`, and
  the planned `targets` or `toolchains`. Text output is unchanged. See
  [operation status documents](docs/reference/json-output.md#operation-status-documents).
- **A declared path outside the root or behind a symbolic link is never a
  quiet pass.** A directory-map tree that was absolute or climbed out of the
  root matched nothing, so the run exited `0`. So did a naming exemption, an
  internal-link source exclusion, an emoji prohibition, a word-budget or other
  surface glob, or a staged-environment pattern that did the same. Each now
  refuses with exit `2` and `rhino.config.unusable`, naming the setting and
  the value, as a license path already did. A directory-map tree, vendor root, or layer root
  behind a symbolic link was skipped as if absent. It now refuses with exit `2`
  and `rhino.file.unreadable`. An environment detector source that exists and
  cannot be read, including one behind a link, refuses the same way.
  `declared-source-unread` now means only that the source does not exist.
- **`env backup` reports the copy it made.** After copying the declared files
  it reported `"status":"planned"` and the text line `planned declared files`.
  It now reports `completed`, the `count` of files written, and the written
  `files`, and its text line reads `completed <count> declared files`. A policy
  with no eligible file also reports `completed`, with a `count` of `0`.
  `status` names the invocation's mode, not its effect. Only `env init` and
  `toolchain provision` without `--apply` report `planned`. Every executing form
  reports `completed` when it finishes, and `env init --apply` and
  `env restore` now also carry `count` and the written `targets`, so a caller
  can tell whether anything was written.

## [v0.5.0] — 2026-09-22

RHINO now reports from one closed status vocabulary and one closed error-code
vocabulary, described in [exit codes](docs/reference/exit-codes.md) and
[error codes](docs/reference/error-codes.md). The version moves in the minor
position because RHINO is in `0.x`, where that is where a breaking change goes.

### Changed — breaking

- **Exit `3` is retired.** A gate child that could not be started now reports
  `127` when it was not found and `126` when it was found and could not be
  executed — the two statuses a shell already reports for these situations. A
  gate child refused for any other reason reports `2`. A caller matching on `3`
  must be updated.
- **An internal failure reports `2`.** A panic used to reach the caller as
  SIGABRT, so the process reported `134`. It now prints
  `rhino: internal error: …` on stderr and exits `2`. The backtrace stays
  behind `RUST_BACKTRACE`.
- **A closed pipe reports `141`.** Writing to a pipe whose reader has gone away
  ends the process the way the signal says to, rather than as `134`.
- **A failed `gate run` leaves stdout empty.** It used to write a result
  document describing a run that had not happened. A caller parsing stdout can
  now rely on it holding a result or holding nothing.

### Added

- **A namespaced `error.code` in the JSON error body.** Under `--output json`
  every refusal writes one document to stderr carrying `schemaVersion`,
  `error.code`, and `error.message`. The code is drawn from a closed,
  [published vocabulary](docs/reference/error-codes.md) and is stable; the
  message is for a person and is not.
- **`--version` and `-V`**, alongside the existing `version` command.
- **A `help` command**, alongside `--help` and `-h`.

### Fixed

- **A bare invocation is a usage mistake.** `rhino` with no command prints one
  line on stderr pointing to `rhino --help`, and exits `2` rather than
  reporting success.
- **A standard-input read failure is reported.** It used to be discarded, so a
  stream that failed halfway through was indistinguishable from an empty one
  and the caller was told its input held no findings.
- **`--json` is honoured by a refusal.** The shorthand now selects the JSON
  error body the same way `--output json` does.
- **`--` ends the options.** It used to mean "forward the rest to a child",
  which nothing consumed.
- **`--help` places `--json` on the leaf that accepts it** rather than in the
  global options block, and publishes every status RHINO can report.

## [v0.4.2] — 2026-09-20

### Fixed

- **Generated harness adapters are formatter-stable.** YAML scalar values that require quoting now use YAML literal
  blocks, and generated catalog and provenance JSON uses stable pretty formatting. Regeneration no longer rewrites
  consumer formatter output.

## [v0.4.1] — 2026-09-20

### Fixed

- **Environment initialization refuses symbolic-link sources before opening or writing.** A declared example source
  is now read through a no-follow boundary. `env init --apply` returns an invocation refusal and leaves declared
  targets absent when that source is a link.

## [v0.4.0] — 2026-09-19

Breaking. **New and migrated repositories use the closed grouped
`rhino/repo-config/v2` contract.** Stable rejects predecessor schemas and has
no compatibility aliases, wrappers, or migration command.

### Added

- **Grouped v2 configuration and schema** — one typed model owns closed core
  groups, portable extensions, the checked-in Draft 2020-12 schema, and local
  offline validation. Markdown, governance, convention, lifecycle, harness,
  environment, and toolchain policy is explicit and repository-neutral.
- **Explicit operation boundaries** — typed lifecycle gates, three-profile
  adapter validation/generation, environment transactions, and toolchain
  provision plan their work before a separately declared mutation boundary.
- **Immutable schema release asset** — every release checksum manifest covers
  the four native archives and the grouped v2 schema. The release workflow
  publishes that exact checked-in schema only after it matches the typed model.

### Changed

- **Stable migration is explicit.** Convert a predecessor configuration with
  the documented owner-by-owner mapping, then validate the grouped document.
  Stable rejects predecessor schemas and does not register the retired command
  leaves or `--harness` selector.

### Fixed

- **Linked-worktree commit hooks keep a narrow message-file boundary.** At
  `commit-msg`, Rhino asks Git for that worktree's canonical `COMMIT_EDITMSG`
  path and reads only that file. A linked worktree therefore works even though
  Git keeps its message outside the checkout; another external path is refused
  before a gate child starts.
- **Gate children receive product-scoped surface metadata.** `gate run` now
  exposes its selected surface as `RHINO_GATE_SURFACE`; it no longer overwrites
  a consumer's `OSE_GATE_SURFACE` adapter variable.
- **Pull-request file gates receive their immutable changed-file selection.** A
  `files` input may bind `explicit-range` on `pull-request`, resolving only the
  repository-relative paths changed between the supplied immutable commits.
  Mutation children no longer receive every path in a large checkout merely
  because the pull-request surface selected them.
- **Environment detection now recognizes declared TypeScript schema properties
  and injected Go lookups.** An exact TypeScript detector path can declare an
  all-uppercase schema key without a redundant `process.env` read. An exact Go
  detector path recognizes the normal `gofmt` composition-root form
  `os.LookupEnv, "KEY"`; a nonliteral injected key remains an
  `unsupported-dynamic-access` finding. Both forms stay confined to the
  repository files the consumer declares.
## [v0.3.0] — 2026-09-11

Additive. **An existing consumer changes nothing but its pin.** The twelve
commands that shipped before this release read `rhino/repo-config/v1` exactly as
they did, and the six this release adds read a second schema, `ose/repo-config/v2`,
that no existing consumer declares. A repository on v1 that runs one of the new
commands gets exit `2` naming the schema it would need, never a governance
convention RHINO chose on its behalf.

Both schemas carry the validator sections. A repository declares one schema or
the other -- v1 in a leading comment, v2 in a leading key -- so a repository that
chose v2 for gate dispatch would otherwise have given up every command that
reads a surface list. The sections keep the names and shapes v1 gave them and
are read by the same code, so a rule keeps one implementation and a policy is
written in one spelling either way. What differs is requiredness: v1 requires
five sections and says so while parsing, v2 requires none and lets each command
refuse for the section it needed.

### Added

- **`ose/repo-config/v2`** — a second schema beside v1, not a replacement. Told
  apart by where the declaration is: v1 in a leading comment, v2 in a leading
  `schema:` key, so no document reads as both. Eighteen keys in a checked order:
  `schema`, `visibility`, `governance`, `model-tiers`, `scan`, `harness-parity`,
  `metadata`, `governance-word-budget`, `governance-directory-map`,
  `md-frontmatter`, `md-heading-hierarchy`, `md-internal-link`, `md-mermaid`,
  `md-naming`, `md-readme-index`, `convention-emoji`, `gates`, `extensions`.
  `schema`, `visibility`, and `gates` are required; every other key is optional
  and is refused if declared with nothing in it. A v2 document that declares no
  `scan` excludes nothing from its walks -- a repository's own answer rather
  than a default RHINO supplied.
- **`rhino gate run --surface <name>`** — runs the gates the surface selects, in
  declaration order, stopping at the first failure. Each child is started
  directly with no shell, is told which surface selected it through
  `OSE_GATE_SURFACE`, and receives everything after `--` appended to its own
  arguments. A `mutation` gate may run only at `pre-commit`. Reads `gates` in
  `ose/repo-config/v2`.
- **`rhino governance roots validate`** — the governance tree's layers and
  categories, and that no governed directory is empty. Finding kinds:
  `unknown-governance-layer`, `undeclared-governance-category`,
  `unused-local-category`, `empty-governed-directory`.
- **`rhino governance companions validate`** — companion sets: a suffix-free
  directory named after its document, an index covering every live companion,
  and contiguous ordinals when the set is ordered. Finding kinds:
  `suffixed-companion-directory`, `missing-companion-index`,
  `missing-indexed-companion`, `unindexed-companion-module`,
  `non-contiguous-companion-ordinals`, `ordinal-in-unordered-set`,
  `unordered-module-in-ordered-set`.
- **`rhino governance instructions validate`** — the five-section `AGENTS.md`
  spine in one order, and `CLAUDE.md` as exactly `@AGENTS.md`. An absent
  `CLAUDE.md` is legal. Finding kinds: `missing-canonical-instruction`,
  `missing-spine-section`, `misordered-spine-section`,
  `interrupted-instruction-spine`, `inexact-instruction-import`.
- **`rhino plan validate`** — plan structure under `plans/`: lifecycle root and
  slug form, the six required documents, one technical shape, companion
  ordinals, acceptance identifiers, and delivery order. Twenty rule identifiers
  in five families (`PLAN-LIFECYCLE-`, `PLAN-DOCUMENT-`, `PLAN-COMPANION-`,
  `PLAN-CRITERION-`, `PLAN-DELIVERY-`), frozen because more than one
  implementation reports them and a consumer compares them by equality.
  Structure only: nothing here judges whether a plan is any good.

  The delivery grammar is stated rather than inferred, because a reader that
  guesses at it reports a plan's prose as a defect. A bullet is a checklist item
  when it carries a task marker -- `- [ ]` or `- [x]` -- or when it opens with a
  bare executor label, `- [AI] …`. The label may be code-formatted once a marker
  has already made the bullet an item, so `- [x] \`[AI]\` …` is one; a quoted
  label with no marker is a plan explaining its own notation and is not. A
  bullet opening with a markdown link is never an item. A second-level heading
  is a delivery phase only when it holds items, so `## Execution Checkout`,
  `## Delivery Boundaries` and a trailing `## Related Documents` are read as the
  structure they are rather than as unnumbered phases.
- **`rhino metadata validate`** — front matter against the schema its path
  selects: `governance`, `workflow`, `skill`, or `agent`. Twenty-three finding
  kinds, all prefixed `metadata-`. Reads the new optional `metadata` section,
  which both schemas carry.
- **`model-tiers`** — an optional section in both schemas, mapping a portable
  tier (`ultra`, `plan`, `execution`, `fast`) to a harness's model and effort,
  so an agent definition can name a tier rather than a vendor's model string.
- **Exit `3`** — `gate run` only: a gate child that could not be started. Kept
  distinct from `1` because a gate that never ran says nothing about the
  repository, and reporting the second as the first would let a broken hook read
  as a caught violation.
- **A second diagnostic shape** — `path:line:column rule field message`, used by
  the six new commands and frozen by the contract they share. The validators
  that shipped earlier keep the shape they had: a consumer's stored output may
  not change because a new command arrived.

## [v0.2.0] — 2026-09-10

Additive. **An existing consumer changes nothing but its pin.** All five new
configuration sections are optional; a `repo-config.yml` that declares none of
them validates clean, and the six commands that shipped before this release
behave exactly as they did. The five new commands exit `2` naming their missing
section rather than enforcing a convention RHINO chose.

### Added

- **`rhino md naming validate`** — filenames against the style their surface
  declares. Two styles: `kebab-case`, and `path-prefixed`, where a filename
  begins with its own directory path encoded, then a declared separator, then a
  kebab-case content name. The encoded prefix is derived from the path being
  walked, so a repository declares the separator and nothing else. Reads
  `md-naming`. Finding kind: `invalid-md-name`.
- **`rhino md frontmatter validate`** — front matter against the schema its
  surface declares: `require`, `enum`, `iso-date`, and `forbid`. `forbid` is
  what lets a repository keep a rule RHINO knows nothing about without RHINO
  having to know what the rule is for. Reads `md-frontmatter`. Finding kinds:
  `unterminated-frontmatter`, `missing-frontmatter-key`,
  `invalid-frontmatter-value`, `forbidden-frontmatter-key`.
- **`rhino md heading-hierarchy validate`** — one level-1 heading per governed
  document, and no heading dropping further below its predecessor than the
  repository allows. A `#` inside a fenced block is not a heading. Reads
  `md-heading-hierarchy`. Finding kinds: `missing-h1`, `multiple-h1`,
  `heading-level-jump`.
- **`rhino md readme-index validate`** — a `README.md` in every directory of a
  declared tree, presence only. Deliberately weaker than
  `governance directory-map validate`, so a repository with many READMEs and no
  map sections can adopt this rung today and the stronger one later. Reads
  `md-readme-index`. Finding kind: `missing-readme-index`.
- **`rhino convention emoji validate`** — no emoji code point in a file matching
  a declared glob. Only the prohibition is expressible: whether an emoji belongs
  in a sentence is a judgement about meaning. This is the one command that reads
  files no Markdown corpus contains. Reads `convention-emoji`. Finding kind:
  `emoji-in-prohibited-file`.

### Changed

- `rhino --help` lists thirteen commands; `rhino md --help` lists seven.

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
