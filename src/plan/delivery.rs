//! The delivery checklist: who executes each item, and what order the phases
//! run in.
//!
//! Three of these rules are about a label and a heading. The fourth is about
//! sequence: archival is the last thing a plan does, and a checklist that puts
//! it first describes a plan that archives itself before it does the work.
//!
//! Reading them requires deciding two things first, because `delivery.md` is a
//! document and not a table. Which of its bullets are checklist items, and
//! which of its sections are phases. The contract requires the document to
//! declare its execution checkout, delivery units and pause safety *before* its
//! phases, so a delivery document legitimately contains sections that are not
//! phases and bullets that are not items. Treating every bullet as an item, or
//! every heading as a phase, reports a defect against every well-formed plan.

use super::Plan;
use crate::report::{Finding, Report};

pub const CHECKLIST: &str = "delivery.md";

/// The two executors a checklist item may name.
const EXECUTORS: [&str; 2] = ["AI", "HUMAN"];

/// The heading that opens the archival section, which carries no phase number
/// because it is not a phase.
const ARCHIVAL: &str = "Plan Archival";

/// The prefix a phase heading carries when it is numbered, and the marker that
/// identifies a heading as a phase even before its items are read.
const PHASE: &str = "Phase ";

const HEADING: &str = "## ";

/// What a bullet in `delivery.md` turned out to be.
enum Bullet<'a> {
    /// A markdown link. Never an item: a list of links is a list of documents,
    /// and reading the link text as an executor's name is how a Related
    /// Documents section becomes a wall of findings.
    Link,
    /// A bullet naming neither a checkbox nor a label. Whether this is an item
    /// missing its label or ordinary prose in list form depends on the section
    /// it sits in, which the bullet itself cannot see.
    Bare,
    /// A checklist item whose checkbox is present and whose label is not.
    Unlabelled,
    /// A checklist item and the label it names.
    Labelled(&'a str),
}

