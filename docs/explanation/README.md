# Explanation

Why RHINO is built the way it is. These pages are for understanding rather than
for doing; nothing here is needed to run the tool.

## Directory Map

- [Why RHINO exists](./why-rhino-exists.md) — the decay it catches, and why documentation rots differently from code.
- [Holding no defaults](./holding-no-defaults.md) — why a missing value is a refusal rather than a guess.
- [What a finding means](./what-a-finding-means.md) — why exit `1` is always a claim about your repository and never about your invocation.
- [Reading only](./reading-only.md) — why tree validation never mutates, runs a subprocess, or touches the network.
- [Why the Grouped Schema Is Generated](./generated-schema.md) — why one typed model produces both parsing and the
  checked-in editor artifact.

## Next steps

- [Tutorials](../tutorials/README.md) if you would rather learn by doing.
- [Reference](../reference/README.md) for the exact statements behind these arguments.
