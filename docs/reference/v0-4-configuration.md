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

`policies.markdown` opts into frontmatter, internal-link, Mermaid, and
README-index validation. `policies.governance` owns vendor vocabulary, layer
structure, traceability declarations, word-budget surfaces, and directory-map
trees. `policies.conventions` owns configured license paths, identifiers,
digests, and exact exclusions. Each group refuses an undeclared or invalid
sub-policy instead of inheriting a repository convention.

`governance word-budget validate` and `governance directory-map validate` read
only `policies.governance.word-budget` and
`policies.governance.directory-map`, respectively. Their `count`, ordered
`surfaces`, `trees`, findings, and optional `scan.exclude-directories` retain
the established leaf contracts; grouped v2 changes configuration ownership,
not the policy a repository declared.

`policies.plans` is a closed owner boundary. It carries no v0.4 command in this
release, so an empty group is not a request to run a legacy plan validator.

## Canonical adapter profiles

`harness` declares the portable requirements and exactly three opaque profiles.
Each profile owns one non-overlapping adapter root and explicitly lists the
capabilities, grants, denials, constraints, routes, and identities it can
represent. Canonical source remains `AGENTS.md` and `.agents/{agents,skills}`;
the generated roots contain only routes, a catalog, and provenance.

```yaml
harness:
  requirements:
    capabilities: [read]
  profiles:
    - id: alpha
      root: adapters/alpha
      supports: { capabilities: [read] }
    - id: beta
      root: adapters/beta
      supports: { capabilities: [read] }
    - id: gamma
      root: adapters/gamma
      supports: { capabilities: [read] }
```

`harness adapters validate` is read-only. `harness adapters generate` first
validates the entire projection, then replaces only the declared roots as one
recoverable transaction. A missing profile capability or another unsupported
requirement refuses before any write.

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
without starting a child.

The same `commit-message` input can bind `hook-message-file` at `commit-msg`
and `explicit-range` with `range: explicit` at `pull-request`. For the latter,
Rhino reads the non-merge commit-message text in the immutable `base..head`
range after it has validated both commit IDs. The gate keeps its one semantic
ID, command, and typed `message.text` projection; only its declared source
changes by lifecycle surface.

A `push-updates` range also declares its repository comparison `fallback`. A
normal update uses the remote object as base. A new ref resolves that declared
ref to an immutable base, while a deleted ref skips the range gate because it
has no head to inspect. Rhino refuses an absent, invalid, or unresolved
fallback rather than inventing a range.

A mutation pairs `local: apply-index` with `ci: verify-clean`. Rhino refuses
to run it until its index/disposable-replay boundary is available, rather than
falling back to the mutable working tree.

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

## RC Migration

During the release candidate only, a legacy configuration may be read by
`rhino repo-config migrate`. The command prints a reviewed plan: write grouped
v2 with the immutable modeline, compare effective policy, and validate local
bytes offline. It does not rewrite configuration. A grouped v2 document is
refused because it needs no migration. The RC-only legacy reader is deleted
before stable.

Follow [How to migrate from v0.3 to v0.4](../how-to/migrate-to-v0-4.md) for
the owner-by-owner transition and no-alias boundary.
