# Docs Propagation

Carry one change into every human-facing document it affects, in one bounded pass. Apply this workflow automatically before committing a change that alters what a document's reader relies on, when a document is added, moved, or deleted, or when an explicitly requested [docs quality gate](docs-quality-gate.md) returns `NEEDS_PROPAGATION`. No separate instruction is required. Propagation is the only writer of documents; edits inside one run start no second one.

## The Document Set

Every `README.md`, the `docs/` and `specs/` trees, the root `CHANGELOG.md`, and a plan's documents where they describe the repository. Governance, `AGENTS.md`, and agent and skill instructions stay with [rules propagation](rules-propagation.md). Formatting, links, directory maps, and word budgets stay with the checks `cargo xtask self-validate` already runs; this workflow runs them and adds none.

## Inputs

Freeze the change — a revision range or the working-tree change — any handed-over gate ledger, the Git revision, and the uncommitted paths. A material external change returns `INPUT_CHANGED`; it never restarts the run.

## Procedure

1. **Find what went stale.** Search the whole document set for every name, path, command, flag, exit code, configuration key, and version the change removed, renamed, or redefined. Each ledger row is an item too.
2. **Remove what is obsolete.** A document describing something the repository no longer has is deleted, with every link and directory-map entry pointing at it. Unique meaning that is still true moves to its canonical home first.
3. **Keep each fact in one home.** The root `README.md` orients; each directory `README.md` maps its directory under [directory maps](../conventions/directory-maps.md); a `docs/` page serves one Diátaxis category under [documentation architecture](../conventions/documentation-architecture.md). A summary links one level down to its detail, under [progressive disclosure](../principles/progressive-disclosure.md), and a fact with a canonical home is linked, never copied.
4. **Write for a newcomer.** Each affected document tells a reader new to this repository what it is and why it matters from its opening, shows the next step without assuming the layout, and leaves no undefined term or skipped prerequisite, in the plain English of [language](../conventions/language.md). Judge this by reading, never by a readability score. A sparing emoji may mark meaning where the policy in `repo-config.yml` permits one; decoration never does.
5. **Run what is safe to run.** Execute every command and example an affected document shows, through `./hippo`, under [truthfulness](../conventions/documentation-architecture.md#truthfulness). Never run one that touches a production or shared system, publishes, spends, needs a secret, or cannot be undone; the document says plainly that it was not exercised.
6. **Treat specifications as canonical.** Refresh their readability, navigation, and links. A document that contradicts `specs/` is repaired to match it. When a specification disagrees with the implementation, that document stays unchanged and the owner is asked through [grill-me](../../.agents/skills/grill-me/SKILL.md), under [last-resort questions](../conventions/last-resort-questions.md); the rest lands.
7. **Change only what is stale, missing, or obsolete.** Never rewrite accurate prose, invent behaviour, or fold in unrelated work.
8. **Verify once.**

   ```sh
   ./hippo run --class ephemeral --resource-tier standard --disk-path . -- cargo xtask self-validate
   ```

   Repair only failures this run caused, and rerun only while the count of failing checks strictly decreases and no new failure class appears.

9. **Commit with the change it explains,** under [thematic commits](../conventions/thematic-commits.md). A handed-over ledger's repairs land as their own commit.

## Terminal Contract

The only results are `NO_CHANGE`, `LANDED`, `PARTIAL`, and `INPUT_CHANGED`. Each reports the updated documents, the removed documents, and every command left unexecuted with its reason. `PARTIAL` names each document held for the owner's answer. A rerun on unchanged inputs changes nothing. Passing authorizes neither commit nor push; [commit authorization](../conventions/commit-authorization.md) still applies.
