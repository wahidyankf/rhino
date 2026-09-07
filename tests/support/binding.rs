//! The step bindings.
//!
//! One `match` over the vocabulary. A sentence that has an arm here runs; one
//! that does not is `Unimplemented` and its scenario is red for that stated
//! reason. Arms arrive as the validators behind them are ported, so the red
//! list is the remaining work and shrinks by construction.

use crate::gherkin::Step;
use crate::harness;
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
        "the repository declares a complete configuration" => Outcome::Passed,
        "the repository has no configuration file" => {
            world.declaration.absent = true;
            Outcome::Passed
        }
        "the repository declares the schema {string}" => {
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
            Outcome::Passed
        }
        "an empty repository" => {
            // About the tree, not the configuration: a repository with a
            // complete policy and nothing to apply it to.
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
        "Markdown text containing a heading marker, Hello, can\'t-stop, naïve, and {int}" => {
            // One line holding exactly the four things a reader would count as
            // words, behind a heading marker that is punctuation rather than a
            // fifth.
            world.files.insert(
                "subject.md".to_string(),
                format!("# Hello can\'t-stop naïve {}\n", matched.integer(0)),
            );
            Outcome::Passed
        }
        "the repository declares word-budget surfaces:" => {
            let declared = rows(step);
            if declared.is_empty() {
                return Outcome::Failed(
                    "the step promised a table of surfaces and carried none".to_string(),
                );
            }
            let entries: Vec<String> = declared
                .iter()
                .filter_map(|row| {
                    let glob = row.get("glob")?;
                    let fail = row.get("fail")?;
                    Some(format!("{{glob: \"{glob}\", fail: {fail}}}"))
                })
                .collect();
            if entries.len() != declared.len() {
                return Outcome::Failed("the table needs a `glob` and a `fail` column".to_string());
            }
            // Order is the declaration: the last matching surface wins, so a
            // set here would lose the very thing the scenario asserts.
            world.declaration.overrides.insert(
                "governance-word-budget.surfaces".to_string(),
                format!("[{}]", entries.join(", ")),
            );
            Outcome::Passed
        }
        "the repository declares the mapped tree {string}" => {
            world.declaration.overrides.insert(
                "governance-directory-map.trees".to_string(),
                format!("[{{path: {}}}]", matched.string(0)),
            );
            Outcome::Passed
        }
        "the repository declares the mapped trees {string} and {string}" => {
            world.declaration.overrides.insert(
                "governance-directory-map.trees".to_string(),
                format!(
                    "[{{path: {}}}, {{path: {}}}]",
                    matched.string(0),
                    matched.string(1)
                ),
            );
            Outcome::Passed
        }
        "file {string} has title {string} and an empty directory map" => {
            world.files.insert(
                matched.string(0).to_string(),
                directory_map(matched.string(1), 0),
            );
            Outcome::Passed
        }
        "file {string} has an empty {string} directory map followed by {int} words" => {
            // The number is the file's word count, which is what a budget
            // measures and what the Then asserts. The title and the section
            // heading are words in the file, so the filler is what is left of
            // the total after them.
            world.files.insert(
                matched.string(0).to_string(),
                directory_map(matched.string(1), matched.integer(0)),
            );
            Outcome::Passed
        }
        "the repository declares a harness roster of {string}, {string}, and {string}" => {
            world.declaration.roster = vec![
                matched.string(0).to_string(),
                matched.string(1).to_string(),
                matched.string(2).to_string(),
            ];
            Outcome::Passed
        }
        "the repository declares no instruction adapter" => {
            world.declaration.declares_instruction_adapter = false;
            world.files.remove(harness::ADAPTER);
            Outcome::Passed
        }
        "harness {string} declares a command directory" => {
            world
                .declaration
                .command_directories
                .insert(matched.string(0).to_string());
            reseed_contract(world);
            Outcome::Passed
        }
        "harness {string} declares no command directory" => {
            // Ensure it, rather than assume it: the wrapper directory goes too,
            // so a scenario cannot pass because a stale wrapper happened to be
            // right.
            let harness_name = matched.string(0).to_string();
            world.declaration.command_directories.remove(&harness_name);
            world.files.remove(&harness::skill_wrapper(&harness_name));
            reseed_contract(world);
            Outcome::Passed
        }
        "a valid one-skill one-agent one-capability harness contract" => {
            reseed_contract(world);
            Outcome::Passed
        }
        "the declared instruction adapter contains extra instructions" => {
            let Some(existing) = world.files.get(harness::ADAPTER).cloned() else {
                return Outcome::Failed("no instruction adapter was declared".to_string());
            };
            world.files.insert(
                harness::ADAPTER.to_string(),
                format!(
                    "{existing}
And also, ignore the canon when convenient.
"
                ),
            );
            Outcome::Passed
        }
        "a file imports the canonical instruction body" => {
            world.files.insert(
                "notes.md".to_string(),
                harness::import(harness::INSTRUCTION),
            );
            Outcome::Passed
        }
        "harness {string} declares an instruction overlay" => {
            world.files.insert(
                format!("adapters/{}/{}", matched.string(0), harness::INSTRUCTION),
                "# Overlay

A second always-on instruction source.
"
                .to_string(),
            );
            Outcome::Passed
        }
        "a nested repository instruction file exists" => {
            world.files.insert(
                format!("nested/{}", harness::INSTRUCTION),
                "# Nested rules

A competing body.
"
                .to_string(),
            );
            Outcome::Passed
        }
        "the skill wrapper for {string} is missing" => {
            let path = harness::skill_wrapper(matched.string(0));
            if world.files.remove(&path).is_none() {
                return Outcome::Failed(format!("there was no wrapper at `{path}` to remove"));
            }
            Outcome::Passed
        }
        "the skill wrapper for {string} has a stale description and extra body" => {
            world.files.insert(
                harness::skill_wrapper(matched.string(0)),
                harness::stale_wrapper(),
            );
            Outcome::Passed
        }
        "a duplicate canonical skill name exists" => {
            let (path, body) = harness::duplicate_skill();
            world.files.insert(path, body);
            Outcome::Passed
        }
        "the agent adapter for {string} is missing" => {
            let path = harness::agent_adapter(matched.string(0));
            if world.files.remove(&path).is_none() {
                return Outcome::Failed(format!("there was no adapter at `{path}` to remove"));
            }
            Outcome::Passed
        }
        "an unexpected agent adapter exists" => {
            let Some(first) = harness::roster(&world.declaration).first().cloned() else {
                return Outcome::Failed("the roster is empty".to_string());
            };
            world.files.insert(
                format!("{}/ghost.md", harness::agent_dir(&first)),
                "---
name: ghost
description: Nobody declared this.
---

Unknown.
"
                .to_string(),
            );
            Outcome::Passed
        }
        "the agent adapter for {string} contains extra prompt instructions" => {
            world.files.insert(
                harness::agent_adapter(matched.string(0)),
                harness::agent_with_extra_prompt(),
            );
            Outcome::Passed
        }
        "the agent adapter for {string} weakens a denied capability" => {
            world.files.insert(
                harness::agent_adapter(matched.string(0)),
                harness::agent_weakening_denial(),
            );
            Outcome::Passed
        }
        "the agent adapter for {string} drops a declared constraint" => {
            world.files.insert(
                harness::agent_adapter(matched.string(0)),
                harness::agent_dropping_constraint(),
            );
            Outcome::Passed
        }
        "the canonical agent requires a capability outside the declared vocabulary" => {
            // The adapters gain it too, so the only thing wrong with the
            // repository is the capability's absence from the vocabulary.
            let requiring = harness::agent_requiring("network-write");
            world
                .files
                .insert(harness::canonical_agent(), requiring.clone());
            for name in harness::roster(&world.declaration) {
                world
                    .files
                    .insert(harness::agent_adapter(&name), requiring.clone());
            }
            Outcome::Passed
        }
        "each harness declares the required capability in its own capability format" => {
            // One harness declares in TOML and the rest in JSON, so a passing
            // run is one that compared meaning rather than syntax.
            let roster = harness::roster(&world.declaration);
            let Some(first) = roster.first().cloned() else {
                return Outcome::Failed("the roster is empty".to_string());
            };
            world
                .declaration
                .capability_formats
                .insert(first, "toml".to_string());
            reseed_contract(world);
            Outcome::Passed
        }
        "the required-capability command for harness {string} diverges" => {
            let name = matched.string(0).to_string();
            let format = harness::format_for(&world.declaration, &name);
            world.files.insert(
                harness::capability_file(&name, &format),
                harness::divergent_capability(&format),
            );
            Outcome::Passed
        }
        "the capability declaration for harness {string} is unreadable" => {
            let name = matched.string(0).to_string();
            let format = harness::format_for(&world.declaration, &name);
            world
                .unreadable
                .insert(harness::capability_file(&name, &format));
            Outcome::Passed
        }
        "excluded instruction sources and a linked skill exist" => {
            let excluded = if world.declaration.excluded_directories.is_empty() {
                return Outcome::Failed(
                    "no excluded directories were declared for this arrangement".to_string(),
                );
            } else {
                world.declaration.excluded_directories.clone()
            };
            for directory in excluded {
                world.files.insert(
                    format!("{directory}/{}", harness::INSTRUCTION),
                    "# Buried rules

Inside a directory nobody scans.
"
                    .to_string(),
                );
            }
            let linked = format!("{}/linked/SKILL.md", harness::SKILLS_ROOT);
            world.files.insert(
                linked.clone(),
                "---
name: linked
description: Reached through a link.
---

Body.
"
                .to_string(),
            );
            world.links.insert(linked);
            Outcome::Passed
        }
        "two sorted harness-parity violations exist" => {
            let roster = harness::roster(&world.declaration);
            if roster.len() < 2 {
                return Outcome::Failed("two violations need at least two harnesses".to_string());
            }
            for name in roster.iter().take(2) {
                world.files.remove(&harness::agent_adapter(name));
            }
            Outcome::Passed
        }
        "I remember the repository snapshot" => {
            world.snapshot = Some(world.files.clone());
            Outcome::Passed
        }
        "the repository declares a configuration with an empty harness roster and no canonical skill or agent root" =>
        {
            world.declaration.empty_roster = true;
            Outcome::Passed
        }
        "the repository declares a configuration with an empty harness roster and a canonical skills root" =>
        {
            world.declaration.empty_roster = true;
            world.declaration.canonical_skills_root = Some("canon/skills".to_string());
            Outcome::Passed
        }

        // -- Act ---------------------------------------------------------------
        "I inspect internal links" => {
            let arguments =
                validator_arguments("internal-link").expect("internal-link is a validator");
            let result = world.driver.invoke(&world.repository(), &arguments);
            world.record(result);
            Outcome::Passed
        }
        "I inspect Mermaid accessibility" => {
            let arguments = validator_arguments("mermaid").expect("mermaid is a validator");
            let result = world.driver.invoke(&world.repository(), &arguments);
            world.record(result);
            Outcome::Passed
        }
        "I invoke the CLI with {string}" => {
            let arguments: Vec<String> = matched
                .string(0)
                .split('|')
                .map(|argument| argument.to_string())
                .collect();
            let result = world.driver.invoke(&world.repository(), &arguments);
            world.record(result);
            Outcome::Passed
        }
        "I scan the declared surfaces"
        | "I find word-limit violations"
        | "I inspect the word budget" => {
            let arguments = validator_arguments("word-budget").expect("word-budget is a validator");
            let result = world.driver.invoke(&world.repository(), &arguments);
            world.record(result);
            Outcome::Passed
        }
        "I count the words in {string}" => {
            let arguments: Vec<String> = ["md", "word-count", "inspect", "--file"]
                .iter()
                .map(|part| (*part).to_string())
                .chain(std::iter::once(matched.string(0).to_string()))
                .collect();
            let result = world.driver.invoke(&world.repository(), &arguments);
            world.record(result);
            Outcome::Passed
        }
        "I inspect directory maps" => {
            let arguments =
                validator_arguments("directory-map").expect("directory-map is a validator");
            let result = world.driver.invoke(&world.repository(), &arguments);
            world.record(result);
            Outcome::Passed
        }
        "I inspect directory maps under the invalid {string} location" => {
            let Some(location) = invalid_location(matched.string(0)) else {
                return Outcome::Failed(format!(
                    "no location is defined for `{}`",
                    matched.string(0)
                ));
            };
            let mut arguments =
                validator_arguments("directory-map").expect("directory-map is a validator");
            arguments.push("--directory".to_string());
            arguments.push(location.to_string());
            let result = world.driver.invoke(&world.repository(), &arguments);
            world.record(result);
            Outcome::Passed
        }
        "I run the directory-map validator for {string}" => {
            let mut arguments =
                validator_arguments("directory-map").expect("directory-map is a validator");
            arguments.push("--directory".to_string());
            arguments.push(matched.string(0).to_string());
            let result = world.driver.invoke(&world.repository(), &arguments);
            world.record(result);
            Outcome::Passed
        }
        "I inspect harness parity" | "I inspect harness parity again" => {
            let arguments =
                validator_arguments("harness-parity").expect("harness-parity is a validator");
            let result = world.driver.invoke(&world.repository(), &arguments);
            world.record(result);
            Outcome::Passed
        }
        "I inspect harness parity twice" => {
            let arguments =
                validator_arguments("harness-parity").expect("harness-parity is a validator");
            let first = world.driver.invoke(&world.repository(), &arguments);
            world.record(first);
            let second = world.driver.invoke(&world.repository(), &arguments);
            world.record(second);
            Outcome::Passed
        }
        "I inspect and remember the harness-parity digest" => {
            let arguments =
                validator_arguments("harness-parity").expect("harness-parity is a validator");
            let result = world.driver.invoke(&world.repository(), &arguments);
            world.remembered_digest = digest_of(&result);
            world.record(result);
            match world.remembered_digest {
                Some(_) => Outcome::Passed,
                None => Outcome::Failed(format!(
                    "stdout carries no digest\nstdout: {}",
                    world.result().stdout
                )),
            }
        }
        "I add a canonical skill supporting resource" => {
            let (path, body) = harness::supporting_resource();
            world.files.insert(path, body);
            Outcome::Passed
        }
        "I inspect harness parity narrowed to {string}" => {
            let mut arguments =
                validator_arguments("harness-parity").expect("harness-parity is a validator");
            arguments.push("--harness".to_string());
            arguments.push(matched.string(0).to_string());
            let result = world.driver.invoke(&world.repository(), &arguments);
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
            let result = world.driver.invoke(&world.repository(), &arguments);
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
        "the scanned Markdown paths are:" => {
            if step.bullets.is_empty() {
                return Outcome::Failed(
                    "the step promised a list of paths and carried none".to_string(),
                );
            }
            let scanned = scanned_paths(world.result());
            let mut expected = step.bullets.clone();
            expected.sort();
            expect(
                scanned == expected,
                format!("expected {expected:?} scanned, got {scanned:?}"),
            )
        }
        "no Markdown files are scanned" => {
            let scanned = scanned_paths(world.result());
            expect(
                scanned.is_empty(),
                format!("expected nothing scanned, got {scanned:?}"),
            )
        }
        "the only violation is a {int}-word limit for {string}" => {
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
            let path = matched.string(0);
            let words = matched.integer(0);
            expect(
                lines[0].contains(path) && lines[0].contains(&format!("words={words}")),
                format!(
                    "expected a {words}-word violation for `{path}`, got `{}`",
                    lines[0]
                ),
            )
        }
        "the word count is {int}" => {
            let expected = matched.integer(0);
            match counted_words(world.result()) {
                Some(counted) => expect(
                    counted == expected,
                    format!(
                        "expected a count of {expected}, got {counted}\nstdout: {}",
                        world.result().stdout
                    ),
                ),
                None => Outcome::Failed(format!(
                    "stdout carries no word count\nstdout: {}",
                    world.result().stdout
                )),
            }
        }
        "{int} directories were inspected" => {
            let expected = matched.integer(0);
            match inspected_count(world.result()) {
                Some(counted) => expect(
                    counted == expected,
                    format!(
                        "expected {expected} directories inspected, the summary says {counted}\nstdout: {}",
                        world.result().stdout
                    ),
                ),
                None => Outcome::Failed(format!(
                    "stdout carries no inspection summary\nstdout: {}",
                    world.result().stdout
                )),
            }
        }
        "an argument error is raised" => {
            let result = world.result();
            expect(
                result.exit_code == 2,
                format!(
                    "expected an argument error, got exit {}\nstderr: {}",
                    result.exit_code, result.stderr
                ),
            )
        }
        "the only violation is a missing README at {string}" => {
            only_violation(world, &[matched.string(0), "missing README"])
        }
        "the only violation is a missing directory map at {string}" => {
            only_violation(world, &[matched.string(0), "missing directory map"])
        }
        "the only violation is a missing map entry from {string} to {string}" => only_violation(
            world,
            &[matched.string(0), matched.string(1), "missing map entry"],
        ),
        "the only violation is an invalid map entry from {string} to {string}" => only_violation(
            world,
            &[matched.string(0), matched.string(1), "invalid map entry"],
        ),
        "all violations are {string}" => {
            let result = world.result();
            let lines: Vec<&str> = result
                .stderr
                .lines()
                .filter(|line| line.starts_with('['))
                .collect();
            if lines.is_empty() {
                return Outcome::Failed("no violations were reported".to_string());
            }
            let phrase = matched.string(0);
            expect(
                lines.iter().all(|line| line.contains(phrase)),
                format!("expected every violation to be `{phrase}`, got {lines:?}"),
            )
        }
        "harness-parity validation succeeds" => {
            let result = world.result();
            expect(
                result.exit_code == 0,
                format!(
                    "expected a clean run, got exit {}\nstderr: {}",
                    result.exit_code, result.stderr
                ),
            )
        }
        "harness-parity validation succeeds with {int} harnesses, {int} skill, {int} agent, and {int} capability" =>
        {
            let result = world.result();
            if result.exit_code != 0 {
                return Outcome::Failed(format!(
                    "expected a clean run, got exit {}\nstderr: {}",
                    result.exit_code, result.stderr
                ));
            }
            let expected = format!(
                "canon {} harnesses, {} skills, {} agents, {} capabilities",
                matched.integer(0),
                matched.integer(1),
                matched.integer(2),
                matched.integer(3)
            );
            expect(
                result.stdout.contains(&expected),
                format!(
                    "expected `{expected}` in the summary\nstdout: {}",
                    result.stdout
                ),
            )
        }
        "the harness-parity violations include {string}" => {
            let result = world.result();
            let kind = matched.string(0);
            expect(
                result
                    .stderr
                    .lines()
                    .any(|line| line.starts_with('[') && line.contains(kind)),
                format!("expected a `{kind}` violation\nstderr: {}", result.stderr),
            )
        }
        "{int} harnesses were inspected" => {
            let expected = matched.integer(0);
            match inspected_count(world.result()) {
                Some(counted) => expect(
                    counted == expected,
                    format!(
                        "expected {expected} harnesses inspected, the summary says {counted}\nstdout: {}",
                        world.result().stdout
                    ),
                ),
                None => Outcome::Failed(format!(
                    "stdout carries no inspection summary\nstdout: {}",
                    world.result().stdout
                )),
            }
        }
        "harness-parity outputs are identical" => {
            let Some(previous) = world.previous_command_result.clone() else {
                return Outcome::Failed("only one inspection was run".to_string());
            };
            expect(
                &previous == world.result(),
                "the two inspections disagreed".to_string(),
            )
        }
        "harness-parity violations are ordinally sorted" => {
            let lines: Vec<&str> = world
                .result()
                .stderr
                .lines()
                .filter(|line| line.starts_with('['))
                .collect();
            if lines.len() < 2 {
                return Outcome::Failed(format!(
                    "sorting needs at least two violations, got {}",
                    lines.len()
                ));
            }
            let mut sorted = lines.clone();
            sorted.sort_unstable();
            expect(
                lines == sorted,
                format!("violations are not in order: {lines:?}"),
            )
        }
        "the repository snapshot is unchanged" => {
            let Some(snapshot) = world.snapshot.clone() else {
                return Outcome::Failed("no snapshot was taken".to_string());
            };
            expect(
                snapshot == world.files,
                "the inspection changed the repository".to_string(),
            )
        }
        "the harness-parity digest changed" => {
            let Some(remembered) = world.remembered_digest.clone() else {
                return Outcome::Failed("no digest was remembered".to_string());
            };
            match digest_of(world.result()) {
                Some(current) => expect(
                    current != remembered,
                    format!("the digest is still `{remembered}`"),
                ),
                None => Outcome::Failed(format!(
                    "stdout carries no digest\nstdout: {}",
                    world.result().stdout
                )),
            }
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

/// Every path a validator says it scanned, sorted.
///
/// Read back out of stdout because that is the only place all three adapters
/// can see it: a scan nobody can observe is a scan nobody can check.
fn scanned_paths(result: &CommandResult) -> Vec<String> {
    let mut paths: Vec<String> = result
        .stdout
        .lines()
        .filter_map(|line| line.split_once("] scanned "))
        .map(|(_, path)| path.trim().to_string())
        .collect();
    paths.sort();
    paths
}

/// The number a word-count inspection reports.
fn counted_words(result: &CommandResult) -> Option<usize> {
    let line = result.stdout.lines().find(|line| line.contains(" words"))?;
    line.rsplit_once(": ")?
        .1
        .split_whitespace()
        .next()?
        .parse()
        .ok()
}

/// Exactly one violation, naming every fragment the sentence promised.
///
/// Count and identity in one assertion: a count cannot tell a correct run from
/// one that reported the wrong thing, and an identity cannot tell it from one
/// that reported the right thing plus something else.
fn only_violation<D: Driver>(world: &World<D>, fragments: &[&str]) -> Outcome {
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
    for fragment in fragments {
        if !lines[0].contains(fragment) {
            return Outcome::Failed(format!(
                "the violation does not name `{fragment}`\nstderr: {}",
                lines[0]
            ));
        }
    }
    Outcome::Passed
}

/// A README with a Directory Map section holding no entries, whose whole file
/// counts `words` words when `words` is non-zero.
fn directory_map(title: &str, words: usize) -> String {
    let mut text = format!("# {title}\n\n## Directory Map\n");
    // "Directory Map" is two words and the title is one; the filler makes up
    // the rest of the requested total.
    let already = crate::binding::word_estimate(title) + 2;
    if words > already {
        let filler: Vec<String> = (0..words - already)
            .map(|index| format!("word{index}"))
            .collect();
        text.push('\n');
        text.push_str(&filler.join(" "));
        text.push('\n');
    }
    text
}

/// How many words a fixture fragment contributes, counted the same way the
/// validator counts: runs of letters, marks, and digits.
pub fn word_estimate(text: &str) -> usize {
    text.split(|c: char| !c.is_alphanumeric())
        .filter(|word| !word.is_empty())
        .count()
}

/// The argument a named invalid location stands for.
///
/// Named in the corpus rather than written out, because "outside repository" is
/// the property under test and `../outside` is only one spelling of it.
fn invalid_location(name: &str) -> Option<&'static str> {
    match name {
        "empty path" => Some(""),
        "absolute repository root" => Some("/"),
        "outside repository" => Some("../outside"),
        _ => None,
    }
}

/// Rebuild the canonical contract from the declaration as it now stands.
///
/// Called whenever a step changes something the contract's shape depends on --
/// the roster, a command directory, a capability format -- so the files and the
/// rendered configuration never disagree about what the repository is.
fn reseed_contract<D: Driver>(world: &mut World<D>) {
    for (path, body) in harness::valid_contract(&world.declaration) {
        world.files.insert(path, body);
    }
}

/// The digest a harness-parity run reports.
fn digest_of(result: &CommandResult) -> Option<String> {
    result
        .stdout
        .lines()
        .find_map(|line| line.split_once("] digest "))
        .map(|(_, digest)| digest.trim().to_string())
}
