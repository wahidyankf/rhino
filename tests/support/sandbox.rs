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

/// Where a declared gate child is written, and where the children record.
const GATES: &str = "gates";
const JOURNAL: &str = ".gate-journal";

/// The marker every recorder writes into its own streams, so a runner that
/// repeated what a child wrote is caught wherever it surfaced rather than by
/// the wording of a summary.
const CHILD_MARKER: &str = crate::binding::CHILD_MARKER;

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
        for path in repository.binary {
            sandbox.fill_with_bytes(path);
        }
        for path in repository.links {
            sandbox.link(path);
        }
        // Created after the files, so a directory a scenario declared empty is
        // empty however the paths happened to sort.
        for path in repository.empty_directories {
            std::fs::create_dir_all(sandbox.root.join(path))
                .expect("the empty directory is creatable");
        }
        for (id, code) in repository.gate_outcomes {
            sandbox.recorder(id, *code);
        }
        // Permissions last: a file has to be written before it can be closed.
        for path in repository.unreadable {
            sandbox.seal(path);
        }
        sandbox
    }

    /// Write the child a scenario declared: a real executable that records what
    /// it was handed and exits with the stated code.
    ///
    /// Real rather than simulated, because the claims this stands under are all
    /// claims about a process boundary. That a vector arrives unsplit, that the
    /// environment carries exactly one variable, that a hook's standard input
    /// reaches every child -- none of them can fail against something standing
    /// in for a process, so none of them would be proved by one.
    fn recorder(&self, id: &str, code: i32) {
        use std::os::unix::fs::PermissionsExt;
        let path = format!("{GATES}/{id}.sh");
        self.write(
            &path,
            &format!(
                r#"#!/bin/sh
journal="$PWD/{JOURNAL}"
arguments=""
for argument in "$@"; do
  if [ -z "$arguments" ]; then arguments="$argument"; else arguments="$arguments|$argument"; fi
done
input=$(cat)
printf 'start\t{id}\t%s\t%s\t%s\t%s\n' "$arguments" "$OSE_GATE_SURFACE" "$input" "$PWD" >> "$journal"
printf '{marker} {id}\n'
printf '{marker} {id}\n' >&2
printf 'stop\t{id}\n' >> "$journal"
exit {code}
"#,
                marker = CHILD_MARKER,
            ),
        );
        let target = self.root.join(&path);
        std::fs::set_permissions(&target, std::fs::Permissions::from_mode(0o755))
            .expect("the recorder is made executable");
    }

    /// What the children recorded, with the sandbox path replaced by the
    /// placeholder a scenario can name.
    pub fn journal(&self) -> Vec<String> {
        // Both spellings of the root are replaced. A temporary directory is
        // reached through a symbolic link on macOS, so the path a child reports
        // as its own working directory is the resolved one and the path this
        // fixture built is not.
        let mut spellings = vec![self.root.to_string_lossy().into_owned()];
        if let Ok(resolved) = std::fs::canonicalize(&self.root) {
            spellings.push(resolved.to_string_lossy().into_owned());
        }
        spellings.sort_by_key(|spelling| std::cmp::Reverse(spelling.len()));
        std::fs::read_to_string(self.root.join(JOURNAL))
            .unwrap_or_default()
            .lines()
            .map(|line| {
                let mut line = line.to_string();
                for spelling in &spellings {
                    line = line.replace(spelling, crate::world::ROOT);
                }
                line
            })
            .collect()
    }

    /// Replace a written file's content with bytes no decoder will accept, so
    /// the file opens and yields no text -- an image or an archive, as far as a
    /// walk that reads everything is concerned.
    fn fill_with_bytes(&self, path: &str) {
        let target = self.root.join(path);
        if let Some(parent) = target.parent() {
            std::fs::create_dir_all(parent).expect("the fixture's parent directory is creatable");
        }
        std::fs::write(&target, [0xff, 0xfe, 0x00, 0x80]).expect("the fixture file is writable");
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
