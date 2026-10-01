# Markdown Visualizations

Use a visualization when it makes relationships, sequence, state, hierarchy, or another nontrivial structure materially
easier to understand. Use prose for a simple fact and a table for an exact mapping. A diagram added as decoration, or
duplicating what a neighbouring table already says better, is worse than no diagram.

## Requirements

- Draw every diagram as plain ASCII art in a `text` fenced block. Mermaid is not used: this repository's Markdown is
  read in terminal editors without a renderer, where Mermaid shows as source.
- Use only printable ASCII: `+`, `-`, `|` for boxes and lines, and `>`, `<`, `^`, `v` for arrow heads. Box-drawing
  characters and arrows such as `→` render at ambiguous widths in some terminals.
- Keep every line within the [Markdown line length](markdown-line-length.md).
- Precede each diagram with one sentence that states what it shows. That sentence carries the meaning for a screen
  reader, and for anyone searching the text.
- Never encode meaning through position or line style alone. Label each box, and name a category in the label, such as
  `(system)` or `[check]`.
- Choose the smallest drawing that carries the point. Where a graph would cross its own edges, draw it as a layered
  tree.
- Place the diagram beside the prose it supports, and update it in the same change as the structure it depicts.

Directory trees are not diagrams; they stay in `text` blocks in their usual form.

## Enforcement

`policies.markdown.mermaid.authoring-rule: plain-text` in [`repo-config.yml`](../../repo-config.yml) makes the `mermaid`
gate refuse any Mermaid block. Whether a drawing is clear is a judgement for review.

The Mermaid validator itself remains a product feature for repositories that declare `rendered`.
