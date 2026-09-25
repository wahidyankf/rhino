//! Grouped-v0.4 lifecycle gate views.
//!
//! Listing and validation do not run a child. They expose the same typed
//! declaration that dispatch will consume, so a hook and a workflow cannot
//! disagree about which semantic IDs belong to one lifecycle surface.

use crate::Outcome;
use crate::cli::{Format, Invocation};
use crate::errors::ErrorCode;
use crate::runtime::{
    Launch, LaunchError, LaunchFailure, Launched, Launcher, Mutated, MutationError, MutationLaunch,
    MutationRunner, Tree, TreeError,
};
use crate::v0_4::config::{
    ArgumentExpansion, Gate, GateKind, Gates, InputBinding, InputKind, InputSource, Surface,
};
use serde::Serialize;
use std::collections::{BTreeMap, BTreeSet};

/// An omitted `gates` group is refused, never read as an empty lifecycle.
///
/// `gates: {}` is a repository saying it runs no gate; leaving the group out
/// says nothing at all. Treating the two alike would let a gate surface report
/// clean for a repository that never declared one, which is the quiet pass
/// every other omitted group already refuses.
fn undeclared(format: Format) -> Outcome {
    Outcome::refusal(
        format,
        ErrorCode::ConfigUndeclared,
        "gates section is not declared, and RHINO holds no default for it",
    )
}

/// Render the seven closed surfaces and their declared semantic gate IDs.
pub(crate) fn list(gates: Option<&Gates>, format: Format) -> Outcome {
    let Some(gates) = gates else {
        return undeclared(format);
    };
    let listed: Vec<SurfaceListing> = Surface::ALL
        .iter()
        .map(|surface| SurfaceListing {
            surface: surface.name(),
            gates: gates
                .entries
                .iter()
                .filter(|gate| gate.run_on.contains_key(surface))
                .map(|gate| gate.id.as_str())
                .collect(),
        })
        .collect();

    match format {
        Format::Text => {
            let mut text = String::new();
            for surface in &listed {
                let ids = if surface.gates.is_empty() {
                    "no gates".to_string()
                } else {
                    surface.gates.join(", ")
                };
                text.push_str(&format!("[gate] {}: {ids}\n", surface.surface));
            }
            Outcome::clean(text)
        }
        // Through a `Value` rather than straight to a string, because that
        // turn cannot fail: every field here is a number, a string, or a list
        // of them. The fallible spelling bought an arm no test could reach and
        // an error code no caller could ever see.
        Format::Json => Outcome::clean(format!(
            "{}\n",
            serde_json::json!(ListingEnvelope {
                schema_version: 1,
                command: ["gate", "list"],
                result: "clean",
                surfaces: listed,
            })
        )),
    }
}

/// Parsing has already applied every lifecycle invariant. This leaf gives
/// adapters a read-only assertion they can call without starting a child.
pub(crate) fn validate(gates: Option<&Gates>, format: Format) -> Outcome {
    if gates.is_none() {
        return undeclared(format);
    }
    match format {
        Format::Text => Outcome::clean("[gate] lifecycle configuration is valid\n"),
        Format::Json => Outcome::clean(
            "{\"schemaVersion\":1,\"command\":[\"gate\",\"validate\"],\"result\":\"clean\"}\n",
        ),
    }
}

/// Resolve one typed lifecycle invocation and start only the declared child
/// vectors. The source variants are selected by the semantic surface; a caller
/// cannot append a replacement command after `--`.
pub(crate) fn run(
    gates: Option<&Gates>,
    invocation: &Invocation,
    tree: &dyn Tree,
    stdin: Option<&str>,
    launcher: &dyn Launcher,
    mutations: &dyn MutationRunner,
) -> Outcome {
    let Some(gates) = gates else {
        return undeclared(invocation.format);
    };
    // Absent and unknown are different mistakes: one leaves out an option the
    // command needs, the other gives it a value this build does not know.
    let Some(surface) = invocation.surface.as_deref().and_then(surface) else {
        let code = match invocation.surface {
            None => ErrorCode::ArgsIncomplete,
            Some(_) => ErrorCode::ArgsUnrecognized,
        };
        return Outcome::refusal(
            invocation.format,
            code,
            format!(
                "`--surface` must name one of {}",
                Surface::ALL
                    .iter()
                    .map(|known| known.name())
                    .collect::<Vec<_>>()
                    .join(", ")
            ),
        );
    };
    if invocation.base.is_some() != invocation.head.is_some() {
        return Outcome::refusal(
            invocation.format,
            ErrorCode::ArgsIncomplete,
            "`--base` and `--head` must be supplied together",
        );
    }

    let mut report = String::new();
    let mut completed = Vec::new();
    let mut changed_paths = Vec::new();
    for gate in gates
        .entries
        .iter()
        .filter(|gate| gate.run_on.contains_key(&surface))
    {
        let Some(membership) = gate.run_on.get(&surface) else {
            continue;
        };
        let resolved = match resolve_inputs(gate, membership.bind.iter(), invocation, tree, stdin) {
            Ok(Some(inputs)) => inputs,
            Ok(None) => continue,
            Err((code, reason)) => {
                return Outcome::refusal(
                    invocation.format,
                    code,
                    format!("gate `{}`: {reason}", gate.id),
                );
            }
        };
        let Some(command) = &gate.command else {
            return Outcome::refusal(
                invocation.format,
                ErrorCode::GateUndeclared,
                format!("gate `{}` declares no executable command", gate.id),
            );
        };
        let (arguments, environment) = match project(command, &resolved) {
            Ok(projected) => projected,
            Err(reason) => {
                return Outcome::refusal(
                    invocation.format,
                    ErrorCode::ConfigUnusable,
                    format!("gate `{}`: {reason}", gate.id),
                );
            }
        };
        if gate.kind == GateKind::Mutation {
            let root = tree.root();
            let selected_paths = selected_paths(&resolved, membership.bind.iter());
            let mutation = MutationLaunch {
                arguments: &arguments,
                directory: &root,
                environment: &environment,
                selected_paths: &selected_paths,
                revision: invocation.head.as_deref(),
            };
            let result = match surface {
                Surface::PreCommit => mutations.apply_index(mutation),
                Surface::PullRequest => mutations.verify_clean(mutation),
                _ => Err(MutationError(
                    "a grouped mutation may run only at pre-commit or pull-request".to_string(),
                )),
            };
            match result {
                Ok(Mutated {
                    code: 0,
                    changes,
                    divergences,
                }) => {
                    for path in changes {
                        report.push_str(&format!("[gate] {} changed {path}\n", gate.id));
                        changed_paths.push(path);
                    }
                    for path in divergences {
                        report.push_str(&format!("[gate] {} preserved unstaged {path}\n", gate.id));
                    }
                    report.push_str(&format!("[gate] {} passed\n", gate.id));
                    completed.push(gate.id.as_str());
                    continue;
                }
                Ok(Mutated {
                    code: _,
                    changes,
                    divergences,
                }) => {
                    for path in &changes {
                        report.push_str(&format!("[gate] {} changed {path}\n", gate.id));
                    }
                    for path in divergences {
                        report.push_str(&format!("[gate] {} preserved unstaged {path}\n", gate.id));
                    }
                    report.push_str(&format!("[gate] {} failed\n", gate.id));
                    return run_outcome(
                        invocation.format,
                        1,
                        "findings",
                        &completed,
                        &changes,
                        report,
                        format!(
                            "[gate] {} reported a finding at {}\n",
                            gate.id,
                            surface.name()
                        ),
                    );
                }
                Err(MutationError(reason)) => {
                    report.push_str(&format!("[gate] {} could not run\n", gate.id));
                    return run_refusal(
                        invocation.format,
                        2,
                        ErrorCode::GateChildRefused,
                        report,
                        format!("gate `{}`: {reason}", gate.id),
                    );
                }
            }
        }
        match launcher.launch(Launch {
            arguments: &arguments,
            directory: &tree.root(),
            surface: surface.name(),
            stdin,
            environment: &environment,
        }) {
            Ok(Launched { code: 0 }) => {
                report.push_str(&format!("[gate] {} passed\n", gate.id));
                completed.push(gate.id.as_str());
            }
            Ok(_) => {
                report.push_str(&format!("[gate] {} failed\n", gate.id));
                return run_outcome(
                    invocation.format,
                    1,
                    "findings",
                    &completed,
                    &[],
                    report,
                    format!(
                        "[gate] {} reported a finding at {}\n",
                        gate.id,
                        surface.name()
                    ),
                );
            }
            Err(LaunchError { reason, failure }) => {
                report.push_str(&format!("[gate] {} could not run\n", gate.id));
                // 127 and 126 are what a shell already reports for these two,
                // so a caller reading the status needs no rhino-specific
                // vocabulary to act on it. Anything else is this tool failing
                // to run, which is 2.
                let (status, code) = match failure {
                    LaunchFailure::NotFound => (127, ErrorCode::GateChildNotFound),
                    LaunchFailure::NotExecutable => (126, ErrorCode::GateChildNotExecutable),
                    LaunchFailure::Refused => (2, ErrorCode::GateChildRefused),
                };
                return run_refusal(
                    invocation.format,
                    status,
                    code,
                    report,
                    format!("gate `{}`: {reason}", gate.id),
                );
            }
        }
    }
    run_outcome(
        invocation.format,
        0,
        "clean",
        &completed,
        &changed_paths,
        report,
        String::new(),
    )
}

