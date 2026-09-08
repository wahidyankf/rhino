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
#[allow(dead_code)]
#[path = "../support/steps.rs"]
mod steps;

#[path = "../e2e/bindings.rs"]
mod e2e_bindings;
#[path = "../integration/bindings.rs"]
mod integration_bindings;
#[path = "../unit/bindings.rs"]
mod unit_bindings;

use gherkin::Corpus;
use std::collections::{BTreeMap, BTreeSet};
use std::path::Path;

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
            // Both fields, because "too slow to run in CI" is the same
            // forbidden reason whichever of the two it is written in.
            let reason = format!(
                "{} {}",
                exemption.boundary.to_lowercase(),
                exemption.alternative_proof.to_lowercase()
            );
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

#[test]
fn every_canonical_feature_resolves_to_scenarios() {
    // A feature file the reader never opens contributes nothing, and a feature
    // name in the corpus with no file behind it means the reader invented one.
    // Both present as a coverage check that is quietly narrower than the corpus.
    let corpus = Corpus::canonical();
    let read: BTreeSet<String> = corpus
        .scenarios
        .iter()
        .map(|scenario| scenario.feature.clone())
        .collect();

    let directory = Path::new(concat!(env!("CARGO_MANIFEST_DIR"), "/specs/behaviours"));
    let committed: BTreeSet<String> = std::fs::read_dir(directory)
        .expect("the corpus directory is readable")
        .flatten()
        .map(|entry| entry.path())
        .filter(|path| path.extension().is_some_and(|kind| kind == "feature"))
        .filter_map(|path| {
            path.file_stem()
                .and_then(|stem| stem.to_str())
                .map(str::to_string)
        })
        .collect();

    assert!(
        !committed.is_empty(),
        "no feature files were found; every check below would pass vacuously"
    );
    assert_eq!(
        committed, read,
        "committed feature files and the features the reader resolved do not match"
    );
}

#[test]
fn every_step_resolves_to_exactly_one_vocabulary_entry() {
    // Zero is a scenario written in a sentence no adapter knows, which would
    // otherwise pass by never being run. Two is worse: which of them answers
    // the sentence depends on their order in the array, so the scenario means
    // something different after an unrelated edit.
    let corpus = Corpus::canonical();
    let mut offenders: Vec<String> = Vec::new();

    for scenario in &corpus.scenarios {
        for expansion in scenario.expansions() {
            for step in expansion {
                let matching: Vec<&str> = steps::VOCABULARY
                    .iter()
                    .copied()
                    .filter(|pattern| steps::matches(pattern, &step.text))
                    .collect();
                if matching.len() != 1 {
                    offenders.push(format!(
                        "{}: {}: {} {} -- {} entries match {matching:?}",
                        scenario.feature,
                        scenario.name,
                        step.keyword,
                        step.text,
                        matching.len()
                    ));
                }
            }
        }
    }

    offenders.sort();
    offenders.dedup();
    assert!(
        offenders.is_empty(),
        "{} steps do not resolve to exactly one vocabulary entry:\n{}",
        offenders.len(),
        offenders.join("\n")
    );
}

#[test]
fn every_vocabulary_entry_is_used_by_the_corpus() {
    // A binding nothing says is a sentence nobody can be held to. It also
    // rots: it keeps compiling against a world that has moved on, and the day
    // someone writes the sentence it silently does the wrong thing.
    let corpus = Corpus::canonical();
    let mut used: BTreeMap<&str, usize> = steps::VOCABULARY
        .iter()
        .copied()
        .map(|pattern| (pattern, 0))
        .collect();

    for scenario in &corpus.scenarios {
        for expansion in scenario.expansions() {
            for step in expansion {
                if let Some(matched) = steps::lookup(&step.text) {
                    *used.entry(matched.pattern).or_default() += 1;
                }
            }
        }
    }

    let unused: Vec<&str> = used
        .iter()
        .filter(|(_, count)| **count == 0)
        .map(|(pattern, _)| *pattern)
        .collect();
    assert!(
        unused.is_empty(),
        "{} vocabulary entries no scenario uses:\n{}",
        unused.len(),
        unused.join("\n")
    );
}

#[test]
fn every_vocabulary_entry_has_exactly_one_dispatch_arm() {
    // Read from the source rather than by running anything, because an entry
    // with no arm falls through to the catch-all and reports `Unimplemented`
    // -- which is the correct answer during a port and the wrong one after it.
    let source = std::fs::read_to_string(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/tests/support/binding.rs"
    ))
    .expect("the shared bindings are readable");

    let mut missing: Vec<&str> = Vec::new();
    let mut duplicated: Vec<&str> = Vec::new();
    for pattern in steps::VOCABULARY {
        // The source escapes an apostrophe inside a string literal. An arm may
        // also be one of several alternatives sharing a body, and rustfmt puts
        // each on its own line -- so what identifies an arm is the literal
        // followed by `=>` or `|`, whatever whitespace lies between.
        let literal = format!("\"{}\"", pattern.replace('\'', "\\'"));
        let mut arms = 0usize;
        let mut rest = source.as_str();
        while let Some(at) = rest.find(&literal) {
            let after = rest[at + literal.len()..].trim_start();
            if after.starts_with("=>") || after.starts_with('|') {
                arms += 1;
            }
            rest = &rest[at + literal.len()..];
        }
        match arms {
            0 => missing.push(pattern),
            1 => {}
            _ => duplicated.push(pattern),
        }
    }

    assert!(
        missing.is_empty(),
        "{} vocabulary entries have no dispatch arm:\n{}",
        missing.len(),
        missing.join("\n")
    );
    assert!(
        duplicated.is_empty(),
        "{} vocabulary entries have more than one dispatch arm:\n{}",
        duplicated.len(),
        duplicated.join("\n")
    );
}

