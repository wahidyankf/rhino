//! The configuration fixtures scenarios build on.
//!
//! `specs/behaviours/README.md` states the convention these implement: a
//! declaring `Given` is additive over a complete, valid configuration, a
//! scenario-level declaration replaces the base value for the same key, and
//! `an empty repository` speaks about the tree rather than the configuration.
//!
//! The base below is fixture data, not a shipped default. RHINO itself holds no
//! value in it -- that is the whole point of the extraction -- so it lives here,
//! where a reader can see every value a scenario silently relies on.

use crate::harness;
use crate::world::Declaration;
use std::collections::BTreeMap;

pub const SCHEMA: &str = "rhino/repo-config/v1";
pub const CONFIG_PATH: &str = "repo-config.yml";

/// A complete, legal configuration in the declared schema.
///
/// Written as text rather than assembled from a type so that a scenario can
/// omit a required key or introduce an unknown one -- neither of which a typed
/// builder could express, and both of which the corpus asserts on.
fn base(declaration: &Declaration) -> BTreeMap<String, String> {
    let schema = declaration
        .schema
        .clone()
        .unwrap_or_else(|| SCHEMA.to_string());
    let mut lines: BTreeMap<String, String> = BTreeMap::new();

    lines.insert("schema".into(), schema);
    lines.insert(
        "governance-word-budget.count".into(),
        declaration
            .word_rule
            .clone()
            .unwrap_or_else(|| "letters-and-digits".into()),
    );
    lines.insert(
        "governance-word-budget.surfaces".into(),
        "[{glob: \"rules/**/*.md\", fail: 40}]".into(),
    );
    lines.insert(
        "governance-directory-map.trees".into(),
        "[{path: rules}]".into(),
    );
    lines.insert(
        "md-internal-link.exclude-sources".into(),
        format!(
            "[{}]",
            declaration
                .excluded_sources
                .iter()
                .map(|glob| format!("\"{glob}\""))
                .collect::<Vec<_>>()
                .join(", ")
        ),
    );
    lines.insert("md-mermaid.node-label-graphemes".into(), "32".into());
    lines.insert("md-mermaid.edge-label-graphemes".into(), "24".into());
    lines.insert(
        "md-mermaid.fill-colors".into(),
        "[\"#0173B2\", \"#DE8F05\", \"#029E73\"]".into(),
    );
    lines.insert(
        "md-mermaid.edge-colors".into(),
        "[\"#0173B2\", \"#000000\"]".into(),
    );
    lines.insert(
        "md-mermaid.text-colors".into(),
        "[\"#000000\", \"#FFFFFF\"]".into(),
    );
    if let (Some(single), Some(jump)) =
        (declaration.heading_single_h1, declaration.heading_max_jump)
    {
        lines.insert(
            "md-heading-hierarchy.surfaces".into(),
            format!("[{}]", declaration.heading_surfaces.join(", ")),
        );
        lines.insert("md-heading-hierarchy.single-h1".into(), single.to_string());
        lines.insert(
            "md-heading-hierarchy.max-level-jump".into(),
            jump.to_string(),
        );
    }
    if !declaration.emoji_prohibited.is_empty() {
        lines.insert(
            "convention-emoji.prohibited".into(),
            format!(
                "[{}]",
                declaration
                    .emoji_prohibited
                    .iter()
                    .map(|glob| format!("{{glob: \"{glob}\"}}"))
                    .collect::<Vec<_>>()
                    .join(", ")
            ),
        );
    }
    if !declaration.readme_index_trees.is_empty() {
        lines.insert(
            "md-readme-index.trees".into(),
            format!(
                "[{}]",
                declaration
                    .readme_index_trees
                    .iter()
                    .map(|path| format!("{{path: {path}}}"))
                    .collect::<Vec<_>>()
                    .join(", ")
            ),
        );
    }
    if !declaration.metadata_surfaces.is_empty() {
        lines.insert(
            "metadata.surfaces".into(),
            format!("[{}]", declaration.metadata_surfaces.join(", ")),
        );
    }
    if !declaration.frontmatter_surfaces.is_empty() {
        lines.insert(
            "md-frontmatter.surfaces".into(),
            format!("[{}]", declaration.frontmatter_surfaces.join(", ")),
        );
    }
    if !declaration.naming_surfaces.is_empty() {
        lines.insert(
            "md-naming.surfaces".into(),
            format!("[{}]", declaration.naming_surfaces.join(", ")),
        );
        lines.insert(
            "md-naming.exempt".into(),
            format!(
                "[{}]",
                declaration
                    .naming_exempt
                    .iter()
                    .map(|glob| format!("\"{glob}\""))
                    .collect::<Vec<_>>()
                    .join(", ")
            ),
        );
    }
    lines.insert(
        "harness-parity.canonical.instruction".into(),
        harness::instruction(declaration).into(),
    );
    if declaration.declares_instruction_adapter {
        lines.insert(
            "harness-parity.canonical.instruction-adapter".into(),
            harness::adapter(declaration).into(),
        );
    }
    if let Some(format) = declaration.prohibited_field_format {
        lines.insert(
            "harness-parity.prohibited-instruction-fields".into(),
            format!(
                "[{{ file: {}, format: {format}, field: {} }}]",
                harness::prohibited_field_file(declaration),
                harness::PROHIBITED_FIELD
            ),
        );
    }

    let roster = harness::roster(declaration);

    if !declaration.omit_skills_root
        && (!roster.is_empty() || declaration.canonical_skills_root.is_some())
    {
        lines.insert(
            "harness-parity.canonical.skill-route".into(),
            format!("\"{}\"", harness::SKILL_ROUTE),
        );
        lines.insert(
            "harness-parity.canonical.skills-root".into(),
            declaration
                .canonical_skills_root
                .clone()
                .unwrap_or_else(|| harness::SKILLS_ROOT.into()),
        );
    }
    if !declaration.omit_agents_root
        && (!roster.is_empty() || declaration.canonical_agents_root.is_some())
    {
        lines.insert(
            "harness-parity.canonical.agent-route".into(),
            match declaration.padded_route_template {
                true => format!("\"{}\"", harness::AGENT_ROUTE.replace(' ', "  ")),
                false => format!("\"{}\"", harness::AGENT_ROUTE),
            },
        );
        lines.insert(
            "harness-parity.canonical.agents-root".into(),
            declaration
                .canonical_agents_root
                .clone()
                .unwrap_or_else(|| harness::AGENTS_ROOT.into()),
        );
    }

    if !declaration.omit_declaration_shape
        && (declaration.keep_declaration_shape
            || (!declaration.omit_agents_root
                && (!roster.is_empty() || declaration.canonical_agents_root.is_some())))
    {
        let [grants, denials, limits] = harness::declaration_names(declaration);
        lines.insert(
            "harness-parity.canonical.declaration".into(),
            format!(
                "{{grants: {grants}, denials: {denials}, limits: {limits}, \
                 fixed: {{{}: {}}}}}",
                harness::FIXED_FIELD,
                harness::FIXED_VALUE
            ),
        );
    }

    let declares_a_server = !declaration.omit_required_server
        && (!roster.is_empty() || declaration.declares_required_mcp);
    let declares_capability_files = !declaration.omit_capability_files
        && (!roster.is_empty() || declaration.declares_required_mcp);

    let entries: Vec<String> = roster
        .iter()
        .map(|name| {
            let format = harness::format_for(declaration, name);
            let mut entry = format!("{{name: {name}");
            if !declaration.omit_agent_adapters {
                entry.push_str(&format!(
                    ", agent-adapter: {}",
                    harness::agent_adapter_declaration_with(
                        name,
                        declaration.unnamed_translation,
                        declaration.undeclared_translation,
                        declaration.closed_agent_adapters,
                    )
                ));
            }
            // Per-harness, because that is how the schema states it: a second
            // harness gaining skill wrappers is one line of configuration.
            if declaration.command_directories.contains(name) {
                entry.push_str(&format!(
                    ", skill-adapter: {}",
                    harness::skill_adapter_declaration(name)
                ));
            }
            // Present exactly when a required server is, which is what the
            // schema pairs: a declaration nothing reads and a rule nothing
            // satisfies are the same fault seen from two sides.
            if declares_capability_files {
                entry.push_str(&format!(
                    ", capability: {{file: {}, format: {format}}}",
                    harness::capability_file(name, &format)
                ));
            }
            entry.push('}');
            entry
        })
        .collect();
    lines.insert(
        "harness-parity.harnesses".into(),
        format!("[{}]", entries.join(", ")),
    );
    lines.insert(
        "harness-parity.prohibited-instruction-sources".into(),
        format!(
            "[\"**/{}\", \"**/{}\"]",
            harness::INSTRUCTION,
            harness::ADAPTER
        ),
    );
    lines.insert(
        "harness-parity.capabilities".into(),
        "[repository-read, repository-write, shell]".into(),
    );
    lines.insert(
        "harness-parity.constraints".into(),
        "[inline-result-only]".into(),
    );
    // An empty roster still has nothing to say about a required server, on the
    // same rule as the canonical roots. A non-empty one merely *may* declare
    // it, which is why a scenario can take it away.
    if declares_a_server {
        lines.insert(
            "harness-parity.required-mcp.name".into(),
            "toolserver".into(),
        );
        lines.insert(
            "harness-parity.required-mcp.command".into(),
            "toolrunner".into(),
        );
        lines.insert(
            "harness-parity.required-mcp.args".into(),
            "[\"serve\"]".into(),
        );
    }
    let excluded = if declaration.excluded_directories.is_empty() {
        vec![
            ".git".to_string(),
            "build-output".to_string(),
            "dependencies".to_string(),
        ]
    } else {
        declaration.excluded_directories.clone()
    };
    lines.insert(
        "scan.exclude-directories".into(),
        format!("[{}]", excluded.join(", ")),
    );

    lines
}

