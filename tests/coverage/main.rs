//! Static behaviour coverage.
//!
//! This adapter executes no scenario. It reads the corpus, reads each adapter's
//! declared bindings, and asserts the mapping between them: every scenario bound
//! exactly once at unit, and at integration and E2E unless a valid exemption
//! names the boundary and its alternative proof.
//!
//! It belongs in the quick gate because it is fast and because a missing binding
//! is exactly the kind of defect that otherwise ships quietly -- a scenario that
//! is never run reports nothing, and reporting nothing looks like passing.
#![forbid(unsafe_code)]

// The support modules are shared with the three executing adapters. This one
// reads the corpus and the registries and runs nothing, so it uses only part of
// each -- the unused remainder is another adapter's, not dead.
#[allow(dead_code)]
#[path = "../support/gherkin.rs"]
mod gherkin;
#[allow(dead_code)]
#[path = "../support/registry.rs"]
mod registry;

#[path = "../e2e/bindings.rs"]
mod e2e_bindings;
#[path = "../integration/bindings.rs"]
mod integration_bindings;
#[path = "../unit/bindings.rs"]
mod unit_bindings;

use gherkin::Corpus;
use std::collections::BTreeSet;

fn declared() -> BTreeSet<(String, String)> {
    Corpus::canonical().keys().into_iter().collect()
}

#[test]
fn the_corpus_is_non_empty_and_unambiguous() {
    let corpus = Corpus::canonical();
    assert!(
        !corpus.scenarios.is_empty(),
        "the corpus is empty; a coverage check over nothing would pass vacuously"
    );
    assert_eq!(
        corpus.duplicates(),
        Vec::<(String, String)>::new(),
        "duplicate scenario names make a binding ambiguous"
    );
}

#[test]
fn every_scenario_has_exactly_one_unit_binding() {
    let declared = declared();
    let bound = registry::bound_keys(unit_bindings::BINDINGS);

    let missing: Vec<_> = declared.difference(&bound).cloned().collect();
    let unused: Vec<_> = bound.difference(&declared).cloned().collect();

    assert!(
        missing.is_empty(),
        "scenarios with no unit binding: {missing:?}"
    );
    assert!(
        unused.is_empty(),
        "unit bindings with no scenario: {unused:?}"
    );
    assert_eq!(
        unit_bindings::BINDINGS.len(),
        bound.len(),
        "a scenario is bound more than once at unit"
    );
}

#[test]
fn unit_declares_no_exemption() {
    // Unit is never exempt. There is no boundary a unit test cannot reach that
    // would still leave a scenario worth having.
    assert_eq!(
        unit_bindings::EXEMPTIONS.len(),
        0,
        "unit exemptions are not permitted"
    );
}

#[test]
fn integration_and_e2e_are_bound_or_validly_exempt() {
    let declared = declared();

    for (layer, bindings, exemptions) in [
        (
            "integration",
            integration_bindings::BINDINGS,
            integration_bindings::EXEMPTIONS,
        ),
        ("e2e", e2e_bindings::BINDINGS, e2e_bindings::EXEMPTIONS),
    ] {
        let bound = registry::bound_keys(bindings);
        let exempt = registry::exempt_keys(exemptions);

        let overlap: Vec<_> = bound.intersection(&exempt).cloned().collect();
        assert!(
            overlap.is_empty(),
            "{layer}: a scenario is both bound and exempt: {overlap:?}"
        );

        let covered: BTreeSet<_> = bound.union(&exempt).cloned().collect();
        let missing: Vec<_> = declared.difference(&covered).cloned().collect();
        assert!(
            missing.is_empty(),
            "{layer}: scenarios neither bound nor exempt: {missing:?}"
        );

        let unknown: Vec<_> = covered.difference(&declared).cloned().collect();
        assert!(
            unknown.is_empty(),
            "{layer}: bindings or exemptions with no scenario: {unknown:?}"
        );
    }
}

#[test]
fn every_exemption_names_a_boundary_and_alternative_proof() {
    // The reason has to be a boundary the layer genuinely cannot reach. These
    // are the words that show up when it is not.
    const FORBIDDEN: [&str; 6] = ["difficult", "slow", "flaky", "expensive", "cost", "time"];

    for (layer, exemptions) in [
        ("integration", integration_bindings::EXEMPTIONS),
        ("e2e", e2e_bindings::EXEMPTIONS),
    ] {
        for exemption in exemptions {
            assert!(
                !exemption.boundary.trim().is_empty(),
                "{layer}: {} names no boundary",
                exemption.scenario
            );
            assert!(
                !exemption.alternative_proof.trim().is_empty(),
                "{layer}: {} names no alternative proof",
                exemption.scenario
            );
            let reason = exemption.boundary.to_lowercase();
            for word in FORBIDDEN {
                assert!(
                    !reason.contains(word),
                    "{layer}: {} exempts on '{word}', which is not a boundary",
                    exemption.scenario
                );
            }
        }
    }
}
