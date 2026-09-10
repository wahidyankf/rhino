//! Plan structure.
//!
//! A formal plan is six documents and exactly one technical shape, kept under a
//! lifecycle root whose name and slug form say where the plan is in its life.
//! More than one implementation validates that shape, so the rules here are not
//! this crate's opinion: identifiers, messages, diagnostics, exit classes, and
//! the fixture corpus are frozen in a shared contract, and a rule whose meaning
//! changes gets a new identifier rather than a new definition.
//!
//! What is deliberately absent is judgement. Whether the writing is clear,
//! whether the approach is sound, whether a checklist item is granular enough:
//! none of that is visible in the shape, and a rule that guessed at it would be
//! enforced confidently in cases nobody considered.

mod companions;
mod criteria;
mod delivery;
mod documents;
mod lifecycle;

use crate::report::Report;
use crate::runtime::{Tree, TreeError};
use std::collections::BTreeSet;

pub const CATEGORY: &str = "plan";

/// The tree a plan lives under. Fixed rather than configured: the contract
/// names it, and a repository that could point this elsewhere would be running
/// a different contract under the same rule identifiers.
pub const ROOT: &str = "plans";

/// The four lifecycle roots, in the order a plan passes through them.
pub const ROOTS: [&str; 4] = ["ideas", "backlog", "in-progress", "done"];

/// The root whose contents are history.
const DONE: &str = "done";

/// The root that holds briefs rather than plans.
const IDEAS: &str = "ideas";

/// One plan directory, with the files it holds anywhere beneath it.
pub struct Plan {
    /// Repository-relative, `plans/<root>/<slug>`.
    pub path: String,
    pub root: String,
    pub slug: String,
    /// Paths relative to `path`, so a rule reads `tech-docs/001-context.md`
    /// rather than repeating the plan it is already inside.
    pub held: Vec<String>,
}

impl Plan {
    pub fn holds(&self, relative: &str) -> bool {
        self.held.iter().any(|path| path == relative)
    }

    pub fn holds_directory(&self, relative: &str) -> bool {
        let prefix = format!("{relative}/");
        self.held.iter().any(|path| path.starts_with(&prefix))
    }
}

pub fn validate(tree: &dyn Tree) -> Report {
    let mut report = Report::new(CATEGORY, "plan");
    let files = tree.files();
    let prefix = format!("{ROOT}/");

    // Unknown roots first. Every later lifecycle rule asks where in its life a
    // plan is, and under a root nobody recognises there is no answer to give.
    let unknown = lifecycle::unknown_roots(&files, &prefix, &mut report);

    for plan in discover(&files, &prefix) {
        report.inspected_one();
        let settled = !unknown.contains(&plan.root);
        if settled {
            lifecycle::inspect(&plan, &files, &prefix, &mut report);
        }

        // An archived plan is held to its lifecycle form and nothing else. Its
        // contents are the record of work that finished, and rewriting history
        // to satisfy a rule introduced later would destroy what the root is
        // for. A brief under `ideas` is not a formal plan at all.
        if plan.root == DONE || plan.root == IDEAS {
            continue;
        }

        documents::inspect(&plan, &mut report);
        if let Err(refusal) = companions::inspect(tree, &plan, &mut report) {
            return *refusal;
        }
        // Read once and handed to both rule families. The checklist is the one
        // document two families ask about, and reading it twice would let a
        // file that changed between the reads be validated as two documents.
        let checklist = match read(tree, &plan, delivery::CHECKLIST) {
            Ok(text) => text,
            Err(refusal) => return *refusal,
        };
        if let Err(refusal) = criteria::inspect(tree, &plan, checklist.as_deref(), &mut report) {
            return *refusal;
        }
        delivery::inspect(&plan, checklist.as_deref(), &mut report);
    }

    report
}

/// Every plan directory, in path order.
///
/// A plan is a directory two levels under the root. A file at that depth is a
/// brief rather than a plan, which is why `ideas` holds documents and the other
/// three hold directories.
fn discover(files: &[String], prefix: &str) -> Vec<Plan> {
    let mut plans: Vec<Plan> = Vec::new();
    for (root, slug, relative) in entries(files, prefix) {
        let owner = format!("{prefix}{root}/{slug}");
        match plans.iter_mut().find(|plan| plan.path == owner) {
            Some(plan) => plan.held.push(relative.to_string()),
            None => plans.push(Plan {
                path: owner,
                root: root.to_string(),
                slug: slug.to_string(),
                held: vec![relative.to_string()],
            }),
        }
    }
    for plan in &mut plans {
        plan.held.sort();
    }
    plans
}

/// Every file under the plan root, split into the root, the slug, and the path
/// within the plan.
///
/// One place, because three callers ask the same question of the same paths and
/// three copies of the split would disagree the first time one was corrected.
pub fn entries<'a>(
    files: &'a [String],
    prefix: &str,
) -> impl Iterator<Item = (&'a str, &'a str, &'a str)> {
    files.iter().filter_map({
        let prefix = prefix.to_string();
        move |path| {
            let rest = path.strip_prefix(&prefix)?;
            let mut segments = rest.splitn(3, '/');
            Some((segments.next()?, segments.next()?, segments.next()?))
        }
    })
}

/// One of a plan's documents, or nothing where the document rules have already
/// said it is missing.
pub fn read(tree: &dyn Tree, plan: &Plan, name: &str) -> Result<Option<String>, Box<Report>> {
    let path = format!("{}/{name}", plan.path);
    match tree.read(&path) {
        Ok(text) => Ok(Some(text)),
        Err(TreeError::NotFound) => Ok(None),
        Err(error) => Err(Box::new(refuse(&path, &error))),
    }
}

/// A read that failed on something other than absence, as a refusal.
///
/// Fail closed. A plan document this command could not open leaves it reporting
/// on a plan it never read, and every way that read can fail obliges the same
/// answer.
pub fn refuse(path: &str, error: &TreeError) -> Report {
    let reason = match error {
        TreeError::Unreadable(reason) => reason.as_str(),
        // Absence is answered by the document rules before this is reached, so
        // folding it in keeps the expression total without an arm nothing runs.
        _ => "holds no text",
    };
    Report::refused(CATEGORY, format!("{path}: {reason}"))
}

/// Whether a name is lowercase, hyphen-separated, and separated by single
/// hyphens.
pub fn is_kebab_case(name: &str) -> bool {
    !name.is_empty()
        && !name.starts_with('-')
        && !name.ends_with('-')
        && !name.contains("--")
        && name.chars().all(|character| {
            character.is_ascii_lowercase() || character.is_ascii_digit() || character == '-'
        })
}

/// The set of plan slugs that appear under more than one lifecycle root.
pub fn shared_slugs(files: &[String], prefix: &str) -> BTreeSet<String> {
    let mut seen: Vec<(String, String)> = Vec::new();
    for (root, slug, _) in entries(files, prefix) {
        let pair = (root.to_string(), slug.to_string());
        if !seen.contains(&pair) {
            seen.push(pair);
        }
    }
    let mut shared = BTreeSet::new();
    for (index, (_, slug)) in seen.iter().enumerate() {
        if seen[..index].iter().any(|(_, held)| held == slug) {
            shared.insert(slug.clone());
        }
    }
    shared
}
