//! Safe environment and toolchain operation boundary.

use super::config::{
    Environment, EnvironmentLanguage, Toolchain, ToolchainOutputParser, Toolchains,
};
use crate::Outcome;
use crate::cli::Format;
use crate::errors::ErrorCode;
use crate::runtime::{
    EnvironmentFile, EnvironmentRestoreFile, EnvironmentRestoreTransaction, EnvironmentStore,
    EnvironmentTransaction, ToolchainLaunch, ToolchainRunner, Tree, TreeError,
};
use globset::{Glob, GlobSetBuilder};
use std::collections::BTreeSet;

/// Validate an explicitly declared environment policy without opening any
/// environment file. Detector-specific reads are added only when a repository
/// declares their roots and keys.
pub(crate) fn validate(
    environment: Option<&Environment>,
    tree: &dyn Tree,
    format: Format,
) -> Outcome {
    let Some(environment) = environment else {
        return Outcome::refusal(
            format,
            ErrorCode::ConfigUndeclared,
            "environment section is not declared",
        );
    };
    if let Err(reason) = validate_detector_policy(environment) {
        return Outcome::refusal(
            format,
            ErrorCode::ConfigUnusable,
            format!("environment detector policy: {reason}"),
        );
    }

    let mut inspected = 0usize;
    let mut findings = BTreeSet::new();
    if let Some(staged) = &environment.staged {
        if staged.allowed.iter().any(|path| !is_exact_path(path)) {
            return Outcome::refusal(
                format,
                ErrorCode::PathEscapesRoot,
                "environment staging guard allows only exact repository-relative paths",
            );
        }
        let indexed = match tree.indexed_files() {
            Ok(indexed) => indexed,
            Err(reason) => {
                return Outcome::refusal(
                    format,
                    ErrorCode::RepositoryUnusable,
                    format!("environment staging guard refused: {reason}"),
                );
            }
        };
        inspected += indexed.len();
        // Both lists in one arm: an unusable pattern is the same refusal
        // whichever list it came from, and two identical arms only meant one
        // of them was never driven.
        let (forbidden, allowed) = match (patterns(&staged.forbidden), patterns(&staged.allowed)) {
            (Ok(forbidden), Ok(allowed)) => (forbidden, allowed),
            (Err(reason), _) | (_, Err(reason)) => {
                return Outcome::refusal(
                    format,
                    ErrorCode::ConfigUnusable,
                    format!("environment staging guard: {reason}"),
                );
            }
        };
        for path in indexed
            .into_iter()
            .filter(|path| forbidden.is_match(path) && !allowed.is_match(path))
        {
            findings.insert(EnvironmentFinding::new(
                "staged-environment-file",
                path,
                None,
            ));
        }
    }

    let declared: BTreeSet<(String, String)> = environment
        .contracts
        .iter()
        .flat_map(|contract| {
            contract
                .keys
                .iter()
                .map(|key| (contract.path.clone(), key.clone()))
        })
        .collect();
    let declared_keys: BTreeSet<String> = declared.iter().map(|(_, key)| key.clone()).collect();
    let mut read_keys = BTreeSet::new();
    for detector in &environment.detectors {
        for path in &detector.paths {
            inspected += 1;
            let source = match tree.read(path) {
                Ok(source) => source,
                Err(_) => {
                    findings.insert(EnvironmentFinding::new(
                        "declared-source-unread",
                        path.clone(),
                        None,
                    ));
                    continue;
                }
            };
            let (keys, dynamic) = detector_accesses(detector.language, &source);
            for key in keys {
                read_keys.insert(key.clone());
                if !declared_keys.contains(&key) {
                    findings.insert(EnvironmentFinding::new(
                        "read-but-undeclared",
                        path.clone(),
                        Some(key),
                    ));
                }
            }
            if dynamic {
                findings.insert(EnvironmentFinding::new(
                    "unsupported-dynamic-access",
                    path.clone(),
                    Some("<dynamic>".to_string()),
                ));
            }
        }
    }
    for (path, key) in declared {
        if !read_keys.contains(&key) {
            findings.insert(EnvironmentFinding::new(
                "declared-but-unread",
                path,
                Some(key),
            ));
        }
    }
    findings.retain(|finding| {
        finding.rule == "staged-environment-file" || !allowlisted(environment, finding)
    });
    render_environment_findings(findings, inspected, format)
}

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
struct EnvironmentFinding {
    rule: String,
    path: String,
    key: Option<String>,
}

impl EnvironmentFinding {
    fn new(rule: impl Into<String>, path: impl Into<String>, key: Option<String>) -> Self {
        Self {
            rule: rule.into(),
            path: path.into(),
            key,
        }
    }
}

fn render_environment_findings(
    findings: BTreeSet<EnvironmentFinding>,
    inspected: usize,
    format: Format,
) -> Outcome {
    if findings.is_empty() {
        return clean_validation(inspected, format);
    }
    match format {
        Format::Text => Outcome {
            exit_code: 1,
            stdout: String::new(),
            stderr: findings
                .iter()
                .map(|finding| match &finding.key {
                    None => format!(
                        "[environment-validate] {}: {}\n",
                        finding.path, finding.rule
                    ),
                    Some(key) => format!(
                        "[environment-validate] {}: {} ({key})\n",
                        finding.path, finding.rule
                    ),
                })
                .collect(),
        },
        Format::Json => Outcome {
            exit_code: 1,
            stdout: format!(
                "{}\n",
                serde_json::json!({
                    "schemaVersion": 1,
                    "command": "environment-validate",
                    "exitCode": 1,
                    "findings": findings.iter().map(|finding| serde_json::json!({
                        "rule": finding.rule,
                        "path": finding.path,
                        "key": finding.key,
                    })).collect::<Vec<_>>(),
                })
            ),
            stderr: String::new(),
        },
    }
}

fn validate_detector_policy(environment: &Environment) -> Result<(), String> {
    let mut contract_pairs = BTreeSet::new();
    for contract in &environment.contracts {
        if !is_exact_path(&contract.path) {
            return Err("contract paths must be exact repository-relative files".to_string());
        }
        for key in &contract.keys {
            if !is_environment_key(key)
                || !contract_pairs.insert((contract.path.clone(), key.clone()))
            {
                return Err("contract keys must be unique valid environment names".to_string());
            }
        }
    }
    let mut detector_sources = BTreeSet::new();
    for detector in &environment.detectors {
        if detector.paths.is_empty() {
            return Err("each detector requires one or more exact source paths".to_string());
        }
        for path in &detector.paths {
            if !is_exact_path(path) || !detector_sources.insert((detector.language, path.clone())) {
                return Err(
                    "detector source paths must be exact and unique per language".to_string(),
                );
            }
        }
    }
    let mut rows = BTreeSet::new();
    for allowlist in &environment.allowlists {
        if allowlist.rule.trim().is_empty()
            || !is_exact_path(&allowlist.path)
            || !is_environment_key(&allowlist.key)
            || allowlist.reason.trim().is_empty()
            || allowlist.owner.trim().is_empty()
            || !is_iso_date(&allowlist.expiry)
            || !rows.insert((
                allowlist.rule.clone(),
                allowlist.path.clone(),
                allowlist.key.clone(),
            ))
        {
            return Err(
                "allowlists require unique exact rule, path, key, reason, owner, and ISO expiry"
                    .to_string(),
            );
        }
    }
    Ok(())
}

fn allowlisted(environment: &Environment, finding: &EnvironmentFinding) -> bool {
    let Some(key) = &finding.key else {
        return false;
    };
    environment.allowlists.iter().any(|allowlist| {
        allowlist.rule == finding.rule && allowlist.path == finding.path && allowlist.key == *key
    })
}

fn is_environment_key(key: &str) -> bool {
    let mut chars = key.bytes();
    matches!(chars.next(), Some(b'A'..=b'Z' | b'a'..=b'z' | b'_'))
        && chars.all(|byte| {
            byte.is_ascii_uppercase()
                || byte.is_ascii_lowercase()
                || byte.is_ascii_digit()
                || byte == b'_'
        })
}

fn is_iso_date(value: &str) -> bool {
    value.len() == 10
        && value.as_bytes()[4] == b'-'
        && value.as_bytes()[7] == b'-'
        && value
            .bytes()
            .enumerate()
            .all(|(index, byte)| matches!(index, 4 | 7) || byte.is_ascii_digit())
}

