//! RHINO — Repository Hygiene & INtegration Orchestrator.
//!
//! A generic repository-hygiene validator. Every value it enforces arrives from
//! the consuming repository's `repo-config.yml`; this crate ships no default for
//! any of them, and no repository, tree, harness, or limit is named in `src/`.
//!
//! Validators are read-only, network-free, and process-free. Declared gate
//! dispatch and adapter generation receive separate explicit boundaries; tests
//! hold both the validator invariants and the operation scopes.
#![forbid(unsafe_code)]

pub mod cli;
pub mod config;
pub mod convention;
pub mod errors;
pub mod governance;
pub mod markdown;
pub mod metadata;
pub mod report;
pub mod runtime;
pub mod scan;
mod v0_4;

use config::{ConfigError, Document};
use report::Report;
use runtime::{
    AdapterStore, EnvironmentStore, Launcher, MutationRunner, NoAdapterStore, NoEnvironmentStore,
    NoLauncher, NoMutationRunner, NoToolchainRunner, ToolchainRunner, Tree, TreeError,
};

/// Everything an invocation produces, and all the process contract exposes.
///
/// Returned rather than printed so the library is callable from a test without
/// a process, which is what lets the behaviour corpus run at the unit boundary.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct Outcome {
    /// The closed vocabulary, and nothing outside it: `0` clean or help, `1`
    /// findings, `2` an unusable invocation, configuration, or run, `126` a gate
    /// child that could not be executed, `127` one that was not found.
    ///
    /// A status from a signal never appears here. The process boundary reports
    /// `128+N` by ending the way the signal says to, because a signal death
    /// translated into a value is the one substitution a caller cannot detect.
    pub exit_code: u8,
    pub stdout: String,
    pub stderr: String,
}

/// The explicit capabilities an invocation may receive. Grouping them keeps the
/// public execution seam legible without merging their authority: callers must
/// still name a concrete implementation for every separate boundary.
pub struct ExecutionBoundaries<'a> {
    pub launcher: &'a dyn Launcher,
    pub mutations: &'a dyn MutationRunner,
    pub adapters: &'a dyn AdapterStore,
    pub environments: &'a dyn EnvironmentStore,
    pub toolchains: &'a dyn ToolchainRunner,
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
    ///
    /// The message arrives already rendered, which is why this stays the
    /// low-level constructor. Every caller that knows which format was asked
    /// for should use [`Outcome::refusal`] instead, so the reason reaches a
    /// program as a code rather than as prose it would have to parse.
    pub fn refused(stderr: impl Into<String>) -> Self {
        Self {
            exit_code: 2,
            stdout: String::new(),
            stderr: stderr.into(),
        }
    }

    /// A refusal carrying a code from the closed vocabulary, rendered for the
    /// format the caller asked for.
    pub fn refusal(format: cli::Format, code: errors::ErrorCode, message: impl AsRef<str>) -> Self {
        Self::refused(errors::body(format, code, message.as_ref()))
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
    execute_using_with_mutations(tree, arguments, stdin, launcher, &NoMutationRunner)
}

/// As [`execute_using`], with the one explicit mutation boundary available to
/// a process entry point that is authorized to update an index or create a
/// disposable pull-request replay.
pub fn execute_using_with_mutations(
    tree: &dyn Tree,
    arguments: &[String],
    stdin: Option<&str>,
    launcher: &dyn Launcher,
    mutations: &dyn MutationRunner,
) -> Outcome {
    execute_using_with_boundaries(
        tree,
        arguments,
        stdin,
        ExecutionBoundaries {
            launcher,
            mutations,
            adapters: &NoAdapterStore,
            environments: &NoEnvironmentStore,
            toolchains: &NoToolchainRunner,
        },
    )
}

