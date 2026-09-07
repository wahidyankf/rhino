//! The integration driver.
//!
//! The same sentences against the concrete filesystem implementation behind the
//! port, in a real temporary directory. This is where a defect that only exists
//! on disk shows up -- path resolution, directory walking, link skipping -- none
//! of which the in-memory tree can be wrong about.

use crate::sandbox::{self, Sandbox};
use crate::world::{CommandResult, Driver, Repository, Run, differences};
use rhino::runtime::DiskTree;

#[derive(Default)]
pub struct IntegrationDriver;

impl Driver for IntegrationDriver {
    fn invoke(&self, repository: &Repository<'_>, arguments: &[String]) -> Run {
        let sandbox = Sandbox::build(repository);
        let tree = DiskTree::new(sandbox.root()).expect("the sandbox root is a directory");

        // Walked with `std::fs` rather than through the port, so a write the
        // port cannot express is still visible. That is the whole point: the
        // claim is that the validator wrote nothing, not that it wrote nothing
        // the validator can see.
        let before = sandbox::observe(sandbox.root());
        let outcome = rhino::execute(&tree, arguments);
        let after = sandbox::observe(sandbox.root());

        Run {
            result: CommandResult {
                exit_code: outcome.exit_code,
                stdout: outcome.stdout,
                stderr: outcome.stderr,
            },
            mutations: differences(&before, &after),
        }
    }
}