fn detector_accesses(language: EnvironmentLanguage, source: &str) -> (BTreeSet<String>, bool) {
    match language {
        EnvironmentLanguage::Rust => quoted_calls(source, &["std::env::var(", "std::env::var_os("]),
        EnvironmentLanguage::TypeScript => typescript_accesses(source),
        EnvironmentLanguage::FSharp => {
            quoted_calls(source, &["Environment.GetEnvironmentVariable("])
        }
        EnvironmentLanguage::Go => {
            quoted_calls(source, &["os.Getenv(", "os.LookupEnv(", "os.LookupEnv,"])
        }
        EnvironmentLanguage::Terraform => quoted_calls(source, &["get_env(", "env("]),
        EnvironmentLanguage::Ansible => ansible_accesses(source),
    }
}

fn quoted_calls(source: &str, markers: &[&str]) -> (BTreeSet<String>, bool) {
    let mut keys = BTreeSet::new();
    let mut dynamic = false;
    for marker in markers {
        let mut remainder = source;
        while let Some(index) = remainder.find(marker) {
            let after = &remainder[index + marker.len()..];
            let trimmed = after.trim_start();
            if let Some((key, rest)) = quoted_value(trimmed) {
                keys.insert(key);
                remainder = rest;
            } else {
                dynamic = true;
                remainder = trimmed.get(1..).unwrap_or_default();
            }
        }
    }
    (keys, dynamic)
}

fn quoted_value(value: &str) -> Option<(String, &str)> {
    let quote = value.as_bytes().first().copied()?;
    if quote != b'\'' && quote != b'"' {
        return None;
    }
    let rest = &value[1..];
    let end = rest.find(char::from(quote))?;
    Some((rest[..end].to_string(), &rest[end + 1..]))
}

fn typescript_accesses(source: &str) -> (BTreeSet<String>, bool) {
    let (mut keys, mut dynamic) = quoted_calls(source, &["process.env["]);
    let mut remainder = source;
    while let Some(index) = remainder.find("process.env.") {
        let after = &remainder[index + "process.env.".len()..];
        let key: String = after
            .chars()
            .take_while(|character| character.is_ascii_alphanumeric() || *character == '_')
            .collect();
        if key.is_empty() {
            dynamic = true;
            remainder = after.get(1..).unwrap_or_default();
        } else {
            keys.insert(key);
            remainder = &after[key_len(after)..];
        }
    }
    keys.extend(source.lines().filter_map(|line| {
        let key = line.trim_start().split_once(':')?.0.trim_end();
        (key.as_bytes().first().is_some_and(u8::is_ascii_uppercase)
            && key
                .bytes()
                .all(|byte| byte.is_ascii_uppercase() || byte.is_ascii_digit() || byte == b'_'))
        .then(|| key.to_string())
    }));
    (keys, dynamic)
}

fn key_len(value: &str) -> usize {
    value
        .chars()
        .take_while(|character| character.is_ascii_alphanumeric() || *character == '_')
        .map(char::len_utf8)
        .sum()
}

fn ansible_accesses(source: &str) -> (BTreeSet<String>, bool) {
    let mut keys = BTreeSet::new();
    let mut dynamic = false;
    let mut remainder = source;
    while let Some(index) = remainder.find("lookup(") {
        let after = remainder[index + "lookup(".len()..].trim_start();
        let Some((lookup, after_lookup)) = quoted_value(after) else {
            dynamic = true;
            remainder = after.get(1..).unwrap_or_default();
            continue;
        };
        if lookup != "env" {
            remainder = after_lookup;
            continue;
        }
        let Some(after_comma) = after_lookup.trim_start().strip_prefix(',') else {
            dynamic = true;
            remainder = after_lookup;
            continue;
        };
        if let Some((key, rest)) = quoted_value(after_comma.trim_start()) {
            keys.insert(key);
            remainder = rest;
        } else {
            dynamic = true;
            remainder = after_comma;
        }
    }
    (keys, dynamic)
}

fn clean_validation(inspected: usize, format: Format) -> Outcome {
    match format {
        Format::Text => Outcome::clean(format!(
            "[environment-validate] checked {inspected} declared files, no findings\n"
        )),
        Format::Json => Outcome::clean(format!(
            "{}\n",
            serde_json::json!({
                "schemaVersion": 1,
                "command": "environment-validate",
                "exitCode": 0,
                "inspected": inspected,
            })
        )),
    }
}

fn patterns(patterns: &[String]) -> Result<globset::GlobSet, String> {
    let mut builder = GlobSetBuilder::new();
    for pattern in patterns {
        builder.add(
            Glob::new(pattern)
                .map_err(|error| format!("invalid path pattern `{pattern}`: {error}"))?,
        );
    }
    builder.build().map_err(|error| error.to_string())
}

/// Reject an empty setup policy before an initialization write boundary is
/// introduced. A later transaction receives only this fully validated list.
pub(crate) fn init(
    environment: Option<&Environment>,
    tree: &dyn Tree,
    root: &str,
    apply: bool,
    store: &dyn EnvironmentStore,
    format: Format,
) -> Outcome {
    let Some(environment) = environment else {
        return Outcome::refusal(
            format,
            ErrorCode::ConfigUndeclared,
            "environment section is not declared",
        );
    };
    if environment.examples.is_empty() {
        return Outcome::refusal(
            format,
            ErrorCode::ConfigUndeclared,
            "environment policy declares no example targets",
        );
    }
    let mut targets = BTreeSet::new();
    let mut files = Vec::new();
    for example in &environment.examples {
        if !is_relative_file(&example.source) || !is_relative_file(&example.target) {
            return Outcome::refusal(
                format,
                ErrorCode::PathEscapesRoot,
                "environment example leaves the repository root",
            );
        }
        if example.source == example.target {
            return Outcome::refusal(
                format,
                ErrorCode::ConfigUnusable,
                "environment example source and target are the same",
            );
        }
        if !targets.insert(example.target.clone()) {
            return Outcome::refusal(
                format,
                ErrorCode::ConfigUnusable,
                format!(
                    "environment target `{}` is declared more than once",
                    example.target
                ),
            );
        }
        if tree.exists(&example.target) {
            return Outcome::refusal(
                format,
                ErrorCode::FileExists,
                format!("environment target `{}` already exists", example.target),
            );
        }
        let contents = match tree.read_no_follow(&example.source) {
            Ok(contents) => contents,
            Err(TreeError::NotFound) => {
                return Outcome::refusal(
                    format,
                    ErrorCode::FileMissing,
                    format!("environment example `{}` is missing", example.source),
                );
            }
            Err(TreeError::Unreadable(_) | TreeError::NotText) => {
                return Outcome::refusal(
                    format,
                    ErrorCode::FileUnreadable,
                    format!("environment example `{}` is unreadable", example.source),
                );
            }
        };
        files.push(EnvironmentFile {
            path: example.target.clone(),
            contents,
        });
    }
    if !apply {
        return match format {
            Format::Text => Outcome::clean(format!(
                "[environment-init] planned {} declared targets; pass --apply to write\n",
                files.len()
            )),
            Format::Json => Outcome::clean(format!(
                "{}\n",
                serde_json::json!({
                    "schemaVersion": 1,
                    "command": "environment-init",
                    "status": "planned",
                    "count": files.len(),
                    "targets": serde_json::Value::from_iter(files.iter().map(|file| file.path.as_str())),
                })
            )),
        };
    }
    if let Err(error) = store.create(root, &EnvironmentTransaction { files }) {
        return Outcome::refusal(
            format,
            ErrorCode::FileUnwritable,
            format!("environment initialization refused: {}", error.0),
        );
    }
    match format {
        Format::Text => Outcome::clean("[environment-init] created declared targets\n"),
        Format::Json => Outcome::clean(
            "{\"schemaVersion\":1,\"command\":\"environment-init\",\"status\":\"completed\"}\n",
        ),
    }
}

fn is_relative_file(path: &str) -> bool {
    !path.trim().is_empty()
        && !path.starts_with('/')
        && path.split('/').all(|segment| {
            !segment.is_empty() && segment != "." && segment != ".." && !segment.contains('\\')
        })
}

