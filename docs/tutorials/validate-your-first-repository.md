# Validate your first repository

About ten minutes. By the end you will have a repository RHINO checks, a
configuration you wrote yourself, and a finding you caused on purpose and then
fixed.

We will build a small repository from nothing so that nothing is hidden.

## Before you start

You need `rhino` on PATH. Check:

```console
$ rhino version
v0.1.0
```

If that fails, see [how to install a pinned
release](../how-to/install-a-pinned-release.md).

## Step 1 — make a repository with something to check

```sh
mkdir rhino-tutorial && cd rhino-tutorial
mkdir -p guides
```

Give it an instruction file and a guide:

```sh
cat > AGENTS.md <<'END'
# Instructions

Keep documents short and links live.
END

cat > guides/README.md <<'END'
# Guides

## Directory Map

- [Colours](colours.md) — the palette this repository draws with.
END

cat > guides/colours.md <<'END'
# Colours

See the [instructions](../AGENTS.md).
END
```

## Step 2 — run RHINO before configuring anything

```console
$ rhino governance word-budget validate
[word-budget] repo-config.yml: no configuration file at line 1: RHINO holds no defaults, so every value it enforces has to be declared here
```

Exit code `2`. This is the point of the tool and not a mistake: RHINO ships no
limits, no palettes, and no paths, so there is nothing for it to check until
you say what your repository enforces. A tool that guessed here would report a
clean run on a repository it never really looked at.

## Step 3 — declare what this repository enforces

Every section is required, even where you have nothing to put in it.

```sh
cat > repo-config.yml <<'END'
# schema: rhino/repo-config/v1

governance-word-budget:
  count: letters-and-digits
  surfaces:
    - glob: "AGENTS.md"
      fail: 20

governance-directory-map:
  trees:
    - path: guides

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

Check the configuration itself first. This is the command to reach for whenever
anything else exits `2`:

```console
$ rhino repo-config validate
[repo-config] checked 1 configuration file, no findings
```

## Step 4 — run the validators

```console
$ rhino governance word-budget validate
[word-budget] checked 1 file, no findings
[word-budget] scanned AGENTS.md

$ rhino governance directory-map validate
[directory-map] checked 1 directory, no findings

$ rhino md internal-link validate
[internal-link] checked 2 links, no findings

$ rhino md mermaid validate
[mermaid] checked 0 diagrams, no findings
```

Look at the word-budget output again. It lists the file it read. That listing
is the difference between _checked and clean_ and _checked nothing_ — a glob
that matches nothing also reports no findings and also exits `0`, and only the
list of scanned paths tells you which happened.

Notice too that zero diagrams is reported explicitly rather than skipped. A
repository with no diagrams has a correct and permanent answer of zero.

## Step 5 — break something on purpose

A gate nobody has seen fail is a gate nobody knows works. Add a link that goes
nowhere:

```console
$ printf '\nAlso see [the roadmap](roadmap.md).\n' >> guides/colours.md
$ rhino md internal-link validate
[internal-link] checked 3 links, 1 finding
[internal-link] guides/colours.md:5: `roadmap.md` does not exist
```

Exit code `1`. Read the shape of that line, because every finding from every
validator has it: the category in brackets, the path that caused it, the line,
then what is wrong.

## Step 6 — fix it, and see the count change

```console
$ printf '# Roadmap\n' > guides/roadmap.md
$ rhino md internal-link validate
[internal-link] checked 3 links, no findings
```

Now the directory map is out of date — `guides/` has a new file that its README
does not list:

```console
$ rhino governance directory-map validate
[directory-map] checked 1 directory, 1 finding
[directory-map] guides/README.md: missing map entry for `guides/roadmap.md`: every direct sibling appears exactly once
```

This is the decay RHINO exists to catch. The map was true when it was written
and became false when someone added a file. Nothing announced it.

```console
$ printf -- '- [Roadmap](roadmap.md) — where this is going.\n' >> guides/README.md
$ rhino governance directory-map validate
[directory-map] checked 1 directory, no findings
```

## Step 7 — see the two failure modes apart

Findings and broken invocations both exit non-zero, and they mean opposite
things:

```console
$ rhino md internal-link validate --harness claude
rhino: `--harness` is not accepted by `md internal-link validate`
```

Exit `2`. Nothing was checked. Compare with the exit `1` in step 5, where
everything was checked and the repository was in breach.

Wire a gate that treats these the same and it will tell a maintainer to fix a
document when the real problem is a typo in a flag.

## What you have

A repository whose policy is written down in one file, six commands that check
it, and three exit codes that mean three different things. Nothing RHINO
enforced came from RHINO.

## Next steps

- [How to wire RHINO into your gates](../how-to/wire-rhino-into-your-gates.md)
- [Configuration](../reference/configuration.md) for every key
- [Why RHINO holds no defaults](../explanation/holding-no-defaults.md)
