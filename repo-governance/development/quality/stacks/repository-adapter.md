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

| Pack    | Status  | Reason                                    |
| ------- | ------- | ----------------------------------------- |
| `rust`  | adapted | the test layout deviates; two gate gaps   |
| `shell` | adapted | two strict-mode deviations; two gate gaps |

## Shared Standards

Adopted as written: [Type and Boundary Safety](../code/type-and-boundary-safety.md),
[Shell Scripts](../code/shell-scripts.md), [Lint Strictness](../checks/lint-strictness.md),
[Meaningful Coverage](../testing/meaningful-coverage.md), and
[Stack Packs](../../../conventions/structure/stack-packs.md).

Kept under their local owners, which an adopted document links wherever the catalog linked its own:
[test-driven development](../../test-driven-development.md),
[red, green, refactor](../../../workflows/red-green-refactor.md),
[behaviour-driven development](../../behaviour-driven-development.md),
[end-to-end testing](../../end-to-end-testing.md), [quality gates](../../quality-gates.md),
[software quality enforcement](../../software-quality-enforcement.md),
[specification maintenance](../../specification-maintenance.md), [public contract](../../public-contract.md),
[code clarity](../../code-clarity.md), [dependency selection](../../dependency-selection.md), and
[docs propagation](../../../workflows/docs-propagation.md).

Not adopted:

- **Test Boundaries and Gates**, because quality gates and behaviour-driven development already own the layers, the
  quick gate, and coverage from the passing run.
- **Test Doubles, Test Data Isolation, and Git Fixture Isolation**, because quality gates owns test isolation here, and
  each rests on owners this repository does not hold.
- **A reference page under `docs/`**, because
  [documentation architecture](../../../conventions/documentation-architecture.md) keeps `docs/` for people using RHINO.

A catalog owner with no local counterpart stays named in the adopted text without a link.

## Adopter Decisions

| Source                        | Decision            | Choice                                                                   | Reason                                                  |
| ----------------------------- | ------------------- | ------------------------------------------------------------------------ | ------------------------------------------------------- |
| `swe-code-maker`              | stack skill loading | read on demand                                                           | a new stack needs no agent edit                         |
| software quality enforcement  | coverage floor      | 99% of lines over the validator modules, in the unit run                 | local floor; two exclusions are proved end to end       |
| quality gates                 | task runner         | `cargo xtask` behind `npm run test:quick`                                | gate automation stays typed and tested; no task graph   |
| `rust-standards.md`           | edition             | 2024 in every manifest                                                   | current; nothing to migrate                             |
| `rust-standards.md`           | unit test placement | deviation: the unit adapter lives in `tests/unit`                        | one behaviour corpus runs through three adapters        |
| `rust-standards.md`           | lint table          | gap: flags and `clippy.toml` only; no `pedantic` or `unwrap_used` denial | review applies the rule; the code work comes later      |
| `rust-standards.md`           | unsafe code         | gap: `#![forbid(unsafe_code)]` per crate root, not the manifest lint     | the local rule stays; test targets are not yet covered  |
| `rust-standards.md`           | async runtime       | not applicable                                                           | no async code                                           |
| `001-code-shape-and-security` | security libraries  | not applicable                                                           | no secret, transport, or query                          |
| `shell-scripts.md`            | interpreter         | Bash; deviation: the pinned `./hippo` wrapper is POSIX `sh`              | it runs before any toolchain                            |
| `shell-scripts.md`            | strict mode         | deviation: the public-safety scripts and the hook test runner omit `-e`  | each handles every child status explicitly              |
| `shell-standards.md`          | static analysis     | gap: ShellCheck covers only `./hippo` and the public-safety scripts      | wiring the rest is later work; the rule is not weakened |
| `shell-standards.md`          | format              | gap: no shell formatter gate                                             | as above                                                |
| `shell-standards.md`          | test tool           | plain shell runners and the behaviour corpus                             | no shell test dependency to pin                         |

## Project Applicability

- [rhino](../../../../README.md) — the CLI and library; [quality gates](../../quality-gates.md) names its commands.
- xtask — the gate and release automation in `xtask/`, named in quality gates; outside the coverage floor.
- [public-safety](../../../../scripts/public-safety/README.md) — the adopted outbound scanner and its case suite.
- repo-scripts — the hook helpers in `scripts/` and the guard in `.claude/hooks/`, named in quality gates and tested by
  their own runners; no coverage, per Meaningful Coverage.

## Version Sources

- `rust`: `rust-toolchain.toml`, `Cargo.toml`, `xtask/Cargo.toml`, `Cargo.lock`
- `shell`: `.github/workflows/pr-quality-gate.yml` (ShellCheck)
