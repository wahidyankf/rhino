//! Ordered companion sets.
//!
//! A governed document too large for its budget splits into an entrypoint plus
//! a sibling directory named exactly after it. The directory carries a README
//! indexing its modules, and the filenames carry ordinal prefixes when, and
//! only when, the entrypoint declares a reading order.
//!
//! What is deliberately not decided here is whether a set *needs* ordering.
//! That the third module assumes the first is a claim about content, and a
//! validator guessing at it would be asserting that word shapes prove meaning.
//! The mechanical half -- the shape a declared order obliges -- is all of this
//! module.

use crate::governance::structure::ROOT;
use crate::report::{Finding, Report};
use crate::runtime::{Tree, TreeError};
use std::collections::BTreeMap;

const INDEX: &str = "README.md";

/// Suffixes a companion directory is written with when the relationship is
/// spelled out a second time.
///
/// Closed on purpose. The shared name already says the directory belongs to the
/// document, and a suffix adds a word that every link, every index, and every
/// reader carries without learning anything from it -- but which suffixes are
/// in use is a fact about existing repositories rather than a pattern to infer,
/// so a fourth is a decision rather than a guess.
const SUFFIXES: [&str; 3] = ["-details", "-modules", "-parts"];

pub fn validate(tree: &dyn Tree) -> Report {
    let mut report = Report::new("governance-companions", "companion set");
    let files = tree.files();
    let prefix = format!("{ROOT}/");

    // Every directory under the governance root, with the modules it holds
    // directly. Built once: a set is decided by what sits beside it, so asking
    // per directory would walk the tree once per directory.
    let mut modules: BTreeMap<String, Vec<String>> = BTreeMap::new();
    for path in &files {
        let Some(rest) = path.strip_prefix(&prefix) else {
            continue;
        };
        // A document directly under the root belongs to no set: a companion
        // directory sits beside the document it carries, and the root is
        // nobody's companion.
        let Some((directory, name)) = rest.rsplit_once('/') else {
            continue;
        };
        modules
            .entry(format!("{prefix}{directory}"))
            .or_default()
            .push(name.to_string());
    }

    for (directory, held) in &modules {
        // Total: every key was built from a path that carries a separator.
        let (parent, name) = directory.rsplit_once('/').unwrap_or((ROOT, directory));

        // A suffixed directory is reported and left alone. Its index, its
        // ordinals, and its modules are all questions about a set that has to
        // be renamed first, and answering them would bury the rename.
        if let Some(stem) = SUFFIXES.iter().find_map(|suffix| name.strip_suffix(suffix))
            && tree.exists(&format!("{parent}/{stem}.md"))
        {
            report.inspected_one();
            report.found(Finding::new(
                "suffixed-companion-directory",
                directory,
                format!(
                    "is named after `{stem}.md` with a suffix; a companion directory is named exactly after its document"
                ),
            ));
            continue;
        }

        let entrypoint = format!("{parent}/{name}.md");
        let text = match tree.read(&entrypoint) {
            Ok(text) => text,
            // Not a companion set. A directory with no sibling document of its
            // own name is a category, and this command has nothing to say
            // about one.
            Err(TreeError::NotFound) => continue,
            Err(error) => return refuse(&entrypoint, &error),
        };

        report.inspected_one();
        if let Some(refusal) = inspect(tree, directory, &text, held, &mut report) {
            return refusal;
        }
    }

    report
}

/// A read that failed on something other than absence, as a refusal.
///
/// One place, because a file this command could not open leaves it reporting on
/// a set it never read, and the two ways that happens oblige the same answer.
fn refuse(path: &str, error: &TreeError) -> Report {
    let reason = match error {
        TreeError::Unreadable(reason) => reason.as_str(),
        // Absence is answered before this is reached, so folding it in here
        // keeps the expression total without an arm nothing runs.
        _ => "holds no text",
    };
    Report::refused("governance-companions", format!("{path}: {reason}"))
}