fn is_exact_path(path: &str) -> bool {
    is_relative_file(path) && !path.contains(['*', '?', '[', ']', '{', '}'])
}

/// Plan a backup before a filesystem write boundary exists.
///
/// An empty, explicitly declared environment group means that no repository
/// files are eligible. That is a valid zero-file plan; omission is different
/// and refuses because RHINO supplies no environment policy by default.
pub(crate) fn backup(
    environment: Option<&Environment>,
    tree: &dyn Tree,
    root: &str,
    destination: Option<&str>,
    store: &dyn EnvironmentStore,
    format: Format,
) -> Outcome {
    let Some(environment) = environment else {
        return Outcome::refusal(
            format,
            ErrorCode::ConfigUndeclared,
            "environment section is not declared",
        );
    };
    let Some(destination) = destination else {
        return Outcome::refusal(
            format,
            ErrorCode::ArgsIncomplete,
            "environment backup requires --dir",
        );
    };
    if leaves_root(destination) {
        return Outcome::refusal(
            format,
            ErrorCode::PathEscapesRoot,
            "backup destination leaves the repository root",
        );
    }
    let mut files = Vec::new();
    for example in &environment.examples {
        if !is_relative_file(&example.target) {
            return Outcome::refusal(
                format,
                ErrorCode::PathEscapesRoot,
                "environment example leaves the repository root",
            );
        }
        let contents = match tree.read(&example.target) {
            Ok(contents) => contents,
            Err(TreeError::NotFound) => {
                return Outcome::refusal(
                    format,
                    ErrorCode::FileMissing,
                    format!("environment target `{}` is missing", example.target),
                );
            }
            Err(TreeError::Unreadable(_) | TreeError::NotText) => {
                return Outcome::refusal(
                    format,
                    ErrorCode::FileUnreadable,
                    format!("environment target `{}` is unreadable", example.target),
                );
            }
        };
        files.push(EnvironmentFile {
            path: backup_path(destination, &example.target),
            contents,
        });
    }
    if !files.is_empty()
        && let Err(error) = store.create(root, &EnvironmentTransaction { files })
    {
        return Outcome::refusal(
            format,
            ErrorCode::FileUnwritable,
            format!("environment backup refused: {}", error.0),
        );
    }
    match format {
        Format::Text => Outcome::clean(format!(
            "[environment-backup] planned declared files into {destination}\n"
        )),
        Format::Json => Outcome::clean(format!(
            "{}\n",
            serde_json::json!({
                "schemaVersion": 1,
                "command": "environment-backup",
                "status": "planned",
                "destination": destination,
            })
        )),
    }
}

fn backup_path(destination: &str, target: &str) -> String {
    match destination.trim_matches('/') {
        "" | "." => target.to_string(),
        destination => format!("{destination}/{target}"),
    }
}

/// Restore only the declared local targets from an explicit backup directory.
/// A conflicting target is never overwritten unless the caller names `--force`;
/// the disk store then preserves the old bytes until the replacement is durable.
pub(crate) fn restore(
    environment: Option<&Environment>,
    tree: &dyn Tree,
    root: &str,
    directory: Option<&str>,
    force: bool,
    store: &dyn EnvironmentStore,
    format: Format,
) -> Outcome {
    let Some(environment) = environment else {
        return Outcome::refusal(
            format,
            ErrorCode::ConfigUndeclared,
            "environment section is not declared",
        );
    };
    let Some(directory) = directory else {
        return Outcome::refusal(
            format,
            ErrorCode::ArgsIncomplete,
            "environment restore requires --dir",
        );
    };
    if leaves_root(directory) {
        return Outcome::refusal(
            format,
            ErrorCode::PathEscapesRoot,
            "backup destination leaves the repository root",
        );
    }
    let mut files = Vec::new();
    for example in &environment.examples {
        if !is_relative_file(&example.target) {
            return Outcome::refusal(
                format,
                ErrorCode::PathEscapesRoot,
                "environment example leaves the repository root",
            );
        }
        let source = backup_path(directory, &example.target);
        let contents = match tree.read(&source) {
            Ok(contents) => contents,
            Err(TreeError::NotFound) => {
                return Outcome::refusal(
                    format,
                    ErrorCode::FileMissing,
                    format!("environment backup `{source}` is missing"),
                );
            }
            Err(TreeError::Unreadable(_) | TreeError::NotText) => {
                return Outcome::refusal(
                    format,
                    ErrorCode::FileUnreadable,
                    format!("environment backup `{source}` is unreadable"),
                );
            }
        };
        if tree.exists(&example.target) && !force {
            return Outcome::refusal(
                format,
                ErrorCode::FileExists,
                format!(
                    "environment target `{}` already exists; use --force",
                    example.target
                ),
            );
        }
        files.push(EnvironmentRestoreFile {
            path: example.target.clone(),
            contents,
            replace: force,
        });
    }
    if let Err(error) = store.restore(root, &EnvironmentRestoreTransaction { files }) {
        return Outcome::refusal(
            format,
            ErrorCode::FileUnwritable,
            format!("environment restore refused: {}", error.0),
        );
    }
    match format {
        Format::Text => Outcome::clean("[environment-restore] restored declared targets\n"),
        Format::Json => Outcome::clean(
            "{\"schemaVersion\":1,\"command\":\"environment-restore\",\"status\":\"completed\"}\n",
        ),
    }
}

/// Probe declared toolchains without installing anything or reporting command
/// output. A required unavailable or wrong-version tool is a finding; optional
/// tools remain an explicit, clean absence.
pub(crate) fn validate_toolchains(
    toolchains: Option<&Toolchains>,
    runner: &dyn ToolchainRunner,
    format: Format,
) -> Outcome {
    let Some(toolchains) = toolchains else {
        return Outcome::refusal(
            format,
            ErrorCode::ConfigUndeclared,
            "toolchains section is not declared",
        );
    };
    if let Err(reason) = validate_toolchain_policy(toolchains) {
        return Outcome::refusal(
            format,
            ErrorCode::ConfigUnusable,
            format!("toolchain policy: {reason}"),
        );
    }
    let mut findings = Vec::new();
    for toolchain in &toolchains.entries {
        let result = runner.run(ToolchainLaunch {
            executable: &toolchain.executable,
            arguments: &toolchain.probe,
            timeout_seconds: toolchain.timeout_seconds,
        });
        let Ok(result) = result else {
            if toolchain.required {
                findings.push((toolchain.id.as_str(), "unavailable"));
            }
            continue;
        };
        if result.code != 0 {
            if toolchain.required {
                findings.push((toolchain.id.as_str(), "probe-failed"));
            }
            continue;
        }
        if !toolchain_version_matches(toolchain, &result.stdout) && toolchain.required {
            findings.push((toolchain.id.as_str(), "version-mismatch"));
        }
    }
    if findings.is_empty() {
        return match format {
            Format::Text => Outcome::clean(format!(
                "[toolchain-validate] checked {} declared toolchains, no findings\n",
                toolchains.entries.len()
            )),
            Format::Json => Outcome::clean(format!(
                "{}\n",
                serde_json::json!({
                    "schemaVersion": 1,
                    "command": "toolchain-validate",
                    "exitCode": 0,
                    "inspected": toolchains.entries.len(),
                })
            )),
        };
    }
    match format {
        Format::Text => Outcome {
            exit_code: 1,
            stdout: String::new(),
            stderr: findings
                .iter()
                .map(|(id, rule)| format!("[toolchain-validate] {id}: {rule}\n"))
                .collect(),
        },
        Format::Json => Outcome {
            exit_code: 1,
            stdout: format!(
                "{}\n",
                serde_json::json!({
                    "schemaVersion": 1,
                    "command": "toolchain-validate",
                    "exitCode": 1,
                    "findings": findings.iter().map(|(id, rule)| serde_json::json!({
                        "toolchain": id,
                        "rule": rule,
                    })).collect::<Vec<_>>(),
                })
            ),
            stderr: String::new(),
        },
    }
}

