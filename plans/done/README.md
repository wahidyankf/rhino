# Done

This stage preserves completed plans as historical delivery records. A completed folder is named `YYYY-MM-DD__<slug>`, where the date is the completion date rather than the creation date, so the directory sorts by when work landed.

A record here is history, not architecture. It deliberately describes a repository as it was, and it may reference paths that have since moved. That is why [`repo-config.yml`](../../repo-config.yml) excludes `plans/done/**` as a link _source_: an archived plan stays a valid link target, but reporting its outward links would make finishing a plan produce findings. For what the binary does today, read [`specs/`](../../specs/README.md).

Before archiving, reconcile required and conditional delivery, acceptance, verification, and learnings. [Plan execution](../../repo-governance/workflows/plan-execution.md) owns the move: it refuses an existing destination rather than merging, overwriting, or adding a suffix; records completion metadata and outcomes; moves the folder from [`../in-progress/`](../in-progress/README.md); updates both stage indexes and maps in one change; and then verifies the archive itself.

## Completed Plans

No plan has been completed in this repository yet.

## Directory Map

This stage holds no plan folder, so this README has no siblings to map.
