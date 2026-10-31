# Release Notes

Every published release carries notes a stranger can act on. A list of pull request titles says that something changed.
It does not say what the change means for a reader, who must act, or whether to pin the release. The standard below
applies to every tag, stable or prerelease, and to the notes a maintainer writes after [release cut](release-cut.md)
publishes.

Notes are public. They must not name a private repository, a machine, a LAN address, or a local path. The
[release cut](release-cut.md) screens the final text before the tag exists.

## Sections

Write these five sections, in this order, with these headings.

### 1. Summary

Two or three plain sentences. Say what this release is, and whether to pin it. For example, say that it is a fix release
for users of the previous version, and that they should move to it.

### 2. What changed

Group the items under four headings: `### Fixes`, `### Features`, `### Breaking`, and `### Docs and CI`. Write `None.`
under a heading with no items. Each item is one sentence about a user-visible effect, followed by its pull request link:

```markdown
- `gate run --output json` reports the surface and completed gates.
  ([#123](https://github.com/wahidyankf/rhino/pull/123))
```

Leave out changes a consumer cannot observe, unless the Docs and CI group needs them to explain a required action.

### 3. Who is affected and what to do

Say who must act and what they do. Cover upgrade or pin advice, known limitations, and any version to avoid, with the
reason. If nobody needs to act, say so in one sentence.

### 4. Verification

Name the assets and show how to check them:

- the four archives: `rhino-aarch64-apple-darwin.tar.gz`, `rhino-x86_64-apple-darwin.tar.gz`,
  `rhino-aarch64-unknown-linux-gnu.tar.gz`, and `rhino-x86_64-unknown-linux-gnu.tar.gz`;
- `checksums.txt`, which covers the four archives.

Give the check as a command:

```sh
sha256sum -c checksums.txt
```

On macOS, use `shasum -a 256 -c checksums.txt`. Every line must end in `OK`. The grouped schema asset
`rhino-repo-config-v2.schema.json` is not in `checksums.txt`; say how to compare it with the tagged commit:
`git show vX.Y.Z:schemas/repo-config/v2.schema.json | cmp - rhino-repo-config-v2.schema.json`.

### 5. Full Changelog

End with one link to the comparison against the previous release, using the version before this one:

```markdown
**Full Changelog**: https://github.com/wahidyankf/rhino/compare/vPREVIOUS...vTHIS
```

## Rules

- Write plain English. Name the user-visible effect, not the internal change.
- Keep every line within 120 columns.
- Use the release tag exactly, and the previous stable tag for a stable release.
- Do not paste generated notes into the body. The generated list is a source for the items, not the notes.

## Template

```markdown
# rhino vX.Y.Z

## Summary

<what this release is, and whether to pin it>

## What changed

### Fixes

- <one sentence of user-visible effect>. ([#N](link))

### Features

None.

### Breaking

None.

### Docs and CI

- <one sentence>. ([#N](link))

## Who is affected and what to do

<who must act, what they do, known limitations, versions to avoid>

## Verification

Download the four archives and `checksums.txt` from this release, then run `sha256sum -c checksums.txt`.
On macOS, use `shasum -a 256 -c checksums.txt`. Every line must end in `OK`.

**Full Changelog**: https://github.com/wahidyankf/rhino/compare/vPREVIOUS...vX.Y.Z
```
