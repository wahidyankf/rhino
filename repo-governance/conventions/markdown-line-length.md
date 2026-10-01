# Markdown Line Length

Every line of Markdown in this repository is at most 120 characters, so a document reads in a terminal editor without a
renderer, wrapping, or horizontal scrolling. Diagrams in it are ASCII, under
[Markdown visualizations](markdown-visualizations.md).

## Requirements

- The limit covers every line: prose, headings, list items, table rows, and fenced code.
- Prettier wraps prose at 120. Lines it cannot break are fixed by hand:
  - a table whose aligned width exceeds 120 has its cells shortened, is split into narrower tables, or becomes a list;
  - a long link target moves to a reference-style definition, which is exempt;
  - a long command in a code block continues on the next line with `\`.
- A fenced example of single-line output may be broken at a field boundary only with a sentence saying the real output
  is one line.
- Generated adapters comply through their source: the route text in [`repo-config.yml`](../../repo-config.yml).

Exempt: `CHANGELOG.md`, archived plans under `plans/done/`, and fixture bytes under `specs/fixtures/`.

## Enforcement

`markdownlint-cli2` runs rule `MD013` alone, configured in [`.markdownlint-cli2.jsonc`](../../.markdownlint-cli2.jsonc),
as the `markdown-line-length` gate on `pre-push` and `pull-request`. Prettier's Markdown settings live in
[`.prettierrc.json`](../../.prettierrc.json).