/// One `## ` section of the checklist, buffered until its kind is known.
struct Section<'a> {
    heading: Option<(usize, &'a str)>,
    bullets: Vec<(usize, Bullet<'a>)>,
}

impl<'a> Section<'a> {
    fn new(heading: Option<(usize, &'a str)>) -> Self {
        Self {
            heading,
            bullets: Vec::new(),
        }
    }

    /// Whether this section holds anything that is recognisably a checklist
    /// item. A bare bullet does not count: it is the very thing whose meaning
    /// depends on the answer, so letting it decide would be circular.
    fn holds_items(&self) -> bool {
        self.bullets
            .iter()
            .any(|(_, bullet)| matches!(bullet, Bullet::Unlabelled | Bullet::Labelled(_)))
    }
}

pub fn inspect(plan: &Plan, checklist: Option<&str>, report: &mut Report) {
    let Some(text) = checklist else {
        return;
    };
    let path = format!("{}/{CHECKLIST}", plan.path);

    let mut sections: Vec<Section> = Vec::new();
    let mut current = Section::new(None);

    for (number, line) in crate::markdown::prose_lines(text) {
        if let Some(heading) = line.strip_prefix(HEADING) {
            sections.push(std::mem::replace(
                &mut current,
                Section::new(Some((number, heading.trim()))),
            ));
            continue;
        }
        if let Some(item) = line.strip_prefix("- ") {
            current.bullets.push((number, classify(item)));
        }
    }
    sections.push(current);

    let mut substantive = 0usize;
    for section in &sections {
        // Anything before the first heading is the document's own preamble.
        let Some((number, heading)) = section.heading else {
            continue;
        };

        if heading == ARCHIVAL {
            // Reported at the archival heading rather than at the phase it
            // preceded: the heading is what moved, and the phases below it
            // are where they always were.
            if substantive == 0 {
                report.found(
                    Finding::new(
                        "PLAN-DELIVERY-004",
                        &path,
                        "archival items appear before a substantive phase",
                    )
                    .at(number, 1)
                    .about("-"),
                );
            }
            report_items(section, &path, report);
            continue;
        }

        // A phase either says so in its heading or proves it by holding items.
        // A section that does neither is one of the declarations the contract
        // requires before the phases, and carries no phase number because it
        // is not a phase.
        if !heading.starts_with(PHASE) && !section.holds_items() {
            continue;
        }

        substantive += 1;
        if !numbered(heading) {
            report.found(
                Finding::new(
                    "PLAN-DELIVERY-003",
                    &path,
                    "delivery phase heading carries no phase number",
                )
                .at(number, 1)
                .about(heading),
            );
        }
        report_items(section, &path, report);
    }
}

/// Report the label rules against one section's items. This runs only for a
/// section already decided to be a phase, which is what makes a bare bullet
/// here an item missing its label rather than prose.
fn report_items(section: &Section, path: &str, report: &mut Report) {
    for (number, bullet) in &section.bullets {
        match bullet {
            Bullet::Link => {}
            Bullet::Bare | Bullet::Unlabelled => {
                report.found(
                    Finding::new(
                        "PLAN-DELIVERY-001",
                        path,
                        "checklist item carries no executor label",
                    )
                    .at(*number, 1)
                    .about("-"),
                );
            }
            Bullet::Labelled(label) if !EXECUTORS.contains(label) => {
                report.found(
                    Finding::new(
                        "PLAN-DELIVERY-002",
                        path,
                        "executor label is not AI or HUMAN",
                    )
                    .at(*number, 1)
                    .about(*label),
                );
            }
            Bullet::Labelled(_) => {}
        }
    }
}

/// Whether a phase heading carries a number, in `Phase <n>` form.
fn numbered(heading: &str) -> bool {
    let Some(rest) = heading.strip_prefix(PHASE) else {
        return false;
    };
    let digits: String = rest.chars().take_while(char::is_ascii_digit).collect();
    !digits.is_empty()
}

/// Read one bullet: strip an optional task marker, then the executor label.
///
/// Both spellings are items. `- [AI] ...` names its executor directly;
/// `- [ ] [AI] ...` carries a task-list checkbox first, which is what an
/// executor ticks as the item resolves, and the label may be code-formatted.
///
/// A code-formatted label without a checkbox is prose, not an item. Plans
/// explain their own notation -- "`[AI]` means the executor may act" -- and a
/// reader that took that sentence for an item would report the document's
/// glossary as a delivery phase.
fn classify(item: &str) -> Bullet<'_> {
    let (marked, rest) = match marker(item) {
        Some(rest) => (true, rest),
        None => (false, item),
    };
    let (quoted, rest) = match rest.strip_prefix('`') {
        Some(rest) => (true, rest),
        None => (false, rest),
    };
    if quoted && !marked {
        return Bullet::Bare;
    }

    match label(rest) {
        Some(Label::Link) => Bullet::Link,
        Some(Label::Named(label)) => Bullet::Labelled(label),
        None if marked => Bullet::Unlabelled,
        None => Bullet::Bare,
    }
}

/// What the leading bracket group on a bullet turned out to be.
enum Label<'a> {
    Link,
    Named(&'a str),
}

/// The task-list checkbox a checklist item may open with, and what follows it.
fn marker(item: &str) -> Option<&str> {
    for open in ["[ ] ", "[x] ", "[X] "] {
        if let Some(rest) = item.strip_prefix(open) {
            return Some(rest);
        }
    }
    None
}

/// The bracketed label a checklist item opens with, when it opens with one.
///
/// A bracket followed by a parenthesis is a markdown link, not a label: the
/// link text would otherwise be read as the executor's name.
fn label(item: &str) -> Option<Label<'_>> {
    let rest = item.strip_prefix('[')?;
    let (label, after) = rest.split_once(']')?;
    if after.starts_with('(') {
        return Some(Label::Link);
    }
    Some(Label::Named(label))
}
