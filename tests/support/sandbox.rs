//! A real directory for the adapters that need one.
//!
//! Unit runs entirely in memory; integration and E2E do not, and both need the
//! declared repository to exist on disk. The directory is created under the
//! system temporary root, named uniquely per process and per scenario, and
//! removed when the guard drops -- including when a scenario fails, which is
//! when a leaked directory would otherwise accumulate fastest.

use crate::fixtures;
use crate::world::Declaration;
use std::collections::BTreeMap;
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
    pub fn build(declaration: &Declaration, files: &BTreeMap<String, String>) -> Self {
        let ordinal = NEXT.fetch_add(1, Ordering::Relaxed);
        let root =
            std::env::temp_dir().join(format!("rhino-spec-{}-{ordinal}", std::process::id()));
        // `create_dir`, not `create_dir_all`: a root that already exists is a
        // leaked fixture from an earlier run, and silently reusing it would
        // mean a scenario inspecting files it never declared.
        std::fs::create_dir(&root).expect("the sandbox root is new and creatable");

        let sandbox = Self { root };
        for (path, content) in files {
            sandbox.write(path, content);
        }
        if declaration.present {
            sandbox.write(fixtures::CONFIG_PATH, &fixtures::render(declaration));
        }
        sandbox
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
        // Best effort: a failure to remove a temporary directory must not turn
        // a passing scenario into a failing one, or mask why a failing one
        // failed.
        let _ = std::fs::remove_dir_all(&self.root);
    }
}
