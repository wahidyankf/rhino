//! The filesystem port.
//!
//! Every validator reaches files through this trait rather than through
//! `std::fs`, which is what lets the whole behaviour corpus run against an
//! in-memory tree at the unit boundary. It is also where the read-only
//! constraint is actually held: the trait exposes no write, so a validator
//! holding a `&dyn Tree` cannot modify the repository it is inspecting even by
//! mistake.

pub mod disk;

pub use disk::{
    DiskAdapterStore, DiskEnvironmentStore, DiskMutationRunner, DiskToolchainRunner, DiskTree,
    ProcessLauncher,
};

use std::cell::RefCell;
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
    /// Declared gate projections only. Values are already resolved from typed
    /// inputs; the launcher never parses a template or reconstructs a shell
    /// assignment.
    pub environment: &'a BTreeMap<String, String>,
}

/// How a child ended.
///
/// Only the code is kept. A child's own streams are deliberately not carried
/// back, because the report may repeat nothing a child wrote and a value that
/// is never read cannot be leaked by a later change.
pub struct Launched {
    pub code: i32,
}

/// Why a child never ran, in the only three shapes a caller can act on.
///
/// The distinction between the first two is the whole difference between `127`
/// and `126`: one says the program is not installed, the other says it is there
/// and the kernel would not run it. A caller fixes those two problems in
/// completely different places, and collapsing them costs exactly the
/// information needed to tell which.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum LaunchFailure {
    /// The named program does not exist.
    NotFound,
    /// The program exists and could not be executed.
    NotExecutable,
    /// Anything else: refused before the attempt, or lost after it.
    Refused,
}

/// Why a child never ran.
pub struct LaunchError {
    pub reason: String,
    pub failure: LaunchFailure,
}

impl LaunchError {
    /// A refusal that is neither an absent program nor an unusable one.
    pub fn refused(reason: impl Into<String>) -> Self {
        Self {
            reason: reason.into(),
            failure: LaunchFailure::Refused,
        }
    }

    /// A spawn failure classified by what the operating system said about it.
    pub fn from_spawn(program: &str, error: &std::io::Error) -> Self {
        let failure = match error.kind() {
            std::io::ErrorKind::NotFound => LaunchFailure::NotFound,
            std::io::ErrorKind::PermissionDenied => LaunchFailure::NotExecutable,
            _ => LaunchFailure::Refused,
        };
        Self {
            reason: format!("`{program}` could not be started: {error}"),
            failure,
        }
    }
}

/// A declared mutation command isolated from the ordinary child launcher.
/// The runner owns the snapshot and write boundary; the gate dispatcher owns
/// only typed projection, result classification, and ordering.
pub struct MutationLaunch<'a> {
    pub arguments: &'a [String],
    pub directory: &'a str,
    pub environment: &'a BTreeMap<String, String>,
    /// Repository-relative paths selected from the actual Git index. The
    /// mutation runner must never infer a wider write set from the working
    /// tree or from the child command.
    pub selected_paths: &'a [String],
    /// Candidate commit used only by a disposable pull-request replay.
    pub revision: Option<&'a str>,
}

/// Sanitized mutation outcome. Changed paths are repository-relative names;
/// content and child streams stay inside the snapshot boundary.
pub struct Mutated {
    pub code: i32,
    pub changes: Vec<String>,
    pub divergences: Vec<String>,
}

pub struct MutationError(pub String);

/// One generated adapter file, held in memory until every profile has been
/// discovered and validated. The bytes never come from an adapter directory:
/// generation is a projection of canonical sources only.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct AdapterFile {
    pub path: String,
    pub contents: String,
}

/// The complete replacement of declared adapter families plus individually
/// managed adapter files. `roots` may replace whole generated directories;
/// `exact_paths` permits a root-level route without claiming its neighbours.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct AdapterTransaction {
    pub roots: Vec<String>,
    pub exact_paths: Vec<String>,
    pub files: Vec<AdapterFile>,
}

pub struct AdapterError(pub String);

