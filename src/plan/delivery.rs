//! The delivery checklist: who executes each item, and what order the phases
//! run in.
//!
//! Three of these rules are about a label and a heading. The fourth is about
//! sequence: archival is the last thing a plan does, and a checklist that puts
//! it first describes a plan that archives itself before it does the work.

use super::Plan;
use crate::report::{Finding, Report};

pub const CHECKLIST: &str = "delivery.md";

/// The two executors a checklist item may name.
const EXECUTORS: [&str; 2] = ["AI", "HUMAN"];

/// The heading that opens the archival section, which carries no phase number
/// because it is not a phase.
const ARCHIVAL: &str = "Plan Archival";

const HEADING: &str = "## ";

pub fn inspect(plan: &Plan, checklist: Option<&str>, report: &mut Report) {
    let Some(text) = checklist else {
        return;
    };
    let path = format!("{}/{CHECKLIST}", plan.path);

    let mut substantive = 0usize;
    for (number, line) in crate::markdown::prose_lines(text) {
        if let Some(heading) = line.strip_prefix(HEADING) {
            let heading = heading.trim();
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
            continue;
        }

        let Some(item) = line.strip_prefix("- ") else {
            continue;
        };
        match label(item) {
            None => {
                report.found(
                    Finding::new(
                        "PLAN-DELIVERY-001",
                        &path,
                        "checklist item carries no executor label",
                    )
                    .at(number, 1)
                    .about("-"),
                );
            }
            Some(label) if !EXECUTORS.contains(&label) => {
                report.found(
                    Finding::new(
                        "PLAN-DELIVERY-002",
                        &path,
                        "executor label is not AI or HUMAN",
                    )
                    .at(number, 1)
                    .about(label),
                );
            }
            Some(_) => {}
        }
    }
}

/// Whether a phase heading carries a number, in `Phase <n>` form.
fn numbered(heading: &str) -> bool {
    let Some(rest) = heading.strip_prefix("Phase ") else {
        return false;
    };
    let digits: String = rest.chars().take_while(char::is_ascii_digit).collect();
    !digits.is_empty()
}

/// The bracketed label a checklist item opens with, when it opens with one.
fn label(item: &str) -> Option<&str> {
    let rest = item.strip_prefix('[')?;
    let (label, _) = rest.split_once(']')?;
    Some(label)
}
