//! RHINO — Repository Hygiene & INtegration Orchestrator.
//!
//! A generic repository-hygiene validator. Every value it enforces arrives from
//! the consuming repository's `repo-config.yml`; this crate ships no default for
//! any of them, and no repository, tree, harness, or limit is named in `src/`.
//!
//! The product is read-only, network-free, and process-free. Those are not
//! conventions to be remembered: they are held by tests under `tests/`.
#![forbid(unsafe_code)]

use std::ffi::OsString;

/// Runs RHINO and returns the process exit code.
///
/// The exit codes are part of the published contract and never widen: `0` for a
/// clean run or for help, `1` when a validator reports findings, and `2` for an
/// invalid invocation, an invalid root, or unreadable or invalid configuration.
/// No command is registered yet, so every invocation is an invalid invocation.
pub fn run<I>(args: I) -> u8
where
    I: IntoIterator<Item = OsString>,
{
    let mut args = args.into_iter();
    let program = args.next();
    let name = program
        .as_deref()
        .and_then(|value| value.to_str())
        .unwrap_or("rhino");
    eprintln!("{name}: no command recognised");
    2
}
