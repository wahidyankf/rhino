# Behaviours

One feature file per validator, plus the contract every command shares and the
configuration grammar they all read. Every scenario binds at the unit adapter;
integration and E2E adapters bind the same scenarios at their own boundaries.

Scenarios establish the policy they assert. RHINO ships no default word limit,
tree list, harness roster, or colour palette, so a scenario that needs one says
so in a `Given` rather than relying on a value compiled into the binary.

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
