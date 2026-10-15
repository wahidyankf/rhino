# Technical Design

## Current Cause and Evidence

At baseline `c8d49ff`, `InputKind` in `src/v0_4/config.rs` has `Files` but no regular-file selection. Its Git index and
range bindings call `indexed_files` and `changed_files` through `Tree`. The disk methods use name-only Git output, so
they cannot distinguish a symlink destination from a regular file. `DiskMutationRunner::apply_index` refuses a selected
symlink before launching the mutator. The existing integration tests
`disk_mutation_keeps_an_unselected_tracked_symlink_as_link_data` and
`disk_mutation_refuses_a_selected_symlink_before_starting_the_mutator` demonstrate the two sides of that boundary.
`verify_clean` has different snapshot setup; do not describe the local refusal as an existing range preflight.

## Public Contract and Selection

Add `InputKind::RegularFiles`, serialized as `regular-files`. It accepts only `git-index` and `explicit-range`;
`checkout` and every other source refuse during configuration validation. Preserve the existing `.paths` field, repeated
argument expansion, and environment projection rules. Resolve to `ResolvedInput::Files`, sharing projection and
selected-path union code. Leave `Files` branches and their diagnostics unchanged.

```yaml
inputs:
  staged:
    kind: regular-files
```

Add `Tree::indexed_regular_files()` and `Tree::changed_regular_files(base, head)`, returning the same result shape as
their existing counterparts. Default implementations refuse, preserving source compatibility for external port
implementations. `MemoryTree` gains explicit selections and setters for both capabilities, preserves them when rerooted,
and never infers a Git snapshot from current files. Cover default refusal and reroot behavior.

