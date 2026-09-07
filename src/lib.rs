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
pub mod governance;
pub mod harness;
pub mod markdown;
pub mod report;
pub mod runtime;
pub mod scan;

use config::ConfigError;
use report::Report;
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

    let outcome = match runtime::DiskTree::at_current_directory() {
        Ok(mut tree) => {
            // Read the exclusion list before walking anything, so a real
            // repository's dependency and build directories are never descended
            // into rather than descended into and then filtered out. A broken
            // configuration excludes nothing and `execute` reports it.
            if let Ok(config) = load(&tree) {
                tree.exclude(&config.scan.exclude_directories);
            }
            execute(&tree, &arguments)
        }
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

    if path.as_slice() == ["version"] {
        return Outcome::clean(format!("{}\n", env!("CARGO_PKG_VERSION")));
    }

    let Some(category) = category_of(&path) else {
        return match path.as_slice() {
            [] => Outcome::refused("rhino: no command given\n".to_string()),
            other => Outcome::refused(format!(
                "rhino: unrecognized command `{}`\n",
                other.join(" ")
            )),
        };
    };

    // Every command reads the configuration before doing anything else, so a
    // configuration it cannot use is refused rather than half-run. A validator
    // that skipped a tree because its configuration was malformed would report
    // a clean repository that was never checked.
    let config = match load(tree) {
        Ok(config) => config,
        Err(error) => return Report::refused(category, error.to_string()),
    };

    match category {
        "repo-config" => Report::new("repo-config", "configuration file")
            .inspected(1)
            .finish(),
        "word-budget" => governance::word_budget::validate(tree, &config),
        "directory-map" => governance::directory_map::validate(tree, &config),
        "harness-parity" => harness::validate(tree, &config),
        "internal-link" => markdown::internal_link::validate(tree, &config),
        "mermaid" => markdown::mermaid::validate(tree, &config),
        other => Report::refused(other, format!("`{}` is not ported yet", path.join(" "))),
    }
}

/// The atomic output prefix each command path reports under.
///
/// The mapping lives here rather than being assembled from the path, so the
/// prefix a consumer greps for cannot change because a command was renamed or
/// nested differently.
fn category_of(path: &[&str]) -> Option<&'static str> {
    match path {
        ["repo-config", "validate"] => Some("repo-config"),
        ["governance", "word-budget", "validate"] => Some("word-budget"),
        ["governance", "directory-map", "validate"] => Some("directory-map"),
        ["harness", "parity", "validate"] => Some("harness-parity"),
        ["md", "internal-link", "validate"] => Some("internal-link"),
        ["md", "mermaid", "validate"] => Some("mermaid"),
        ["md", "word-count", "inspect"] => Some("word-count"),
        _ => None,
    }
}

fn load(tree: &dyn Tree) -> Result<config::Config, ConfigError> {
    let text = tree.read(config::PATH).map_err(|error| match error {
        TreeError::NotFound => ConfigError::Missing,
        TreeError::Unreadable(reason) => ConfigError::Unreadable(reason),
    })?;
    config::parse(&text)
}
