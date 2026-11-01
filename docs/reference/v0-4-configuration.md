# Grouped v2 Configuration

`rhino/repo-config/v2` is Rhino's grouped configuration contract. Its checked-in Draft 2020-12 schema is
[v2.schema.json](../../schemas/repo-config/v2.schema.json). Rhino validates the document from local bytes; it never
fetches the modeline URL or any schema while checking a repository.

```yaml
schema: rhino/repo-config/v2
repository: {}
scan: {}
harness: {}
policies:
  markdown: {}
  governance: {}
  conventions: {}
  plans: {}
environment: {}
toolchains: {}
gates: {}
extensions: {}
```

`schema` is required. The other core groups are optional: an omitted group declares no policy. An unknown top-level key
is a configuration error. Beyond that, the binary refuses an unknown key only where a validator reads it:

- Each validator named here refuses an unknown key inside the policy group it reads. The validators are the seven
  `policies.markdown` sub-policies (`frontmatter`, `heading-hierarchy`, `internal-link`, `mermaid`, `naming`,
  `readme-index`, and `metadata`), the six `policies.governance` sub-policies (`word-budget`, `directory-map`,
  `quality-gates`, `layers`, `traceability`, and `vendor`), and `policies.conventions.license`.
- `policies.conventions.emoji` is the exception. The emoji validator reads the group, but it accepts an extra key on the
  group and on a `prohibited` entry, with no error and no finding.
- The umbrella keys `policies`, `policies.markdown`, `policies.governance`, `policies.conventions`, and `policies.plans`
  are read whole by no validator. An unknown key under them is not reported by `repo-config validate` or by any leaf.
  `policies.plans` has no validator at all.
- Under `scan`, the scan-based leaves (for example `md mermaid validate` and `md internal-link validate`) refuse an
  unknown key. Under `environment`, `toolchains`, and `harness`, the leaves `env validate`, `toolchain validate`, and
  `harness adapters validate` refuse one. `repo-config validate` passes all of these keys.
- Gate argument, environment projection, and `run-on` entries refuse an unknown field under both `repo-config validate`
  and `gate validate`. A key directly on a gate entry or its command, and a key directly under `gates` or `repository`,
  pass both.

The checked-in schema is stricter than the binary. It rejects an unknown key at each level named here, including the
umbrella keys, the emoji group, and each `prohibited` entry. This page describes what the binary does, not what the
schema does.

`extensions` is the sole escape hatch. Each owner name matches `^[a-z][a-z0-9-]*$`, and its value is a mapping. Rhino
preserves that mapping without assigning it product meaning.

`scan.exclude-directories` is the optional shared list of directory names that the scan-based Markdown and governance
leaves exclude at any depth. It is explicit policy, not an inference from ignored files: an ignored local artifact
remains in scope until its repository declares otherwise.

## Portable policy groups

`policies.markdown` opts into frontmatter, heading hierarchy, internal-link, metadata, Mermaid, filename, and
README-index validation. `policies.governance` owns vendor vocabulary, layer structure, traceability declarations,
word-budget surfaces, directory-map trees, and quality-gate structure. `policies.conventions` owns configured license
paths, identifiers, digests, exact exclusions, and emoji-prohibited file surfaces. Each validator that reads one of
these groups refuses an undeclared or invalid sub-policy instead of inheriting a repository convention.

`governance word-budget validate` and `governance directory-map validate` read only `policies.governance.word-budget`
and `policies.governance.directory-map`, respectively. Their `count`, ordered `surfaces`, `trees`, findings, and
optional `scan.exclude-directories` retain the established leaf contracts; grouped v2 changes configuration ownership,
not the policy a repository declared.

`md heading-hierarchy validate`, `md naming validate`, and `metadata validate` read their respective `policies.markdown`
sub-policies. `convention emoji validate` reads `policies.conventions.emoji`. These grouped projections retain the
existing scanner, findings, and no-default behavior.

### Governance and license keys

`policies.governance.word-budget` needs `count` and `surfaces`.

- `count`: `letters-and-digits` counts runs of letters, marks, and digits, joined by an apostrophe, hyphen, or
  underscore. `whitespace-separated` counts the fields between whitespace.
- `surfaces`: a list of `{glob, fail}` entries, and both keys are required. `glob` selects Markdown files, and `fail` is
  the most words a selected file may hold. Where two entries match one file, the last one wins.
- `warn` and `target` are accepted in an entry and ignored. They are not checked, reported, or enforced.