#[test]
fn no_scenario_or_binding_is_a_placeholder() {
    // A placeholder name passes every structural check above while promising
    // work that was never done, which is the one failure a static gate can
    // catch and a running one cannot.
    const PLACEHOLDER: [&str; 6] = ["todo", "tbd", "fixme", "xxx", "placeholder", "wip"];
    let corpus = Corpus::canonical();

    let mut named: Vec<String> = corpus
        .scenarios
        .iter()
        .map(|scenario| format!("{}: {}", scenario.feature, scenario.name))
        .collect();
    for (layer, bindings) in [
        ("unit", unit_bindings::BINDINGS),
        ("integration", integration_bindings::BINDINGS),
        ("e2e", e2e_bindings::BINDINGS),
    ] {
        named.extend(
            bindings
                .iter()
                .map(|(feature, name)| format!("{layer}: {feature}: {name}")),
        );
    }

    let offenders: Vec<&String> = named
        .iter()
        .filter(|entry| {
            let lowered = entry.to_lowercase();
            PLACEHOLDER.iter().any(|word| lowered.contains(word))
        })
        .collect();
    assert!(offenders.is_empty(), "placeholder names: {offenders:?}");
}

#[test]
fn the_quick_gate_and_the_hooks_run_no_slow_adapter() {
    // The gate contract puts integration and E2E in the scheduled workflow and
    // nowhere else. Checked here rather than left to review because the failure
    // is silent in the wrong direction: a hook that got slower is a hook a
    // maintainer starts passing `--no-verify` to, and by then the gate is
    // decorative.
    const GATES: [(&str, &str); 5] = [
        ("xtask/src/main.rs", "//"),
        (".husky/pre-push", "#"),
        (".husky/pre-commit", "#"),
        (".husky/commit-msg", "#"),
        (".github/workflows/pr-quality-gate.yml", "#"),
    ];

    let root = Path::new(env!("CARGO_MANIFEST_DIR"));
    let mut offenders: Vec<String> = Vec::new();
    let mut quick_gate = String::new();

    for (relative, comment) in GATES {
        let text = std::fs::read_to_string(root.join(relative))
            .unwrap_or_else(|error| panic!("{relative} is readable: {error}"));
        // Comments here are prose *about* the policy -- this file is full of
        // it -- so only what the shell or the compiler would act on counts.
        let code: String = text
            .lines()
            .filter(|line| !line.trim_start().starts_with(comment))
            .collect::<Vec<_>>()
            .join("\n");
        for slow in ["integration", "e2e"] {
            if code.contains(slow) {
                offenders.push(format!("{relative} invokes the {slow} adapter"));
            }
        }
        if relative == "xtask/src/main.rs" {
            quick_gate = code;
        }
    }

    assert!(offenders.is_empty(), "{}", offenders.join("\n"));
    // The positive control: an absence proves nothing unless the same reading
    // can find what is certainly there.
    for expected in [
        "\"unit\"",
        "\"coverage\"",
        "\"--fail-under-lines\"",
        "\"99\"",
        "EXCLUDED_FROM_COVERAGE",
    ] {
        assert!(
            quick_gate.contains(expected),
            "the quick gate does not name {expected}, so the search above proves nothing"
        );
    }
}

#[test]
fn the_scheduled_workflow_runs_both_slow_adapters_unfiltered() {
    let text = std::fs::read_to_string(Path::new(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/.github/workflows/scheduled.yml"
    )))
    .expect("the scheduled workflow is committed");
    let code: String = text
        .lines()
        .filter(|line| !line.trim_start().starts_with('#'))
        .collect::<Vec<_>>()
        .join("\n");

    let integration = code
        .find("cargo test --test integration")
        .expect("the scheduled workflow runs the integration adapter");
    let e2e = code
        .find("cargo test --test e2e")
        .expect("the scheduled workflow runs the E2E adapter");
    assert!(
        integration < e2e,
        "integration must run before E2E: the boundary that can explain a \
         defect should be the one that reports it first"
    );
    // Unfiltered. A scheduled run that quietly narrowed itself would report a
    // green suite for a subset nobody chose.
    for narrowing in ["--skip", "--exact", "--test-threads"] {
        assert!(
            !code.contains(narrowing),
            "the scheduled run narrows itself with `{narrowing}`"
        );
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
                // Only this outline's own column names count. Angle brackets
                // are ordinary text in Markdown and in a Mermaid label, and a
                // check that flagged every `<...>` would forbid a scenario
                // from stating what markup looks like.
                let unsubstituted: Vec<&String> = scenario
                    .examples
                    .iter()
                    .flat_map(BTreeMap::keys)
                    .filter(|column| step.text.contains(&format!("<{column}>")))
                    .collect();
                if !unsubstituted.is_empty() {
                    offenders.push(format!(
                        "{}: {}: unsubstituted {unsubstituted:?} in {}",
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
