# Execution Learnings

Capture discoveries here when they happen. Before archival, give each entry exactly one durable owner or discard it with
a reason under the knowledge-capture convention. This file is not a second progress record.

## Planning Discovery — Snapshot Selection and Mutation Are Separate

The current local mutation runner already refuses a selected symlink and preserves an unselected tracked link as link
data. Narrowing the input does not authorize weakening the full snapshot transaction or its containment checks.

Disposition: pending execution; intended durable owner is the regression test for staged formatting beside compatibility
links. The final disposition must name its exact test after implementation.

## Planning Discovery — Release Notes Need an Explicit Baseline

The release workflow carries version-specific prior-stable note baselines. The new minor release must add its baseline
there and use the same baseline for the screened preview.

Disposition: pending execution; intended durable owner is the additive branch in `.github/workflows/release.yml`.
