//! A real directory for the adapters that need one.
//!
//! Unit runs entirely in memory; integration and E2E do not, and both need the
//! declared repository to exist on disk. The directory is created under the
//! system temporary root, named uniquely per process and per scenario, and
//! removed when the guard drops -- including when a scenario fails, which is
//! when a leaked directory would otherwise accumulate fastest.

use crate::fixtures;
use crate::world::{Observation, Repository};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicUsize, Ordering};

static NEXT: AtomicUsize = AtomicUsize::new(0);

pub struct Sandbox {
    root: PathBuf,
}

impl Sandbox {
    /// Materialise the declared repository. Uniqueness comes from the process
    /// id and a counter rather than a clock, so two adapters running at once
    /// cannot land in the same directory and no test depends on wall time.
    pub fn build(repository: &Repository<'_>) -> Self {
        // A disk sandbox cannot make a file disappear between the walk and the
        // read without a second thread racing the run under inspection. Refuse
        // the precondition rather than build a repository that quietly does not
        // hold it: a fixture that cannot express a Given must fail loudly, or a
        // scenario moved here later would report proof it never had.
        assert!(
            repository.vanished.is_empty(),
            "a disk sandbox cannot express a file vanishing mid-run: {:?}\n\
             this scenario belongs at the unit boundary, with an Exemption here",
            repository.vanished
        );
        let ordinal = NEXT.fetch_add(1, Ordering::Relaxed);
        let root =
            std::env::temp_dir().join(format!("rhino-spec-{}-{ordinal}", std::process::id()));
        // `create_dir`, not `create_dir_all`: a root that already exists is a
        // leaked fixture from an earlier run, and silently reusing it would
        // mean a scenario inspecting files it never declared.
        std::fs::create_dir(&root).expect("the sandbox root is new and creatable");

        let sandbox = Self { root };
        for (path, content) in repository.files {
            sandbox.write(path, content);
        }
        if !repository.declaration.absent {
            sandbox.write(
                fixtures::CONFIG_PATH,
                &fixtures::render(repository.declaration),
            );
        }
        for path in repository.links {
            sandbox.link(path);
        }
        // Permissions last: a file has to be written before it can be closed.
        for path in repository.unreadable {
            sandbox.seal(path);
        }
        sandbox
    }

    /// Replace a written file with a symbolic link pointing outside the
    /// repository, which is the case the walk has to refuse to follow.
    fn link(&self, path: &str) {
        let target = self.root.join(path);
        let _ = std::fs::remove_file(&target);
        if let Some(parent) = target.parent() {
            std::fs::create_dir_all(parent).expect("the link's parent directory is creatable");
        }
        std::os::unix::fs::symlink("/dev/null", &target).expect("the link is creatable");
    }

    /// Make a written file unreadable, so opening it fails the way a
    /// permission-denied file fails in a real repository.
    fn seal(&self, path: &str) {
        use std::os::unix::fs::PermissionsExt;
        let target = self.root.join(path);
        std::fs::set_permissions(&target, std::fs::Permissions::from_mode(0o000))
            .expect("the fixture's permissions are settable");
    }

    pub fn root(&self) -> &Path {
        &self.root
    }

    fn write(&self, path: &str, content: &str) {
        let target = self.root.join(path);
        if let Some(parent) = target.parent() {
            std::fs::create_dir_all(parent).expect("the fixture's parent directory is creatable");
        }
        std::fs::write(&target, content).expect("the fixture file is writable");
    }
}

impl Drop for Sandbox {
    fn drop(&mut self) {
        // Restore readability first: a sealed file cannot be removed from a
        // directory the test no longer has permission to traverse.
        restore(&self.root);
        // Best effort: a failure to remove a temporary directory must not turn
        // a passing scenario into a failing one, or mask why a failing one
        // failed.
        let _ = std::fs::remove_dir_all(&self.root);
    }
}

/// Give every file back its ordinary permissions so the tree can be removed.
fn restore(root: &Path) {
    use std::os::unix::fs::PermissionsExt;
    let Ok(entries) = std::fs::read_dir(root) else {
        return;
    };
    for entry in entries.flatten() {
        let path = entry.path();
        let Ok(metadata) = std::fs::symlink_metadata(&path) else {
            continue;
        };
        if metadata.is_symlink() {
            continue;
        }
        if metadata.is_dir() {
            restore(&path);
        } else {
            let _ = std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o644));
        }
    }
}

/// Every path under `root`, walked with `std::fs` directly.
///
/// Deliberately not routed through `Tree`: a validator that wrote into a
/// directory the port excludes, or followed a link and wrote through it, would
/// be invisible to an observation taken through the same port it used. A
/// sealed file is present with no content, and a link is recorded by its
/// target rather than followed.
pub fn observe(root: &Path) -> Observation {
    let mut found = crate::world::working_directory();
    walk(root, root, &mut found);
    found
}

fn walk(root: &Path, directory: &Path, found: &mut Observation) {
    let Ok(entries) = std::fs::read_dir(directory) else {
        return;
    };
    for entry in entries.flatten() {
        let path = entry.path();
        let Ok(metadata) = std::fs::symlink_metadata(&path) else {
            continue;
        };
        let Ok(relative) = path.strip_prefix(root) else {
            continue;
        };
        let name = relative.to_string_lossy().replace('\\', "/");

        if metadata.is_symlink() {
            found.insert(
                name,
                std::fs::read_link(&path)
                    .ok()
                    .map(|target| target.to_string_lossy().into_owned()),
            );
        } else if metadata.is_dir() {
            found.insert(format!("{name}/"), None);
            walk(root, &path, found);
        } else {
            found.insert(
                name,
                std::fs::read(&path).ok().map(|bytes| format!("{bytes:?}")),
            );
        }
    }
}
