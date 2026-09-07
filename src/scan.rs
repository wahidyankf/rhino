//! Choosing what to look at, and reading it once.
//!
//! Every validator that walks the repository walks it the same way, so the
//! answer to "was this file inspected?" does not depend on which command asked.
//! The exclusion list is the consuming repository's, matched by directory
//! *name* at any depth rather than by prefix, because a build output directory
//! is a build output directory wherever it appears.
//!
//! Reading is centralised for a second reason beyond cost: an unreadable file
//! has to refuse the whole run with exit `2`, and one implementation of that
//! refusal is one place for it to be right. A validator that quietly skipped a
//! file it could not open would report a clean repository it never read.

use crate::config::Config;
use crate::runtime::{Tree, TreeError};
use globset::{GlobBuilder, GlobSet, GlobSetBuilder};

/// One Markdown file, read.
pub struct Document {
    pub path: String,
    pub text: String,
}

/// Every Markdown file a validator may look at, read once.
pub struct Corpus {
    documents: Vec<Document>,
}

impl Corpus {
    /// Read the repository's Markdown, in sorted path order.
    ///
    /// Order is part of the contract rather than an accident of the walk:
    /// findings are reported in the order files are read, and a report whose
    /// order changed between runs could not be diffed.
    pub fn read(tree: &dyn Tree, config: &Config) -> Result<Self, String> {
        let mut documents = Vec::new();

        for path in markdown_files(tree, config) {
            match tree.read(&path) {
                Ok(text) => documents.push(Document { path, text }),
                // A file that vanished between the walk and the read is not
                // this repository's policy being violated.
                Err(TreeError::NotFound) => {}
                Err(TreeError::Unreadable(reason)) => return Err(format!("{path}: {reason}")),
            }
        }

        Ok(Self { documents })
    }

    pub fn documents(&self) -> &[Document] {
        &self.documents
    }

    /// The documents a validator treats as sources, given what it excludes.
    ///
    /// Exclusion is a per-validator question and never removes a file from the
    /// corpus: a document excluded as a *source* is still a perfectly valid
    /// link target and still a file another validator may inspect.
    pub fn sources<'a>(&'a self, excluded: &'a GlobSet) -> impl Iterator<Item = &'a Document> {
        self.documents
            .iter()
            .filter(move |document| !excluded.is_match(&document.path))
    }
}

/// Every Markdown path a validator may look at.
pub fn markdown_files(tree: &dyn Tree, config: &Config) -> Vec<String> {
    tree.files()
        .into_iter()
        .filter(|path| is_markdown(path))
        .filter(|path| !is_excluded(path, &config.scan.exclude_directories))
        .collect()
}

/// Whether a path names a Markdown file.
///
/// Case-insensitive on the extension, because `README.MD` is a Markdown file
/// that a repository will eventually contain and that a reader would be
/// surprised to see skipped.
fn is_markdown(path: &str) -> bool {
    path.rsplit_once('.')
        .is_some_and(|(_, extension)| extension.eq_ignore_ascii_case("md"))
}

/// Compile a declared list of globs, naming the configuration key in any
/// failure so a bad pattern is a configuration fault a reader can locate.
///
/// Matching is case-insensitive for the same reason the extension check is: a
/// surface declared as `rules/**/*.md` is meant to cover the Markdown under
/// `rules`, and whether one file shouts its extension is not a policy decision
/// the repository made.
pub fn glob_set(key: &str, patterns: &[String]) -> Result<GlobSet, String> {
    let mut builder = GlobSetBuilder::new();
    for pattern in patterns {
        let glob = GlobBuilder::new(pattern)
            .case_insensitive(true)
            .build()
            .map_err(|error| format!("{key}: `{pattern}`: {error}"))?;
        builder.add(glob);
    }
    builder.build().map_err(|error| format!("{key}: {error}"))
}

/// Whether any directory on the way to a file is one the repository excludes.
///
/// The file's own name is never tested, so a file called `.git` is inspected
/// while everything inside a directory called `.git` is not.
pub fn is_excluded(path: &str, excluded: &[String]) -> bool {
    let mut segments: Vec<&str> = path.split('/').collect();
    segments.pop();
    segments
        .iter()
        .any(|segment| excluded.iter().any(|name| name == segment))
}
