//! Acceptance identifiers, defined once and cited only where defined.
//!
//! The identifier is the only thread between what a plan promised and what its
//! checklist claims to have delivered. A duplicate definition makes the thread
//! ambiguous; a citation with no definition makes it break. Neither is a
//! judgement about the criterion, which is why both are checkable.

use super::{Plan, refuse};
use crate::report::{Finding, Report};
use crate::runtime::{Tree, TreeError};
use std::collections::BTreeSet;

const DEFINITIONS: &str = "prd.md";
const CITATIONS: &str = "delivery.md";

pub fn inspect(
    tree: &dyn Tree,
    plan: &Plan,
    checklist: Option<&str>,
    report: &mut Report,
) -> Result<(), Box<Report>> {
    let requirements = format!("{}/{DEFINITIONS}", plan.path);
    // Read line by line rather than as prose: the criteria live inside a
    // fenced Gherkin block, and a reader that skipped fences would find a plan
    // that defines nothing and report every citation as undefined.
    let defined = match tree.read(&requirements) {
        Ok(text) => {
            let mut seen: BTreeSet<String> = BTreeSet::new();
            for (index, line) in text.lines().enumerate() {
                if !line.trim_start().starts_with("Scenario:") {
                    continue;
                }
                for identifier in identifiers(line) {
                    if !seen.insert(identifier.clone()) {
                        report.found(
                            Finding::new(
                                "PLAN-CRITERION-001",
                                &requirements,
                                "acceptance identifier is defined more than once",
                            )
                            .at(index + 1, 1)
                            .about(identifier),
                        );
                    }
                }
            }
            seen
        }
        Err(TreeError::NotFound) => return Ok(()),
        Err(error) => return Err(Box::new(refuse(&requirements, &error))),
    };

    let Some(text) = checklist else {
        return Ok(());
    };
    let path = format!("{}/{CITATIONS}", plan.path);
    for (number, line) in crate::markdown::prose_lines(text) {
        for identifier in identifiers(line) {
            if !defined.contains(&identifier) {
                report.found(
                    Finding::new(
                        "PLAN-CRITERION-002",
                        &path,
                        "delivery item cites an undefined acceptance identifier",
                    )
                    .at(number, 1)
                    .about(identifier),
                );
            }
        }
    }

    Ok(())
}

/// Every bracketed acceptance identifier on one line.
///
/// Shaped rather than named: uppercase letters, a hyphen, digits. That is what
/// separates `[AC-01]` from `[AI]`, which shares the brackets and means
/// something else entirely one line above it.
fn identifiers(line: &str) -> Vec<String> {
    let mut found = Vec::new();
    let mut rest = line;
    while let Some((_, after)) = rest.split_once('[') {
        // A `[` that nothing closes names nothing and ends the line, read as an
        // empty pair rather than as an arm no document reaches.
        let (candidate, remainder) = after.split_once(']').unwrap_or(("", ""));
        if let Some((letters, digits)) = candidate.split_once('-')
            && !letters.is_empty()
            && letters.chars().all(|c| c.is_ascii_uppercase())
            && !digits.is_empty()
            && digits.chars().all(|c| c.is_ascii_digit())
        {
            found.push(candidate.to_string());
        }
        rest = remainder;
    }
    found
}
