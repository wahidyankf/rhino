# Why RHINO validation only reads

RHINO tree validators open files and print. They write nothing and connect to nothing, and the only child processes they
start are read-only Git queries. Explicit operations are separate: `gate run` starts only a declared child argv,
toolchain commands start declared typed argv without a shell or reported output, `env validate` runs one
`git diff --cached` query for the staged paths when `environment.staged` is declared, and adapter generation replaces
only declared adapter families and exact adapter files after planning every output in memory, starting at most a tier's
declared `model-resolve` argv. These are enforced as tests, not incidental properties of the current implementation: the
import and dependency rules live in `internal/bootstrap/boundary_test.go`, and the tree adapter's write and Git rules
live in `internal/adapters/runtime/read_boundary_test.go`.

## The four boundaries

**A validator writes nothing to the repository it inspects.** Not a cache, not a report, not a lock file. A hygiene tool
that writes into the tree it is judging can change the answer it is about to give, and can turn a read-only checkout
into a failure. An explicit adapter transaction has a different port: it rejects paths outside declared adapter families
or exact files and is never reachable from a validator.

**A tree validator starts no child process except a read-only Git query.** The validator cores import no process
package. The tree adapter starts only `git diff`, `git log`, and `git rev-parse`, and only when a validator asks for Git
state, such as the staging guard. No validator starts a formatter or a shell. Everything it reports is derived from
bytes it read itself or from one of those queries. `gate run` and the toolchain commands are separate declared
operations: each starts typed argv at a narrow port and never interprets a shell command. Toolchain probes and provision
never report child output; provision stops at its first failure. Adapter generation may start a tier's declared
`model-resolve` argv through its own port, with a timeout; `harness adapters validate` never does.

**It opens no socket, including loopback.** No product package imports `net`, and the `rhino` binary's dependency graph
contains no `net` package. There is no telemetry, no version check, no remote policy fetch. This is stricter than the
layer rule it sits under, which permits an integration test to own a loopback socket — that permission is about test
plumbing, and this is a claim about the product.

**Product code imports no `unsafe` package, cgo, or `plugin`.** The validator cores import no `os` package at all, so
they cannot touch the file system except through the tree port the entry point supplies.

## Why they are tests rather than prose

A README that says "RHINO does not reach the network" is a promise. A test that fails when a network package appears in
the binary's dependency graph is a mechanism. `internal/bootstrap/boundary_test.go` is one, and it also fails when a
validator core imports a process or file package. The rule predicates each have a positive control in
`TestImportRulesClassifyEachPathAsDeclared`, and the process searches each check a known case, so an empty result is a
real result and not a search that found nothing.

Any test whose success condition is "found nothing" needs a **positive control**: the same search, in the same form,
must find a case that is certainly there. A search that finds nothing looks exactly like a search that is broken.

## What it cannot reach

Two rules keep a run inside the repository it was pointed at.

**A path leaving the root is refused.** A `--file`, `--directory`, or `--dir` value whose `..` segments climb out of the
root is an invocation error, not a path to resolve. So is an absolute one: a selection is relative to the repository
root, or the answer would depend on where the command ran. `--root` itself is not a selection; it names the repository,
so any directory is accepted there.

**A symbolic link is followed only inside the root, and only by a surface glob.** The walk skips links regardless of
what the scan exclusions say, because following one can leave the repository entirely — and a validator that walked out
of the tree would report findings about files the repository does not contain. The same holds for any path that passes
through a link, at any component. A `--file` or `--directory` selection behind one is refused with
`rhino.path.escapes-root`, an internal link whose target lies behind one is reported as outside the repository, and a
declared file behind one is refused as unreadable. None of them is read.

A declared surface glob is the one exception, because a glob that silently matched nothing behind a link would report a
policy it never checked. It asks where a link it reaches leads, without reading through it, and follows the link only
when the target resolves inside the root. A link that leads out, or loops, refuses the run instead of being skipped. The
[configuration reference](../reference/v0-4-configuration.md#surface-globs-and-symbolic-links) states the rule.

## What this buys you

You can run a RHINO validator against a repository you do not trust, in a sandbox with no network, on a read-only mount,
in a hook that runs before every push, without thinking about any of it. The cost of validation is a directory walk. An
operation names and proves its narrower authority before it changes anything.

It also means the answer is reproducible. Two runs over one repository produce byte-identical output — findings and
scanned paths are sorted — so a diff between two runs is a diff about the repository, never about ordering, timing, or
something the tool fetched.

## Related

- [Why RHINO exists](./why-rhino-exists.md)
- [Exit codes](../reference/exit-codes.md)
