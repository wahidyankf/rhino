# RHINO Architecture

The as-built C4 model for the RHINO command-line validator. Behaviour lives in [`behaviours/`](behaviours/README.md); this file describes structure. Where the two disagree, the scenarios are canonical and this file is wrong.

## Scope

RHINO reads one repository's Markdown, instruction files, and harness adapters, compares them against that repository's declared policy, and reports findings. It is a single short-lived process with no state between runs.

Three constraints shape every box below, and none of them is a convention that could be relaxed later:

- **Read-only.** RHINO opens no file for writing inside the inspected tree.
- **Network-free.** RHINO opens no socket, including loopback. External URLs found in Markdown are recognized and skipped, never fetched.
- **Process-free.** RHINO spawns no child process. Everything it reports, it read itself.

## System Context

```mermaid
graph TD
    Maintainer["Maintainer"]
    Gate["Repository gate or hook"]
    RHINO["RHINO validator"]
    Tree["Inspected repository"]

    Maintainer -->|runs a check| RHINO
    Gate -->|runs on push| RHINO
    RHINO -->|reads files| Tree
    RHINO -->|findings, exit code| Gate
    RHINO -->|findings, exit code| Maintainer

    classDef person fill:#0173B2,stroke:#000000,color:#FFFFFF
    classDef system fill:#029E73,stroke:#000000,color:#000000
    classDef external fill:#808080,stroke:#000000,color:#FFFFFF

    class Maintainer,Gate person
    class RHINO system
    class Tree external
```

A gate and a maintainer are the same caller from RHINO's side: both pass arguments and read an exit code. The tool has no notion of who invoked it, which is why its output has to be legible to a human and parseable by a script from the same run.

## Containers

```mermaid
graph TD
    Binary["rhino executable"]
    Library["rhino library crate"]
    Config["repo-config.yml"]
    Files["Repository files"]

    Binary -->|parses arguments| Library
    Library -->|reads declared policy| Config
    Library -->|reads through one port| Files
    Library -->|findings| Binary

    classDef unit fill:#0173B2,stroke:#000000,color:#FFFFFF
    classDef data fill:#CA9161,stroke:#000000,color:#000000

    class Binary,Library unit
    class Config,Files data
```

The split between the binary and the library is not stylistic. Rust links integration tests under `tests/` against a crate's library target only, so a binary-only crate would have forced the behaviour corpus into `#[cfg(test)]` modules inside `src/` — which is exactly the boundary the specification standard forbids. The library holds every decision; the binary holds argument parsing, output writing, and the exit code.

## Components

```mermaid
graph TD
    Cli["cli — command tree"]
    Runtime["runtime — filesystem port"]
    ConfigMod["config — schema"]
    Scan["markdown scan"]
    Link["internal_link"]
    Mermaid["mermaid"]
    Word["word_budget"]
    Map["directory_map"]
    Harness["harness"]

    Cli --> ConfigMod
    Cli --> Link
    Cli --> Mermaid
    Cli --> Word
    Cli --> Map
    Cli --> Harness
    ConfigMod --> Runtime
    Scan --> Runtime
    Harness --> Runtime
    Map --> Runtime
    Link --> Scan
    Mermaid --> Scan
    Word --> Scan

    classDef entry fill:#DE8F05,stroke:#000000,color:#000000
    classDef check fill:#0173B2,stroke:#000000,color:#FFFFFF
    classDef shared fill:#029E73,stroke:#000000,color:#000000

    class Cli entry
    class Link,Mermaid,Word,Map,Harness check
    class ConfigMod,Runtime,Scan shared
```

Every validator reaches the filesystem through one port trait rather than through `std::fs` directly. That is what makes the unit adapter able to drive the whole corpus against an in-memory tree, and it is also the seam the read-only constraint is enforced at: the port exposes no write operation, so a validator cannot write even by mistake.

The three Markdown validators share one scan so the tree is walked once per invocation. `harness` and `directory_map` do not use it, because they read named paths and directory structure rather than Markdown content.

## Data

RHINO owns no persistent store. Its inputs are the declared configuration and the inspected tree; its outputs are a findings report on stdout and an exit code. Nothing survives the process.

`repo-config.yml` is owned by the consuming repository, not by RHINO. RHINO ships no default for any value in it: a repository that declares nothing gets a configuration error rather than a borrowed assumption from whichever repository the author had in mind.

## Boundaries

| Boundary        | Where it is                                              | How it is held                                       |
| --------------- | -------------------------------------------------------- | ---------------------------------------------------- |
| Process         | The `rhino` executable's argument and exit-code contract | Observed by the E2E adapter, which sees nothing else |
| Filesystem      | The port trait in `runtime`                              | Substituted wholesale by the unit adapter            |
| Repository root | Every resolved path                                      | Paths that escape the root are refused, not clamped  |
| Trust           | The inspected tree is untrusted input                    | Linear-time matching only; no backtracking engine    |

The trust boundary is the one most easily lost. RHINO matches patterns against Markdown a repository committed, so a pattern engine that can backtrack catastrophically would let content hang the gate. That is why the regex engine is chosen for its linear-time guarantee and why three patterns are hand-written parsers rather than being made to fit a more expressive engine.