fn validate_toolchain_policy(toolchains: &Toolchains) -> Result<(), String> {
    let mut identifiers = BTreeSet::new();
    for toolchain in &toolchains.entries {
        if toolchain.id.trim().is_empty() || !identifiers.insert(toolchain.id.clone()) {
            return Err("identifiers must be unique and non-empty".to_string());
        }
        if !valid_executable(&toolchain.executable) {
            return Err(format!("`{}` has an invalid executable", toolchain.id));
        }
        if toolchain.probe.is_empty() {
            return Err(format!("`{}` declares no probe argv", toolchain.id));
        }
        if toolchain.version.is_some() != toolchain.parser.is_some() {
            return Err(format!(
                "`{}` must declare both parser and version, or neither",
                toolchain.id
            ));
        }
        if toolchain
            .version
            .as_ref()
            .is_some_and(|version| version.trim().is_empty())
        {
            return Err(format!(
                "`{}` has an empty version constraint",
                toolchain.id
            ));
        }
        if toolchain.timeout_seconds == Some(0) {
            return Err(format!("`{}` has a zero timeout", toolchain.id));
        }
        let mut platforms = BTreeSet::new();
        for provision in &toolchain.provision {
            if !is_platform(&provision.platform)
                || !valid_executable(&provision.executable)
                || !platforms.insert(provision.platform.clone())
            {
                return Err(format!(
                    "`{}` has a duplicate or invalid provision declaration",
                    toolchain.id
                ));
            }
        }
    }
    Ok(())
}

fn valid_executable(executable: &str) -> bool {
    !executable.trim().is_empty()
        && !executable.starts_with('-')
        && !executable.contains(['\0', '\n', '\r'])
}

fn is_platform(platform: &str) -> bool {
    !platform.is_empty()
        && platform
            .bytes()
            .all(|byte| byte.is_ascii_lowercase() || byte.is_ascii_digit() || byte == b'-')
}

fn toolchain_version_matches(toolchain: &Toolchain, stdout: &str) -> bool {
    let Some(expected) = &toolchain.version else {
        return true;
    };
    let Some(parser) = toolchain.parser else {
        return false;
    };
    let found = match parser {
        ToolchainOutputParser::FirstLine => stdout.lines().next().unwrap_or_default().trim(),
        ToolchainOutputParser::FirstToken => stdout.split_whitespace().next().unwrap_or_default(),
    };
    found == expected
}

/// Plan each supported provision vector by default. `--apply` is the explicit
/// confirmation that starts the declared no-shell children in config order.
pub(crate) fn provision(
    toolchains: Option<&Toolchains>,
    apply: bool,
    runner: &dyn ToolchainRunner,
    format: Format,
) -> Outcome {
    let Some(toolchains) = toolchains else {
        return Outcome::refusal(
            format,
            ErrorCode::ConfigUndeclared,
            "toolchains section is not declared",
        );
    };
    if let Err(reason) = validate_toolchain_policy(toolchains) {
        return Outcome::refusal(
            format,
            ErrorCode::ConfigUnusable,
            format!("toolchain policy: {reason}"),
        );
    }
    let platform = std::env::consts::OS;
    let mut declared = Vec::new();
    for toolchain in &toolchains.entries {
        if toolchain.provision.is_empty() {
            continue;
        }
        let Some(provision) = toolchain
            .provision
            .iter()
            .find(|provision| provision.platform == platform)
        else {
            return Outcome::refusal(
                format,
                ErrorCode::ToolchainUndeclared,
                format!(
                    "toolchain `{}` has no provision declaration for `{platform}`",
                    toolchain.id,
                ),
            );
        };
        declared.push((toolchain, provision));
    }
    if !apply {
        return match format {
            Format::Text => Outcome::clean(format!(
                "[toolchain-provision] planned {} declared toolchains; pass --apply to run\n",
                declared.len()
            )),
            Format::Json => Outcome::clean(format!(
                "{}\n",
                serde_json::json!({
                    "schemaVersion": 1,
                    "command": "toolchain-provision",
                    "status": "planned",
                    "count": declared.len(),
                    "toolchains": serde_json::Value::from_iter(
                        declared.iter().map(|(toolchain, _)| toolchain.id.as_str())
                    ),
                })
            )),
        };
    }
    let mut completed = 0usize;
    for (toolchain, provision) in declared {
        let result = match runner.run(ToolchainLaunch {
            executable: &provision.executable,
            arguments: &provision.arguments,
            timeout_seconds: toolchain.timeout_seconds,
        }) {
            Ok(result) => result,
            Err(error) => {
                return Outcome::refusal(
                    format,
                    ErrorCode::ToolchainFailed,
                    format!("toolchain `{}` could not start: {}", toolchain.id, error.0),
                );
            }
        };
        if result.code != 0 {
            return Outcome::refusal(
                format,
                ErrorCode::ToolchainFailed,
                format!(
                    "toolchain `{}` provision failed with status {}",
                    toolchain.id, result.code
                ),
            );
        }
        completed += 1;
    }
    match format {
        Format::Text => Outcome::clean(format!(
            "[toolchain-provision] completed {completed} declared toolchains\n"
        )),
        Format::Json => Outcome::clean(format!(
            "{}\n",
            serde_json::json!({
                "schemaVersion": 1,
                "command": "toolchain-provision",
                "status": "completed",
                "count": completed,
            })
        )),
    }
}

