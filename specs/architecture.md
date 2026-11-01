# RHINO Architecture

This is the as-built C4 model of the Go implementation. It describes the code that ships today. The Rust library crate
that preceded it is gone, and nothing here reasons about crates, features, or a second implementation.

RHINO is one process. A command reads one repository's `repo-config.yml`, decides what to inspect, and exits with a code
the caller can branch on. Reads go through the `Tree` port. Writes reach the repository only through the environment
store, the adapter store, and the apply step of a mutation gate.

## Level 1: System Context

```text
  developer or hook --+
  CI job -------------+
                      |
                      v
            +--------------------+
            |  RHINO (one Go     |
            |  static binary)    |
            +----------+---------+
                       |
     +-----------------+------------------+
     v                 v                  v
  repository       Git (plumbing)     declared children
  working tree,    index, range,      gate commands,
  repo-config.yml  hook message,      mutators, toolchain,
                   push input         probes, model cmds
```

The repository is the only source of policy. RHINO holds no default for any policy key: an undeclared policy is a
refusal, not a silent pass.

## Level 2: Containers

```text
  +-----------------------------------------------------------------+
  | rhino (cmd/rhino)                                               |
  |   signals become a context; the result becomes the exit code    |
  |                                                                 |
  | internal/bootstrap      wires the production adapters           |
  | internal/adapters/cli   argv, streams, signal context           |
  | internal/adapters/config   repo-config.yml decoding             |
  | internal/adapters/runtime  every operating-system interaction   |
  | internal/application    use cases and ports                     |
  | internal/domain         closed error vocabulary                 |
  +-----------------------------------------------------------------+

  scripts/release-archive   builds the release archive, not part of the binary
  tests/bdd                 the behaviour corpus runner; builds the binary once
```

There is one product executable, `cmd/rhino`. The other packages are its libraries, or a harness that runs it. The one
exception is `scripts/release-archive`, a build-time tool: `scripts/build-release.sh` compiles it separately to write
release archives, and it is not part of the binary.

## Level 3: Components

### cmd/rhino and internal/bootstrap

`cmd/rhino` owns the process boundary. It turns SIGINT and SIGTERM into a cancelled context and turns the returned
integer into the exit status. `internal/bootstrap` wires the production adapters into the CLI application. No other
package names a concrete adapter.

### internal/domain

The closed error vocabulary. Each `ErrorCode` has a stable `rhino.*` identifier, and every refusal and JSON error
document carries one of them. The package has no behaviour beyond naming and classifying errors.

### internal/application

The use cases and the ports they need. `cli.go` holds the closed command table: 28 leaves, each with its path, category,
accepted flags, and summary. `Parse` in `cli.go` checks an invocation against that table; `execute.go` dispatches it and
renders a result. Validator cores (`naming.go`, `heading.go`, `frontmatter.go`, `metadata.go`, `link.go`, and the
others) are pure over a `Tree` and the parsed configuration.

`surfaces.go` is the one place a declared surface glob follows a filesystem link. It follows a link only when the link's
target lies inside the root. It refuses a link that leads outside with `rhino.path.escapes-root`, and a link that loops
or resolves nowhere with `rhino.file.unreadable`. A link no surface reaches is neither followed nor refused.

### internal/adapters/cli

The argument-to-invocation boundary and the `Application` value that holds each optional port. A nil port is never
silently replaced by a no-op. Each port documents its own failure:

- A nil `Environment` fails `environment init`, `backup`, and `restore` closed. Reads still work.
- A nil `Adapters` fails adapter generation closed. Validation still works.
- A nil `Models` resolves through the declared fallbacks and reports a warning.
- A nil `Toolchains` and a nil `Mutations` produce the same refusal an unwired entry point reports.
- A nil `Launcher` refuses gate dispatch honestly.

### internal/adapters/config

Parses `repo-config.yml` into a `Document`, in three layers applied in order. First, shape checks on gate argv and
environment entries: a plain scalar or sequence where a struct belongs, an unknown field inside one, and an environment
mapping without its `input` are each refused, with a message that names the field, line, and column. Second,
closed-vocabulary and reference checks for gate inputs and commands (`gate_validation.go`). Third, surface-membership
checks that decide which lifecycle surfaces each gate runs.

### internal/adapters/runtime

Every operating-system interaction lives in this package:

