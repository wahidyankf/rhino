# Public Safety

The generic gate. It runs first on every surface that can publish, and a finding
ends the sequence before anything else produces output.

This is the layer every public repository owns a copy of. It screens credential
patterns with a pinned scanner and generic private-metadata shapes with a
tracked regular-expression set, and it holds no identifier of its own: it ships
in a public repository, so a literal private term written here would be the
disclosure it exists to stop. The confidential layer that does hold identifiers
lives in the command center and never enters a public checkout.

## Contract

```bash
scripts/public-safety/public-safety.sh --surface <surface> \
  [--text <string>]... [--file <path>]... [--shapes <path>]
scripts/public-safety/public-safety.sh --print-pin
scripts/public-safety/public-safety.sh --print-platform
```

| Exit | Meaning    | What it means for publication  |
| ---: | ---------- | ------------------------------ |
|    0 | clean      | may proceed                    |
|    1 | blocked    | must not proceed; a finding    |
|    2 | scan error | must not proceed; not screened |

`1` and `2` are both refusals. They differ only in whether we know what is
wrong, and a scan that could not run is not a scan that passed. There is no
allowlist, inline suppression, known-test-value, or bypass. Replace a safe
example with a semantic placeholder; if replacement destroys the artifact's
meaning, keep the artifact private.

## Surfaces

| Surface        | Inputs                                                   |
| -------------- | -------------------------------------------------------- |
| `baseline`     | every tracked file and its name                          |
| `diff`         | every staged file and its name                           |
| `commit`       | a commit message                                         |
| `ref`          | a branch or tag name                                     |
| `pull-request` | a title, a body, an automated comment                    |
| `release`      | a tag annotation, release notes, a changelog excerpt     |
| `logs`         | an output template or an emitted record about to be kept |

`baseline` and `diff` find their own inputs from Git when none are named, so a
hook can state the surface and nothing else. Every other surface must be handed
what is outbound: a wrapper that answered "clean" for a surface nobody gave
anything to would pass by default.

## Where it runs here

| Surface      | Screened                                    |
| ------------ | ------------------------------------------- |
| `commit-msg` | the commit message, before commitlint       |
| `pre-commit` | the staged content and names, before format |
| `pre-push`   | the tracked tree, then the branch name      |
| pull request | the tree, the branch, the title and body    |

The tracked tree is screened at `pre-push` and in CI rather than at
`pre-commit`. A full-tree credential scan costs about twenty seconds; paid on
every commit it buys nothing that is not already paid before anything leaves the
machine, and a gate that slow on the most frequent surface teaches people to
skip it.

## The pinned scanner

TruffleHog `3.97.1`, resolved for a closed platform matrix, downloaded only when
that exact version is absent from a cache outside the repository, verified
against an approved SHA-256 before extraction, and stamped with the digest of
what was extracted. A later run re-checks the binary against that stamp: a
cached scanner that drifted from it drifted after it was approved, and the
wrapper refuses rather than re-downloading, because a hook may have no network
and a gate that reaches for one is least available when it is most needed.
Delete the cache directory to bootstrap again.

Run `--print-pin` to see the version and every approved digest. The scan itself
is offline:

```text
trufflehog <source> --json --no-verification --no-update --fail --fail-on-scan-errors
```

## The canary

Before any repository material is read, the wrapper generates a real key in a
temporary directory outside the repository, scans it, and requires that the
scanner reported a finding, that the generated value reached neither stream nor
any retained file, and that the directory was removed. A canary failure blocks.

This is also what makes `PUBLIC_SAFETY_SCANNER` safe to expose: naming a
different scanner can make the wrapper refuse, and cannot make it pass, because
a scanner that finds nothing fails the canary.

## The shape set

`shapes.txt` holds one row per shape: a class, the kind `regex`, and a POSIX
extended regular expression running to the end of the line. A `literal` row is
refused, as is a pattern carrying a credential prefix or a connection string.
An absent, unreadable, empty, or malformed set makes publication ineligible;
the refusal names what could not be trusted and never quotes the row.

## Diagnostics

A finding is `finding <detector> <path>:<line>`. Nothing else survives: not the
matched text, not the decoder output, not the verification error, not the commit
author, not the raw JSON, and not a scanner stream. A path that is itself
prohibited is reported as `<blocked-path>`.

## Tests

```bash
bash scripts/public-safety/tests/run.sh              # offline, one case per scenario
PUBLIC_SAFETY_ONLINE=1 bash scripts/public-safety/tests/run.sh
```

The contract is [`public-safety.feature`](public-safety.feature) and each case in
`tests/cases/` is named after the scenario it binds. The offline suite drives a
stub scanner so it is fast enough to sit on a hook; the online run additionally
bootstraps, verifies, and canaries the real pinned binary, and is reported as
skipped rather than passed when it does not run.
