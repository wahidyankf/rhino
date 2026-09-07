# Reconcile your first harness contract

About fifteen minutes, and it assumes you have finished [validate your first
repository](validate-your-first-repository.md). By the end you will have one
canonical instruction body, one canonical skill, one canonical agent, a harness
that reaches all three through adapters, and two kinds of drift you introduced
on purpose and watched RHINO tell apart.

Harness parity is the least obvious of the validators, so we build the
repository one layer at a time and run the command after every layer.

## The problem this solves

Most repositories that work with more than one coding harness end up with the
same instructions written down more than once — `AGENTS.md` for one tool,
`CLAUDE.md` for another, a per-harness copy of every agent prompt, a per-harness
copy of every skill. Copies drift. The drift is silent, and the first sign of it
is two harnesses behaving differently on the same repository.

RHINO's answer is that a repository declares **one canonical body** for each of
those things and a **roster of harnesses** that must reach it. Everything a
harness holds is an adapter: a route to the canon, not a second copy of it.

## Step 1 — the canonical instruction, and nothing else

```sh
mkdir rhino-harness && cd rhino-harness
mkdir -p skills/summarise agents .claude/agents .claude/commands

cat > AGENTS.md <<'END'
# Contributing

One instruction body, read by every harness.
END

cat > CLAUDE.md <<'END'
@AGENTS.md
END
```

`CLAUDE.md` contains the import and nothing else. That is a rule rather than a
convention: an adapter exists to route to the instruction, so any prose in it is
a second instruction source wearing the adapter's name.

Declare that much, with an empty roster:

```sh
cat > repo-config.yml <<'END'
# schema: rhino/repo-config/v1

governance-word-budget:
  surfaces:
    - glob: "AGENTS.md"
      fail: 400

governance-directory-map:
  trees: []

md-internal-link:
  exclude-sources: []

md-mermaid:
  node-label-graphemes: 32
  edge-label-graphemes: 24
  fill-colors: ["#0173B2"]
  edge-colors: ["#000000"]
  text-colors: ["#000000"]

harness-parity:
  canonical:
    instruction: AGENTS.md
    instruction-adapter: CLAUDE.md
  harnesses: []
  prohibited-instruction-sources:
    - "**/GEMINI.md"
  capabilities: []
  constraints: []

scan:
  exclude-directories:
    - .git
END
```

```console
$ rhino harness parity validate
[harness-parity] checked 0 harnesses, no findings
[harness-parity] canon 0 harnesses, 0 skills, 0 agents, 0 reconciled capability declarations
[harness-parity] digest ec7a3f0b950d5fa09d7e727df47afff1953cc364b77b11e1c1b68b41400e3f77
```

Zero harnesses is a legal, permanent answer, and RHINO says so out loud rather
than skipping the command. The instruction and its adapter were still checked —
the roster governs harnesses, not the canon.

Keep the digest line in view. It is a hash over the canonical files only.

## Step 2 — write the canon

A skill lives in a directory named after itself and carries a `SKILL.md`:

```sh
cat > skills/summarise/SKILL.md <<'END'
---
name: summarise
description: Condense a long document into its load-bearing claims.
---

Read the document, then write one paragraph per claim it actually makes.
END
```

An agent is one file named after itself, and it declares what it may do:

```sh
cat > agents/reviewer.md <<'END'
---
name: reviewer
description: Read a change and report what it breaks.
capabilities:
  - repository-read
denied:
  - repository-write
constraints:
  - inline-result-only
---

Read the change. Report only what you can point at in the diff.
END
```

Those three lists are not free text. Every name in them has to appear in the
vocabulary your configuration declares, so that a typo fails instead of quietly
granting an agent nothing.

## Step 3 — declare a harness

Replace the `harness-parity` section with this. Note that the canonical roots,
the capability vocabulary, and the required server all arrive together: they
exist to be reconciled against a harness, so a roster that is not empty requires
them.

```sh
harness-parity:
  canonical:
    instruction: AGENTS.md
    instruction-adapter: CLAUDE.md
    skills-root: skills
    agents-root: agents
  harnesses:
    - name: claude
      agent-dir: .claude/agents
      agent-extension: ".md"
      command-dir: .claude/commands
      capability:
        file: .mcp.json
        format: json
  required-mcp:
    name: toolserver
    command: toolrunner
    args: ["serve"]
  prohibited-instruction-sources:
    - "**/GEMINI.md"
  capabilities:
    - repository-read
    - repository-write
  constraints:
    - inline-result-only
```

Every path there is yours. RHINO knows no harness by name; `claude` is a string
in your configuration, and a second harness is another entry rather than another
release of the tool.

```console
$ rhino harness parity validate
[harness-parity] checked 1 harness, 3 findings
[harness-parity] canon 1 harnesses, 1 skills, 1 agents, 1 reconciled capability declarations
[harness-parity] digest d5029c0340f75589619f2fac234667d145940493f716eb1ac1917bc1d1e529f5
[harness-parity] .claude/agents/reviewer.md: missing-agent-adapter: `claude` has no adapter for the canonical agent `reviewer`
[harness-parity] .claude/commands/summarise.md: missing-skill-adapter: `claude` declares a command directory but has no wrapper for `summarise`
[harness-parity] .mcp.json: divergent-capability: `claude` declares no capability file
```

Three findings, and each names the path that would fix it. The digest changed,
because the canon gained two files.

The reconciled count still reads `0`: it counts capability declarations RHINO
actually found, parsed, and matched. A constant there would claim a comparison
this run never made.

## Step 4 — write the adapters

