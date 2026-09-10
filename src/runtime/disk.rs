//! The concrete filesystem tree.
//!
//! Split into its own module so it can be named as a coverage exclusion.
//! Every line here talks to `std::fs`, which a unit test may not do, so its
//! proof is the integration and E2E adapters running the whole corpus
//! against it -- not a smaller number in a report.

use super::{Launch, LaunchError, Launched, Launcher, Tree, TreeError};
use std::io::Write;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};

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

    /// Whether a directory holds any file at any depth, stopping at the first.
    ///
    /// The walk's definition of a child asked of one subtree rather than of the
    /// whole repository: the port exposes files, so a directory appearing in no
    /// file path is not a child. Short-circuiting matters -- the common case is
    /// a directory whose very first entry is a file.
    fn holds_a_file(&self, directory: &Path) -> bool {
        let Ok(entries) = std::fs::read_dir(directory) else {
            return false;
        };
        for entry in entries.flatten() {
            let path = entry.path();
            let Ok(metadata) = std::fs::symlink_metadata(&path) else {
                continue;
            };
            if metadata.is_symlink() {
                continue;
            }
            if !metadata.is_dir() {
                return true;
            }
            let name = entry.file_name().to_string_lossy().into_owned();
            if !self.excluded_directories.contains(&name) && self.holds_a_file(&path) {
                return true;
            }
        }
        false
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
            // The one failure that is about the bytes rather than about access.
            Err(error) if error.kind() == std::io::ErrorKind::InvalidData => {
                Err(TreeError::NotText)
            }
            Err(error) => Err(TreeError::Unreadable(error.to_string())),
        }
    }

    fn files(&self) -> Vec<String> {
        let mut found = Vec::new();
        self.walk(&self.root, &mut found);
        found.sort();
        found
    }

    /// The direct children of one directory, read from that directory alone.
    ///
    /// The port defines a child in terms of the file list, and the trait's
    /// default obtains it by walking the whole repository -- correct for any
    /// implementation, affordable only for the in-memory one. `directory-map`
    /// asks this once per mapped directory, so on disk the default turned
    /// forty-nine questions about one directory each into forty-nine walks of
    /// the tree, and the cost grew with the repository rather than with what
    /// was being inspected.
    ///
    /// The answer does not change, which means keeping three things the walk
    /// implies rather than states: a filesystem link is not a child, an
    /// excluded directory name is not a child at any depth, and a subdirectory
    /// holding no file is not a child at all.
    fn children(&self, directory: &str) -> Vec<String> {
        let prefix = super::normalise_directory(directory);
        // A directory the exclusion list removes contributes no file to the
        // walk, so it has no children either. Asking about one directly has to
        // answer the same way as asking about its parent did.
        if prefix
            .split('/')
            .any(|segment| self.excluded_directories.iter().any(|name| name == segment))
        {
            return Vec::new();
        }
        let Some(resolved) = self.resolve(directory) else {
            return Vec::new();
        };
        let Ok(entries) = std::fs::read_dir(&resolved) else {
            return Vec::new();
        };
        let mut found = Vec::new();
        for entry in entries.flatten() {
            let path = entry.path();
            let Ok(metadata) = std::fs::symlink_metadata(&path) else {
                continue;
            };
            if metadata.is_symlink() {
                continue;
            }
            let name = entry.file_name().to_string_lossy().into_owned();
            if metadata.is_dir()
                && (self.excluded_directories.contains(&name) || !self.holds_a_file(&path))
            {
                continue;
            }
            found.push(format!("{prefix}{name}"));
        }
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

    fn root(&self) -> String {
        self.root.to_string_lossy().into_owned()
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

/// The real launcher: one child at a time, through `std::process`.
///
/// Lives here rather than beside the port because this module is the only one
/// allowed to reach the operating system, and because the corpus proves the
/// dispatch itself at three boundaries through the port instead.
pub struct ProcessLauncher;

impl Launcher for ProcessLauncher {
    fn launch(&self, launch: Launch<'_>) -> Result<Launched, LaunchError> {
        let Some((program, arguments)) = launch.arguments.split_first() else {
            return Err(LaunchError("the gate declares no command".to_string()));
        };

        // Standard input is always a pipe, closed when the hook supplied
        // nothing. Inheriting the terminal would let a child block a hook on a
        // read the hook never intended to allow.
        // Both output streams are captured and dropped. A gate exists to look
        // at content that may not be publishable, so a runner that let a
        // child's stream through would publish what it found on the way to
        // saying it should not be published. Only the identifier and a
        // sanitized status reach the report.
        let mut child = Command::new(program)
            .args(arguments)
            .current_dir(launch.directory)
            .env("OSE_GATE_SURFACE", launch.surface)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
            .map_err(|error| LaunchError(format!("`{program}` could not be started: {error}")))?;

        {
            let Some(mut pipe) = child.stdin.take() else {
                return Err(LaunchError(format!("`{program}` has no standard input")));
            };
            if let Some(text) = launch.stdin {
                let _ = pipe.write_all(text.as_bytes());
            }
        }

        // Waited on with its output collected rather than with `wait`: a child
        // writing more than a pipe holds would otherwise block forever on a
        // buffer nobody is draining.
        let output = child
            .wait_with_output()
            .map_err(|error| LaunchError(format!("`{program}` could not be waited on: {error}")))?;

        // A signal leaves no code. Reported as the protocol failure it is
        // rather than as a finding, because a killed child checked nothing.
        match output.status.code() {
            Some(code) => Ok(Launched { code }),
            None => Err(LaunchError(format!("`{program}` was ended by a signal"))),
        }
    }
}
