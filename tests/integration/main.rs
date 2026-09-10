//! The integration adapter.
//!
//! The same sentences driven against a real temporary directory, so the
//! concrete filesystem implementation behind the port is exercised rather than
//! its in-memory double. Never runs in a Git hook or in the quick gate.
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
#[path = "../support/launcher.rs"]
mod launcher;
#[allow(dead_code)]
#[path = "../support/mermaid.rs"]
mod mermaid;
#[allow(dead_code)]
#[path = "../support/plan.rs"]
mod plan;
#[allow(dead_code)]
#[path = "../support/registry.rs"]
mod registry;
#[allow(dead_code)]
#[path = "../support/runner.rs"]
mod runner;
#[allow(dead_code)]
#[path = "../support/sandbox.rs"]
mod sandbox;
#[allow(dead_code)]
#[path = "../support/steps.rs"]
mod steps;
#[allow(dead_code)]
#[path = "../support/world.rs"]
mod world;

#[allow(dead_code)]
mod bindings;
mod driver;
mod policy;

#[test]
fn the_corpus_passes_at_the_integration_boundary() {
    runner::run_bound_scenarios::<driver::IntegrationDriver>("integration", bindings::BINDINGS);
}
