# Architecture Specifications

`specs/architecture.md` holds the C4 model of this repository. It is a specification, not a diagram gallery: it states the boundaries the code is required to have.

## Requirements

- Keep it synchronized with the as-built structure. A boundary that moved and a document that did not are a defect in the same change, not a follow-up.
- Assess it for impact before every change, alongside the behaviour corpus, and record a verified no-op rather than editing it because a change was nearby.
- Describe boundaries and responsibilities, not file layout. A rename that moves no responsibility changes nothing here.
- Diagrams follow the [Mermaid conventions](../conventions/markdown-visualizations.md): the declared palette, the 32-grapheme node limit, and the 24-grapheme edge limit. The repository's own validator checks this file like any other.
- Every boundary drawn here should correspond to something a test can observe. A layer the tests cannot distinguish is a diagram, not an architecture.

## What Belongs Here

Contexts, containers, components, and the direction of dependency between them. The read-only, network-free, process-free, path-contained boundary of the product belongs here as structure, and in [software quality enforcement](software-quality-enforcement.md) as the rule tests enforce.

## What Does Not

Implementation detail, module inventories that duplicate the source tree, and anything `docs/` explains better to a consumer. `specs/` describes what this repository must be; `docs/` describes what a user can do with it.

## Related

- [Specification maintenance](specification-maintenance.md)
