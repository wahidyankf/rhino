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

Replace the `harness-parity` section with this. Each canonical root arrives with
the **route** its adapters must carry in place of the canonical body, because an
adapter is a route rather than a copy.

```sh
harness-parity:
  canonical:
    instruction: AGENTS.md
    instruction-adapter: CLAUDE.md
    skills-root: skills
    skill-route: "Read {path} completely, then follow it."
    agents-root: agents
    agent-route: "Read {path} completely and follow it as authoritative."
  harnesses:
    - name: claude
      agent-adapter:
        path: ".claude/agents/{name}.md"
        format: front-matter
        route-field: body
        identity: { name: name, description: description }
        translations:
          - { when: always, field: tools, members: [Read] }
          - {
              when: requires,
              capability: repository-read,
              field: tools,
              members: [Glob, Grep],
            }
          - {
              when: denies,
              capability: repository-write,
              field: tools,
              absent-members: [Write, Edit],
            }
      skill-adapter:
        path: ".claude/commands/{name}.md"
        format: front-matter
        route-field: body
        identity: { description: description }
        closed: true
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

The `translations` block is where the canon meets this harness's own vocabulary.
The canon says `repository-read`; this harness spells that `Glob` and `Grep` in
a `tools` list. RHINO knows neither word — it knows that when the canon requires
`repository-read`, this harness's adapter must list those two.

Every path and name there is yours. `claude` is a string in your configuration,
and a second harness is another entry rather than another release of the tool.

```console
$ rhino harness parity validate
[harness-parity] checked 1 harness, 3 findings
[harness-parity] canon 1 harnesses, 1 skills, 1 agents, 0 reconciled capability declarations
[harness-parity] digest d5029c0340f75589619f2fac234667d145940493f716eb1ac1917bc1d1e529f5
[harness-parity] .claude/agents/reviewer.md: missing-agent-adapter: `claude` has no adapter for the canonical agent `reviewer`
[harness-parity] .claude/commands/summarise.md: missing-skill-adapter: `claude` declares skill wrappers but has none for `summarise`
[harness-parity] .mcp.json: divergent-capability: `claude` declares no capability file
```

Three findings, and each names the path that would fix it.

The reconciled count reads `0`: it counts capability declarations RHINO actually
found, parsed, and matched. A constant there would claim a comparison this run
never made.

## Step 4 — write the adapters

The agent adapter carries the route and this harness's own permissions. It does
**not** repeat the canonical prompt, and it does not repeat the canonical
capability names either:

```sh
cat > .claude/agents/reviewer.md <<'END'
---
name: reviewer
description: Read a change and report what it breaks.
tools: Read, Glob, Grep
---

Read agents/reviewer.md completely and follow it as authoritative.
END
```

The skill wrapper is the same idea with less in it — the description a reader
chooses by, the route, and nothing else, because its declaration is `closed`:

```sh
cat > .claude/commands/summarise.md <<'END'
---
description: Condense a long document into its load-bearing claims.
---

Read skills/summarise/SKILL.md completely, then follow it.
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

## Step 5 — reword a route, and watch it fail

Append `, and be brief.` to the adapter's route sentence — a change anyone might
make in passing, to one harness only, and never notice again.

```console
$ rhino harness parity validate
[harness-parity] checked 1 harness, 1 finding
[harness-parity] canon 1 harnesses, 1 skills, 1 agents, 1 reconciled capability declarations
[harness-parity] digest d5029c0340f75589619f2fac234667d145940493f716eb1ac1917bc1d1e529f5
[harness-parity] .claude/agents/reviewer.md: agent-prompt-divergence: `body` is not the canonical route to `reviewer`
```

## Step 6 — grant what the canon denies, and watch it fail differently

Restore the route, then add `Write` to the adapter's `tools`:

```console
$ rhino harness parity validate
[harness-parity] checked 1 harness, 1 finding
[harness-parity] canon 1 harnesses, 1 skills, 1 agents, 1 reconciled capability declarations
[harness-parity] digest d5029c0340f75589619f2fac234667d145940493f716eb1ac1917bc1d1e529f5
[harness-parity] .claude/agents/reviewer.md: agent-semantic-divergence: `tools` grants `Write`, which the canon denies
```

The canon never mentions `Write`. It says the agent may not write to the
repository, and the translation is what turns that into a claim about this
harness's tool list. A different harness spelling the same grant a different way
is caught by its own translation, not by this one.

Note what is _not_ a finding: an adapter granting **more** than the canon
requires. The contract is non-weakening, not equality — the harness's own
defaults are not the canon's business.
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
[harness-parity] checked 1 harness, no findings
[harness-parity] canon 1 harnesses, 1 skills, 1 agents, 1 reconciled capability declarations
[harness-parity] digest f02c8fff513b040b71984741c0c12d57ae130856eacfaf6f06421f61591b7520
```

**No finding, and a different digest.** That is the route model working as
intended: rewriting the canonical prompt is not drift, because no adapter
carries a copy of it to drift from. Every harness already reads the new wording
the moment the file changes.

The digest is what noticed. Record it somewhere and you have a single value that
changes whenever any canonical file changes — useful in a review, where "the
contract moved" is a different question from "an adapter drifted", and only one
of them can be a finding.

Restore the canonical prompt to get the earlier digest back.

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
