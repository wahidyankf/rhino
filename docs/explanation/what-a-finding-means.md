# What a finding means

A finding is one violation of one rule the repository declared for itself. That
sentence carries more weight than it looks like it does, and most of RHINO's
design is in the words it leaves out.

## A finding is not advice

RHINO has no opinion about how long a document should be, which colours a
diagram may use, or where instructions live. It enforces what
`repo-config.yml` says and nothing else. So a finding is never a suggestion,
never a style preference, and never a matter of taste.

This is what makes exit `1` usable in a gate. If findings mixed "you broke your
own rule" with "the tool would prefer otherwise", nobody could block a push on
one without arguing about the other.

## A finding is attributable

Every finding names the path that caused it, and a line where the rule has a
position. Not the file that noticed the problem — the file that has it.

This sounds obvious and is easy to get wrong. A validator that walks blocks
inside a document, or reconciles a canon against several harnesses, has to
carry the origin all the way to the report. When it does not, a maintainer gets
told that _something_ is wrong _somewhere_ in a tree, which is the same amount
of information as no finding at all.

## A finding has a kind, and the kind is a contract

Every finding carries a stable identifier — `word-limit-exceeded`,
`missing-map-entry`, `agent-prompt-divergence`. The kind is what a consumer
filters on, so it is never reworded to improve a message. The message is prose
and exists to be improved.

That split is why the JSON output puts them in separate fields, and why the
advice in the how-to guides is always _filter on `kind`_.

## What is deliberately not a finding

The interesting cases are the ones RHINO refuses to report.

**A file that exists and cannot be opened.** This is exit `2`, not a finding.
Reporting a permission fault as a missing file would tell a maintainer to write
a file that is already there, and would let an infrastructure problem
masquerade as a policy violation.

**A file that vanishes between the walk and the read.** The repository changed
underneath the run. Blaming the maintainer for a race is blaming them for the
tool's timing.

**Syntax RHINO does not understand.** A fenced block whose diagram declaration
this build cannot parse is declined, not reported. Reporting on it would be
reporting on the tool's own ignorance, and would make every RHINO upgrade a
potential source of new findings in unchanged files.

**Zero.** A surface that matched no files, a tree with no directories, a roster
with no harnesses — these pass. But the run says how many it inspected, every
time, including when the answer is zero. A clean word-budget run lists every
path it read:

```console
$ rhino governance word-budget validate
[word-budget] checked 2 files, no findings
[word-budget] scanned AGENTS.md
[word-budget] scanned README.md
```

That listing is the difference between "checked and clean" and "checked
nothing". Both exit `0`. Only one of them means anything.

## Why the same shape for every validator

One finding type, one printer, one exit-code mapping, shared by all five
validators. The alternative — five nearly-identical result types and five
nearly-identical printers — drifts apart one fix at a time until a caller has
to know which validator produced a line before it can read it.

The category prefix is atomic rather than assembled from the command path, so
`grep '\[word-budget\]'` gets that validator's output and nothing else,
whichever spelling of the command produced it.

## Related

- [Findings](../reference/findings.md) for every kind
- [Exit codes](../reference/exit-codes.md)
- [Why RHINO holds no defaults](./holding-no-defaults.md)
