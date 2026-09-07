//! The concrete filesystem tree.
//!
//! Split into its own module so it can be named as a coverage exclusion.
//! Every line here talks to `std::fs`, which a unit test may not do, so its
//! proof is the integration and E2E adapters running the whole corpus
//! against it -- not a smaller number in a report.

use super::{Tree, TreeError};
use std::path::{Path, PathBuf};

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

    fn excluding(&self, directories: &[String]) -> Box<dyn Tree> {
        let mut excluded = self.clone();
        excluded.exclude(directories);
        Box::new(excluded)
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
        // Deliberately *not* carrying this tree's exclusion list across: the
        // new root declares its own, and inheriting one would skip directories
        // the selected repository never excluded.
        Ok(Box::new(Self::new(candidate)?))
    }
}