/// Render the declaration as the YAML text a repository would commit.
pub fn render(declaration: &Declaration) -> String {
    let mut flat = base(declaration);

    // Prefix-aware: omitting `harness-parity.required-mcp` has to remove the
    // block, not just a key nothing was stored under. A scenario names the key
    // the schema names, and the flattening here is an implementation detail of
    // the fixture.
    for key in &declaration.omissions {
        let nested = format!("{key}.");
        flat.retain(|held, _| held != key && !held.starts_with(&nested));
    }
    for (key, value) in &declaration.overrides {
        flat.insert(key.clone(), quote_if_needed(value));
    }

    let schema = flat.remove("schema").unwrap_or_else(|| SCHEMA.to_string());
    let mut out = format!("# schema: {schema}\n");

    // Group the flattened dotted keys back into nested blocks, deepest last, so
    // the emitted document is the shape the contract describes rather than a
    // flat map that happens to parse.
    let mut sections: BTreeMap<String, BTreeMap<String, String>> = BTreeMap::new();
    for (key, value) in flat {
        let (section, rest) = key.split_once('.').unwrap_or((key.as_str(), ""));
        sections
            .entry(section.to_string())
            .or_default()
            .insert(rest.to_string(), value);
    }

    for (section, entries) in sections {
        out.push_str(&format!("\n{section}:\n"));
        let mut nested: BTreeMap<String, BTreeMap<String, String>> = BTreeMap::new();
        for (key, value) in &entries {
            match key.split_once('.') {
                Some((parent, child)) => {
                    nested
                        .entry(parent.to_string())
                        .or_default()
                        .insert(child.to_string(), value.clone());
                }
                None => out.push_str(&format!("  {key}: {value}\n")),
            }
        }
        for (parent, children) in nested {
            out.push_str(&format!("  {parent}:\n"));
            for (child, value) in children {
                out.push_str(&format!("    {child}: {value}\n"));
            }
        }
    }

    if let Some(mapping) = model_tiers(declaration) {
        // Written as one flow mapping rather than through the flattening above,
        // because the section nests three deep -- harness, tier, field -- and a
        // fixture that could only express two levels could not state the shape
        // the contract actually describes.
        out.push_str(&format!("\nmodel-tiers: {mapping}\n"));
    }

    for section in &declaration.extra_sections {
        out.push_str(&format!("\n{section}:\n  note: ignored by the contract\n"));
    }

    out
}

