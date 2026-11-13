# Markdown Visualizations

Use a visualization when it makes relationships, sequence, state, hierarchy, or another nontrivial structure materially
easier to understand. Use prose for a simple fact and a table for an exact mapping. A diagram added as decoration, or
duplicating what a neighbouring table already says better, is worse than no diagram.

## Requirements

- Prefer a Mermaid diagram over ASCII art whenever Mermaid can express the needed visualization.
- Choose the smallest diagram type that communicates the point clearly, such as a flowchart, a sequence diagram, a state
  diagram, or an entity-relationship diagram.
- Place the diagram near the prose it supports and give nodes, edges, actors, and states meaningful labels.
- Precede each diagram with one sentence that states what it shows. That sentence carries the meaning for a screen
  reader, and for anyone searching the text.
- Every conceptual diagram is a Mermaid block carrying an accessible title (`accTitle`) and an accessible description
  (`accDescr`) naming what it shows.
- Keep every visible node label segment at 32 Unicode grapheme clusters or fewer and every edge label segment at 24 or
  fewer. Use `<br>`, `<br/>`, or an escaped newline to split a longer label into independently measured segments.
- Never encode meaning through color alone. Combine color with clear labels and, when distinctions matter, different
  shapes or line styles.
- Declare colors only with `classDef` and hex values: fills from blue `#0173B2`, orange `#DE8F05`, teal `#029E73`, and
  brown `#CA9161`; strokes from blue `#0173B2` and black `#000000`; text in black `#000000` or white `#FFFFFF`. Use a
  black stroke for shape definition, white text on dark fills, and black text on light fills.
- Meet WCAG AA text contrast: at least 4.5:1 for text on its fill.
- Keep Mermaid source readable, and update a diagram in the same change as the structure it depicts.

Directory trees are not diagrams; they stay in `text` blocks in their usual form. Literal command output, file contents,
and commit messages also stay in their fenced blocks and are never diagrams.

## Enforcement

`policies.markdown.mermaid.authoring-rule: rendered` in [`repo-config.yml`](../../repo-config.yml) makes the `mermaid`
gate refuse a Mermaid block without an accessible title and description, a label longer than the declared limits, and a
color outside the declared palette. Whether a drawing is clear is a judgement for review.
