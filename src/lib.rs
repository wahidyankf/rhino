//! RHINO — Repository Hygiene & INtegration Orchestrator.
//!
//! A generic repository-hygiene validator. Every value it enforces arrives from
//! the consuming repository's `repo-config.yml`; this crate ships no default for
//! any of them, and no repository, tree, harness, or limit is named in `src/`.
//!
//! The product is read-only, network-free, and process-free. Those are not
//! conventions to be remembered: they are held by tests under `tests/`.
#![forbid(unsafe_code)]

pub mod cli;
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
            // Standard input is read only when a leaf was asked for it, so an
            // ordinary run never blocks on a terminal.
            let stdin = wants_stdin(&arguments).then(read_stdin);
            execute_with(&tree, &arguments, stdin.as_deref())
        }
        Err(reason) => Outcome::refused(format!("rhino: {reason}\n")),
    };

    if !outcome.stdout.is_empty() {
        print!("{}", outcome.stdout);
    }
    if !outcome.stderr.is_empty() {
        eprint!("{}", outcome.stderr);
    }
    outcome.exit_code
}

fn wants_stdin(arguments: &[String]) -> bool {
    arguments
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

/// Runs one invocation against a tree.
///
/// The seam the whole corpus drives: a repository is whatever implements
/// [`Tree`], so the same sentence can be asserted against an in-memory tree, a
/// temporary directory, and the built executable.
pub fn execute(tree: &dyn Tree, arguments: &[String]) -> Outcome {
    execute_with(tree, arguments, None)
}

/// As [`execute`], with whatever a `--file -` selection should read.
///
/// Standard input is passed in rather than read here, because a library that
/// reached for the process's own stdin could not be driven twice in one test
/// process -- and because the E2E adapter has to be the one boundary where the
/// real stream is involved.
pub fn execute_with(tree: &dyn Tree, arguments: &[String], stdin: Option<&str>) -> Outcome {
    let invocation = match cli::parse(arguments) {
        Ok(cli::Parsed::Help(cli::Help(text))) => return Outcome::clean(text),
        Ok(cli::Parsed::Run(invocation)) => *invocation,
        Err(refusal) => return Outcome::refused(format!("{refusal}\n")),
    };

    // `--root` is answered before anything is read, so a root that names
    // nothing is an invalid invocation rather than an empty repository
    // reported clean.
    let rerooted;
    let tree: &dyn Tree = match &invocation.root {
        None => tree,
        Some(path) => match tree.rooted_at(path) {
            Ok(rooted) => {
                rerooted = rooted;
                rerooted.as_ref()
            }
            Err(reason) => return Outcome::refused(format!("rhino: {reason}\n")),
        },
    };

    if invocation.category == "version" {
        return version(invocation.format);
    }

    // Every command reads the configuration before doing anything else, so a
    // configuration it cannot use is refused rather than half-run. A validator
    // that skipped a tree because its configuration was malformed would report
    // a clean repository that was never checked.
    let config = match load(tree) {
        Ok(config) => config,
        Err(error) => {
            return Report::refused(invocation.category, error.to_string())
                .render(invocation.format);
        }
    };

    // Exclusions are applied only now, from the configuration of the repository
    // that is about to be walked. Reading the list earlier would mean a
    // `--root` run skipping directories the selected repository never excluded
    // -- a clean result for a tree that was never fully read, which is the one
    // failure this tool exists to prevent.
    let excluded = tree.excluding(&config.scan.exclude_directories);
    let tree: &dyn Tree = excluded.as_ref();

    let scope = scan::Scope {
        files: invocation.files.clone(),
        directory: invocation.directory.clone(),
        harness: invocation.harness.clone(),
        stdin: stdin.map(str::to_string),
    };

    let report = match invocation.category {
        "repo-config" => {
            let mut report = Report::new("repo-config", "configuration file");
            report.inspected(1);
            report
        }
        "word-budget" => governance::word_budget::validate(tree, &config),
        "word-count" => governance::word_budget::inspect(tree, &scope),
        "directory-map" => governance::directory_map::validate(tree, &config, &scope),
        "harness-parity" => harness::validate(tree, &config, &scope),
        "internal-link" => markdown::internal_link::validate(tree, &config),
        // The parser only produces categories the leaf table holds, so this
        // arm is the last leaf rather than a fallback for an unknown one.
        _ => markdown::mermaid::validate(tree, &config, &scope),
    };

    report.render(invocation.format)
}

/// This build's release identity.
///
/// The commit is embedded at compile time by `build.rs`; a build made outside a
/// repository reports forty zeros rather than lying about which revision it is.
fn version(format: cli::Format) -> Outcome {
    let version = env!("CARGO_PKG_VERSION");
    let commit = env!("RHINO_COMMIT");
    match format {
        cli::Format::Text => Outcome::clean(format!("{version}\n")),
        cli::Format::Json => Outcome::clean(format!(
            "{{\"schemaVersion\":1,\"version\":\"{version}\",\"commit\":\"{commit}\"}}\n"
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