/// A run that never reached a verdict.
///
/// Stdout stays empty. A caller parsing the result document has to be able to
/// trust that stdout either holds a verdict or holds nothing, so the progress
/// lines accumulated before the failure move to stderr alongside the
/// diagnostic rather than masquerading as a result.
fn run_refusal(
    format: Format,
    exit_code: u8,
    code: ErrorCode,
    progress: String,
    message: String,
) -> Outcome {
    // The progress lines are a text-mode affordance. Under `--output json` a
    // caller reading stderr wants one document, not a document preceded by
    // prose it has to learn to skip.
    let progress = match format {
        Format::Text => progress,
        Format::Json => String::new(),
    };
    Outcome {
        exit_code,
        stdout: String::new(),
        stderr: format!("{progress}{}", crate::errors::body(format, code, &message)),
    }
}

fn run_outcome(
    format: Format,
    exit_code: u8,
    result: &str,
    completed: &[&str],
    changes: &[String],
    text: String,
    stderr: String,
) -> Outcome {
    match format {
        Format::Text => Outcome {
            exit_code,
            stdout: text,
            stderr,
        },
        Format::Json => Outcome {
            exit_code,
            stdout: format!(
                "{}\n",
                serde_json::json!({
                    "schemaVersion": 1,
                    "command": ["gate", "run"],
                    "result": result,
                    "summary": {
                        "scanned": completed.len(),
                        "findings": if exit_code == 1 { 1 } else { 0 },
                        "changed": 0,
                    },
                    "findings": [],
                    "changes": changes,
                    "metadata": { "gates": completed },
                })
            ),
            stderr,
        },
    }
}

fn surface(name: &str) -> Option<Surface> {
    Surface::ALL.into_iter().find(|known| known.name() == name)
}

#[derive(Debug, Clone, PartialEq, Eq)]
enum ResolvedInput {
    Files(Vec<String>),
    CommitMessage(String),
    CommitRange { base: String, head: String },
    RepositoryState { root: String },
}

fn resolve_inputs<'a>(
    gate: &Gate,
    bindings: impl Iterator<Item = (&'a String, &'a InputBinding)>,
    invocation: &Invocation,
    tree: &dyn Tree,
    stdin: Option<&str>,
) -> Result<Option<BTreeMap<String, ResolvedInput>>, (ErrorCode, String)> {
    // Each failure names what went wrong in the closed vocabulary: an option
    // the invocation left out, a value it gave that is not a commit, a file it
    // named, standard input, or a Git answer the repository could not give.
    // None of them is a fault in the configuration, which is already valid.
    let incomplete = |reason: &str| (ErrorCode::ArgsIncomplete, reason.to_string());
    let not_commits = || {
        (
            ErrorCode::ArgsUnrecognized,
            "requires hexadecimal commit IDs for `--base` and `--head`".to_string(),
        )
    };
    let git = |reason: String| (ErrorCode::RepositoryUnusable, reason);
    let mut resolved = BTreeMap::new();
    for (name, binding) in bindings {
        let input = gate
            .inputs
            .get(name)
            .expect("the grouped configuration validates every binding name");
        let value = match (input.kind, binding.source) {
            (InputKind::Files, InputSource::Checkout) => ResolvedInput::Files(tree.files()),
            (InputKind::Files, InputSource::ExplicitRange) => {
                let (Some(base), Some(head)) =
                    (invocation.base.as_deref(), invocation.head.as_deref())
                else {
                    return Err(incomplete(
                        "requires both `--base` and `--head` for an explicit file range",
                    ));
                };
                if !is_commit(base) || !is_commit(head) {
                    return Err(not_commits());
                }
                let paths = tree.changed_files(base, head).map_err(|reason| {
                    git(format!(
                        "cannot read changed files for the explicit range: {reason}"
                    ))
                })?;
                ResolvedInput::Files(paths)
            }
            (InputKind::RepositoryState, InputSource::Checkout) => {
                ResolvedInput::RepositoryState { root: tree.root() }
            }
            (InputKind::CommitMessage, InputSource::HookMessageFile) => {
                let Some(path) = invocation.message_file.as_deref() else {
                    return Err(incomplete(
                        "requires `--message-file` for a commit-message input",
                    ));
                };
                let message = tree.hook_message_file(path).map_err(|error| match error {
                    TreeError::NotFound => (
                        ErrorCode::FileMissing,
                        format!("message file `{path}` does not exist"),
                    ),
                    TreeError::Unreadable(reason) => (
                        ErrorCode::FileUnreadable,
                        format!("message file `{path}` cannot be read: {reason}"),
                    ),
                    TreeError::NotText => (
                        ErrorCode::FileUnreadable,
                        format!("message file `{path}` is not text"),
                    ),
                })?;
                ResolvedInput::CommitMessage(message)
            }
            (InputKind::CommitMessage, InputSource::ExplicitRange) => {
                let (Some(base), Some(head)) =
                    (invocation.base.as_deref(), invocation.head.as_deref())
                else {
                    return Err(incomplete(
                        "requires both `--base` and `--head` for an explicit commit range",
                    ));
                };
                if !is_commit(base) || !is_commit(head) {
                    return Err(not_commits());
                }
                let messages = tree.commit_messages(base, head).map_err(|reason| {
                    git(format!(
                        "cannot read commit messages for the explicit range: {reason}"
                    ))
                })?;
                ResolvedInput::CommitMessage(messages)
            }
            (InputKind::CommitRange, InputSource::ExplicitRange) => {
                let (Some(base), Some(head)) =
                    (invocation.base.as_deref(), invocation.head.as_deref())
                else {
                    return Err(incomplete(
                        "requires both `--base` and `--head` for an explicit commit range",
                    ));
                };
                if !is_commit(base) || !is_commit(head) {
                    return Err(not_commits());
                }
                ResolvedInput::CommitRange {
                    base: base.to_string(),
                    head: head.to_string(),
                }
            }
            (InputKind::CommitRange, InputSource::PushUpdates) => {
                if !invocation.push_updates_stdin {
                    return Err(incomplete(
                        "requires `--push-updates-stdin` for a push-update range",
                    ));
                }
                let parsed =
                    parse_push_range(stdin.unwrap_or_default(), binding.fallback.as_deref())
                        .map_err(|reason| (ErrorCode::InputUnreadable, reason))?;
                let (base, head) = match parsed {
                    PushRange::Range { base, head } => (base, head),
                    PushRange::NewRef { fallback, head } => (
                        tree.resolve_git_ref(&fallback).map_err(|reason| {
                            git(format!(
                                "cannot resolve push fallback `{fallback}`: {reason}"
                            ))
                        })?,
                        head,
                    ),
                    PushRange::Deleted => return Ok(None),
                };
                ResolvedInput::CommitRange { base, head }
            }
            (InputKind::Files, InputSource::GitIndex) => ResolvedInput::Files(
                tree.indexed_files()
                    .map_err(|reason| git(format!("cannot resolve the Git index: {reason}")))?,
            ),
            _ => {
                return Err((
                    ErrorCode::ConfigUnusable,
                    "has an invalid typed input binding".to_string(),
                ));
            }
        };
        resolved.insert(name.clone(), value);
    }
    Ok(Some(resolved))
}

