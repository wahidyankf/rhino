# The Public Contract

A consumer pins RHINO by version, commit, and checksum, and calls it from a hook or a gate it does not want to think about. That pin must mean the same thing tomorrow. The surfaces below are public, and changing one is a version decision rather than an implementation detail.

## Exit Codes

- `0` — the requested checks ran and found nothing.
- `1` — the requested checks ran and reported findings.
- `2` — the invocation, the root, or the configuration was unusable, so nothing was checked.

Only a validator may report `1`. The distinction that matters is between "checked and clean" and "never checked": a caller must be able to act on the code alone, without knowing which subcommand ran. A check that cannot run reports `2` rather than an empty finding list, because an unread tree and a clean tree are not the same claim.

## `version --json`

Emits `schemaVersion`, `version`, and `commit`. A consumer bootstrap compares a downloaded executable's embedded identity against its lock before running it, so this shape is load-bearing rather than informational. Adding a field is compatible; renaming, removing, or changing the type of one is not.

## Commands, Flags, and Configuration Keys

Never remove or rename a command, flag, exit code, or configuration key without a major version. Adding is free; moving is not.

A configuration key that becomes optional is compatible. A key that becomes required is not, and neither is tightening validation of a value a consumer already declares — a repository that validated yesterday must validate today on the same pin.

## What Is Not Public

Finding message wording, output ordering beyond what a documented format guarantees, log lines on standard error, and every internal module. A consumer that parses a human-readable message has taken a dependency this repository did not offer.

## Related

- [Vision](../vision/README.md)
- [Release cut](../workflows/release-cut.md)
