//! Where a plan is in its life, read from the root it sits under and the shape
//! of its slug.
//!
//! The two are one rule family because they answer one question together. A
//! date on the front of a slug means the plan finished; the root it sits under
//! means the same thing; and a plan whose two answers disagree is the defect
//! this family exists to catch.

use super::{DONE, Plan, ROOTS, is_kebab_case, shared_slugs};
use crate::report::{Finding, Report};
use std::collections::BTreeSet;

/// The prefix a completed plan's slug carries: a date, then two underscores.
const SEPARATOR: &str = "__";

/// Report every lifecycle root outside the four canonical names, and answer
/// with the set, because no later rule in this family can be evaluated for a
/// plan sitting under one.
pub fn unknown_roots(files: &[String], prefix: &str, report: &mut Report) -> BTreeSet<String> {
    let mut unknown = BTreeSet::new();
    for (root, _) in files
        .iter()
        .filter_map(|path| path.strip_prefix(prefix))
        .filter_map(|rest| rest.split_once('/'))
    {
        if ROOTS.contains(&root) || !unknown.insert(root.to_string()) {
            continue;
        }
        report.found(
            Finding::new(
                "PLAN-LIFECYCLE-001",
                format!("{prefix}{root}"),
                "plan root is not one of ideas, backlog, in-progress, done",
            )
            .at(1, 1)
            .about(root),
        );
    }
    unknown
}

pub fn inspect(plan: &Plan, files: &[String], prefix: &str, report: &mut Report) {
    let completed = completed(&plan.slug);
    let live = plan.root != DONE;

    // Ordered by identifier, and each later rule asks a question the earlier
    // one has already answered no to. A slug whose date prefix is wrong for its
    // root is not then also reported for the hyphens inside it: that would make
    // one defect look like two, and the author would fix the wrong one first.
    if live && opens_with_a_date(&plan.slug) {
        report.found(
            Finding::new(
                "PLAN-LIFECYCLE-002",
                &plan.path,
                "live plan slug carries a date",
            )
            .at(1, 1)
            .about(&plan.slug),
        );
        return;
    }
    if !live && completed.is_none() {
        report.found(
            Finding::new(
                "PLAN-LIFECYCLE-003",
                &plan.path,
                "done plan slug has no YYYY-MM-DD__ prefix",
            )
            .at(1, 1)
            .about(&plan.slug),
        );
        return;
    }

    // The name half of a completed slug, which is the whole of a live one.
    let name = completed.unwrap_or(plan.slug.as_str());
    if !is_kebab_case(name) {
        report.found(
            Finding::new(
                "PLAN-LIFECYCLE-004",
                &plan.path,
                "plan slug is not lowercase hyphen-separated",
            )
            .at(1, 1)
            .about(&plan.slug),
        );
        return;
    }

    // Reported against the second root the slug appears under, in path order,
    // so a repository that moved a plan and left the original behind is told
    // about the copy rather than about the plan it moved.
    if shared_slugs(files, prefix).contains(&plan.slug)
        && first_root(files, prefix, &plan.slug) != Some(plan.root.clone())
    {
        report.found(
            Finding::new(
                "PLAN-LIFECYCLE-005",
                &plan.path,
                "plan slug occupies more than one lifecycle root",
            )
            .at(1, 1)
            .about(&plan.slug),
        );
    }
}

/// The name half of a completed slug: a date, then two underscores.
fn completed(slug: &str) -> Option<&str> {
    let (date, name) = slug.split_once(SEPARATOR)?;
    is_a_date(date).then_some(name)
}

/// Whether a slug opens with a date at all.
///
/// Deliberately looser than the completed form. A live slug carrying
/// `2026-01-15-` is claiming a completion date just as plainly as one carrying
/// `2026-01-15__`, and reading only the second would let the first through on a
/// separator.
fn opens_with_a_date(slug: &str) -> bool {
    slug.split_at_checked(10)
        .is_some_and(|(date, _)| is_a_date(date))
}

fn is_a_date(text: &str) -> bool {
    text.len() == 10
        && text
            .chars()
            .enumerate()
            .all(|(index, character)| match index {
                4 | 7 => character == '-',
                _ => character.is_ascii_digit(),
            })
}

/// The first lifecycle root, in path order, that holds one slug.
fn first_root(files: &[String], prefix: &str, slug: &str) -> Option<String> {
    super::entries(files, prefix)
        .find(|(_, held, _)| *held == slug)
        .map(|(root, _, _)| root.to_string())
}
