//! Governance root structure.
//!
//! The governance tree has five layers and a shared registry of categories
//! inside them. A repository needing a category the registry does not provide
//! declares it in configuration rather than by creating a directory, because
//! inferring a category from a directory removes the only moment anyone
//! considers whether the category should exist -- and a typo, or a directory an
//! abandoned experiment left behind, becomes a layer of governance nobody
//! decided on.
//!
//! Unlike every other validator here, the root is not configuration. The shared
//! registry is a set of `<layer>/<category>` paths, so a layer name means
//! something only under one root; a configurable root would let two
//! repositories mean different things by `conventions/structure` while both
//! claiming to have adopted the same registry.

use crate::config::v2::{Document, LAYERS};
use crate::report::{Finding, Report};
use crate::runtime::Tree;
use std::collections::BTreeSet;

/// Where a repository keeps its governance, fixed by the shared contract.
pub const ROOT: &str = "repo-governance";

/// The categories the canonical catalog publishes.
///
/// "Useful across repositories" is not a judgement this tool can make, and the
/// catalog is the set every adopter actually receives -- which makes it the one
/// definition that is not somebody's taste. Anything else a repository wants is
/// a local category, which is exactly what `governance.local-categories` is
/// for.
const REGISTRY: [&str; 10] = [
    "conventions/security",
    "conventions/structure",
    "conventions/writing",
    "development/agents",
    "development/quality",
    "development/workflow",
    "workflows/adoption",
    "workflows/maintenance",
    "workflows/plan",
    "workflows/quality",
];

pub fn validate(tree: &dyn Tree, document: &Document) -> Report {
    let mut report = Report::new("governance-roots", "directory");
    let declared: BTreeSet<&str> = document
        .local_categories
        .iter()
        .map(String::as_str)
        .collect();

    let directories = tree.directories();
    let files = tree.files();
    let holds_a_file = |directory: &str| {
        let prefix = format!("{directory}/");
        files.iter().any(|path| path.starts_with(&prefix))
    };

    // A directory already named by the more specific rule is not named again by
    // the general one. Two findings about one directory would say the same
    // thing twice and leave a reader guessing which to act on.
    let mut reported: BTreeSet<String> = BTreeSet::new();

    // The declaration is checked before the tree, because a category declared
    // and never used is a fault in the configuration whether or not an empty
    // directory was left behind for it.
    for category in &document.local_categories {
        let path = format!("{ROOT}/{category}");
        if !holds_a_file(&path) {
            reported.insert(path);
            report.found(Finding::new(
                "unused-local-category",
                category,
                "is declared and holds nothing; a category exists because it holds something, and the registry does not oblige a repository to create one it does not use",
            ));
        }
    }

    for directory in &directories {
        let Some(relative) = directory.strip_prefix(&format!("{ROOT}/")) else {
            continue;
        };
        report.inspected_one();
        let mut segments = relative.split('/');
        // Total: a stripped path holds at least one segment.
        let layer = segments.next().unwrap_or_default();

        if !LAYERS.contains(&layer) {
            // Reported once, at the layer. A finding for every document under
            // an unrecognized layer would bury the one decision that has to be
            // made about it.
            if relative == layer {
                reported.insert(directory.clone());
                report.found(Finding::new(
                    "unknown-governance-layer",
                    directory,
                    format!(
                        "is not a governed layer; the layers are {}",
                        crate::config::listed(&LAYERS)
                    ),
                ));
            }
            continue;
        }

        if let Some(category) = segments.next()
            && segments.next().is_none()
            // A directory with a sibling document of the same name is that
            // document's companion set, not a category. The two are
            // indistinguishable by depth, and the sibling is what the naming
            // contract already uses to tell them apart.
            && !tree.exists(&format!("{ROOT}/{layer}/{category}.md"))
        {
            let named = format!("{layer}/{category}");
            if !REGISTRY.contains(&named.as_str()) && !declared.contains(named.as_str()) {
                reported.insert(directory.clone());
                report.found(Finding::new(
                    "undeclared-governance-category",
                    directory,
                    "is not a registered category, and this repository declares no local category of that name under `governance.local-categories`",
                ));
                continue;
            }
        }

        // The innermost empty directory, not every directory above it. A layer
        // whose only content is an empty category holds no file either, and
        // naming both would report one absence twice while pointing the reader
        // at the directory that is not the one to remove.
        let holds_a_directory = directories
            .iter()
            .any(|held| held.starts_with(&format!("{directory}/")));
        if !holds_a_file(directory) && !holds_a_directory && !reported.contains(directory) {
            report.found(Finding::new(
                "empty-governed-directory",
                directory,
                "holds no file; a directory exists because it holds something, and an empty one is a promise about future content rather than content",
            ));
        }
    }

    report
}
