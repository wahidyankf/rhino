# Report vendor vocabulary exceptions that name no file

`governance vendor validate` and `repo-config validate` accept a vocabulary exception whose path matches no file, so an
exception left behind by a rename or a deletion stays in configuration with no gate reporting it.

> Filed 2026-10-06 by a consuming repository under its Upstream Tool Defects standard. Reproduced on released `v0.11.0`
> (`db711617a95ed67700f42d6652c5a9c271a166e6`) and on trunk `6d31682fe724e3a5331a13af94b40d0cc85ce160`. Low severity.

## Problem and evidence

### Description

`policies.governance.vendor.vocabulary-exceptions[].paths` entries are checked for shape: an escaping path such as
`../outside` exits `2` with "exact repository-relative paths". They are not checked for existence. After the excepted
file is renamed or deleted, both commands stay green and the exception becomes a stale declaration. The vendor scan does
catch the renamed file at its new path; it never reports the stale exception itself.

### Steps to reproduce

From an empty directory, with a `rhino` binary on `PATH`:

```sh
mkdir docs
cat > repo-config.yml <<'EOF'
schema: rhino/repo-config/v2
repository: {}
policies:
  governance:
    vendor:
      roots: [docs]
      forbidden-terms: ["Acme"]
      vocabulary-exceptions:
        - term: "Acme"
          paths:
            - docs/this-file-was-renamed-away.md
EOF
printf '# Clean\n\nNothing forbidden here.\n' > docs/a.md
rhino governance vendor validate; echo "exit $?"
rhino repo-config validate; echo "exit $?"
```

### Expected behaviour

A finding, or the configuration exit `2` that an escaping exception path already gets, naming the exception path that
matches no file. Alternatively, documentation stating that existence is deliberately unchecked. The contract scenarios
in [`v0-4-contract.feature`](../../../specs/behaviours/v0-4-contract.feature) require only "exact repository-relative
paths", and do not say either way.

### Actual behaviour

Identical on `v0.11.0` and on trunk:

```text
[vendor] checked 1 file, no findings
[vendor] scanned docs/a.md
exit 0
[repo-config] checked 1 configuration file, no findings
exit 0
```

### Environment

macOS on arm64. The released `v0.11.0` archive and a `cargo build --release` of trunk.

### Documentation consulted

- [Findings](../../../docs/reference/findings.md#vendor-terms): `forbidden-vendor-term` is "A declared forbidden term
  appears in a file outside its exact exception." Nothing covers the exception itself.
- [v0.4 configuration](../../../docs/reference/v0-4-configuration.md) names vendor vocabulary as a governance policy
  without describing exception validation.

### Impact

Low. A stale exception is harmless to the scan, but stale exceptions accumulate and hide the rename that made them
stale. A consumer found stale governance paths in its declarations only by searching by hand. Other lists of declared
paths in `repo-config.yml` may share the gap; they were not surveyed.

## Why now

There is no time pressure and the impact is small, so this brief parks the sighting where the next consumer to meet it
finds it rather than rediscovering it.

## Prior art

Duplicate check, run 2026-10-06 against `wahidyankf/rhino`:

- Issues, all states: none exist in the repository (`is:issue` returns 0).
- Pull requests: none open, and none in any state match `vocabulary-exceptions` or "vocabulary exception".
- In-flight plans: [`harden-rhino-validation-edge-cases`](../../backlog/harden-rhino-validation-edge-cases/README.md)
  does not touch vendor policy. `plans/in-progress/` holds no plan.
- Idea briefs: none existed in any quadrant.

Precedent inside RHINO: `governance traceability validate` already reports a declared artifact path that does not exist
as `missing-traceability-artifact`, and `governance layers validate` reports a missing declared category. Both are
contract scenarios in the same feature file. A declared path that names nothing is already a finding elsewhere in the
tool.

## Proposed direction

- Report an exception path that matches no file, either as a finding from `governance vendor validate` or as a
  configuration fault from both commands, following whichever precedent above fits the owner's taxonomy.
- Or, smallest: state in the findings reference that exception paths are not checked for existence.

## Scope and non-goals

In scope: vendor vocabulary exception paths.

Out of scope: a survey of every other declared path list, which is a separate question worth asking once this is
decided.

## Risks and open questions

- Whether a new finding on existing configuration counts as a breaking change for a consumer whose configuration already
  holds a stale exception, under the [public contract](../../../repo-governance/development/public-contract.md).
- Whether the check belongs in `governance vendor validate` alone, or also in `repo-config validate`, which would then
  have to read the tree.

## Workaround

On every rename or deletion under a vendor root, search `repo-config.yml` for the old path and update its exception by
hand.

## Success and promotion signal

Success: the reproduction above produces a finding or an exit `2` naming the stale path, or the reference states the
limit.

Promote when a second consumer meets it, or when a survey of declared path lists turns up the same gap elsewhere.
