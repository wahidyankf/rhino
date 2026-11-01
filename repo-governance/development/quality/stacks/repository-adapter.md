---
description: >-
  Records which stack packs this repository adopted, the decisions their standards leave open, and every deviation,
  including the shared standards kept under their local owners.
when_to_use: >-
  Use when working in a project here, adopting or retiring a stack pack, or changing a recorded stack decision.
---

# Repository Adapter

This document owns the `extensions.software-development` inventory in [`repo-config.yml`](../../../../repo-config.yml).
A stronger local rule always wins over an adopted one, and every difference is recorded here with its reason.

## Adopted Packs

| Pack     | Status  | Reason                                                                 |
| -------- | ------- | ---------------------------------------------------------------------- |
| `golang` | adapted | applies to the product code; inventory key `go`; no local standard yet |
| `shell`  | adapted | two strict-mode deviations; two gate gaps                              |

## Shared Standards

Adopted as written: [Type and Boundary Safety](../code/type-and-boundary-safety.md),
[Shell Scripts](../code/shell-scripts.md), [Lint Strictness](../checks/lint-strictness.md),
[Meaningful Coverage](../testing/meaningful-coverage.md), and
[Stack Packs](../../../conventions/structure/stack-packs.md).

Kept under their local owners, which an adopted document links wherever the catalog linked its own:
[test-driven development](../../test-driven-development.md),
[red, green, refactor](../../../workflows/quality/red-green-refactor.md),
[behaviour-driven development](../../behaviour-driven-development.md),
[end-to-end testing](../../end-to-end-testing.md), [quality gates](../../quality-gates.md),
[software quality enforcement](../../software-quality-enforcement.md),
[specification maintenance](../../specification-maintenance.md), [public contract](../../public-contract.md),
[code clarity](../../code-clarity.md), [dependency selection](../../dependency-selection.md), and
[docs propagation](../../../workflows/quality/docs-propagation.md).

Not adopted:

- **Test Boundaries and Gates**, because quality gates and behaviour-driven development already own the layers, the
  quick gate, and coverage from the passing run.
- **Test Doubles, Test Data Isolation, and Git Fixture Isolation**, because quality gates owns test isolation here, and
  each rests on owners this repository does not hold.
- **A reference page under `docs/`**, because
  [documentation architecture](../../../conventions/documentation-architecture.md) keeps `docs/` for people using RHINO.
- **Rust Standards and the Rust skill**, because this repository has no Rust code.

A catalog owner with no local counterpart stays named in the adopted text without a link.

## Adopter Decisions

- `swe-architect` — **ADR location**: the default, `docs/explanation/decisions/NNN-<slug>.md`.
- `swe-developer` — **Stack skills**: read on demand, so a new stack needs no agent edit. **Host-integrated proof**: not
  required, the default.
- `swe-reviewer` — **Reviewer output**: inline. **Specification completeness**: not checked. **Test boundary**: Test
  Boundaries and Gates. Each is the default.
- `swe-releaser` — **Deploy targets**: none; RHINO deploys nothing and publishes only through
  [release cut](../../../workflows/maintenance/release-cut.md).
- **software quality enforcement**
  - Decision: coverage floor
  - Choice: policy 99% over the domain, application, and config packages; the quick gate pins 94%
  - Reason: the gap is stated in [software quality enforcement](../../software-quality-enforcement.md)
- **quality gates**
  - Decision: task runner
  - Choice: plain shell gates behind `npm run test:quick`
  - Reason: no task graph
- `golang`
  - Decision: formatting, vet, and race detection
  - Choice: `gofmt`, `go vet ./...`, and `go test -race -count=1 ./...` in the quick gate
  - Reason: a race in this tool is a finding, not a flake
- `golang`
  - Decision: unit test placement
  - Choice: `_test.go` beside each package; `tests/bdd` runs every scenario against the built binary
  - Reason: every scenario is a command-line scenario
- `golang`
  - Decision: static analysis and vulnerability checks
  - Choice: `go vet`, and `go tool govulncheck ./...` in the full gate; gaps: no linter, licence, duplicate-module, or
    `unsafe` check
  - Reason: review covers the rest
- `shell-scripts.md`
  - Decision: interpreter
  - Choice: Bash; deviation: the pinned `./hippo` wrapper is POSIX `sh`
  - Reason: it runs before any toolchain
- `shell-scripts.md`
  - Decision: strict mode
  - Choice: deviation: the public-safety scripts and the hook test runner omit `-e`
  - Reason: each handles every child status explicitly
- `shell-standards.md`
  - Decision: static analysis
  - Choice: gap: ShellCheck covers only `./hippo` and the public-safety scripts
  - Reason: wiring the rest is later work; the rule is not weakened
- `shell-standards.md`
  - Decision: format
  - Choice: gap: no shell formatter gate
  - Reason: as above
- `shell-standards.md`
  - Decision: test tool
  - Choice: plain shell runners and the behaviour corpus
  - Reason: no shell test dependency to pin

## Skill Names

`swe-releaser` loads the catalog skill `cutting-releases` as the local
[`release-cut`](../../../../.agents/skills/release-cut/SKILL.md).

## Project Applicability

- [rhino](../../../../README.md) — the binary in `cmd/rhino` and its `internal/` packages;
  [quality gates](../../quality-gates.md) names its commands.
- `scripts/release-archive` — the Go archiver that `scripts/build-release.sh` runs; outside the coverage floor.
- [public-safety](../../../../scripts/public-safety/README.md) — the adopted outbound scanner and its case suite.
- repo-scripts — the hook helpers in `scripts/` and the guard in `.claude/hooks/`, named in quality gates and tested by
  their own runners; no coverage, per Meaningful Coverage.

## Version Sources

- `golang`: `go.mod` and `go.sum`, which pin the Go toolchain and the `govulncheck` tool
- `shell`: `.github/workflows/pr-quality-gate.yml` (ShellCheck)
