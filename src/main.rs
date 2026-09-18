//! The process boundary, and the only place it is crossed.
//!
//! Argument collection, standard input, the working directory, the two output
//! streams, and the exit code. Every decision lives in the library crate so the
//! behaviour corpus can reach it; what is left here is what a unit test may not
//! do, which is why this file is a declared coverage exclusion.
#![forbid(unsafe_code)]

use rhino::runtime::{
    DiskAdapterStore, DiskEnvironmentStore, DiskMutationRunner, DiskToolchainRunner, DiskTree,
    ProcessLauncher,
};
use rhino::{ExecutionBoundaries, Outcome};
use std::process::ExitCode;

fn main() -> ExitCode {
    let mut args = std::env::args_os();
    let _program = args.next();
    let arguments: Vec<String> = args
        .map(|value| value.to_string_lossy().into_owned())
        .collect();

    let outcome = match DiskTree::at_current_directory() {
        Ok(tree) => {
            // Standard input is read only when a leaf was asked for it, so an
            // ordinary run never blocks on a terminal.
            let stdin = wants_stdin(&arguments).then(read_stdin);
            rhino::execute_using_with_boundaries(
                &tree,
                &arguments,
                stdin.as_deref(),
                ExecutionBoundaries {
                    launcher: &ProcessLauncher,
                    mutations: &DiskMutationRunner,
                    adapters: &DiskAdapterStore,
                    environments: &DiskEnvironmentStore,
                    toolchains: &DiskToolchainRunner,
                },
            )
        }
        Err(reason) => Outcome::refused(format!("rhino: {reason}\n")),
    };

    if !outcome.stdout.is_empty() {
        print!("{}", outcome.stdout);
    }
    if !outcome.stderr.is_empty() {
        eprint!("{}", outcome.stderr);
    }
    ExitCode::from(outcome.exit_code)
}

/// Whether this invocation reads the real standard input.
///
/// A `--file -` selection asks for it. Of the closed lifecycle surfaces, only
/// `pre-push` receives Git update records on standard input; reading from a
/// terminal for every other gate would block a local or scheduled run before a
/// declared child can start.
fn wants_stdin(arguments: &[String]) -> bool {
    let is_pre_push = arguments.starts_with(&["gate".to_string(), "run".to_string()])
        && arguments
            .windows(2)
            .any(|pair| pair == ["--surface", "pre-push"]);
    is_pre_push
        || arguments
            .windows(2)
            .any(|pair| pair[0] == "--file" && pair[1] == "-")
}

fn read_stdin() -> String {
    use std::io::Read;
    let mut text = String::new();
    // A failed read leaves the string empty, which the leaf then reports as an
    // empty document rather than as a crash.
    let _ = std::io::stdin().read_to_string(&mut text);
    text
}
