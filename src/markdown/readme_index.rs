//! README-index validation.
//!
//! Every directory under a declared tree carries a `README.md`. Presence only.
//!
//! This is deliberately weaker than [`crate::governance::directory_map`], which
//! wants the README *and* a section listing every direct sibling. Both exist so
//! a repository can say which rung it is on: one with a hundred READMEs and no
//! map sections can adopt this today and the stronger rule when it has written
//! them, instead of choosing between a hundred findings and no check at all.

use crate::config::{Config, ReadmeIndex};
use crate::report::{Finding, Report};
use crate::runtime::Tree;
use crate::scan::{self, README};

pub fn validate(tree: &dyn Tree, config: &Config, index: &ReadmeIndex) -> Report {
    let mut report = Report::new("readme-index", "directory");

    for declared in &index.trees {
        for directory in scan::directories(tree, config, &declared.path) {
            report.inspected_one();
            // Existence, not readability. A README that is there and cannot be
            // opened is present, and telling a maintainer to write a file that
            // already exists would be the wrong instruction.
            if !tree.exists(&format!("{directory}/{README}")) {
                report.found(Finding::new(
                    "missing-readme-index",
                    directory,
                    "missing README: every directory under a declared tree carries one",
                ));
            }
        }
    }

    report
}
