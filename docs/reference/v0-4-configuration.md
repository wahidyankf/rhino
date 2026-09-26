# Grouped v2 Configuration

`rhino/repo-config/v2` is Rhino's grouped configuration contract. Its checked-in
Draft 2020-12 schema is [v2.schema.json](../../schemas/repo-config/v2.schema.json).
Rhino validates the document from local bytes; it never fetches the modeline URL
or any schema while checking a repository.

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

`schema` is required. The other core groups are optional and closed: an omitted
group declares no policy, while an unknown group or key is a configuration
error. The policy subgroups are also closed.

`extensions` is the sole escape hatch. Each owner name matches
`^[a-z][a-z0-9-]*$`, and its value is a mapping. Rhino preserves that mapping
without assigning it product meaning.

`scan.exclude-directories` is the optional shared list of directory names that
the scan-based Markdown and governance leaves exclude at any depth. It is
explicit policy, not an inference from ignored files: an ignored local artifact
remains in scope until its repository declares otherwise.

## Portable policy groups

`policies.markdown` opts into frontmatter, heading hierarchy, internal-link,
metadata, Mermaid, filename, and README-index validation.
`policies.governance` owns vendor vocabulary, layer structure, traceability
declarations, word-budget surfaces, and directory-map trees.
`policies.conventions` owns configured license paths, identifiers, digests,
exact exclusions, and emoji-prohibited file surfaces. Each group refuses an
undeclared or invalid sub-policy instead of inheriting a repository convention.

`governance word-budget validate` and `governance directory-map validate` read
only `policies.governance.word-budget` and
`policies.governance.directory-map`, respectively. Their `count`, ordered
`surfaces`, `trees`, findings, and optional `scan.exclude-directories` retain
the established leaf contracts; grouped v2 changes configuration ownership,
not the policy a repository declared.

`md heading-hierarchy validate`, `md naming validate`, and `metadata validate`
read their respective `policies.markdown` sub-policies. `convention emoji
validate` reads `policies.conventions.emoji`. These grouped projections retain
the existing scanner, findings, and no-default behavior.

### Surface globs and symbolic links

A surface glob — a word-budget, front-matter, heading-hierarchy, naming, or
metadata surface, an emoji prohibition, or a harness marker surface — follows a symbolic link it reaches
when the link's target lies inside the repository root. The file is reported by
the path the glob matched, such as `gov/rules.md` for a `gov/**/*.md` surface
where `gov` links to `governance`. A glob reaches a link when the link sits at
or under the glob's literal prefix, the segments before its first wildcard, or
when that prefix passes through the link.

A reached link that leads outside the root refuses the run with exit `2` and
`rhino.path.escapes-root`. One that loops back into a directory it is already
inside, or whose target does not exist, refuses with `rhino.file.unreadable`.
A link no surface glob reaches is neither followed nor refused, and a link in a
directory `scan.exclude-directories` names is never considered. A glob that
matches nothing still exits `0`. Every other read, including internal-link
targets, `--file` and `--directory` selections, and exact declared paths, still
never follows a link.

`policies.plans` is a closed owner boundary. It carries no v0.4 command in this
release, so an empty group is not a request to run a legacy plan validator.

## Canonical adapter profiles

`harness` declares the portable requirements and exactly three opaque profiles.
Each profile explicitly lists the capabilities, grants, denials, constraints,
routes, and identities its documented native adapter can represent. The
canonical field vocabulary and each native adapter representation are also
declared, so Rhino does not assume a harness's path, format, front-matter keys,
or route syntax. Canonical source remains `AGENTS.md`,
`.agents/agents/*.md`, and `.agents/skills/*/SKILL.md`.

An agent or skill adapter has one `{name}` output placeholder, a native
`front-matter` or `toml` format, an identity mapping, optional fixed fields,
capability translations, and an exact route containing one `{path}`
placeholder. An `instruction-adapter` is an individual managed file, rather
than ownership of its parent directory. The transaction replaces only the
directory families derived from agent and skill paths plus those exact
instruction-adapter files; unrelated siblings remain outside its authority.

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

