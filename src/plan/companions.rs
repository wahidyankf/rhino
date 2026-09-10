//! The technical directory's modules, and the index that orders them.
//!
//! A plan splits its technical document when it outgrows one file, and the
//! split is only useful if the pieces have an order a reader can follow. The
//! ordinals are that order made mechanical: contiguous from 001, one per
//! module, and listed by the index in the same sequence.

use super::documents::TECHNICAL;
use super::{Plan, is_kebab_case, refuse};
use crate::report::{Finding, Report};
use crate::runtime::{Tree, TreeError};
use std::collections::{BTreeMap, BTreeSet};

const INDEX: &str = "README.md";

pub fn inspect(tree: &dyn Tree, plan: &Plan, report: &mut Report) -> Result<(), Box<Report>> {
    let prefix = format!("{TECHNICAL}/");
    let modules: Vec<&String> = plan
        .held
        .iter()
        .filter(|held| held.starts_with(&prefix) && !held.ends_with(&format!("/{INDEX}")))
        .collect();
    if modules.is_empty() {
        return Ok(());
    }

    let index_path = format!("{}/{TECHNICAL}/{INDEX}", plan.path);
    let listed = match tree.read(&index_path) {
        Ok(text) => crate::markdown::sibling_links(&text),
        // The index's absence is the directory-map rule's to report, not this
        // family's. Reading no index is not the same as reading an empty one,
        // so nothing here is reported against a file nobody has.
        Err(TreeError::NotFound) => BTreeMap::new(),
        Err(error) => return Err(Box::new(refuse(&index_path, &error))),
    };

    let directory = format!("{}/{TECHNICAL}", plan.path);
    let mut ordinals: Vec<(usize, &str)> = Vec::new();
    let mut malformed = false;

    for held in &modules {
        let name = held.strip_prefix(&prefix).unwrap_or(held);
        let path = format!("{}/{held}", plan.path);
        match ordinal(name) {
            Some((value, digits, rest)) => {
                ordinals.push((value, digits));
                // Reported only where the ordinal is well formed. The name
                // after a malformed ordinal is not the name after an ordinal.
                let stem = rest.rsplit_once('.').map_or(rest, |(stem, _)| stem);
                if !is_kebab_case(stem) {
                    report.found(
                        Finding::new(
                            "PLAN-COMPANION-004",
                            &path,
                            "companion name after its ordinal is not lowercase hyphen-separated",
                        )
                        .at(1, 1)
                        .about(stem),
                    );
                }
            }
            None => {
                malformed = true;
                report.found(
                    Finding::new(
                        "PLAN-COMPANION-001",
                        &path,
                        "companion ordinal is not exactly three digits",
                    )
                    .at(1, 1)
                    .about(leading_digits(name).unwrap_or(name)),
                );
            }
        }

        if !listed.contains_key(name) {
            report.found(
                Finding::new(
                    "PLAN-COMPANION-005",
                    &path,
                    "companion exists but the entrypoint does not list it",
                )
                .at(1, 1)
                .about("-"),
            );
        }
    }

    // Contiguity and uniqueness are properties of the set, and a set holding a
    // malformed ordinal has neither in any meaningful sense: the contract's own
    // example of within-family suppression.
    if !malformed {
        sequence(&ordinals, &directory, report);
    }

    for (target, line) in &listed {
        if !modules
            .iter()
            .any(|held| held.strip_prefix(&prefix) == Some(target.as_str()))
        {
            report.found(
                Finding::new(
                    "PLAN-COMPANION-006",
                    &index_path,
                    "entrypoint lists a companion that does not exist",
                )
                .at(*line, 1)
                .about(target),
            );
        }
    }

    Ok(())
}

/// Duplicates first, then contiguity: a set whose ordinals repeat cannot run
/// from 001 without a gap, so reporting both would name one defect twice.
fn sequence(ordinals: &[(usize, &str)], directory: &str, report: &mut Report) {
    let mut duplicated: BTreeSet<&str> = BTreeSet::new();
    for (index, (value, digits)) in ordinals.iter().enumerate() {
        if ordinals[..index].iter().any(|(held, _)| held == value) {
            duplicated.insert(digits);
        }
    }
    if !duplicated.is_empty() {
        for digits in duplicated {
            report.found(
                Finding::new(
                    "PLAN-COMPANION-003",
                    directory,
                    "companion ordinal is used more than once",
                )
                .at(1, 1)
                .about(digits),
            );
        }
        return;
    }

    let mut values: Vec<usize> = ordinals.iter().map(|(value, _)| *value).collect();
    values.sort_unstable();
    if values
        .iter()
        .enumerate()
        .any(|(index, value)| *value != index + 1)
    {
        report.found(
            Finding::new(
                "PLAN-COMPANION-002",
                directory,
                "companion ordinals are not contiguous from 001",
            )
            .at(1, 1)
            .about("-"),
        );
    }
}

/// A three-digit ordinal, the digits as written, and what follows the hyphen.
fn ordinal(name: &str) -> Option<(usize, &str, &str)> {
    let (digits, rest) = name.split_at_checked(3)?;
    if !rest.starts_with('-') || !digits.chars().all(|c| c.is_ascii_digit()) {
        return None;
    }
    Some((digits.parse().ok()?, digits, &rest[1..]))
}

fn leading_digits(name: &str) -> Option<&str> {
    let count = name.chars().take_while(char::is_ascii_digit).count();
    (count > 0).then(|| &name[..count])
}
