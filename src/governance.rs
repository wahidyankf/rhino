//! Governance validators.
//!
//! These enforce how a repository's own documents are organised: how long a
//! governed file may be, whether every directory in a mapped tree documents its
//! own contents, which categories the governance tree may hold, how a document
//! that outgrew its budget splits, and what the canonical instruction opens
//! with.

pub mod companion;
pub mod directory_map;
pub mod instructions;
pub mod structure;
pub mod word_budget;