fn leaves_root(path: &str) -> bool {
    path.trim().is_empty()
        || path.starts_with('/')
        || path
            .split('/')
            .scan(0isize, |depth, segment| {
                match segment {
                    ".." => *depth -= 1,
                    "." | "" => {}
                    _ => *depth += 1,
                }
                Some(*depth)
            })
            .any(|depth| depth < 0)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::runtime::{
        MemoryEnvironmentStore, MemoryTree, NoEnvironmentStore, NoToolchainRunner, ToolchainError,
        ToolchainResult,
    };
    use crate::v0_4::config::{
        EnvironmentAllowlist, EnvironmentContract, EnvironmentDetector, EnvironmentExample,
        StagedEnvironmentPolicy, Toolchain, ToolchainProvision,
    };
    use std::cell::RefCell;
    use std::collections::{BTreeSet, VecDeque};

    #[test]
    fn backup_refuses_every_escaping_or_empty_destination_before_any_write_port() {
        let environment = Environment::default();
        for destination in ["", "/outside", "../outside", "safe/../../outside"] {
            let tree = MemoryTree::default();
            let store = MemoryEnvironmentStore::new(&tree);
            let outcome = backup(
                Some(&environment),
                &tree,
                ".",
                Some(destination),
                &store,
                Format::Text,
            );
            assert_eq!(outcome.exit_code, 2, "{destination:?}");
            assert!(
                outcome
                    .stderr
                    .contains("backup destination leaves the repository root"),
                "{destination:?}"
            );
        }
    }

    #[test]
    fn operations_refuse_absent_policy_and_plan_empty_provisioning() {
        let tree = MemoryTree::default();
        let store = MemoryEnvironmentStore::new(&tree);
        assert_eq!(
            backup(None, &tree, ".", Some("safe-backups"), &store, Format::Text,).exit_code,
            2
        );
        assert_eq!(validate(None, &tree, Format::Text).exit_code, 2);

        let toolchains = Toolchains::default();
        let outcome = provision(Some(&toolchains), false, &NoToolchainRunner, Format::Text);
        assert_eq!(outcome.exit_code, 0);
        assert!(outcome.stdout.contains("planned 0"));
        assert_eq!(
            provision(None, true, &NoToolchainRunner, Format::Text).exit_code,
            2
        );
    }

    #[test]
    fn initialization_reads_only_declared_examples_and_creates_only_absent_targets() {
        let tree = MemoryTree::default();
        tree.write(".env.example", "SYNTHETIC=value\n");
        let environment = Environment {
            examples: vec![EnvironmentExample {
                source: ".env.example".to_string(),
                target: ".env.fixture".to_string(),
            }],
            ..Environment::default()
        };
        let store = MemoryEnvironmentStore::new(&tree);

        let planned = init(Some(&environment), &tree, ".", false, &store, Format::Text);
        assert_eq!(planned.exit_code, 0);
        assert!(matches!(
            tree.read(".env.fixture"),
            Err(TreeError::NotFound)
        ));

        let completed = init(Some(&environment), &tree, ".", true, &store, Format::Text);
        assert_eq!(completed.exit_code, 0);
        assert_eq!(tree.read(".env.fixture").unwrap(), "SYNTHETIC=value\n");

        let repeated = init(Some(&environment), &tree, ".", true, &store, Format::Text);
        assert_eq!(repeated.exit_code, 2);
        assert!(repeated.stderr.contains("already exists"));
    }

    #[test]
    fn initialization_refuses_invalid_or_duplicate_targets_before_a_write() {
        let tree = MemoryTree::default();
        tree.write(".env.example", "SYNTHETIC=value\n");
        let store = MemoryEnvironmentStore::new(&tree);
        for target in ["../outside", "/outside"] {
            let environment = Environment {
                examples: vec![EnvironmentExample {
                    source: ".env.example".to_string(),
                    target: target.to_string(),
                }],
                ..Environment::default()
            };
            assert_eq!(
                init(Some(&environment), &tree, ".", true, &store, Format::Text).exit_code,
                2
            );
            assert!(matches!(
                tree.read(".env.fixture"),
                Err(TreeError::NotFound)
            ));
        }
    }

    #[test]
    fn initialization_refuses_a_declared_source_symlink_before_a_write() {
        let mut tree = MemoryTree::default();
        tree.write(".env.example", "SYNTHETIC=value\n");
        tree.mark_link(".env.example");
        let environment = Environment {
            examples: vec![EnvironmentExample {
                source: ".env.example".to_string(),
                target: ".env.fixture".to_string(),
            }],
            ..Environment::default()
        };
        let store = MemoryEnvironmentStore::new(&tree);

        let outcome = init(Some(&environment), &tree, ".", true, &store, Format::Text);

        assert_eq!(outcome.exit_code, 2);
        assert!(outcome.stderr.contains("unreadable"));
        assert!(matches!(
            tree.read(".env.fixture"),
            Err(TreeError::NotFound)
        ));
    }

    #[test]
    fn backup_copies_only_declared_local_targets_without_replacing_a_prior_backup() {
        let tree = MemoryTree::default();
        tree.write(".env.fixture", "SYNTHETIC=value\n");
        let environment = Environment {
            examples: vec![EnvironmentExample {
                source: ".env.example".to_string(),
                target: ".env.fixture".to_string(),
            }],
            ..Environment::default()
        };
        let store = MemoryEnvironmentStore::new(&tree);

        let outcome = backup(
            Some(&environment),
            &tree,
            ".",
            Some("safe-backups"),
            &store,
            Format::Text,
        );
        assert_eq!(outcome.exit_code, 0);
        assert_eq!(
            tree.read("safe-backups/.env.fixture").unwrap(),
            "SYNTHETIC=value\n"
        );
        assert_eq!(
            backup(
                Some(&environment),
                &tree,
                ".",
                Some("safe-backups"),
                &store,
                Format::Text,
            )
            .exit_code,
            2
        );
    }

    #[test]
    fn restore_refuses_a_conflict_until_force_and_then_replaces_from_the_declared_backup() {
        let tree = MemoryTree::default();
        tree.write("safe-backups/.env.fixture", "SYNTHETIC=restored\n");
        tree.write(".env.fixture", "SYNTHETIC=current\n");
        let environment = Environment {
            examples: vec![EnvironmentExample {
                source: ".env.example".to_string(),
                target: ".env.fixture".to_string(),
            }],
            ..Environment::default()
        };
        let store = MemoryEnvironmentStore::new(&tree);

        let conflict = restore(
            Some(&environment),
            &tree,
            ".",
            Some("safe-backups"),
            false,
            &store,
            Format::Text,
        );
        assert_eq!(conflict.exit_code, 2);
        assert!(conflict.stderr.contains("use --force"));
        assert_eq!(tree.read(".env.fixture").unwrap(), "SYNTHETIC=current\n");

        let outcome = restore(
            Some(&environment),
            &tree,
            ".",
            Some("safe-backups"),
            true,
            &store,
            Format::Text,
        );
        assert_eq!(outcome.exit_code, 0);
        assert_eq!(tree.read(".env.fixture").unwrap(), "SYNTHETIC=restored\n");
    }

    #[test]
    fn staging_guard_reads_only_the_declared_index_and_never_reports_contents() {
        let mut tree = MemoryTree::default();
        tree.set_indexed_files([".env.example", ".env.fixture", "notes/ordinary.md"]);
        let environment = Environment {
            staged: Some(StagedEnvironmentPolicy {
                forbidden: vec![".env*".to_string()],
                allowed: vec![".env.example".to_string()],
            }),
            ..Environment::default()
        };

        let outcome = validate(Some(&environment), &tree, Format::Text);

        assert_eq!(outcome.exit_code, 1);
        assert!(
            outcome
                .stderr
                .contains(".env.fixture: staged-environment-file")
        );
        assert!(!outcome.stderr.contains("SYNTHETIC"));
        assert!(!outcome.stderr.contains("ordinary.md"));
    }

    #[test]
    fn staging_guard_refuses_a_broad_allowlist_before_reading_the_index() {
        let environment = Environment {
            staged: Some(StagedEnvironmentPolicy {
                forbidden: vec![".env*".to_string()],
                allowed: vec![".env*".to_string()],
            }),
            ..Environment::default()
        };
        let tree = MemoryTree::default();

        let outcome = validate(Some(&environment), &tree, Format::Text);

        assert_eq!(outcome.exit_code, 2);
        assert!(outcome.stderr.contains("exact repository-relative paths"));
    }

    #[test]
    fn curated_detectors_read_only_declared_sources_and_report_only_names() {
        let tree = MemoryTree::default();
        let sources = [
            ("src/rust.rs", "std::env::var(\"RUST_KEY\")"),
            ("src/site.ts", "process.env.TYPESCRIPT_KEY"),
            (
                "src/program.fs",
                "Environment.GetEnvironmentVariable(\"FSHARP_KEY\")",
            ),
            ("src/main.go", "os.LookupEnv(\"GO_KEY\")"),
            ("infra/main.tf", "get_env(\"TERRAFORM_KEY\")"),
            ("playbook.yml", "{{ lookup('env', 'ANSIBLE_KEY') }}"),
        ];
        for (path, source) in sources {
            tree.write(path, &format!("{source}\n// SYNTHETIC=value"));
        }
        let environment = Environment {
            contracts: vec![EnvironmentContract {
                path: ".env.example".to_string(),
                keys: [
                    "RUST_KEY",
                    "TYPESCRIPT_KEY",
                    "FSHARP_KEY",
                    "GO_KEY",
                    "TERRAFORM_KEY",
                    "ANSIBLE_KEY",
                ]
                .into_iter()
                .map(str::to_string)
                .collect(),
            }],
            detectors: vec![
                EnvironmentDetector {
                    language: EnvironmentLanguage::Rust,
                    paths: vec!["src/rust.rs".to_string()],
                },
                EnvironmentDetector {
                    language: EnvironmentLanguage::TypeScript,
                    paths: vec!["src/site.ts".to_string()],
                },
                EnvironmentDetector {
                    language: EnvironmentLanguage::FSharp,
                    paths: vec!["src/program.fs".to_string()],
                },
                EnvironmentDetector {
                    language: EnvironmentLanguage::Go,
                    paths: vec!["src/main.go".to_string()],
                },
                EnvironmentDetector {
                    language: EnvironmentLanguage::Terraform,
                    paths: vec!["infra/main.tf".to_string()],
                },
                EnvironmentDetector {
                    language: EnvironmentLanguage::Ansible,
                    paths: vec!["playbook.yml".to_string()],
                },
            ],
            ..Environment::default()
        };

        let outcome = validate(Some(&environment), &tree, Format::Text);

        assert_eq!(outcome.exit_code, 0, "{}", outcome.stderr);
        assert!(!outcome.stdout.contains("SYNTHETIC"));
    }

    #[test]
    fn detector_allowlist_is_exact_and_dynamic_access_stays_a_redacted_finding() {
        let tree = MemoryTree::default();
        tree.write(
            "src/site.ts",
            "process.env.ALLOWED; process.env.OUTSIDE; process.env[value]; // SYNTHETIC=value",
        );
        let environment = Environment {
            contracts: vec![EnvironmentContract {
                path: ".env.example".to_string(),
                keys: vec!["ALLOWED".to_string()],
            }],
            detectors: vec![EnvironmentDetector {
                language: EnvironmentLanguage::TypeScript,
                paths: vec!["src/site.ts".to_string()],
            }],
            allowlists: vec![EnvironmentAllowlist {
                rule: "read-but-undeclared".to_string(),
                path: "src/site.ts".to_string(),
                key: "OUTSIDE".to_string(),
                reason: "third-party type definition".to_string(),
                owner: "tooling".to_string(),
                expiry: "2099-01-01".to_string(),
            }],
            ..Environment::default()
        };

        let outcome = validate(Some(&environment), &tree, Format::Text);

        assert_eq!(outcome.exit_code, 1);
        assert!(outcome.stderr.contains("unsupported-dynamic-access"));
        assert!(!outcome.stderr.contains("OUTSIDE"));
        assert!(!outcome.stderr.contains("SYNTHETIC"));
    }

    #[test]
    fn environment_policy_validation_and_rendering_cover_every_declared_failure_shape() {
        let tree = MemoryTree::default();
        let invalid_allow = Environment {
            staged: Some(StagedEnvironmentPolicy {
                forbidden: vec!["[".to_string()],
                allowed: Vec::new(),
            }),
            ..Environment::default()
        };
        let invalid_index = validate(Some(&invalid_allow), &tree, Format::Text);
        assert_eq!(invalid_index.exit_code, 2);
        assert!(invalid_index.stderr.contains("staging guard refused"));

        let mut indexed = MemoryTree::default();
        indexed.set_indexed_files([".env.fixture"]);
        let invalid_pattern = validate(Some(&invalid_allow), &indexed, Format::Text);
        assert_eq!(invalid_pattern.exit_code, 2);
        assert!(invalid_pattern.stderr.contains("invalid path pattern"));

        for environment in [
            Environment {
                contracts: vec![EnvironmentContract {
                    path: "../outside".to_string(),
                    keys: vec!["KEY".to_string()],
                }],
                ..Environment::default()
            },
            Environment {
                contracts: vec![EnvironmentContract {
                    path: ".env.example".to_string(),
                    keys: vec!["invalid-key".to_string()],
                }],
                ..Environment::default()
            },
            Environment {
                detectors: vec![EnvironmentDetector {
                    language: EnvironmentLanguage::Rust,
                    paths: Vec::new(),
                }],
                ..Environment::default()
            },
            Environment {
                detectors: vec![EnvironmentDetector {
                    language: EnvironmentLanguage::Rust,
                    paths: vec!["../outside".to_string()],
                }],
                ..Environment::default()
            },
            Environment {
                allowlists: vec![EnvironmentAllowlist {
                    rule: String::new(),
                    path: "src/app.rs".to_string(),
                    key: "KEY".to_string(),
                    reason: "reason".to_string(),
                    owner: "owner".to_string(),
                    expiry: "not-a-date".to_string(),
                }],
                ..Environment::default()
            },
        ] {
            assert!(validate_detector_policy(&environment).is_err());
        }
        assert!(is_environment_key("_lower_1"));
        assert!(!is_environment_key("1INVALID"));
        assert!(is_iso_date("2099-01-01"));
        assert!(!is_iso_date("2099-1-01"));
        assert!(is_exact_path("src/app.rs"));
        assert!(!is_exact_path("src/*.rs"));

        for (language, source, expected_key, expected_dynamic) in [
            (EnvironmentLanguage::Rust, "std::env::var(KEY)", None, true),
            (
                EnvironmentLanguage::TypeScript,
                "process.env[KEY]; process.env.",
                None,
                true,
            ),
            (
                EnvironmentLanguage::FSharp,
                "Environment.GetEnvironmentVariable(\"FS_KEY\")",
                Some("FS_KEY"),
                false,
            ),
            (
                EnvironmentLanguage::Go,
                "os.Getenv('GO_KEY')",
                Some("GO_KEY"),
                false,
            ),
            (
                EnvironmentLanguage::Terraform,
                "env(\"TF_KEY\")",
                Some("TF_KEY"),
                false,
            ),
            (
                EnvironmentLanguage::Ansible,
                "{{ lookup('file', 'ignored') }} {{ lookup('env') }}",
                None,
                true,
            ),
        ] {
            let (keys, dynamic) = detector_accesses(language, source);
            if let Some(key) = expected_key {
                assert!(keys.contains(key));
            }
            assert_eq!(dynamic, expected_dynamic);
        }
        assert_eq!(
            quoted_value("'KEY' rest"),
            Some(("KEY".to_string(), " rest"))
        );
        assert_eq!(quoted_value("unquoted"), None);
        assert_eq!(key_len("KEY_2-rest"), 5);

        let findings = BTreeSet::from([
            EnvironmentFinding::new("declared-source-unread", "src/missing.rs", None),
            EnvironmentFinding::new(
                "read-but-undeclared",
                "src/app.rs",
                Some("OUTSIDE".to_string()),
            ),
        ]);
        let text = render_environment_findings(findings.clone(), 2, Format::Text);
        assert_eq!(text.exit_code, 1);
        assert!(text.stderr.contains("OUTSIDE"));
        let json = render_environment_findings(findings, 2, Format::Json);
        assert_eq!(json.exit_code, 1);
        assert!(json.stdout.contains("\"findings\""));
        assert!(
            clean_validation(3, Format::Json)
                .stdout
                .contains("\"inspected\":3")
        );
        assert!(patterns(&["docs/**".to_string()]).is_ok());
    }

    #[test]
    fn environment_transactions_refuse_unwritable_inputs_and_render_declared_json_outcomes() {
        let tree = MemoryTree::default();
        tree.write(".env.example", "SYNTHETIC=value");
        let store = MemoryEnvironmentStore::new(&tree);
        let one = Environment {
            examples: vec![EnvironmentExample {
                source: ".env.example".to_string(),
                target: ".env.fixture".to_string(),
            }],
            ..Environment::default()
        };
        assert_eq!(
            init(Some(&one), &tree, ".", false, &store, Format::Json).stdout,
            "{\"command\":\"environment-init\",\"count\":1,\"schemaVersion\":1,\"status\":\"planned\",\"targets\":[\".env.fixture\"]}\n"
        );
        assert!(!tree.exists(".env.fixture"));
        assert!(
            init(None, &tree, ".", true, &store, Format::Text)
                .stderr
                .contains("not declared")
        );
        assert!(
            init(
                Some(&Environment::default()),
                &tree,
                ".",
                true,
                &store,
                Format::Text
            )
            .stderr
            .contains("no example targets")
        );
        let same = Environment {
            examples: vec![EnvironmentExample {
                source: ".env.example".to_string(),
                target: ".env.example".to_string(),
            }],
            ..Environment::default()
        };
        assert!(
            init(Some(&same), &tree, ".", true, &store, Format::Text)
                .stderr
                .contains("source and target are the same")
        );
        let duplicate = Environment {
            examples: vec![
                EnvironmentExample {
                    source: ".env.example".to_string(),
                    target: ".env.fixture".to_string(),
                },
                EnvironmentExample {
                    source: ".env.example".to_string(),
                    target: ".env.fixture".to_string(),
                },
            ],
            ..Environment::default()
        };
        assert!(
            init(Some(&duplicate), &tree, ".", true, &store, Format::Text)
                .stderr
                .contains("more than once")
        );
        let missing = Environment {
            examples: vec![EnvironmentExample {
                source: "missing.example".to_string(),
                target: ".env.fixture".to_string(),
            }],
            ..Environment::default()
        };
        assert!(
            init(Some(&missing), &tree, ".", true, &store, Format::Text)
                .stderr
                .contains("is missing")
        );
        let mut unreadable_tree = MemoryTree::default();
        unreadable_tree.write(".env.example", "SYNTHETIC=value");
        unreadable_tree.mark_unreadable(".env.example");
        assert!(
            init(
                Some(&one),
                &unreadable_tree,
                ".",
                true,
                &MemoryEnvironmentStore::new(&unreadable_tree),
                Format::Text,
            )
            .stderr
            .contains("unreadable")
        );
        assert!(
            init(
                Some(&one),
                &tree,
                ".",
                true,
                &NoEnvironmentStore,
                Format::Text
            )
            .stderr
            .contains("no environment write boundary")
        );
        assert!(
            init(Some(&one), &tree, ".", true, &store, Format::Json)
                .stdout
                .contains("\"status\":\"completed\"")
        );

        assert!(
            backup(Some(&one), &tree, ".", None, &store, Format::Text)
                .stderr
                .contains("requires --dir")
        );
        assert!(
            backup(None, &tree, ".", Some("backup"), &store, Format::Text)
                .stderr
                .contains("not declared")
        );
        let backup_missing = Environment {
            examples: vec![EnvironmentExample {
                source: ".env.example".to_string(),
                target: ".env.never-created".to_string(),
            }],
            ..Environment::default()
        };
        assert!(
            backup(
                Some(&backup_missing),
                &tree,
                ".",
                Some("backup"),
                &store,
                Format::Text,
            )
            .stderr
            .contains("target `.env.never-created` is missing")
        );
        tree.write(".env.fixture", "SYNTHETIC=value");
        assert!(
            backup(
                Some(&one),
                &tree,
                ".",
                Some("backup"),
                &NoEnvironmentStore,
                Format::Text
            )
            .stderr
            .contains("no environment write boundary")
        );
        let empty_backup = backup(
            Some(&Environment::default()),
            &tree,
            ".",
            Some("backup"),
            &store,
            Format::Json,
        );
        assert_eq!(empty_backup.exit_code, 0);
        assert!(empty_backup.stdout.contains("\"destination\":\"backup\""));
        assert_eq!(backup_path(".", ".env.fixture"), ".env.fixture");
        assert_eq!(
            backup_path("backup/", ".env.fixture"),
            "backup/.env.fixture"
        );

        assert!(
            restore(Some(&one), &tree, ".", None, false, &store, Format::Text)
                .stderr
                .contains("requires --dir")
        );
        assert!(
            restore(
                None,
                &tree,
                ".",
                Some("backup"),
                false,
                &store,
                Format::Text
            )
            .stderr
            .contains("not declared")
        );
        assert!(
            restore(
                Some(&one),
                &tree,
                ".",
                Some("backup"),
                false,
                &store,
                Format::Text
            )
            .stderr
            .contains("backup `backup/.env.fixture` is missing")
        );
        tree.write("backup/.env.fixture", "SYNTHETIC=restored");
        assert!(
            restore(
                Some(&one),
                &tree,
                ".",
                Some("backup"),
                true,
                &NoEnvironmentStore,
                Format::Text
            )
            .stderr
            .contains("no environment write boundary")
        );
        let restored = restore(
            Some(&one),
            &tree,
            ".",
            Some("backup"),
            true,
            &store,
            Format::Json,
        );
        assert_eq!(restored.exit_code, 0);
        assert!(restored.stdout.contains("\"environment-restore\""));
        assert!(leaves_root("safe/../../outside"));
        assert!(!leaves_root("safe/../backup"));
    }

    struct RecordingToolchainRunner {
        outcomes: RefCell<VecDeque<Result<ToolchainResult, ToolchainError>>>,
        launches: RefCell<Vec<(String, Vec<String>)>>,
    }

    impl RecordingToolchainRunner {
        fn with(
            outcomes: impl IntoIterator<Item = Result<ToolchainResult, ToolchainError>>,
        ) -> Self {
            Self {
                outcomes: RefCell::new(outcomes.into_iter().collect()),
                launches: RefCell::new(Vec::new()),
            }
        }
    }

    impl ToolchainRunner for RecordingToolchainRunner {
        fn run(&self, launch: ToolchainLaunch<'_>) -> Result<ToolchainResult, ToolchainError> {
            self.launches
                .borrow_mut()
                .push((launch.executable.to_string(), launch.arguments.to_vec()));
            self.outcomes.borrow_mut().pop_front().unwrap_or_else(|| {
                Err(ToolchainError(
                    "the synthetic runner had no declared result".to_string(),
                ))
            })
        }
    }

    fn toolchain(id: &str, provision: Vec<ToolchainProvision>) -> Toolchain {
        Toolchain {
            id: id.to_string(),
            executable: "probe-tool".to_string(),
            probe: vec!["--version".to_string()],
            parser: Some(ToolchainOutputParser::FirstToken),
            version: Some("1.2.3".to_string()),
            timeout_seconds: None,
            provision,
            required: true,
        }
    }

    #[test]
    fn toolchain_probe_hides_output_and_provision_stops_after_first_failure() {
        let first = ToolchainProvision {
            platform: std::env::consts::OS.to_string(),
            executable: "installer-one".to_string(),
            arguments: vec!["--literal=$VALUE".to_string()],
        };
        let second = ToolchainProvision {
            platform: std::env::consts::OS.to_string(),
            executable: "installer-two".to_string(),
            arguments: vec!["--next".to_string()],
        };
        let toolchains = Toolchains {
            entries: vec![
                toolchain("first", vec![first]),
                toolchain("second", vec![second]),
            ],
        };
        let probe_runner = RecordingToolchainRunner::with([Ok(ToolchainResult {
            code: 0,
            stdout: "1.2.3 SYNTHETIC=value".to_string(),
        })]);

        let probe = validate_toolchains(
            Some(&Toolchains {
                entries: vec![toolchain("first", Vec::new())],
            }),
            &probe_runner,
            Format::Text,
        );

        assert_eq!(probe.exit_code, 0);
        assert!(!probe.stdout.contains("SYNTHETIC"));

        let runner = RecordingToolchainRunner::with([Ok(ToolchainResult {
            code: 1,
            stdout: "SYNTHETIC=value".to_string(),
        })]);
        let planned = provision(Some(&toolchains), false, &runner, Format::Text);
        assert_eq!(planned.exit_code, 0);
        assert!(runner.launches.borrow().is_empty());
        let planned = provision(Some(&toolchains), false, &runner, Format::Json);
        assert_eq!(
            planned.stdout,
            "{\"command\":\"toolchain-provision\",\"count\":2,\"schemaVersion\":1,\"status\":\"planned\",\"toolchains\":[\"first\",\"second\"]}\n"
        );
        assert!(runner.launches.borrow().is_empty());

        let outcome = provision(Some(&toolchains), true, &runner, Format::Text);
        assert_eq!(outcome.exit_code, 2);
        assert_eq!(runner.launches.borrow().len(), 1);
        assert_eq!(runner.launches.borrow()[0].0, "installer-one");
        assert!(!outcome.stderr.contains("SYNTHETIC"));
    }

    #[test]
    fn toolchain_refuses_zero_timeout_before_it_reaches_the_runner() {
        let mut declared = toolchain("invalid-timeout", Vec::new());
        declared.timeout_seconds = Some(0);
        let toolchains = Toolchains {
            entries: vec![declared],
        };
        let runner = RecordingToolchainRunner::with([]);

        let outcome = validate_toolchains(Some(&toolchains), &runner, Format::Text);

        assert_eq!(outcome.exit_code, 2);
        assert!(outcome.stderr.contains("zero timeout"));
        assert!(runner.launches.borrow().is_empty());
    }

    #[test]
    fn toolchain_policy_validation_probe_and_provision_cover_every_terminal_result() {
        let valid = toolchain("valid", Vec::new());
        for declared in [
            Toolchains {
                entries: vec![Toolchain {
                    id: String::new(),
                    ..valid.clone()
                }],
            },
            Toolchains {
                entries: vec![Toolchain {
                    executable: "-invalid".to_string(),
                    ..valid.clone()
                }],
            },
            Toolchains {
                entries: vec![Toolchain {
                    probe: Vec::new(),
                    ..valid.clone()
                }],
            },
            Toolchains {
                entries: vec![Toolchain {
                    parser: None,
                    ..valid.clone()
                }],
            },
            Toolchains {
                entries: vec![Toolchain {
                    version: Some(String::new()),
                    ..valid.clone()
                }],
            },
            Toolchains {
                entries: vec![Toolchain {
                    provision: vec![
                        ToolchainProvision {
                            platform: "linux".to_string(),
                            executable: "installer".to_string(),
                            arguments: Vec::new(),
                        },
                        ToolchainProvision {
                            platform: "linux".to_string(),
                            executable: "installer".to_string(),
                            arguments: Vec::new(),
                        },
                    ],
                    ..valid.clone()
                }],
            },
        ] {
            assert!(validate_toolchain_policy(&declared).is_err());
        }
        assert!(valid_executable("tool"));
        assert!(!valid_executable("tool\n"));
        assert!(is_platform("macos-14"));
        assert!(!is_platform("macOS"));
        assert!(toolchain_version_matches(&valid, "1.2.3 extra"));
        let mut first_line = valid.clone();
        first_line.parser = Some(ToolchainOutputParser::FirstLine);
        assert!(toolchain_version_matches(&first_line, "1.2.3\nsecond"));
        let mut no_constraint = valid.clone();
        no_constraint.parser = None;
        no_constraint.version = None;
        assert!(toolchain_version_matches(&no_constraint, "anything"));
        let mut half_constraint = valid.clone();
        half_constraint.parser = None;
        assert!(!toolchain_version_matches(&half_constraint, "1.2.3"));

        assert_eq!(
            validate_toolchains(None, &NoToolchainRunner, Format::Text).exit_code,
            2
        );
        let unavailable =
            RecordingToolchainRunner::with([Err(ToolchainError("offline".to_string()))]);
        let unavailable_outcome = validate_toolchains(
            Some(&Toolchains {
                entries: vec![valid.clone()],
            }),
            &unavailable,
            Format::Text,
        );
        assert_eq!(unavailable_outcome.exit_code, 1);
        assert!(unavailable_outcome.stderr.contains("unavailable"));
        let failed_probe = RecordingToolchainRunner::with([Ok(ToolchainResult {
            code: 9,
            stdout: "SYNTHETIC=value".to_string(),
        })]);
        let failed_probe_outcome = validate_toolchains(
            Some(&Toolchains {
                entries: vec![valid.clone()],
            }),
            &failed_probe,
            Format::Json,
        );
        assert_eq!(failed_probe_outcome.exit_code, 1);
        assert!(failed_probe_outcome.stdout.contains("probe-failed"));
        let mismatch = RecordingToolchainRunner::with([Ok(ToolchainResult {
            code: 0,
            stdout: "9.9.9".to_string(),
        })]);
        assert!(
            validate_toolchains(
                Some(&Toolchains {
                    entries: vec![valid.clone()],
                }),
                &mismatch,
                Format::Text,
            )
            .stderr
            .contains("version-mismatch")
        );
        let mut optional = valid.clone();
        optional.required = false;
        let optional_runner =
            RecordingToolchainRunner::with([Err(ToolchainError("offline".to_string()))]);
        let optional_outcome = validate_toolchains(
            Some(&Toolchains {
                entries: vec![optional],
            }),
            &optional_runner,
            Format::Json,
        );
        assert_eq!(optional_outcome.exit_code, 0);
        assert!(optional_outcome.stdout.contains("\"exitCode\":0"));

        let unsupported = Toolchains {
            entries: vec![toolchain(
                "unsupported",
                vec![ToolchainProvision {
                    platform: "unavailable-platform".to_string(),
                    executable: "installer".to_string(),
                    arguments: Vec::new(),
                }],
            )],
        };
        assert!(
            provision(Some(&unsupported), true, &NoToolchainRunner, Format::Text,)
                .stderr
                .contains("no provision declaration")
        );
        let declared = Toolchains {
            entries: vec![toolchain(
                "declared",
                vec![ToolchainProvision {
                    platform: std::env::consts::OS.to_string(),
                    executable: "installer".to_string(),
                    arguments: vec!["install".to_string()],
                }],
            )],
        };
        let launch_error =
            RecordingToolchainRunner::with([Err(ToolchainError("not found".to_string()))]);
        assert!(
            provision(Some(&declared), true, &launch_error, Format::Text)
                .stderr
                .contains("could not start")
        );
        let completed = RecordingToolchainRunner::with([Ok(ToolchainResult {
            code: 0,
            stdout: String::new(),
        })]);
        let completed_outcome = provision(Some(&declared), true, &completed, Format::Json);
        assert_eq!(completed_outcome.exit_code, 0);
        assert!(completed_outcome.stdout.contains("\"count\":1"));
    }

    #[test]
    fn uncommon_environment_and_toolchain_refusals_stay_inside_declared_boundaries() {
        let unread_detector = Environment {
            contracts: vec![EnvironmentContract {
                path: "src/missing.rs".to_string(),
                keys: vec!["DECLARED".to_string()],
            }],
            detectors: vec![EnvironmentDetector {
                language: EnvironmentLanguage::Rust,
                paths: vec!["src/missing.rs".to_string()],
            }],
            ..Environment::default()
        };
        let tree = MemoryTree::default();
        let detector_result = validate(Some(&unread_detector), &tree, Format::Text);
        assert_eq!(detector_result.exit_code, 1);
        assert!(detector_result.stderr.contains("declared-source-unread"));
        assert!(detector_result.stderr.contains("declared-but-unread"));

        let invalid_policy = Environment {
            allowlists: vec![EnvironmentAllowlist {
                rule: "rule".to_string(),
                path: "src/app.rs".to_string(),
                key: "KEY".to_string(),
                reason: "reason".to_string(),
                owner: "owner".to_string(),
                expiry: "not-a-date".to_string(),
            }],
            ..Environment::default()
        };
        assert_eq!(
            validate(Some(&invalid_policy), &tree, Format::Text).exit_code,
            2
        );
        assert!(!allowlisted(
            &Environment::default(),
            &EnvironmentFinding::new("rule", "src/app.rs", None),
        ));
        assert!(ansible_accesses("lookup(unquoted)").1);
        assert!(ansible_accesses("lookup('env')").1);
        assert!(ansible_accesses("lookup('env', unquoted)").1);

        let escape = Environment {
            examples: vec![EnvironmentExample {
                source: ".env.example".to_string(),
                target: "../outside".to_string(),
            }],
            ..Environment::default()
        };
        let store = MemoryEnvironmentStore::new(&tree);
        assert_eq!(
            backup(
                Some(&escape),
                &tree,
                ".",
                Some("backup"),
                &store,
                Format::Text
            )
            .exit_code,
            2
        );
        assert_eq!(
            restore(
                Some(&escape),
                &tree,
                ".",
                Some("backup"),
                false,
                &store,
                Format::Text
            )
            .exit_code,
            2
        );
        assert_eq!(
            restore(
                Some(&Environment::default()),
                &tree,
                ".",
                Some("../outside"),
                false,
                &store,
                Format::Text,
            )
            .exit_code,
            2
        );

        let mut binary = MemoryTree::default();
        binary.write(".env.fixture", "synthetic");
        binary.mark_binary(".env.fixture");
        let target = Environment {
            examples: vec![EnvironmentExample {
                source: ".env.example".to_string(),
                target: ".env.fixture".to_string(),
            }],
            ..Environment::default()
        };
        assert!(
            backup(
                Some(&target),
                &binary,
                ".",
                Some("backup"),
                &MemoryEnvironmentStore::new(&binary),
                Format::Text,
            )
            .stderr
            .contains("unreadable")
        );
        binary.write("backup/.env.fixture", "synthetic");
        binary.mark_binary("backup/.env.fixture");
        assert!(
            restore(
                Some(&target),
                &binary,
                ".",
                Some("backup"),
                true,
                &MemoryEnvironmentStore::new(&binary),
                Format::Text,
            )
            .stderr
            .contains("unreadable")
        );

        assert_eq!(
            provision(None, false, &NoToolchainRunner, Format::Text).exit_code,
            2
        );
        let valid = toolchain("valid", Vec::new());
        let invalid = Toolchains {
            entries: vec![Toolchain {
                id: String::new(),
                ..valid.clone()
            }],
        };
        assert_eq!(
            provision(Some(&invalid), false, &NoToolchainRunner, Format::Text).exit_code,
            2
        );
        assert_eq!(
            provision(
                Some(&Toolchains {
                    entries: vec![valid.clone()],
                }),
                false,
                &NoToolchainRunner,
                Format::Text,
            )
            .exit_code,
            0
        );
        let no_result = RecordingToolchainRunner::with([]);
        assert_eq!(
            validate_toolchains(
                Some(&Toolchains {
                    entries: vec![valid]
                }),
                &no_result,
                Format::Text,
            )
            .exit_code,
            1
        );
    }
}