fn selected_paths<'a>(
    inputs: &BTreeMap<String, ResolvedInput>,
    bindings: impl Iterator<Item = (&'a String, &'a InputBinding)>,
) -> Vec<String> {
    bindings
        .filter(|(_, binding)| {
            matches!(
                binding.source,
                InputSource::GitIndex | InputSource::ExplicitRange
            )
        })
        .filter_map(|(name, _)| inputs.get(name))
        .filter_map(|input| match input {
            ResolvedInput::Files(paths) => Some(paths),
            _ => None,
        })
        .flatten()
        .cloned()
        .collect::<BTreeSet<_>>()
        .into_iter()
        .collect()
}

fn project(
    command: &crate::v0_4::config::GateCommand,
    inputs: &BTreeMap<String, ResolvedInput>,
) -> Result<(Vec<String>, BTreeMap<String, String>), String> {
    let mut arguments = vec![command.executable.clone()];
    for argument in &command.args {
        match (&argument.literal, &argument.input, argument.expand) {
            (Some(literal), None, None) => arguments.push(literal.clone()),
            (None, Some(reference), Some(expansion)) => {
                let values = referenced_values(inputs, reference)?;
                match expansion {
                    ArgumentExpansion::Single if values.len() == 1 => {
                        arguments.push(values[0].clone())
                    }
                    ArgumentExpansion::Single => {
                        return Err(format!(
                            "`{reference}` is not one value for `expand: single`"
                        ));
                    }
                    ArgumentExpansion::Repeat => arguments.extend(values),
                }
            }
            _ => return Err("has an invalid argv item after validation".to_string()),
        }
    }
    let mut environment = BTreeMap::new();
    for (name, projection) in &command.environment {
        let values = referenced_values(inputs, &projection.input)?;
        if values.len() != 1 {
            return Err(format!(
                "`{}` cannot project a sequence into environment variable `{name}`",
                projection.input
            ));
        }
        environment.insert(name.clone(), values[0].clone());
    }
    Ok((arguments, environment))
}

fn referenced_values(
    inputs: &BTreeMap<String, ResolvedInput>,
    reference: &str,
) -> Result<Vec<String>, String> {
    let Some((name, field)) = reference.split_once('.') else {
        return Err(format!("`{reference}` is not an input field reference"));
    };
    let Some(input) = inputs.get(name) else {
        return Err(format!("`{reference}` was not resolved for this surface"));
    };
    match (input, field) {
        (ResolvedInput::Files(paths), "paths") => Ok(paths.clone()),
        (ResolvedInput::CommitMessage(text), "text") => Ok(vec![text.clone()]),
        (ResolvedInput::CommitRange { base, .. }, "base") => Ok(vec![base.clone()]),
        (ResolvedInput::CommitRange { head, .. }, "head") => Ok(vec![head.clone()]),
        (ResolvedInput::RepositoryState { root }, "root") => Ok(vec![root.clone()]),
        _ => Err(format!(
            "`{reference}` is not available on its resolved input"
        )),
    }
}

fn is_commit(value: &str) -> bool {
    (7..=64).contains(&value.len()) && value.chars().all(|character| character.is_ascii_hexdigit())
}

enum PushRange {
    Range { base: String, head: String },
    NewRef { fallback: String, head: String },
    Deleted,
}

fn parse_push_range(stdin: &str, fallback: Option<&str>) -> Result<PushRange, String> {
    let mut records = stdin.lines();
    let Some(record) = records.next() else {
        return Err("received no pre-push update record".to_string());
    };
    if records.next().is_some() {
        return Err(
            "received multiple pre-push update records; select one declared ref first".to_string(),
        );
    }
    let fields: Vec<&str> = record.split_whitespace().collect();
    let [_, local, _, remote] = fields.as_slice() else {
        return Err("received an invalid pre-push update record".to_string());
    };
    if is_zero(local) {
        return Ok(PushRange::Deleted);
    }
    if !is_commit(local) {
        return Err("received a push update without a hexadecimal local commit ID".to_string());
    }
    if is_zero(remote) {
        let Some(fallback) = fallback else {
            return Err("received a new push ref without a declared fallback ref".to_string());
        };
        return Ok(PushRange::NewRef {
            fallback: fallback.to_string(),
            head: (*local).to_string(),
        });
    }
    if !is_commit(remote) {
        return Err("received a push update without a hexadecimal remote commit ID".to_string());
    }
    Ok(PushRange::Range {
        base: (*remote).to_string(),
        head: (*local).to_string(),
    })
}

fn is_zero(value: &str) -> bool {
    !value.is_empty() && value.bytes().all(|byte| byte == b'0')
}

#[derive(Serialize)]
struct ListingEnvelope<'a> {
    #[serde(rename = "schemaVersion")]
    schema_version: u8,
    command: [&'a str; 2],
    result: &'a str,
    surfaces: Vec<SurfaceListing<'a>>,
}