/// As [`execute_using_with_mutations`], with the separately-scoped adapter
/// transaction port. Gate mutations and generated adapter replacement have
/// different ownership and must never share an implicit write capability.
pub fn execute_using_with_boundaries(
    tree: &dyn Tree,
    arguments: &[String],
    stdin: Option<&str>,
    boundaries: ExecutionBoundaries<'_>,
) -> Outcome {
    let ExecutionBoundaries {
        launcher,
        mutations,
        adapters,
        environments,
        toolchains,
    } = boundaries;
    let invocation = match cli::parse(arguments) {
        Ok(cli::Parsed::Help(cli::Help(text))) => return Outcome::clean(text),
        Ok(cli::Parsed::Run(invocation)) => *invocation,
        Err(refusal) => {
            return Outcome::refusal(refusal.format, refusal.code, &refusal.message);
        }
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
            Err(reason) => {
                return Outcome::refusal(
                    invocation.format,
                    errors::ErrorCode::RepositoryUnusable,
                    &reason,
                );
            }
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
    match (invocation.category, document) {
        ("repo-config", _) => {
            let mut report = Report::new("repo-config", "configuration file");
            report.inspected(1);
            report.render(invocation.format)
        }
        ("gate-list", Document::V0_4(document)) => {
            v0_4::gates::list(document.gates.as_ref(), invocation.format)
        }
        ("gate-validate", Document::V0_4(document)) => {
            v0_4::gates::validate(document.gates.as_ref(), invocation.format)
        }
        ("gate", Document::V0_4(document)) => v0_4::gates::run(
            document.gates.as_ref(),
            &invocation,
            tree,
            stdin,
            launcher,
            mutations,
        ),
        ("harness-adapters-validate", Document::V0_4(document)) => v0_4::harnesses::validate(
            document.harness.as_ref(),
            document.scan.as_ref(),
            tree,
            invocation.format,
        ),
        ("harness-adapters-generate", Document::V0_4(document)) => v0_4::harnesses::generate(
            document.harness.as_ref(),
            document.scan.as_ref(),
            tree,
            adapters,
            invocation.format,
        ),
        ("environment-backup", Document::V0_4(document)) => v0_4::operations::backup(
            document.environment.as_ref(),
            tree,
            &tree.root(),
            invocation.backup_directory.as_deref(),
            environments,
            invocation.format,
        ),
        ("environment-validate", Document::V0_4(document)) => {
            v0_4::operations::validate(document.environment.as_ref(), tree, invocation.format)
        }
        ("environment-init", Document::V0_4(document)) => v0_4::operations::init(
            document.environment.as_ref(),
            tree,
            &tree.root(),
            invocation.apply,
            environments,
            invocation.format,
        ),
        ("environment-restore", Document::V0_4(document)) => v0_4::operations::restore(
            document.environment.as_ref(),
            tree,
            &tree.root(),
            invocation.backup_directory.as_deref(),
            invocation.force,
            environments,
            invocation.format,
        ),
        ("toolchain-validate", Document::V0_4(document)) => v0_4::operations::validate_toolchains(
            document.toolchains.as_ref(),
            toolchains,
            invocation.format,
        ),
        ("toolchain-provision", Document::V0_4(document)) => v0_4::operations::provision(
            document.toolchains.as_ref(),
            invocation.apply,
            toolchains,
            invocation.format,
        ),
        ("license", Document::V0_4(document)) => v0_4::validators::license(
            document
                .policies
                .as_ref()
                .and_then(|policies| policies.conventions.as_ref()),
            tree,
        )
        .render(invocation.format),
        ("emoji", Document::V0_4(document)) => v0_4::validators::emoji(
            document
                .policies
                .as_ref()
                .and_then(|policies| policies.conventions.as_ref()),
            document.scan.as_ref(),
            tree,
        )
        .render(invocation.format),
        ("frontmatter", Document::V0_4(document)) => v0_4::validators::frontmatter(
            document
                .policies
                .as_ref()
                .and_then(|policies| policies.markdown.as_ref()),
            document.scan.as_ref(),
            tree,
        )
        .render(invocation.format),
        ("heading-hierarchy", Document::V0_4(document)) => v0_4::validators::heading_hierarchy(
            document
                .policies
                .as_ref()
                .and_then(|policies| policies.markdown.as_ref()),
            document.scan.as_ref(),
            tree,
        )
        .render(invocation.format),
        ("internal-link", Document::V0_4(document)) => v0_4::validators::internal_link(
            document
                .policies
                .as_ref()
                .and_then(|policies| policies.markdown.as_ref()),
            document.scan.as_ref(),
            tree,
        )
        .render(invocation.format),
        ("metadata", Document::V0_4(document)) => v0_4::validators::metadata(
            document
                .policies
                .as_ref()
                .and_then(|policies| policies.markdown.as_ref()),
            document.scan.as_ref(),
            tree,
        )
        .render(invocation.format),
        ("mermaid", Document::V0_4(document)) => {
            let scope = scan::Scope {
                files: invocation.files.clone(),
                directory: invocation.directory.clone(),
                stdin: stdin.map(str::to_string),
            };
            v0_4::validators::mermaid(
                document
                    .policies
                    .as_ref()
                    .and_then(|policies| policies.markdown.as_ref()),
                document.scan.as_ref(),
                tree,
                &scope,
            )
            .render(invocation.format)
        }
        ("readme-index", Document::V0_4(document)) => v0_4::validators::readme_index(
            document
                .policies
                .as_ref()
                .and_then(|policies| policies.markdown.as_ref()),
            tree,
        )
        .render(invocation.format),
        ("naming", Document::V0_4(document)) => v0_4::validators::naming(
            document
                .policies
                .as_ref()
                .and_then(|policies| policies.markdown.as_ref()),
            document.scan.as_ref(),
            tree,
        )
        .render(invocation.format),
        ("word-budget", Document::V0_4(document)) => v0_4::validators::word_budget(
            document
                .policies
                .as_ref()
                .and_then(|policies| policies.governance.as_ref()),
            document.scan.as_ref(),
            tree,
        )
        .render(invocation.format),
        ("directory-map", Document::V0_4(document)) => {
            let scope = scan::Scope {
                files: invocation.files.clone(),
                directory: invocation.directory.clone(),
                stdin: stdin.map(str::to_string),
            };
            v0_4::validators::directory_map(
                document
                    .policies
                    .as_ref()
                    .and_then(|policies| policies.governance.as_ref()),
                document.scan.as_ref(),
                tree,
                &scope,
            )
            .render(invocation.format)
        }
        ("vendor", Document::V0_4(document)) => v0_4::validators::vendor(
            document
                .policies
                .as_ref()
                .and_then(|policies| policies.governance.as_ref()),
            tree,
        )
        .render(invocation.format),
        ("layers", Document::V0_4(document)) => v0_4::validators::layers(
            document
                .policies
                .as_ref()
                .and_then(|policies| policies.governance.as_ref()),
            tree,
        )
        .render(invocation.format),
        ("traceability", Document::V0_4(document)) => v0_4::validators::traceability(
            document
                .policies
                .as_ref()
                .and_then(|policies| policies.governance.as_ref()),
            tree,
        )
        .render(invocation.format),
        // A grouped document refuses a command whose policy has no stable owner.
        (category, Document::V0_4(_)) => Report::refused(
            category,
            "the grouped v0.4 configuration declares no command in this stable release",
        )
        .render(invocation.format),
    }
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

#[cfg(test)]
mod tests {
    use super::*;
    use crate::runtime::{MemoryTree, NoLauncher, NoMutationRunner};

    fn arguments(words: &[&str]) -> Vec<String> {
        words.iter().map(|word| (*word).to_string()).collect()
    }

    fn grouped_tree() -> MemoryTree {
        let tree = MemoryTree::default();
        tree.write(config::PATH, "schema: rhino/repo-config/v2\n");
        tree
    }

    #[test]
    fn public_execution_seams_preserve_the_same_version_result() {
        let tree = MemoryTree::default();
        let version_arguments = arguments(&["version"]);

        assert_eq!(execute(&tree, &version_arguments).exit_code, 0);
        assert_eq!(
            execute_with(&tree, &version_arguments, Some("unused")).exit_code,
            0
        );
        assert_eq!(
            execute_using(&tree, &version_arguments, None, &NoLauncher).exit_code,
            0
        );
        assert_eq!(
            execute_using_with_mutations(
                &tree,
                &version_arguments,
                None,
                &NoLauncher,
                &NoMutationRunner,
            )
            .exit_code,
            0
        );
        let json = execute(&tree, &arguments(&["version", "--json"]));
        assert_eq!(json.exit_code, 0);
        assert!(json.stdout.contains("schemaVersion"));
    }

    #[test]
    fn grouped_dispatch_reaches_every_declared_owner_without_implicit_policy() {
        let tree = grouped_tree();
        for command in [
            &["gate", "list"][..],
            &["gate", "validate"],
            &["gate", "run", "--surface", "pre-commit"],
            &["harness", "adapters", "validate"],
            &["harness", "adapters", "generate"],
            &["env", "backup", "--dir", "backups"],
            &["env", "validate"],
            &["env", "init"],
            &["env", "restore", "--dir", "backups"],
            &["toolchain", "validate"],
            &["toolchain", "provision"],
            &["convention", "license", "validate"],
            &["md", "frontmatter", "validate"],
            &["md", "internal-link", "validate"],
            &["md", "mermaid", "validate"],
            &["md", "readme-index", "validate"],
            &["governance", "vendor", "validate"],
            &["governance", "layers", "validate"],
            &["governance", "traceability", "validate"],
            &["md", "heading-hierarchy", "validate"],
        ] {
            let outcome = execute(&tree, &arguments(command));
            assert!(
                matches!(outcome.exit_code, 0 | 2),
                "{} returned {}: {}",
                command.join(" "),
                outcome.exit_code,
                outcome.stderr
            );
        }
    }

    #[test]
    fn stable_reader_refuses_predecessor_configuration_before_dispatch() {
        let tree = MemoryTree::default();
        tree.write(config::PATH, "schema: ose/repo-config/v2\n");
        let outcome = execute(&tree, &arguments(&["repo-config", "validate"]));
        assert_eq!(outcome.exit_code, 2, "{}", outcome.stderr);
        assert!(
            outcome
                .stderr
                .contains("no longer accepts predecessor configuration")
        );
    }

    #[test]
    fn stable_cli_does_not_register_retired_legacy_leaves() {
        let tree = grouped_tree();
        for command in [
            &["governance", "roots", "validate"][..],
            &["governance", "companions", "validate"],
            &["governance", "instructions", "validate"],
            &["plan", "validate"],
            &["harness", "parity", "validate"],
            &["md", "word-count", "inspect"],
        ] {
            let outcome = execute(&tree, &arguments(command));
            assert_eq!(outcome.exit_code, 2, "{}", command.join(" "));
            assert!(
                outcome.stderr.contains("unrecognized command"),
                "{}: {}",
                command.join(" "),
                outcome.stderr
            );
        }
    }

    #[test]
    fn load_refuses_nontext_configuration_before_any_policy_can_run() {
        let mut tree = MemoryTree::default();
        tree.mark_binary(config::PATH);

        let outcome = execute(&tree, &arguments(&["repo-config", "validate"]));

        assert_eq!(outcome.exit_code, 2);
        assert!(outcome.stderr.contains("holds no text"));
    }

    #[test]
    fn public_boundary_returns_help_version_root_and_configuration_faults_before_dispatch() {
        let tree = grouped_tree();
        let help = execute(&tree, &arguments(&["--help"]));
        assert_eq!(help.exit_code, 0);
        assert!(help.stdout.contains("Repository Hygiene"));
        let version_text = execute(&tree, &arguments(&["version"]));
        assert_eq!(version_text.exit_code, 0);
        assert!(version_text.stdout.starts_with('v'));
        assert_eq!(
            execute(
                &tree,
                &arguments(&["repo-config", "validate", "--root", "missing"])
            )
            .exit_code,
            2
        );

        let missing = MemoryTree::default();
        assert_eq!(
            execute(&missing, &arguments(&["repo-config", "validate"])).exit_code,
            2
        );
        let mut unreadable = MemoryTree::default();
        unreadable.mark_unreadable(config::PATH);
        assert_eq!(
            execute(&unreadable, &arguments(&["repo-config", "validate"])).exit_code,
            2
        );
        assert!(matches!(load(&missing), Err(ConfigError::Missing)));
    }
}
