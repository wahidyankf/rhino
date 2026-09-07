//! The filesystem port.
//!
//! Every validator reaches files through this trait rather than through
//! `std::fs`, which is what lets the whole behaviour corpus run against an
//! in-memory tree at the unit boundary. It is also where the read-only
//! constraint is actually held: the trait exposes no write, so a validator
//! holding a `&dyn Tree` cannot modify the repository it is inspecting even by
//! mistake.

use std::collections::{BTreeMap, BTreeSet};
use std::path::{Path, PathBuf};

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum TreeError {
    NotFound,
    Unreadable(String),
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
}

impl MemoryTree {
    pub fn write(&mut self, path: &str, content: &str) {
        self.files.insert(
            path.trim_start_matches('/').to_string(),
            content.to_string(),
        );
    }
}

impl Tree for MemoryTree {
    fn read(&self, path: &str) -> Result<String, TreeError> {
        self.files
            .get(path.trim_start_matches('/'))
            .cloned()
            .ok_or(TreeError::NotFound)
    }

    fn files(&self) -> Vec<String> {
        self.files.keys().cloned().collect()
    }
}

/// A tree backed by a real directory.
///
/// The only place in the crate that touches `std::fs`, and read-only there.
/// Filesystem links are skipped unconditionally rather than by configuration,
/// because following one can leave the repository entirely -- a validator that
/// walked out of the root would report findings against files the repository
/// does not own.
#[derive(Debug, Clone)]
pub struct DiskTree {
    root: PathBuf,
    excluded_directories: Vec<String>,
}

impl DiskTree {
    pub fn new(root: impl Into<PathBuf>) -> Result<Self, String> {
        let root = root.into();
        if !root.is_dir() {
            return Err(format!("{} is not a directory", root.display()));
        }
        Ok(Self {
            root,
            excluded_directories: Vec::new(),
        })
    }

    pub fn at_current_directory() -> Result<Self, String> {
        let root = std::env::current_dir()
            .map_err(|error| format!("cannot determine the working directory: {error}"))?;
        Self::new(root)
    }

    /// Directory names skipped at any depth. Applied after construction because
    /// the list is configuration, and the configuration is itself read through
    /// this tree.
    pub fn exclude(&mut self, directories: &[String]) {
        self.excluded_directories = directories.to_vec();
    }

    fn walk(&self, directory: &Path, found: &mut Vec<String>) {
        let Ok(entries) = std::fs::read_dir(directory) else {
            return;
        };
        for entry in entries.flatten() {
            let path = entry.path();
            // `symlink_metadata` does not follow the link, so a link is
            // identified as one rather than as whatever it points at.
            let Ok(metadata) = std::fs::symlink_metadata(&path) else {
                continue;
            };
            if metadata.is_symlink() {
                continue;
            }
            let name = entry.file_name().to_string_lossy().into_owned();
            if metadata.is_dir() {
                if !self.excluded_directories.contains(&name) {
                    self.walk(&path, found);
                }
            } else if let Ok(relative) = path.strip_prefix(&self.root) {
                found.push(relative.to_string_lossy().replace('\\', "/"));
            }
        }
    }

    /// Resolve a repository-relative path, refusing one that would leave the
    /// root even if the caller built it from configuration.
    fn resolve(&self, path: &str) -> Option<PathBuf> {
        let mut resolved = self.root.clone();
        for segment in path.split('/') {
            match segment {
                "" | "." => {}
                ".." => return None,
                _ => resolved.push(segment),
            }
        }
        Some(resolved)
    }
}

impl Tree for DiskTree {
    fn read(&self, path: &str) -> Result<String, TreeError> {
        let Some(resolved) = self.resolve(path) else {
            return Err(TreeError::Unreadable(format!(
                "{path} leaves the repository root"
            )));
        };
        match std::fs::read_to_string(&resolved) {
            Ok(text) => Ok(text),
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => Err(TreeError::NotFound),
            Err(error) => Err(TreeError::Unreadable(error.to_string())),
        }
    }

    fn files(&self) -> Vec<String> {
        let mut found = Vec::new();
        self.walk(&self.root, &mut found);
        found.sort();
        found
    }

    fn is_directory(&self, path: &str) -> bool {
        self.resolve(path).is_some_and(|resolved| resolved.is_dir())
    }
}
