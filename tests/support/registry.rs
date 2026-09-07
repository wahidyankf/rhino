//! The binding registry.
//!
//! A binding is registered by feature and scenario name. The static coverage
//! check compares this registry against the corpus without running anything,
//! which is what makes "every scenario is bound exactly once" a property that
//! can be read and checked rather than a claim.

use std::collections::BTreeSet;

pub type Binding = (&'static str, &'static str);

/// A binding a layer legitimately does not have, with the concrete boundary it
/// cannot reach and the alternative proof that covers it. Difficulty, runtime,
/// flakiness, and cost are never valid reasons, and a unit exemption is never
/// valid at all.
pub struct Exemption {
    pub feature: &'static str,
    pub scenario: &'static str,
    pub boundary: &'static str,
    pub alternative_proof: &'static str,
}

pub fn bound_keys(bindings: &[Binding]) -> BTreeSet<(String, String)> {
    bindings
        .iter()
        .map(|(feature, scenario)| (feature.to_string(), scenario.to_string()))
        .collect()
}

pub fn exempt_keys(exemptions: &[Exemption]) -> BTreeSet<(String, String)> {
    exemptions
        .iter()
        .map(|exemption| {
            (
                exemption.feature.to_string(),
                exemption.scenario.to_string(),
            )
        })
        .collect()
}
