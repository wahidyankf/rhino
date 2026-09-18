# Reference

Exact statements about what RHINO accepts and what it emits. These pages
describe behaviour rather than teach it; if you are looking for a walkthrough,
start with the [tutorials](../tutorials/README.md).

Where a page here and [`specs/behaviours/`](../../specs/behaviours/README.md)
disagree, the corpus wins.

## Directory Map

- [Command line](./cli.md) — every command, every flag, and which flags each command accepts.
- [Exit codes](./exit-codes.md) — `0` through `3`, and what each one promises.
- [Findings](./findings.md) — predecessor finding kinds retained during the RC migration window.
- [JSON output](./json-output.md) — the legacy report envelope and grouped command-specific status documents.
- [Configuration](./configuration.md) — the grouped v2 contract and its RC migration boundary.
- [Grouped v2 Configuration](./v0-4-configuration.md) — the closed grouped contract, generated schema, modelines,
  and offline verification.

## Next steps

- [How-to guides](../how-to/README.md) to put these to work.
- [Explanation](../explanation/README.md) for the reasoning behind them.
