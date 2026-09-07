# Reference

Exact statements about what RHINO accepts and what it emits. These pages
describe behaviour rather than teach it; if you are looking for a walkthrough,
start with the [tutorials](../tutorials/README.md).

Where a page here and [`specs/behaviours/`](../../specs/behaviours/README.md)
disagree, the corpus wins.

## Directory Map

- [Command line](./cli.md) — every command, every flag, and which flags each command accepts.
- [Exit codes](./exit-codes.md) — `0`, `1`, and `2`, and what each one promises.
- [Findings](./findings.md) — every finding kind each validator can report, and what causes it.
- [JSON output](./json-output.md) — the `--output json` document, field by field.
- [Configuration](./configuration.md) — every key of `repo-config.yml`, its type, and the rules that bind it to the others.

## Next steps

- [How-to guides](../how-to/README.md) to put these to work.
- [Explanation](../explanation/README.md) for the reasoning behind them.
