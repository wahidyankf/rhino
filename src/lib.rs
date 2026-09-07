//! RHINO — Repository Hygiene & INtegration Orchestrator.
//!
//! A generic repository-hygiene validator. Every value it enforces arrives from
//! the consuming repository's `repo-config.yml`; this crate ships no default for
//! any of them, and no repository, tree, harness, or limit is named in `src/`.
//!
//! The product is read-only, network-free, and process-free. Those are not
//! conventions to be remembered: they are held by tests under `tests/`.
#![forbid(unsafe_code)]

pub mod config;
pub mod runtime;

use config::ConfigError;
use runtime::{Tree, TreeError};
use std::ffi::OsString;

/// Everything an invocation produces, and all the process contract exposes.
///
/// Returned rather than printed so the library is callable from a test without
/// a process, which is what lets the behaviour corpus run at the unit boundary.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct Outcome {
    /// `0` clean or help, `1` findings, `2` invalid invocation or configuration.
    pub exit_code: u8,
    pub stdout: String,
    pub stderr: String,
}

impl Outcome {
    fn clean(stdout: impl Into<String>) -> Self {
        Self {
            exit_code: 0,
            stdout: stdout.into(),
            stderr: String::new(),
        }
    }

    /// A fault in the invocation or the configuration. Never `1`: exit `1` has
    /// to mean "the repository violates its declared policy" whichever command
    /// produced it, so nothing else may borrow it.
    fn refused(stderr: impl Into<String>) -> Self {
        Self {
            exit_code: 2,
            stdout: String::new(),
            stderr: stderr.into(),
        }
    }
}

/// Runs RHINO against a real repository and returns the process exit code.
pub fn run<I>(args: I) -> u8
where
    I: IntoIterator<Item = OsString>,
{
    let mut args = args.into_iter();
    let _program = args.next();
    let arguments: Vec<String> = args
        .map(|value| value.to_string_lossy().into_owned())
        .collect();

    let tree = runtime::DiskTree::at_current_directory();
    let outcome = match tree {
        Ok(tree) => execute(&tree, &arguments),
        Err(reason) => Outcome::refused(format!("rhino: {reason}")),
    };

    if !outcome.stdout.is_empty() {
        print!("{}", outcome.stdout);
    }
    if !outcome.stderr.is_empty() {
        eprint!("{}", outcome.stderr);
    }
    outcome.exit_code
}

/// Runs one invocation against a tree.
///
/// The seam the whole corpus drives: a repository is whatever implements
/// [`Tree`], so the same sentence can be asserted against an in-memory tree, a
/// temporary directory, and the built executable.
pub fn execute(tree: &dyn Tree, arguments: &[String]) -> Outcome {
    let path: Vec<&str> = arguments.iter().map(String::as_str).collect();

    match path.as_slice() {
        ["version"] => Outcome::clean(format!("{}\n", env!("CARGO_PKG_VERSION"))),
        ["repo-config", "validate"] => match load(tree) {
            Ok(_) => Outcome::clean(String::new()),
            Err(error) => Outcome::refused(format!("{error}\n")),
        },
        // Every remaining command still reads the configuration before doing
        // anything, so an unreadable one is refused rather than half-run.
        ["governance", "word-budget", "validate"]
        | ["governance", "directory-map", "validate"]
        | ["harness", "parity", "validate"]
        | ["md", "internal-link", "validate"]
        | ["md", "mermaid", "validate"]
        | ["md", "word-count", "inspect"] => match load(tree) {
            Ok(_) => Outcome::refused(format!("rhino: `{}` is not ported yet\n", path.join(" "))),
            Err(error) => Outcome::refused(format!("{error}\n")),
        },
        [] => Outcome::refused("rhino: no command given\n".to_string()),
        other => Outcome::refused(format!(
            "rhino: unrecognized command `{}`\n",
            other.join(" ")
        )),
    }
}

fn load(tree: &dyn Tree) -> Result<config::Config, ConfigError> {
    let text = tree.read(config::PATH).map_err(|error| match error {
        TreeError::NotFound => ConfigError::Missing,
        TreeError::Unreadable(reason) => ConfigError::Unreadable(reason),
    })?;
    config::parse(&text)
}