/// One private local environment file, fully materialized before its target is
/// opened. The transaction never carries a value outside this narrow boundary.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct EnvironmentFile {
    pub path: String,
    pub contents: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct EnvironmentTransaction {
    pub files: Vec<EnvironmentFile>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct EnvironmentRestoreFile {
    pub path: String,
    pub contents: String,
    pub replace: bool,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct EnvironmentRestoreTransaction {
    pub files: Vec<EnvironmentRestoreFile>,
}

#[derive(Debug)]
pub struct EnvironmentError(pub String);

/// A declared executable and argv vector. Unlike a gate launch it carries no
/// hook surface or inherited environment, so a toolchain operation cannot be
/// mistaken for lifecycle dispatch.
pub struct ToolchainLaunch<'a> {
    pub executable: &'a str,
    pub arguments: &'a [String],
    /// A configured upper bound for one child; `None` leaves no time limit.
    pub timeout_seconds: Option<u64>,
}

#[derive(Debug)]
pub struct ToolchainResult {
    pub code: i32,
    /// Captured only for the declared version parser; operation output never
    /// includes these bytes.
    pub stdout: String,
}

pub struct ToolchainError(pub String);

/// The narrow write port used only by canonical adapter generation.
///
/// A caller supplies all desired bytes before asking the store to replace any
/// family. This makes semantic loss a pre-write failure and keeps validation
/// read-only even when it compares the same desired projection.
pub trait AdapterStore {
    fn replace(&self, root: &str, transaction: &AdapterTransaction) -> Result<(), AdapterError>;
}

/// The separate no-overwrite write boundary for declared local environment
/// targets. It is never held by a validator or a dry-run planner.
pub trait EnvironmentStore {
    fn create(
        &self,
        root: &str,
        transaction: &EnvironmentTransaction,
    ) -> Result<(), EnvironmentError>;
    fn restore(
        &self,
        root: &str,
        transaction: &EnvironmentRestoreTransaction,
    ) -> Result<(), EnvironmentError>;
}

/// The only capability that starts a declared toolchain probe or provision
/// vector. The runner never receives a shell string or tool output to report.
pub trait ToolchainRunner {
    fn run(&self, launch: ToolchainLaunch<'_>) -> Result<ToolchainResult, ToolchainError>;
}

pub struct NoAdapterStore;

pub struct NoEnvironmentStore;

pub struct NoToolchainRunner;

impl AdapterStore for NoAdapterStore {
    fn replace(&self, _: &str, _: &AdapterTransaction) -> Result<(), AdapterError> {
        Err(AdapterError(
            "this entry point has no canonical-adapter write boundary".to_string(),
        ))
    }
}

impl EnvironmentStore for NoEnvironmentStore {
    fn create(&self, _: &str, _: &EnvironmentTransaction) -> Result<(), EnvironmentError> {
        Err(EnvironmentError(
            "this entry point has no environment write boundary".to_string(),
        ))
    }

    fn restore(&self, _: &str, _: &EnvironmentRestoreTransaction) -> Result<(), EnvironmentError> {
        Err(EnvironmentError(
            "this entry point has no environment write boundary".to_string(),
        ))
    }
}

impl ToolchainRunner for NoToolchainRunner {
    fn run(&self, _: ToolchainLaunch<'_>) -> Result<ToolchainResult, ToolchainError> {
        Err(ToolchainError(
            "this entry point has no toolchain process boundary".to_string(),
        ))
    }
}

/// The only capability that may turn a declared mutation contract into a
/// write. Read-only callers receive [`NoMutationRunner`] and therefore fail
/// closed instead of quietly falling back to the mutable working tree.
pub trait MutationRunner {
    fn apply_index(&self, launch: MutationLaunch<'_>) -> Result<Mutated, MutationError>;
    fn verify_clean(&self, launch: MutationLaunch<'_>) -> Result<Mutated, MutationError>;
}

pub struct NoMutationRunner;

impl MutationRunner for NoMutationRunner {
    fn apply_index(&self, _: MutationLaunch<'_>) -> Result<Mutated, MutationError> {
        Err(MutationError(
            "this entry point has no index mutation boundary".to_string(),
        ))
    }

    fn verify_clean(&self, _: MutationLaunch<'_>) -> Result<Mutated, MutationError> {
        Err(MutationError(
            "this entry point has no disposable pull-request replay boundary".to_string(),
        ))
    }
}

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
        Err(LaunchError::refused(
            "this build was called through an entry point that starts no process",
        ))
    }
}

/// A repository as RHINO is allowed to see it: read, list, and nothing else.
pub trait Tree {
    fn read(&self, path: &str) -> Result<String, TreeError>;

    /// Read one declared mutation source without following a filesystem link.
    /// The default preserves the in-memory behavior; disk-backed trees tighten
    /// this boundary before a caller can copy any source bytes.
    fn read_no_follow(&self, path: &str) -> Result<String, TreeError> {
        self.read(path)
    }

    /// Every file in the tree, repository-relative and sorted, with excluded
    /// directories and filesystem links already dropped.
    fn files(&self) -> Vec<String>;

    /// The staged paths selected by the Git index, never by a mutable working
    /// tree. A tree with no Git snapshot capability must refuse the request;
    /// returning `files()` here would quietly widen a mutation boundary.
    fn indexed_files(&self) -> Result<Vec<String>, String> {
        Err("this tree has no Git index snapshot boundary".to_string())
    }

    /// The paths changed between two already validated immutable commits. A
    /// pull-request mutation receives only that reviewed selection, never the
    /// complete checkout or a mutable working-tree approximation.
    fn changed_files(&self, _base: &str, _head: &str) -> Result<Vec<String>, String> {
        Err("this tree has no changed-file range boundary".to_string())
    }

    /// Resolve a repository-declared comparison ref to one immutable commit.
    /// The source is a capability rather than a string convention: a caller
    /// with no Git boundary cannot manufacture a base for a newly pushed ref.
    fn resolve_git_ref(&self, _reference: &str) -> Result<String, String> {
        Err("this tree has no Git ref-resolution boundary".to_string())
    }

    /// Read the commit-message text for one already validated immutable range.
    /// A pull-request gate consumes the same semantic message input as the
    /// commit-msg hook, but receives it from the reviewed range rather than a
    /// mutable hook file.
    fn commit_messages(&self, _base: &str, _head: &str) -> Result<String, String> {
        Err("this tree has no commit-message range boundary".to_string())
    }

    /// Read the one message file Git assigned to the current `commit-msg`
    /// hook. This is deliberately separate from [`Tree::read`]: Git keeps the
    /// file outside a linked worktree, while arbitrary external paths are not
    /// repository inputs RHINO may read.
    fn hook_message_file(&self, _path: &str) -> Result<String, TreeError> {
        Err(TreeError::Unreadable(
            "the Git hook message-file boundary is unavailable".to_string(),
        ))
    }

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
    files: RefCell<BTreeMap<String, String>>,
    unreadable: BTreeSet<String>,
    vanished: BTreeSet<String>,
    binary: BTreeSet<String>,
    links: BTreeSet<String>,
    empty: BTreeSet<String>,
    indexed: Option<Vec<String>>,
    changed_files: BTreeMap<(String, String), Vec<String>>,
    git_refs: BTreeMap<String, String>,
    commit_messages: BTreeMap<(String, String), String>,
    hook_message_file: Option<String>,
}

impl MemoryTree {
    pub fn write(&self, path: &str, content: &str) {
        self.files.borrow_mut().insert(
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

    /// State a synthetic index explicitly. Tests use it to prove that dispatch
    /// passes only the index selection into a mutation port, rather than
    /// treating every visible file as staged.
    pub fn set_indexed_files<I, S>(&mut self, paths: I)
    where
        I: IntoIterator<Item = S>,
        S: AsRef<str>,
    {
        let mut paths: Vec<String> = paths
            .into_iter()
            .map(|path| path.as_ref().trim_start_matches('/').to_string())
            .collect();
        paths.sort();
        paths.dedup();
        self.indexed = Some(paths);
    }

    /// State the exact path selection for one immutable commit range. This is
    /// distinct from the index because a pull-request replay must never widen
    /// its formatter input to the snapshot's complete file inventory.
    pub fn set_changed_files<I, S>(&mut self, base: &str, head: &str, paths: I)
    where
        I: IntoIterator<Item = S>,
        S: AsRef<str>,
    {
        let mut paths: Vec<String> = paths
            .into_iter()
            .map(|path| path.as_ref().trim_start_matches('/').to_string())
            .collect();
        paths.sort();
        paths.dedup();
        self.changed_files
            .insert((base.to_string(), head.to_string()), paths);
    }

    /// State a synthetic resolved Git ref for a boundary test. The value is a
    /// commit ID already resolved by the fixture, not a fallback inferred from
    /// visible repository files.
    pub fn set_git_ref(&mut self, reference: &str, commit: &str) {
        self.git_refs
            .insert(reference.to_string(), commit.to_string());
    }

    /// State the non-merge commit-message text for one immutable range.
    pub fn set_commit_messages(&mut self, base: &str, head: &str, messages: &str) {
        self.commit_messages
            .insert((base.to_string(), head.to_string()), messages.to_string());
    }

    /// State the one file a synthetic Git hook supplied. Keeping this distinct
    /// from ordinary files lets the unit boundary prove that a typed message
    /// input does not become a general file-read capability.
    pub fn set_hook_message_file(&mut self, path: &str) {
        self.hook_message_file = Some(path.to_string());
    }

    fn replace_adapter_files(&self, transaction: &AdapterTransaction) -> Result<(), AdapterError> {
        let roots = adapter_roots(&transaction.roots)?;
        let exact_paths = adapter_exact_paths(&roots, &transaction.exact_paths)?;
        let files = adapter_files(&roots, &exact_paths, &transaction.files)?;
        let mut current = self.files.borrow_mut();
        current.retain(|path, _| {
            !roots.iter().any(|root| under_root(path, root)) && !exact_paths.contains(path)
        });
        current.extend(files);
        Ok(())
    }
}

/// The in-memory implementation keeps the unit boundary able to prove both a
/// first generated write and a no-op regeneration without touching a disk.
pub struct MemoryAdapterStore<'a> {
    tree: &'a MemoryTree,
}

impl<'a> MemoryAdapterStore<'a> {
    pub fn new(tree: &'a MemoryTree) -> Self {
        Self { tree }
    }
}

impl AdapterStore for MemoryAdapterStore<'_> {
    fn replace(&self, _: &str, transaction: &AdapterTransaction) -> Result<(), AdapterError> {
        self.tree.replace_adapter_files(transaction)
    }
}

/// The in-memory counterpart used only by the unit boundary to prove that a
/// planned initialization changes exactly its declared synthetic targets.
pub struct MemoryEnvironmentStore<'a> {
    tree: &'a MemoryTree,
}

impl<'a> MemoryEnvironmentStore<'a> {
    pub fn new(tree: &'a MemoryTree) -> Self {
        Self { tree }
    }
}

impl EnvironmentStore for MemoryEnvironmentStore<'_> {
    fn create(
        &self,
        _: &str,
        transaction: &EnvironmentTransaction,
    ) -> Result<(), EnvironmentError> {
        let mut current = self.tree.files.borrow_mut();
        for file in &transaction.files {
            let Some(path) = normal_adapter_path(&file.path) else {
                return Err(EnvironmentError(format!(
                    "invalid environment target `{}`",
                    file.path
                )));
            };
            if current.contains_key(&path) {
                return Err(EnvironmentError(format!(
                    "environment target `{path}` already exists"
                )));
            }
        }
        for file in &transaction.files {
            current.insert(file.path.clone(), file.contents.clone());
        }
        Ok(())
    }

    fn restore(
        &self,
        _: &str,
        transaction: &EnvironmentRestoreTransaction,
    ) -> Result<(), EnvironmentError> {
        let mut current = self.tree.files.borrow_mut();
        for file in &transaction.files {
            let Some(path) = normal_adapter_path(&file.path) else {
                return Err(EnvironmentError(format!(
                    "invalid environment target `{}`",
                    file.path
                )));
            };
            if current.contains_key(&path) && !file.replace {
                return Err(EnvironmentError(format!(
                    "environment target `{path}` already exists"
                )));
            }
        }
        for file in &transaction.files {
            current.insert(file.path.clone(), file.contents.clone());
        }
        Ok(())
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
        self.files
            .borrow()
            .get(path)
            .cloned()
            .ok_or(TreeError::NotFound)
    }

    fn read_no_follow(&self, path: &str) -> Result<String, TreeError> {
        let path = path.trim_start_matches('/');
        if self.links.contains(path) {
            return Err(TreeError::Unreadable(format!("{path} is a symbolic link")));
        }
        self.read(path)
    }

    fn files(&self) -> Vec<String> {
        self.files
            .borrow()
            .keys()
            .filter(|path| !self.links.contains(*path))
            .cloned()
            .collect()
    }

    fn indexed_files(&self) -> Result<Vec<String>, String> {
        self.indexed
            .clone()
            .ok_or_else(|| "this tree has no Git index snapshot boundary".to_string())
    }

    fn changed_files(&self, base: &str, head: &str) -> Result<Vec<String>, String> {
        self.changed_files
            .get(&(base.to_string(), head.to_string()))
            .cloned()
            .ok_or_else(|| "this tree has no changed-file range boundary".to_string())
    }

    fn resolve_git_ref(&self, reference: &str) -> Result<String, String> {
        self.git_refs
            .get(reference)
            .cloned()
            .ok_or_else(|| format!("the declared fallback ref `{reference}` does not resolve"))
    }

    fn commit_messages(&self, base: &str, head: &str) -> Result<String, String> {
        self.commit_messages
            .get(&(base.to_string(), head.to_string()))
            .cloned()
            .ok_or_else(|| "this tree has no commit-message range boundary".to_string())
    }

    fn hook_message_file(&self, path: &str) -> Result<String, TreeError> {
        let Some(expected) = self.hook_message_file.as_deref() else {
            return Err(TreeError::Unreadable(
                "the Git hook message-file boundary is unavailable".to_string(),
            ));
        };
        if path != expected {
            return Err(TreeError::Unreadable(
                "the supplied path is not Git's canonical COMMIT_EDITMSG".to_string(),
            ));
        }
        self.read(path)
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
        if !self
            .files
            .borrow()
            .keys()
            .any(|held| held.starts_with(&prefix))
        {
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
            files: RefCell::new(
                self.files
                    .borrow()
                    .iter()
                    .filter_map(|(held, content)| {
                        held.strip_prefix(&prefix)
                            .map(|rest| (rest.to_string(), content.clone()))
                    })
                    .collect(),
            ),
            unreadable: strip(&self.unreadable),
            vanished: strip(&self.vanished),
            links: strip(&self.links),
            indexed: self.indexed.as_ref().map(|paths| {
                paths
                    .iter()
                    .filter_map(|held| held.strip_prefix(&prefix).map(str::to_string))
                    .collect()
            }),
            changed_files: self
                .changed_files
                .iter()
                .map(|((base, head), paths)| {
                    (
                        (base.clone(), head.clone()),
                        paths
                            .iter()
                            .filter_map(|held| held.strip_prefix(&prefix).map(str::to_string))
                            .collect(),
                    )
                })
                .collect(),
            git_refs: self.git_refs.clone(),
            commit_messages: self.commit_messages.clone(),
            hook_message_file: self.hook_message_file.clone(),
        }))
    }
}

