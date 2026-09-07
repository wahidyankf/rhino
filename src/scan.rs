//! Choosing what to look at.
//!
//! Every validator that walks the repository walks it the same way, so the
//! answer to "was this file inspected?" does not depend on which command asked.
//! The exclusion list is the consuming repository's, matched by directory
//! *name* at any depth rather than by prefix, because a build output directory
//! is a build output directory wherever it appears.

use crate::config::Config;
use crate::runtime::Tree;

/// Every Markdown file a validator may look at.
pub fn markdown_files(tree: &dyn Tree, config: &Config) -> Vec<String> {
    tree.files()
        .into_iter()
        .filter(|path| path.ends_with(".md"))
        .filter(|path| !is_excluded(path, &config.scan.exclude_directories))
        .collect()
}

/// Whether any directory on the way to a file is one the repository excludes.
///
/// The file's own name is never tested, so a file called `.git` is inspected
/// while everything inside a directory called `.git` is not.
fn is_excluded(path: &str, excluded: &[String]) -> bool {
    let mut segments: Vec<&str> = path.split('/').collect();
    segments.pop();
    segments
        .iter()
        .any(|segment| excluded.iter().any(|name| name == segment))
}
