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

**Adding a fourth repository is configuration, not code.** The roster loop
inside RHINO names no harness. A repository with three coding harnesses and a
repository with none are the same code path with different declarations.

## What the rule costs

Adoption is slower. There is no `rhino init` that guesses, and the first run in
a new repository is a configuration error rather than a result. That is the
intended trade: the alternative is a first run that passes for reasons nobody
chose.

The configuration is also long. Every section is required, including sections a
repository has nothing in. `harnesses: []` is a required key with a legal empty
value, because the difference between _this repository has no coding harnesses_
and _I forgot to configure harnesses_ has to stay visible. A key you may omit
is a key you may omit by accident.

## Where the rule bends, and where it does not

It bends for **optional structure**: `canonical.instruction-adapter` may be
absent, because a repository may genuinely have no file that merely imports the
instruction. Absent means "there is nothing to route", and every other rule
still applies in full.

It bends for **keys that follow the roster**: `skills-root`, `agents-root`, and
`required-mcp` are required when harnesses are declared and refused when they
are not. That is not a default — it is the schema refusing to accept a
declaration that could not mean anything.

It does not bend for **values**. There is no fallback limit, no built-in
palette, no assumed directory. The two string constants inside RHINO —
`README.md` for the file a directory map lives in, `SKILL.md` for the file a
skill declares itself in — are the tool's own behaviour, not a consumer's
layout, and they are the only ones.

## The test that keeps it honest

RHINO validates RHINO. Its own `repo-config.yml` is the second repository this
schema ever described, and writing it immediately found a rule the schema had
inherited without noticing: `required-mcp` was mandatory even for a repository
with no harnesses to reconcile it against, so RHINO could not describe itself.

That is the value of the rule stated as a practice. A tool that holds no
defaults will still smuggle assumptions in through required keys, and the only
way to find them is to point it at a repository that is not the one it grew up
in.

## Related

- [Configuration](../reference/configuration.md)
- [Why RHINO exists](./why-rhino-exists.md)
- [What a finding means](./what-a-finding-means.md)
