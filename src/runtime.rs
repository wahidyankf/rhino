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
        self.files.get(path).cloned().ok_or(TreeError::NotFound)
    }

    fn files(&self) -> Vec<String> {
        self.files
            .keys()
            .filter(|path| !self.links.contains(*path))
            .cloned()
            .collect()
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
            files: self
                .files
                .iter()
                .filter_map(|(held, content)| {
                    held.strip_prefix(&prefix)
                        .map(|rest| (rest.to_string(), content.clone()))
                })
                .collect(),
            unreadable: strip(&self.unreadable),
            links: strip(&self.links),
        }))
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

    fn rooted_at(&self, path: &str) -> Result<Box<dyn Tree>, String> {
        // Resolved against the current root when relative and taken as given
        // when absolute, so `--root` means the same thing to a caller wherever
        // they happen to have run the tool from.
        let candidate = if std::path::Path::new(path).is_absolute() {
            PathBuf::from(path)
        } else {
            self.root.join(path)
        };
        let mut rooted = Self::new(candidate)?;
        rooted.exclude(&self.excluded_directories);
        Ok(Box::new(rooted))
    }
}
