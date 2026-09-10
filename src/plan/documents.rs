//! The six documents, and the one technical shape.
//!
//! Five documents are the same in every plan. The sixth is the technical one,
//! which is a file while it fits in a file and a directory once it does not --
//! and a plan carrying both, or neither, is a plan whose reader does not know
//! where to start.

use super::Plan;
use crate::report::{Finding, Report};

/// The five documents whose names never change.
const REQUIRED: [&str; 5] = [
    "README.md",
    "brd.md",
    "delivery.md",
    "learnings.md",
    "prd.md",
];

/// The technical document, in the two shapes it is allowed to take.
pub const TECHNICAL: &str = "tech-docs";

pub fn inspect(plan: &Plan, report: &mut Report) {
    for name in REQUIRED {
        if !plan.holds(name) {
            report.found(
                Finding::new(
                    "PLAN-DOCUMENT-001",
                    &plan.path,
                    "required plan document is missing",
                )
                .at(1, 1)
                .about(name),
            );
        }
    }

    let single = plan.holds(&format!("{TECHNICAL}.md"));
    let directory = plan.holds_directory(TECHNICAL);
    match (single, directory) {
        (true, true) => {
            report.found(
                Finding::new(
                    "PLAN-DOCUMENT-002",
                    &plan.path,
                    "plan carries both technical shapes",
                )
                .at(1, 1)
                .about(TECHNICAL),
            );
        }
        (false, false) => {
            report.found(
                Finding::new(
                    "PLAN-DOCUMENT-003",
                    &plan.path,
                    "plan carries no technical shape",
                )
                .at(1, 1)
                .about(TECHNICAL),
            );
        }
        _ => {}
    }
}
