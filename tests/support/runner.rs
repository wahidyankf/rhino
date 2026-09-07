//! The scenario runner shared by all three adapters.
//!
//! It walks a scenario's expanded steps in order and stops at the first one that
//! does not produce a result, so a failure names the sentence that stopped it
//! rather than the scenario as a whole.

use crate::binding::{self, Outcome};
use crate::gherkin::{Corpus, Scenario, Step};
use crate::registry::Binding;
use crate::world::{Driver, World};

/// Run one expansion of one scenario in a world of its own.
fn run_expansion<D: Driver + Default>(
    scenario: &Scenario,
    steps_of: &[Step],
) -> Result<(), String> {
    let mut world: World<D> = World::default();

    for step in steps_of {
        let outcome = binding::run_step(&mut world, step);
        let reason = match outcome {
            Outcome::Passed => continue,
            Outcome::Failed(reason) => reason,
            Outcome::Unimplemented(pattern) => {
                format!("vocabulary entry `{pattern}` has no implementation yet")
            }
            Outcome::Undefined => "no vocabulary entry matches this sentence".to_string(),
        };
        return Err(format!(
            "{}: {}: {} {} -- {reason}",
            scenario.feature, scenario.name, step.keyword, step.text
        ));
    }

    Ok(())
}

/// Run every bound scenario and fail with the complete list of what did not
/// pass. Reporting all of them at once is deliberate: during a port the useful
/// question is how much is left, not which single scenario stopped first.
pub fn run_bound_scenarios<D: Driver + Default>(layer: &str, bindings: &[Binding]) {
    let corpus = Corpus::canonical();
    let mut failures: Vec<String> = Vec::new();
    let mut passed = 0usize;

    for (feature, name) in bindings {
        let scenario = corpus.find(feature, name).unwrap_or_else(|| {
            panic!("{layer}: bound scenario is not in the corpus: {feature} :: {name}")
        });

        for expansion in scenario.expansions() {
            match run_expansion::<D>(scenario, &expansion) {
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