pub(crate) fn adapter_roots(roots: &[String]) -> Result<Vec<String>, AdapterError> {
    let mut normalized: Vec<String> = roots
        .iter()
        .map(|root| {
            normal_adapter_path(root)
                .ok_or_else(|| AdapterError(format!("invalid adapter root `{root}`")))
        })
        .collect::<Result<_, _>>()?;
    normalized.sort();
    normalized.dedup();
    if normalized.len() != roots.len()
        || normalized
            .windows(2)
            .any(|pair| under_root(&pair[1], &pair[0]))
    {
        return Err(AdapterError(
            "adapter roots must be distinct, non-overlapping repository paths".to_string(),
        ));
    }
    Ok(normalized)
}

/// Normalize individually managed adapter files and refuse any overlap with a
/// replaceable family. A path in a family is already covered by that family's
/// atomic replacement; listing it twice would make ownership ambiguous.
pub(crate) fn adapter_exact_paths(
    roots: &[String],
    exact_paths: &[String],
) -> Result<Vec<String>, AdapterError> {
    let mut normalized: Vec<String> = exact_paths
        .iter()
        .map(|path| {
            normal_adapter_path(path)
                .ok_or_else(|| AdapterError(format!("invalid exact adapter path `{path}`")))
        })
        .collect::<Result<_, _>>()?;
    normalized.sort();
    normalized.dedup();
    if normalized.len() != exact_paths.len()
        || normalized
            .iter()
            .any(|path| roots.iter().any(|root| under_root(path, root)))
    {
        return Err(AdapterError(
            "exact adapter paths must be distinct and outside every adapter root".to_string(),
        ));
    }
    Ok(normalized)
}

