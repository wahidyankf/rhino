//! The unit driver.
//!
//! Builds the declared repository in memory and calls the library directly, so
//! the whole corpus runs at the cheapest boundary that can express it. Nothing
//! here touches the filesystem, spawns a process, or opens a socket -- the
//! in-memory tree is the port's other implementation, not a mock of it.

use crate::fixtures;
use crate::world::{CommandResult, Driver, Repository};
use rhino::runtime::MemoryTree;

#[derive(Default)]
pub struct UnitDriver;

impl Driver for UnitDriver {
    fn invoke(&self, repository: &Repository<'_>, arguments: &[String]) -> CommandResult {
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
        for path in repository.links {
            tree.mark_link(path);
        }

        let outcome = rhino::execute(&tree, arguments);
        CommandResult {
            exit_code: outcome.exit_code,
            stdout: outcome.stdout,
            stderr: outcome.stderr,
        }
    }
}