#[derive(Serialize)]
struct SurfaceListing<'a> {
    surface: &'a str,
    gates: Vec<&'a str>,
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::cli::Invocation;
    use crate::runtime::{
        Launch, LaunchError, Launched, Launcher, MemoryTree, Mutated, MutationError,
        MutationLaunch, MutationRunner, NoMutationRunner,
    };
    use crate::v0_4::config::{
        Argument, EnvironmentProjection, GateInput, GateMembership, InputBinding, parse,
    };
    use std::cell::RefCell;
    use std::collections::BTreeMap;

    type CapturedLaunch = (Vec<String>, BTreeMap<String, String>);
    type CapturedReplay = (Vec<String>, Vec<String>, Option<String>);

    #[derive(Default)]
    struct CapturingLauncher {
        calls: RefCell<Vec<CapturedLaunch>>,
    }

    impl Launcher for CapturingLauncher {
        fn launch(&self, launch: Launch<'_>) -> Result<Launched, LaunchError> {
            self.calls
                .borrow_mut()
                .push((launch.arguments.to_vec(), launch.environment.clone()));
            Ok(Launched { code: 0 })
        }
    }

    #[derive(Default)]
    struct CapturingMutations {
        apply_index: RefCell<Vec<(Vec<String>, Vec<String>)>>,
        verify_clean: RefCell<Vec<CapturedReplay>>,
    }

    enum LaunchResult {
        Clean,
        Finding,
        Error,
        NotFound,
        NotExecutable,
    }

    struct TerminalLauncher(LaunchResult);

    impl Launcher for TerminalLauncher {
        fn launch(&self, _launch: Launch<'_>) -> Result<Launched, LaunchError> {
            match self.0 {
                LaunchResult::Clean => Ok(Launched { code: 0 }),
                LaunchResult::Finding => Ok(Launched { code: 1 }),
                LaunchResult::Error => Err(LaunchError::refused("launcher refused")),
                LaunchResult::NotFound => Err(LaunchError::from_spawn(
                    "/no/such/program",
                    &std::io::Error::from(std::io::ErrorKind::NotFound),
                )),
                LaunchResult::NotExecutable => Err(LaunchError::from_spawn(
                    "not-executable",
                    &std::io::Error::from(std::io::ErrorKind::PermissionDenied),
                )),
            }
        }
    }

    enum MutationResult {
        Changed,
        Finding,
        Error,
    }

    struct TerminalMutations(MutationResult);

    impl MutationRunner for TerminalMutations {
        fn apply_index(&self, _launch: MutationLaunch<'_>) -> Result<Mutated, MutationError> {
            match self.0 {
                MutationResult::Changed => Ok(Mutated {
                    code: 0,
                    changes: vec!["docs/changed.md".to_string()],
                    divergences: vec!["docs/unstaged.md".to_string()],
                }),
                MutationResult::Finding => Ok(Mutated {
                    code: 1,
                    changes: vec!["docs/changed.md".to_string()],
                    divergences: vec!["docs/unstaged.md".to_string()],
                }),
                MutationResult::Error => Err(MutationError("mutation refused".to_string())),
            }
        }

        fn verify_clean(&self, launch: MutationLaunch<'_>) -> Result<Mutated, MutationError> {
            self.apply_index(launch)
        }
    }

    fn manual_gate(command: Option<crate::v0_4::config::GateCommand>) -> Gates {
        Gates {
            entries: vec![Gate {
                id: "check".to_string(),
                kind: GateKind::Check,
                inputs: BTreeMap::new(),
                command,
                mutation: None,
                run_on: BTreeMap::from([(Surface::Manual, GateMembership::default())]),
            }],
            composition: None,
        }
    }

    fn command() -> crate::v0_4::config::GateCommand {
        crate::v0_4::config::GateCommand {
            executable: "checker".to_string(),
            args: Vec::new(),
            environment: BTreeMap::new(),
        }
    }

    impl MutationRunner for CapturingMutations {
        fn apply_index(&self, launch: MutationLaunch<'_>) -> Result<Mutated, MutationError> {
            self.apply_index
                .borrow_mut()
                .push((launch.arguments.to_vec(), launch.selected_paths.to_vec()));
            Ok(Mutated {
                code: 0,
                changes: Vec::new(),
                divergences: Vec::new(),
            })
        }

        fn verify_clean(&self, launch: MutationLaunch<'_>) -> Result<Mutated, MutationError> {
            self.verify_clean.borrow_mut().push((
                launch.arguments.to_vec(),
                launch.selected_paths.to_vec(),
                launch.revision.map(str::to_string),
            ));
            Ok(Mutated {
                code: 0,
                changes: Vec::new(),
                divergences: Vec::new(),
            })
        }
    }

    #[test]
    fn typed_range_projection_keeps_argv_and_environment_separate() {
        let document = parse(
            r#"schema: rhino/repo-config/v2
gates:
  entries:
    - id: range-check
      type: check
      inputs:
        range: { kind: commit-range }
      command:
        executable: npm
        args:
          - { literal: exec }
          - { literal: -- }
          - { literal: nx }
          - { literal: affected }
        environment:
          NX_BASE: { input: range.base }
          NX_HEAD: { input: range.head }
      run-on:
        pre-push:
          bind:
            range: { source: push-updates, fallback: refs/remotes/origin/main }
        pull-request:
          bind:
            range: { source: explicit-range, range: explicit }
  composition:
    pull-request:
      relation: exact
"#,
        )
        .expect("the typed test model parses");
        let invocation = Invocation {
            category: "gate",
            surface: Some("pre-push".to_string()),
            push_updates_stdin: true,
            ..Invocation::default()
        };
        let launcher = CapturingLauncher::default();
        let outcome = run(
            document.gates.as_ref(),
            &invocation,
            &MemoryTree::default(),
            Some("refs/heads/main aaaaaaaa refs/heads/main bbbbbbbb\n"),
            &launcher,
            &NoMutationRunner,
        );

        assert_eq!(outcome.exit_code, 0);
        assert_eq!(
            launcher.calls.into_inner(),
            vec![(
                [
                    "npm".to_string(),
                    "exec".to_string(),
                    "--".to_string(),
                    "nx".to_string(),
                    "affected".to_string(),
                ]
                .to_vec(),
                BTreeMap::from([
                    ("NX_BASE".to_string(), "bbbbbbbb".to_string()),
                    ("NX_HEAD".to_string(), "aaaaaaaa".to_string()),
                ])
            )]
        );
    }

    #[test]
    fn no_declaration_means_no_child_starts() {
        let launcher = CapturingLauncher::default();
        let outcome = run(
            Some(&Gates::default()),
            &Invocation {
                category: "gate",
                surface: Some("manual".to_string()),
                ..Invocation::default()
            },
            &MemoryTree::default(),
            None,
            &launcher,
            &NoMutationRunner,
        );

        assert_eq!(outcome.exit_code, 0);
        assert!(launcher.calls.into_inner().is_empty());
    }

    #[test]
    fn mutation_pre_commit_receives_only_the_declared_index_snapshot() {
        let document = parse(
            r#"schema: rhino/repo-config/v2
gates:
  entries:
    - id: staged-format
      type: mutation
      inputs:
        staged: { kind: files }
      mutation:
        local: apply-index
        ci: verify-clean
      command:
        executable: formatter
        args:
          - { input: staged.paths, expand: repeat }
      run-on:
        pre-commit:
          bind:
            staged: { source: git-index }
        pull-request:
          bind:
            staged: { source: git-index }
  composition:
    pull-request:
      relation: exact
"#,
        )
        .expect("the mutation test model parses");
        let mut tree = MemoryTree::default();
        tree.write("notes/staged.md", "staged bytes");
        tree.write("notes/unstaged.md", "unstaged bytes");
        tree.set_indexed_files(["notes/staged.md"]);
        let mutations = CapturingMutations::default();
        let outcome = run(
            document.gates.as_ref(),
            &Invocation {
                category: "gate",
                surface: Some("pre-commit".to_string()),
                ..Invocation::default()
            },
            &tree,
            None,
            &CapturingLauncher::default(),
            &mutations,
        );

        assert_eq!(outcome.exit_code, 0);
        assert_eq!(
            mutations.apply_index.into_inner(),
            vec![(
                vec!["formatter".to_string(), "notes/staged.md".to_string()],
                vec!["notes/staged.md".to_string()],
            )]
        );
    }

    #[test]
    fn mutation_pull_request_receives_an_explicit_candidate_revision() {
        let document = parse(
            r#"schema: rhino/repo-config/v2
gates:
  entries:
    - id: candidate-format
      type: mutation
      inputs:
        files: { kind: files }
      mutation:
        local: apply-index
        ci: verify-clean
      command:
        executable: formatter
        args:
          - { input: files.paths, expand: repeat }
      run-on:
        pre-commit:
          bind:
            files: { source: git-index }
        pull-request:
          bind:
            files: { source: explicit-range, range: explicit }
  composition:
    pull-request:
      relation: exact
"#,
        )
        .expect("the pull-request mutation model parses");
        let mut tree = MemoryTree::default();
        tree.write("notes/candidate.md", "candidate bytes");
        tree.write("notes/unselected.md", "unselected bytes");
        tree.set_changed_files("aaaaaaaa", "bbbbbbbb", ["notes/candidate.md"]);
        let mutations = CapturingMutations::default();
        let outcome = run(
            document.gates.as_ref(),
            &Invocation {
                category: "gate",
                surface: Some("pull-request".to_string()),
                base: Some("aaaaaaaa".to_string()),
                head: Some("bbbbbbbb".to_string()),
                ..Invocation::default()
            },
            &tree,
            None,
            &CapturingLauncher::default(),
            &mutations,
        );

        assert_eq!(outcome.exit_code, 0);
        assert_eq!(
            mutations.verify_clean.into_inner(),
            vec![(
                vec!["formatter".to_string(), "notes/candidate.md".to_string()],
                vec!["notes/candidate.md".to_string()],
                Some("bbbbbbbb".to_string()),
            )]
        );
    }

    #[test]
    fn push_new_ref_uses_the_declared_fallback_and_deletion_skips_the_gate() {
        let document = parse(
            r#"schema: rhino/repo-config/v2
gates:
  entries:
    - id: range-check
      type: check
      inputs:
        range: { kind: commit-range }
      command:
        executable: runner
        args:
          - { input: range.base, expand: single }
          - { input: range.head, expand: single }
        environment:
          BASE: { input: range.base }
          HEAD: { input: range.head }
      run-on:
        pre-push:
          bind:
            range: { source: push-updates, fallback: refs/remotes/origin/main }
        pull-request:
          bind:
            range: { source: explicit-range, range: explicit }
  composition:
    pull-request:
      relation: exact
"#,
        )
        .expect("the push test model parses");
        let mut tree = MemoryTree::default();
        tree.set_git_ref("refs/remotes/origin/main", "cccccccc");
        let launcher = CapturingLauncher::default();
        let outcome = run(
            document.gates.as_ref(),
            &Invocation {
                category: "gate",
                surface: Some("pre-push".to_string()),
                push_updates_stdin: true,
                ..Invocation::default()
            },
            &tree,
            Some(
                "refs/heads/new aaaaaaaa refs/heads/new 0000000000000000000000000000000000000000\n",
            ),
            &launcher,
            &NoMutationRunner,
        );
        assert_eq!(outcome.exit_code, 0);
        assert_eq!(
            launcher.calls.borrow().as_slice(),
            [(
                vec![
                    "runner".to_string(),
                    "cccccccc".to_string(),
                    "aaaaaaaa".to_string(),
                ],
                BTreeMap::from([
                    ("BASE".to_string(), "cccccccc".to_string()),
                    ("HEAD".to_string(), "aaaaaaaa".to_string()),
                ]),
            )]
        );
        let deletion = run(
            document.gates.as_ref(),
            &Invocation {
                category: "gate",
                surface: Some("pre-push".to_string()),
                push_updates_stdin: true,
                ..Invocation::default()
            },
            &tree,
            Some(
                "refs/heads/deleted 0000000000000000000000000000000000000000 refs/heads/deleted aaaaaaaa\n",
            ),
            &CapturingLauncher::default(),
            &NoMutationRunner,
        );
        assert_eq!(deletion.exit_code, 0);
        assert!(deletion.stdout.is_empty());
    }

    #[test]
    fn grouped_gate_listing_and_validation_cover_text_and_json_without_a_child() {
        let empty = Gates::default();
        let listed = list(Some(&empty), Format::Text);
        assert_eq!(listed.exit_code, 0);
        assert!(listed.stdout.contains("[gate] pre-commit: no gates"));
        let json = list(Some(&manual_gate(None)), Format::Json);
        assert_eq!(json.exit_code, 0);
        assert!(json.stdout.contains("\"manual\""));
        assert!(json.stdout.contains("\"check\""));
        assert!(
            validate(Some(&empty), Format::Text)
                .stdout
                .contains("configuration is valid")
        );
        assert!(
            validate(Some(&empty), Format::Json)
                .stdout
                .contains("\"gate\",\"validate\"")
        );
    }

    #[test]
    fn an_omitted_gates_group_is_refused_by_every_leaf_rather_than_read_as_empty() {
        let tree = MemoryTree::default();
        let invocation = Invocation {
            category: "gate",
            surface: Some("manual".to_string()),
            format: Format::Json,
            ..Invocation::default()
        };
        for outcome in [
            list(None, Format::Json),
            validate(None, Format::Json),
            run(
                None,
                &invocation,
                &tree,
                None,
                &CapturingLauncher::default(),
                &NoMutationRunner,
            ),
        ] {
            assert_eq!(outcome.exit_code, 2);
            assert!(outcome.stdout.is_empty());
            assert!(outcome.stderr.contains("\"rhino.config.undeclared\""));
            assert!(outcome.stderr.contains("gates section is not declared"));
        }
    }

    #[test]
    fn grouped_gate_run_refuses_invalid_invocation_or_missing_command_before_a_child() {
        let tree = MemoryTree::default();
        let empty = Gates::default();
        let no_surface = run(
            Some(&empty),
            &Invocation::default(),
            &tree,
            None,
            &CapturingLauncher::default(),
            &NoMutationRunner,
        );
        assert_eq!(no_surface.exit_code, 2);
        assert!(no_surface.stderr.contains("--surface"));

        let half_range = run(
            Some(&empty),
            &Invocation {
                category: "gate",
                surface: Some("manual".to_string()),
                base: Some("aaaaaaa".to_string()),
                ..Invocation::default()
            },
            &tree,
            None,
            &CapturingLauncher::default(),
            &NoMutationRunner,
        );
        assert_eq!(half_range.exit_code, 2);
        assert!(half_range.stderr.contains("supplied together"));

        let missing_command = run(
            Some(&manual_gate(None)),
            &Invocation {
                category: "gate",
                surface: Some("manual".to_string()),
                ..Invocation::default()
            },
            &tree,
            None,
            &CapturingLauncher::default(),
            &NoMutationRunner,
        );
        assert_eq!(missing_command.exit_code, 2);
        assert!(missing_command.stderr.contains("no executable command"));
    }

    #[test]
    fn grouped_gate_defensive_input_and_projection_errors_refuse_before_launch() {
        let message_gate = Gate {
            id: "message".to_string(),
            kind: GateKind::Check,
            inputs: BTreeMap::from([(
                "message".to_string(),
                GateInput {
                    kind: InputKind::CommitMessage,
                },
            )]),
            command: Some(command()),
            mutation: None,
            run_on: BTreeMap::new(),
        };
        let message_binding = GateMembership {
            reason: None,
            bind: BTreeMap::from([(
                "message".to_string(),
                InputBinding {
                    source: InputSource::HookMessageFile,
                    range: None,
                    fallback: None,
                },
            )]),
        };
        let invocation = Invocation {
            category: "gate",
            message_file: Some("message".to_string()),
            ..Invocation::default()
        };
        let mut unreadable = MemoryTree::default();
        unreadable.write("message", "synthetic");
        unreadable.set_hook_message_file("message");
        unreadable.mark_unreadable("message");
        assert!(
            resolve_inputs(
                &message_gate,
                message_binding.bind.iter(),
                &invocation,
                &unreadable,
                None,
            )
            .is_err()
        );
        let mut binary = MemoryTree::default();
        binary.write("message", "synthetic");
        binary.set_hook_message_file("message");
        binary.mark_binary("message");
        assert!(
            resolve_inputs(
                &message_gate,
                message_binding.bind.iter(),
                &invocation,
                &binary,
                None,
            )
            .is_err()
        );

        let range_gate = Gate {
            id: "range".to_string(),
            kind: GateKind::Check,
            inputs: BTreeMap::from([(
                "range".to_string(),
                GateInput {
                    kind: InputKind::CommitRange,
                },
            )]),
            command: Some(command()),
            mutation: None,
            run_on: BTreeMap::new(),
        };
        let push_binding = GateMembership {
            reason: None,
            bind: BTreeMap::from([(
                "range".to_string(),
                InputBinding {
                    source: InputSource::PushUpdates,
                    range: None,
                    fallback: Some("refs/main".to_string()),
                },
            )]),
        };
        assert!(
            resolve_inputs(
                &range_gate,
                push_binding.bind.iter(),
                &Invocation {
                    category: "gate",
                    push_updates_stdin: true,
                    ..Invocation::default()
                },
                &MemoryTree::default(),
                Some("local aaaaaaa remote 0000000"),
            )
            .is_err()
        );

        let files_gate = Gate {
            id: "files".to_string(),
            kind: GateKind::Check,
            inputs: BTreeMap::from([(
                "files".to_string(),
                GateInput {
                    kind: InputKind::Files,
                },
            )]),
            command: Some(command()),
            mutation: None,
            run_on: BTreeMap::new(),
        };
        let index_binding = GateMembership {
            reason: None,
            bind: BTreeMap::from([(
                "files".to_string(),
                InputBinding {
                    source: InputSource::GitIndex,
                    range: None,
                    fallback: None,
                },
            )]),
        };
        assert!(
            resolve_inputs(
                &files_gate,
                index_binding.bind.iter(),
                &Invocation::default(),
                &MemoryTree::default(),
                None,
            )
            .is_err()
        );

        let values = BTreeMap::from([(
            "files".to_string(),
            ResolvedInput::Files(vec!["a.md".to_string(), "b.md".to_string()]),
        )]);
        let single = crate::v0_4::config::GateCommand {
            executable: "checker".to_string(),
            args: vec![Argument {
                literal: None,
                input: Some("files.paths".to_string()),
                expand: Some(ArgumentExpansion::Single),
            }],
            environment: BTreeMap::from([(
                "FILES".to_string(),
                EnvironmentProjection {
                    input: "files.paths".to_string(),
                },
            )]),
        };
        assert!(project(&single, &values).is_err());
        let malformed = crate::v0_4::config::GateCommand {
            executable: "checker".to_string(),
            args: vec![Argument {
                literal: Some("literal".to_string()),
                input: None,
                expand: Some(ArgumentExpansion::Single),
            }],
            environment: BTreeMap::new(),
        };
        assert!(project(&malformed, &BTreeMap::new()).is_err());

        let mutation = Gates {
            entries: vec![Gate {
                id: "manual-mutation".to_string(),
                kind: GateKind::Mutation,
                inputs: BTreeMap::new(),
                command: Some(command()),
                mutation: None,
                run_on: BTreeMap::from([(Surface::Manual, GateMembership::default())]),
            }],
            composition: None,
        };
        assert_eq!(
            run(
                Some(&mutation),
                &Invocation {
                    category: "gate",
                    surface: Some("manual".to_string()),
                    ..Invocation::default()
                },
                &MemoryTree::default(),
                None,
                &CapturingLauncher::default(),
                &NoMutationRunner,
            )
            .exit_code,
            // A mutation that could not be carried out is this tool failing to
            // run, not a verdict about the repository.
            2
        );
    }

    /// An input that cannot be resolved stops the run before any child starts.
    ///
    /// `resolve_inputs` is driven directly elsewhere; this drives it through
    /// `run`, which is where its refusal becomes an exit status and a code a
    /// caller can read.
    #[test]
    fn an_unresolvable_input_refuses_before_a_child_is_launched() {
        let gate = Gate {
            id: "message".to_string(),
            kind: GateKind::Check,
            inputs: BTreeMap::from([(
                "message".to_string(),
                GateInput {
                    kind: InputKind::CommitMessage,
                },
            )]),
            command: Some(command()),
            mutation: None,
            run_on: BTreeMap::from([(
                Surface::Manual,
                GateMembership {
                    reason: None,
                    bind: BTreeMap::from([(
                        "message".to_string(),
                        InputBinding {
                            source: InputSource::HookMessageFile,
                            range: None,
                            fallback: None,
                        },
                    )]),
                },
            )]),
        };
        let mut unreadable = MemoryTree::default();
        unreadable.write("message", "synthetic");
        unreadable.set_hook_message_file("message");
        unreadable.mark_unreadable("message");

        let launcher = CapturingLauncher::default();
        let outcome = run(
            Some(&Gates {
                entries: vec![gate],
                composition: None,
            }),
            &Invocation {
                category: "gate",
                surface: Some("manual".to_string()),
                message_file: Some("message".to_string()),
                format: Format::Json,
                ..Invocation::default()
            },
            &unreadable,
            None,
            &launcher,
            &NoMutationRunner,
        );

        assert_eq!(outcome.exit_code, 2);
        assert!(outcome.stdout.is_empty());
        assert!(
            outcome
                .stderr
                .contains("\"code\":\"rhino.file.unreadable\"")
        );
        assert!(outcome.stderr.contains("gate `message`"));
        assert!(launcher.calls.borrow().is_empty());
    }

    /// The two statuses that replaced exit `3`, and the result document.
    ///
    /// A shell reports `127` for a program it cannot find and `126` for one it
    /// finds and cannot execute. RHINO reports the same two, so a caller needs
    /// no RHINO-specific vocabulary -- and the distinction only survives if
    /// something drives both arms.
    #[test]
    fn a_gate_child_that_never_started_reports_what_a_shell_would() {
        let tree = MemoryTree::default();
        let invocation = Invocation {
            category: "gate",
            surface: Some("manual".to_string()),
            format: Format::Json,
            ..Invocation::default()
        };

        for (result, status, code) in [
            (LaunchResult::NotFound, 127, "rhino.gate.child-not-found"),
            (
                LaunchResult::NotExecutable,
                126,
                "rhino.gate.child-not-executable",
            ),
        ] {
            let outcome = run(
                Some(&manual_gate(Some(command()))),
                &invocation,
                &tree,
                None,
                &TerminalLauncher(result),
                &NoMutationRunner,
            );
            assert_eq!(outcome.exit_code, status);
            // Nothing on stdout: a caller parsing a result must never be handed
            // a document describing a run that did not happen.
            assert!(outcome.stdout.is_empty());
            assert!(outcome.stderr.contains(code));
            // And no progress prose ahead of the document in this format.
            assert!(outcome.stderr.starts_with('{'));
        }
    }

    #[test]
    fn a_gate_run_that_reached_a_verdict_renders_the_result_document() {
        let tree = MemoryTree::default();
        let invocation = Invocation {
            category: "gate",
            surface: Some("manual".to_string()),
            format: Format::Json,
            ..Invocation::default()
        };

        let clean = run(
            Some(&manual_gate(Some(command()))),
            &invocation,
            &tree,
            None,
            &TerminalLauncher(LaunchResult::Clean),
            &NoMutationRunner,
        );
        assert_eq!(clean.exit_code, 0);
        assert!(clean.stdout.contains("\"result\":\"clean\""));
        assert!(clean.stdout.contains("\"schemaVersion\":1"));
        assert!(clean.stdout.contains("\"scanned\":1"));

        let finding = run(
            Some(&manual_gate(Some(command()))),
            &invocation,
            &tree,
            None,
            &TerminalLauncher(LaunchResult::Finding),
            &NoMutationRunner,
        );
        assert_eq!(finding.exit_code, 1);
        assert!(finding.stdout.contains("\"result\":\"findings\""));
        assert!(finding.stdout.contains("\"findings\":1"));
    }

    #[test]
    fn grouped_gate_run_renders_check_and_mutation_terminal_outcomes() {
        let tree = MemoryTree::default();
        let invocation = Invocation {
            category: "gate",
            surface: Some("manual".to_string()),
            ..Invocation::default()
        };
        let failed = run(
            Some(&manual_gate(Some(command()))),
            &invocation,
            &tree,
            None,
            &TerminalLauncher(LaunchResult::Finding),
            &NoMutationRunner,
        );
        assert_eq!(failed.exit_code, 1);
        assert!(failed.stdout.contains("[gate] check failed"));
        assert!(failed.stderr.contains("reported a finding"));
        let unavailable = run(
            Some(&manual_gate(Some(command()))),
            &Invocation {
                format: Format::Json,
                ..invocation
            },
            &tree,
            None,
            &TerminalLauncher(LaunchResult::Error),
            &NoMutationRunner,
        );
        // A refused launch is a refusal: stdout stays empty rather than
        // carrying a result document describing a run that never happened.
        assert_eq!(unavailable.exit_code, 2);
        assert!(unavailable.stdout.is_empty());
        assert!(unavailable.stderr.contains("launcher refused"));
        assert!(
            unavailable
                .stderr
                .contains("\"code\":\"rhino.gate.child-refused\"")
        );

        let mutation = Gates {
            entries: vec![Gate {
                id: "format".to_string(),
                kind: GateKind::Mutation,
                inputs: BTreeMap::new(),
                command: Some(command()),
                mutation: Some(crate::v0_4::config::MutationContract {
                    local: crate::v0_4::config::LocalMutation::ApplyIndex,
                    ci: crate::v0_4::config::CiMutation::VerifyClean,
                }),
                run_on: BTreeMap::from([(Surface::PreCommit, GateMembership::default())]),
            }],
            composition: None,
        };
        let pre_commit = Invocation {
            category: "gate",
            surface: Some("pre-commit".to_string()),
            ..Invocation::default()
        };
        let changed = run(
            Some(&mutation),
            &pre_commit,
            &tree,
            None,
            &CapturingLauncher::default(),
            &TerminalMutations(MutationResult::Changed),
        );
        assert_eq!(changed.exit_code, 0);
        assert!(changed.stdout.contains("changed docs/changed.md"));
        assert!(
            changed
                .stdout
                .contains("preserved unstaged docs/unstaged.md")
        );
        let finding = run(
            Some(&mutation),
            &pre_commit,
            &tree,
            None,
            &CapturingLauncher::default(),
            &TerminalMutations(MutationResult::Finding),
        );
        assert_eq!(finding.exit_code, 1);
        let error = run(
            Some(&mutation),
            &pre_commit,
            &tree,
            None,
            &CapturingLauncher::default(),
            &TerminalMutations(MutationResult::Error),
        );
        assert_eq!(error.exit_code, 2);
        assert!(error.stdout.is_empty());
        assert!(error.stderr.contains("mutation refused"));
    }

    #[test]
    fn grouped_gate_typed_resolution_projection_and_push_parser_cover_declared_variants() {
        let mut tree = MemoryTree::default();
        tree.write("message.txt", "checked message");
        tree.set_hook_message_file("message.txt");
        tree.write("docs/a.md", "a");
        tree.set_indexed_files(["docs/a.md", "docs/a.md"]);
        tree.set_changed_files("aaaaaaa", "bbbbbbb", ["docs/b.md", "docs/a.md"]);
        tree.set_git_ref("refs/main", "ccccccc");
        tree.set_commit_messages("aaaaaaa", "bbbbbbb", "feat: checked message\n\nbody");

        let gate = Gate {
            id: "typed".to_string(),
            kind: GateKind::Check,
            inputs: BTreeMap::from([
                (
                    "files".to_string(),
                    GateInput {
                        kind: InputKind::Files,
                    },
                ),
                (
                    "message".to_string(),
                    GateInput {
                        kind: InputKind::CommitMessage,
                    },
                ),
                (
                    "range".to_string(),
                    GateInput {
                        kind: InputKind::CommitRange,
                    },
                ),
                (
                    "state".to_string(),
                    GateInput {
                        kind: InputKind::RepositoryState,
                    },
                ),
            ]),
            command: None,
            mutation: None,
            run_on: BTreeMap::new(),
        };
        let invocation = Invocation {
            category: "gate",
            message_file: Some("message.txt".to_string()),
            base: Some("aaaaaaa".to_string()),
            head: Some("bbbbbbb".to_string()),
            push_updates_stdin: true,
            ..Invocation::default()
        };

        let checkout = BTreeMap::from([(
            "files".to_string(),
            InputBinding {
                source: InputSource::Checkout,
                range: None,
                fallback: None,
            },
        )]);
        assert!(matches!(
            resolve_inputs(&gate, checkout.iter(), &invocation, &tree, None),
            Ok(Some(values)) if matches!(values["files"], ResolvedInput::Files(_))
        ));
        let state = BTreeMap::from([(
            "state".to_string(),
            InputBinding {
                source: InputSource::Checkout,
                range: None,
                fallback: None,
            },
        )]);
        assert!(matches!(
            resolve_inputs(&gate, state.iter(), &invocation, &tree, None),
            Ok(Some(values)) if matches!(values["state"], ResolvedInput::RepositoryState { .. })
        ));
        let message = BTreeMap::from([(
            "message".to_string(),
            InputBinding {
                source: InputSource::HookMessageFile,
                range: None,
                fallback: None,
            },
        )]);
        assert!(matches!(
            resolve_inputs(&gate, message.iter(), &invocation, &tree, None),
            Ok(Some(values)) if matches!(values["message"], ResolvedInput::CommitMessage(_))
        ));
        assert!(
            resolve_inputs(&gate, message.iter(), &Invocation::default(), &tree, None)
                .unwrap_err()
                .1
                .contains("--message-file")
        );
        let absent_message_file = Invocation {
            message_file: Some("missing-message.txt".to_string()),
            ..Invocation::default()
        };
        tree.set_hook_message_file("missing-message.txt");
        assert!(
            resolve_inputs(&gate, message.iter(), &absent_message_file, &tree, None)
                .unwrap_err()
                .1
                .contains("does not exist")
        );
        assert_eq!(
            resolve_inputs(&gate, message.iter(), &absent_message_file, &tree, None)
                .unwrap_err()
                .0,
            ErrorCode::FileMissing
        );

        let explicit_message = BTreeMap::from([(
            "message".to_string(),
            InputBinding {
                source: InputSource::ExplicitRange,
                range: Some(crate::v0_4::config::RangeSelector::Explicit),
                fallback: None,
            },
        )]);
        assert!(matches!(
            resolve_inputs(&gate, explicit_message.iter(), &invocation, &tree, None),
            Ok(Some(values))
                if matches!(
                    values["message"],
                    ResolvedInput::CommitMessage(ref message)
                        if message == "feat: checked message\n\nbody"
                )
        ));
        assert!(
            resolve_inputs(
                &gate,
                explicit_message.iter(),
                &Invocation::default(),
                &tree,
                None
            )
            .unwrap_err()
            .1
            .contains("both `--base`")
        );
        let absent_range_messages = Invocation {
            base: Some("ccccccc".to_string()),
            head: Some("ddddddd".to_string()),
            ..Invocation::default()
        };
        assert!(
            resolve_inputs(
                &gate,
                explicit_message.iter(),
                &absent_range_messages,
                &tree,
                None
            )
            .unwrap_err()
            .1
            .contains("cannot read commit messages")
        );

        let explicit = BTreeMap::from([(
            "range".to_string(),
            InputBinding {
                source: InputSource::ExplicitRange,
                range: Some(crate::v0_4::config::RangeSelector::Explicit),
                fallback: None,
            },
        )]);
        assert!(matches!(
            resolve_inputs(&gate, explicit.iter(), &invocation, &tree, None),
            Ok(Some(values)) if matches!(values["range"], ResolvedInput::CommitRange { .. })
        ));
        assert!(
            resolve_inputs(&gate, explicit.iter(), &Invocation::default(), &tree, None)
                .unwrap_err()
                .1
                .contains("both `--base`")
        );
        let invalid_commit = Invocation {
            base: Some("not-a-commit".to_string()),
            head: Some("bbbbbbb".to_string()),
            ..Invocation::default()
        };
        assert!(
            resolve_inputs(&gate, explicit.iter(), &invalid_commit, &tree, None)
                .unwrap_err()
                .1
                .contains("hexadecimal")
        );
        assert!(
            resolve_inputs(&gate, explicit_message.iter(), &invalid_commit, &tree, None)
                .unwrap_err()
                .1
                .contains("hexadecimal")
        );

        let push = BTreeMap::from([(
            "range".to_string(),
            InputBinding {
                source: InputSource::PushUpdates,
                range: None,
                fallback: Some("refs/main".to_string()),
            },
        )]);
        assert!(
            resolve_inputs(&gate, push.iter(), &Invocation::default(), &tree, None)
                .unwrap_err()
                .1
                .contains("--push-updates-stdin")
        );
        assert!(matches!(
            resolve_inputs(
                &gate,
                push.iter(),
                &invocation,
                &tree,
                Some("refs/new aaaaaaa refs/new 0000000\n")
            ),
            Ok(Some(values)) if matches!(values["range"], ResolvedInput::CommitRange { .. })
        ));
        assert!(matches!(
            resolve_inputs(
                &gate,
                push.iter(),
                &invocation,
                &tree,
                Some("refs/deleted 0000000 refs/deleted aaaaaaa\n")
            ),
            Ok(None)
        ));
        let indexed = BTreeMap::from([(
            "files".to_string(),
            InputBinding {
                source: InputSource::GitIndex,
                range: None,
                fallback: None,
            },
        )]);
        assert!(matches!(
            resolve_inputs(&gate, indexed.iter(), &invocation, &tree, None),
            Ok(Some(values)) if matches!(values["files"], ResolvedInput::Files(_))
        ));
        let explicit_files = BTreeMap::from([(
            "files".to_string(),
            InputBinding {
                source: InputSource::ExplicitRange,
                range: Some(crate::v0_4::config::RangeSelector::Explicit),
                fallback: None,
            },
        )]);
        assert!(matches!(
            resolve_inputs(&gate, explicit_files.iter(), &invocation, &tree, None),
            Ok(Some(values))
                if matches!(
                    values["files"],
                    ResolvedInput::Files(ref paths)
                        if paths == &vec!["docs/a.md".to_string(), "docs/b.md".to_string()]
                )
        ));
        assert!(
            resolve_inputs(
                &gate,
                explicit_files.iter(),
                &Invocation::default(),
                &tree,
                None
            )
            .unwrap_err()
            .1
            .contains("both `--base`")
        );
        assert!(
            resolve_inputs(&gate, explicit_files.iter(), &invalid_commit, &tree, None)
                .unwrap_err()
                .1
                .contains("hexadecimal")
        );
        for (bound, invocation, code) in [
            (
                &explicit_files,
                &Invocation::default(),
                ErrorCode::ArgsIncomplete,
            ),
            (
                &explicit_files,
                &invalid_commit,
                ErrorCode::ArgsUnrecognized,
            ),
            (&push, &Invocation::default(), ErrorCode::ArgsIncomplete),
            (
                &explicit_message,
                &absent_range_messages,
                ErrorCode::RepositoryUnusable,
            ),
        ] {
            assert_eq!(
                resolve_inputs(&gate, bound.iter(), invocation, &tree, None)
                    .unwrap_err()
                    .0,
                code
            );
        }
        let unavailable_file_range = Invocation {
            base: Some("ccccccc".to_string()),
            head: Some("ddddddd".to_string()),
            ..Invocation::default()
        };
        assert!(
            resolve_inputs(
                &gate,
                explicit_files.iter(),
                &unavailable_file_range,
                &tree,
                None
            )
            .unwrap_err()
            .1
            .contains("cannot read changed files")
        );
        assert_eq!(
            selected_paths(
                &BTreeMap::from([(
                    "files".to_string(),
                    ResolvedInput::Files(vec![
                        "b.md".to_string(),
                        "a.md".to_string(),
                        "a.md".to_string()
                    ]),
                )]),
                indexed.iter(),
            ),
            vec!["a.md".to_string(), "b.md".to_string()]
        );
        assert_eq!(
            selected_paths(
                &BTreeMap::from([(
                    "files".to_string(),
                    ResolvedInput::Files(vec!["docs/b.md".to_string(), "docs/a.md".to_string()]),
                )]),
                explicit_files.iter(),
            ),
            vec!["docs/a.md".to_string(), "docs/b.md".to_string()]
        );

        let inputs = BTreeMap::from([
            (
                "files".to_string(),
                ResolvedInput::Files(vec!["a.md".to_string(), "b.md".to_string()]),
            ),
            (
                "message".to_string(),
                ResolvedInput::CommitMessage("message".to_string()),
            ),
            (
                "range".to_string(),
                ResolvedInput::CommitRange {
                    base: "aaaaaaa".to_string(),
                    head: "bbbbbbb".to_string(),
                },
            ),
            (
                "state".to_string(),
                ResolvedInput::RepositoryState {
                    root: "/synthetic".to_string(),
                },
            ),
        ]);
        let command = crate::v0_4::config::GateCommand {
            executable: "checker".to_string(),
            args: vec![
                crate::v0_4::config::Argument {
                    literal: Some("--check".to_string()),
                    input: None,
                    expand: None,
                },
                crate::v0_4::config::Argument {
                    literal: None,
                    input: Some("files.paths".to_string()),
                    expand: Some(ArgumentExpansion::Repeat),
                },
                crate::v0_4::config::Argument {
                    literal: None,
                    input: Some("message.text".to_string()),
                    expand: Some(ArgumentExpansion::Single),
                },
                crate::v0_4::config::Argument {
                    literal: None,
                    input: Some("range.base".to_string()),
                    expand: Some(ArgumentExpansion::Single),
                },
                crate::v0_4::config::Argument {
                    literal: None,
                    input: Some("range.head".to_string()),
                    expand: Some(ArgumentExpansion::Single),
                },
                crate::v0_4::config::Argument {
                    literal: None,
                    input: Some("state.root".to_string()),
                    expand: Some(ArgumentExpansion::Single),
                },
            ],
            environment: BTreeMap::from([(
                "CHECK_MESSAGE".to_string(),
                crate::v0_4::config::EnvironmentProjection {
                    input: "message.text".to_string(),
                },
            )]),
        };
        let (arguments, environment) =
            project(&command, &inputs).expect("the typed vectors project");
        assert_eq!(
            arguments,
            vec![
                "checker",
                "--check",
                "a.md",
                "b.md",
                "message",
                "aaaaaaa",
                "bbbbbbb",
                "/synthetic"
            ]
        );
        assert_eq!(environment["CHECK_MESSAGE"], "message");
        assert!(
            project(
                &crate::v0_4::config::GateCommand {
                    executable: "checker".to_string(),
                    args: vec![crate::v0_4::config::Argument {
                        literal: None,
                        input: Some("files.paths".to_string()),
                        expand: Some(ArgumentExpansion::Single)
                    }],
                    environment: BTreeMap::new(),
                },
                &inputs,
            )
            .unwrap_err()
            .contains("not one value")
        );
        assert!(
            project(
                &crate::v0_4::config::GateCommand {
                    executable: "checker".to_string(),
                    args: Vec::new(),
                    environment: BTreeMap::from([(
                        "FILES".to_string(),
                        crate::v0_4::config::EnvironmentProjection {
                            input: "files.paths".to_string()
                        },
                    )]),
                },
                &inputs,
            )
            .unwrap_err()
            .contains("cannot project a sequence")
        );
        assert!(
            referenced_values(&inputs, "missing")
                .unwrap_err()
                .contains("not an input field")
        );
        assert!(
            referenced_values(&inputs, "unknown.paths")
                .unwrap_err()
                .contains("not resolved")
        );
        assert!(
            referenced_values(&inputs, "files.root")
                .unwrap_err()
                .contains("not available")
        );

        for (record, fallback, expected) in [
            ("", None, "no pre-push update"),
            (
                "a aaaaaaa b bbbbbbb\na ccccccc b ddddddd",
                None,
                "multiple pre-push",
            ),
            ("only two", None, "invalid pre-push"),
            ("a nope b bbbbbbb", None, "local commit"),
            ("a aaaaaaa b 0000000", None, "without a declared fallback"),
            ("a aaaaaaa b nope", None, "remote commit"),
        ] {
            match parse_push_range(record, fallback) {
                Ok(_) => panic!("the malformed push record is refused"),
                Err(reason) => assert!(reason.contains(expected)),
            }
        }
        assert!(matches!(
            parse_push_range("a aaaaaaa b bbbbbbb", None),
            Ok(PushRange::Range { .. })
        ));
        assert!(is_commit("abcdef1"));
        assert!(!is_commit("abcdef"));
        assert!(is_zero("000"));
        assert!(!is_zero("0a0"));
    }
}
