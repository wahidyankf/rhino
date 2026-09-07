//! The scenario runner shared by all three adapters.
//!
//! It walks a scenario's expanded steps in order and stops at the first one that
//! does not produce a result, so a failure names the sentence that stopped it
//! rather than the scenario as a whole.

use crate::gherkin::{Corpus, Scenario};
use crate::registry::Binding;
use crate::steps::{self, StepOutcome};

/// Run one expansion of one scenario.
fn run_expansion(scenario: &Scenario, steps_of: &[crate::gherkin::Step]) -> Result<(), String> {
    // Every arm below is a failure today, because no step has an implementation
    // yet -- which is what makes this lint fire. The moment a step passes, the
    // closure starts returning `None` and the expectation stops being met,
    // which is the signal to delete this attribute rather than keep it.
    #[expect(
        clippy::unnecessary_find_map,
        reason = "temporary: every step fails until its validator is ported"
    )]
    let first_failure = steps_of
        .iter()
        .find_map(|step| match steps::dispatch(&step.text) {
            StepOutcome::Unimplemented(pattern) => Some(format!(
                "{}: {}: {} {} -- vocabulary entry `{}` has no implementation yet",
                scenario.feature, scenario.name, step.keyword, step.text, pattern
            )),
            StepOutcome::Undefined => Some(format!(
                "{}: {}: {} {} -- no vocabulary entry matches this sentence",
                scenario.feature, scenario.name, step.keyword, step.text
            )),
        });

    match first_failure {
        Some(reason) => Err(reason),
        None => Ok(()),
    }
}

/// Run every bound scenario and fail with the complete list of what did not
/// pass. Reporting all of them at once is deliberate: during a port the useful
/// question is how much is left, not which single scenario stopped first.
pub fn run_bound_scenarios(layer: &str, bindings: &[Binding]) {
    let corpus = Corpus::canonical();
    let mut failures: Vec<String> = Vec::new();
    let mut passed = 0usize;

    for (feature, name) in bindings {
        let scenario = corpus.find(feature, name).unwrap_or_else(|| {
            panic!("{layer}: bound scenario is not in the corpus: {feature} :: {name}")
        });

        for expansion in scenario.expansions() {
            match run_expansion(scenario, &expansion) {
                Ok(()) => passed += 1,
                Err(reason) => failures.push(reason),
            }
        }
    }

    assert!(
        failures.is_empty(),
        "{layer}: {} of {} expanded scenarios pass; {} do not:\n{}",
        passed,
        passed + failures.len(),
        failures.len(),
        failures.join("\n")
    );
}

/// Every sentence the corpus uses must be one an adapter could dispatch. A
/// sentence no entry matches is a scenario that would otherwise never run.
pub fn assert_every_step_is_in_the_vocabulary() {
    let corpus = Corpus::canonical();
    let mut undefined: Vec<String> = Vec::new();

    for scenario in &corpus.scenarios {
        for expansion in scenario.expansions() {
            for step in expansion {
                if steps::lookup(&step.text).is_none() {
                    undefined.push(format!(
                        "{}: {}: {} {}",
                        scenario.feature, scenario.name, step.keyword, step.text
                    ));
                }
            }
        }
    }

    undefined.sort();
    undefined.dedup();
    assert!(
        undefined.is_empty(),
        "{} step sentences are outside the vocabulary:\n{}",
        undefined.len(),
        undefined.join("\n")
    );
}
