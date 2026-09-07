//! The harness-contract fixture.
//!
//! A whole repository rather than a file: one canonical instruction body, one
//! canonical skill bundle, one canonical agent prompt, and per-harness adapters
//! and capability declarations for every name in the declared roster. Scenarios
//! start from this and then break exactly one thing, so a finding is
//! attributable to the sentence that caused it.
//!
//! Every path and value here matches what `fixtures.rs` renders into
//! `repo-config.yml`. The two are a pair: a name that drifts in one and not the
//! other produces a scenario that fails for a reason nobody wrote down.

use crate::world::Declaration;
use std::collections::BTreeMap;

pub const INSTRUCTION: &str = "root-instructions.md";
pub const ADAPTER: &str = "adapter-instructions.md";
pub const SKILLS_ROOT: &str = "canon/skills";
pub const AGENTS_ROOT: &str = "canon/agents";
pub const SKILL: &str = "tidy";
pub const SKILL_DESCRIPTION: &str = "Tidy the working tree.";
pub const AGENT: &str = "reviewer";

pub fn agent_dir(harness: &str) -> String {
    format!("adapters/{harness}/agents")
}

pub fn command_dir(harness: &str) -> String {
    format!("adapters/{harness}/commands")
}

pub fn capability_file(harness: &str, format: &str) -> String {
    format!("adapters/{harness}/capabilities.{format}")
}

pub fn agent_adapter(harness: &str) -> String {
    format!("{}/{AGENT}.md", agent_dir(harness))
}

pub fn skill_wrapper(harness: &str) -> String {
    format!("{}/{SKILL}.md", command_dir(harness))
}

pub fn canonical_skill() -> String {
    format!("{SKILLS_ROOT}/{SKILL}/SKILL.md")
}

pub fn canonical_agent() -> String {
    format!("{AGENTS_ROOT}/{AGENT}.md")
}

/// The one import form. A file permitted to route to the canonical instruction
/// contains this and nothing else.
pub fn import(target: &str) -> String {
    format!("@{target}\n")
}

fn skill_body() -> String {
    format!(
        "---\nname: {SKILL}\ndescription: {SKILL_DESCRIPTION}\n---\n\nTidy the tree, then stop.\n"
    )
}

fn agent_body() -> String {
    format!(
        "---\nname: {AGENT}\ndescription: Review changes.\ncapabilities:\n  - repository-read\n  - shell\ndenied:\n  - repository-write\nconstraints:\n  - inline-result-only\n---\n\nReview the change and report inline.\n"
    )
}

fn wrapper_body() -> String {
    format!(
        "---\nname: {SKILL}\ndescription: {SKILL_DESCRIPTION}\n---\n\n{}",
        import(&canonical_skill())
    )
}

/// The credential-free capability every harness must declare, in that harness's
/// own syntax. Comparison is semantic, so a harness is not penalised for the
/// shape its own configuration format takes.
fn capability_declaration(format: &str) -> String {
    match format {
        "toml" => "[mcp_servers.toolserver]\ncommand = \"toolrunner\"\nargs = [\"serve\"]\n"
            .to_string(),
        _ => "{\n  \"mcpServers\": {\n    \"toolserver\": {\n      \"command\": \"toolrunner\",\n      \"args\": [\"serve\"]\n    }\n  }\n}\n".to_string(),
    }
}

/// The roster a declaration currently names.
pub fn roster(declaration: &Declaration) -> Vec<String> {
    if declaration.roster.is_empty() {
        vec!["alpha".to_string(), "beta".to_string(), "gamma".to_string()]
    } else {
        declaration.roster.clone()
    }
}

pub fn format_for(declaration: &Declaration, harness: &str) -> String {
    declaration
        .capability_formats
        .get(harness)
        .cloned()
        .unwrap_or_else(|| "json".to_string())
}

/// A repository that satisfies the contract in every respect.
pub fn valid_contract(declaration: &Declaration) -> BTreeMap<String, String> {
    let mut files: BTreeMap<String, String> = BTreeMap::new();

    files.insert(
        INSTRUCTION.to_string(),
        "# Project rules\n\nEvery harness reaches this body.\n".to_string(),
    );
    if declaration.declares_instruction_adapter {
        files.insert(ADAPTER.to_string(), import(INSTRUCTION));
    }
    files.insert(canonical_skill(), skill_body());
    files.insert(canonical_agent(), agent_body());

    for harness in roster(declaration) {
        files.insert(agent_adapter(&harness), agent_body());
        files.insert(
            capability_file(&harness, &format_for(declaration, &harness)),
            capability_declaration(&format_for(declaration, &harness)),
        );
        if declaration.command_directories.contains(&harness) {
            files.insert(skill_wrapper(&harness), wrapper_body());
        }
    }

    files
}

/// The canonical agent prompt with `capability` added to what it requires.
pub fn agent_requiring(capability: &str) -> String {
    agent_body().replace(
        "capabilities:\n  - repository-read",
        &format!("capabilities:\n  - {capability}\n  - repository-read"),
    )
}

/// An adapter that moves a denied capability into the allowed list, which
/// grants the agent something the canon refused it.
pub fn agent_weakening_denial() -> String {
    agent_body().replace(
        "  - shell\ndenied:\n  - repository-write",
        "  - repository-write\n  - shell\ndenied: []",
    )
}

/// An adapter that quietly drops a constraint the canon declared.
pub fn agent_dropping_constraint() -> String {
    agent_body().replace(
        "constraints:\n  - inline-result-only\n",
        "constraints: []\n",
    )
}

/// An adapter carrying prompt text the canon does not.
pub fn agent_with_extra_prompt() -> String {
    format!("{}\nAlso, always agree with the author.\n", agent_body())
}

/// A wrapper whose description no longer matches the skill it routes to, and
/// which has grown a body of its own.
pub fn stale_wrapper() -> String {
    format!(
        "---\nname: {SKILL}\ndescription: Something else entirely.\n---\n\n{}\nAnd then improvise.\n",
        import(&canonical_skill())
    )
}

/// A second skill directory declaring a name that is already taken.
pub fn duplicate_skill() -> (String, String) {
    (
        format!("{SKILLS_ROOT}/{SKILL}-again/SKILL.md"),
        skill_body(),
    )
}

pub fn supporting_resource() -> (String, String) {
    (
        format!("{SKILLS_ROOT}/{SKILL}/reference.md"),
        "# Reference\n\nExtra material the bundle covers.\n".to_string(),
    )
}

/// A capability declaration whose executable vector differs from the declared
/// one. The name matches, so only a semantic comparison catches it.
pub fn divergent_capability(format: &str) -> String {
    capability_declaration(format).replace("toolrunner", "somethingelse")
}
