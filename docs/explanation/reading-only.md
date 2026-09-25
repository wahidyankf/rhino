# Why RHINO validation only reads

RHINO tree validators open files and print. They write nothing, spawn nothing,
and connect to nothing. Explicit operations are separate: `gate run` starts
only a declared child argv, toolchain commands start declared typed argv without
a shell or reported output, and adapter generation replaces only declared
adapter families and exact instruction-adapter files after planning every output
in memory. These are enforced as
tests, not incidental properties of the current implementation.

## The four boundaries

**A validator writes nothing to the repository it inspects.** Not a cache, not
a report, not a lock file. A hygiene tool that writes into the tree it is
judging can change the answer it is about to give, and can turn a read-only
checkout into a failure. An explicit adapter transaction has a different port:
it rejects paths outside declared adapter families or exact files and is never reachable from a
validator.

**A tree validator spawns no child process.** No `git`, no formatter, no shell.
Everything it reports, it derived from bytes it read itself. `gate run` and the
toolchain commands are separate declared operations: each starts typed argv at
a narrow port and never interprets a shell command. Toolchain probes and
provision never report child output; provision stops at its first failure.

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

**A path leaving the root is refused.** A `--file`, `--directory`, or `--dir`
value whose `..` segments climb out of the root is an invocation error, not a
path to resolve. So is an absolute one: a selection is relative to the
repository root, or the answer would depend on where the command ran. `--root`
itself is not a selection; it names the repository, so any directory is
accepted there.

**A symbolic link is not followed.** The walk skips links regardless of what
the scan exclusions say, because following one can leave the repository
entirely — and a validator that walked out of the tree would report findings
about files the repository does not contain. The same holds for any path that
passes through a link, at any component. A `--file` or `--directory` selection
behind one is refused with `rhino.path.escapes-root`, an internal link whose
target lies behind one is reported as outside the repository, and a declared
file behind one is refused as unreadable. None of them is read.

## What this buys you

You can run a RHINO validator against a repository you do not trust, in a
sandbox with no network, on a read-only mount, in a hook that runs before every
push, without thinking about any of it. The cost of validation is a directory
walk. An operation names and proves its narrower authority before it changes
anything.

It also means the answer is reproducible. Two runs over one repository produce
byte-identical output — findings and scanned paths are sorted — so a diff
between two runs is a diff about the repository, never about ordering, timing,
or something the tool fetched.

## Related

- [Why RHINO exists](./why-rhino-exists.md)
- [Exit codes](../reference/exit-codes.md)
