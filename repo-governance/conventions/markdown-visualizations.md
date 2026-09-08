# Markdown Visualizations

Use a visualization when it makes relationships, sequence, state, hierarchy, or another nontrivial structure materially easier to understand. Use prose for a simple fact and a table for an exact mapping. A diagram added as decoration, or duplicating what a neighbouring table already says better, is worse than no diagram.

## Requirements

- Prefer Mermaid over ASCII art wherever Mermaid can express the structure. Use ASCII only when it cannot, or when a plain-text fallback is specifically required.
- Choose the smallest diagram type that carries the point.
- Place the diagram beside the prose it supports, and keep enough surrounding text that the meaning survives without the image and remains searchable.
- Update a diagram in the same change as the structure it depicts.

## Accessibility

Every diagram must stay understandable to colour-blind readers and legible in light and dark rendering.

- Never encode meaning through colour alone. Combine colour with labels and, where the distinction matters, with shape or line style.
- Use the Okabe-Ito fills declared in [`repo-config.yml`](../../repo-config.yml): blue `#0173B2`, orange `#DE8F05`, teal `#029E73`, brown `#CA9161`. Borders are black `#000000`; text is black or white `#FFFFFF`.
- Do not use red, green, yellow, or pink, including for success and failure semantics.
- Declare colours with `classDef` and hex values, never CSS colour names or scattered inline styles.
- Meet WCAG AA contrast: 4.5:1 for normal text, 3:1 for large. White text on `#029E73` does not reach it; use black.
- Never put the palette in a `%%` comment. Some renderers fail on a diagram whose first line is a comment. Explain colour meaning in prose.

The validator enforces styled `classDef` declarations in `flowchart`, `graph`, `classDiagram`, `stateDiagram`, `stateDiagram-v2`, `erDiagram`, `requirementDiagram`, and `block`. Unstyled diagrams pass. It requires 4.5:1 throughout, because rendered text size cannot be proven statically.

## Legibility

Keep every visible node or state label segment at 32 Unicode grapheme clusters or fewer, and every edge or transition label segment at 24 or fewer. Split a longer label with `<br>`, `<br/>`, or an escaped newline; each segment is measured on its own. The validator decodes basic HTML entities, strips markup, normalizes whitespace, and counts user-perceived graphemes rather than code points. It rejects semicolons in state-transition labels, which make statement boundaries ambiguous.

Class members, ER attributes, requirement body fields, directives, comments, and front matter are excluded. The limits prevent known clipping without a renderer; they do not judge layout, so inspect a materially changed diagram yourself.

## Enforcement

```sh
./hippo run --class ephemeral --disk-path . -- cargo xtask self-validate
```

For a single changed file, `rhino md-mermaid --file <path>` validates only that file. The full repository still runs before push.
