//! The step bindings.
//!
//! One `match` over the vocabulary. A sentence that has an arm here runs; one
//! that does not is `Unimplemented` and its scenario is red for that stated
//! reason. Arms arrive as the validators behind them are ported, so the red
//! list is the remaining work and shrinks by construction.

use crate::gherkin::Step;
use crate::mermaid;
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
            world.declaration.present = true;
            Outcome::Passed
        }
        "the repository has no configuration file" => {
            world.declaration.present = false;
            Outcome::Passed
        }
        "the repository declares the schema {string}" => {
            world.declaration.present = true;
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
        "file {string} contains this Markdown:" => {
            let Some(body) = step.docstring.as_deref() else {
                return Outcome::Failed(
                    "the step promised a docstring and carried none".to_string(),
                );
            };
            world
                .files
                .insert(matched.string(0).to_string(), materialise(body));
            Outcome::Passed
        }
        "the repository contains:" => {
            let mut placed = 0usize;
            for row in rows(step) {
                let (Some(path), Some(content)) = (row.get("path"), row.get("content")) else {
                    return Outcome::Failed(
                        "the table needs a `path` and a `content` column".to_string(),
                    );
                };
                world.files.insert(path.clone(), materialise(content));
                placed += 1;
            }
            expect(
                placed > 0,
                "the step promised a table of files and carried none".to_string(),
            )
        }
        "the repository declares the internal-link excluded source {string}" => {
            world
                .declaration
                .excluded_sources
                .push(matched.string(0).to_string());
            Outcome::Passed
        }
        "the repository declares the accessible palette" => {
            // The fixture's declared palette is the accessible one. Saying so
            // is what makes the diagram scenarios readable without repeating
            // six colours in every Background.
            world.declaration.present = true;
            Outcome::Passed
        }
        "an empty repository" => {
            // About the tree, not the configuration: a repository with a
            // complete policy and nothing to apply it to.
            world.declaration.present = true;
            world.files.clear();
            Outcome::Passed
        }
        "the repository declares node labels at {int} graphemes and edge labels at {int}" => {
            world.declaration.overrides.insert(
                "md-mermaid.node-label-graphemes".to_string(),
                matched.integer(0).to_string(),
            );
            world.declaration.overrides.insert(
                "md-mermaid.edge-label-graphemes".to_string(),
                matched.integer(1).to_string(),
            );
            Outcome::Passed
        }
        "the repository declares a palette whose only fill color is {string}" => {
            world.declaration.overrides.insert(
                "md-mermaid.fill-colors".to_string(),
                format!("[\"{}\"]", matched.string(0)),
            );
            Outcome::Passed
        }
        "the repository declares excluded scan directories:" => {
            if step.bullets.is_empty() {
                return Outcome::Failed(
                    "the step promised a list of directories and carried none".to_string(),
                );
            }
            world.declaration.excluded_directories = step.bullets.clone();
            Outcome::Passed
        }
        "each declared excluded directory contains an unsafe Mermaid diagram" => {
            if world.declaration.excluded_directories.is_empty() {
                return Outcome::Failed(
                    "no excluded directories were declared for this assertion".to_string(),
                );
            }
            for directory in world.declaration.excluded_directories.clone() {
                world.files.insert(
                    format!("{directory}/diagram.md"),
                    mermaid::unsafe_diagram("flowchart LR", mermaid::Fence::Backtick),
                );
            }
            Outcome::Passed
        }
        "the repository contains Mermaid sample {string} at {string}" => {
            let Some(text) = mermaid::sample(matched.string(0)) else {
                return Outcome::Failed(format!(
                    "no Mermaid sample is catalogued under `{}`",
                    matched.string(0)
                ));
            };
            world.files.insert(matched.string(1).to_string(), text);
            Outcome::Passed
        }
        "an unsafe {string} Mermaid diagram exists at {string} using backtick fences" => {
            world.files.insert(
                matched.string(1).to_string(),
                mermaid::unsafe_diagram(matched.string(0), mermaid::Fence::Backtick),
            );
            Outcome::Passed
        }
        "an unsafe {string} Mermaid diagram exists at {string} using tilde fences" => {
            world.files.insert(
                matched.string(1).to_string(),
                mermaid::unsafe_diagram(matched.string(0), mermaid::Fence::Tilde),
            );
            Outcome::Passed
        }
        "the repository contains a Mermaid class filled {string} with a declared stroke and text color" =>
        {
            world.files.insert(
                "guides/diagram.md".to_string(),
                mermaid::class_filled(matched.string(0)),
            );
            Outcome::Passed
        }
        "the repository contains a Mermaid node label of {int} graphemes" => {
            world.files.insert(
                "guides/diagram.md".to_string(),
                mermaid::node_label_of(matched.integer(0)),
            );
            Outcome::Passed
        }
        "the repository declares a word-budget surface {string} failing above {int}" => {
            world.declaration.overrides.insert(
                "governance-word-budget.surfaces".to_string(),
                format!(
                    "[{{glob: \"{}\", fail: {}}}]",
                    matched.string(0),
                    matched.integer(0)
                ),
            );
            Outcome::Passed
        }
        "file {string} contains {int} words" => {
            let words: Vec<String> = (0..matched.integer(0))
                .map(|index| format!("word{index}"))
                .collect();
            world.files.insert(
                matched.string(0).to_string(),
                format!("{}\n", words.join(" ")),
            );
            Outcome::Passed
        }
        "the repository declares a configuration with an empty harness roster and no canonical skill or agent root" =>
        {
            world.declaration.present = true;
            world.declaration.empty_roster = true;
            Outcome::Passed
        }
        "the repository declares a configuration with an empty harness roster and a canonical skills root" =>
        {
            world.declaration.present = true;
            world.declaration.empty_roster = true;
            world.declaration.canonical_skills_root = Some("canon/skills".to_string());
            Outcome::Passed
        }

        // -- Act ---------------------------------------------------------------
        "I inspect internal links" => {
            let arguments =
                validator_arguments("internal-link").expect("internal-link is a validator");
            let result = world
                .driver
                .invoke(&world.declaration, &world.files, &arguments);
            world.record(result);
            Outcome::Passed
        }
        "I inspect Mermaid accessibility" => {
            let arguments = validator_arguments("mermaid").expect("mermaid is a validator");
            let result = world
                .driver
                .invoke(&world.declaration, &world.files, &arguments);
            world.record(result);
            Outcome::Passed
        }
        "I invoke the CLI with {string}" => {
            let arguments: Vec<String> = matched
                .string(0)
                .split('|')
                .map(|argument| argument.to_string())
                .collect();
            let result = world
                .driver
                .invoke(&world.declaration, &world.files, &arguments);
            world.record(result);
            Outcome::Passed
        }
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
        "there are {int} violations" => {
            let expected = matched.integer(0);
            let counted = violations(world.result());
            expect(
                counted == expected,
                format!(
                    "expected {expected} violations, counted {counted}\nstderr: {}",
                    world.result().stderr
                ),
            )
        }
        "there are no violations" => {
            let counted = violations(world.result());
            expect(
                counted == 0,
                format!(
                    "expected no violations, counted {counted}\nstderr: {}",
                    world.result().stderr
                ),
            )
        }
        "{int} Mermaid diagrams were inspected" => {
            let expected = matched.integer(0);
            match inspected_count(world.result()) {
                Some(counted) => expect(
                    counted == expected,
                    format!(
                        "expected {expected} diagrams inspected, the summary says {counted}\nstdout: {}",
                        world.result().stdout
                    ),
                ),
                None => Outcome::Failed(format!(
                    "stdout carries no inspection summary\nstdout: {}",
                    world.result().stdout
                )),
            }
        }
        "stdout reports that zero diagrams were checked" => {
            let stdout = world.result().stdout.clone();
            expect(
                stdout.contains("checked 0 diagram"),
                format!("expected an explicit zero in the summary\nstdout: {stdout}"),
            )
        }
        "the only violation is a Mermaid accessibility issue at {string}" => {
            let result = world.result();
            let lines: Vec<&str> = result
                .stderr
                .lines()
                .filter(|line| line.starts_with('['))
                .collect();
            if lines.len() != 1 {
                return Outcome::Failed(format!(
                    "expected exactly one violation, got {}\nstderr: {}",
                    lines.len(),
                    result.stderr
                ));
            }
            expect(
                lines[0].starts_with("[mermaid] ") && lines[0].contains(matched.string(0)),
                format!(
                    "expected a mermaid violation at `{}`, got `{}`",
                    matched.string(0),
                    lines[0]
                ),
            )
        }
        "the only violation starts with {string}" => {
            // Count and identity in one sentence. A count alone cannot tell a
            // correct implementation from one that reported the wrong thing,
            // and an identity alone cannot tell it from one that reported the
            // right thing plus something else.
            let result = world.result();
            let lines: Vec<&str> = result
                .stderr
                .lines()
                .filter(|line| line.starts_with('['))
                .collect();
            if lines.len() != 1 {
                return Outcome::Failed(format!(
                    "expected exactly one violation, got {}\nstderr: {}",
                    lines.len(),
                    result.stderr
                ));
            }
            let body = lines[0].split_once("] ").map_or(lines[0], |(_, rest)| rest);
            expect(
                body.starts_with(matched.string(0)),
                format!(
                    "expected the violation to start `{}`, got `{body}`",
                    matched.string(0)
                ),
            )
        }
        "the formatted violation starts with {string}" => {
            let result = world.result();
            let Some(line) = result.stderr.lines().find(|line| line.starts_with('[')) else {
                return Outcome::Failed(format!(
                    "no violation was reported\nstderr: {}",
                    result.stderr
                ));
            };
            let body = line.split_once("] ").map_or(line, |(_, rest)| rest);
            expect(
                body.starts_with(matched.string(0)),
                format!(
                    "expected a violation starting `{}`, got `{body}`",
                    matched.string(0)
                ),
            )
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

/// A table read as a list of column-keyed rows, so a binding names its columns
/// rather than indexing them and silently reading the wrong one when a corpus
/// author reorders a table.
fn rows(step: &Step) -> Vec<std::collections::BTreeMap<String, String>> {
    let Some((header, body)) = step.table.split_first() else {
        return Vec::new();
    };
    body.iter()
        .map(|cells| {
            header
                .iter()
                .cloned()
                .zip(cells.iter().cloned())
                .collect::<std::collections::BTreeMap<_, _>>()
        })
        .collect()
}

/// Turn corpus fixture text into the bytes a repository would actually hold.
///
/// `{nul}` is written literally in the corpus because a Gherkin file cannot
/// carry a NUL byte, and a path containing one is exactly what "a malformed
/// local target" means on every platform RHINO ships to.
fn materialise(body: &str) -> String {
    body.replace("{nul}", "\0")
}

/// How many findings a validator reported.
///
/// Counted from the prefixed stderr lines, which is the one place all three
/// adapters can see: the E2E adapter has only the process contract, so a
/// violation count that came from anywhere else would not be the same claim.
fn violations(result: &CommandResult) -> usize {
    result
        .stderr
        .lines()
        .filter(|line| line.starts_with('['))
        .count()
}

/// The number a report's summary line says was inspected.
///
/// Read back out of stdout rather than out of the world, because that summary
/// is the only place all three adapters can see it -- and because an explicit
/// zero that is not printed is not an explicit zero.
fn inspected_count(result: &CommandResult) -> Option<usize> {
    let line = result
        .stdout
        .lines()
        .find(|line| line.contains("checked "))?;
    let after = line.split("checked ").nth(1)?;
    after.split_whitespace().next()?.parse().ok()
}
