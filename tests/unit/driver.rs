//! The unit driver.
//!
//! Builds the declared repository in memory and calls the library directly, so
//! the whole corpus runs at the cheapest boundary that can express it. Nothing
//! here touches the filesystem, spawns a process, or opens a socket -- the
//! in-memory tree is the port's other implementation, not a mock of it.

use crate::fixtures;
use crate::world::{CommandResult, Declaration, Driver};
use rhino::runtime::MemoryTree;
use std::collections::BTreeMap;

#[derive(Default)]
pub struct UnitDriver;

impl Driver for UnitDriver {
    fn invoke(
        &self,
        declaration: &Declaration,
        files: &BTreeMap<String, String>,
        arguments: &[String],
    ) -> CommandResult {
        let mut tree = MemoryTree::default();
        for (path, content) in files {
            tree.write(path, content);
        }
        if !declaration.absent {
            tree.write(fixtures::CONFIG_PATH, &fixtures::render(declaration));
        }

        let outcome = rhino::execute(&tree, arguments);
        CommandResult {
            exit_code: outcome.exit_code,
            stdout: outcome.stdout,
            stderr: outcome.stderr,
        }
    }
}
