//! Plan trees a scenario starts from.
//!
//! A formal plan is six documents and one technical shape, and almost every
//! plan-structure scenario is about one defect in an otherwise conforming
//! plan. Writing all six out per scenario would bury the defect in the
//! boilerplate that surrounds it, so a scenario states the conforming plan and
//! then the one thing wrong with it.
//!
//! Everything here is invented. The bodies are the smallest text that satisfies
//! the rule they exist for, not sample content borrowed from a real plan.

use crate::world::World;

/// The five documents that are the same in every shape, and the single-file
/// technical shape that completes the set.
const DOCUMENTS: [(&str, &str); 4] = [
    ("README.md", "# Plan\n"),
    ("brd.md", "# Business Requirements\n"),
    ("learnings.md", "# Learnings\n"),
    ("tech-docs.md", "# Technical Documentation\n"),
];

/// One acceptance criterion and one delivery item citing it, so a plan that is
/// conforming in shape is also conforming in the two rules that read inside a
/// document.
const PRD: &str = "# Product Requirements\n\n## Acceptance Criteria\n\n```gherkin\nFeature: Fixture criteria\n\n  Scenario: [AC-01] Condition 1\n    Given a fixture repository\n    When the validator runs\n    Then it reads this scenario\n```\n";

const DELIVERY: &str = "# Delivery\n\n## Phase 1: Build\n\n- [AI] Do the thing this phase names. `[AC-01]`\n\n## Plan Archival\n\n- [AI] Move the plan folder to the done root.\n";

/// A conforming plan in the single-file technical shape.
pub fn seed<T>(world: &mut World<T>, root: &str) {
    for (name, body) in DOCUMENTS {
        world
            .files
            .insert(format!("{root}/{name}"), body.to_string());
    }
    world
        .files
        .insert(format!("{root}/prd.md"), PRD.to_string());
    world
        .files
        .insert(format!("{root}/delivery.md"), DELIVERY.to_string());
}

/// A companion set under the plan's technical directory, indexed in the order
/// it is given.
pub fn companions<T>(world: &mut World<T>, root: &str, names: &[&str]) {
    let mut index = String::from("# Technical Documentation\n\n## Modules\n\n");
    for (position, name) in names.iter().enumerate() {
        index.push_str(&format!("{}. [{name}]({name})\n", position + 1));
        world
            .files
            .insert(format!("{root}/tech-docs/{name}"), format!("# {name}\n"));
    }
    world
        .files
        .insert(format!("{root}/tech-docs/README.md"), index);
}

/// Rename one companion, in the set and in the index that lists it.
///
/// Resolved by suffix across the whole tree rather than by a path the scenario
/// repeats: a scenario names the companion it is about, and there is one.
pub fn rename<T>(world: &mut World<T>, from: &str, to: &str) -> Result<(), String> {
    let matches: Vec<String> = world
        .files
        .keys()
        .filter(|path| path.ends_with(&format!("/{from}")))
        .cloned()
        .collect();
    let [path] = matches.as_slice() else {
        return Err(format!(
            "expected exactly one companion named `{from}`, found {}",
            matches.len()
        ));
    };
    let Some(body) = world.files.remove(path) else {
        return Err(format!("{path} vanished between the look and the move"));
    };
    let directory = path
        .rsplit_once('/')
        .map_or(String::new(), |(directory, _)| directory.to_string());
    world.files.insert(format!("{directory}/{to}"), body);

    let index = format!("{directory}/README.md");
    if let Some(text) = world.files.get(&index).cloned() {
        world.files.insert(index, text.replace(from, to));
    }
    Ok(())
}
