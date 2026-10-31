# Behaviour Suite

This package runs the Gherkin corpus in `specs/behaviours` against the real `rhino` binary. Every scenario is a
command-line scenario, so one adapter drives the compiled executable and no scenario binds to an internal function.

Run it with `go test ./tests/bdd`. `scripts/test.sh` runs it again under the race detector.

## Files

| File                 | Role                                                                           |
| -------------------- | ------------------------------------------------------------------------------ |
| `main_test.go`       | `TestMain` builds `rhino` once into a temporary directory and removes it after |
| `behaviours_test.go` | `TestBehaviours`: runs the corpus in strict mode and fails on a count mismatch |
| `world_test.go`      | Per-scenario state: a temporary repository, a scenario `bin` directory, logs   |
| `git_test.go`        | Git fixtures built with plumbing: base commit, range head, real index          |
| `steps_test.go`      | The step bindings, one pattern per step text                                   |
| `compliance_test.go` | Corpus compliance: complete, bound once, every binding used, no parse problem  |

## How A Scenario Runs

Each scenario gets its own directory under the suite's temporary root. The world places a scenario `bin` directory first
on `PATH` for bare child names, and strips inherited `PATH`, `RHINO_*`, and `GIT_*` variables so the host cannot leak
into the run. The step bindings write fixture files, run `rhino` as a child process, and assert on the exit code,
stdout, stderr, the JSON document, and the repository state the run left behind.

A scenario that needs Git builds its fixtures with plumbing commands only. A base commit holds the fixtures. A range
head is built from a scratch index with the base as its parent. Tracked executables are staged with mode `100755`,
because the mutation runner replays committed content.

## Exemptions

The corpus has no exemptions. The suite has no skip path: a skipped scenario makes the executed count differ from the
parsed count, and `TestBehaviours` fails. A scenario that cannot run hermetically must be proposed for an exemption and
reviewed before its scenario is excluded.

## Compliance

`TestFeatureCompliance` parses the corpus without running any scenario. It fails when a feature file does not parse,
when a step has no binding, when a step matches more than one binding, or when a binding is never used. Its rejection
tests prove each check fires, so the compliance check cannot pass by accident.
