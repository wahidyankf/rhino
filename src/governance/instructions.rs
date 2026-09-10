//! The instruction spine.
//!
//! Every repository keeps one canonical instruction body, opening with five
//! top-level sections in a fixed order. Repository-specific sections may
//! follow; none may be inserted between spine sections, because a reader
//! looking for the same fact in two repositories has to find it in the same
//! place.
//!
//! `CLAUDE.md` becomes exactly an import of that body. Only the terminal state
//! is checked here, and deliberately so: whether every pre-existing statement
//! received a disposition is a fact about a file that no longer exists, and a
//! validator claiming to have checked it would be claiming to have read
//! history.

use crate::markdown;
use crate::report::{Finding, Report};
use crate::runtime::{Tree, TreeError};

pub const CANONICAL: &str = "AGENTS.md";
pub const IMPORT: &str = "CLAUDE.md";

/// The five sections, in the order they are written.
const SPINE: [&str; 5] = [
    "Repository Contract",
    "Ownership",
    "Required Workflow",
    "Authoring Rules",
    "Harnesses",
];

/// The heading level a spine section is written at. A heading at any other
/// level is a heading inside a section rather than a section.
const LEVEL: &str = "## ";

pub fn validate(tree: &dyn Tree) -> Report {
    let mut report = Report::new("governance-instructions", "instruction");

    let text = match tree.read(CANONICAL) {
        Ok(text) => text,
        Err(TreeError::NotFound) => {
            report.inspected_one();
            report.found(Finding::new(
                "missing-canonical-instruction",
                CANONICAL,
                "is not there; every repository keeps one canonical instruction body, and every harness reaches it",
            ));
            return report;
        }
        Err(error) => return refuse(CANONICAL, &error),
    };

    report.inspected_one();
    spine(&text, &mut report);

    // An absent import is legal. A repository with no Claude surface has no
    // adapter to keep honest, and inventing one would be this tool activating a
    // harness the repository never declared.
    match tree.read(IMPORT) {
        Ok(import) => {
            report.inspected_one();
            if import.trim() != format!("@{CANONICAL}") {
                report.found(Finding::new(
                    "inexact-instruction-import",
                    IMPORT,
                    format!(
                        "is not exactly `@{CANONICAL}`; the exact import is the terminal state, reached after every statement it used to carry received a disposition"
                    ),
                ));
            }
        }
        Err(TreeError::NotFound) => {}
        Err(error) => return refuse(IMPORT, &error),
    }

    report
}

/// A read that failed on something other than absence, as a refusal.
///
/// One place for both surfaces, because a file this command could not open
/// leaves it with nothing to say about the spine or the import, and the two
/// ways a read fails oblige the same answer.
fn refuse(path: &str, error: &TreeError) -> Report {
    let reason = match error {
        TreeError::Unreadable(reason) => reason.as_str(),
        // Absence is answered before this is reached, so folding it in here
        // keeps the expression total without an arm nothing runs.
        _ => "holds no text",
    };
    Report::refused("governance-instructions", format!("{path}: {reason}"))
}

/// The five sections, present, in order, and uninterrupted.
fn spine(text: &str, report: &mut Report) {
    // Read from prose rather than from every line, so a document showing a
    // heading inside a fenced example is not credited with a section it is
    // quoting.
    let sections: Vec<&str> = markdown::prose_lines(text)
        .into_iter()
        .filter_map(|(_, line)| line.strip_prefix(LEVEL))
        .map(str::trim)
        .collect();

    for name in SPINE {
        if !sections.contains(&name) {
            report.found(Finding::new(
                "missing-spine-section",
                CANONICAL,
                format!("declares no `{name}` section; the spine is five sections in one order, and a repository missing one has moved the fact somewhere a reader of another repository will not look"),
            ));
        }
    }

    // Interruption is read from the sections that are actually there. A
    // missing one is already reported, and holding the rest to a position it
    // left empty would report the same absence twice.
    let mut highest: Option<usize> = None;
    for section in &sections {
        match SPINE.iter().position(|name| name == section) {
            Some(position) => {
                highest = Some(highest.map_or(position, |earlier: usize| earlier.max(position)));
            }
            None => {
                // A section written after the whole spine is the ordinary case
                // and says nothing. One written after part of it divides the
                // spine, and a spine section following it is the same fault
                // seen from the other side rather than a second one.
                let complete = highest.is_some_and(|position| position + 1 == SPINE.len());
                if highest.is_some() && !complete {
                    report.found(Finding::new(
                        "interrupted-instruction-spine",
                        CANONICAL,
                        format!("`{section}` is written between spine sections; repository-specific sections follow the spine rather than divide it"),
                    ));
                    break;
                }
            }
        }
    }

    // The section named is the one that jumped the queue: the first written
    // before a section the spine puts ahead of it. Naming the later one instead
    // would point a reader at the section that stayed where it belonged.
    let positions: Vec<(usize, &&str)> = sections
        .iter()
        .filter_map(|section| {
            SPINE
                .iter()
                .position(|name| name == section)
                .map(|position| (position, section))
        })
        .collect();
    if let Some((_, section)) = positions
        .iter()
        .enumerate()
        .find(|(index, (position, _))| {
            positions[*index + 1..]
                .iter()
                .any(|(later, _)| later < position)
        })
        .map(|(_, entry)| entry)
    {
        report.found(Finding::new(
            "misordered-spine-section",
            CANONICAL,
            format!("`{section}` is out of spine order"),
        ));
    }
}
