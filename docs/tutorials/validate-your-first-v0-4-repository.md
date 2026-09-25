# Validate your first v0.4 repository

This tutorial creates a minimal repository policy, validates it, introduces one
broken internal link, and reads the result without relying on a default.

Before you start, put `rhino` on your `PATH`. [Install a pinned
release](../how-to/install-a-pinned-release.md) explains the download and the
build from source.

## 1. Create the files

Create an empty checkout with a document that will contain one local link:

```sh
mkdir rhino-example
cd rhino-example
mkdir docs
printf '# Notes\n\n[Guide](guide.md)\n' > docs/README.md
```

## 2. Declare grouped policy

Create `repo-config.yml`. The declaration below opts into only local Markdown
link validation; every other policy remains absent.

```yaml
schema: rhino/repo-config/v2
policies:
  markdown:
    internal-link:
      exclude-sources: []
```

Validate the configuration before asking Rhino to inspect the tree:

```console
$ rhino repo-config validate
```

The command exits `0` when the grouped document is usable. It does not create a
policy for an omitted group.

## 3. Observe a declared violation

The link points to a file that does not exist, so the declared validator reports
a finding and exits `1`:

```console
$ rhino md internal-link validate
```

RHINO reports the repository-relative source and target. It never follows a
path outside the selected repository root. A target reached through a symbolic
link is reported as outside the repository, never read.

## 4. Fix the repository

Create the target and run the same leaf again:

```sh
printf '# Guide\n' > docs/guide.md
rhino md internal-link validate
```

The second run exits `0`. A clean result means the declared internal-link policy
ran and found no violations; it does not claim that another, undeclared policy
also passed.

## 5. Add policy deliberately

Add another group only when the repository can name its scope and owner. For
example, `policies.markdown` can opt into frontmatter, Mermaid, and README-index
validation, while `gates` can declare a lifecycle registry with typed inputs.
The [grouped configuration reference](../reference/v0-4-configuration.md)
defines those contracts.

## Next steps

- [How to wire RHINO into your gates](../how-to/wire-rhino-into-your-gates.md)
- [How to migrate from v0.3 to v0.4](../how-to/migrate-to-v0-4.md)
- [Command line](../reference/cli.md)
