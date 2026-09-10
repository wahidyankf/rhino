//! The unit driver.
//!
//! Builds the declared repository in memory and calls the library directly, so
//! the whole corpus runs at the cheapest boundary that can express it. Nothing
//! here touches the filesystem, spawns a process, or opens a socket -- the
//! in-memory tree is the port's other implementation, not a mock of it.

use crate::fixtures;
use crate::world::{self, CommandResult, Driver, Observation, Repository, Run, differences};
use rhino::runtime::{MemoryTree, Tree};

#[derive(Default)]
pub struct UnitDriver;

impl Driver for UnitDriver {
    fn invoke(&self, repository: &Repository<'_>, arguments: &[String]) -> Run {
        let mut tree = MemoryTree::default();
        for (path, content) in repository.files {
            tree.write(path, content);
        }
        if !repository.declaration.absent {
            tree.write(
                fixtures::CONFIG_PATH,
                &fixtures::render(repository.declaration),
            );
        }
        for path in repository.unreadable {
            tree.mark_unreadable(path);
        }
        for path in repository.vanished {
            tree.mark_vanished(path);
        }
        for path in repository.binary {
            tree.mark_binary(path);
        }
        for path in repository.links {
            tree.mark_link(path);
        }

        // Read back through the port on both sides rather than cloning the map
        // the fixture built: what the subject can see is the only thing it
        // could have changed, and it is what the assertion has to be about.
        // `.` is this tree seen from itself, and a name nothing starts with
        // is a root that is not there -- the same two claims the filesystem
        // adapters make with real paths.
        let arguments = world::with_root(arguments, ".", "no-such-directory");

        let before = observe(&tree);
        // `execute` where there is no standard input to supply, because that
        // is the entry point a consumer calls and a seam nothing exercised
        // would be a seam nothing keeps working.
        let outcome = match repository.stdin {
            None => rhino::execute(&tree, &arguments),
            Some(_) => rhino::execute_with(&tree, &arguments, repository.stdin),
        };
        let after = observe(&tree);

        Run {
            result: CommandResult {
                exit_code: outcome.exit_code,
                stdout: outcome.stdout,
                stderr: outcome.stderr,
            },
            mutations: differences(&before, &after),
            // No child has been dispatched by anything yet, so there is
            // nothing for a gate assertion to read. Filled once the runner
            // exists and this adapter can supply its children.
            journal: Vec::new(),
        }
    }
}

/// The tree as the port exposes it. An unreadable file is present with no
/// content, which is exactly what a validator sees.
fn observe(tree: &dyn Tree) -> Observation {
    tree.files()
        .into_iter()
        .map(|path| {
            let content = tree.read(&path).ok();
            (path, content)
        })
        .chain(world::working_directory())
        .collect()
}
