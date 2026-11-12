# Behaviour Suite

This package runs the Gherkin corpus in `specs/behaviours` against the real `rhino` binary. Most scenarios are
command-line scenarios: the command-line adapter drives the compiled executable, and no such scenario binds to an
internal function.

The environment vault corpora, `environment-vault-validation.feature` and `environment-vault.feature`, are the
exception. Their steps are bound once, in `tests/bdd/vault`, and that one registry runs at three adapters: the unit
adapter (an in-process, in-memory tree), the integration adapter (a real temporary Git repository), and the command-line
adapter in this package. The unit and integration runners sit beside the code they test, in `internal/application` and
`internal/adapters/runtime`.

Run it with `go test ./tests/bdd`. `scripts/test.sh` runs it again under the race detector.

## Files

| File                   | Role                                                                            |
| ---------------------- | ------------------------------------------------------------------------------- |
| `main_test.go`         | `TestMain` builds `rhino` once and removes it after                             |
| `behaviours_test.go`   | `TestBehaviours`: runs the corpus in strict mode, fails on a count mismatch     |
| `world_test.go`        | Per-scenario state: a temporary repository, a `bin` directory, logs             |
| `git_test.go`          | Git fixtures built with plumbing: base commit, range head, real index           |
| `steps_test.go`        | Command-line step bindings, plus the shared vault steps from `vault.Bindings()` |
| `compliance_test.go`   | Corpus compliance: complete, bound once, every binding used, parses             |
| `vault_driver_test.go` | Command-line driver for the vault corpus: a repository and the built `rhino`    |
| `vault/`               | Shared vault step registry, suite runner, Git trace shim, wiring check          |

## How A Scenario Runs

Each scenario gets its own directory under the suite's temporary root. The world places a scenario `bin` directory first
on `PATH` for bare child names, and strips inherited `PATH`, `RHINO_*`, and `GIT_*` variables so the host cannot leak
into the run. The step bindings write fixture files, run `rhino` as a child process, and assert on the exit code,
stdout, stderr, the JSON document, and the repository state the run left behind.

A scenario that needs Git builds its fixtures with plumbing commands only. A base commit holds the fixtures. A range
head is built from a scratch index with the base as its parent. Tracked executables are staged with mode `100755`,
because the mutation runner replays committed content.

## The Vault Corpus At Three Adapters

`vault.RunSuite` runs the corpus at each adapter. It fails first on any step that is unbound, ambiguous, or unused. It
then runs every scenario in strict mode against that adapter's driver, and fails when the executed count differs from
the parsed count. `vault/wiring_test.go` checks, by syntax tree, that the unit and integration runners each call
`vault.RunSuite` from a Test function with no skip before the call. It checks by text that the command-line runner and
the step file use the shared registry.

Each driver builds one scenario repository. Each run keeps a process record. The unit driver records the two Git name
queries it is asked, with the argv it hard-codes. The integration and command-line drivers record the git children that
a PATH shim logs. The shim lives beside the repository under test, never inside it. The limits of each record, and the
boundary tests that stand beside it, are stated on the `startedOnlyReadOnlyQueries` step in `vault/vault.go`.

## Exemptions

The corpus has no exemptions. The suite has no skip path: a skipped scenario makes the executed count differ from the
parsed count, and `TestBehaviours` fails. A scenario that cannot run hermetically must be proposed for an exemption and
reviewed before its scenario is excluded.

## Compliance

`TestFeatureCompliance` parses the corpus without running any scenario. It fails when a feature file does not parse,
when a step has no binding, when a step matches more than one binding, or when a binding is never used. Its rejection
tests prove each check fires, so the compliance check cannot pass by accident.
