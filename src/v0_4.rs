//! Internal v0.4 boundaries.
//!
//! Phase 2 introduces names and dependency seams only. No command dispatch or
//! product behavior is routed through these modules until the corresponding
//! RED contracts have an owning GREEN phase.

pub(crate) mod config;
pub(crate) mod gates;
pub(crate) mod harnesses;
pub(crate) mod operations;
mod output;
mod release;
pub(crate) mod validators;