`policies.governance.layers` needs `root`, and `order` and `categories` are optional.

- `root`: an exact repository path. Every direct child of `root` is a layer, so a file there is reported too.
- `order`: the layer names, which must each exist. A name uses only lowercase letters, digits, and hyphens.
- `categories`: a map from a layer named in `order` to the directory names that layer must hold. A missing category is
  reported, and so is a directory the layer does not list. Files directly inside a layer are not checked.

An omitted `order` reports every direct child of `root` as an undeclared layer. An omitted `categories` reports every
directory inside a declared layer. `categories` without `order` refuses the run with exit `2`.

`policies.governance.vendor` has four optional lists, each empty when omitted.

- `roots`: exact paths. Every file under a root is read.
- `excluded-binding-roots`: exact paths. Files under them are never read, even inside a root.
- `forbidden-terms`: strings. A file under a root that contains a term, case-sensitively, is a `forbidden-vendor-term`
  finding. An empty term refuses the run.
- `vocabulary-exceptions`: entries of `{term, paths}`. A term is accepted only at its exact `paths`. An exception with
  no `paths` accepts the term nowhere.

`policies.governance.traceability` has two optional lists, `artifacts` and `relationships`.

- `artifacts`: entries of `{id, path}`. An `id` uses lowercase letters, digits, and hyphens, and it must be unique. When
  the file at `path` is missing, the run reports `missing-traceability-artifact`.
- `relationships`: entries of `{from, to}`, and both must name declared ids. `from` must contain a local Markdown link
  that resolves to the `to` artifact's path, or the run reports `missing-traceability-relationship`. An undeclared id
  refuses the run with exit `2`.

`policies.conventions.license` has two optional lists, `paths` and `exclusions`.

- `paths`: entries of `{path, identifiers, sha256}`. `path` is an exact path and must not repeat. Each `identifiers`
  item must appear in the file, case-sensitively. `sha256` is 64 lowercase hexadecimal characters, the SHA-256 of the
  file's content. A missing file is `missing-license`, a missing identifier is `license-identifier-mismatch`, and a
  different digest is `license-digest-mismatch`.
- `exclusions`: exact paths. A listed path is neither read nor checked.

A `paths` entry without `path` refuses the run with exit `2`. Omitted `identifiers` and `sha256` skip their checks, so
an entry with only `path` exits `0` when the file exists.

### Mermaid policy

`policies.markdown.mermaid` configures `md mermaid validate`. Every key below except the required ones is optional, and
a configuration that omits an optional key behaves as it did before that key existed.

- `authoring-rule` (`rendered` or `plain-text`, absent by default): the one rule for conceptual diagrams.
- `node-label-graphemes`, `edge-label-graphemes` (integers, required): the longest visible label segment.
- `fill-colors`, `edge-colors`, `text-colors` (hex lists, required): the declared fills, outlines, and text colours.
- `exclude` (glob list, absent by default): paths the Mermaid scan leaves out. Declared, it replaces
  `scan.exclude-directories` for this command.
- `require-default-class` (boolean, `false`): require a complete `classDef default` where the renderer applies one.
- `allowed-types` (name list, absent by default): the permitted diagram types. Declared, every type is judged and an
  unlisted one is refused.
- `forbid-theme-overrides` (boolean, `false`): refuse initialization directives and front-matter themes.
- `canvas-colors` (hex list, empty by default): the canvases a filled class must stay visible against.