pub(crate) fn adapter_files(
    roots: &[String],
    exact_paths: &[String],
    files: &[AdapterFile],
) -> Result<BTreeMap<String, String>, AdapterError> {
    let mut desired = BTreeMap::new();
    for file in files {
        let Some(path) = normal_adapter_path(&file.path) else {
            return Err(AdapterError(format!(
                "invalid generated adapter path `{}`",
                file.path
            )));
        };
        if !roots.iter().any(|root| under_root(&path, root)) && !exact_paths.contains(&path) {
            return Err(AdapterError(format!(
                "generated adapter path `{path}` is outside every declared adapter family or exact path"
            )));
        }
        if desired
            .insert(path.clone(), file.contents.clone())
            .is_some()
        {
            return Err(AdapterError(format!(
                "duplicate generated adapter path `{path}`"
            )));
        }
    }
    if exact_paths.iter().any(|path| !desired.contains_key(path)) {
        return Err(AdapterError(
            "every exact adapter path must have generated bytes".to_string(),
        ));
    }
    Ok(desired)
}

pub(crate) fn normal_adapter_path(path: &str) -> Option<String> {
    let trimmed = path.trim_matches('/');
    if trimmed.is_empty()
        || trimmed.split('/').any(|segment| {
            segment.is_empty() || segment == "." || segment == ".." || segment.contains('\\')
        })
    {
        None
    } else {
        Some(trimmed.to_string())
    }
}

