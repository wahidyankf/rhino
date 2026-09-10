//! Directory-map validation.
//!
//! Within a declared tree, every directory carries a `README.md` whose
//! `## Directory Map` section lists each direct sibling exactly once with a
//! resolvable relative target. The point is navigability: a reader who opens
//! any directory can see what is in it without listing the filesystem, and a
//! map that has drifted from the tree is worse than no map, because it is
//! believed.

use crate::config::{Config, DirectoryMap};
use crate::report::{Finding, Report};
use crate::runtime::{Tree, TreeError};
use crate::scan::{self, README, Scope};
use std::collections::BTreeSet;

const SECTION: &str = "## Directory Map";

pub fn validate(tree: &dyn Tree, config: &Config, map: &DirectoryMap, scope: &Scope) -> Report {
    let mut report = Report::new("directory-map", "directory");

    // A selected tree replaces the declared ones rather than adding to them,
    // and must actually be a directory: `--directory` naming nothing is a
    // mistake in the invocation, and reporting it as a clean walk of zero
    // directories would be the wrong answer to the wrong question.
    let trees: Vec<String> = match &scope.directory {
        Some(selected) => {
            if !tree.is_directory(selected) {
                return Report::refused(
                    "directory-map",
                    format!("{selected} is not a directory in this repository"),
                );
            }
            vec![selected.clone()]
        }
        None => map
            .trees
            .iter()
            .map(|declared| declared.path.clone())
            .collect(),
    };

    for path in &trees {
        for directory in scan::directories(tree, config, path) {
            report.inspected_one();
            if let Err(reason) = inspect(tree, &directory, &mut report) {
                return Report::refused("directory-map", reason);
            }
        }
    }

    report
}

/// `Ok(())` when the directory was inspected, `Err` when it could not be.
///
/// A README that exists and cannot be opened is not a missing README: reporting
/// it as one would tell a maintainer to write a file that is already there,
/// and would let a permission fault masquerade as a policy violation.
fn inspect(tree: &dyn Tree, directory: &str, report: &mut Report) -> Result<(), String> {
    let readme = format!("{directory}/{README}");
    let text = match tree.read(&readme) {
        Ok(text) => text,
        Err(TreeError::Unreadable(reason)) => return Err(format!("{readme}: {reason}")),
        // A README the repository declared as this directory's map, holding no
        // text to read it from. Refused rather than reported as missing, on the
        // same reasoning as above: the file is there.
        Err(TreeError::NotText) => return Err(format!("{readme}: holds no text")),
        Err(TreeError::NotFound) => {
            report.found(Finding::new(
                "missing-readme",
                directory,
                "missing README: a mapped directory has to document its own contents",
            ));
            return Ok(());
        }
    };

    let Some(entries) = map_entries(&text) else {
        report.found(Finding::new(
            "missing-directory-map",
            &readme,
            format!("missing directory map: a mapped README needs a `{SECTION}` section"),
        ));
        return Ok(());
    };

    // Resolve every entry first, so an unusable one is reported as itself
    // rather than as a sibling that went missing.
    let mut covered: BTreeSet<String> = BTreeSet::new();
    for (line, target) in entries {
        match resolve(tree, directory, &target) {
            Some(sibling) => {
                covered.insert(sibling);
            }
            None => {
                report.found(
                    Finding::new(
                    "invalid-map-entry",
                    &readme,
                    format!(
                        "invalid map entry `{}`: an entry has to name a direct sibling by a relative path that exists",
                        target.replace('\0', "\\0")
                    ),
                    )
                    .at_line(line),
                );
            }
        }
    }

    for sibling in siblings(tree, directory) {
        if !covered.contains(&sibling) {
            report.found(Finding::new(
                "missing-map-entry",
                &readme,
                format!(
                    "missing map entry for `{sibling}`: every direct sibling appears exactly once"
                ),
            ));
        }
    }

    Ok(())
}

/// The direct children of a directory, excluding its own README.
fn siblings(tree: &dyn Tree, directory: &str) -> Vec<String> {
    tree.children(directory)
        .into_iter()
        .filter(|child| child != &format!("{directory}/{README}"))
        .collect()
}

/// The entries of a README's map section, or `None` when it has no section.
///
/// A section with no entries is `Some(empty)` rather than `None`: a directory
/// with nothing beside its README has an empty map, and that is a complete map
/// rather than a missing one.
fn map_entries(text: &str) -> Option<Vec<(usize, String)>> {
    let mut entries = Vec::new();
    let mut inside = false;

    for (index, line) in text.lines().enumerate() {
        let trimmed = line.trim();
        if trimmed.starts_with("##") {
            if inside {
                break;
            }
            inside = trimmed == SECTION;
            continue;
        }
        if trimmed.starts_with("# ") && inside {
            break;
        }
        if !inside {
            continue;
        }
        if let Some(target) = list_entry(trimmed) {
            entries.push((index + 1, target));
        }
    }

    inside.then_some(entries)
}

/// `- [Text](target)`, the one form a map entry takes.
fn list_entry(line: &str) -> Option<String> {
    let rest = line
        .strip_prefix("- ")
        .or_else(|| line.strip_prefix("* "))?;
    let (_, after) = rest.split_once("](")?;
    let (target, _) = after.split_once(')')?;
    Some(target.trim().to_string())
}

/// The sibling an entry covers, or `None` when the entry is unusable.
///
/// A map entry is narrower than an ordinary link: it may not be absolute, carry
/// a scheme, leave its own directory, or point at something that is not there.
/// A map is a claim about *this* directory, and an entry that addresses
/// anywhere else is a claim about something the reader did not ask about.
fn resolve(tree: &dyn Tree, directory: &str, target: &str) -> Option<String> {
    if target.is_empty() || target.contains('\0') || target.starts_with('/') {
        return None;
    }
    if target.split_once(':').is_some_and(|(scheme, _)| {
        !scheme.is_empty()
            && scheme.starts_with(|c: char| c.is_ascii_alphabetic())
            && scheme
                .chars()
                .all(|c| c.is_ascii_alphanumeric() || matches!(c, '+' | '.' | '-'))
    }) {
        return None;
    }

    let without_fragment = target.split('#').next().unwrap_or(target);
    let path = without_fragment
        .split('?')
        .next()
        .unwrap_or(without_fragment);

    let mut segments: Vec<&str> = Vec::new();
    for segment in path.split('/') {
        match segment {
            "" | "." => {}
            ".." => return None,
            name => segments.push(name),
        }
    }
    let (first, rest) = segments.split_first()?;

    // An entry may address a sibling directly, or through that directory's own
    // README -- which is how a map links a subdirectory to something readable.
    let sibling = format!("{directory}/{first}");
    let addressed = match rest {
        [] => sibling.clone(),
        [only] if *only == README => format!("{sibling}/{README}"),
        _ => return None,
    };

    tree.exists(&addressed).then_some(sibling)
}
