//! The end-to-end adapter.
//!
//! The same sentences again, observed only through the built executable's
//! process contract: arguments in; stdout, stderr, and an exit code out. It
//! reaches for nothing else, which is what makes it an end-to-end test rather
//! than an integration test with a bigger budget.
#![forbid(unsafe_code)]

// The support modules are shared with the other adapters and with the static
// coverage check. This one uses part of each; the unused remainder belongs to
// another adapter rather than to nobody.
#[allow(dead_code)]
#[path = "../support/gherkin.rs"]
mod gherkin;
#[allow(dead_code)]
#[path = "../support/registry.rs"]
mod registry;
#[allow(dead_code)]
#[path = "../support/runner.rs"]
mod runner;
#[path = "../support/steps.rs"]
mod steps;

#[allow(dead_code)]
mod bindings;

#[test]
fn the_corpus_passes_at_the_process_boundary() {
    runner::run_bound_scenarios("e2e", bindings::BINDINGS);
}
