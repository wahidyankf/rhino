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
pub mod convention;
pub mod gate;
pub mod governance;
pub mod harness;
pub mod markdown;
pub mod metadata;
pub mod plan;
pub mod report;
pub mod runtime;
pub mod scan;

use config::{ConfigError, Document};
use report::Report;
use runtime::{Launcher, NoLauncher, Tree, TreeError};

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
    pub fn clean(stdout: impl Into<String>) -> Self {
        Self {
            exit_code: 0,
            stdout: stdout.into(),
            stderr: String::new(),
        }
    }

    /// A fault in the invocation or the configuration. Never `1`: exit `1` has
    /// to mean "the repository violates its declared policy" whichever command
    /// produced it, so nothing else may borrow it.
    pub fn refused(stderr: impl Into<String>) -> Self {
        Self {
            exit_code: 2,
            stdout: String::new(),
            stderr: stderr.into(),
        }
    }
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
    execute_using(tree, arguments, stdin, &NoLauncher)
}

/// As [`execute_with`], with the launcher a gate dispatch starts its children
/// through.
///
/// Passed in for the same reason the tree is: which gates ran, what each was
/// handed, and whether any two overlapped are claims the unit boundary has to
/// be able to make, and it has no process to spawn.
pub fn execute_using(
    tree: &dyn Tree,
    arguments: &[String],
    stdin: Option<&str>,
    launcher: &dyn Launcher,
) -> Outcome {
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
    let document = match load(tree) {
        Ok(document) => document,
        Err(error) => {
            return Report::refused(invocation.category, error.to_string())
                .render(invocation.format);
        }
    };

    // Which schema a repository wrote decides which commands it can be asked
    // for. Refusing rather than defaulting: a validator run against a document
    // that declares none of its sections would report a clean tree it never
    // checked.
    let config = match (invocation.category, document) {
        ("repo-config", _) => {
            let mut report = Report::new("repo-config", "configuration file");
            report.inspected(1);
            return report.render(invocation.format);
        }
        ("gate", Document::V2(document)) => {
            let surface = invocation.surface.clone().unwrap_or_default();
            return gate::dispatch(
                &document,
                &surface,
                &tree.root(),
                &invocation.forwarded,
                stdin,
                launcher,
            );
        }
        // The governance families arrived with v2. A v1 repository adopted
        // none of them, and holding it to a contract it never agreed to is
        // exactly what the optional sections of the v1 schema exist to
        // prevent -- so the refusal names the schema that carries the rule
        // rather than a section the repository could add.
        ("governance-roots", Document::V2(document)) => {
            return governance::structure::validate(tree, &document).render(invocation.format);
        }
        ("governance-companions", Document::V2(_)) => {
            return governance::companion::validate(tree).render(invocation.format);
        }
        ("governance-instructions", Document::V2(_)) => {
            return governance::instructions::validate(tree).render(invocation.format);
        }
        ("plan", Document::V2(_)) => {
            return plan::validate(tree).render(invocation.format);
        }
        ("gate", Document::V1(_)) => {
            return Report::refused(
                "gate",
                format!(
                    "{}: line 1: gates: `{}` declares no gates, and there is nothing to dispatch",
                    config::PATH,
                    config::SCHEMA
                ),
            )
            .render(invocation.format);
        }
        (
            category @ ("governance-roots"
            | "governance-companions"
            | "governance-instructions"
            | "plan"),
            Document::V1(_),
        ) => {
            return Report::refused(
                category,
                format!(
                    "{}: line 1: this rule is part of `{}`, which this repository does not declare",
                    config::PATH,
                    config::v2::SCHEMA
                ),
            )
            .render(invocation.format);
        }
        // Both schemas carry the validator sections, so both reach the same leaf
        // table below. What differs is requiredness, and that is a rule of the
        // schema rather than of the command: v1 requires five sections and says
        // so when it parses, v2 requires none and lets each command refuse for
        // the one section it needed.
        (_, Document::V2(document)) => *document.sections,
        (_, Document::V1(config)) => *config,
    };

    // Exclusions are applied only now, from the configuration of the repository
    // that is about to be walked. Reading the list earlier would mean a
    // `--root` run skipping directories the selected repository never excluded
    // -- a clean result for a tree that was never fully read, which is the one
    // failure this tool exists to prevent.
    let excluded = tree.excluding(config.excluded());
    let tree: &dyn Tree = excluded.as_ref();

    let scope = scan::Scope {
        files: invocation.files.clone(),
        directory: invocation.directory.clone(),
        harness: invocation.harness.clone(),
        stdin: stdin.map(str::to_string),
    };

    let report = match invocation.category {
        "word-budget" => match &config.word_budget {
            Some(section) => governance::word_budget::validate(tree, &config, section),
            None => undeclared("word-budget", "governance-word-budget"),
        },
        "word-count" => match &config.word_budget {
            Some(section) => governance::word_budget::inspect(tree, section, &scope),
            None => undeclared("word-count", "governance-word-budget"),
        },
        "directory-map" => match &config.directory_map {
            Some(section) => governance::directory_map::validate(tree, &config, section, &scope),
            None => undeclared("directory-map", "governance-directory-map"),
        },
        "emoji" => match &config.emoji {
            Some(section) => convention::emoji::validate(tree, &config, section),
            None => undeclared("emoji", "convention-emoji"),
        },
        "harness-parity" => match &config.harness_parity {
            Some(section) => harness::validate(tree, &config, section, &scope),
            None => undeclared("harness-parity", "harness-parity"),
        },
        "frontmatter" => match &config.frontmatter {
            Some(section) => markdown::frontmatter::validate(tree, &config, section),
            None => undeclared("frontmatter", "md-frontmatter"),
        },
        "heading-hierarchy" => match &config.heading_hierarchy {
            Some(section) => markdown::heading_hierarchy::validate(tree, &config, section),
            None => undeclared("heading-hierarchy", "md-heading-hierarchy"),
        },
        "internal-link" => markdown::internal_link::validate(tree, &config),
        "metadata" => match &config.metadata {
            Some(section) => metadata::validate(tree, &config, section),
            None => undeclared("metadata", "metadata"),
        },
        "readme-index" => match &config.readme_index {
            Some(section) => markdown::readme_index::validate(tree, &config, section),
            None => undeclared("readme-index", "md-readme-index"),
        },
        "naming" => match &config.naming {
            Some(section) => markdown::naming::validate(tree, &config, section),
            None => undeclared("naming", "md-naming"),
        },
        // The parser only produces categories the leaf table holds, so this
        // arm is the last leaf rather than a fallback for an unknown one.
        _ => match &config.mermaid {
            Some(section) => markdown::mermaid::validate(tree, &config, section, &scope),
            None => undeclared("mermaid", "md-mermaid"),
        },
    };

    report.render(invocation.format)
}

