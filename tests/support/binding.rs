//! The step bindings.
//!
//! One `match` over the vocabulary. A sentence that has an arm here runs; one
//! that does not is `Unimplemented` and its scenario is red for that stated
//! reason. Arms arrive as the validators behind them are ported, so the red
//! list is the remaining work and shrinks by construction.

use crate::fixtures;
use crate::gherkin::Step;
use crate::harness;
use crate::mermaid;
use crate::steps::{self, Match};
use crate::world::{CommandResult, Declaration, Driver, World};

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
        "heading-hierarchy" => &["md", "heading-hierarchy", "validate"],
        "emoji" => &["convention", "emoji", "validate"],
        "frontmatter" => &["md", "frontmatter", "validate"],
        "readme-index" => &["md", "readme-index", "validate"],
        "naming" => &["md", "naming", "validate"],
        "repo-config" => &["repo-config", "validate"],
        _ => return None,
    };
    Some(path.iter().map(|part| (*part).to_string()).collect())
}

/// Every configuration section a release added after the one a consumer already
/// runs. Each is optional, and a repository declaring none of them has to be
/// told nothing at all.
const OPTIONAL_SECTIONS: [&str; 5] = [
    "convention-emoji",
    "md-frontmatter",
    "md-heading-hierarchy",
    "md-naming",
    "md-readme-index",
];

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
        // The base fixture *is* the complete configuration, so this step has
        // nothing to add. It stays in the vocabulary because a scenario that
        // breaks one key has to be able to say what it started from.
        "the repository declares a complete configuration" => Outcome::Passed,
        "a required MCP server is declared" => {
            world.declaration.declares_required_mcp = true;
            Outcome::Passed
        }
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
        "the repository declares the front-matter surface {string} requiring {string}" => {
            let required: Vec<&str> = matched.string(1).split(',').map(str::trim).collect();
            push_frontmatter_surface(
                world,
                matched.string(0),
                &format!("require: [{}]", required.join(", ")),
            )
        }
        "the repository declares the front-matter surface {string} requiring nothing" => {
            push_frontmatter_surface(world, matched.string(0), "require: []")
        }
        "the repository declares the front-matter surface {string} with the {string} values {string}" =>
        {
            let permitted: Vec<&str> = matched.string(2).split(',').map(str::trim).collect();
            push_frontmatter_surface(
                world,
                matched.string(0),
                &format!(
                    "require: [], enum: {{{}: [{}]}}",
                    matched.string(1),
                    permitted.join(", ")
                ),
            )
        }
        "the repository declares the front-matter surface {string} with the ISO date key {string}" => {
            push_frontmatter_surface(
                world,
                matched.string(0),
                &format!("require: [], iso-date: [{}]", matched.string(1)),
            )
        }
        "the repository declares the front-matter surface {string} forbidding {string}" => {
            push_frontmatter_surface(
                world,
                matched.string(0),
                &format!("require: [], forbid: [{}]", matched.string(1)),
            )
        }
        "the repository declares the heading surface {string} with one H1 and a maximum jump of {int}" => {
            push_heading_surface(world, matched.string(0), true, matched.integer(0))
        }
        "the repository declares the heading surface {string} with any number of H1s and a maximum jump of {int}" => {
            push_heading_surface(world, matched.string(0), false, matched.integer(0))
        }
        "the repository declares the README-index tree {string}" => {
            world
                .declaration
                .readme_index_trees
                .push(matched.string(0).to_string());
            Outcome::Passed
        }
        "the repository declares the emoji-prohibited surface {string}" => {
            world
                .declaration
                .emoji_prohibited
                .push(matched.string(0).to_string());
            Outcome::Passed
        }
        "the repository declares the kebab-case naming surface {string}" => {
            world.declaration.naming_surfaces.push(format!(
                "{{glob: \"{}\", style: kebab-case}}",
                matched.string(0)
            ));
            Outcome::Passed
        }
        "the repository declares the path-prefixed naming surface {string} separated by {string}" =>
        {
            world.declaration.naming_surfaces.push(format!(
                "{{glob: \"{}\", style: path-prefixed, separator: \"{}\"}}",
                matched.string(0),
                matched.string(1)
            ));
            Outcome::Passed
        }
        "the repository declares the naming exemption {string}" => {
            world
                .declaration
                .naming_exempt
                .push(matched.string(0).to_string());
            Outcome::Passed
        }
        "the repository declares the internal-link excluded source {string}" => {
            world
                .declaration
                .excluded_sources
                .push(matched.string(0).to_string());
            Outcome::Passed
        }
        "a repository declaring every section with nothing to find" => {
            // Every section of the schema is declared by the base fixture. What
            // this sentence adds is a *tree* that satisfies all of them at once
            // -- the harness contract complete, and the mapped tree simply
            // absent. The mapped tree is present and complete rather than
            // missing: a walk of nothing and a clean walk of something produce
            // the same exit code, and only the second can tell a validator that
            // stopped reading from one that found nothing wrong.
            reseed_contract(world);
            // One accessible diagram and one resolvable link, for the same
            // reason as the mapped tree: a walk of nothing and a clean walk of
            // something produce the same exit code, and only the second can
            // tell a validator that looked from one that did not.
            world.files.insert(
                "rules/README.md".to_string(),
                "# Rules\n\n## Directory Map\n\n- [Diagram](diagram.md)\n\nSee [the diagram](diagram.md).\n"
                    .to_string(),
            );
            world.files.insert(
                "rules/diagram.md".to_string(),
                mermaid::sample("accessible colored class")
                    .expect("the accessible sample is part of the fixture set"),
            );
            Outcome::Passed
        }
        "file {string} exceeds its declared word budget" => {
            let path = matched.string(0).to_string();
            // The budget is declared for this file specifically, so the
            // scenario's own words are what the tool is measured against
            // rather than whichever surface the base happened to name.
            world.declaration.overrides.insert(
                "governance-word-budget.surfaces".to_string(),
                format!("[{{glob: \"{path}\", fail: 3}}]"),
            );
            world
                .files
                .insert(path, "one two three four five six\n".to_string());
            Outcome::Passed
        }
        "the governed files are exclusively locked" => {
            // Everything the repository holds, but not the configuration: the
            // scenario is about a tree that cannot be read, not about a tool
            // that cannot be configured.
            if world.files.is_empty() {
                return Outcome::Failed("there is nothing to lock".to_string());
            }
            for path in world.files.keys() {
                world.unreadable.insert(path.clone());
            }
            Outcome::Passed
        }
        "an unsafe {string} Mermaid diagram is supplied on standard input" => {
            world.stdin = Some(mermaid::unsafe_diagram(
                matched.string(0),
                mermaid::Fence::Backtick,
            ));
            Outcome::Passed
        }
        "a nested repository under {string} scans a directory the outer repository excludes" => {
            // A complete second repository inside the first. The outer one
            // excludes `guides`; the nested one does not, and the finding is in
            // `guides`. Only a run that read the *nested* configuration can see
            // it, so a run that inherited the outer exclusions reports a clean
            // walk of nothing.
            let root = matched.string(0).to_string();
            world
                .declaration
                .excluded_directories
                .extend([root.clone(), "guides".to_string()]);
            let nested = fixtures::render(&Declaration::default());
            world
                .files
                .insert(format!("{root}/{}", fixtures::CONFIG_PATH), nested);
            world.files.insert(
                format!("{root}/guides/diagram.md"),
                mermaid::unsafe_diagram("flowchart LR", mermaid::Fence::Backtick),
            );
            Outcome::Passed
        }
        "the configuration file is this text:" => {
            // Written verbatim rather than rendered, because what these
            // scenarios are about is the shape of the file's first lines --
            // which is precisely what the renderer would normalise away.
            let Some(text) = step.docstring.clone() else {
                return Outcome::Failed("the sentence promised a document".to_string());
            };
            world.declaration.absent = true;
            world
                .files
                .insert(fixtures::CONFIG_PATH.to_string(), format!("{text}\n"));
            Outcome::Passed
        }
        "the configuration file cannot be read" => {
            world.unreadable.insert(fixtures::CONFIG_PATH.to_string());
            Outcome::Passed
        }
        "stderr names the missing schema declaration" => names(
            world.result(),
            &[fixtures::CONFIG_PATH, "declares no schema"],
        ),
        "stderr names the unreadable configuration file" => {
            names(world.result(), &[fixtures::CONFIG_PATH, "cannot be read"])
        }
        "stdout is the version the JSON form reported" => {
            // The two spellings of `version` have to name one build. Comparing
            // them against each other rather than against a literal keeps the
            // check true across releases while still refusing a text form that
            // has drifted into reporting something else entirely.
            let Some(previous) = world.previous_command_result.clone() else {
                return Outcome::Failed("only one invocation was run".to_string());
            };
            let Some(reported) = json_string(&previous.stdout, "version") else {
                return Outcome::Failed(format!(
                    "the JSON form reported no version\nstdout: {}",
                    previous.stdout
                ));
            };
            let stdout = world.result().stdout.clone();
            expect(
                stdout.trim() == reported,
                format!(
                    "the text form says `{}`, the JSON form says `{reported}`",
                    stdout.trim()
                ),
            )
        }
        "stdout is one non-empty line" => {
            let stdout = &world.result().stdout;
            let lines: Vec<&str> = stdout.lines().collect();
            expect(
                lines.len() == 1 && !lines[0].trim().is_empty(),
                format!("expected exactly one non-empty line, got {lines:?}"),
            )
        }
        "the repository contains a Mermaid node label written as {string}" => {
            // The label as an author *types* it, so the scenario can state the
            // difference between what is written and what is read.
            world.files.insert(
                "guides/diagram.md".to_string(),
                mermaid::document(
                    &format!("flowchart LR\n    Alpha[{}]", matched.string(0)),
                    mermaid::Fence::Backtick,
                ),
            );
            Outcome::Passed
        }
        "I invoke the CLI with no arguments" => {
            let result = world.driver.invoke(&world.repository(), &[]);
            world.record(result);
            Outcome::Passed
        }
        "stderr contains {string}" => {
            // A plain fragment check. `names` additionally requires a line
            // number, which is right for a configuration diagnostic and wrong
            // for everything else that has no position to report.
            let needle = matched.string(0);
            let stderr = &world.result().stderr;
            expect(
                stderr.contains(needle),
                format!("stderr does not contain `{needle}`\nstderr: {stderr}"),
            )
        }
        "file {string} vanishes between the walk and the read" => {
            world.vanished.insert(matched.string(0).to_string());
            Outcome::Passed
        }
        "an index README sits in the canonical agents root" => {
            world.files.insert(
                format!("{}/{}", harness::AGENTS_ROOT, "README.md"),
                "# Agents\n\nWhat lives here.\n".to_string(),
            );
            Outcome::Passed
        }
        "an index README sits in every harness agent directory" => {
            for name in harness::roster(&world.declaration) {
                world.files.insert(
                    // An index is always `README.md`, whatever extension the
                    // harness's adapters use. For a harness whose adapters are
                    // not Markdown it does not even match the pattern, which is
                    // the answer too.
                    harness::agent_adapter_directory(&name) + "/README.md",
                    "# Adapters\n\nWhat lives here.\n".to_string(),
                );
            }
            Outcome::Passed
        }
        "a source file contains the canonical import" => {
            // The route appears because this file *implements* the check that
            // looks for it -- which is the shape the false positive took in a
            // real repository, and the reason the scan is a Markdown question.
            world.files.insert(
                "src/parity.rs".to_string(),
                format!(
                    "const ROUTE: &str = \"{}\";\n",
                    harness::import(harness::INSTRUCTION).trim()
                ),
            );
            Outcome::Passed
        }
        "a harness configuration field is declared as a prohibited instruction source" => {
            declare_prohibited_field(world, "json")
        }
        "a harness configuration field in TOML is declared as a prohibited instruction source" => {
            declare_prohibited_field(world, "toml")
        }
        "that field is written as {string}" => {
            let file = harness::prohibited_field_file(&world.declaration);
            let body = settings_holding(&world.declaration, matched.string(0))
                .expect("the shape table names every shape the corpus writes");
            world.files.insert(file.to_string(), body);
            Outcome::Passed
        }
        "that settings file is not in the repository" => {
            let file = harness::prohibited_field_file(&world.declaration);
            world.files.remove(file);
            Outcome::Passed
        }
        "that settings file is not written in its declared format" => {
            // Neither JSON nor TOML, so the declared reader fails whichever
            // format the scenario declared.
            let file = harness::prohibited_field_file(&world.declaration);
            world
                .files
                .insert(file.to_string(), "{ this is not either one\n".to_string());
            Outcome::Passed
        }
        "the canonical instruction body and its adapter are not Markdown" => {
            // Declared before the contract is seeded, so the files the fixture
            // writes and the configuration that names them agree about where
            // the canon lives.
            world.declaration.canon_is_not_markdown = true;
            reseed_contract(world);
            Outcome::Passed
        }
        "a file with a prohibited name is not Markdown" => {
            // Prohibited by name and unreadable as Markdown, so only the glob
            // can catch it -- which is the half of the rule that has to stay
            // an every-file question.
            world.declaration.overrides.insert(
                "harness-parity.prohibited-instruction-sources".to_string(),
                "[\"**/*.instructions\"]".to_string(),
            );
            world.files.insert(
                "nested/always.instructions".to_string(),
                "always read this first\n".to_string(),
            );
            Outcome::Passed
        }
        "the canonical agents are gone and only their declaration shape remains" => {
            world.declaration.omit_agents_root = true;
            world.declaration.omit_agent_adapters = true;
            world.declaration.keep_declaration_shape = true;
            Outcome::Passed
        }
        "the canonical agents declare their permissions under {string} names" => {
            world.declaration.renamed_declaration = matched.string(0) == "alternate";
            Outcome::Passed
        }
        "the canonical agent omits the field every declaration must carry" => {
            world.files.insert(
                harness::canonical_agent(),
                harness::agent_without_its_fixed_field(),
            );
            Outcome::Passed
        }
        "no canonical agents are declared and no harness expresses them" => {
            world.declaration.omit_agents_root = true;
            world.declaration.omit_agent_adapters = true;
            Outcome::Passed
        }
        "no canonical agents are declared" => {
            world.declaration.omit_agents_root = true;
            Outcome::Passed
        }
        "no harness declares an agent adapter" => {
            world.declaration.omit_agent_adapters = true;
            Outcome::Passed
        }
        "no canonical skills are declared" => {
            world.declaration.omit_skills_root = true;
            Outcome::Passed
        }
        "a harness translates a capability it does not name" => {
            world.declaration.unnamed_translation = true;
            Outcome::Passed
        }
        "a harness translates a capability the repository never declared" => {
            world.declaration.undeclared_translation = true;
            Outcome::Passed
        }
        "no capability server is required and no harness declares a capability file" => {
            world.declaration.omit_required_server = true;
            world.declaration.omit_capability_files = true;
            Outcome::Passed
        }
        "no capability server is required" => {
            world.declaration.omit_required_server = true;
            Outcome::Passed
        }
        "no harness declares a capability file" => {
            world.declaration.omit_capability_files = true;
            Outcome::Passed
        }
        "file {string} holds bytes that are not text" => {
            // Written as an ordinary file first so the walk lists it. What
            // makes it interesting is only what a reader finds inside.
            let path = matched.string(0).to_string();
            world.files.insert(path.clone(), String::new());
            world.binary.insert(path);
            Outcome::Passed
        }
        "file {string} cannot be opened" => {
            let path = matched.string(0).to_string();
            world.files.insert(path.clone(), String::new());
            world.unreadable.insert(path);
            Outcome::Passed
        }
        "a loose file sits directly under the canonical skills root" => {
            world.files.insert(
                format!("{}/README.md", harness::SKILLS_ROOT),
                "# Skills\n".to_string(),
            );
            Outcome::Passed
        }
        "the canonical agent carries no declaration" => {
            world.files.insert(
                harness::canonical_agent(),
                harness::document_without_declaration(),
            );
            Outcome::Passed
        }
        "the agent adapter for {string} carries no declaration" => {
            world.files.insert(
                harness::agent_adapter(matched.string(0)),
                harness::document_without_declaration(),
            );
            Outcome::Passed
        }
        "the skill wrapper for {string} carries no declaration" => {
            world.files.insert(
                harness::skill_wrapper(matched.string(0)),
                harness::document_without_declaration(),
            );
            Outcome::Passed
        }
        "the agent directory for {string} holds the extra file {string}" => {
            world.files.insert(
                harness::agent_adapter_pattern(matched.string(0))
                    .replace("{name}.md", matched.string(1))
                    .replace("{name}.toml", matched.string(1)),
                "# Note\n".to_string(),
            );
            Outcome::Passed
        }
        "the repository holds a map entry whose target needs escaping" => {
            // Every character `quote` has an escape for that a single line can
            // carry. A message rendered as JSON has to survive its own content.
            world.files.insert(
                "rules/README.md".to_string(),
                "# Rules\n\n## Directory Map\n\n- [Odd](a\"b\\c\td\u{1}e)\n".to_string(),
            );
            Outcome::Passed
        }
        "stdout escapes the quote, backslash, tab, and control character" => {
            let stdout = &world.result().stdout;
            let missing: Vec<&str> = [r#"\""#, r"\\", r"\t", r"\u0001"]
                .into_iter()
                .filter(|escape| !stdout.contains(escape))
                .collect();
            expect(
                missing.is_empty(),
                format!("stdout carries no {missing:?}\nstdout: {stdout}"),
            )
        }
        "the canonical skill front matter is never closed" => {
            world.files.insert(
                harness::canonical_skill(),
                harness::skill_without_terminator(),
            );
            Outcome::Passed
        }
        "the capability declaration for harness {string} is not valid in its declared format" => {
            capability_of(world, matched.string(0), "{ not json".to_string())
        }
        "the capability declaration for harness {string} is missing the required capability" => {
            capability_of(
                world,
                matched.string(0),
                harness::capability_without_the_required_server(),
            )
        }
        "the capability declaration for harness {string} is a list of server groups" => {
            capability_of(
                world,
                matched.string(0),
                harness::capability_inside_a_list(),
            )
        }
        "the repository declares an unusable prohibited instruction source" => {
            world.declaration.overrides.insert(
                "harness-parity.prohibited-instruction-sources".to_string(),
                "[\"rules/[\"]".to_string(),
            );
            Outcome::Passed
        }
        "the repository declares the accessible palette" => {
            // Written by the step rather than inherited from the base fixture.
            // A scenario that names the palette it declares has to keep saying
            // so if the base one is ever changed underneath it -- otherwise the
            // step means whatever `fixtures.rs` happens to say this week.
            for (key, value) in [
                (
                    "md-mermaid.fill-colors",
                    "[\"#0173B2\", \"#DE8F05\", \"#029E73\"]",
                ),
                ("md-mermaid.edge-colors", "[\"#0173B2\", \"#000000\"]"),
                ("md-mermaid.text-colors", "[\"#000000\", \"#FFFFFF\"]"),
            ] {
                world
                    .declaration
                    .overrides
                    .insert(key.to_string(), value.to_string());
            }
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
        "harness {string} writes its capability as one command vector" => {
            let format = harness::format_for(&world.declaration, matched.string(0));
            world.files.insert(
                harness::capability_file(matched.string(0), &format),
                harness::capability_as_one_vector(&format),
            );
            Outcome::Passed
        }
        "the repository writes its route template with uneven spacing" => {
            world.declaration.padded_route_template = true;
            Outcome::Passed
        }
        "every harness closes its agent adapter declaration" => {
            world.declaration.closed_agent_adapters = true;
            Outcome::Passed
        }
        "the agent adapter for {string} wraps its route across lines" => {
            world.files.insert(
                harness::agent_adapter(matched.string(0)),
                harness::agent_with_a_wrapped_route(matched.string(0)),
            );
            Outcome::Passed
        }
        "the repository counts words as {string}" => {
            world.declaration.word_rule = Some(matched.string(0).to_string());
            Outcome::Passed
        }
        "file {string} holds a bolded word and a URL" => {
            // Two whitespace-separated fields, and six runs of letters and
            // digits. Which number the budget is measured against is the whole
            // difference between the two rules.
            world.files.insert(
                matched.string(0).to_string(),
                "**bold** https://example.com/a/b\n".to_string(),
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
            // The route sits mid-sentence on a later line, not alone on the
            // first. A one-line fixture would pin neither which prose lines the
            // Markdown path reads nor how it matches within one, and both
            // became load-bearing when fenced examples stopped counting.
            world.files.insert(
                "notes.md".to_string(),
                format!(
                    "# Notes\n\nEverything else lives in {}, always.\n",
                    harness::import(harness::INSTRUCTION).trim()
                ),
            );
            Outcome::Passed
        }
        "a documentation page shows the canonical import inside a fenced example" => {
            // Fenced, and surrounded by prose, so a fix that simply stopped
            // reading a file after its first fence would still pass this.
            world.files.insert(
                "docs/adapters.md".to_string(),
                format!(
                    "# Adapters\n\nAn adapter contains the import and nothing else:\n\n```sh\n{}```\n\nAnything more is a second instruction source.\n",
                    harness::import(harness::INSTRUCTION)
                ),
            );
            Outcome::Passed
        }
        "a documentation page shows the canonical import in an inline code span" => {
            // Backticks around the route, in the middle of a sentence a page
            // about adapters would actually write. A harness that expands `@`
            // imports does not expand one written inside a code span either.
            world.files.insert(
                "docs/spans.md".to_string(),
                format!(
                    "# Adapters\n\nWrite `{}` into the adapter and nothing else.\n",
                    harness::import(harness::INSTRUCTION).trim()
                ),
            );
            Outcome::Passed
        }
        "a documentation page shows the canonical import in a code span containing a backtick" => {
            // A doubled opener, because the span's own content contains a
            // backtick -- which is the case the run length exists for. Closing
            // at the inner backtick instead would end the span early and spill
            // the route into prose, so this fixture is what makes the length
            // rule observable rather than merely present.
            world.files.insert(
                "docs/doubled.md".to_string(),
                format!(
                    "# Adapters\n\nWritten ``a ` b {}`` it is an example, not an import.\n",
                    harness::import(harness::INSTRUCTION).trim()
                ),
            );
            Outcome::Passed
        }
        "a documentation page carries a stray backtick before the canonical import" => {
            // One backtick with no partner. It opens nothing, so the rest of
            // the line is still prose and the route in it is still an import.
            world.files.insert(
                "docs/stray.md".to_string(),
                format!(
                    "# Notes\n\nA lone ` backtick, and then {} on the same line.\n",
                    harness::import(harness::INSTRUCTION).trim()
                ),
            );
            Outcome::Passed
        }
        "a documentation page hides the canonical import behind an unclosed fence" => {
            // The four-backtick opener cannot be closed by the three-backtick
            // line beneath it, so every later line reads as an example. That is
            // the correct Markdown reading and a poor reason to stop looking: a
            // harness reads the file as text and will import all the same.
            world.files.insert(
                "docs/hidden.md".to_string(),
                format!(
                    "# Notes\n\n````text\nan example\n```\n\n{}\n\nMore rules.\n",
                    harness::import(harness::INSTRUCTION).trim()
                ),
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
        "the skill wrapper for {string} has a stale description" => {
            world.files.insert(
                harness::skill_wrapper(matched.string(0)),
                harness::wrapper_with_stale_description(),
            );
            Outcome::Passed
        }
        "the skill wrapper for {string} has an extra body" => {
            world.files.insert(
                harness::skill_wrapper(matched.string(0)),
                harness::wrapper_with_extra_body(),
            );
            Outcome::Passed
        }
        "the skill wrapper for {string} declares more than its route" => {
            world.files.insert(
                harness::skill_wrapper(matched.string(0)),
                harness::wrapper_with_extra_declaration(),
            );
            Outcome::Passed
        }
        "an unexpected skill wrapper exists for {string}" => {
            world.files.insert(
                harness::unexpected_skill_wrapper(matched.string(0)),
                harness::undeclared_wrapper(),
            );
            Outcome::Passed
        }
        "the canonical skill carries no declaration" => {
            world.files.insert(
                harness::canonical_skill(),
                harness::skill_without_declaration(),
            );
            Outcome::Passed
        }
        "the canonical skill declares no description" => {
            world.files.insert(
                harness::canonical_skill(),
                harness::skill_without_description(),
            );
            Outcome::Passed
        }
        "a canonical skill declares a name that is not its directory" => {
            let (path, body) = harness::misnamed_skill();
            world.files.insert(path, body);
            Outcome::Passed
        }
        "the canonical instruction body is absent" => {
            if world.files.remove(harness::INSTRUCTION).is_none() {
                return Outcome::Failed("there was no instruction body to remove".to_string());
            }
            Outcome::Passed
        }
        "the declared instruction adapter is absent" => {
            if world.files.remove(harness::ADAPTER).is_none() {
                return Outcome::Failed("there was no instruction adapter to remove".to_string());
            }
            Outcome::Passed
        }
        "I add a file outside the canon" => {
            let (path, body) = harness::uncounted_file();
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
                harness::unexpected_agent_adapter(&first),
                harness::undeclared_agent(&first),
            );
            Outcome::Passed
        }
        "the agent adapter for {string} contains extra prompt instructions" => {
            world.files.insert(
                harness::agent_adapter(matched.string(0)),
                harness::agent_with_extra_prompt(matched.string(0)),
            );
            Outcome::Passed
        }
        "the agent adapter for {string} weakens a denied capability" => {
            world.files.insert(
                harness::agent_adapter(matched.string(0)),
                harness::agent_weakening_denial(matched.string(0)),
            );
            Outcome::Passed
        }
        "the canonical agent requires less and no adapter answers for it" => {
            // Both halves together, which is what makes this the case the
            // trigger has to get right: the obligation is gone from the canon
            // and unmet in every adapter, so a run that applied it anyway
            // would hold three harnesses to a rule nobody wrote.
            world
                .files
                .insert(harness::canonical_agent(), harness::agent_requiring_less());
            for name in harness::roster(&world.declaration) {
                world.files.insert(
                    harness::agent_adapter(&name),
                    harness::agent_without_a_matching_grant(&name),
                );
            }
            Outcome::Passed
        }
        "the canonical agent requires less than its adapters grant" => {
            world
                .files
                .insert(harness::canonical_agent(), harness::agent_requiring_less());
            Outcome::Passed
        }
        "the agent adapter for {string} lists its grants" => {
            world.files.insert(
                harness::agent_adapter(matched.string(0)),
                harness::agent_adapter_listing_its_grants(matched.string(0)),
            );
            Outcome::Passed
        }
        "the agent adapter for {string} names another agent" => {
            world.files.insert(
                harness::agent_adapter(matched.string(0)),
                harness::agent_with_a_wrong_name(matched.string(0)),
            );
            Outcome::Passed
        }
        "the agent adapter for {string} grants nothing answering a capability" => {
            world.files.insert(
                harness::agent_adapter(matched.string(0)),
                harness::agent_without_a_matching_grant(matched.string(0)),
            );
            Outcome::Passed
        }
        "the agent adapter for {string} withholds a required capability" => {
            world.files.insert(
                harness::agent_adapter(matched.string(0)),
                harness::agent_withholding_capability(matched.string(0)),
            );
            Outcome::Passed
        }
        "the agent adapter for {string} declares a field its harness forbids" => {
            world.files.insert(
                harness::agent_adapter(matched.string(0)),
                harness::agent_declaring_a_forbidden_field(matched.string(0)),
            );
            Outcome::Passed
        }
        "the agent adapter for {string} changes a field its harness fixes" => {
            world.files.insert(
                harness::agent_adapter(matched.string(0)),
                harness::agent_with_a_changed_fixed_field(matched.string(0)),
            );
            Outcome::Passed
        }
        "the canonical agent declares a constraint outside the declared vocabulary" => {
            // Canon and every adapter alike, so the only thing wrong with the
            // repository is the constraint's absence from the vocabulary.
            // Written to the canon alone: an adapter translates the canon
            // into its own vocabulary and never repeats it, so a constraint
            // nothing translates leaves every adapter untouched.
            world.files.insert(
                harness::canonical_agent(),
                harness::agent_constrained_by("no-network"),
            );
            Outcome::Passed
        }
        "the agent adapter for {string} ignores a declared constraint" => {
            world.files.insert(
                harness::agent_adapter(matched.string(0)),
                harness::agent_ignoring_constraint(matched.string(0)),
            );
            Outcome::Passed
        }
        "the canonical agent requires a capability outside the declared vocabulary" => {
            // Written to the canon alone, on the same reasoning as the
            // constraint above: no translation names it, so no adapter has
            // anything to say about it and the only fault left is the
            // vocabulary's.
            world.files.insert(
                harness::canonical_agent(),
                harness::agent_requiring("network-write"),
            );
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
        "the required-capability arguments for harness {string} diverge" => {
            let name = matched.string(0).to_string();
            let format = harness::format_for(&world.declaration, &name);
            world.files.insert(
                harness::capability_file(&name, &format),
                harness::divergent_capability_arguments(&format),
            );
            Outcome::Passed
        }
        "the capability declaration for harness {string} is absent" => {
            let name = matched.string(0).to_string();
            let format = harness::format_for(&world.declaration, &name);
            let path = harness::capability_file(&name, &format);
            if world.files.remove(&path).is_none() {
                return Outcome::Failed(format!("there was no declaration at `{path}` to remove"));
            }
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
        "an out-of-order pair of harness-parity violations exists" => {
            let Some(first) = harness::roster(&world.declaration).first().cloned() else {
                return Outcome::Failed("the roster names no harness".to_string());
            };
            // Deliberately one harness rather than two. Production emits the
            // missing adapter before the unexpected one, while sorted order is
            // the reverse -- so a run that does not sort produces a visibly
            // different order, which is the only arrangement that can fail.
            world.files.remove(&harness::agent_adapter(&first));
            world.files.insert(
                harness::unexpected_agent_adapter(&first),
                harness::undeclared_agent(&first),
            );
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
        "I invoke the CLI from the repository directory with {string}" => {
            // No `--root`: the tool has to find the repository it was started
            // in. Only the E2E adapter can be wrong about that, and it is the
            // one where the process really has a working directory.
            let arguments: Vec<String> = matched
                .string(0)
                .split('|')
                .map(|argument| argument.to_string())
                .collect();
            let run = world.driver.invoke(&world.repository(), &arguments);
            world.record(run);
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
            let run = world.driver.invoke(&world.repository(), &arguments);
            world.remembered_digest = digest_of(&run.result);
            world.record(run);
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
            names(world.result(), &[fixtures::CONFIG_PATH])
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
        "no output names an optional section" => {
            // The additive claim, asserted rather than assumed: a repository
            // that declares none of the sections a later release added must not
            // be told about them. The base fixture declares none, so this
            // sentence is about the configuration every other scenario is
            // built on.
            let result = world.result();
            let named: Vec<&str> = OPTIONAL_SECTIONS
                .into_iter()
                .filter(|section| {
                    result.stdout.contains(section) || result.stderr.contains(section)
                })
                .collect();
            expect(
                named.is_empty(),
                format!(
                    "output names {named:?}\nstdout: {}\nstderr: {}",
                    result.stdout, result.stderr
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
        "{int} Mermaid diagrams were inspected" => inspected(world, matched.integer(0), "diagram"),
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
        "harness-parity validation succeeds with {int} harnesses, {int} skill, {int} agent, and {int} reconciled capability declarations" =>
        {
            let result = world.result();
            if result.exit_code != 0 {
                return Outcome::Failed(format!(
                    "expected a clean run, got exit {}\nstderr: {}",
                    result.exit_code, result.stderr
                ));
            }
            let expected = format!(
                "canon {} harnesses, {} skills, {} agents, {} reconciled capability declarations",
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
        "the harness-parity violation for {string} is {string}" => {
            // Kind and path together. A kind asserted on its own passes for a
            // repository where the two labels were exchanged, which mislabels
            // every finding a consumer filters on.
            let prefix = format!("[harness-parity] {}: ", matched.string(0));
            let kind = matched.string(1);
            let result = world.result();
            match result.stderr.lines().find(|line| line.starts_with(&prefix)) {
                Some(line) => expect(
                    line[prefix.len()..].starts_with(&format!("{kind}: ")),
                    format!("the violation is not a `{kind}`: {line}"),
                ),
                None => Outcome::Failed(format!(
                    "no violation names `{}`\nstderr: {}",
                    matched.string(0),
                    result.stderr
                )),
            }
        }
        "{int} capability declarations were reconciled" => {
            // Asserted on a run that has a finding, which is the only kind of
            // run where a counter that always increments differs from one that
            // counts what actually reconciled.
            let expected = matched.integer(0);
            let result = world.result();
            let needle = format!("{expected} reconciled capability declarations");
            expect(
                result.stdout.contains(&needle),
                format!(
                    "expected `{needle}` in the summary\nstdout: {}",
                    result.stdout
                ),
            )
        }
        "stdout names the command {string}" => {
            let needle = matched.string(0);
            let stdout = &world.result().stdout;
            expect(
                stdout.contains(needle),
                format!("help does not list `{needle}`\nstdout: {stdout}"),
            )
        }
        "stdout does not name the command {string}" => {
            // Help that lists every leaf whatever was asked for is not scoped
            // help, and no positive assertion can tell the two apart.
            let needle = matched.string(0);
            let stdout = &world.result().stdout;
            expect(
                !stdout.contains(needle),
                format!(
                    "help lists `{needle}`, which is outside the path that asked for it\nstdout: {stdout}"
                ),
            )
        }
        "stdout names every exit code" => {
            // Help that lists commands but not what their codes mean is help a
            // script author still has to read the source for.
            let stdout = &world.result().stdout;
            let missing: Vec<&str> = ["0  ", "1  ", "2  "]
                .into_iter()
                .filter(|code| !stdout.contains(code))
                .collect();
            expect(
                missing.is_empty(),
                format!("help explains no exit code {missing:?}\nstdout: {stdout}"),
            )
        }
        "stderr names a file it could not read" => {
            let result = world.result();
            let named: Vec<&String> = world
                .unreadable
                .iter()
                .filter(|path| result.stderr.contains(path.as_str()))
                .collect();
            expect(
                !named.is_empty(),
                format!(
                    "stderr names none of the {} sealed files\nstderr: {}",
                    world.unreadable.len(),
                    result.stderr
                ),
            )
        }
        "{int} links were inspected" => inspected(world, matched.integer(0), "link"),
        "{int} files were inspected" => inspected(world, matched.integer(0), "file"),
        "{int} directories were inspected" => inspected(world, matched.integer(0), "director"),
        "stdout is empty" => {
            let result = world.result();
            expect(
                result.stdout.is_empty(),
                format!("stdout is not empty: {}", result.stdout),
            )
        }
        "stdout lines start with {string}" => {
            prefixed(&world.result().stdout, matched.string(0), "stdout")
        }
        "stderr lines start with {string}" => {
            prefixed(&world.result().stderr, matched.string(0), "stderr")
        }
        "stdout JSON property {string} is {int}" => {
            // String and integer captures are indexed independently, so the
            // first {int} is at 0 however many {string}s precede it.
            let key = matched.string(0);
            let expected = matched.integer(0);
            match json_number(&world.result().stdout, key) {
                Some(found) => expect(
                    found == expected,
                    format!("`{key}` is {found}, not {expected}"),
                ),
                None => Outcome::Failed(format!(
                    "stdout carries no numeric `{key}`\nstdout: {}",
                    world.result().stdout
                )),
            }
        }
        "stdout JSON has a {string} and a {int}-character hexadecimal {string}" => {
            let present = matched.string(0);
            let sized = matched.string(1);
            let width = matched.integer(0);
            let stdout = &world.result().stdout;
            let Some(value) = json_string(stdout, present) else {
                return Outcome::Failed(format!("stdout carries no `{present}`\nstdout: {stdout}"));
            };
            if value.is_empty() {
                return Outcome::Failed(format!("`{present}` is empty"));
            }
            let Some(found) = json_string(stdout, sized) else {
                return Outcome::Failed(format!("stdout carries no `{sized}`\nstdout: {stdout}"));
            };
            // Not merely the right length: forty zeros is what a build made
            // outside a repository reports, and a scenario that accepted it
            // could not tell an embedded revision from a missing one.
            if found.chars().count() != width {
                return Outcome::Failed(format!(
                    "`{sized}` is {} characters, not {width}",
                    found.chars().count()
                ));
            }
            if !found.chars().all(|c| c.is_ascii_hexdigit()) {
                return Outcome::Failed(format!("`{sized}` is not hexadecimal: {found}"));
            }
            expect(
                found.chars().any(|c| c != '0'),
                format!("`{sized}` is all zeros, which is the no-repository fallback"),
            )
        }
        "the reported version is the one the product manifest declares" => {
            // Read out of the manifest text rather than through
            // `env!("CARGO_PKG_VERSION")`, which inside this crate is the very
            // constant `version()` prints -- comparing a value to itself proves
            // that a build is self-consistent and nothing else. The manifest is
            // where a release is cut from, so it is the independent side.
            //
            // Shape is not identity: `v9.9.9` satisfies every spelling rule and
            // is still the wrong release.
            let stdout = &world.result().stdout;
            let Some(reported) = json_string(stdout, "version") else {
                return Outcome::Failed(format!("stdout carries no `version`\nstdout: {stdout}"));
            };
            let expected = format!("v{}", manifest_version());
            expect(
                reported == expected,
                format!("the build reports `{reported}`, and the manifest declares `{expected}`"),
            )
        }
        "stdout is exactly the release identity envelope" => {
            // Field order and spacing are part of the release contract, not a
            // formatting detail: the consumer bootstrap compares this string to
            // an identity it assembles from its lock file. Parsed by hand so
            // that a reordered key, an added field, or a space after a colon is
            // a failure rather than a difference no assertion can see.
            let stdout = world.result().stdout.trim_end_matches('\n').to_string();
            let opening = "{\"schemaVersion\":1,\"version\":\"";
            let middle = "\",\"commit\":\"";
            let closing = "\"}";
            let Some(rest) = stdout.strip_prefix(opening) else {
                return Outcome::Failed(format!(
                    "stdout does not open `{opening}`\nstdout: {stdout}"
                ));
            };
            let Some(rest) = rest.strip_suffix(closing) else {
                return Outcome::Failed(format!(
                    "stdout does not close `{closing}`\nstdout: {stdout}"
                ));
            };
            let Some((version, commit)) = rest.split_once(middle) else {
                return Outcome::Failed(format!(
                    "stdout does not join with `{middle}`\nstdout: {stdout}"
                ));
            };
            expect(
                !version.contains('"') && !commit.contains('"'),
                format!("the envelope carries more than the two fields\nstdout: {stdout}"),
            )
        }
        "the reported version is spelled as its release tag" => {
            // The release tag is what a consumer pins and what the lock file
            // holds, so the executable has to report the tag verbatim rather
            // than something a reader has to convert into one. Cargo cannot
            // carry the `v`, which is exactly why this is asserted here.
            let stdout = &world.result().stdout;
            let Some(version) = json_string(stdout, "version") else {
                return Outcome::Failed(format!("stdout carries no `version`\nstdout: {stdout}"));
            };
            let Some(number) = version.strip_prefix('v') else {
                return Outcome::Failed(format!(
                    "`{version}` is not spelled as a tag; a release tag starts with `v`"
                ));
            };
            // Three numeric parts and nothing else. A prerelease tag such as
            // `v0.2.0-rc.1` fails here, deliberately: this project publishes
            // releases and nothing else, and a predicate widened for a shape
            // nobody ships would stop catching the shapes that go wrong.
            let parts: Vec<&str> = number.split('.').collect();
            expect(
                parts.len() == 3
                    && parts
                        .iter()
                        .all(|part| !part.is_empty() && part.chars().all(|c| c.is_ascii_digit())),
                format!("`{version}` is not `v` followed by a three-part version"),
            )
        }
        "the first stdout JSON violation kind is {string}" => {
            let expected = matched.string(0);
            match first_violation(&world.result().stdout).and_then(|v| json_string(&v, "kind")) {
                Some(kind) => expect(
                    kind == expected,
                    format!("the first violation is `{kind}`, not `{expected}`"),
                ),
                None => Outcome::Failed(format!(
                    "stdout carries no violation\nstdout: {}",
                    world.result().stdout
                )),
            }
        }
        "the first stdout JSON legibility fields are {string}, {int}, and {int}" => {
            let Some(violation) = first_violation(&world.result().stdout) else {
                return Outcome::Failed(format!(
                    "stdout carries no violation\nstdout: {}",
                    world.result().stdout
                ));
            };
            // Three fields together, because a measurement is only meaningful
            // beside the limit it was compared against and the segment it was
            // taken from.
            let segment = json_string(&violation, "segment");
            let measured = json_number(&violation, "measured");
            let limit = json_number(&violation, "limit");
            let wanted = (
                Some(matched.string(0).to_string()),
                Some(matched.integer(0)),
                Some(matched.integer(1)),
            );
            expect(
                (segment.clone(), measured, limit) == wanted,
                format!(
                    "the violation measures {segment:?}, {measured:?}, {limit:?}, not {wanted:?}"
                ),
            )
        }
        "the harness-parity digest is unchanged" => {
            let Some(remembered) = world.remembered_digest.clone() else {
                return Outcome::Failed("no digest was remembered".to_string());
            };
            match digest_of(world.result()) {
                Some(current) => expect(
                    current == remembered,
                    format!("the digest changed from `{remembered}` to `{current}`"),
                ),
                None => Outcome::Failed(format!(
                    "stdout carries no digest\nstdout: {}",
                    world.result().stdout
                )),
            }
        }
        "stderr names the unreadable capability file" => {
            let Some(path) = world.unreadable.iter().next().cloned() else {
                return Outcome::Failed("the scenario sealed no file".to_string());
            };
            let result = world.result();
            expect(
                result.stderr.contains(&path),
                format!("stderr does not name `{path}`\nstderr: {}", result.stderr),
            )
        }
        "{int} harnesses were inspected" => inspected(world, matched.integer(0), "harness"),
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
        "the repository is unchanged by the inspection" => {
            // The adapter observed the repository on both sides of every
            // invocation this scenario made. Anything listed here is a path the
            // command created, removed, or rewrote.
            expect(
                world.mutations.is_empty(),
                format!(
                    "the inspection changed the repository: {:?}",
                    world.mutations
                ),
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

/// Append one front-matter surface, written as the flow mapping a repository
/// would commit. Assembled here rather than in five arms so the glob and the
/// braces are spelled once.
fn push_frontmatter_surface<D: Driver>(world: &mut World<D>, glob: &str, rules: &str) -> Outcome {
    world
        .declaration
        .frontmatter_surfaces
        .push(format!("{{glob: \"{glob}\", {rules}}}"));
    Outcome::Passed
}

/// Append one heading surface and state the section's two rules.
///
/// The rules are section-level rather than per surface, so a scenario declaring
/// two surfaces states the same pair twice and means it once.
fn push_heading_surface<D: Driver>(
    world: &mut World<D>,
    glob: &str,
    single_h1: bool,
    max_jump: usize,
) -> Outcome {
    world
        .declaration
        .heading_surfaces
        .push(format!("{{glob: \"{glob}\"}}"));
    world.declaration.heading_single_h1 = Some(single_h1);
    world.declaration.heading_max_jump = Some(max_jump);
    Outcome::Passed
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
    let contract = harness::valid_contract(&world.declaration);
    // Prune first. A declaration that moves a harness from one capability
    // format to another, or takes its command directory away, leaves the file
    // the previous shape owned behind; a repository holding both is not the one
    // the scenario declared, and the day an unexpected-file rule exists it
    // would fail for a reason nobody wrote.
    let owned = harness::contract_paths();
    world
        .files
        .retain(|path, _| !owned(path) || contract.contains_key(path));
    for (path, body) in contract {
        world.files.insert(path, body);
    }
}

/// Declare one field of a harness's own settings file off limits.
///
/// The file itself is legitimate -- it is that harness's settings, and holds
/// everything else the harness needs -- so the repository carries it whether or
/// not the prohibited key is there.
fn declare_prohibited_field<D: Driver>(world: &mut World<D>, format: &'static str) -> Outcome {
    world.declaration.prohibited_field_format = Some(format);
    let file = harness::prohibited_field_file(&world.declaration);
    let body = match format {
        "toml" => "model = \"inherit\"\n".to_string(),
        _ => "{\n  \"model\": \"inherit\"\n}\n".to_string(),
    };
    world.files.insert(file.to_string(), body);
    Outcome::Passed
}

/// That settings file with the prohibited key written in one shape.
///
/// The shapes are the ones a configuration language can hold, because "does
/// this key say anything" is a question about the value's shape rather than
/// about its text: a rule that answered it for lists alone would let the same
/// instructions through written as a string. Every value below is spelled the
/// way both formats spell it, so one table of shapes serves both.
fn settings_holding(declaration: &crate::world::Declaration, shape: &str) -> Option<String> {
    let field = harness::PROHIBITED_FIELD;
    const SHAPES: [(&str, &str); 8] = [
        ("a non-empty list", "[\"always read this first\"]"),
        ("a non-empty text", "\"always read this first\""),
        ("a non-empty table", "{ \"always\": \"read this first\" }"),
        ("a number", "7"),
        ("an empty list", "[]"),
        ("blank text", "\"   \""),
        ("an empty table", "{}"),
        ("nothing", "null"),
    ];
    let value = SHAPES.iter().find(|(name, _)| *name == shape)?.1;
    Some(if declaration.prohibited_field_format == Some("toml") {
        format!("model = \"inherit\"\n{field} = {value}\n")
    } else {
        format!("{{\n  \"model\": \"inherit\",\n  \"{field}\": {value}\n}}\n")
    })
}

/// Replace one harness's capability declaration, in the format it declares.
fn capability_of<D: Driver>(world: &mut World<D>, harness_name: &str, body: String) -> Outcome {
    let format = harness::format_for(&world.declaration, harness_name);
    world
        .files
        .insert(harness::capability_file(harness_name, &format), body);
    Outcome::Passed
}

/// Every non-empty line of a stream begins with `prefix`.
fn prefixed(stream: &str, prefix: &str, name: &str) -> Outcome {
    let offending: Vec<&str> = stream
        .lines()
        .filter(|line| !line.is_empty() && !line.starts_with(prefix))
        .collect();
    if stream.lines().all(str::is_empty) {
        return Outcome::Failed(format!("{name} is empty, so nothing carries `{prefix}`"));
    }
    expect(
        offending.is_empty(),
        format!("{name} lines do not start with `{prefix}`: {offending:?}"),
    )
}

/// A numeric JSON member, read without a parser.
///
/// The corpus asserts on a handful of scalars in a document this tool writes
/// itself, and a dependency here would let a bug in the writer be hidden by the
/// same bug in the reader.
fn json_number(text: &str, key: &str) -> Option<usize> {
    let needle = format!("\"{key}\":");
    // Scoped to the outer object, so a member of the same name inside a
    // violation cannot answer a question asked about the run.
    let outer = text.split("\"violations\":[").next().unwrap_or(text);
    let after = outer
        .split(&needle)
        .nth(1)
        .or_else(|| text.rsplit(']').next()?.split(&needle).nth(1))?;
    after
        .chars()
        .take_while(char::is_ascii_digit)
        .collect::<String>()
        .parse()
        .ok()
}

fn json_string(text: &str, key: &str) -> Option<String> {
    let needle = format!("\"{key}\":\"");
    let after = text.split(&needle).nth(1)?;
    // Terminated on the first *unescaped* quote. The writer escapes quotes
    // inside a value, and a reader that stopped at the first `"` would silently
    // truncate every message that contains one.
    let mut value = String::new();
    let mut characters = after.chars();
    while let Some(character) = characters.next() {
        match character {
            '"' => return Some(value),
            '\\' => match characters.next() {
                Some('n') => value.push('\n'),
                Some(escaped) => value.push(escaped),
                None => return None,
            },
            other => value.push(other),
        }
    }
    None
}

/// The first object inside the `violations` array, as text.
fn first_violation(text: &str) -> Option<String> {
    let after = text.split("\"violations\":[").nth(1)?;
    // Braces are balanced rather than split on, and a brace inside a string
    // value does not count -- otherwise a message containing `}` would end the
    // record early and the fields after it would read as absent.
    let mut depth = 0usize;
    let mut inside_string = false;
    let mut escaped = false;
    let mut record = String::new();
    for character in after.chars() {
        if depth > 0 {
            record.push(character);
        }
        match character {
            _ if escaped => escaped = false,
            '\\' if inside_string => escaped = true,
            '"' => inside_string = !inside_string,
            '{' if !inside_string => depth += 1,
            '}' if !inside_string => {
                depth -= 1;
                if depth == 0 {
                    record.pop();
                    return Some(record);
                }
            }
            _ => {}
        }
    }
    None
}

/// The summary line's inspected count, named as the scenario named it.
///
/// The noun is checked as well as the number. A leaf reports what it looked at
/// -- `checked 2 files`, `checked 1 directory` -- and a scenario that says
/// "2 files were inspected" is making a claim about both halves. Asserting only
/// the number lets a leaf report the wrong subject entirely and stay green.
///
/// `stem` is the shared prefix of the singular and the plural, because the
/// renderer picks between them by count: `directory`/`directories` share
/// `director`, and pinning either spelling alone would break at the other count.
fn inspected<D: Driver>(world: &World<D>, expected: usize, stem: &str) -> Outcome {
    let stdout = world.result().stdout.clone();
    let Some(counted) = inspected_count(world.result()) else {
        return Outcome::Failed(format!("stdout carries no summary\nstdout: {stdout}"));
    };
    if counted != expected {
        return Outcome::Failed(format!(
            "expected {expected} {stem}(s) inspected, the summary says {counted}\nstdout: {stdout}"
        ));
    }
    expect(
        stdout
            .lines()
            .any(|line| line.contains(&format!("checked {expected} {stem}"))),
        format!("the summary does not call what it checked `{stem}`\nstdout: {stdout}"),
    )
}

/// The digest a harness-parity run reports.
fn digest_of(result: &CommandResult) -> Option<String> {
    result
        .stdout
        .lines()
        .find_map(|line| line.split_once("] digest "))
        .map(|(_, digest)| digest.trim().to_string())
}

/// The version the product's own manifest declares, read as text.
///
/// The same seam `xtask` uses to verify a release artifact, for the same
/// reason: it is the one statement of the version that does not come from the
/// binary being asserted about. `[workspace]` ends the product's table, so a
/// member crate's version can never be picked up instead.
fn manifest_version() -> String {
    let manifest = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("Cargo.toml");
    let text = std::fs::read_to_string(&manifest)
        .unwrap_or_else(|error| panic!("{}: {error}", manifest.display()));
    text.lines()
        .take_while(|line| !line.starts_with("[workspace]"))
        .find_map(|line| line.strip_prefix("version = "))
        .map(|value| value.trim().trim_matches('"').to_string())
        .expect("the product manifest declares a version")
}