pub(crate) fn under_root(path: &str, root: &str) -> bool {
    path == root
        || path
            .strip_prefix(root)
            .is_some_and(|remainder| remainder.starts_with('/'))
}

#[cfg(test)]
mod tests {
    use super::*;

    /// The three ways a spawn can fail, and only three.
    ///
    /// The first two carry the statuses a shell already reports; everything
    /// else is this tool failing to run, which is not the child's fault and
    /// must not be reported as though it were.
    #[test]
    fn a_spawn_failure_is_classified_by_what_the_system_said() {
        use std::io::ErrorKind;

        for (kind, expected) in [
            (ErrorKind::NotFound, LaunchFailure::NotFound),
            (ErrorKind::PermissionDenied, LaunchFailure::NotExecutable),
            (ErrorKind::WouldBlock, LaunchFailure::Refused),
            (ErrorKind::Interrupted, LaunchFailure::Refused),
        ] {
            let error = LaunchError::from_spawn("child", &std::io::Error::from(kind));
            assert_eq!(error.failure, expected);
            assert!(error.reason.starts_with("`child` could not be started: "));
        }

        assert_eq!(
            LaunchError::refused("declined").failure,
            LaunchFailure::Refused
        );
    }

    #[derive(Clone)]
    struct ReadOnlyTree;

