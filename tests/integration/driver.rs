//! The integration driver.
//!
//! The same sentences against the concrete filesystem implementation behind the
//! port, in a real temporary directory. This is where a defect that only exists
//! on disk shows up -- path resolution, directory walking, link skipping -- none
//! of which the in-memory tree can be wrong about.

use crate::launcher::Recorder;
use crate::sandbox::{self, Sandbox};
use crate::world::{self, CommandResult, Driver, Repository, Run, differences};
use rhino::ExecutionBoundaries;
use rhino::runtime::{
    DiskAdapterStore, DiskEnvironmentStore, DiskTree, NoLauncher, NoMutationRunner,
    NoToolchainRunner,
};

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
        let root = sandbox.root().to_string_lossy().into_owned();
        let arguments = world::with_root(arguments, &root, &format!("{root}/no-such-directory"));

        let before = sandbox::observe(sandbox.root());
        let recorder = Recorder::new(repository, &root);
        let launcher = match repository.gate_outcomes.is_empty() {
            true => &NoLauncher as &dyn rhino::runtime::Launcher,
            false => &recorder as &dyn rhino::runtime::Launcher,
        };
        let outcome = rhino::execute_using_with_boundaries(
            &tree,
            &arguments,
            repository.stdin,
            ExecutionBoundaries {
                launcher,
                mutations: &NoMutationRunner,
                adapters: &DiskAdapterStore,
                environments: &DiskEnvironmentStore,
                toolchains: &NoToolchainRunner,
            },
        );
        let after = sandbox::observe(sandbox.root());

        Run {
            result: CommandResult {
                exit_code: outcome.exit_code,
                stdout: outcome.stdout,
                stderr: outcome.stderr,
            },
            mutations: differences(&before, &after),
            files_after: sandbox::text_files(sandbox.root()),
            persist_state: arguments.starts_with(&[
                "harness".to_string(),
                "adapters".to_string(),
                "generate".to_string(),
            ]),
            journal: recorder.journal(),
        }
    }
}
