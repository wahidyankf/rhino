//! The filesystem port.
//!
//! Every validator reaches files through this trait rather than through
//! `std::fs`, which is what lets the whole behaviour corpus run against an
//! in-memory tree at the unit boundary. It is also where the read-only
//! constraint is actually held: the trait exposes no write, so a validator
//! holding a `&dyn Tree` cannot modify the repository it is inspecting even by
//! mistake.

pub mod disk;

pub use disk::{DiskTree, ProcessLauncher};

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

/// What one gate child was asked to do.
///
/// Every field is stated by the runner rather than inherited from the process
/// RHINO happens to have been started in. A hook that behaved one way under Git
/// and another under a hosted runner would be a gate nobody could trust, and
/// inheritance is exactly how that difference gets in.
pub struct Launch<'a> {
    /// The declared vector with the hook's own arguments already appended. The
    /// first value is the executable; there is no shell, so a value holding a
    /// space or a metacharacter is one ordinary argument.
    pub arguments: &'a [String],
    /// The repository root, which is the child's working directory.
    pub directory: &'a str,
    /// The one environment variable RHINO sets, and the only thing a leaf may
    /// branch on.
    pub surface: &'a str,
    /// The hook's own standard input, forwarded unmodified.
    pub stdin: Option<&'a str>,
}

/// How a child ended.
///
/// Only the code is kept. A child's own streams are deliberately not carried
/// back, because the report may repeat nothing a child wrote and a value that
/// is never read cannot be leaked by a later change.
pub struct Launched {
    pub code: i32,
}

/// Why a child never ran.
pub struct LaunchError(pub String);

/// The one place RHINO starts a process.
///
/// A port rather than a direct `Command`, for the same reason the filesystem is
/// one: which gates ran, what each was handed, and whether any two overlapped
/// are claims the unit boundary has to be able to make, and it has no process
/// to spawn.
pub trait Launcher {
    fn launch(&self, launch: Launch<'_>) -> Result<Launched, LaunchError>;
}

/// The launcher every command that is not a gate dispatch is given.
///
/// Refusing rather than absent, so `execute` stays the entry point a consumer
/// calls without having to supply something it has no use for, and so a gate
/// dispatch reaching it is a protocol failure with a reason rather than a
/// silent pass.
pub struct NoLauncher;

impl Launcher for NoLauncher {
    fn launch(&self, _: Launch<'_>) -> Result<Launched, LaunchError> {
        Err(LaunchError(
            "this build was called through an entry point that starts no process".to_string(),
        ))
    }
}

/// A repository as RHINO is allowed to see it: read, list, and nothing else.
pub trait Tree {
    fn read(&self, path: &str) -> Result<String, TreeError>;

    /// Every file in the tree, repository-relative and sorted, with excluded
    /// directories and filesystem links already dropped.
    fn files(&self) -> Vec<String>;

    /// Every directory in the tree, repository-relative and sorted, including
    /// one that holds no file at any depth.
    ///
    /// Separate from `files` because a directory holding nothing is the one
    /// directory a file list cannot describe, and a rule about exactly that
    /// state has to be able to see it. Every other caller wants the directories
    /// documents live in and derives them from the files, which is why this is
    /// not the ordinary way to ask.
    ///
    /// Git tracks no empty directory, so an answer here is a fact about a
    /// working tree -- which is the tree a pre-commit gate inspects.
    fn directories(&self) -> Vec<String> {
        parents(&self.files())
    }

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

    /// Where this repository is, as a child would have to be told it.
    ///
    /// Only the gate runner asks, and only so a child's working directory is
    /// the repository root rather than wherever the hook happened to be
    /// started. A tree with no place on a filesystem answers with the working
    /// directory, which is what it already means to every other caller.
    fn root(&self) -> String {
        ".".to_string()
    }
}

/// Every directory a list of file paths implies, sorted and without repeats.
fn parents(files: &[String]) -> Vec<String> {
    let mut found: BTreeSet<String> = BTreeSet::new();
    for path in files {
        let mut segments: Vec<&str> = path.split('/').collect();
        segments.pop();
        while !segments.is_empty() {
            found.insert(segments.join("/"));
            segments.pop();
        }
    }
    found.into_iter().collect()
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
    empty: BTreeSet<String>,
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

    /// A directory the repository holds and no file lives under.
    ///
    /// Stated rather than derived, because a tree built from its files has no
    /// way to hold one -- which is exactly why a rule about empty directories
    /// would otherwise be provable only where there is a disk.
    pub fn mark_directory(&mut self, path: &str) {
        self.empty.insert(path.trim_matches('/').to_string());
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

    fn directories(&self) -> Vec<String> {
        let mut found = parents(&self.files());
        // A stated empty directory contributes itself and every directory on
        // the way to it, because a directory holding only an empty directory
        // holds no file either.
        for path in &self.empty {
            let mut segments: Vec<&str> = path.split('/').collect();
            while !segments.is_empty() {
                found.push(segments.join("/"));
                segments.pop();
            }
        }
        found.sort();
        found.dedup();
        found
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
            empty: strip(&self.empty),
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
