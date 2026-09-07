# 🦏 RHINO — Repository Hygiene & INtegration Orchestrator

A configuration-driven repository-hygiene validator. RHINO checks the things a
repository's documentation has to get right — word budgets on governed
instructions, directory maps that match the tree, internal Markdown links that
resolve, Mermaid diagrams that stay legible, and one canonical instruction body
kept in parity across every coding harness the repository declares.

It holds no repository's policy. There is no default word limit, no default tree
list, and no default harness roster compiled into the binary: every value it
enforces arrives from the consuming repository's `repo-config.yml`. The product
is read-only, network-free, and process-free.

## Project status

Early construction. The specification corpus under `specs/behaviours/` is the
canonical statement of behaviour and is being implemented against; where this
README and the corpus disagree, the corpus wins. No release is published yet, so
there is nothing to install.

## Development

```console
$ cargo xtask test-quick
```

Heavy local work runs under the pinned [HIPPO](https://github.com/wahidyankf/hippo)
guard via `./hippo run --class ephemeral --disk-path . -- <command>`.

### Test topology

One corpus, three executing boundaries, and one static check over the mapping
between them. Each test is classified by the strongest real boundary its setup,
subject, or assertions touch — not by the resources it is permitted to use.

| Target                   | Boundary    | What it touches                                                         |
| ------------------------ | ----------- | ----------------------------------------------------------------------- |
| `cargo test --test unit` | unit        | the corpus against an in-memory tree, entirely in process               |
| `--test integration`     | integration | the corpus and six boundary policies against a real temporary directory |
| `--test e2e`             | E2E         | the corpus and one boundary policy against the spawned executable       |
| `--test coverage`        | static      | executes no scenario; reads the corpus and each adapter's registry      |

Every scenario is bound at all three executing boundaries, and **no layer
declares an exemption**. That is a property of this corpus rather than a
default: a sentence here is a claim about behaviour that holds whether the tree
is a map, a directory, or a process's working directory, so a boundary that
could not run one would be a boundary at which the claim is untested.

Two tests are deliberately split across boundaries. `no_leaf_writes_to_the_repository_it_inspects`
appears at integration _and_ E2E: integration proves nothing was written through
the repository root, and only E2E — where the inspected repository is also the
running process's working directory — can see a write to a relative path. The
loopback policy is stricter than the layer rule it sits under, which permits an
integration test to own a loopback socket; that permission is about test
plumbing, and this is a claim about the product.

Only the unit adapter and the static check run in the quick gate and the push
hook. Integration and E2E never do.

### Coverage

`cargo xtask test-quick` measures unit line coverage in the same execution that
runs the unit adapter, and fails below **99%**. Measured once rather than twice,
because two runs can disagree and the number that gates has to be the number the
passing run produced.

Two modules are excluded from the denominator, and they are the only two:

| Excluded              | Why a unit test may not reach it                                                                       |
| --------------------- | ------------------------------------------------------------------------------------------------------ |
| `src/main.rs`         | the process boundary — arguments, standard input, the working directory, stdout, stderr, the exit code |
| `src/runtime/disk.rs` | the only module that talks to `std::fs`                                                                |

Neither is untested. Both are proved by the integration and E2E adapters running
the whole corpus through them, plus the boundary policies — which is a stronger
claim than a line count, and the reason a number that included them would say
less rather than more.

## License

MIT.