    impl Tree for ReadOnlyTree {
        fn read(&self, _: &str) -> Result<String, TreeError> {
            Err(TreeError::NotFound)
        }

        fn files(&self) -> Vec<String> {
            vec!["docs/guide.md".to_string()]
        }

        fn excluding(&self, _: &[String]) -> Box<dyn Tree> {
            Box::new(self.clone())
        }

        fn rooted_at(&self, _: &str) -> Result<Box<dyn Tree>, String> {
            Ok(Box::new(self.clone()))
        }
    }

    #[test]
    fn no_capability_ports_and_default_tree_behavior_fail_closed() {
        let arguments = vec!["tool".to_string()];
        let environment = BTreeMap::new();
        let adapters = AdapterTransaction {
            roots: vec!["adapters/test".to_string()],
            exact_paths: Vec::new(),
            files: Vec::new(),
        };
        let environment_transaction = EnvironmentTransaction { files: Vec::new() };
        let restore_transaction = EnvironmentRestoreTransaction { files: Vec::new() };

        assert!(NoAdapterStore.replace(".", &adapters).is_err());
        assert!(
            NoEnvironmentStore
                .create(".", &environment_transaction)
                .is_err()
        );
        assert!(
            NoEnvironmentStore
                .restore(".", &restore_transaction)
                .is_err()
        );
        assert!(
            NoToolchainRunner
                .run(ToolchainLaunch {
                    executable: "tool",
                    arguments: &arguments,
                    timeout_seconds: None,
                })
                .is_err()
        );
        assert!(
            NoMutationRunner
                .apply_index(MutationLaunch {
                    arguments: &arguments,
                    directory: ".",
                    environment: &environment,
                    selected_paths: &[],
                    revision: None,
                })
                .is_err()
        );
        assert!(
            NoMutationRunner
                .verify_clean(MutationLaunch {
                    arguments: &arguments,
                    directory: ".",
                    environment: &environment,
                    selected_paths: &[],
                    revision: Some("revision"),
                })
                .is_err()
        );
        assert!(
            NoLauncher
                .launch(Launch {
                    arguments: &arguments,
                    directory: ".",
                    surface: "local",
                    stdin: None,
                    environment: &environment,
                })
                .is_err()
        );

        let tree = ReadOnlyTree;
        assert!(tree.indexed_files().is_err());
        assert!(tree.changed_files("aaaaaaa", "bbbbbbb").is_err());
        assert!(tree.resolve_git_ref("main").is_err());
        assert!(tree.commit_messages("aaaaaaa", "bbbbbbb").is_err());
        assert!(tree.hook_message_file("COMMIT_EDITMSG").is_err());
        assert!(matches!(
            tree.read_no_follow("docs/guide.md"),
            Err(TreeError::NotFound)
        ));
        assert_eq!(
            tree.excluding(&["ignored".to_string()]).files(),
            vec!["docs/guide.md".to_string()]
        );
        assert_eq!(
            tree.rooted_at("docs").unwrap().files(),
            vec!["docs/guide.md".to_string()]
        );
        assert_eq!(tree.directories(), vec!["docs".to_string()]);
        assert_eq!(tree.children("docs"), vec!["docs/guide.md".to_string()]);
        assert!(tree.is_directory("docs"));
        assert!(!tree.exists("absent"));
    }

