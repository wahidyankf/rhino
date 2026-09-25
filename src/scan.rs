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
use std::collections::BTreeSet;

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
                // Refusal follows the claim. This walk selected the file
                // because its name says Markdown; a name that says text over
                // bytes that are not is a fault to report, not to work around.
                Err(TreeError::NotText) => return Err(format!("{path}: holds no text")),
            }
        }

        Ok(Self { documents })
    }

    /// The documents, owned, for a caller that has to hold them beside a
    /// selection built some other way.
    pub fn into_documents(self) -> Vec<Document> {
        self.documents
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

/// Every path a validator may look at.
///
/// Filesystem links are already absent: the port never reports one, because
/// following one can leave the repository.
pub fn files(tree: &dyn Tree, config: &Config) -> Vec<String> {
    tree.files()
        .into_iter()
        .filter(|path| !is_excluded(path, config.excluded()))
        .collect()
}

/// Every Markdown path a validator may look at.
pub fn markdown_files(tree: &dyn Tree, config: &Config) -> Vec<String> {
    tree.files()
        .into_iter()
        .filter(|path| is_markdown(path))
        .filter(|path| !is_excluded(path, config.excluded()))
        .collect()
}

/// Whether a path names a Markdown file.
///
/// Case-insensitive on the extension, because `README.MD` is a Markdown file
/// that a repository will eventually contain and that a reader would be
/// surprised to see skipped.
pub fn is_markdown(path: &str) -> bool {
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
pub fn glob_set<'a>(
    key: &str,
    patterns: impl IntoIterator<Item = &'a str>,
) -> Result<GlobSet, String> {
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

/// The file a directory documents itself in.
///
/// Named here rather than in each validator that looks for it, because two
/// already do and they have to be looking for the same file.
pub const README: &str = "README.md";

/// Every directory at or under a tree, in path order.
///
/// Derived from the files rather than from a directory listing, because the
/// port's read-only tree exposes files and a directory that holds nothing is
/// not a directory a README can be missing from.
pub fn directories(tree: &dyn Tree, config: &Config, root: &str) -> Vec<String> {
    let prefix = format!("{}/", root.trim_end_matches('/'));
    let mut found: BTreeSet<String> = BTreeSet::new();

    for path in tree.files() {
        if !path.starts_with(&prefix) {
            continue;
        }
        if is_excluded(&path, config.excluded()) {
            continue;
        }
        let mut segments: Vec<&str> = path.split('/').collect();
        segments.pop();
        while segments.len() >= prefix.matches('/').count() {
            found.insert(segments.join("/"));
            segments.pop();
        }
    }

    found.into_iter().collect()
}

/// An ordered list of declared surface globs.
///
/// Two sections already state the same rule -- ordered globs where the last
/// matching entry wins -- so the rule lives here once rather than in each of
/// them. A third section stating it the other way round would be a repository
/// policy that changed meaning depending on which command read it.
pub struct Surfaces(GlobSet);

impl Surfaces {
    pub fn compile<'a>(
        key: &str,
        globs: impl IntoIterator<Item = &'a str>,
    ) -> Result<Self, String> {
        glob_set(key, globs).map(Self)
    }

    /// The index of the surface that governs a path, or `None` when no declared
    /// surface covers it.
    ///
    /// The *last* match rather than the first: where two surfaces cover one
    /// file, the later declaration is the more specific intent, which is how a
    /// repository says "this tree, except that one file".
    pub fn governing(&self, path: &str) -> Option<usize> {
        self.0.matches(path).into_iter().max()
    }
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

/// What an invocation narrowed itself to.
///
/// Passed to the leaves that accept a narrowing flag rather than read from the
/// command line by each of them, so "the declared surface" and "the surface I
/// was pointed at" are one decision made once.
#[derive(Debug, Clone, Default)]
pub struct Scope {
    /// Paths given with `--file`. `-` stands for standard input.
    pub files: Vec<String>,
    pub directory: Option<String>,
    /// What `--file -` should read, when a caller supplied it.
    pub stdin: Option<String>,
}

/// The path a document read from standard input is reported under.
pub const STDIN: &str = "-";

/// Why a selected path yielded no document.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Unselectable {
    /// The path passes through a filesystem link, which RHINO never follows,
    /// so it names something outside the repository root.
    Escapes,
    /// The path names no file.
    Missing,
    /// The path names a file that could not be read.
    Unreadable,
}

impl Unselectable {
    /// The refusal a leaf reports for a selection it could not read.
    ///
    /// One place, so every leaf that accepts `--file` answers a bad selection
    /// with the same code and the same words.
    pub fn refusal(self, category: &'static str, path: &str) -> crate::report::Report {
        let (code, message) = match self {
            Self::Escapes => (
                crate::errors::ErrorCode::PathEscapesRoot,
                format!("{path}: passes through a symbolic link, which RHINO never follows"),
            ),
            Self::Missing => (
                crate::errors::ErrorCode::FileMissing,
                format!("{path}: does not exist"),
            ),
            Self::Unreadable => (
                crate::errors::ErrorCode::FileUnreadable,
                format!("{path}: cannot be read"),
            ),
        };
        crate::report::Report::refused_as(code, category, message)
    }
}

impl Scope {
    pub fn is_narrowed(&self) -> bool {
        !self.files.is_empty()
    }

    /// The documents this scope selects, in the order they were given.
    ///
    /// A path that cannot be read is *not* skipped: it is returned with the
    /// reason so the caller reports it, because a selection naming a file that
    /// is not there is a mistake in the invocation and not an empty repository.
    pub fn documents(&self, tree: &dyn Tree) -> Vec<(String, Result<String, Unselectable>)> {
        self.files
            .iter()
            .map(|path| {
                if path == STDIN {
                    let text = self.stdin.clone().ok_or(Unselectable::Unreadable);
                    (STDIN.to_string(), text)
                } else if tree.passes_through_link(path) {
                    (path.clone(), Err(Unselectable::Escapes))
                } else {
                    let text = tree.read(path).map_err(|error| match error {
                        TreeError::NotFound => Unselectable::Missing,
                        TreeError::Unreadable(_) | TreeError::NotText => Unselectable::Unreadable,
                    });
                    (path.clone(), text)
                }
            })
            .collect()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::runtime::MemoryTree;

    #[test]
    fn corpus_scope_and_directory_walks_distinguish_read_state_and_declared_exclusions() {
        let mut tree = MemoryTree::default();
        tree.write("docs/README.MD", "# docs");
        tree.write("docs/nested/guide.md", "# guide");
        tree.write("generated/ignored.md", "# ignored");
        tree.write("docs/gone.md", "# gone");
        tree.mark_vanished("docs/gone.md");
        let config = Config {
            scan: Some(crate::config::Scan {
                exclude_directories: vec!["generated".to_string()],
            }),
            ..Config::default()
        };
        assert!(Corpus::read(&tree, &config).is_ok());
        assert!(directories(&tree, &config, "docs").contains(&"docs/nested".to_string()));
        assert!(is_markdown("docs/README.MD"));
        assert!(!is_markdown("docs/readme"));
        assert!(is_excluded("generated/ignored.md", config.excluded()));
        assert_eq!(
            Surfaces::compile("test", ["docs/*.md"])
                .unwrap()
                .governing("docs/a.md"),
            Some(0)
        );

        let scope = Scope {
            files: vec![STDIN.to_string(), "missing.md".to_string()],
            directory: None,
            stdin: Some("standard input".to_string()),
        };
        assert_eq!(
            scope.documents(&tree),
            vec![
                (STDIN.to_string(), Ok("standard input".to_string())),
                ("missing.md".to_string(), Err(Unselectable::Missing))
            ]
        );
        tree.mark_binary("docs/nested/guide.md");
        assert!(Corpus::read(&tree, &config).is_err());
    }

    #[test]
    fn a_selection_behind_a_link_escapes_and_an_unreadable_one_is_unreadable() {
        let mut tree = MemoryTree::default();
        tree.write("docs/outside/held.md", "# held");
        tree.mark_link("docs/outside");
        let scope = Scope {
            files: vec!["docs/outside/held.md".to_string(), STDIN.to_string()],
            directory: None,
            stdin: None,
        };
        assert_eq!(
            scope.documents(&tree),
            vec![
                (
                    "docs/outside/held.md".to_string(),
                    Err(Unselectable::Escapes)
                ),
                (STDIN.to_string(), Err(Unselectable::Unreadable)),
            ]
        );
        tree.write("docs/sealed.md", "# sealed");
        tree.mark_unreadable("docs/sealed.md");
        let sealed = Scope {
            files: vec!["docs/sealed.md".to_string()],
            ..Scope::default()
        };
        assert_eq!(
            sealed.documents(&tree),
            vec![("docs/sealed.md".to_string(), Err(Unselectable::Unreadable))]
        );
        for (reason, code) in [
            (Unselectable::Escapes, "rhino.path.escapes-root"),
            (Unselectable::Missing, "rhino.file.missing"),
            (Unselectable::Unreadable, "rhino.file.unreadable"),
        ] {
            let refused = reason
                .refusal("mermaid", "docs/a.md")
                .render(crate::cli::Format::Json);
            assert_eq!(refused.exit_code, 2);
            assert!(refused.stderr.contains(code), "{}", refused.stderr);
        }
    }
}
