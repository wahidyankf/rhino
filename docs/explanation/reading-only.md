# Why RHINO only reads

RHINO opens files and prints. It writes nothing, spawns nothing, and connects
to nothing. These are not incidental properties of the current implementation —
they are enforced as tests, and a change that broke one would fail the build.

## The four boundaries

**It writes nothing to the repository it inspects.** Not a cache, not a report,
not a lock file. A hygiene tool that writes into the tree it is judging can
change the answer it is about to give, and can turn a read-only checkout into a
failure.

**It spawns no child process.** No `git`, no formatter, no shell. Everything
RHINO reports, it derived from bytes it read itself.

**It opens no socket, including loopback.** There is no telemetry, no version
check, no remote policy fetch. This is stricter than the layer rule it sits
under, which permits an integration test to own a loopback socket — that
permission is about test plumbing, and this is a claim about the product.

**It contains no `unsafe` code.** `#![forbid(unsafe_code)]`, checked.

## Why they are tests rather than prose

A README that says "RHINO does not reach the network" is a promise. A test that
fails when a dependency capable of reaching the network appears in the lock
file is a mechanism.

The difference showed up while writing those tests. The first version of the
network check searched the dependency list for known networking crates and
passed. It also passed when the crate it searched for was absent for a reason
that had nothing to do with networking — because a search that finds nothing
looks exactly like a search that is broken. The test now includes a **positive
control**: the same search, in the same form, must find a crate that is
certainly there. Any test whose success condition is "found nothing" needs one.

## What it cannot reach

Two rules keep a run inside the repository it was pointed at.

**A path leaving the root is refused.** A `--root`, `--file`, or `--directory`
containing `..` is an invocation error, not a path to resolve. So is an
absolute path: a selection is relative to the repository root, or the answer
would depend on where the command ran.

**A symbolic link is not followed.** The walk skips links regardless of what
the scan exclusions say, because following one can leave the repository
entirely — and a validator that walked out of the tree would report findings
about files the repository does not contain.

## What this buys you

You can run RHINO against a repository you do not trust, in a sandbox with no
network, on a read-only mount, in a hook that runs before every push, without
thinking about any of it. The cost of running it is a directory walk.

It also means the answer is reproducible. Two runs over one repository produce
byte-identical output — findings and scanned paths are sorted — so a diff
between two runs is a diff about the repository, never about ordering, timing,
or something the tool fetched.

## Related

- [Why RHINO exists](./why-rhino-exists.md)
- [Exit codes](../reference/exit-codes.md)
