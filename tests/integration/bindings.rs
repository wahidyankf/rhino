//! Integration bindings.
//!
//! The same scenarios, driven against a real temporary directory rather than an
//! in-memory tree, so the filesystem port's concrete implementation is exercised
//! by the same sentences.

use crate::registry::{Binding, Exemption};

pub const BINDINGS: &[Binding] = &[
    ("v0-4-contract", "A grouped v0.4 configuration is accepted"),
    (
        "v0-4-contract",
        "A grouped configuration retains each live documentation validator",
    ),
    (
        "v0-4-contract",
        "An omitted grouped documentation policy refuses at its owner",
    ),
    ("v0-4-contract", "An unknown grouped-core key is refused"),
    (
        "v0-4-contract",
        "An extension owner outside the portable namespace is refused",
    ),
    (
        "v0-4-contract",
        "A predecessor configuration is rejected after stable retirement",
    ),
    (
        "v0-4-contract",
        "The RC-only migration command is unknown after stable retirement",
    ),
    (
        "v0-4-contract",
        "A retired legacy command is unknown after stable retirement",
    ),
    (
        "v0-4-contract",
        "Lifecycle identities are listed declaratively",
    ),
    (
        "v0-4-contract",
        "Each declared lifecycle membership uses one closed surface",
    ),
    (
        "v0-4-contract",
        "A legacy ci lifecycle membership is refused",
    ),
    (
        "v0-4-contract",
        "Pull-request exact composition includes every local gate",
    ),
    (
        "v0-4-contract",
        "Pull-request at-least extras name their direct reason",
    ),
    (
        "v0-4-contract",
        "Pull-request at-least extras retain their direct reason",
    ),
    (
        "v0-4-contract",
        "Duplicate semantic lifecycle IDs are refused",
    ),
    (
        "v0-4-contract",
        "Generic gate inputs are validated before dispatch",
    ),
    ("v0-4-contract", "Tool-named input selectors are refused"),
    (
        "v0-4-contract",
        "A pull-request file input declares an explicit immutable range",
    ),
    (
        "v0-4-contract",
        "Typed argv and environment projections name resolved range fields",
    ),
    (
        "v0-4-contract",
        "A pre-push range declares its new-ref fallback",
    ),
    (
        "v0-4-contract",
        "Unresolved typed input projections are refused",
    ),
    (
        "v0-4-contract",
        "Typed bindings reject a source for the wrong input kind",
    ),
    (
        "v0-4-contract",
        "Surface membership cannot replace a declared command",
    ),
    (
        "v0-4-contract",
        "A mutation gate declares paired local and pull-request behavior",
    ),
    (
        "v0-4-contract",
        "A mutation gate cannot omit its pull-request replay contract",
    ),
    (
        "v0-4-contract",
        "A mutation refuses a mutable-working-tree fallback",
    ),
    (
        "v0-4-contract",
        "Harness adapter validation refuses an undeclared grouped profile",
    ),
    (
        "v0-4-contract",
        "Typed harness profiles render native agent and skill adapters",
    ),
    (
        "v0-4-contract",
        "A canonical denial removes native member projections",
    ),
    (
        "v0-4-contract",
        "Environment backup starts with an explicit destination plan",
    ),
    (
        "v0-4-contract",
        "Environment backup refuses a destination outside its repository",
    ),
    (
        "v0-4-contract",
        "Environment restore with no declared target completes with a zero count",
    ),
    (
        "v0-4-contract",
        "Environment initialization reports the targets it created",
    ),
    (
        "v0-4-contract",
        "Environment backup with no eligible file completes with a zero count",
    ),
    (
        "v0-4-contract",
        "Environment backup reports the copy it completed",
    ),
    (
        "v0-4-contract",
        "Text and JSON gate listings share a result envelope",
    ),
    ("v0-4-contract", "Retired gate aliases are unknown"),
    ("v0-4-contract", "Retired gate audit alias is unknown"),
    (
        "v0-4-contract",
        "Conventional Commit input follows the commit-message lifecycle",
    ),
    (
        "v0-4-contract",
        "Hook message input is exclusive to Git's hook boundary",
    ),
    (
        "v0-4-contract",
        "Lifecycle composition is checked before a gate runs",
    ),
    (
        "v0-4-contract",
        "Pull-request replay receives an explicit immutable range",
    ),
    (
        "v0-4-contract",
        "An omitted lifecycle group is refused rather than read as empty",
    ),
    (
        "v0-4-contract",
        "Harness adapter generation refuses an undeclared grouped profile",
    ),
    (
        "v0-4-contract",
        "Grouped harness adapters refuse an inherited profile selector",
    ),
    (
        "v0-4-contract",
        "Canonical adapters generate once and then become a deterministic no-op",
    ),
    (
        "v0-4-contract",
        "An unrepresentable canonical requirement refuses before adapter generation",
    ),
    (
        "v0-4-contract",
        "Agent adapters project a declared canonical list",
    ),
    (
        "v0-4-contract",
        "An adapter list naming a disallowed key refuses",
    ),
    (
        "v0-4-contract",
        "An adapter without lists renders unchanged",
    ),
    (
        "v0-4-contract",
        "A profile renders only its selected agents",
    ),
    (
        "v0-4-contract",
        "A selected agent without a canonical source refuses",
    ),
    ("v0-4-contract", "A file outside the selection is stale"),
    (
        "v0-4-contract",
        "Toolchain provisioning requires an explicit apply contract",
    ),
    (
        "v0-4-contract",
        "Toolchain provisioning plans before an explicit mutation",
    ),
    (
        "v0-4-contract",
        "Toolchain provisioning plans as one JSON object",
    ),
    (
        "v0-4-contract",
        "Toolchain validation reports a required unavailable probe without output bytes",
    ),
    (
        "v0-4-contract",
        "Curated environment detection accepts a declared Rust key",
    ),
    (
        "v0-4-contract",
        "Curated environment detection accepts a declared TypeScript schema key",
    ),
    (
        "v0-4-contract",
        "Curated environment detection accepts an injected Go lookup key",
    ),
    (
        "v0-4-contract",
        "Curated environment detection redacts a dynamic injected Go lookup",
    ),
    (
        "v0-4-contract",
        "Curated environment detection redacts unsupported dynamic access",
    ),
    (
        "v0-4-contract",
        "Environment validation reports an explicitly empty policy clean",
    ),
    (
        "v0-4-contract",
        "Environment initialization refuses a policy with no targets",
    ),
    (
        "v0-4-contract",
        "Environment initialization refuses a symbolic-link source before writing",
    ),
    (
        "v0-4-contract",
        "Environment initialization plans as one JSON object",
    ),
    (
        "v0-4-contract",
        "License policy is repository-configured rather than OSE-defined",
    ),
    (
        "v0-4-contract",
        "License policy reports only its configured identifier mismatch",
    ),
    (
        "v0-4-contract",
        "License policy reports a configured digest mismatch",
    ),
    (
        "v0-4-contract",
        "README index requires only declared direct-child links and annotations",
    ),
    (
        "v0-4-contract",
        "README index reports an omitted declared direct child",
    ),
    (
        "v0-4-contract",
        "README index excludes only a declared direct-child subtree",
    ),
    (
        "v0-4-contract",
        "README index refuses an escaping exclusion",
    ),
    (
        "v0-4-contract",
        "Grouped governance word budgets retain declared scan exclusions",
    ),
    (
        "v0-4-contract",
        "Grouped scan exclusions preserve Markdown validator scope",
    ),
    (
        "v0-4-contract",
        "Grouped governance directory maps retain their declared trees",
    ),
    (
        "v0-4-contract",
        "Vendor policy permits only an exact declared vocabulary exception",
    ),
    (
        "v0-4-contract",
        "Vendor policy reports an undeclared forbidden term",
    ),
    (
        "v0-4-contract",
        "Vendor policy refuses an escaping exception path",
    ),
    (
        "v0-4-contract",
        "Layer policy validates only the declared layer order and categories",
    ),
    (
        "v0-4-contract",
        "Layer policy reports a missing declared category",
    ),
    ("v0-4-contract", "Layer policy reports an undeclared layer"),
    ("v0-4-contract", "Layer policy refuses an escaping category"),
    (
        "v0-4-contract",
        "Traceability policy validates a declared artifact relationship",
    ),
    (
        "v0-4-contract",
        "Traceability policy reports an omitted declared relationship",
    ),
    (
        "v0-4-contract",
        "Traceability policy reports a missing declared artifact",
    ),
    (
        "v0-4-contract",
        "Frontmatter policy accepts a declared required key",
    ),
    (
        "v0-4-contract",
        "Frontmatter policy reports an absent declared key",
    ),
    (
        "v0-4-contract",
        "Internal-link policy accepts a declared local target",
    ),
    (
        "v0-4-contract",
        "Internal-link policy reports a missing local target",
    ),
    (
        "v0-4-contract",
        "An omitted policy is refused as undeclared configuration",
    ),
    (
        "v0-4-contract",
        "A selected file that does not exist is refused as missing",
    ),
    (
        "v0-4-contract",
        "A selected file that cannot be opened is refused as unreadable",
    ),
    (
        "v0-4-contract",
        "A selected directory that does not exist is refused as missing",
    ),
    (
        "v0-4-contract",
        "An inspected file that cannot be opened is refused as unreadable",
    ),
    (
        "v0-4-contract",
        "Environment initialization reports a target it cannot write",
    ),
    (
        "v0-4-contract",
        "Environment backup reports a destination it cannot write",
    ),
    (
        "v0-4-contract",
        "Environment restore reports a target it cannot write",
    ),
    (
        "v0-4-contract",
        "An unknown lifecycle surface is an unrecognized option value",
    ),
    (
        "v0-4-contract",
        "A range bound that names no commit is an unrecognized option value",
    ),
    (
        "v0-4-contract",
        "A range input the repository cannot answer is refused as an unusable repository",
    ),
    (
        "v0-4-contract",
        "A hook message file outside Git's hook boundary is refused as unreadable",
    ),
    (
        "v0-4-contract",
        "An inconsistent toolchain policy is refused as unusable configuration",
    ),
    (
        "v0-4-contract",
        "A staging guard in a repository with no Git index is refused as an unusable repository",
    ),
    (
        "v0-4-contract",
        "Internal-link policy reports a target behind a symbolic link as outside the repository",
    ),
    (
        "v0-4-contract",
        "A declared file behind a symbolic link is refused rather than read",
    ),
    (
        "v0-4-contract",
        "A selected file behind a symbolic link escapes the repository root",
    ),
    (
        "v0-4-contract",
        "A selected directory behind a symbolic link escapes the repository root",
    ),
    (
        "v0-4-contract",
        "A declared directory-map tree outside the repository root is refused",
    ),
    (
        "v0-4-contract",
        "A declared glob outside the repository root is refused",
    ),
    (
        "v0-4-contract",
        "A declared directory-map tree behind a symbolic link is refused rather than skipped",
    ),
    (
        "v0-4-contract",
        "A declared vendor root behind a symbolic link is refused rather than skipped",
    ),
    (
        "v0-4-contract",
        "A declared layer root behind a symbolic link is refused rather than skipped",
    ),
    (
        "v0-4-contract",
        "A declared environment source behind a symbolic link is refused rather than reported unread",
    ),
    (
        "v0-4-contract",
        "Mermaid policy accepts a declared plain-text repository with no diagrams",
    ),
    (
        "v0-4-contract",
        "Mermaid policy reports a diagram forbidden by its authoring rule",
    ),
];

/// Bindings this layer legitimately does not have. Each must name the concrete
/// boundary it cannot reach and the alternative proof that covers it.
pub const EXEMPTIONS: &[Exemption] = &[];
