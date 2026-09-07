//! The integration driver.
//!
//! The same sentences against the concrete filesystem implementation behind the
//! port, in a real temporary directory. This is where a defect that only exists
//! on disk shows up -- path resolution, directory walking, link skipping -- none
//! of which the in-memory tree can be wrong about.

use crate::sandbox::Sandbox;
use crate::world::{CommandResult, Driver, Repository};
use rhino::runtime::DiskTree;

#[derive(Default)]
pub struct IntegrationDriver;

impl Driver for IntegrationDriver {
    fn invoke(&self, repository: &Repository<'_>, arguments: &[String]) -> CommandResult {
        let sandbox = Sandbox::build(repository);
        let tree = DiskTree::new(sandbox.root()).expect("the sandbox root is a directory");

        let outcome = rhino::execute(&tree, arguments);
        CommandResult {
            exit_code: outcome.exit_code,
            stdout: outcome.stdout,
            stderr: outcome.stderr,
        }
    }
}
