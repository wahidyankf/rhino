# Why RHINO holds no defaults

RHINO ships no default for any value it enforces. Not a word limit, not a
palette, not a label length, not a path. A repository that declares nothing
gets a configuration error and exit `2`.

This is the single decision the rest of the tool follows from, and it is worth
explaining, because "sensible defaults" is normally a virtue.

## Where the rule came from

RHINO's validators were extracted from a working tool inside one repository.
That tool knew things: which directories held governance documents, how long an
instruction file was allowed to be, which harness directories to reconcile. It
knew them because they were true of the repository it lived in.

Every one of those facts was invisible until a second repository tried to use
it. Then each one became a wrong answer delivered confidently. A default is not
neutral — it is one repository's policy, applied to a repository that never
agreed to it, in a tool whose entire job is to report policy violations.

The failure mode is specific and bad: a repository adopts the tool, the tool
finds nothing, and everyone concludes the repository is clean. It was never
checked, because the surface the default named does not exist there.

## What the rule buys

**A finding always means the repository broke a rule it wrote down.** There is
no third category of "RHINO thinks you should". Exit `1` is never a matter of
taste, which is what makes it safe to put in front of a push.

**Configuration is the specification.** `repo-config.yml` is the whole policy
in one readable file. A maintainer asking "what does this repository enforce?"
reads it, rather than reading the tool's source and subtracting the overrides.

**Adding a fourth repository is configuration, not code.** Adapter generation
names no harness. `harness` declares three opaque profiles, and each profile's
paths, format, and fields come from the configuration. A repository with no
coding harness omits the group.

## What the rule costs

Adoption is slower. There is no `rhino init` that guesses. The first run in a
repository without `repo-config.yml` is a configuration error rather than a
result, and so is a run of any validator whose policy is not declared. That is
the intended trade: the alternative is a first run that passes for reasons
nobody chose.

The configuration also has to name every value it enforces. Only `schema` is
required, and every group is optional, but an omitted group is never a quiet
pass: the validator that needs it exits `2` and names the missing policy. The
difference between _this repository declares no Mermaid policy_ and _I forgot to
configure Mermaid_ stays visible, because neither is ever reported as clean.

## Where the rule bends, and where it does not

It bends for **optional structure**: a profile's `instruction-adapter`,
`skill-adapter`, and `agent-adapter` may each be absent, because a harness may
genuinely have no file that merely imports the instruction, or no adapter of
that kind to generate. Absent means "there is nothing to generate", and every
other rule still applies in full.

It does not bend for **values**. There is no fallback limit, no built-in
palette, no assumed directory. The fixed names inside RHINO — `README.md` for
the file a directory map lives in, and the canonical sources adapter generation
reads, `AGENTS.md`, `.agents/agents/*.md`, and `.agents/skills/*/SKILL.md` —
are the tool's own behaviour, the input its generator is defined over, not a
consumer's layout.

## The test that keeps it honest

RHINO validates RHINO. Its own `repo-config.yml` is the second repository this
schema ever described, and writing it immediately found a rule the schema had
inherited without noticing: `required-mcp` was mandatory even for a repository
with no harnesses to reconcile it against, so RHINO could not describe itself.

Pointing it at a third repository found the same rule leaking the other way. A
repository with three harnesses and no capability server at all still could not
be described, because a non-empty roster forced `required-mcp` — a claim about
how one repository is arranged, wearing the clothes of a schema rule.

RHINO has since become that third shape itself. Its configuration declares
three profiles — `claude`, `codex`, and `opencode` — over ten canonical skills,
four canonical agents, and no capability server at all. Only `claude` declares
a skill adapter and an instruction adapter; `codex` and `opencode` declare
agent adapters alone. Every one of those is a shape the schema had to permit
without preferring.

That is the value of the rule stated as a practice. A tool that holds no
defaults will still smuggle assumptions in through required keys, and the only
way to find them is to point it at a repository that is not the one it grew up
in.

## Related

- [Configuration](../reference/configuration.md)
- [Why RHINO exists](./why-rhino-exists.md)
- [What a finding means](./what-a-finding-means.md)