The agent adapter carries the same declaration and the same prompt:

```sh
cat > .claude/agents/reviewer.md <<'END'
---
name: reviewer
description: Read a change and report what it breaks.
capabilities:
  - repository-read
denied:
  - repository-write
constraints:
  - inline-result-only
---

Read the change. Report only what you can point at in the diff.
END
```

The skill wrapper is a route. It mirrors the description a reader chooses by,
and its body is the import and nothing else:

```sh
cat > .claude/commands/summarise.md <<'END'
---
name: summarise
description: Condense a long document into its load-bearing claims.
---

@skills/summarise/SKILL.md
END
```

The capability file is the harness's own format, read as the harness writes it:

```sh
cat > .mcp.json <<'END'
{
  "mcpServers": {
    "toolserver": {
      "command": "toolrunner",
      "args": ["serve"]
    }
  }
}
END
```

RHINO looks for the server by the name you declared, wherever the vendor nests
it. It never learns the key path, because that path is the vendor's syntax and
pinning it would put a harness back inside the binary.

```console
$ rhino harness parity validate
[harness-parity] checked 1 harness, no findings
[harness-parity] canon 1 harnesses, 1 skills, 1 agents, 1 reconciled capability declarations
[harness-parity] digest d5029c0340f75589619f2fac234667d145940493f716eb1ac1917bc1d1e529f5
```

The digest is unchanged from step 3. Adapters are not canon, so writing three of
them moved nothing the digest covers.

## Step 5 — reword an adapter, and watch it fail

```console
$ rhino harness parity validate
[harness-parity] checked 1 harness, 1 finding
[harness-parity] canon 1 harnesses, 1 skills, 1 agents, 1 reconciled capability declarations
[harness-parity] digest d5029c0340f75589619f2fac234667d145940493f716eb1ac1917bc1d1e529f5
[harness-parity] .claude/agents/reviewer.md: agent-prompt-divergence: the adapter's prompt is not the canonical one
```

That is after appending `, and be brief.` to the adapter's prompt — a change
anyone might make in passing, to one harness only, and never notice again.

## Step 6 — remove a denial, and watch it fail differently

Restore the wording, then delete the `denied:` block from
`.claude/agents/reviewer.md`:

```console
$ rhino harness parity validate
[harness-parity] checked 1 harness, 1 finding
[harness-parity] canon 1 harnesses, 1 skills, 1 agents, 1 reconciled capability declarations
[harness-parity] digest d5029c0340f75589619f2fac234667d145940493f716eb1ac1917bc1d1e529f5
[harness-parity] .claude/agents/reviewer.md: agent-semantic-divergence: the adapter grants, denies, or constrains differently from the canon
```

A different finding kind, and the reason to care about the distinction. Step 5
was an adapter that says the same thing differently. This is an adapter that
**is a different agent** — under `claude` it may now write to the repository the
canon forbade it to touch. Reporting that as prose drift would understate it,
and a gate that treated the two alike would teach you to ignore both.

Put the `denied:` block back before continuing.

## Step 7 — edit the canon and watch the digest move

Change the canonical prompt in `agents/reviewer.md`, leaving the adapter alone:

```console
$ rhino harness parity validate
[harness-parity] checked 1 harness, 1 finding
[harness-parity] canon 1 harnesses, 1 skills, 1 agents, 1 reconciled capability declarations
[harness-parity] digest aee7c86b87d9c69d67f85a115550a542459c6e1c6c58bca48874af4ca3253418
[harness-parity] .claude/agents/reviewer.md: agent-prompt-divergence: the adapter's prompt is not the canonical one
```

The digest moved this time, because the canon did. Record it somewhere and you
have a single value that changes whenever any canonical file changes — useful in
a review, where "the contract moved" is a different question from "an adapter
drifted".

Restore the canonical prompt to get back to a clean run.

## Step 8 — ask about one harness

```console
$ rhino harness parity validate --harness claude
[harness-parity] checked 1 harness, no findings
[harness-parity] canon 1 harnesses, 1 skills, 1 agents, 1 reconciled capability declarations
[harness-parity] digest d5029c0340f75589619f2fac234667d145940493f716eb1ac1917bc1d1e529f5
```

Narrowing asks a smaller question; it never gives a quieter answer to the same
one. Everything selected is still reported in full.

A name you never declared is a broken invocation rather than an empty result:

```console
$ rhino harness parity validate --harness codex
rhino: `codex` is not a harness this repository declares
```

Exit `2`, and stdout is empty. Nothing was checked, so there is nothing to
report — which is exactly what a gate needs to tell apart from a clean run.

## Step 9 — take the same answer as JSON

```console
$ rhino harness parity validate --output json
{"schemaVersion":1,"command":"harness-parity","exitCode":0,"subject":"harness","inspected":1,"scanned":[],"notes":["canon 1 harnesses, 1 skills, 1 agents, 1 reconciled capability declarations","digest d5029c0340f75589619f2fac234667d145940493f716eb1ac1917bc1d1e529f5"],"violations":[]}
```

Same run, same exit code, one line. See [how to consume the JSON
output](../how-to/consume-the-json-output.md).

## What you have

One instruction body, one skill, one agent, and a harness that reaches all three
through adapters that cannot silently diverge from them. Adding a second harness
is one entry in `repo-config.yml` and the adapters it needs — no change to
RHINO, and no second copy of anything canonical.

## Next steps

- [Findings](../reference/findings.md) for every harness-parity finding kind
- [Configuration](../reference/configuration.md) for the whole `harness-parity` section
- [What a finding means](../explanation/what-a-finding-means.md)
