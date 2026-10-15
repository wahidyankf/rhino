# Execution Learnings

Capture discoveries here when they happen. Before archival, give each entry exactly one durable owner or discard it with
a reason under the knowledge-capture convention. This file is not a second progress record.

## Planning Discovery — Snapshot Selection and Mutation Are Separate

The current local mutation runner already refuses a selected symlink and preserves an unselected tracked link as link
data. Narrowing the input does not authorize weakening the full snapshot transaction or its containment checks.

Disposition: resolved by `policy::regular_files_host_selection_preserves_nine_indexed_links_and_partial_staging` in
`tests/integration/policy.rs`. The real pre-commit fixture proves that the new input formats only the selected partially
staged regular file while all nine staged links retain their raw targets and `120000` modes; the same fixture proves the
legacy `Files` input still refuses its selected-link set before starting the formatter. This keeps the two selection
contracts explicit and preserves the full mutation snapshot boundary.

## Planning Discovery — Release Notes Need an Explicit Baseline

The release workflow carries version-specific prior-stable note baselines. The new minor release must add its baseline
there and use the same baseline for the screened preview.

Disposition: pending execution; intended durable owner is the additive branch in `.github/workflows/release.yml`.
