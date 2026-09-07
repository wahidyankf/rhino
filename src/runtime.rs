//! The filesystem port.
//!
//! Every validator reaches files through this trait rather than through
//! `std::fs`, which is what lets the whole behaviour corpus run against an
//! in-memory tree at the unit boundary. It is also where the read-only
//! constraint is actually held: the trait exposes no write, so a validator
//! holding a `&dyn Tree` cannot modify the repository it is inspecting even by
//! mistake.

pub mod disk;

pub use disk::DiskTree;

use std::collections::{BTreeMap, BTreeSet};

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum TreeError {
    NotFound,
    Unreadable(String),
    /// The file opened and holds no text.
    ///
    /// A separate kind from `Unreadable` because the two oblige different
    /// answers. A file RHINO could not open might have held anything, so a
    /// validator that skipped it would be reporting on a repository it had not
    /// read. A file it did open and found bytes in is simply not the sort of
    /// thing any rule here is about -- an image, an archive, a compiled
    /// artifact -- and refusing the run over one would mean no repository with
    /// a logo in it could be inspected at all.
    NotText,
}

/// A repository as RHINO is allowed to see it: read, list, and nothing else.
pub trait Tree {
    fn read(&self, path: &str) -> Result<String, TreeError>;

    /// Every file in the tree, repository-relative and sorted, with excluded
    /// directories and filesystem links already dropped.
    fn files(&self) -> Vec<String>;

    fn exists(&self, path: &str) -> bool {
        self.read(path).is_ok() || self.is_directory(path)
    }

    /// The direct children of a directory, as repository-relative paths.
    fn children(&self, directory: &str) -> Vec<String> {
        let prefix = normalise_directory(directory);
        let mut seen: BTreeSet<String> = BTreeSet::new();
        for path in self.files() {
            let Some(rest) = path.strip_prefix(&prefix) else {
                continue;
            };
            match rest.split_once('/') {
                Some((child, _)) => seen.insert(format!("{prefix}{child}")),
                None => seen.insert(path.clone()),
            };
        }
        seen.into_iter().collect()
    }

    fn is_directory(&self, path: &str) -> bool {
        let prefix = normalise_directory(path);
        self.files().iter().any(|file| file.starts_with(&prefix))
    }

    /// The same repository with these directory names skipped at any depth.
    ///
    /// Applied after the configuration is read, because the list *is*
    /// configuration -- and applied to the tree that will actually be walked,
    /// which is the point: a list read from one repository and applied to
    /// another would skip directories the second never excluded and report it
    /// clean for a reason it never declared.
    fn excluding(&self, directories: &[String]) -> Box<dyn Tree>;

    /// The same repository seen from `path`, or why it cannot be.
    ///
    /// `--root` is a claim about which repository to inspect, and every
    /// implementation of this port has to be able to answer it -- otherwise the
    /// flag would only be testable at the boundary that happens to have a
    /// filesystem, which is where a defect in it would then live.
    fn rooted_at(&self, path: &str) -> Result<Box<dyn Tree>, String>;
}

/// `""` addresses the root; everything else gains one trailing separator, so a
/// prefix test cannot match a sibling whose name merely starts the same way.
fn normalise_directory(path: &str) -> String {
    let trimmed = path.trim_matches('/');
    if trimmed.is_empty() || trimmed == "." {
        String::new()
    } else {
        format!("{trimmed}/")
    }
}

/// A tree held in memory.
///
/// This is the port's second implementation rather than a mock of the first:
/// scenarios exercise the same trait the product uses. Files are placed through
/// `write` while the fixture is being built; once a validator holds it as a
/// `&dyn Tree`, no write is reachable.
#[derive(Debug, Default, Clone)]
pub struct MemoryTree {
    files: BTreeMap<String, String>,
    unreadable: BTreeSet<String>,
    vanished: BTreeSet<String>,
    binary: BTreeSet<String>,
    links: BTreeSet<String>,
}

impl MemoryTree {
    pub fn write(&mut self, path: &str, content: &str) {
        self.files.insert(
            path.trim_start_matches('/').to_string(),
            content.to_string(),
        );
    }

    /// A file the repository holds but that cannot be opened.
    ///
    /// It stays in the tree on purpose: a validator has to see it and fail
    /// closed, not miss it and report the repository clean.
    pub fn mark_unreadable(&mut self, path: &str) {
        self.unreadable
            .insert(path.trim_start_matches('/').to_string());
    }

    /// A file the walk listed and the read can no longer find.
    ///
    /// A real repository changes underneath a validator, and "not found" is a
    /// different answer from "cannot be opened": the first is a file that is
    /// gone, which is nobody's policy violation, and the second is a file that
    /// is there and must be failed closed on. The in-memory tree has to be able
    /// to say both, or the difference is proved only where there is a disk.
    pub fn mark_vanished(&mut self, path: &str) {
        self.vanished
            .insert(path.trim_start_matches('/').to_string());
    }

    /// A file whose bytes are not text. It opens; there is simply nothing in it
    /// to read as a document.
    pub fn mark_binary(&mut self, path: &str) {
        self.binary.insert(path.trim_start_matches('/').to_string());
    }

    /// A filesystem link. Never reported, because following one can leave the
    /// repository entirely.
    pub fn mark_link(&mut self, path: &str) {
        self.links.insert(path.trim_start_matches('/').to_string());
    }
}

impl Tree for MemoryTree {
    fn read(&self, path: &str) -> Result<String, TreeError> {
        let path = path.trim_start_matches('/');
        if self.unreadable.contains(path) {
            return Err(TreeError::Unreadable("permission denied".to_string()));
        }
        if self.binary.contains(path) {
            return Err(TreeError::NotText);
        }
        if self.vanished.contains(path) {
            return Err(TreeError::NotFound);
        }
        self.files.get(path).cloned().ok_or(TreeError::NotFound)
    }

    fn files(&self) -> Vec<String> {
        self.files
            .keys()
            .filter(|path| !self.links.contains(*path))
            .cloned()
            .collect()
    }

    fn excluding(&self, _directories: &[String]) -> Box<dyn Tree> {
        // Nothing to do: the in-memory tree holds only what a scenario put in
        // it, and `scan` applies the declared exclusions to every walk.
        Box::new(self.clone())
    }

    fn rooted_at(&self, path: &str) -> Result<Box<dyn Tree>, String> {
        let prefix = normalise_directory(path);
        if prefix.is_empty() {
            return Ok(Box::new(self.clone()));
        }
        // A root naming nothing is refused rather than silently inspected as an
        // empty repository, which would report every tree clean.
        if !self.files.keys().any(|held| held.starts_with(&prefix)) {
            return Err(format!("{path} is not a directory"));
        }
        let strip = |set: &BTreeSet<String>| -> BTreeSet<String> {
            set.iter()
                .filter_map(|held| held.strip_prefix(&prefix).map(str::to_string))
                .collect()
        };
        Ok(Box::new(Self {
            binary: strip(&self.binary),
            files: self
                .files
                .iter()
                .filter_map(|(held, content)| {
                    held.strip_prefix(&prefix)
                        .map(|rest| (rest.to_string(), content.clone()))
                })
                .collect(),
            unreadable: strip(&self.unreadable),
            vanished: strip(&self.vanished),
            links: strip(&self.links),
        }))
    }
}
