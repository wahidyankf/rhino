//! The step bindings.
//!
//! One `match` over the vocabulary. A sentence that has an arm here runs; one
//! that does not is `Unimplemented` and its scenario is red for that stated
//! reason. Arms arrive as the validators behind them are ported, so the red
//! list is the remaining work and shrinks by construction.

use crate::gherkin::Step;
use crate::steps::{self, Match};
use crate::world::{CommandResult, Driver, World};

pub enum Outcome {
    Passed,
    Failed(String),
    Unimplemented(&'static str),
    Undefined,
}

/// The argument vector a validator name maps to. The corpus says
/// `I run the "directory-map" validator`; the CLI spells the same leaf as a
/// nested path, and that mapping belongs here rather than in the corpus.
fn validator_arguments(name: &str) -> Option<Vec<String>> {
    let path: &[&str] = match name {
        "word-budget" => &["governance", "word-budget", "validate"],
        "directory-map" => &["governance", "directory-map", "validate"],
        "harness-parity" => &["harness", "parity", "validate"],
        "internal-link" => &["md", "internal-link", "validate"],
        "mermaid" => &["md", "mermaid", "validate"],
        "repo-config" => &["repo-config", "validate"],
        _ => return None,
    };
    Some(path.iter().map(|part| (*part).to_string()).collect())
}

fn expect(condition: bool, message: String) -> Outcome {
    if condition {
        Outcome::Passed
    } else {
        Outcome::Failed(message)
    }
}

pub fn run_step<D: Driver>(world: &mut World<D>, step: &Step) -> Outcome {
    let Some(matched) = steps::lookup(&step.text) else {
        return Outcome::Undefined;
    };
    dispatch(world, step, &matched)
}

fn dispatch<D: Driver>(world: &mut World<D>, step: &Step, matched: &Match) -> Outcome {
    match matched.pattern {
        // -- Arrange: the configuration ---------------------------------------
        "the repository declares a complete configuration" => {
            world.declaration.complete = true;
            world.declaration.present = true;
            Outcome::Passed
        }
        "the repository has no configuration file" => {
            world.declaration.present = false;
            Outcome::Passed
        }
        "the repository declares the schema {string}" => {
            world.declaration.present = true;
            world.declaration.complete = true;
            world.declaration.schema = Some(matched.string(0).to_string());
            Outcome::Passed
        }
        "the configuration sets {string} to {string}" => {
            world
                .declaration
                .overrides
                .insert(matched.string(0).to_string(), matched.string(1).to_string());
            Outcome::Passed
        }
        "the configuration omits {string}" => {
            world
                .declaration
                .omissions
                .push(matched.string(0).to_string());
            Outcome::Passed
        }
        "the configuration adds the unknown key {string} to {string}" => {
            let key = format!("{}.{}", matched.string(1), matched.string(0));
            world
                .declaration
                .overrides
                .insert(key, "unexpected".to_string());
            Outcome::Passed
        }
        "the configuration adds the top-level section {string}" => {
            world
                .declaration
                .extra_sections
                .push(matched.string(0).to_string());
            Outcome::Passed
        }
        "the repository declares a configuration with an empty harness roster and no canonical skill or agent root" =>
        {
            world.declaration.present = true;
            world.declaration.complete = true;
            world.declaration.empty_roster = true;
            Outcome::Passed
        }
        "the repository declares a configuration with an empty harness roster and a canonical skills root" =>
        {
            world.declaration.present = true;
            world.declaration.complete = true;
            world.declaration.empty_roster = true;
            world.declaration.canonical_skills_root = Some("canon/skills".to_string());
            Outcome::Passed
        }

        // -- Act ---------------------------------------------------------------
        "I run the {string} validator" => {
            let Some(arguments) = validator_arguments(matched.string(0)) else {
                return Outcome::Failed(format!(
                    "no command path is defined for the `{}` validator",
                    matched.string(0)
                ));
            };
            let result = world
                .driver
                .invoke(&world.declaration, &world.files, &arguments);
            world.record(result);
            Outcome::Passed
        }

        // -- Assert ------------------------------------------------------------
        "the exit code is {int}" => {
            let result = world.result();
            let expected = matched.integer(0);
            expect(
                usize::from(result.exit_code) == expected,
                format!(
                    "expected exit {expected}, got {}\nstdout: {}\nstderr: {}",
                    result.exit_code, result.stdout, result.stderr
                ),
            )
        }
        "stderr names the missing configuration file" => {
            names(world.result(), &[crate::fixtures::CONFIG_PATH])
        }
        "stderr names the unrecognized schema" => {
            let schema = world
                .declaration
                .schema
                .clone()
                .unwrap_or_else(|| crate::fixtures::SCHEMA.to_string());
            names(world.result(), &[schema.as_str()])
        }
        "stderr names the unknown key and its position" => names_an_override_key(world),
        "stderr names the offending key and its position" => names_an_override_key(world),
        "stderr names the missing key" => {
            let omitted: Vec<String> = world.declaration.omissions.clone();
            let leaf: Vec<&str> = omitted
                .iter()
                .map(|key| key.rsplit('.').next().unwrap_or(key))
                .collect();
            names(world.result(), &leaf)
        }
        "stderr names the canon that has nowhere to be reconciled" => {
            names(world.result(), &["skills-root"])
        }
        "no directories were inspected" => {
            let result = world.result();
            expect(
                result.stdout.trim().is_empty(),
                format!(
                    "expected no inspection output, got stdout: {}",
                    result.stdout
                ),
            )
        }

        other => {
            let _ = step;
            Outcome::Unimplemented(other)
        }
    }
}

/// stderr must name each fragment, and carry a line reference so a maintainer
/// can find it. Asserting only the exit code would let any refusal pass for any
/// other refusal's reason.
fn names(result: &CommandResult, fragments: &[&str]) -> Outcome {
    for fragment in fragments {
        if !result.stderr.contains(fragment) {
            return Outcome::Failed(format!(
                "stderr does not name `{fragment}`\nstderr: {}",
                result.stderr
            ));
        }
    }
    expect(
        result.stderr.contains("line "),
        format!(
            "stderr names the subject but not its position\nstderr: {}",
            result.stderr
        ),
    )
}

fn names_an_override_key<D: Driver>(world: &World<D>) -> Outcome {
    let keys: Vec<String> = world
        .declaration
        .overrides
        .keys()
        .map(|key| key.rsplit('.').next().unwrap_or(key).to_string())
        .collect();
    if keys.is_empty() {
        return Outcome::Failed("no override was arranged for this assertion".to_string());
    }
    let borrowed: Vec<&str> = keys.iter().map(String::as_str).collect();
    names(world.result(), &borrowed)
}
