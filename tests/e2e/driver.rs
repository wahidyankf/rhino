//! The end-to-end driver.
//!
//! Spawns the built executable and observes only what the process contract
//! exposes: an exit code, stdout, stderr, and what the repository looks like
//! afterwards. It deliberately reaches for nothing else -- no library call, no
//! internal state -- because anything more would be an integration test wearing
//! an end-to-end label.

use crate::sandbox::{self, Sandbox};
use crate::world::{self, CommandResult, Driver, Repository, Run, differences};
use std::io::Write;
use std::process::{Command, Stdio};

#[derive(Default)]
pub struct E2eDriver;

impl Driver for E2eDriver {
    fn invoke(&self, repository: &Repository<'_>, arguments: &[String]) -> Run {
        let sandbox = Sandbox::build(repository);

        let root = sandbox.root().to_string_lossy().into_owned();
        let arguments = world::with_root(arguments, &root, &format!("{root}/no-such-directory"));

        let before = sandbox::observe(sandbox.root());
        // Standard input is always a pipe, closed when nothing was supplied, so
        // a leaf that reads it when it should not blocks the test rather than
        // silently inheriting the terminal.
        let mut child = Command::new(env!("CARGO_BIN_EXE_rhino"))
            .args(&arguments)
            .current_dir(sandbox.root())
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
            .expect("the built executable is runnable");
        {
            let mut pipe = child.stdin.take().expect("the child's stdin is a pipe");
            if let Some(text) = repository.stdin {
                let _ = pipe.write_all(text.as_bytes());
            }
        }
        let output = child
            .wait_with_output()
            .expect("the child runs to completion");
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
            // Read from the sandbox rather than from the runner: these are
            // records real children wrote, at the one boundary where a child
            // is a process.
            journal: sandbox.journal(),
        }
    }
}
