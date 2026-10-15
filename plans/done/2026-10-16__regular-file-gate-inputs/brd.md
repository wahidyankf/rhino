# Business Requirements

## Need

A repository can track compatibility links alongside source files. The existing `files` input can select a staged
compatibility link, but the local mutation boundary refuses it before launching the formatter. This protects the
repository, but it leaves maintainers without a way to express the narrower formatter input they need.

Removing useful tracked links changes repository behavior. Weakening snapshot containment risks formatting through a
link. Maintainers need a supported way to retain the links and format eligible changed source files in one gate.

## Roles and Outcomes

- Repository maintainers can declare the files a formatter may receive without maintaining a separate path filter.
- Contributors can stage a regular source change beside compatibility links and complete the declared gate.
- Reviewers can replay the same policy using immutable commits, independent of the current checkout's file types.
- Release consumers can pin a verified artifact whose schema and runtime accept the same new input kind.

Success means the acceptance criteria in [the product requirements](prd.md) are proved, the compatible release is
published with verified assets, and RHINO's delivery reaches `origin/main` with primary `main` reconciled.

## Non-goals

This work does not change existing `files` semantics, follow selected links, add arbitrary file globs, introduce a
dependency, change release platforms, or edit consumers. It does not change ordinary validator process or write
boundaries. It retains partial-staging and snapshot containment guarantees.

## Risks

A selection based on the working tree can silently disagree with the index or reviewed commit. A formatter launched with
no filenames may widen its scope. An untested parser can misread unusual filenames or malformed metadata. Changing
`files` in place would alter existing contracts. The design addresses these risks through immutable metadata, explicit
opt-in, empty-mutation handling, and tests at both pure and host boundaries.

Public artifacts must use synthetic fixtures and describe generic behavior. Release publication remains immutable; an
artifact defect requires another version rather than replacing bytes consumers may already have pinned.
