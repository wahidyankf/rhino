# Resolve `#fragment` anchors in `md internal-link validate`

`md internal-link validate` checks that a link's file exists but never checks the heading its `#fragment` names, so a
link to a heading that was renamed or never existed passes clean.

> Filed 2026-10-06 by a consuming repository under its Upstream Tool Defects standard. Reproduced on released `v0.11.0`
> (`db711617a95ed67700f42d6652c5a9c271a166e6`) and on trunk `6d31682fe724e3a5331a13af94b40d0cc85ce160`. The behaviour is
> documented, so this is a feature request rather than a contract defect.

## Problem and evidence

### Description

`rhino --help` describes the command as "Check that every local Markdown link resolves inside the repository." A link
whose path exists but whose `#fragment` names no heading in the target file passes. A fragment-only link (`#section`)
whose heading is absent from the same file is skipped and not counted. A consumer cannot use the gate to catch a heading
rename or a document split that strands anchors, and the one-line description reads as if it could.

### Steps to reproduce

From an empty directory, with a `rhino` binary on `PATH`:

```sh
printf 'schema: rhino/repo-config/v2\nrepository: {}\npolicies:\n  markdown:\n    internal-link: {}\n' > repo-config.yml
printf '# B\n\n## Real Heading\n' > b.md
printf '# A\n\n[dangling](./b.md#nope)\n\n[dangling in same file](#also-nope)\n\n[good](./b.md#real-heading)\n' > a.md
rhino md internal-link validate; echo "exit $?"
rhino md internal-link validate --output json; echo "exit $?"
```

### Expected behaviour

Taken literally, the command description promises that every local link resolves: exit `1`, with findings for `a.md:3`
(`./b.md#nope`) and `a.md:5` (`#also-nope`). If fragments are deliberately out of scope, which the reference and the
markdown-links rule both say, the one-line description should say "every local link target exists" rather than
"resolves", and a consumer that wants fragments checked should have a way to opt in.

### Actual behaviour

Identical on `v0.11.0` and on trunk:

```text
[internal-link] checked 2 links, no findings
exit 0
{"schemaVersion":1,"command":"internal-link","exitCode":0,"subject":"link","inspected":2,
"scanned":[],"notes":[],"violations":[]}
exit 0
```

The real JSON output is one line, broken here after `"inspected":2,` to fit the page. The two counted links are the two
that carry a path; the fragment-only link is skipped.

### Environment

macOS on arm64. The released `v0.11.0` archive and a `cargo build --release` of trunk. The code path is `local()` in
[`src/markdown/internal_link.rs`](../../../src/markdown/internal_link.rs), which drops a fragment-only destination and
splits every other destination at `#` before resolving it.

### Documentation consulted

- [Findings](../../../docs/reference/findings.md#internal-links): "Fragment-only links (`#section`) and links carrying a
  scheme (`https:`, `mailto:`) are skipped rather than reported". A fragment after a path is not mentioned.
- [Markdown links](../../../repo-governance/conventions/markdown-links.md): "Fragments and query strings do not change
  the target-file check."

The behaviour matches both pages. The gap is between them and the command's one-line description, and between the
command and what a consumer needs from it.

### Impact

One consumer found seven dangling fragments in its documentation only by hand, after the gate had passed them. Its
linking rule now asks authors to check each fragment by hand against GitHub's slug rules, which is the check a gate
could run.

## Why now

Consumers adopt `md internal-link validate` as their link gate on every push, and progressive-disclosure splits, which
move headings between files, are routine work for them. Each split can strand anchors that no gate reports. Nothing is
blocked: the workaround below covers the gap today. It stays important rather than urgent.

## Prior art

Duplicate check, run 2026-10-06 against `wahidyankf/rhino`:

- Issues, all states: none exist in the repository (`is:issue` returns 0).
- Pull requests: none open. Searching all pull requests for `fragment` and `anchor` finds only unrelated closed work.
- In-flight plans: [`harden-rhino-validation-edge-cases`](../../backlog/harden-rhino-validation-edge-cases/README.md)
  does not touch internal links. `plans/in-progress/` holds no plan.
- Idea briefs: none existed in any quadrant.

References, all accessed 2026-10-06:

- markdownlint [`MD051` link-fragments](https://github.com/DavidAnson/markdownlint/blob/main/doc/md051.md) validates
  fragments against generated heading anchors, including in-file fragments. It is the workaround below.
- [lychee](https://github.com/lycheeverse/lychee) checks fragments behind its `--include-fragments` flag, off by
  default: opt-in fragment checking in a link checker.
- [github-slugger](https://github.com/Flet/github-slugger) emulates how GitHub turns a heading into an anchor, including
  its suffixes for repeated headings. It is the reference slug algorithm a consumer rendering on GitHub expects.
- GitHub's [section links][github-section-links] documentation states the anchor rules for rendered headings.

## Proposed direction

- Smallest fix: reword the command description so it promises only that a link's target exists.
- Preferred: an optional key under `policies.markdown.internal-link`, off by default, that resolves a fragment against
  the target file's headings, or against the containing file for `#section`, and reports a new finding kind for one that
  names no heading. Absent the key, today's contract and output stay exactly as they are.
- Slug rules come from configuration or a named algorithm, so RHINO ships no host-specific default.

## Scope and non-goals

In scope: fragment resolution for Markdown headings, its finding kind, the description wording, and the reference page.

Out of scope: explicit HTML anchors (`<a id>`), fragments into non-Markdown files, external links, and changing the
default behaviour of the existing command.

## Risks and open questions

- Which slug algorithms to support, and whether a consumer names one. GitHub's rules differ from other renderers'.
- Whether an explicit `<a id="...">` or `{#id}` attribute counts as a target, and how far HTML is parsed for it.
- Whether a new finding kind under an opt-in key is a minor change under the
  [public contract](../../../repo-governance/development/public-contract.md). It appears to be, since no existing output
  changes.

## Workaround

Run markdownlint with `MD051` enabled beside `md internal-link validate`, or check each fragment by hand against the
renderer's slug rules.

## Success and promotion signal

Success: with the opt-in key declared, the reproduction above exits `1` naming both dangling fragments, and the good
link still passes. Without the key, output is byte-identical to today's.

Promote when the slug-algorithm question is settled and a second consumer asks for it, or when the owner chooses to
retire the markdownlint workaround from consumer gates.

[github-section-links]:
  https://docs.github.com/en/get-started/writing-on-github/getting-started-with-writing-and-formatting-on-github/basic-writing-and-formatting-syntax#section-links
