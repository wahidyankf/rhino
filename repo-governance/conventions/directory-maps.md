# Directory Maps

Every directory under `repo-governance/`, `docs/`, `specs/`, and `plans/` — including each tree's own root — must contain a `README.md` that explains the directory and maps its contents.

## Requirements

- Include a `## Directory Map` section in every such `README.md`.
- List every direct sibling file and subdirectory other than the `README.md` itself, whatever its file type.
- Use relative links and give each entry a concise description. A directory entry may link to that directory's own `README.md`.
- When a README has no siblings, state that explicitly in the map.
- Update the map in the same change that adds, removes, moves, or renames a sibling.

Never omit a sibling to make a map shorter. The map is the reason the tree is navigable without opening every file, which is what [progressive disclosure](../principles/progressive-disclosure.md) asks of it.

Which trees are mapped is declared in [`repo-config.yml`](../../repo-config.yml) under `governance-directory-map`. This document states the rule; that file is where the enforced list lives, and adding a tree to one without the other is the mistake to watch for.

## Word Budget

Root `AGENTS.md` and every Markdown file under `repo-governance/` carry the word limits declared in `repo-config.yml`. Markdown under `docs/`, `specs/`, and `plans/` is exempt, because reference material, a behaviour corpus, and a delivery record are each sized by what they must say.

The budget is a maximum, not a target. When a governed document outgrows it, split it into coherent documents with distinct reader tasks and give each one a map entry. Never raise the limit to fit a document and never cut substance to fit the limit.

No shell one-liner reproduces the count. `repo-config.yml` declares `count: letters-and-digits`, and the validator applies that rule to link syntax too, so a Markdown link contributes one word per path segment while a table's `|` and `---` separators contribute none. `wc -w` therefore understates a link-dense document and overstates a table-heavy one. Use it for a rough sense of size and take the verdict from the gate.

Budget the headroom, not just the file. When a document sits near its limit, state a new rule in its canonical home and link to it rather than compressing a neighbour to make room.

## Enforcement

```sh
./hippo run --class ephemeral --disk-path . -- cargo xtask self-validate
```

The same command runs as the last step of `cargo xtask test-quick`, which is what the `pre-push` hook and the [pull-request quality gate](../../.github/workflows/pr-quality-gate.yml) run.
