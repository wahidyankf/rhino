//! The one boundary policy that only the process boundary can prove.
//!
//! `tests/integration/policy.rs` holds the rest. This one is here because at
//! unit and integration `rhino::execute` runs *inside the test process*, so a
//! write to a relative path lands in the crate root rather than in the
//! repository under inspection -- and once any run has created it, the file is
//! present on both sides of the next observation and the difference vanishes.
//! Only a spawned process whose working directory *is* the inspected
//! repository puts every write it could make inside the observation.

use crate::sandbox::{self, Sandbox};
use crate::world::{World, differences};
use std::process::{Command, Stdio};

/// Every command path the tool answers, as a caller would type it.
const LEAVES: &[&[&str]] = &[
    &["governance", "word-budget", "validate"],
    &["governance", "directory-map", "validate"],
    &["harness", "parity", "validate"],
    &["md", "internal-link", "validate"],
    &["md", "mermaid", "validate"],
    &["md", "word-count", "inspect", "--file", "rules/README.md"],
    &["repo-config", "validate"],
    &["version"],
];

#[test]
fn no_leaf_writes_to_the_repository_it_inspects() {
    let mut world: World<()> = World::default();
    world.files.insert(
        "rules/README.md".to_string(),
        "# Rules\n\n## Directory Map\n\nNo other entries.\n".to_string(),
    );
    let repository = world.repository();

    for leaf in LEAVES {
        // A fresh sandbox per leaf, so a file one leaf left behind cannot be
        // mistaken for part of the next leaf's declared repository.
        let sandbox = Sandbox::build(&repository);
        let before = sandbox::observe(sandbox.root());
        let output = Command::new(env!("CARGO_BIN_EXE_rhino"))
            .args(*leaf)
            .current_dir(sandbox.root())
            .stdin(Stdio::null())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .output()
            .expect("the built executable is runnable");
        let after = sandbox::observe(sandbox.root());

        // `<cwd>/` entries describe the *test* process's working directory,
        // which is the crate root and not the child's. Another test binary
        // writing there concurrently is noise about someone else's run.
        let changed: Vec<String> = differences(&before, &after)
            .into_iter()
            .filter(|change| !change.starts_with("<cwd>/"))
            .collect();
        assert!(
            changed.is_empty(),
            "`rhino {}` changed the repository it inspected: {changed:?}\nstderr: {}",
            leaf.join(" "),
            String::from_utf8_lossy(&output.stderr)
        );
    }
}
