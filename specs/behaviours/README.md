# Behaviours

One feature file per validator, plus the contract every command shares and the
configuration grammar they all read. Every scenario binds at the unit adapter;
integration and E2E adapters bind the same scenarios at their own boundaries.

Scenarios establish the policy they assert. RHINO ships no default word limit,
tree list, harness roster, or colour palette, so a scenario that needs one says
so in a `Given` rather than relying on a value compiled into the binary.

Three conventions make those declarations unambiguous, because an adapter has to
implement them the same way at three boundaries:

- A declaring `Given` is **additive over a complete, valid configuration**. It
  states the one thing the scenario cares about; everything else the schema
  requires is already present and legal. That is why a feature can declare only
  a palette without its scenarios failing as an incomplete configuration.
- A **scenario-level declaration replaces** the `Background`'s value for the same
  key rather than merging with it. Two scenarios depend on that to prove a
  declared value is really data: one declares a one-colour palette and another a
  different grapheme limit, and each must fail if the `Background` value won.
- **`an empty repository` speaks about the tree, not the configuration.** It
  removes files; it never removes a declaration made before it.

## Directory Map

- [cli-contract.feature](cli-contract.feature) — command tree, flags, output
  formats, and the exit codes every command shares.
- [directory-map.feature](directory-map.feature) — README presence and
  directory-map completeness across declared trees.
- [harness-parity.feature](harness-parity.feature) — one canonical instruction
  body, skill bundle, and agent roster reconciled across every declared harness.
- [internal-link.feature](internal-link.feature) — repository-local Markdown
  links resolve to files that exist.
- [mermaid-cli.feature](mermaid-cli.feature) — how Mermaid diagrams are
  discovered, parsed, and reported on.
- [mermaid-legibility.feature](mermaid-legibility.feature) — the accessibility
  rules a diagram must satisfy once it has been parsed.
- [repo-config.feature](repo-config.feature) — the configuration grammar, its
  required fields, and what an unusable configuration does.
- [word-budget.feature](word-budget.feature) — declared word surfaces, their
  limits, and word counting itself.