`harness adapters validate` is read-only. `harness adapters generate` first
validates and renders the entire projection, then replaces only its declared
families and exact files as one recoverable transaction. An unrepresentable
requirement, malformed source metadata, invalid native representation, or
conflicting output refuses before any write.

### Agent lists and selection

Two optional agent-adapter keys carry more of an agent into a profile. Omitting
both leaves every generated file byte-identical.

- `lists` maps a native field to a canonical agent metadata list. The only
  projectable list is `skills`; `capabilities` and `constraints` reach native
  output through translations instead, and any other value refuses the
  configuration. The generator renders the list in the order the canonical
  source wrote it: as a block sequence in `front-matter` format, or as an
  array in `toml` format. An agent with no list, or an empty one, renders no
  field. The projection covers every agent the adapter renders, with no
  per-agent opt-out, so adding `lists` to a profile gives each
  already-generated agent that declares `skills` the new field at the next
  `harness adapters generate`.
- `agents` lists the canonical agent names a profile renders. Only those agents
  render for that profile, its `catalog.json` and `provenance.json` list only
  them, and `harness adapters validate` reports every other file under that
  family root as `stale-adapter`. A name with no canonical source refuses
  before any write. When `agents` is omitted, every canonical agent renders.

Neither key applies to a skill adapter; declaring one there refuses.

### Marker surfaces