`canvas-colors` takes six-digit hex colours only, and an invalid `exclude` glob is a configuration fault: either makes
the command refuse with exit `2` instead of reporting findings. Declaring `allowed-types` also reads a colour by what it
is assigned to and refuses a sequence-diagram `box` that names a colour. The checks and finding details are in
[Findings](findings.md#mermaid).

### Quality-gate policy

`policies.governance.quality-gates` configures `governance quality-gates validate`, which checks the structure of a
repository's bounded quality gates and never judges their prose. Every key is required except `retired-inputs`,
`defaults`, and `gates`.

```yaml
policies:
  governance:
    quality-gates:
      root: repo-governance/workflows
      agents: .agents/agents
      groups: [plan, quality, maintenance]
      gate-group: quality
      gate-headings: [Entry, Inputs, Deterministic Boundary, Cycle, Termination, Verdict, Ledger]
      verdict-heading: Verdict
      verdicts: [PASS, PASS_WITH_FINDINGS, FAIL, BLOCKED]
      retired-inputs: [max-iterations, min-iterations, max-audits]
      propagation-headings: [Contract, Scope, Executor, Row Verification, Family Rules]
      defaults:
        mode: normal
        max-cycles: 3
      gates:
        - family: plan
        - family: ui-web
          subject: apps/web
        - { family: api-http, judge: api-tester, repairer: developer }
```

- `root`, `agents` (exact paths): the workflow root and the canonical agent directory. `agents` holds each declared
  family's judge and repairer agent files (QG08).
- `groups` (unique simple names): the only directories `root` may hold beside its `README.md`. A simple name uses only
  lowercase ASCII letters, digits, and hyphens.
- `gate-group` (one of `groups`): the group that holds every `<family>-quality-gate.md` and `<family>-propagation.md`.
- `gate-headings` (ordered): the second-level headings every gate carries, in this order. Other headings may appear
  anywhere.
- `verdict-heading` (one of `gate-headings`) and `verdicts`: the section, up to the next first- or second-level heading,
  whose all-capital code spans must be declared verdicts. An all-capital span has at least two characters, starts with a
  letter, and uses only `A`–`Z`, `0`–`9`, and `_`, such as `PASS_WITH_FINDINGS`.
- `retired-inputs`: names no gate may mention anywhere, fenced code included, as a whole name.
- `propagation-headings`: the second-level headings every propagation carries.
- `defaults.mode` (`lax`, `normal`, `strict`, or `all`) and `defaults.max-cycles` (1, 2, or 3): the defaults a gate run
  reads. RHINO supplies neither. `max-cycles` outside 1–3 is a configuration error (QG09), because a gate never runs
  more than three cycles. The default never changes what a gate file may declare: every gate is judged against 1–3.
- `gates[].family` (unique simple names) and the optional `gates[].subject`: the families this repository holds.
- `gates[].judge` and `gates[].repairer` (optional simple names): the agents, named without `.md`, that judge a family's
  gate and repair what it finds. Absent, they are `<family>-checker` and `<family>-fixer`. QG08 requires
  `<agents>/<judge>.md` and `<agents>/<repairer>.md`, and each `missing-gate-agent` finding carries `role` (`judge` or
  `repairer`) and `agent` details. Any other key on a `gates[]` entry is refused.

`gate-headings`, `propagation-headings`, and `verdicts` must each be non-empty and unique, and no `retired-inputs` entry
may be blank. A declaration that breaks any rule in this list is a configuration error with exit `2` for
`repo-config validate` and `governance quality-gates validate`, before any file is read; a command that does not read
the key is unaffected. A `defaults.max-cycles` outside 1–3 makes every command that loads `repo-config.yml` exit `2`,
including a command that reads no quality-gate policy.

Each finding names its rule as a `rule` detail: `QG01` to `QG08`, `QG10`, or `QG11`. QG09 is the configuration error
above, never a finding. The kinds are in [Findings](findings.md#quality-gates).

Every gate carries a `max-cycles` input, read from the Markdown table row whose first cell is `max-cycles`: every
integer in the row and its last cell, the default, must be 1 to 3. A gate with no such row, a row with no cell after the
name, or a value or default outside 1–3 is `unbounded-gate-cycles` (QG06). Headings and rows inside fenced code are not
read.

The split-workflow rule, `QG11`, applies to a module directory that sits beside its workflow and holds at least one
`NNN-*.md` module. It measures the workflow and its modules together against the `fail` limit of the
`policies.governance.word-budget` surface governing the workflow's entrypoint, and is skipped when no surface governs
it. A file that vanishes while this command reads it refuses the run with exit `2`.

### Surface globs and symbolic links

A surface glob — a word-budget, front-matter, heading-hierarchy, naming, or metadata surface, an emoji prohibition, or a
harness marker surface — follows a symbolic link it reaches when the link's target lies inside the repository root. The
file is reported by the path the glob matched, such as `gov/rules.md` for a `gov/**/*.md` surface where `gov` links to
`governance`. A glob reaches a link when the link sits at or under the glob's literal prefix, the segments before its
first wildcard, or when that prefix passes through the link.

A reached link that leads outside the root refuses the run with exit `2` and `rhino.path.escapes-root`. One that loops
back into a directory it is already inside, or whose target does not exist, refuses with `rhino.file.unreadable`. A link
no surface glob reaches is neither followed nor refused, and a link in a directory `scan.exclude-directories` names is
never considered. A glob that matches nothing still exits `0`. Every other read, including internal-link targets,
`--file` and `--directory` selections, and exact declared paths, still never follows a link.

`policies.plans` is a reserved owner boundary. It carries no v0.4 command in this release, so `repo-config validate`
does not check keys inside it, and an empty group is not a request to run a legacy plan validator.

### README index completeness

`md readme-index validate` reads `policies.markdown.readme-index.trees`. Each entry names an exact `path` and may list
`annotations` and `exclusions`, both relative to that path. `require-direct-children` chooses how much of the tree its
indexes must cover:

- `false` (default) — The declared directory has a `README.md`.
- `true` — That `README.md` also links every direct child of the declared directory.
- `every-directory` — The declared directory and every directory under it have a `README.md` that links its direct
  children, and every link resolves.

Under `every-directory`, each index must link every Markdown file beside it and every subdirectory that has its own
`README.md`. A subdirectory counts as linked when the index links the directory itself or its `README.md`, or when a
Markdown file named after it sits beside it, such as `area.md` beside `area/`; that file must itself be linked. Other
files are not required. Every relative link in each index must name a file or directory the repository holds; an
external address, an absolute path, an anchor alone, or a path climbing above the repository root is not checked. An
exclusion removes that child and everything under it from the walk, and so does a directory `scan.exclude-directories`
names, at any depth and in either child mode.

```yaml
policies:
  markdown:
    readme-index:
      trees:
        - path: docs
          require-direct-children: every-directory
```

A tree that omits the key, or sets `true` or `false`, is checked exactly as before. The findings are in
[the findings reference](findings.md#readme-index).

## Canonical adapter profiles

`harness` declares the portable requirements and a nonempty list of opaque profiles. Each profile explicitly lists the
capabilities, grants, denials, constraints, routes, and identities its documented native adapter can represent. The
canonical field vocabulary and each native adapter representation are also declared, so Rhino does not assume a
harness's path, format, front-matter keys, or route syntax. Canonical source remains `AGENTS.md`,
`.agents/skills/*/SKILL.md`, and the flat `*.md` files (excluding `README.md`) directly under the canonical agents root,
which is `.agents/agents/` unless `agents-root` names another root.

An agent or skill adapter has one `{name}` output placeholder, a native `front-matter` or `toml` format, an identity
mapping, optional fixed fields, capability translations, and an exact route containing one `{path}` placeholder. An
`instruction-adapter` is an individual managed file, rather than ownership of its parent directory. An agent adapter
shaped `.agents/agents/{name}/AGENT.md` — one `{name}` plus a literal trailing segment under `.agents/agents/` — is
likewise an individual managed file: generation merges each rendered file into the canonical agents tree instead of
replacing the directory. A flat `.agents/agents/{name}.md` has no trailing segment and still refuses, as does root
`AGENTS.md`, so generation can never overwrite a canonical source. The transaction replaces only the directory families
derived from agent and skill paths plus those exact instruction-adapter and exact-agent files; unrelated siblings remain
outside its authority.

```yaml
harness:
  canonical:
    agents:
      name: name
      description: description
      grants: requires
      denials: denies
      constraints: constraints
    skills: { name: name, description: description }
  requirements:
    capabilities: [read]
    grants: [files-read]
    routes: [canonical-import]
    identities: [reviewer]
  profiles:
    - id: alpha
      supports:
        {
          capabilities: [read],
          grants: [files-read],
          routes: [canonical-import],
          identities: [reviewer],
        }
      instruction-adapter: { path: CLAUDE.md, route: "@{path}" }
      agent-adapter:
        path: adapters/alpha/agents/{name}.md
        format: front-matter
        route-field: body
        route: "Read {path} completely."
        identity: { name: name, description: description }
      skill-adapter:
        path: adapters/alpha/skills/{name}/SKILL.md
        format: front-matter
        route-field: body
        route: "Read {path} completely."
        identity: { name: name, description: description }
    - id: beta
      supports:
        {
          capabilities: [read],
          grants: [files-read],
          routes: [canonical-import],
          identities: [reviewer],
        }
      agent-adapter:
        {
          path: "adapters/beta/agents/{name}.toml",
          format: toml,
          route-field: developer_instructions,
          route: "Read {path} completely.",
          identity: { name: name, description: description },
        }
    - id: gamma
      supports:
        {
          capabilities: [read],
          grants: [files-read],
          routes: [canonical-import],
          identities: [reviewer],
        }
      agent-adapter:
        {
          path: "adapters/gamma/agents/{name}.md",
          format: front-matter,
          route-field: body,
          route: "Read {path} completely.",
          fixed: { mode: subagent },
        }
```

`harness adapters validate` is read-only. `harness adapters generate` first validates and renders the entire projection,
then replaces only its declared families and exact files as one recoverable transaction. An unrepresentable requirement,
malformed source metadata, invalid native representation, or conflicting output refuses before any write.

### Canonical agents root

`agents-root` is an optional relative path under `harness` naming the repository root that holds the canonical agent
definitions. Omitted, discovery reads the historical `.agents/agents/` tree and every adapter output is byte-identical
to generation without the key. Declared, discovery and every `{path}` route resolve against that root instead, and the
old tree is no longer read. A declared root that holds nothing refuses with `rhino.harness.refused` naming the missing
root, and a root that climbs out of the repository, or is empty, refuses as an invalid path; both refuse before any
write.

```yaml
harness:
  agents-root: agents
```

### Tiers, absent fields, and capability translations

These optional keys shape an agent adapter's native fields beyond identity and `fixed` values. Every refusal below exits
`2` with `rhino.harness.refused` before any write.

**Tiers.** `canonical.agents.tier` names the canonical front-matter key that holds an agent's tier, such as
`tier: plan`. Once it is declared, every canonical agent must name a tier (`harness-tier-missing`), and every profile
with a `tiers` block must map each tier its selected agents name (`harness-tier-undeclared`). A profile with no `tiers`
block maps no tier and is exempt.

A profile's `tiers` maps each tier name to one of three forms, and its agent adapter's `tier-fields` names the two
native fields that receive them:

- `{ model: <name>, effort: <value> }` — pins both. A `model` or a `model-resolve` without an `effort`, an `effort`
  alone, or a `model` together with a `model-resolve` is refused while the configuration is read, naming
  `harness.profiles.<id>.tiers.<tier>`.
- `{ model-resolve: {...}, effort: <value> }` — resolves the model when adapters are generated; see below.
- `{}` — leaves the model and effort to the harness. The agent gets only the values its adapter's optional `empty-tier`
  declares, such as `empty-tier: { model: inherit }`, and no tier field when it declares none. `empty-tier` in a skill
  adapter, or without `tier-fields`, is refused.

`tier-fields` in a skill adapter, or with an empty or repeated field name, is refused.

```yaml
harness:
  canonical:
    agents:
      {
        name: name,
        description: description,
        tier: tier,
        grants: requires,
        denials: denies,
        constraints: constraints,
      }
  profiles:
    - id: alpha
      agent-adapter:
        # path, format, route-field, route, and identity as above
        tier-fields: { model: model, effort: effort }
        empty-tier: { model: inherit }
      tiers:
        plan: { model: large-model, effort: high }
        execution: {}
```

**Resolved models.** A `model-resolve` mapping names `command`, an argument vector run from the repository root with no
shell; `pattern`, a model name with exactly one `*` standing for a version of digits and dots; and `fallback`, a model
name. Only `harness adapters generate` starts the command, through its own model-resolver boundary with a 30 second
timeout. It reads the output as tokens of letters, digits, `.`, `_`, and `-`, picks the matching token with the highest
version compared as dot-separated numbers, renders it, and records it as `resolvedModels` in the binding family's
`provenance.json`. When the command fails, times out, or matches nothing, generation still succeeds: it renders the last
recorded model, or `fallback` when none was ever recorded, and prints a `[harness-adapters] warning:` line to stderr.
`harness adapters validate` never starts the command; it compares against the recorded model, or `fallback`.

```yaml
tiers:
  plan:
    model-resolve:
      { command: [scripts/list-models.sh], pattern: "large-*", fallback: large-4 }
    effort: high
```

**Dispatch lists.** `canonical.agents.dispatches` names the canonical front-matter key that lists the agents an agent
may dispatch, written as a block list such as `dispatches:` followed by the indented lines `- writer` and `- tester`; a
flow list such as `dispatches: [writer, tester]` is refused. In canonical metadata it is an optional list that follows
`constraints`. An agent adapter's optional `dispatches` projects that list, in canonical order, into one native field:

- `{ field: tools, format: "Agent({names})" }` — a member template with exactly one `{names}` placeholder adds one
  member to a list field, holding the names comma-separated: `Agent(writer, tester)`.
- `{ field: permission.task, format: allow-map }` — writes a map of `"*": deny` and one `<name>: allow` per name. A
  dotted field, at most one dot deep, nests the map under a map field that a translation's `entries` also writes.

An agent without a list renders nothing for it. `dispatches` in a skill adapter, a template without exactly one
`{names}`, a field more than one dot deep or with an empty segment, a field that collides with an identity, `fixed`, or
`lists` field, and an adapter `dispatches` with no `canonical.agents.dispatches` are each refused.

A member template `<P>({names})` declares `<P>` as the profile's spawn tool. An agent whose rendered field still holds a
bare `<P>` member, such as `Agent` beside or instead of `Agent(writer)`, may spawn any agent, so it is refused with
`harness-dispatch-unscoped`, naming the profile and the canonical agent, whether or not the agent declares a list. An
`allow-map` format, or a template without that exact `({names})` suffix, declares no spawn tool.

**Absent fields.** An adapter's `absent` lists native fields its output must never carry, such as
`absent: [model, effort]` for a harness that has no such setting. A render that would write one refuses, naming the
canonical source.

**Capability translations.** An adapter's `translations` project canonical capabilities into native fields. Each entry
has:

- `when` — `always`; `no-dispatches`, which applies to every agent that declares no dispatch list (an absent or empty
  `canonical.agents.dispatches` key); or `requires`, `denies`, or `constrains`: the entry applies when the agent's
  grants, denials, or constraints list (as `canonical.agents` names them) holds `capability`.
- `capability` — The capability to look for. Required for every `when` except `always` and `no-dispatches`, and refused
  with either.
- `field` — The native field written. It must not repeat an identity, `fixed`, or `lists` field.
- `members`, `absent-members`, or `entries` — Exactly one of the three. `members` adds values to a list field;
  `absent-members` removes values other entries added; `entries` adds keys to a map field.

Applying entries accumulate in declaration order. A `members` field renders as one comma-separated value, and an
`entries` field as a nested map in front matter or a table in TOML. Removals run after every addition, and removing
members from a field no applying entry wrote is refused. One field cannot mix member and map entries.

```yaml
translations:
  - { when: always, field: tools, members: [Read] }
  - { when: requires, capability: shell, field: tools, members: [Bash] }
  - { when: denies, capability: shell, field: tools, absent-members: [Bash] }
  - { when: always, field: permission, entries: { read: allow } }
  - { when: no-dispatches, field: permission, entries: { task: deny } }
```

The `no-dispatches` entry denies the spawn tool to every agent without a list, while an agent with one renders its
`dispatches` allow map in `permission.task` instead, so the two never meet in one binding.

### Agent lists and selection

Two optional agent-adapter keys carry more of an agent into a profile. Omitting both leaves every generated file
byte-identical.

- `lists` maps a native field to a canonical agent metadata list. The only projectable list is `skills`; `capabilities`
  and `constraints` reach native output through translations instead, and any other value refuses the configuration. The
  generator renders the list in the order the canonical source wrote it: as a block sequence in `front-matter` format,
  or as an array in `toml` format. An agent with no list, or an empty one, renders no field. The projection covers every
  agent the adapter renders, with no per-agent opt-out, so adding `lists` to a profile gives each already-generated
  agent that declares `skills` the new field at the next `harness adapters generate`.
- `agents` lists the canonical agent names a profile renders. Only those agents render for that profile, its
  `catalog.json` and `provenance.json` list only them, and `harness adapters validate` reports every other file under
  that family root as `stale-adapter`. A name with no canonical source refuses before any write. When `agents` is
  omitted, every canonical agent renders.

Neither key applies to a skill adapter; declaring one there refuses.

### Marker surfaces

`marker-surfaces` is an optional list of globs under `harness` naming files outside every adapter family, such as a
harness's own settings file, for adapter validation to inspect. A matched file with a line containing `Rhino generated`,
in any letter case, claims to be RHINO output. No adapter generation writes that phrase or such a file, so
`harness adapters validate` and `harness adapters generate` report it as `stale-marker` and exit `1`. A file under a
declared family root or at an exact adapter path is never inspected here, because the adapter comparison already covers
it, and neither is a file in a directory `scan.exclude-directories` names. A glob that climbs out of the root refuses
with `rhino.config.unusable`, a matched file that cannot be read refuses with `rhino.file.unreadable`, and a file
holding no text is skipped. Like every surface glob, a marker surface follows a symbolic link only as
[surface globs and symbolic links](#surface-globs-and-symbolic-links) describes. Omitted, nothing is inspected.

```yaml
harness:
  marker-surfaces: [".codex/config.toml"]
```

```yaml
agent-adapter:
  path: adapters/alpha/agents/{name}.md
  format: front-matter
  route-field: body
  route: "Read {path} completely."
  identity: { name: name }
  lists: { skills: skills }
  agents: [reviewer]
```

With a canonical agent declaring `skills` as `zeta-check` then `alpha-check`, that adapter renders:

```markdown
---
name: reviewer
skills:
  - zeta-check
  - alpha-check
---

Read .agents/agents/reviewer.md completely.
```

The same list projected by a `toml` adapter as `lists: { preload: skills }` renders
`preload = ["zeta-check", "alpha-check"]`.

## Environment files and toolchains

`environment.examples` declares public example-source and local-target path pairs. `environment.contracts` names public
declaration paths and key names; `environment.detectors` names one of six curated languages and exact source paths.
`env validate` reads only those source-code paths plus optional Git-index names, and never reports file contents.
`env init` plans the pairs by default and creates only absent targets with `--apply`; it rejects escaping, duplicate,
same-path, unreadable, and existing targets before a write. `env backup --dir` copies only declared local targets into a
new repository-relative destination. `env restore --dir` refuses a conflicting target unless `--force` explicitly
authorizes a recoverable replacement.

```yaml
environment:
  examples:
    - source: .env.example
      target: .env.fixture
  staged:
    forbidden: [".env*"]
    allowed: [".env.example"]
  contracts:
    - path: .env.example
      keys: [SERVICE_TOKEN]
  detectors:
    - language: rust
      paths: [src/main.rs]
toolchains:
  entries: []
```

An omitted `environment` or `toolchains` group refuses its operation. An explicitly empty `toolchains` group permits the
zero-tool plan or `--apply` result without launching a child.

When `environment.staged` is present, `env validate` checks only paths from the Git index. It reports a forbidden staged
path unless an `allowed` pattern matches. `allowed` accepts exact repository-relative paths only, so it cannot silence a
directory or class of files; validation never reads staged or unstaged file contents.

Each detector path is exact. The curated languages are `rust`, `type-script`, `f-sharp`, `go`, `terraform`, and
`ansible`. Detected reads must name a contracted key, and each contracted key must be read. A suppression requires one
exact rule, source path, key, reason, owner, and ISO expiry; it cannot suppress a whole detector class. For a declared
TypeScript path, Rhino recognizes `process.env.KEY`, quoted bracket access, and all-uppercase line-leading schema
properties. For a declared Go path, it recognizes direct reads plus the injected `os.LookupEnv, "KEY"` composition-root
form. Both forms remain local to the declared files; Rhino neither discovers directories nor assumes a framework.

`toolchains.entries` declares a probe executable/argv, optional output parser and exact version, required status,
optional positive `timeout-seconds`, and platform-specific provision executable/argv. `toolchain validate` runs probes
only. `toolchain provision` plans platform vectors by default, requires `--apply` to run them, and stops at the first
failure without reporting tool output.

## Lifecycle gates

The closed surfaces are `pre-commit`, `commit-msg`, `pre-push`, `pull-request`, `main`, `scheduled`, and `manual`; `ci`
is not a v0.4 surface. Each gate has one lowercase-kebab-case semantic ID and declares direct `run-on` membership. A
pull request explicitly composes the local union with `exact` or `at-least`; an at-least-only ID has its own non-empty
`reason`.

Inputs are named as `files`, `commit-message`, `commit-range`, or `repository-state`, then bound by a generic source
such as `git-index`, `hook-message-file`, `push-updates`, `explicit-range`, or `checkout`. Commands are an executable,
typed argv mappings, and explicit environment projections; they are never a shell string. Use `gate list` to inspect the
seven surfaces and `gate validate` to check declaration, composition, binding, and projection without starting a child.
An omitted `gates` group makes `gate list`, `gate validate`, and `gate run` exit `2`; `gates: {}` declares no gate and
reports clean.

A `files` input binds `git-index` for a local mutation or `explicit-range` with `range: explicit` for a pull-request
replay. The latter resolves only changed repository-relative paths from the immutable `base..head` pair; it never turns
the complete checkout into a formatter argument list. `checkout` remains for a check that intentionally reads every
visible path or repository-state.

An opt-in `regular-files` input accepts only `git-index` or `explicit-range` with `range: explicit`. RHINO reads raw Git
metadata for changed paths and exposes eligible repository-relative destinations through the same `.paths` projection.
Destination modes `100644` and `100755` are included; symlinks (`120000`), gitlinks (`160000`), and deletions (`000000`)
are excluded. Malformed or unmerged records make selection fail. This selects by Git's recorded destination mode, not by
the current working-tree type. It is useful when a formatter should receive regular-file destinations while tracked
links stay outside its argument list.

For a mutation that declares `regular-files`, RHINO resolves and projects all bound paths first. If their combined file
selection is empty, the mutation passes without starting its child; a nonempty selection that also includes a legacy
`files` input still runs, and check gates keep their existing empty-input invocation. The legacy `files` input retains
its existing selection and selected-link behavior. See
[How to wire RHINO into your gates](../how-to/wire-rhino-into-your-gates.md#3-select-regular-git-destinations) for a
formatter declaration.

The same `commit-message` input can bind `hook-message-file` at `commit-msg` and `explicit-range` with `range: explicit`
at `pull-request`. For the latter, Rhino reads the non-merge commit-message text in the immutable `base..head` range
after it has validated both commit IDs. The gate keeps its one semantic ID, command, and typed `message.text`
projection; only its declared source changes by lifecycle surface. At `commit-msg`, Rhino asks Git for the current
worktree's canonical `COMMIT_EDITMSG` path and reads only that file. This also supports linked worktrees without
widening the repository file reader to an arbitrary external path.

A `push-updates` range also declares its repository comparison `fallback`. A normal update uses the remote object as
base. A new ref resolves that declared ref to an immutable base, while a deleted ref skips the range gate because it has
no head to inspect. Rhino refuses an absent, invalid, or unresolved fallback rather than inventing a range.

A mutation pairs `local: apply-index` with `ci: verify-clean`. Rhino refuses to run it until its index/disposable-replay
boundary is available, rather than falling back to the mutable working tree.

For the legacy `files` input, the index snapshot retains an unselected tracked symlink as opaque link data; it never
resolves that target. On the local `apply-index` surface, a selected symlink is refused before the declared mutator
starts, so a formatter cannot follow it outside the snapshot. The CI `verify-clean` surface runs the mutator on a
disposable candidate checkout without that selected-symlink preflight. The `regular-files` input excludes non-regular
Git destinations from its selection.

A mutation child runs from its disposable boundary, but a relative `PATH` segment keeps the meaning it had for the
calling repository. Rhino resolves that segment from the original repository root before the child starts; an absolute
segment remains unchanged. A consumer therefore declares a portable tool name without embedding its machine-local
installation path. When the original repository has a `node_modules/.bin` directory, the mutator's `PATH` searches it
first, so a formatter installed in the project is found on every surface.

On `apply-index`, the selected paths are copied from the Git index into a temporary snapshot, and the mutator runs
there. A changed file has its new bytes written to the index, and to the working tree when that copy matches the old
blob. A working-tree copy with unstaged edits is kept as it is, and the change is reported as a divergence. A selected
file the mutator deletes is removed from the index. A change to any path outside the selection refuses the run with exit
`2`. Mutation children see `RHINO_GATE_SURFACE` as check children do.

On `verify-clean`, the candidate revision is checked out in a detached, disposable worktree, the mutator runs there, and
any change to the replay is a finding with exit `1`. The disposable checkout is removed afterwards, even on failure.

## Modelines

Rhino's source example uses a relative modeline because its schema is checked in beside it:

```yaml
# yaml-language-server: $schema=./v2.schema.json
```

A consumer pins an immutable release URL instead. The consumer fixture shows the canonical modeline shape; it never
makes the Rhino runtime perform a network request. Pin the release you run: its schema describes the keys that binary
reads, and `policies.governance.quality-gates` needs a `v0.9.0` or later schema. For the umbrella keys, which no
validator reads, and for the emoji group and its `prohibited` entries, which accept extra keys, the schema is stricter
than the binary: it flags a key the binary accepts.

## Verification

The validation sequence below was not exercised during this documentation pass. Separate source-gate evidence does not
record execution of this example sequence.

There is no schema generator in this tree, so a model change cannot regenerate the artifact; that tooling is tracked
work. Verify local configuration bytes before review:

```sh
rhino repo-config validate
```

## Stable predecessor handling

Stable accepts only grouped v2 configuration. It rejects predecessor schemas and does not provide
`rhino repo-config migrate`; create the grouped document, pin its immutable modeline, and validate local bytes offline.

Follow [How to migrate from v0.3 to v0.4](../how-to/migrate-to-v0-4.md) for the owner-by-owner transition and no-alias
boundary.
