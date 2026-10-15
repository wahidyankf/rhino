# RHINO Architecture

The as-built C4 model for the RHINO command-line validator. Behaviour lives in [`behaviours/`](behaviours/README.md);
this file describes structure. Where the two disagree, the scenarios are canonical and this file is wrong.

## Scope

RHINO validates one repository's Markdown, instruction files, and harness adapters against declared policy. Explicit
gate, toolchain, and adapter commands use separately injected process or transaction ports. A gate with a
`regular-files` input also reads Git's staged-change or immutable-range metadata through the disk tree adapter before it
decides whether to dispatch its declared child; ordinary tree validation remains a short-lived, read-only process.

Four constraints shape every box below, and none of them is a convention that could be relaxed later:

- **Read-only validation.** A validator opens no file for writing inside the inspected tree.
- **Network-free.** RHINO opens no socket, including loopback. External URLs found in Markdown are recognized and
  skipped, never fetched.
- **Bounded process launch.** `gate run`, toolchain operations, and adapter generation's `model-resolve` command start
  only declared argv through separate ports.
- **Bounded adapter mutation.** Only generation receives an adapter-store port; it replaces declared roots only after a
  complete lossless plan exists.

## System Context

Both callers run RHINO and read its findings and exit code; RHINO only reads the inspected repository.

```text
+------------------------+  runs a check         +-----------------+  reads files  +----------------------+
| Maintainer (person)    |---------------------->|                 |-------------->| Inspected repository |
|                        |<----------------------| RHINO validator |               | (external)           |
+------------------------+  findings, exit code  | (system)        |               +----------------------+
                                                 |                 |
+------------------------+  runs on push         |                 |
| Repository gate or     |---------------------->|                 |
| hook (person)          |<----------------------|                 |
+------------------------+  findings, exit code  +-----------------+
```

A gate and a maintainer are the same caller from RHINO's side: both pass arguments and read an exit code. The tool has
no notion of who invoked it, which is why its output has to be legible to a human and parseable by a script from the
same run.

## Containers

The executable hands parsed arguments to the library, which reads policy and files, runs declared children, and returns
findings.

```text
+------------------+  parses arguments  +---------------------+  reads declared policy  +---------------------+
| rhino executable |------------------->| rhino library crate |------------------------>| repo-config.yml     |
| (unit)           |<-------------------| (unit)              |                         | (data)              |
+------------------+  findings          |                     |                         +---------------------+
                                        |                     |  validator reads        +---------------------+
                                        |                     |------------------------>| Repository files    |
                                        |                     |  adapter transaction    | (data)              |
                                        |                     |------------------------>|                     |
                                        |                     |                         +---------------------+
                                        |                     |  declared argv only     +---------------------+
                                        |                     |------------------------>| Declared child argv |
                                        +---------------------+                         | (data)              |
                                                                                        +---------------------+
```

The split between the binary and the library is not stylistic. Rust links integration tests under `tests/` against a
crate's library target only, so a binary-only crate would have forced the behaviour corpus into `#[cfg(test)]` modules
inside `src/` — which is exactly the boundary the specification standard forbids. The library holds every decision; the
binary holds argument parsing, output writing, and the exit code.

## Components

The command tree dispatches to grouped checks and operation owners; every check that reads Markdown shares one scan, and
tree file access converges on the runtime filesystem port. The opt-in `regular-files` gate input uses a separate Git
metadata boundary in the disk tree adapter. Tags mark entry points, checks, and shared components.

```text
cli -- command tree [entry]
 |
 +--> config -- grouped schema [shared] ------------------------------------+
 |                                                                          |
 +--> v0_4 -- grouped dispatch owners [shared]                              |
 |     |                                                                    |
 |     +--> internal_link [check] ---+                                      |
 |     +--> mermaid [check] ---------+                                      |
 |     +--> word_budget [check] -----+--> markdown scan [shared] ---------->+
 |     +--> metadata [check] --------+                                      |
 |     +--> directory_map [check] ----------------------------------------->+
 |     +--> quality_gates [check] ----------------------------------------->+
 |     +--> conventions [check]                                             |
 |                                                                          |
 +--> harness adapters [check] -------------------------------------------->+
 +--> environment and toolchains ------------------------------------------>+--> runtime -- filesystem port [shared]
 +--> lifecycle gates [entry] --> gate resolver [shared] --> Tree port [shared] --> disk tree
                                      |                            |                +--> Git metadata
                                      |                            |                       --> raw parser [shared]
                                      +--> launcher port [shared]  |
                                      +--> mutation runner port [shared]
```

Tree validators reach the inspected filesystem through one port trait rather than through `std::fs` directly. That is
what lets the unit adapter drive the corpus against an in-memory tree, and it is also the seam that keeps validators
read-only: the port exposes no write operation. `regular-files` is an explicit gate selection capability. The gate
resolver asks the tree for indexed or immutable-range regular destinations before projecting paths to the declared
child. The disk tree obtains raw Git metadata with fixed local queries; the private `runtime::git_paths` parser
interprets the NUL-delimited bytes without filesystem or process access. The in-memory tree supplies explicit selections
for tests. The gate resolver separately dispatches checks through the launcher port and mutations through the mutation
runner port; the disk metadata adapter starts neither.

The Markdown validators share one scan so the tree is walked once per invocation. Directory maps use the declared
directory projection; harness adapters use their own named inputs and transaction boundary.

The grouped `v0_4` boundary owns the stable validation, lifecycle, adapter, and operation paths. `gates` dispatches
declared children through the launcher port. `v0_4::operations` owns environment transactions and the separate toolchain
runner port; it starts only declared typed argv, without a shell or reported child output.

## Data

RHINO owns no persistent store. Its inputs are the declared configuration and the inspected tree; its outputs are a
findings report on stdout and an exit code. Nothing survives the process.

`repo-config.yml` is owned by the consuming repository, not by RHINO. RHINO ships no default for any value in it: a
repository that declares nothing gets a configuration error rather than a borrowed assumption from whichever repository
the author had in mind.

## Boundaries

- **Process**
  - Where it is: The `rhino` executable's argument and exit-code contract
  - How it is held: Observed by the E2E adapter, which sees nothing else
- **Filesystem**
  - Where it is: The port trait in `runtime`
  - How it is held: Substituted wholesale by the unit adapter
- **Git metadata**
  - Where it is: The disk tree adapter's fixed index and immutable-range queries, parsed by `runtime::git_paths`
  - How it is held: Queried only for a declared Git-backed gate input; parser tests use bytes, and the unit tree uses
    explicit selections
- **Host process**
  - Where it is: Launcher, toolchain-runner, and model-resolver ports in `runtime`
  - How it is held: Declared typed argv only; no shell or output report
- **Repository root**
  - Where it is: Every resolved path
  - How it is held: Escapes by `..`, absolute path, or link are refused or reported, never followed; a surface glob
    follows a link only to a target inside the root
- **Trust**
  - Where it is: The inspected tree is untrusted input
  - How it is held: Linear-time matching only; no backtracking engine

The trust boundary is the one most easily lost. RHINO matches patterns against Markdown a repository committed, so a
pattern engine that can backtrack catastrophically would let content hang the gate. That is why the regex engine is
chosen for its linear-time guarantee and why three patterns are hand-written parsers rather than being made to fit a
more expressive engine.