| File           | Port implemented                | Responsibility                                      |
| -------------- | ------------------------------- | --------------------------------------------------- |
| `disk.go`      | `Tree`                          | Read-only walk; never follows a link to read a file |
| `launcher.go`  | `Launcher`                      | Gate children: argv, environment, working directory |
| `mutation.go`  | `MutationRunner`                | Index snapshot and pull-request replay checkouts    |
| `toolchain.go` | `ToolchainRunner`               | Bounded output, timeouts, and no shell              |
| `adapters.go`  | `AdapterStore`, `ModelResolver` | Generation writes; model commands run declared argv |
| `gitpaths.go`  | Git plumbing helpers            | Mode-aware index and range path listing             |

## Ports

The application package declares each boundary as an interface. A validator never reaches past its port.

| Port               | Declared in      | Production adapter             | Used by                                 |
| ------------------ | ---------------- | ------------------------------ | --------------------------------------- |
| `Tree`             | `execute.go`     | `runtime.DiskTree`             | every read-only validator               |
| `Launcher`         | `execute.go`     | `runtime.OSLauncher`           | `gate run`                              |
| `MutationRunner`   | `mutation.go`    | `runtime.DiskMutationRunner`   | `gate run` on mutation gates            |
| `ToolchainRunner`  | `toolchain.go`   | `runtime.DiskToolchainRunner`  | `toolchain validate`, applied provision |
| `EnvironmentStore` | `environment.go` | `runtime.DiskEnvironmentStore` | `environment init`, `backup`, `restore` |
| `AdapterStore`     | `harness.go`     | `runtime.DiskAdapterStore`     | `harness adapters generate`             |
| `ModelResolver`    | `harness.go`     | `runtime.DiskModelResolver`    | adapter generation with model commands  |

`Tree` is link-aware. `Files` lists regular files and never a link. `Links` lists every link with where its target lies:
inside the root, outside it, or nowhere. `PassesThroughLink` reports whether any component of a path is a link, and
`Read` refuses such a path before it opens anything.

## Data

- **Configuration:** one YAML document, decoded once into a `config.Document`. `repo-config validate` refuses an unknown
  top-level key and an unknown field in a gate argument, environment projection, or `run-on` entry. Each policy
  validator refuses an unknown key inside the group it reads, except the emoji group, which accepts extra keys, and the
  umbrella policy keys, which no validator reads. Leaves also refuse unknown keys under the core groups they read: the
  scan-based leaves (for example `md mermaid validate`) under `scan`, `env validate` under `environment`,
  `toolchain validate` under `toolchains`, and `harness adapters validate` under `harness`. `repo-config validate`
  passes those keys, and it passes a key directly on a gate entry or its command. There is no second configuration
  source.
- **Reports:** each command builds an `application.Report` and renders it as text, or as one JSON document when
  `--output json` is given. `version` alone also accepts `--json`, a shorthand for the same. JSON documents carry
  `schemaVersion`.
- **Gate dispatch:** a declared argv, environment projection, and working directory. The child inherits the process
  environment plus `RHINO_GATE_SURFACE` and the projected entries. Its output is discarded; its exit status decides.

## Exit Codes

| Exit    | Meaning                                                                               |
| ------- | ------------------------------------------------------------------------------------- |
| `0`     | Ran and found nothing                                                                 |
| `1`     | Ran and found a violation, or a `gate run` child exited non-zero or ended by a signal |
| `2`     | Could not run: invocation, configuration, or an unreadable input                      |
| `126`   | `gate run` only: a declared child exists but cannot be executed                       |
| `127`   | `gate run` only: a declared child cannot be found                                     |
| `128+N` | RHINO was ended by signal `N`: `130` interrupt, `143` termination, `141` closed pipe  |

A check child that ends by a signal is a failing gate, reported as `1`. A mutation child that ends by a signal is
reported as `2`, a gate that could not run. RHINO's own status describes RHINO only, so a child's signal never becomes
`128+N`.

## Test Layout

| Location                | What it proves                                                  | Run by                |
| ----------------------- | --------------------------------------------------------------- | --------------------- |
| `internal/**/*_test.go` | Parsers, validator cores, and adapters in temporary directories | `go test ./...`       |
| `tests/bdd/`            | Every scenario in `specs/behaviours`, against the built binary  | `go test ./tests/bdd` |
| `tests/bdd` compliance  | Every step is bound once, every binding is used, count matches  | `go test ./tests/bdd` |

The behaviour suite builds the binary once per run. Each scenario gets its own temporary repository, its own Git
repository built with plumbing commands, and its own `PATH` directory. It runs `rhino` as a child process and asserts on
the exit code, the streams, the JSON, and the repository state that remains afterwards. A scenario that cannot run
hermetically is reported with a proposed reviewed exemption, never dropped.
