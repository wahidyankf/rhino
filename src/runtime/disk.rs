//! The concrete filesystem tree.
//!
//! Split into its own module so it can be named as a coverage exclusion.
//! Every line here talks to the operating system, which a unit test may not
//! do, so its proof is the integration and E2E adapters running the whole
//! corpus against it -- not a smaller number in a report. `DiskTree` stays
//! read-only; a later, separately named mutation runner will own the narrow
//! index and disposable-replay write boundary.

use super::{
    AdapterError, AdapterStore, AdapterTransaction, EnvironmentError, EnvironmentStore,
    EnvironmentTransaction, Launch, LaunchError, Launched, Launcher, Mutated, MutationError,
    MutationLaunch, MutationRunner, ToolchainError, ToolchainLaunch, ToolchainResult,
    ToolchainRunner, Tree, TreeError,
};
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;
use std::io::{Read, Write};
#[cfg(unix)]
use std::os::unix::fs::OpenOptionsExt;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::sync::atomic::{AtomicUsize, Ordering};
use std::time::{Duration, Instant};

static NEXT_SNAPSHOT: AtomicUsize = AtomicUsize::new(0);
static NEXT_ADAPTER_TRANSACTION: AtomicUsize = AtomicUsize::new(0);
static NEXT_ENVIRONMENT_TRANSACTION: AtomicUsize = AtomicUsize::new(0);

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

    /// Every directory at or under one directory, repository-relative.
    ///
    /// Links are skipped for the same reason the file walk skips them:
    /// following one can leave the repository, and a directory outside the root
    /// is not this repository's to report on.
    fn walk_directories(&self, directory: &Path, found: &mut Vec<String>) {
        let Ok(entries) = std::fs::read_dir(directory) else {
            return;
        };
        for entry in entries.flatten() {
            let path = entry.path();
            let Ok(metadata) = std::fs::symlink_metadata(&path) else {
                continue;
            };
            if metadata.is_symlink() || !metadata.is_dir() {
                continue;
            }
            let name = entry.file_name().to_string_lossy().into_owned();
            if self.excluded_directories.contains(&name) {
                continue;
            }
            if let Ok(relative) = path.strip_prefix(&self.root) {
                found.push(relative.to_string_lossy().replace('\\', "/"));
            }
            self.walk_directories(&path, found);
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

    fn indexed_files(&self) -> Result<Vec<String>, String> {
        let output = Command::new("git")
            .arg("-C")
            .arg(&self.root)
            .args(["diff", "--cached", "--name-only", "-z", "--"])
            .output()
            .map_err(|error| format!("could not read the Git index: {error}"))?;
        if !output.status.success() {
            return Err(format!(
                "Git refused the index query with status {}",
                output.status.code().unwrap_or(2)
            ));
        }
        let mut paths = Vec::new();
        for bytes in output.stdout.split(|byte| *byte == 0) {
            if bytes.is_empty() {
                continue;
            }
            let path = std::str::from_utf8(bytes)
                .map_err(|_| "the Git index contains a non-UTF-8 path".to_string())?;
            if !is_repository_path(path) {
                return Err(format!("the Git index returned an escaping path `{path}`"));
            }
            paths.push(path.to_string());
        }
        paths.sort();
        paths.dedup();
        Ok(paths)
    }

    fn resolve_git_ref(&self, reference: &str) -> Result<String, String> {
        if reference.is_empty()
            || reference.starts_with('-')
            || reference.contains(char::is_whitespace)
        {
            return Err("the declared fallback ref is invalid".to_string());
        }
        let revision = format!("{reference}^{{commit}}");
        let output = Command::new("git")
            .arg("-C")
            .arg(&self.root)
            .args(["rev-parse", "--verify", "--end-of-options", &revision])
            .output()
            .map_err(|error| format!("could not resolve the declared fallback ref: {error}"))?;
        if !output.status.success() {
            return Err("the declared fallback ref does not resolve to a commit".to_string());
        }
        let commit = std::str::from_utf8(&output.stdout)
            .map_err(|_| "Git returned a non-UTF-8 fallback commit".to_string())?
            .trim();
        if !commit
            .chars()
            .all(|character| character.is_ascii_hexdigit())
        {
            return Err("Git returned an invalid fallback commit".to_string());
        }
        Ok(commit.to_string())
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

    /// Every directory under the root, read from the filesystem rather than
    /// derived from the file list.
    ///
    /// The default answers from the files, which is right for every caller that
    /// wants the directories documents live in and wrong for the one rule that
    /// is about a directory holding nothing. Only a real listing can report
    /// that, so only the implementation that has one overrides this.
    fn directories(&self) -> Vec<String> {
        let mut found = Vec::new();
        self.walk_directories(&self.root, &mut found);
        found.sort();
        found
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

fn is_repository_path(path: &str) -> bool {
    !path.is_empty()
        && !Path::new(path).is_absolute()
        && path
            .split('/')
            .all(|segment| !segment.is_empty() && segment != "." && segment != "..")
}

/// The only real mutation boundary. It materializes a Git-index snapshot or a
/// disposable candidate checkout before a declared mutator starts, so the
/// ordinary tree and a CI checkout never become the child's write target.
pub struct DiskMutationRunner;

impl MutationRunner for DiskMutationRunner {
    fn apply_index(&self, launch: MutationLaunch<'_>) -> Result<Mutated, MutationError> {
        let root = Path::new(launch.directory);
        git(root, &["rev-parse", "--is-inside-work-tree"]).map_err(|_| {
            MutationError("this entry point has no index mutation boundary".to_string())
        })?;
        if launch.selected_paths.is_empty() {
            return Ok(Mutated {
                code: 0,
                changes: Vec::new(),
                divergences: Vec::new(),
            });
        }
        for path in launch.selected_paths {
            if !is_repository_path(path) {
                return Err(MutationError(format!(
                    "selected index path escapes its root `{path}`"
                )));
            }
        }

        let snapshot = SnapshotDirectory::new("index")?;
        let prefix = format!("--prefix={}/", snapshot.root.display());
        git(root, &["checkout-index", &prefix, "-a"])?;
        let before = snapshot.files()?;
        let code = run_mutator(&snapshot.root, &launch)?;
        if code != 0 {
            return Ok(Mutated {
                code,
                changes: Vec::new(),
                divergences: Vec::new(),
            });
        }
        let after = snapshot.files()?;
        let changed = changed_paths(&before, &after);
        for path in &changed {
            if !launch
                .selected_paths
                .iter()
                .any(|selected| selected == path)
            {
                return Err(MutationError(format!(
                    "mutator changed `{path}` outside its selected index paths"
                )));
            }
        }

        let mut divergences = Vec::new();
        for path in &changed {
            match after.get(path) {
                Some(_) => {
                    update_index_blob(root, &snapshot.root.join(path), path)?;
                    if before.get(path) == file_fingerprint(&root.join(path))?.as_ref() {
                        atomic_replace(&root.join(path), &snapshot.root.join(path))?;
                    } else {
                        divergences.push(path.clone());
                    }
                }
                None => {
                    git(root, &["update-index", "--remove", "--", path])?;
                    if before.get(path) == file_fingerprint(&root.join(path))?.as_ref() {
                        std::fs::remove_file(root.join(path)).map_err(|error| {
                            MutationError(format!(
                                "could not synchronize deleted `{path}`: {error}"
                            ))
                        })?;
                    } else {
                        divergences.push(path.clone());
                    }
                }
            }
        }
        Ok(Mutated {
            code: 0,
            changes: changed,
            divergences,
        })
    }

    fn verify_clean(&self, launch: MutationLaunch<'_>) -> Result<Mutated, MutationError> {
        let Some(revision) = launch.revision else {
            return Err(MutationError(
                "a disposable pull-request replay requires an explicit candidate revision"
                    .to_string(),
            ));
        };
        let snapshot = SnapshotDirectory::reserve("pull-request")?;
        let root = Path::new(launch.directory);
        git(
            root,
            &[
                "worktree",
                "add",
                "--detach",
                "--force",
                snapshot.root.to_string_lossy().as_ref(),
                revision,
            ],
        )?;
        let result = (|| {
            let code = run_mutator(&snapshot.root, &launch)?;
            if code != 0 {
                return Ok(Mutated {
                    code,
                    changes: Vec::new(),
                    divergences: Vec::new(),
                });
            }
            let mut changes = git_paths(
                &snapshot.root,
                &[
                    "diff",
                    "--no-ext-diff",
                    "--no-renames",
                    "--name-only",
                    "-z",
                    "HEAD",
                ],
            )?;
            changes.extend(git_paths(
                &snapshot.root,
                &["ls-files", "--others", "--exclude-standard", "-z"],
            )?);
            changes.sort();
            changes.dedup();
            Ok(Mutated {
                code: if changes.is_empty() { 0 } else { 1 },
                changes,
                divergences: Vec::new(),
            })
        })();
        let cleanup = git(
            root,
            &[
                "worktree",
                "remove",
                "--force",
                snapshot.root.to_string_lossy().as_ref(),
            ],
        );
        match (result, cleanup) {
            (Ok(mutated), Ok(_)) => Ok(mutated),
            (Err(error), Ok(_)) => Err(error),
            (_, Err(error)) => Err(error),
        }
    }
}

struct SnapshotDirectory {
    root: PathBuf,
}

impl SnapshotDirectory {
    fn new(kind: &str) -> Result<Self, MutationError> {
        let root = Self::path(kind);
        std::fs::create_dir(&root)
            .map_err(|error| MutationError(format!("could not create {kind} snapshot: {error}")))?;
        Ok(Self { root })
    }

    fn reserve(kind: &str) -> Result<Self, MutationError> {
        let root = Self::path(kind);
        if root.exists() {
            return Err(MutationError(format!(
                "reserved {kind} snapshot path already exists"
            )));
        }
        Ok(Self { root })
    }

    fn path(kind: &str) -> PathBuf {
        let ordinal = NEXT_SNAPSHOT.fetch_add(1, Ordering::Relaxed);
        std::env::temp_dir().join(format!("rhino-{kind}-{}-{ordinal}", std::process::id()))
    }

    fn files(&self) -> Result<BTreeMap<String, FileFingerprint>, MutationError> {
        let mut files = BTreeMap::new();
        collect_snapshot_files(&self.root, &self.root, &mut files)?;
        Ok(files)
    }
}

impl Drop for SnapshotDirectory {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.root);
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct FileFingerprint([u8; 32]);

fn collect_snapshot_files(
    root: &Path,
    directory: &Path,
    files: &mut BTreeMap<String, FileFingerprint>,
) -> Result<(), MutationError> {
    for entry in std::fs::read_dir(directory)
        .map_err(|error| MutationError(format!("could not read snapshot: {error}")))?
        .flatten()
    {
        let path = entry.path();
        let metadata = std::fs::symlink_metadata(&path)
            .map_err(|error| MutationError(format!("could not inspect snapshot: {error}")))?;
        if metadata.is_symlink() {
            return Err(MutationError(
                "snapshot contains a link that leaves its boundary".to_string(),
            ));
        }
        if metadata.is_dir() {
            collect_snapshot_files(root, &path, files)?;
            continue;
        }
        let relative = path
            .strip_prefix(root)
            .map_err(|_| MutationError("snapshot path leaves its root".to_string()))?
            .to_string_lossy()
            .replace('\\', "/");
        if !is_repository_path(&relative) {
            return Err(MutationError(format!(
                "snapshot contains an escaping path `{relative}`"
            )));
        }
        let fingerprint = file_fingerprint(&path)?.ok_or_else(|| {
            MutationError(format!(
                "snapshot file `{relative}` disappeared while it was inspected"
            ))
        })?;
        files.insert(relative, fingerprint);
    }
    Ok(())
}

fn file_fingerprint(path: &Path) -> Result<Option<FileFingerprint>, MutationError> {
    match std::fs::read(path) {
        Ok(bytes) => Ok(Some(FileFingerprint(Sha256::digest(bytes).into()))),
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(None),
        Err(error) => Err(MutationError(format!(
            "could not read `{}`: {error}",
            path.display()
        ))),
    }
}

fn changed_paths(
    before: &BTreeMap<String, FileFingerprint>,
    after: &BTreeMap<String, FileFingerprint>,
) -> Vec<String> {
    before
        .keys()
        .chain(after.keys())
        .collect::<std::collections::BTreeSet<_>>()
        .into_iter()
        .filter(|path| before.get(*path) != after.get(*path))
        .cloned()
        .collect()
}

fn run_mutator(snapshot: &Path, launch: &MutationLaunch<'_>) -> Result<i32, MutationError> {
    let Some((program, arguments)) = launch.arguments.split_first() else {
        return Err(MutationError("the gate declares no command".to_string()));
    };
    let output = Command::new(program)
        .args(arguments)
        .current_dir(snapshot)
        .envs(launch.environment)
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .output()
        .map_err(|error| MutationError(format!("`{program}` could not be started: {error}")))?;
    output
        .status
        .code()
        .ok_or_else(|| MutationError(format!("`{program}` was ended by a signal")))
}

fn git(root: &Path, arguments: &[&str]) -> Result<std::process::Output, MutationError> {
    let output = Command::new("git")
        .arg("-C")
        .arg(root)
        .args(arguments)
        .output()
        .map_err(|error| MutationError(format!("Git could not start: {error}")))?;
    if output.status.success() {
        Ok(output)
    } else {
        Err(MutationError(format!(
            "Git command failed with status {}",
            output.status.code().unwrap_or(2)
        )))
    }
}

fn git_paths(root: &Path, arguments: &[&str]) -> Result<Vec<String>, MutationError> {
    let output = git(root, arguments)?;
    let mut paths = Vec::new();
    for record in output.stdout.split(|byte| *byte == 0) {
        if record.is_empty() {
            continue;
        }
        let path = std::str::from_utf8(record)
            .map_err(|_| MutationError("Git returned a non-UTF-8 changed path".to_string()))?;
        if !is_repository_path(path) {
            return Err(MutationError(format!(
                "Git returned an escaping changed path `{path}`"
            )));
        }
        paths.push(path.to_string());
    }
    paths.sort();
    paths.dedup();
    Ok(paths)
}

fn update_index_blob(root: &Path, source: &Path, path: &str) -> Result<(), MutationError> {
    let mode = git(root, &["ls-files", "--stage", "--", path])?;
    let mode = std::str::from_utf8(&mode.stdout)
        .ok()
        .and_then(|text| text.split_whitespace().next())
        .filter(|mode| !mode.is_empty())
        .ok_or_else(|| {
            MutationError(format!(
                "selected path `{path}` is absent from the Git index"
            ))
        })?;
    let hash = git(
        root,
        &["hash-object", "-w", "--", source.to_string_lossy().as_ref()],
    )?;
    let hash = std::str::from_utf8(&hash.stdout)
        .map_err(|_| MutationError("Git returned a non-UTF-8 object ID".to_string()))?
        .trim();
    let cache_info = format!("{mode},{hash},{path}");
    git(root, &["update-index", "--cacheinfo", &cache_info])?;
    Ok(())
}

fn atomic_replace(target: &Path, source: &Path) -> Result<(), MutationError> {
    let parent = target
        .parent()
        .ok_or_else(|| MutationError("working-tree target has no parent".to_string()))?;
    let ordinal = NEXT_SNAPSHOT.fetch_add(1, Ordering::Relaxed);
    let temporary = parent.join(format!(".rhino-index-{ordinal}"));
    let bytes = std::fs::read(source)
        .map_err(|error| MutationError(format!("could not read snapshot replacement: {error}")))?;
    let result = (|| {
        let mut file = std::fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .open(&temporary)
            .map_err(|error| {
                MutationError(format!("could not create atomic replacement: {error}"))
            })?;
        file.write_all(&bytes).map_err(|error| {
            MutationError(format!("could not write atomic replacement: {error}"))
        })?;
        file.sync_all().map_err(|error| {
            MutationError(format!("could not flush atomic replacement: {error}"))
        })?;
        std::fs::rename(&temporary, target)
            .map_err(|error| MutationError(format!("could not replace working-tree file: {error}")))
    })();
    if result.is_err() {
        let _ = std::fs::remove_file(&temporary);
    }
    result
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
            .envs(launch.environment)
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

/// The concrete canonical-adapter write boundary. It stages every declared
/// family below the repository root, rejects links before a path is touched,
/// and restores already-moved families if a later rename fails.
pub struct DiskAdapterStore;

impl AdapterStore for DiskAdapterStore {
    fn replace(&self, root: &str, transaction: &AdapterTransaction) -> Result<(), AdapterError> {
        let root = PathBuf::from(root);
        let metadata = std::fs::symlink_metadata(&root)
            .map_err(|error| AdapterError(format!("cannot inspect adapter root: {error}")))?;
        if !metadata.is_dir() || metadata.is_symlink() {
            return Err(AdapterError(
                "the repository root is not a non-link directory".to_string(),
            ));
        }

        let roots = super::adapter_roots(&transaction.roots)?;
        let desired = super::adapter_files(&roots, &transaction.files)?;
        for managed in &roots {
            ensure_no_links(&root, managed)?;
        }

        let ordinal = NEXT_ADAPTER_TRANSACTION.fetch_add(1, Ordering::Relaxed);
        let stage_name = format!(
            ".rhino-adapter-transaction-{}-{ordinal}",
            std::process::id()
        );
        let stage = root.join(&stage_name);
        std::fs::create_dir(&stage)
            .map_err(|error| AdapterError(format!("cannot create adapter transaction: {error}")))?;

        let result = replace_adapter_families(&root, &stage, &roots, &desired);
        let _ = std::fs::remove_dir_all(&stage);
        result
    }
}

/// The no-overwrite local-environment boundary. It stages every file below the
/// selected repository root, rejects a link at any existing path component,
/// and removes only files it installed if a later rename fails.
pub struct DiskEnvironmentStore;

impl EnvironmentStore for DiskEnvironmentStore {
    fn create(
        &self,
        root: &str,
        transaction: &EnvironmentTransaction,
    ) -> Result<(), EnvironmentError> {
        let root = PathBuf::from(root);
        let metadata = std::fs::symlink_metadata(&root).map_err(|error| {
            EnvironmentError(format!("cannot inspect environment root: {error}"))
        })?;
        if !metadata.is_dir() || metadata.is_symlink() {
            return Err(EnvironmentError(
                "the repository root is not a non-link directory".to_string(),
            ));
        }
        let mut files = BTreeMap::new();
        for file in &transaction.files {
            let Some(path) = super::normal_adapter_path(&file.path) else {
                return Err(EnvironmentError(format!(
                    "invalid environment target `{}`",
                    file.path
                )));
            };
            if files.insert(path.clone(), file.contents.clone()).is_some() {
                return Err(EnvironmentError(format!(
                    "duplicate environment target `{path}`"
                )));
            }
            ensure_environment_path_is_safe(&root, &path)?;
            if root.join(&path).exists() {
                return Err(EnvironmentError(format!(
                    "environment target `{path}` already exists"
                )));
            }
        }
        let ordinal = NEXT_ENVIRONMENT_TRANSACTION.fetch_add(1, Ordering::Relaxed);
        let stage = root.join(format!(
            ".rhino-environment-transaction-{}-{ordinal}",
            std::process::id()
        ));
        std::fs::create_dir(&stage).map_err(|error| {
            EnvironmentError(format!("cannot stage environment transaction: {error}"))
        })?;
        let result = install_environment_files(&root, &stage, &files);
        let _ = std::fs::remove_dir_all(&stage);
        result
    }

    fn restore(
        &self,
        root: &str,
        transaction: &super::EnvironmentRestoreTransaction,
    ) -> Result<(), EnvironmentError> {
        let root = PathBuf::from(root);
        let metadata = std::fs::symlink_metadata(&root).map_err(|error| {
            EnvironmentError(format!("cannot inspect environment root: {error}"))
        })?;
        if !metadata.is_dir() || metadata.is_symlink() {
            return Err(EnvironmentError(
                "the repository root is not a non-link directory".to_string(),
            ));
        }
        let mut files = BTreeMap::new();
        for file in &transaction.files {
            let Some(path) = super::normal_adapter_path(&file.path) else {
                return Err(EnvironmentError(format!(
                    "invalid environment target `{}`",
                    file.path
                )));
            };
            if files
                .insert(path.clone(), (file.contents.clone(), file.replace))
                .is_some()
            {
                return Err(EnvironmentError(format!(
                    "duplicate environment target `{path}`"
                )));
            }
            ensure_environment_path_is_safe(&root, &path)?;
            if let Ok(metadata) = std::fs::symlink_metadata(root.join(&path)) {
                if metadata.is_symlink() || metadata.is_dir() {
                    return Err(EnvironmentError(format!(
                        "environment target `{path}` is not a regular non-link file"
                    )));
                }
                if !file.replace {
                    return Err(EnvironmentError(format!(
                        "environment target `{path}` already exists"
                    )));
                }
            }
        }
        let ordinal = NEXT_ENVIRONMENT_TRANSACTION.fetch_add(1, Ordering::Relaxed);
        let stage = root.join(format!(
            ".rhino-environment-restore-{}-{ordinal}",
            std::process::id()
        ));
        std::fs::create_dir(&stage).map_err(|error| {
            EnvironmentError(format!("cannot stage environment restore: {error}"))
        })?;
        let result = restore_environment_files(&root, &stage, &files);
        let _ = std::fs::remove_dir_all(&stage);
        result
    }
}

fn install_environment_files(
    root: &Path,
    stage: &Path,
    files: &BTreeMap<String, String>,
) -> Result<(), EnvironmentError> {
    for (path, contents) in files {
        let staged = stage.join(path);
        let parent = staged
            .parent()
            .ok_or_else(|| EnvironmentError("an environment target has no parent".to_string()))?;
        std::fs::create_dir_all(parent).map_err(|error| {
            EnvironmentError(format!("cannot stage environment target `{path}`: {error}"))
        })?;
        let mut file = private_environment_file(&staged).map_err(|error| {
            EnvironmentError(format!("cannot create environment stage `{path}`: {error}"))
        })?;
        file.write_all(contents.as_bytes()).map_err(|error| {
            EnvironmentError(format!("cannot write environment stage `{path}`: {error}"))
        })?;
        file.sync_all().map_err(|error| {
            EnvironmentError(format!("cannot sync environment stage `{path}`: {error}"))
        })?;
    }

    let mut installed = Vec::new();
    for path in files.keys() {
        let target = root.join(path);
        let parent = target
            .parent()
            .ok_or_else(|| EnvironmentError("an environment target has no parent".to_string()))?;
        if let Err(error) = std::fs::create_dir_all(parent) {
            remove_environment_files(&installed);
            return Err(EnvironmentError(format!(
                "cannot create environment parent `{path}`: {error}"
            )));
        }
        if let Err(error) = ensure_environment_path_is_safe(root, path) {
            remove_environment_files(&installed);
            return Err(error);
        }
        if target.exists() {
            remove_environment_files(&installed);
            return Err(EnvironmentError(format!(
                "environment target `{path}` already exists"
            )));
        }
        if let Err(error) = std::fs::rename(stage.join(path), &target) {
            remove_environment_files(&installed);
            return Err(EnvironmentError(format!(
                "cannot install environment target `{path}`: {error}"
            )));
        }
        installed.push(target);
    }
    Ok(())
}

fn restore_environment_files(
    root: &Path,
    stage: &Path,
    files: &BTreeMap<String, (String, bool)>,
) -> Result<(), EnvironmentError> {
    let staged = stage.join("desired");
    std::fs::create_dir(&staged)
        .map_err(|error| EnvironmentError(format!("cannot stage environment restore: {error}")))?;
    for (path, (contents, _)) in files {
        let target = staged.join(path);
        let parent = target
            .parent()
            .ok_or_else(|| EnvironmentError("an environment target has no parent".to_string()))?;
        std::fs::create_dir_all(parent).map_err(|error| {
            EnvironmentError(format!("cannot stage environment target `{path}`: {error}"))
        })?;
        let mut file = private_environment_file(&target).map_err(|error| {
            EnvironmentError(format!("cannot create environment stage `{path}`: {error}"))
        })?;
        file.write_all(contents.as_bytes()).map_err(|error| {
            EnvironmentError(format!("cannot write environment stage `{path}`: {error}"))
        })?;
        file.sync_all().map_err(|error| {
            EnvironmentError(format!("cannot sync environment stage `{path}`: {error}"))
        })?;
    }

    let backups = stage.join("backups");
    std::fs::create_dir(&backups).map_err(|error| {
        EnvironmentError(format!("cannot prepare environment rollback: {error}"))
    })?;
    let mut installed: Vec<(PathBuf, Option<PathBuf>)> = Vec::new();
    for (index, (path, (_, replace))) in files.iter().enumerate() {
        let target = root.join(path);
        let parent = target
            .parent()
            .ok_or_else(|| EnvironmentError("an environment target has no parent".to_string()))?;
        if let Err(error) = std::fs::create_dir_all(parent) {
            restore_environment_files_after_failure(&installed);
            return Err(EnvironmentError(format!(
                "cannot create environment parent `{path}`: {error}"
            )));
        }
        if let Err(error) = ensure_environment_path_is_safe(root, path) {
            restore_environment_files_after_failure(&installed);
            return Err(error);
        }
        let backup = if target.exists() {
            if !replace {
                restore_environment_files_after_failure(&installed);
                return Err(EnvironmentError(format!(
                    "environment target `{path}` already exists"
                )));
            }
            let backup = backups.join(index.to_string());
            if let Err(error) = std::fs::rename(&target, &backup) {
                restore_environment_files_after_failure(&installed);
                return Err(EnvironmentError(format!(
                    "cannot preserve environment target `{path}`: {error}"
                )));
            }
            Some(backup)
        } else {
            None
        };
        if let Err(error) = std::fs::rename(staged.join(path), &target) {
            if let Some(backup) = &backup {
                let _ = std::fs::rename(backup, &target);
            }
            restore_environment_files_after_failure(&installed);
            return Err(EnvironmentError(format!(
                "cannot restore environment target `{path}`: {error}"
            )));
        }
        installed.push((target, backup));
    }
    Ok(())
}

fn restore_environment_files_after_failure(installed: &[(PathBuf, Option<PathBuf>)]) {
    for (target, backup) in installed.iter().rev() {
        let _ = std::fs::remove_file(target);
        if let Some(backup) = backup {
            let _ = std::fs::rename(backup, target);
        }
    }
}

fn private_environment_file(path: &Path) -> std::io::Result<std::fs::File> {
    let mut options = std::fs::OpenOptions::new();
    options.write(true).create_new(true);
    #[cfg(unix)]
    options.mode(0o600);
    options.open(path)
}

fn remove_environment_files(files: &[PathBuf]) {
    for file in files.iter().rev() {
        let _ = std::fs::remove_file(file);
    }
}

fn ensure_environment_path_is_safe(root: &Path, relative: &str) -> Result<(), EnvironmentError> {
    let mut current = root.to_path_buf();
    for segment in relative.split('/') {
        current.push(segment);
        match std::fs::symlink_metadata(&current) {
            Ok(metadata) if metadata.is_symlink() => {
                return Err(EnvironmentError(format!(
                    "environment target `{relative}` crosses a symbolic link"
                )));
            }
            Ok(_) | Err(_) => {}
        }
    }
    Ok(())
}

fn replace_adapter_families(
    root: &Path,
    stage: &Path,
    roots: &[String],
    desired: &BTreeMap<String, String>,
) -> Result<(), AdapterError> {
    let staged = stage.join("desired");
    std::fs::create_dir(&staged)
        .map_err(|error| AdapterError(format!("cannot stage adapter bytes: {error}")))?;
    for (path, contents) in desired {
        let target = staged.join(path);
        let parent = target
            .parent()
            .ok_or_else(|| AdapterError("a generated adapter path has no parent".to_string()))?;
        std::fs::create_dir_all(parent)
            .map_err(|error| AdapterError(format!("cannot create adapter stage path: {error}")))?;
        let mut file = std::fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .open(&target)
            .map_err(|error| AdapterError(format!("cannot stage adapter file: {error}")))?;
        file.write_all(contents.as_bytes())
            .map_err(|error| AdapterError(format!("cannot write adapter stage file: {error}")))?;
        file.sync_all()
            .map_err(|error| AdapterError(format!("cannot sync adapter stage file: {error}")))?;
    }

    let backups = stage.join("backups");
    std::fs::create_dir(&backups)
        .map_err(|error| AdapterError(format!("cannot prepare adapter rollback: {error}")))?;
    let mut moved: Vec<(PathBuf, PathBuf, bool)> = Vec::new();

    for (index, managed) in roots.iter().enumerate() {
        let target = root.join(managed);
        let staged_family = staged.join(managed);
        if !staged_family.is_dir() {
            restore_adapter_families(&moved);
            return Err(AdapterError(format!(
                "adapter transaction has no generated family for `{managed}`"
            )));
        }
        let parent = target
            .parent()
            .ok_or_else(|| AdapterError("an adapter family has no parent directory".to_string()))?;
        if let Err(error) = std::fs::create_dir_all(parent) {
            restore_adapter_families(&moved);
            return Err(AdapterError(format!(
                "cannot create parent for adapter family `{managed}`: {error}"
            )));
        }
        let backup = backups.join(index.to_string());
        let had_current = target.exists();
        if had_current {
            let metadata = std::fs::symlink_metadata(&target).map_err(|error| {
                AdapterError(format!(
                    "cannot inspect current adapter family `{managed}`: {error}"
                ))
            })?;
            if metadata.is_symlink() || !metadata.is_dir() {
                restore_adapter_families(&moved);
                return Err(AdapterError(format!(
                    "adapter family `{managed}` is not a non-link directory"
                )));
            }
            if let Err(error) = std::fs::rename(&target, &backup) {
                restore_adapter_families(&moved);
                return Err(AdapterError(format!(
                    "cannot move current adapter family `{managed}` aside: {error}"
                )));
            }
        }
        moved.push((target.clone(), backup, false));
        if let Err(error) = std::fs::rename(&staged_family, &target) {
            restore_adapter_families(&moved);
            return Err(AdapterError(format!(
                "cannot install adapter family `{managed}`: {error}"
            )));
        }
        let latest = moved
            .last_mut()
            .expect("the target was recorded before installing it");
        latest.2 = true;
    }
    Ok(())
}

fn restore_adapter_families(moved: &[(PathBuf, PathBuf, bool)]) {
    for (target, backup, installed) in moved.iter().rev() {
        if *installed && target.exists() {
            let _ = std::fs::remove_dir_all(target);
        }
        if backup.exists() {
            let _ = std::fs::rename(backup, target);
        }
    }
}

fn ensure_no_links(root: &Path, relative: &str) -> Result<(), AdapterError> {
    let mut current = root.to_path_buf();
    for segment in relative.split('/') {
        current.push(segment);
        match std::fs::symlink_metadata(&current) {
            Ok(metadata) if metadata.is_symlink() => {
                return Err(AdapterError(format!(
                    "adapter path `{relative}` crosses a symbolic link"
                )));
            }
            Ok(_) => {}
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => break,
            Err(error) => {
                return Err(AdapterError(format!(
                    "cannot inspect adapter path `{relative}`: {error}"
                )));
            }
        }
    }
    Ok(())
}

/// The no-shell process boundary for declared toolchain commands. Captured
/// probe bytes stay at this boundary for version comparison and never enter a
/// Rhino report; a bound keeps a hostile child from retaining unbounded output.
pub struct DiskToolchainRunner;

impl ToolchainRunner for DiskToolchainRunner {
    fn run(&self, launch: ToolchainLaunch<'_>) -> Result<ToolchainResult, ToolchainError> {
        if launch.executable.trim().is_empty() || launch.executable.starts_with('-') {
            return Err(ToolchainError(
                "the declared executable is invalid".to_string(),
            ));
        }
        const MAX_TOOLCHAIN_OUTPUT: u64 = 64 * 1024;
        let mut child = Command::new(launch.executable)
            .args(launch.arguments)
            .stdin(Stdio::null())
            .stdout(Stdio::piped())
            .stderr(Stdio::null())
            .spawn()
            .map_err(|error| {
                ToolchainError(format!("could not start declared toolchain: {error}"))
            })?;
        let stdout = child.stdout.take().ok_or_else(|| {
            ToolchainError("the declared toolchain has no captured output stream".to_string())
        })?;
        let reader = std::thread::spawn(move || {
            let mut output = Vec::new();
            stdout
                .take(MAX_TOOLCHAIN_OUTPUT + 1)
                .read_to_end(&mut output)
                .map(|_| output)
                .map_err(|error| error.to_string())
        });
        let started = Instant::now();
        let timeout = launch.timeout_seconds.map(Duration::from_secs);
        let status = loop {
            match child.try_wait() {
                Ok(Some(status)) => break status,
                Ok(None) if timeout.is_some_and(|limit| started.elapsed() >= limit) => {
                    let _ = child.kill();
                    let _ = child.wait();
                    let _ = reader.join();
                    return Err(ToolchainError(
                        "the declared toolchain exceeded its timeout".to_string(),
                    ));
                }
                Ok(None) => std::thread::sleep(Duration::from_millis(10)),
                Err(error) => {
                    let _ = child.kill();
                    let _ = child.wait();
                    let _ = reader.join();
                    return Err(ToolchainError(format!(
                        "could not observe declared toolchain: {error}"
                    )));
                }
            }
        };
        let output = reader
            .join()
            .map_err(|_| ToolchainError("the toolchain output reader panicked".to_string()))?
            .map_err(|error| ToolchainError(format!("could not read toolchain output: {error}")))?;
        if output.len() as u64 > MAX_TOOLCHAIN_OUTPUT {
            return Err(ToolchainError(
                "the declared toolchain emitted too much output".to_string(),
            ));
        }
        let Some(code) = status.code() else {
            return Err(ToolchainError(
                "the declared toolchain ended by signal".to_string(),
            ));
        };
        let stdout = String::from_utf8(output).map_err(|_| {
            ToolchainError("the declared toolchain emitted non-text output".to_string())
        })?;
        Ok(ToolchainResult { code, stdout })
    }
}