/// The model-tier section a scenario declared, if it declared one.
///
/// A literal declaration wins over stated pairs: a scenario reaching for the
/// literal form is saying something about a shape no well-formed pair can
/// express, and merging the two would quietly repair the thing under test.
fn model_tiers(declaration: &Declaration) -> Option<String> {
    if let Some(literal) = &declaration.model_tier_literal {
        return Some(literal.clone());
    }
    if declaration.model_tier_pairs.is_empty() {
        return None;
    }
    let mut harnesses: BTreeMap<String, Vec<String>> = BTreeMap::new();
    for (harness, tier, model, effort) in &declaration.model_tier_pairs {
        harnesses
            .entry(harness.clone())
            .or_default()
            .push(format!("{tier}: {{model: {model}, effort: {effort}}}"));
    }
    let body = harnesses
        .into_iter()
        .map(|(harness, tiers)| format!("{harness}: {{{}}}", tiers.join(", ")))
        .collect::<Vec<_>>()
        .join(", ");
    Some(format!("{{{body}}}"))
}

/// Values that would otherwise be read as YAML structure keep their quotes; a
/// flow sequence or mapping is passed through as written.
fn quote_if_needed(value: &str) -> String {
    let structural = value.starts_with('[') || value.starts_with('{');
    let numeric = value.parse::<i64>().is_ok();
    if structural || numeric {
        value.to_string()
    } else {
        format!("\"{value}\"")
    }
}
