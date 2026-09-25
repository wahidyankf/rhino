# Refuse a Surface Glob Behind a Link

A declared surface glob whose literal prefix is a symbolic link matches nothing and passes; decide whether it should
refuse the way every exact declared path behind a link now does.

Provenance: found on 2026-09-26 during a cross-repository standards adoption, after the v0.6.0 release.

## Problem and Evidence

- With the v0.6.0 binary, a `word-budget` surface `gov/**/*.md` where `gov` is a symbolic link to a directory of
  Markdown files reports `checked 0 files, no findings` and exits `0`. The files the author meant to govern are never
  read, and nothing says why.
- The docs permit this: [findings](../../../docs/reference/findings.md) lists "a surface, tree, or roster that matched
  nothing" among the situations that are not findings, and [exit codes](../../../docs/reference/exit-codes.md) says a
  surface that matched no files still exits `0`.
- Every exact declared path behind a link already refuses. The v0.6.0 fix that refuses declared paths outside the root
  or behind a link made a directory-map tree, vendor root, or layer root behind a link refuse with
  `rhino.file.unreadable`, and [reading only](../../../docs/explanation/reading-only.md) says "a declared file behind
  one is refused as unreadable". A glob's literal prefix is the one declared path that still reads as empty.

## Why Now

The v0.6.0 audit closed the neighbouring cases, so this is the last declared-path shape where a link turns a policy
into a silent pass. An empty match is visible only to a reader who checks the inspected count.

## Prior Art

- [ripgrep's guide](https://github.com/BurntSushi/ripgrep/blob/master/GUIDE.md), accessed 2026-09-26: symbolic links
  are not followed unless `--follow` is passed, and a skipped link is silent. RHINO keeps the not-following half; the
  question is only whether the silence is acceptable for a declared prefix.
- `src/runtime/disk.rs` already answers whether a path passes through a link at any component; `src/scan.rs` compiles
  the surface globs.

## Proposed Direction

Take the literal, wildcard-free prefix of each declared surface glob. When any component of that prefix exists and is a
symbolic link, refuse with exit `2` and `rhino.file.unreadable`, naming the configuration key. A link reached only
through a wildcard component keeps today's skip-and-walk-on behaviour, because the author never named it.

Alternatives to weigh: report a note rather than refuse, which keeps exit `0` but makes the empty match explicit; or
keep the behaviour and document the glob case beside the exact-path refusals.

## Scope and Non-Goals

In scope: surface globs across every policy that compiles through the shared glob compiler, their Gherkin, and the
reference and explanation pages above. Not in scope: following links, changing exclusion lists (never read), or
changing the zero-match answer for a prefix that simply does not exist.

## Risks and Open Questions

- A configuration that passes today would exit `2`. Does the [public contract](../../../repo-governance/development/public-contract.md)
  treat that as a behaviour fix, as the v0.6.0 exact-path refusals were, or as major-version work?
- Should a missing literal prefix also refuse, or does zero stay a real answer there?

## Related

- [Harden RHINO Validation Edge Cases](../../backlog/harden-rhino-validation-edge-cases/README.md) is a sibling
  validation plan with owner-approved scope; this idea is not part of it and would be promoted separately.

## Success and Promotion Signal

Success: no declared surface can pass by reading nothing because of a link the author named. Promote once the owner
chooses between refusal and a note and answers the contract question.