The disk adapter requests raw NUL-delimited Git records with `--no-renames --no-ext-diff --no-textconv`. It uses
`git diff --cached --raw -z` for the index and the existing validated immutable base/head range for range selection,
retaining option termination. Destination modes `100644` and `100755` are eligible; `120000`, `160000`, and `000000` are
excluded. With rename detection disabled, a rename is deletion plus addition, so its regular destination survives. Git's
[raw output contract](https://git-scm.com/docs/git-diff#_raw_output_format) defines the mode/status header and NUL
pathname boundaries. No filesystem type query participates in selection.

## Parser and Runtime Boundary

Create private `src/runtime/git_paths.rs`, declared by `mod git_paths;` in `src/runtime.rs`. Its entry point is
`pub(super) fn regular_file_paths(raw: &[u8]) -> Result<Vec<String>, String>`. Move the existing pure
`is_repository_path` helper from `disk.rs` into this sibling unchanged and import it back into the disk adapter. Process
execution, filesystem access, and snapshot writes remain in `disk.rs`.

Parse complete header/path pairs; reject invalid structure, unsupported records, unmerged statuses, invalid UTF-8, and
unsafe repository-relative paths. Validate all records before returning a selection, including excluded entries. Accept
valid hexadecimal object IDs without assuming one fixed hash width. Handle terminal NULs deliberately; reject truncation
and unexpected extra fields. Sort and deduplicate the selected paths. Preserve tabs, newlines, spaces, and leading
hyphens in valid paths as data. Do not trim or line-split filenames.

Pure byte-fixture tests in this module prove eligible/excluded modes, additions/modifications/deletions, mode changes,
both type-change directions, rename-as-delete/add, mixed and duplicate records, empty output, unusual names, malformed
headers and delimiters, missing paths, unmerged records, invalid UTF-8, unsafe paths, and differing object-ID lengths.
They do not instantiate `DiskTree`, touch the filesystem, or launch Git. The new module stays inside the existing 99%
coverage denominator; do not add an exclusion or reduce the floor.

## Empty Mutations

In `src/v0_4/gates.rs`, after resolving/projecting inputs and computing the combined selected-path union, a mutation
declaring at least one `RegularFiles` input succeeds without calling either mutation port when that union is empty.
Apply the rule to both pre-commit `apply-index` and pull-request `verify-clean`. A mixed declaration with another
selected file still runs. Existing `files`-only mutations and all checks retain their current behavior.

Preserve existing snapshot containment, opaque unselected links, formatter-relative path resolution, and partial-staging
behavior. The opt-in changes the selected argv, not the full-index snapshot boundary.

## Specification Changes

AC-RF-01 through AC-RF-06 and the empty-check portion of AC-RF-07 become durable scenarios in the new feature below.
AC-RF-07's remaining legacy claim stays in existing unit and disk regression tests, verified by delivery item T4.
AC-RF-08 stays plan-level because it observes index blobs, raw links, and unstaged filesystem bytes together; T4 proves
it in real Git. AC-RF-09's malformed byte protocol stays in pure parser tests and a real unmerged-index test, proved by
T2 and T4. AC-RF-10 is structural schema/runtime testing in T1 and T5. Release/delivery outcomes AC-RF-11 and AC-RF-12
are operational criteria proved by R1–R4 and C1–C4, not behavior corpus scenarios.

### [N] `specs/behaviours/regular-files.feature`

```diff
+ Feature: Regular file gate inputs
+ Accept Git index and explicit-range regular-files bindings
+ Refuse unsupported regular-files sources before dispatch
+ Select regular destinations from the index snapshot
+ Select regular destinations from the immutable range head
+ Preserve regular-files argument and environment projections
+ Skip an empty regular-files mutation on pre-commit
+ Skip an empty regular-files mutation on pull-request
+ Run a mixed-input mutation when another file input selects a path
+ Run a regular-files check even when its selection is empty
```

- Preserve: no existing scenarios move or change; the entire current corpus and its binding/exemption inventory remain.
- Actors: maintainer for configuration validation; contributor for index dispatch; reviewer for immutable-range replay.
- Preconditions: the first two scenarios declare the selected/unsupported sources. Selection scenarios declare staged or
  range destination modes and contrasting current working-tree types. Projection declares both argv and environment use.
  Empty scenarios declare excluded-only selections; the mixed scenario adds a selected ordinary file input.
- Actions/outcomes: configuration validation accepts or refuses; gate invocation records the independently observed
  selected argv/environment, skipped child, or launched child appropriate to the scenario name.
- Bindings: `tests/unit/bindings.rs`, `tests/integration/bindings.rs`, `tests/e2e/bindings.rs`; every new scenario binds
  at unit without exemption. Extend shared vocabulary in `tests/support/steps.rs` and `tests/support/binding.rs`, with
  declared snapshot fixture state in `tests/support/world.rs` and recorder support in `tests/support/launcher.rs`.
- Drivers: `tests/unit/driver.rs` configures `MemoryTree` from declared snapshot data; `tests/integration/driver.rs` and
  `tests/e2e/driver.rs` materialize real isolated Git fixtures. `tests/support/sandbox.rs` owns shared disk fixture
  setup. Checks use recorder output; mutation spies record actual port calls without performing host work at the unit
  boundary.
- Proof: `cargo test --test unit`; add a focused `regular_files_contract` test to each higher adapter's `policy.rs` that
  passes only this feature's binding subset to the existing scenario runner. Run
  `cargo test --test integration regular_files` and `cargo test --test e2e regular_files`.

### [E] `specs/behaviours/README.md`

```diff
+ Map regular-files.feature as the explicit Git destination-mode input contract.
```

- Preserve: both existing feature map entries.
- Proof: `cargo xtask self-validate`.

### [E] `specs/architecture.md`

```diff
+ Components: lifecycle gate resolution asks the Tree port for regular-file Git selections.
+ Components: the private runtime git_paths module makes pure metadata decisions.
+ Boundaries: disk owns Git process execution; selection reads index or immutable head modes.
```

- Preserve: System Context and Containers views; existing validator, network, root, and host-process constraints.
- Update: Components runtime relationship and Boundaries explanation; no new system or container.
- Proof: inspect the as-built delta against `runtime`, `disk`, and `gates`, then run `cargo xtask self-validate`.

## Verification Design

Use split RED, GREEN, REFACTOR increments, each with recorded command, revision, expected result, and actual result. The
first scenario red must reject the unknown new kind, not fail because its steps lack bindings. Parser red tests exercise
wrong eligibility/refusal decisions before implementation. Empty-mutation reds must observe a child/port call or refusal
where the specified outcome is skip-success.

In `tests/integration/policy.rs`, construct a real repository with one partially staged regular source and nine
synthetic tracked compatibility links to ordinary sources. Stage link changes beside the source. Invoke the production
gate with a fixture formatter that records filenames and changes only its argv paths. Assert only regular names reach
it, index regular bytes change as expected, symlink modes and raw targets stay identical, and unrelated/unstaged working
bytes remain exact. Retain the existing selected-symlink refusal test under legacy `files` semantics.

Additional real Git tests prove immutable head selection despite different working-tree types, executable-bit and both
type changes, regular rename destinations, excluded deletions/gitlinks, and unmerged index refusal. Tests for child skip
on both surfaces use a child marker or failing child that would expose an accidental launch. `tests/e2e/policy.rs`
proves the new contract through process exit, streams, and fixture child observations. Keep full E2E scheduled.

Extend `tests/v0_4_schema.rs` for valid new-kind structures and runtime semantic source rejection. Regenerate the schema
through `cargo xtask schema`; prove exact bytes with `cargo xtask schema --check`. Run the unchanged quick gate once the
focused increments pass; broaden only for actual new failures or changed behavior.

All local compute commands use `./hippo run --class ephemeral --resource-tier <tier> --disk-path . -- <command>`:
`light` for narrow static checks, `standard` for focused tests/schema generation, and `heavy` for the quick gate or
release artifacts. Use a working supported Node environment for hooks. Commands here are prescribed execution steps, not
claims that they have already run.

## Release and Documentation

Prepare the next unused minor version, expected `0.13.0`, in `Cargo.toml` and `Cargo.lock`, without dependency changes.
Add the new version's prior-stable baseline `v0.12.0` to `.github/workflows/release.yml`; use that same baseline for the
screened generated-note preview. Preserve all existing release branches and platforms.

Update the configuration reference with eligibility, sources, projections, and empty-mutation behavior. The gate wiring
guide shows opt-in adoption without weakening legacy selection. The schema example demonstrates the new kind.
`README.md` remains an orientation summary; `CHANGELOG.md` states the resulting behavior. Run Docs Propagation for the
whole feature unit, with a terminal status, affected paths, and commands not exercised. Run Docs Quality Gate with
subject `all` before the release-preparation PR is ready. Read affected rules through Rules Propagation first and record
its terminal result, including a justified `no-change` when product specifications suffice.

After merge, verify ancestry and reconcile primary `main` to `origin/main` with `0 0` divergence. Release Cut runs only
from that clean primary checkout, with exact-revision quick/schema gates, screened notes/tag, native artifact tests,
four platform archives, schema, and checksums. Download published bytes and verify the manifest. Record native matrix
identity evidence for every platform and execute the matching local platform archive as well. Never replace a tag or
asset; a defective published release requires a new patch and a new pin.

## File Impact

Every expected implementation path is named below; a newly discovered path is recorded before mutation.

```text
src/v0_4/config.rs                    additive kind, sources, projections, inline tests
src/v0_4/gates.rs                     resolution, empty mutation, inline tests
src/runtime.rs                       port defaults, memory selections, inline tests
src/runtime/git_paths.rs              new private pure parser and byte-fixture tests
src/runtime/disk.rs                   Git metadata adapter and helper import
tests/v0_4_schema.rs                   structural parity and semantic refusal cases
tests/unit/bindings.rs                new feature scenario bindings
tests/unit/driver.rs                  declared in-memory snapshot setup
tests/integration/bindings.rs         new feature scenario bindings
tests/integration/driver.rs           real snapshot fixtures
tests/integration/policy.rs           index/range preservation and focused adapter run
tests/e2e/bindings.rs                 new feature scenario bindings
tests/e2e/driver.rs                   process-boundary snapshot fixtures
tests/e2e/policy.rs                   focused feature/process contract proof
tests/support/steps.rs                shared snapshot vocabulary
tests/support/binding.rs              shared observable assertions
tests/support/world.rs                declared snapshot fixture state
tests/support/launcher.rs             recording launcher/mutation spy support
tests/support/sandbox.rs              isolated Git snapshot fixture support
specs/behaviours/regular-files.feature new durable feature scenarios
specs/behaviours/README.md             new feature map entry
specs/architecture.md                 as-built runtime/Git selection relationships
schemas/repo-config/v2.schema.json    generated artifact
schemas/repo-config/v2.example.yml    opt-in example
docs/reference/v0-4-configuration.md  canonical input contract
docs/how-to/wire-rhino-into-your-gates.md opt-in formatter wiring
README.md                            brief capability orientation
CHANGELOG.md                         compatible release notes
Cargo.toml                           next minor version
Cargo.lock                           root package version only
.github/workflows/release.yml        explicit prior-stable note baseline
```

The plan-only unit creates this folder's six documents and edits `plans/in-progress/README.md`. Its execution record and
learnings evolve in later units. Final archival moves all six documents and updates `plans/in-progress/README.md` and
`plans/done/README.md`; update any discovered live inbound reference in the same transaction.

The post-write owner decision retains the one execution checkout through the archival PR merge, then removes that
checkout and its local/remote task branches. Record the final removal proof in the archival PR rather than editing the
archived plan. The separately owned portable governance correction must land in its canonical catalog before this plan's
final validation and plan-only delivery. This plan lands alone with explicit owner adoption pending. Next, sync the same
clean checkout and land the separate RHINO governance-adoption PR from that published source revision. The Plan Quality
Gate follows the adoption merge; implementation follows its recorded verdict and finding ownership. The delivery record
must cite the actual source and adoption commits; neither exists as a presumed placeholder. The separate adoption
delivery owns its exact file mapping, generated bindings, checks, and PR; it does not share the plan-only transaction or
direct another repository's mutations.