    #[test]
    fn memory_environment_store_refuses_invalid_or_conflicting_transactions() {
        let tree = MemoryTree::default();
        let store = MemoryEnvironmentStore::new(&tree);
        let invalid = EnvironmentTransaction {
            files: vec![EnvironmentFile {
                path: "../outside".to_string(),
                contents: "synthetic".to_string(),
            }],
        };
        assert!(store.create(".", &invalid).is_err());

        let declared = EnvironmentTransaction {
            files: vec![EnvironmentFile {
                path: ".env.fixture".to_string(),
                contents: "synthetic".to_string(),
            }],
        };
        assert!(store.create(".", &declared).is_ok());
        assert!(store.create(".", &declared).is_err());

        let invalid_restore = EnvironmentRestoreTransaction {
            files: vec![EnvironmentRestoreFile {
                path: "../outside".to_string(),
                contents: "synthetic".to_string(),
                replace: true,
            }],
        };
        assert!(store.restore(".", &invalid_restore).is_err());
        let conflict = EnvironmentRestoreTransaction {
            files: vec![EnvironmentRestoreFile {
                path: ".env.fixture".to_string(),
                contents: "replaced".to_string(),
                replace: false,
            }],
        };
        assert!(store.restore(".", &conflict).is_err());
        let replacement = EnvironmentRestoreTransaction {
            files: vec![EnvironmentRestoreFile {
                replace: true,
                ..conflict.files[0].clone()
            }],
        };
        assert!(store.restore(".", &replacement).is_ok());
        assert_eq!(tree.read(".env.fixture").unwrap(), "replaced");
    }

