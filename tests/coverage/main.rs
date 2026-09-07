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

        // A duplicated binding runs its scenario twice and reads as extra
        // coverage rather than as the mistake it is.
        assert_eq!(
            bindings.len(),
            bound.len(),
            "{layer}: a scenario is bound more than once"
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

// -- Parser regressions -------------------------------------------------------
//
// The Gherkin reader is the harness's own production code, and it shipped with
// two defects that nothing here would have caught: it split table rows on
// escaped pipes, and it dropped `Background:` entirely. Both were quiet. The
// first truncated a command vector to its first segment, and several such rows
// assert exit 2, which a truncated command also returns -- so they passed while
// invoking nothing the scenario named. The second discarded the very `Given`s
// that declare a repository's policy, leaving those scenarios asserting against
// policy nobody had declared. These tests exist so neither can return.

#[test]
fn a_table_cell_may_contain_an_escaped_pipe() {
    let corpus = Corpus::canonical();
    let scenario = corpus
        .find("cli-contract", "Help requests succeed")
        .expect("the escaped-pipe scenario is in the corpus");

    let invocations: Vec<String> = scenario
        .expansions()
        .iter()
        .filter_map(|steps| steps.iter().find(|step| step.text.starts_with("I invoke")))
        .map(|step| step.text.clone())
        .collect();

    assert!(
        invocations
            .iter()
            .any(|text| text.contains("word-budget|validate|--help")),
        "an escaped pipe was treated as a column separator; invocations were {invocations:?}"
    );
    assert!(
        !invocations.iter().any(|text| text.contains('\\')),
        "a cell kept its backslash instead of being unescaped: {invocations:?}"
    );
}

#[test]
fn no_expanded_step_carries_an_unresolved_escape() {
    let corpus = Corpus::canonical();
    let mut offenders: Vec<String> = Vec::new();

    for scenario in &corpus.scenarios {
        for expansion in scenario.expansions() {
            for step in expansion {
                if step.text.contains('\\') {
                    offenders.push(format!(
                        "{}: {}: {}",
                        scenario.feature, scenario.name, step.text
                    ));
                }
                if step.text.contains('<') && step.text.contains('>') && scenario.is_outline {
                    offenders.push(format!(
                        "{}: {}: unsubstituted placeholder in {}",
                        scenario.feature, scenario.name, step.text
                    ));
                }
            }
        }
    }

    assert!(
        offenders.is_empty(),
        "steps still carrying an escape or placeholder after expansion:\n{}",
        offenders.join("\n")
    );
}

#[test]
fn a_background_reaches_every_scenario_in_its_feature() {
    // Each of these features declares its policy in a `Background:`. Every
    // scenario in that feature must carry the declaration, or it asserts
    // against a value nothing established.
    const DECLARED: [(&str, &str); 5] = [
        ("directory-map", "the repository declares the mapped tree"),
        (
            "internal-link",
            "the repository declares a complete configuration",
        ),
        (
            "harness-parity",
            "the repository declares a harness roster of",
        ),
        (
            "mermaid-cli",
            "the repository declares the accessible palette",
        ),
        (
            "mermaid-legibility",
            "the repository declares the accessible palette",
        ),
    ];

    let corpus = Corpus::canonical();
    let mut missing: Vec<String> = Vec::new();

    for (feature, declaration) in DECLARED {
        let scenarios: Vec<_> = corpus
            .scenarios
            .iter()
            .filter(|scenario| scenario.feature == feature)
            .collect();
        assert!(
            !scenarios.is_empty(),
            "{feature} has no scenarios; the check would pass vacuously"
        );

        for scenario in scenarios {
            let declared = scenario
                .steps
                .iter()
                .any(|step| step.text.starts_with(declaration));
            if !declared {
                missing.push(format!("{feature}: {}", scenario.name));
            }
        }
    }

    assert!(
        missing.is_empty(),
        "scenarios missing their feature's Background declaration:\n{}",
        missing.join("\n")
    );
}

#[test]
fn a_docstring_reaches_the_step_it_belongs_to() {
    // Docstrings were dropped entirely at first, and silently: `#` opened a
    // Gherkin comment even inside a `"""` block, so a fixture whose first line
    // is a Markdown heading vanished along with the rest of the file it
    // described. The scenarios then inspected an empty tree.
    let corpus = Corpus::canonical();
    let scenario = corpus
        .find(
            "internal-link",
            "Existing local links and non-local links pass",
        )
        .expect("the docstring scenario is in the corpus");

    let bodies: Vec<&String> = scenario
        .steps
        .iter()
        .filter_map(|step| step.docstring.as_ref())
        .collect();

    assert_eq!(
        bodies.len(),
        4,
        "expected four fixture files to carry content, got {}",
        bodies.len()
    );
    assert!(
        bodies.iter().any(|body| body.starts_with("# Root")),
        "a Markdown heading inside a docstring was eaten as a Gherkin comment"
    );
    assert!(
        bodies
            .iter()
            .any(|body| body.contains("[Reference target][target-ref]")),
        "docstring content is missing the link the scenario inspects"
    );
}

#[test]
fn a_step_sentence_ending_in_a_colon_always_carries_a_payload() {
    // A sentence written with a trailing colon promises a table, a bullet list,
    // or a docstring beneath it. One that arrives empty means the parser lost
    // the payload -- which is how all three parser defects presented.
    let corpus = Corpus::canonical();
    let mut empty: Vec<String> = Vec::new();

    for scenario in &corpus.scenarios {
        for expansion in scenario.expansions() {
            for step in expansion {
                let promises_payload = step.text.ends_with(':');
                let carries_payload =
                    !step.table.is_empty() || !step.bullets.is_empty() || step.docstring.is_some();
                if promises_payload && !carries_payload {
                    empty.push(format!(
                        "{}: {}: {} {}",
                        scenario.feature, scenario.name, step.keyword, step.text
                    ));
                }
            }
        }
    }

    empty.sort();
    empty.dedup();
    assert!(
        empty.is_empty(),
        "steps promising a payload that arrived empty:\n{}",
        empty.join("\n")
    );
}
