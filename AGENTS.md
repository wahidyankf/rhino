# RHINO Contributor Rules

RHINO is a generic repository-hygiene validator. It reads a repository's declared policy and reports findings against it. It owns no repository's answers.

## The Generic-Tool Rule

- Ship no default for any value a repository could reasonably decide differently. A repository that declares nothing gets a configuration error, never a borrowed assumption from whichever repository was in front of the author.
- Name no specific repository, harness, or organization in `src/`. Harnesses, trees, globs, limits, and vocabularies arrive from `repo-config.yml`; the code iterates what it is given.
- The line between policy and tool behaviour is whether a consumer could hold a different opinion and be right. Which diagram syntaxes the parser understands is tool behaviour. Which colours are acceptable is policy.

## Compatibility

- Exit `0` means checked and clean, `1` means findings, `2` means the invocation, root, or configuration was unusable. Only validators report `1`. A caller must be able to read the code without knowing which subcommand ran.
- `version --json` emits `schemaVersion`, `version`, and `commit`. Consumers verify a pinned install against it, so its shape is a public contract.
- Never remove or rename a command, flag, or exit code without a major version. Adding is free; moving is not.

## Specifications

- `specs/` is canonical and lives at the repository root, not under a project directory: this repository is the tool, so there is no second project to disambiguate from.
- `specs/behaviours/` holds the Gherkin corpus and `specs/architecture.md` the C4 model. Assess both for impact before every change, and record a verified no-op rather than churning an unaffected specification.
- Behaviour changes are written Gherkin-first. Prove the binding fails for the stated reason before writing production code.
- Every scenario binds at the unit adapter; there is no unit exemption. An integration or E2E exemption must name the concrete boundary that cannot be reached and the alternative proof that covers it — never difficulty, runtime, flakiness, or cost.
- Classify a test by the strongest real boundary its setup, subject, or assertions touch. E2E is defined by observing only the process contract, not by permission to use more resources.

## Testing

- `cargo xtask test-quick` runs the unit adapter and the static behaviour check. Integration and E2E adapters run on a schedule, never in a Git hook and never in the quick gate.
- Keep line coverage over the validator modules at or above 99%. `src/main.rs` and the concrete filesystem adapter are the only declared exclusions, and `README.md` says why.
- The product is read-only, network-free, and process-free: it opens no socket including loopback, spawns no child process, writes nothing into the tree it inspects, and follows no path outside the declared root. These are enforced by tests, not by documentation. The no-loopback rule is stricter than the integration layer's own boundary, which permits an owned socket — that is deliberate.

## Change Discipline

- Understand before adding, reuse before writing, and stop at the smallest change that is verified. A new dependency needs a stated need, rejected alternatives, evidence of maintenance and defect response, and an owned consequence.
- `#![forbid(unsafe_code)]` stays. `cargo deny check` gates advisories, licences, and duplicate sources.
- Keep `README.md`, `docs/`, and `CHANGELOG.md` true to the built binary. `docs/` follows Diátaxis: each page belongs to exactly one of `tutorials/`, `how-to/`, `reference/`, or `explanation/`. Never publish a command or transcript that has not been executed against the current build. `docs/` may not contradict `specs/`.
- Separate setup, validation, decision, mutation, and return phases with blank lines. A formatter is not semantic grouping.
- Comment non-obvious safety invariants and lifecycle boundaries; do not narrate line by line.

## Release

- Build release assets only through `cargo xtask dist`. A release describes a commit reachable from the default branch or it does not publish.
- Never replace an existing tag, and never weaken checksum verification. A defect becomes a new patch version and a new pin.
- Every release publishes an archive per supported platform plus `checksums.txt`, and each executable's embedded identity matches the tag and commit exactly.

## Repository Hygiene

- Install locked tooling with `npm ci`; hooks enforce Conventional Commits, staged formatting, and the quick gate before push.
- Keep build output, coverage, and `local-tmp/` scratch ignored.
- Never commit credentials, personal or machine identifiers, absolute local paths, or private infrastructure values.
