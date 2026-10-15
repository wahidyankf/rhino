# Product Requirements

## Users and Scope

A maintainer wants changed regular source paths passed to a declared command while preserving tracked links. A
contributor wants staged formatting to preserve unstaged edits. A reviewer wants selection derived from the reviewed
range rather than the current working tree. A release consumer wants the editor schema and executable to agree.

The capability is an additive input kind in `rhino/repo-config/v2`. It has no new CLI flag, output format, or exit code.
Ordinary check gates continue launching their declared command even when their selected file list is empty.

## Acceptance Criteria

### AC-RF-01 — Explicit input contract

```gherkin
Given a gate declares a regular-files input with a Git index or explicit-range binding
When the maintainer validates its configuration
Then the declaration is accepted
```

### AC-RF-02 — Unsupported sources refuse

```gherkin
Given a regular-files input is bound to checkout, hook-message-file, or push-updates
When the maintainer validates its configuration
Then the declaration is refused as incompatible configuration
```

### AC-RF-03 — Destination modes determine eligibility

```gherkin
Given changed entries include regular files, executable files, symlinks, gitlinks, and deletions
When a command receives a regular-files path projection
Then it receives only changed paths whose destination is a regular or executable file
```

This includes regular rename destinations, executable-mode changes, and link-to-regular changes; it excludes
regular-to-link changes. The output is sorted and duplicate-free.

### AC-RF-04 — The snapshot owns selection

```gherkin
Given working-tree file types disagree with the selected index or immutable range head
When a command receives the regular-files projection
Then its path selection follows the selected Git snapshot
```

### AC-RF-05 — Existing projections remain available

```gherkin
Given a regular-files input resolves to changed regular paths
When the declared command projects its paths into repeated arguments and environment
Then it receives the same path values through the existing projection contracts
```

### AC-RF-06 — Empty mutations do not launch children

```gherkin
Given a mutation declares regular-files and its combined selected file paths are empty
When the mutation runs on pre-commit or pull-request
Then it succeeds without launching its declared child
```

If another file input selects a path, the combined selection is nonempty and the mutation still runs.

### AC-RF-07 — Legacy behavior remains stable

```gherkin
Given a gate uses only the existing files input kind
When the gate runs
Then its existing selection and empty-input behavior are preserved
```

The existing local selected-symlink refusal remains a regression requirement. Existing check gates, including checks
declaring `regular-files`, retain child invocation when their resolved file selection is empty.

### AC-RF-08 — Staged formatting preserves repository state

```gherkin
Given a staged regular source and nine synthetic tracked compatibility symlinks
And the source has a separate unstaged edit
When a regular-files mutation formats the index snapshot
Then only selected regular source blobs change in the index
```

The proof separately asserts unchanged symlink modes and raw targets, unchanged unstaged bytes, and unchanged unrelated
working files. It must inspect actual Git and filesystem state, not fixture copies.

### AC-RF-09 — Unusable metadata fails closed

```gherkin
Given the Git selection metadata is malformed, unmerged, non-UTF-8, or path-escaping
When the regular-files selection is resolved
Then resolution refuses before a child is launched
```

Valid names containing whitespace, tabs, newlines, or leading hyphens remain intact.

### AC-RF-10 — Schema and runtime agree

```gherkin
Given the generated schema and typed runtime configuration model
When regular-files declarations are validated
Then their structural acceptance agrees and runtime source validation remains explicit
```

### AC-RF-11 — A verified compatible release is available

```gherkin
Given the accepted implementation and release preparation have reached origin/main
When the authorized Release Cut completes
Then the new immutable release has four native platform archives, its schema, and verified checksums
```

Each executable's version and commit match the tag, with native identity proof supplied by the release matrix.

### AC-RF-12 — Owner delivery is complete

```gherkin
Given all substantive owner delivery units have merged
When delivery reconciliation runs
Then task commits are reachable from origin/main and primary main has no divergence
```

The evidence is a successful ancestry check and `git rev-list --left-right --count HEAD...origin/main` returning `0 0`
from primary `main`. Learning disposition and execution review precede the archival delivery. The one execution checkout
survives through its merge; final checkout/branch cleanup follows, with proof recorded in that PR.

## Product Risks and Verification Boundaries

Metadata decisions need pure tests; snapshot preservation needs real Git tests. A passing schema alone cannot establish
semantic source compatibility. The non-interactive CLI's observable interface is exit status, streams, child argv, and
repository state. Visual, browser, and interactive device checks are not applicable; unit, integration, and targeted
process evidence establish those claims instead.