`marker-surfaces` is an optional list of globs under `harness` naming files
outside every adapter family, such as a harness's own settings file, for
adapter validation to inspect. A matched file with a line containing
`Rhino generated`, in any letter case, claims to be RHINO output. No adapter
generation writes that phrase or such a file, so `harness adapters validate`
and `harness adapters generate` report it as `stale-marker` and exit `1`. A
file under a declared family root or at an exact adapter path is never
inspected here, because the adapter comparison already covers it, and neither is a file in a directory
`scan.exclude-directories` names. A glob that
climbs out of the root refuses with `rhino.config.unusable`, a matched file
that cannot be read refuses with `rhino.file.unreadable`, and a file holding no
text is skipped. Like every surface glob, a marker surface follows a symbolic
link only as [surface globs and symbolic links](#surface-globs-and-symbolic-links)
describes. Omitted, nothing is inspected.

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

With a canonical agent declaring `skills` as `zeta-check` then `alpha-check`,
that adapter renders:

```markdown
---
name: reviewer
skills:
  - zeta-check
  - alpha-check
---

Read .agents/agents/reviewer.md completely.
```

The same list projected by a `toml` adapter as `lists: { preload: skills }`
renders `preload = ["zeta-check", "alpha-check"]`.

## Environment files and toolchains

`environment.examples` declares public example-source and local-target path
pairs. `environment.contracts` names public declaration paths and key names;
`environment.detectors` names one of six curated languages and exact source
paths. `env validate` reads only those source-code paths plus optional Git-index
names, and never reports file contents. `env init` plans the pairs by default
and creates only absent targets with `--apply`; it rejects escaping, duplicate,
same-path, unreadable, and existing targets before a write. `env backup --dir`
copies only declared local targets into a new repository-relative destination.
`env restore --dir` refuses a conflicting target unless `--force` explicitly
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

An omitted `environment` or `toolchains` group refuses its operation. An
explicitly empty `toolchains` group permits the zero-tool plan or `--apply`
result without launching a child.

When `environment.staged` is present, `env validate` checks only paths from the
Git index. It reports a forbidden staged path unless an `allowed` pattern
matches. `allowed` accepts exact repository-relative paths only, so it cannot
silence a directory or class of files; validation never reads staged or unstaged
file contents.

Each detector path is exact. The curated languages are Rust, TypeScript, F#,
Go, Terraform, and Ansible. Detected reads must name a contracted key, and each
contracted key must be read. A suppression requires one exact rule, source path,
key, reason, owner, and ISO expiry; it cannot suppress a whole detector class.
For a declared TypeScript path, Rhino recognizes `process.env.KEY`, quoted
bracket access, and all-uppercase line-leading schema properties. For a
declared Go path, it recognizes direct reads plus the injected
`os.LookupEnv, "KEY"` composition-root form. Both forms remain local to the
declared files; Rhino neither discovers directories nor assumes a framework.

`toolchains.entries` declares a probe executable/argv, optional output parser
and exact version, required status, optional positive `timeout-seconds`, and
platform-specific provision executable/argv. `toolchain validate` runs probes
only. `toolchain provision` plans platform vectors by default, requires
`--apply` to run them, and stops at the first failure without reporting tool
output.

## Lifecycle gates

The closed surfaces are `pre-commit`, `commit-msg`, `pre-push`,
`pull-request`, `main`, `scheduled`, and `manual`; `ci` is not a v0.4 surface.
Each gate has one lowercase-kebab-case semantic ID and declares direct
`run-on` membership. A pull request explicitly composes the local union with
`exact` or `at-least`; an at-least-only ID has its own non-empty `reason`.

Inputs are named as `files`, `commit-message`, `commit-range`, or
`repository-state`, then bound by a generic source such as `git-index`,
`hook-message-file`, `push-updates`, `explicit-range`, or `checkout`. Commands
are an executable, typed argv mappings, and explicit environment projections;
they are never a shell string. Use `gate list` to inspect the seven surfaces
and `gate validate` to check declaration, composition, binding, and projection
without starting a child. An omitted `gates` group makes `gate list`,
`gate validate`, and `gate run` exit `2`; `gates: {}` declares no gate and
reports clean.

A `files` input binds `git-index` for a local mutation or `explicit-range` with
`range: explicit` for a pull-request replay. The latter resolves only changed
repository-relative paths from the immutable `base..head` pair; it never turns
the complete checkout into a formatter argument list. `checkout` remains for a
check that intentionally reads every visible path or repository-state.

The same `commit-message` input can bind `hook-message-file` at `commit-msg`
and `explicit-range` with `range: explicit` at `pull-request`. For the latter,
Rhino reads the non-merge commit-message text in the immutable `base..head`
range after it has validated both commit IDs. The gate keeps its one semantic
ID, command, and typed `message.text` projection; only its declared source
changes by lifecycle surface. At `commit-msg`, Rhino asks Git for the current
worktree's canonical `COMMIT_EDITMSG` path and reads only that file. This also
supports linked worktrees without widening the repository file reader to an
arbitrary external path.

A `push-updates` range also declares its repository comparison `fallback`. A
normal update uses the remote object as base. A new ref resolves that declared
ref to an immutable base, while a deleted ref skips the range gate because it
has no head to inspect. Rhino refuses an absent, invalid, or unresolved
fallback rather than inventing a range.

A mutation pairs `local: apply-index` with `ci: verify-clean`. Rhino refuses
to run it until its index/disposable-replay boundary is available, rather than
falling back to the mutable working tree.

The index snapshot retains an unselected tracked symlink as opaque link data;
it never resolves that target. A selected symlink is refused before the
declared mutator starts, so a formatter cannot follow it outside the snapshot.

A mutation child runs from its disposable boundary, but a relative `PATH`
segment keeps the meaning it had for the calling repository. Rhino resolves
that segment from the original repository root before the child starts; an
absolute segment remains unchanged. A consumer therefore declares a portable
tool name without embedding its machine-local installation path.

## Modelines

Rhino's source example uses a relative modeline because its generated schema is
checked in beside it:

```yaml
# yaml-language-server: $schema=./v2.schema.json
```

A consumer pins an immutable release URL instead. The consumer fixture is the
canonical shape; it never makes the Rhino runtime perform a network request.

## Verification

Generate the artifact after changing the model, then verify it before review:

```sh
cargo xtask schema
cargo xtask schema --check
rhino repo-config validate
```

## Stable predecessor handling

Stable accepts only grouped v2 configuration. It rejects predecessor schemas
and does not provide `rhino repo-config migrate`; create the grouped document,
pin its immutable modeline, and validate local bytes offline.

Follow [How to migrate from v0.3 to v0.4](../how-to/migrate-to-v0-4.md) for
the owner-by-owner transition and no-alias boundary.
