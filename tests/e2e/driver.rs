//! The end-to-end driver.
//!
//! Spawns the built executable and observes only what the process contract
//! exposes: an exit code, stdout, stderr, and what the repository looks like
//! afterwards. It deliberately reaches for nothing else -- no library call, no
//! internal state -- because anything more would be an integration test wearing
//! an end-to-end label.

use crate::sandbox::{self, Sandbox};
use crate::world::{CommandResult, Driver, Repository, Run, differences};
use std::process::Command;

#[derive(Default)]
pub struct E2eDriver;

impl Driver for E2eDriver {
    fn invoke(&self, repository: &Repository<'_>, arguments: &[String]) -> Run {
        let sandbox = Sandbox::build(repository);

        let before = sandbox::observe(sandbox.root());
        let output = Command::new(env!("CARGO_BIN_EXE_rhino"))
            .args(arguments)
            .current_dir(sandbox.root())
            .output()
            .expect("the built executable is runnable");
        let after = sandbox::observe(sandbox.root());

        Run {
            result: CommandResult {
                // A signal leaves no code. Reporting it as 2 rather than
                // panicking keeps the scenario's own assertion the thing that
                // fails.
                exit_code: u8::try_from(output.status.code().unwrap_or(2)).unwrap_or(2),
                stdout: String::from_utf8_lossy(&output.stdout).into_owned(),
                stderr: String::from_utf8_lossy(&output.stderr).into_owned(),
            },
            mutations: differences(&before, &after),
        }
    }
}