/// A command whose section the repository never declared.
///
/// Refused rather than skipped, and refused rather than defaulted. Every section
/// added after `v0.1` is optional so that a release costs an existing consumer
/// nothing, and the whole value of that optionality depends on absence meaning
/// *nothing to enforce* instead of *enforce whatever the binary happens to
/// think*.
fn undeclared(category: &'static str, section: &str) -> Report {
    Report::refused(
        category,
        format!(
            "{}: line 1: {section}: the section is not declared, and RHINO holds no default for it",
            config::PATH
        ),
    )
}

/// This build's release identity.
///
/// The commit is embedded at compile time by `build.rs`; a build made outside a
/// repository reports forty zeros rather than lying about which revision it is.
///
/// The version is reported the way the release tag spells it, `v` and all.
/// Cargo's manifest cannot hold that prefix, so it is added here rather than
/// left for every consumer to add back: a lock file holds a tag, and an
/// identity a consumer has to reformat before comparing is one more place for
/// the comparison to be written differently in two repositories.
fn version(format: cli::Format) -> Outcome {
    let version = concat!("v", env!("CARGO_PKG_VERSION"));
    let commit = env!("RHINO_COMMIT");
    match format {
        cli::Format::Text => Outcome::clean(format!("{version}\n")),
        cli::Format::Json => Outcome::clean(format!(
            "{{\"schemaVersion\":1,\"version\":\"{version}\",\"commit\":\"{commit}\"}}\n"
        )),
    }
}

fn load(tree: &dyn Tree) -> Result<Document, ConfigError> {
    let text = tree.read(config::PATH).map_err(|error| match error {
        TreeError::NotFound => ConfigError::Missing,
        TreeError::Unreadable(reason) => ConfigError::Unreadable(reason),
        TreeError::NotText => ConfigError::Unreadable("holds no text".to_string()),
    })?;
    config::parse(&text)
}
