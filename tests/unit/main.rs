//! The unit adapter.
//!
//! Drives the library against an in-memory tree through the filesystem port, so
//! every scenario in the corpus is proved at the cheapest boundary that can
//! express it. There are no exemptions here and there never will be.
#![forbid(unsafe_code)]

// The support modules are shared with the other adapters and with the static
// coverage check. This one uses part of each; the unused remainder belongs to
// another adapter rather than to nobody.
#[allow(dead_code)]
#[path = "../support/binding.rs"]
mod binding;
#[allow(dead_code)]
#[path = "../support/fixtures.rs"]
mod fixtures;
#[allow(dead_code)]
#[path = "../support/gherkin.rs"]
mod gherkin;
#[allow(dead_code)]
#[path = "../support/harness.rs"]
mod harness;
#[allow(dead_code)]
#[path = "../support/mermaid.rs"]
mod mermaid;
#[allow(dead_code)]
#[path = "../support/registry.rs"]
mod registry;
#[allow(dead_code)]
#[path = "../support/runner.rs"]
mod runner;
#[allow(dead_code)]
#[path = "../support/steps.rs"]
mod steps;
#[allow(dead_code)]
#[path = "../support/world.rs"]
mod world;

#[allow(dead_code)]
mod bindings;
mod driver;

#[test]
fn the_corpus_passes_at_the_unit_boundary() {
    runner::run_bound_scenarios::<driver::UnitDriver>("unit", bindings::BINDINGS);
}