/// One companion set: its index, and the ordinals its entrypoint obliges.
fn inspect(
    tree: &dyn Tree,
    directory: &str,
    entrypoint: &str,
    held: &[String],
    report: &mut Report,
) -> Option<Report> {
    let live: Vec<&String> = held.iter().filter(|name| name.as_str() != INDEX).collect();

    let index_path = format!("{directory}/{INDEX}");
    let index = match tree.read(&index_path) {
        Ok(index) => Some(index),
        Err(TreeError::NotFound) => None,
        // Fail closed. A README that cannot be opened leaves the index unknown,
        // and reporting every module as unindexed would be reporting on a file
        // nobody read.
        Err(error) => return Some(refuse(&index_path, &error)),
    };
    match index {
        Some(index) => {
            let linked = crate::markdown::sibling_links(&index);
            for name in &live {
                if !linked.contains_key(name.as_str()) {
                    report.found(Finding::new(
                        "unindexed-companion-module",
                        format!("{directory}/{name}"),
                        "is not indexed by its companion README, which lists the modules of the set in reading order",
                    ));
                }
            }
            for target in linked.keys() {
                if !held.iter().any(|name| name == target) {
                    report.found(Finding::new(
                        "missing-indexed-companion",
                        directory,
                        format!("indexes `{target}`, which is not there"),
                    ));
                }
            }
        }
        // A set with no index is reported once. Reporting every module as
        // unindexed as well would name the same missing file once per module.
        None => {
            report.found(Finding::new(
                "missing-companion-index",
                directory,
                "carries no README; a companion directory indexes its modules in reading order",
            ));
        }
    }

    // The entrypoint declares a reading order by numbering the list it links
    // its modules from. A bulleted list is a collection; a numbered one is a
    // sequence, and the difference is the only mechanical statement of intent
    // the document makes.
    let ordered = declares_an_order(entrypoint, directory);
    let mut ordinals: Vec<usize> = Vec::new();
    for name in &live {
        match (ordered, ordinal(name)) {
            (true, None) => {
                report.found(Finding::new(
                    "unordered-module-in-ordered-set",
                    format!("{directory}/{name}"),
                    "carries no ordinal prefix, and its entrypoint declares a reading order",
                ));
            }
            (false, Some(_)) => {
                report.found(Finding::new(
                "ordinal-in-unordered-set",
                format!("{directory}/{name}"),
                    "carries an ordinal prefix, and its entrypoint declares no reading order; a prefix asserts a sequence the entrypoint does not claim",
                ));
            }
            (true, Some(value)) => ordinals.push(value),
            (false, None) => {}
        }
    }

    if ordered && !ordinals.is_empty() {
        ordinals.sort_unstable();
        let contiguous = ordinals
            .iter()
            .enumerate()
            .all(|(index, value)| *value == index + 1);
        if !contiguous {
            report.found(Finding::new(
                "non-contiguous-companion-ordinals",
                directory,
                "ordinals are not contiguous from 001; inserting a module renumbers what follows, because the reading order is the only thing these names are for",
            ));
        }
    }

    None
}

/// Whether the entrypoint links into its companion directory from a numbered
/// list.
fn declares_an_order(entrypoint: &str, directory: &str) -> bool {
    // Total: every companion directory path carries a separator.
    let name = directory
        .rsplit_once('/')
        .map_or(directory, |(_, name)| name);
    let into = format!("({name}/");
    crate::markdown::prose_lines(entrypoint)
        .into_iter()
        .any(|(_, line)| line.contains(&into) && is_numbered(line))
}

/// Whether a line opens a numbered list item.
fn is_numbered(line: &str) -> bool {
    let trimmed = line.trim_start();
    let digits: String = trimmed.chars().take_while(char::is_ascii_digit).collect();
    !digits.is_empty() && trimmed[digits.len()..].starts_with(['.', ')'])
}

/// The ordinal a filename carries, when it carries a three-digit one.
fn ordinal(name: &str) -> Option<usize> {
    let (prefix, rest) = name.split_at_checked(3)?;
    if !rest.starts_with('-') || !prefix.chars().all(|c| c.is_ascii_digit()) {
        return None;
    }
    prefix.parse().ok()
}
