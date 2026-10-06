# Align what links a direct child under `require-direct-children: true`

`md readme-index validate` with `require-direct-children: true` counts a subdirectory as linked only when the link names
the directory itself, while `every-directory` also accepts a link to its `README.md` or a sibling `<name>.md`, so the
natural index entry `[Sub](sub/README.md)` fails one mode and passes the other.

> Filed 2026-10-06 by a consuming repository under its Upstream Tool Defects standard. Reproduced on released `v0.11.0`
> (`db711617a95ed67700f42d6652c5a9c271a166e6`) and on trunk `a7884d8cf89b01f6100e5ef0fefcbce22c602b9d`. Low severity.

## Problem and evidence

### Description

Under `true`, `root_index` compares each direct child against the index's link targets exactly. A link to
`sub/README.md` resolves to `docs/sub/README.md`, not `docs/sub`, so the subdirectory is reported as omitted. A sibling
`sub.md` beside `sub/` does not cover it either, and every non-Markdown file beside the index must be linked too. Under
`every-directory`, `complete_index` accepts all three directory forms and requires no link to non-Markdown files. Both
behaviours are consistent with their own code; they disagree with each other, and only `every-directory`'s rules are
written down.

### Steps to reproduce

From an empty directory, with a `rhino` binary on `PATH`:

```sh
mkdir -p docs/sub
printf '# Sub\n' > docs/sub/README.md
config() {
  printf 'schema: rhino/repo-config/v2\nrepository: {}\npolicies:\n  markdown:\n    readme-index:\n' > repo-config.yml
  printf '      trees:\n        - path: docs\n          require-direct-children: %s\n' "$1" >> repo-config.yml
}
index() { printf '# Docs\n\n- [Sub](%s)\n' "$1" > docs/README.md; }
config true;            index sub/README.md; rhino md readme-index validate; echo "exit $?"
config true;            index sub/;          rhino md readme-index validate; echo "exit $?"
config every-directory; index sub/README.md; rhino md readme-index validate; echo "exit $?"
config true; printf '# Sub\n' > docs/sub.md; index sub.md; rhino md readme-index validate; echo "exit $?"
rm docs/sub.md; index sub/; printf 'x' > docs/logo.txt; rhino md readme-index validate; echo "exit $?"
```

### Expected behaviour

One rule for what links a subdirectory, shared by both modes, or a sentence in the reference stating that `true` needs a
link to the directory itself and to every non-Markdown file. Today the reference says only that `true` makes the
`README.md` "also link every direct child of the declared directory".

### Actual behaviour

Identical on `v0.11.0` and on trunk (the `scanned` lines omitted):

```text
[readme-index] checked 1 directory, 1 finding
[readme-index] docs/README.md: the declared README index omits a direct child (child=docs/sub)
exit 1
[readme-index] checked 1 directory, no findings
exit 0
[readme-index] checked 2 directories, no findings
exit 0
[readme-index] checked 1 directory, 1 finding
[readme-index] docs/README.md: the declared README index omits a direct child (child=docs/sub)
exit 1
[readme-index] checked 1 directory, 1 finding
[readme-index] docs/README.md: the declared README index omits a direct child (child=docs/logo.txt)
exit 1
```

`./sub/README.md` behaves like `sub/README.md`, and `./sub` and `sub` behave like `sub/`.

### Environment

macOS on arm64. The released `v0.11.0` archive and a `cargo build --release` of trunk.

### Documentation consulted

- [v0.4 configuration](../../../docs/reference/v0-4-configuration.md#readme-index-completeness): under
  `every-directory`, "A subdirectory counts as linked when the index links the directory itself or its `README.md`, or
  when a Markdown file named after it sits beside it" and "Other files are not required". Nothing equivalent is said for
  `true`.
- [Findings](../../../docs/reference/findings.md#readme-index): `missing-readme-index-child` is "A tree declaring
  `require-direct-children` has a direct child its README omits", for both modes.

### Impact

Low. A consumer enabling `true` on 20 trees met 108 findings, 99 of them directory entries written as
`./<dir>/README.md`, and rewrote each link to `./<dir>/`. Its own convention had claimed that a sibling `<dir>.md`
satisfies the requirement, which `every-directory` honours and `true` does not. The finding text gives no hint that the
link form is the cause.

## Why now

No time pressure: the workaround is mechanical and `every-directory` already has the wanted rule. The brief parks the
sighting so the next consumer to enable `true` does not rediscover it.

## Prior art

Duplicate check, run 2026-10-06 against `wahidyankf/rhino`:

- Issues, all states: none exist in the repository.
- Pull requests: none match beyond #79, which added `every-directory` and stated that "`true` and `false` keep their
  meaning". That preserved `true`'s scope, the declared root only; it did not document `true`'s link forms.
- In-flight plans: [`harden-rhino-validation-edge-cases`](../../backlog/harden-rhino-validation-edge-cases/README.md)
  does not touch the README index. `plans/in-progress/` holds no plan.
- Idea briefs: none existed in any quadrant.

Precedent inside RHINO: `complete_index` already implements the directory, index, and sibling-file forms, with a unit
test and contract scenarios in [`v0-4-contract.feature`](../../../specs/behaviours/v0-4-contract.feature).

## Proposed direction

- Let `true` reuse `every-directory`'s child rule for the declared root: accept the directory, its `README.md`, or a
  linked sibling `<name>.md`, and require only Markdown files and indexed subdirectories.
- Or, smallest: document `true`'s exact rule in the configuration reference and name the expected link form in the
  finding detail.

## Scope and non-goals

In scope: what counts as linking a direct child under `true`.

Out of scope: making `true` recurse, which is what `every-directory` is for.

## Risks and open questions

- Aligning `true` stops it from requiring links to non-Markdown files. Under the
  [public contract](../../../repo-governance/development/public-contract.md), dropping findings a consumer may rely on
  could count as a behaviour change.
- Whether `true` should instead be described as a legacy mode, with `every-directory` the recommended setting.

## Workaround

Link each subdirectory as `./<dir>/` rather than `./<dir>/README.md`, link non-Markdown files too, or declare the tree
`every-directory`.

## Success and promotion signal

Success: the first and fourth reproductions above exit `0`, or the reference states `true`'s link rule.

Promote when a second consumer meets it, or when `true` is next touched for any other reason.