    #[test]
    fn memory_tree_rerooting_and_adapter_transactions_preserve_declared_boundaries() {
        let mut tree = MemoryTree::default();
        tree.write("nested/guide.md", "synthetic\n");
        tree.set_indexed_files(["nested/guide.md"]);
        tree.set_changed_files(
            "aaaaaaa",
            "bbbbbbbb",
            ["nested/guide.md", "outside/ignored.md"],
        );
        let rerooted = tree.rooted_at("nested").unwrap();
        assert_eq!(rerooted.read("guide.md").unwrap(), "synthetic\n");
        assert_eq!(
            rerooted.indexed_files().unwrap(),
            vec!["guide.md".to_string()]
        );
        assert_eq!(
            rerooted.changed_files("aaaaaaa", "bbbbbbbb").unwrap(),
            vec!["guide.md".to_string()]
        );

        tree.write("COMMIT_EDITMSG", "feat: synthetic hook\n");
        tree.set_hook_message_file("COMMIT_EDITMSG");
        assert_eq!(
            tree.hook_message_file("COMMIT_EDITMSG").unwrap(),
            "feat: synthetic hook\n"
        );
        assert!(matches!(
            tree.hook_message_file("another-message"),
            Err(TreeError::Unreadable(reason))
                if reason.contains("not Git's canonical COMMIT_EDITMSG")
        ));

        assert!(adapter_roots(&["adapters".to_string(), "adapters/nested".to_string()]).is_err());
        assert!(
            adapter_files(
                &["adapters".to_string()],
                &[],
                &[AdapterFile {
                    path: "../outside".to_string(),
                    contents: "synthetic".to_string(),
                }],
            )
            .is_err()
        );
        assert!(
            adapter_files(
                &["adapters".to_string()],
                &[],
                &[AdapterFile {
                    path: "other/file.md".to_string(),
                    contents: "synthetic".to_string(),
                }],
            )
            .is_err()
        );
        let repeated = AdapterFile {
            path: "adapters/file.md".to_string(),
            contents: "synthetic".to_string(),
        };
        assert!(
            adapter_files(
                &["adapters".to_string()],
                &[],
                &[repeated.clone(), repeated]
            )
            .is_err()
        );
        assert!(
            adapter_exact_paths(&["adapters".to_string()], &["adapters/file.md".to_string()])
                .is_err()
        );
        assert!(
            adapter_files(
                &["adapters".to_string()],
                &["CLAUDE.md".to_string()],
                &[AdapterFile {
                    path: "CLAUDE.md".to_string(),
                    contents: "@AGENTS.md\n".to_string(),
                }],
            )
            .is_ok()
        );
        assert!(
            adapter_files(&["adapters".to_string()], &["CLAUDE.md".to_string()], &[],).is_err()
        );
        assert_eq!(
            normal_adapter_path("/adapters/file.md/"),
            Some("adapters/file.md".to_string())
        );
        assert_eq!(normal_adapter_path("adapters/../file.md"), None);
        assert!(under_root("adapters/file.md", "adapters"));
        assert!(!under_root("adapter/file.md", "adapters"));
    }

    #[test]
    fn memory_tree_models_each_read_state_directory_shape_and_root_boundary() {
        let mut tree = MemoryTree::default();
        tree.write("docs/guide.md", "guide");
        tree.write("docs/link.md", "link");
        tree.write("docs/gone.md", "gone");
        tree.write("docs/binary.md", "bytes");
        tree.write("COMMIT_EDITMSG", "feat: message");
        tree.mark_link("docs/link.md");
        tree.mark_vanished("docs/gone.md");
        tree.mark_binary("docs/binary.md");
        tree.mark_directory("empty/nested");

        assert_eq!(normalise_directory("/docs/"), "docs/");
        assert_eq!(normalise_directory("."), "");
        assert!(matches!(
            tree.read("docs/gone.md"),
            Err(TreeError::NotFound)
        ));
        assert!(matches!(
            tree.read("docs/binary.md"),
            Err(TreeError::NotText)
        ));
        assert!(matches!(
            tree.read_no_follow("docs/link.md"),
            Err(TreeError::Unreadable(reason)) if reason.contains("symbolic link")
        ));
        assert_eq!(
            tree.read_no_follow("docs/guide.md")
                .expect("ordinary file stays readable"),
            "guide"
        );
        assert!(!tree.files().contains(&"docs/link.md".to_string()));
        assert!(tree.directories().contains(&"empty/nested".to_string()));
        assert!(
            tree.excluding(&["docs".to_string()])
                .files()
                .contains(&"docs/guide.md".to_string())
        );

        let rooted = tree.rooted_at("docs").expect("docs exists");
        assert_eq!(rooted.read("guide.md").expect("rerooted file"), "guide");
        assert!(tree.rooted_at("missing").is_err());
        assert!(tree.rooted_at(".").is_ok());

        tree.set_hook_message_file("COMMIT_EDITMSG");
        assert_eq!(
            tree.hook_message_file("COMMIT_EDITMSG")
                .expect("hook message"),
            "feat: message"
        );
        assert!(tree.hook_message_file("other").is_err());
        assert!(
            parents(&["a/b/c.md".to_string(), "a/d.md".to_string()]).contains(&"a/b".to_string())
        );
    }
}
