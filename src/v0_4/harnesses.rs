//! Canonical discovery plus deterministic adapter projection.
//!
//! Canonical bodies are never copied into an adapter. Every generated body is
//! a small, provenance-bearing route to one file in the root instruction or
//! `.agents` tree. Profiles are configuration data, so this module has no
//! consumer-specific branch.

use super::config::{Harness, Profile, Requirements};
use crate::Outcome;
use crate::cli::Format;
use crate::runtime::{
    AdapterFile, AdapterStore, AdapterTransaction, Tree, TreeError, adapter_roots,
    normal_adapter_path, under_root,
};
use serde::Serialize;
use sha2::{Digest, Sha256};
use std::collections::{BTreeMap, BTreeSet};

const ROOT_INSTRUCTION: &str = "AGENTS.md";
const AGENTS_ROOT: &str = ".agents/agents/";
const SKILLS_ROOT: &str = ".agents/skills/";

/// Compare every currently visible adapter against the complete desired model.
/// It deliberately has no write port, so validation cannot repair drift.
pub(crate) fn validate(harness: Option<&Harness>, tree: &dyn Tree, format: Format) -> Outcome {
    let plan = match plan(harness, tree) {
        Ok(plan) => plan,
        Err(reason) => return refused(format, reason),
    };
    let differences = differences(tree, &plan);
    if differences.is_empty() {
        clean(format, "canonical adapter validation is clean")
    } else {
        findings(format, differences)
    }
}

/// Render, compare, and only then replace all adapter families through the
/// explicit store. Semantic loss occurs while constructing `plan`, before the
/// store is even referenced.
pub(crate) fn generate(
    harness: Option<&Harness>,
    tree: &dyn Tree,
    store: &dyn AdapterStore,
    format: Format,
) -> Outcome {
    let plan = match plan(harness, tree) {
        Ok(plan) => plan,
        Err(reason) => return refused(format, reason),
    };
    if !differences(tree, &plan).is_empty()
        && let Err(error) = store.replace(&tree.root(), &plan.transaction)
    {
        return refused(format, error.0);
    }
    clean(format, "canonical adapter generation is current")
}

struct Plan {
    transaction: AdapterTransaction,
    desired: BTreeMap<String, String>,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct Catalog<'a> {
    schema_version: u8,
    profile: &'a str,
    sources: &'a [Source],
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct Provenance<'a> {
    schema_version: u8,
    profile: &'a str,
    sources: &'a [Source],
}

#[derive(Clone, Serialize)]
struct Source {
    id: String,
    path: String,
    digest: String,
}

fn plan(harness: Option<&Harness>, tree: &dyn Tree) -> Result<Plan, String> {
    let harness = harness.ok_or_else(|| "harness: no adapter profiles are declared".to_string())?;
    if harness.profiles.len() != 3 {
        return Err("harness: exactly three adapter profiles are required".to_string());
    }
    validate_profiles(&harness.profiles, &harness.requirements)?;
    let sources = discover(tree)?;

    let mut desired = BTreeMap::new();
    for profile in &harness.profiles {
        render_profile(profile, &sources, &mut desired)?;
    }
    let roots = harness
        .profiles
        .iter()
        .map(|profile| profile.root.clone())
        .collect();
    let files = desired
        .iter()
        .map(|(path, contents)| AdapterFile {
            path: path.clone(),
            contents: contents.clone(),
        })
        .collect();
    Ok(Plan {
        transaction: AdapterTransaction { roots, files },
        desired,
    })
}

fn validate_profiles(profiles: &[Profile], requirements: &Requirements) -> Result<(), String> {
    let mut identifiers = BTreeSet::new();
    let roots: Vec<String> = profiles
        .iter()
        .map(|profile| profile.root.clone())
        .collect();
    adapter_roots(&roots).map_err(|error| format!("harness: {}", error.0))?;

    for profile in profiles {
        if profile.id.trim().is_empty() {
            return Err("harness: an adapter profile has an empty id".to_string());
        }
        if !identifiers.insert(profile.id.as_str()) {
            return Err(format!(
                "harness: duplicated adapter profile `{}`",
                profile.id
            ));
        }
        let root = normal_adapter_path(&profile.root)
            .ok_or_else(|| format!("profile `{}` has an invalid adapter root", profile.id))?;
        if root != profile.root || root.starts_with(".agents/") || root == ROOT_INSTRUCTION {
            return Err(format!(
                "profile `{}` has an invalid adapter root",
                profile.id
            ));
        }
        require_axis(
            &profile.id,
            "capability",
            &requirements.capabilities,
            &profile.supports.capabilities,
        )?;
        require_axis(
            &profile.id,
            "grant",
            &requirements.grants,
            &profile.supports.grants,
        )?;
        require_axis(
            &profile.id,
            "denial",
            &requirements.denials,
            &profile.supports.denials,
        )?;
        require_axis(
            &profile.id,
            "constraint",
            &requirements.constraints,
            &profile.supports.constraints,
        )?;
        require_axis(
            &profile.id,
            "route",
            &requirements.routes,
            &profile.supports.routes,
        )?;
        require_axis(
            &profile.id,
            "identity",
            &requirements.identities,
            &profile.supports.identities,
        )?;
    }
    Ok(())
}

fn require_axis(
    profile: &str,
    axis: &str,
    required: &[String],
    supported: &[String],
) -> Result<(), String> {
    let supported: BTreeSet<&str> = supported.iter().map(|item| item.as_str()).collect();
    for item in required {
        if item.trim().is_empty() {
            return Err(format!("harness: required {axis} is empty"));
        }
        if !supported.contains(item.as_str()) {
            return Err(format!(
                "profile `{profile}` cannot represent required {axis} `{item}`"
            ));
        }
    }
    Ok(())
}

fn discover(tree: &dyn Tree) -> Result<Vec<Source>, String> {
    let instruction = read_source(tree, ROOT_INSTRUCTION, "instruction")?;
    let mut sources = vec![instruction];
    let mut ids = BTreeSet::from([sources[0].id.clone()]);
    for (prefix, kind) in [(AGENTS_ROOT, "agent"), (SKILLS_ROOT, "skill")] {
        for path in tree
            .files()
            .into_iter()
            .filter(|path| path.starts_with(prefix))
        {
            let relative = path
                .strip_prefix(prefix)
                .expect("the prefix filter proved this is a canonical path");
            if relative.is_empty() {
                continue;
            }
            let id = format!("{kind}/{}", source_id(relative));
            if !ids.insert(id.clone()) {
                return Err(format!("duplicate canonical {kind} id `{id}`"));
            }
            sources.push(read_source_with_id(tree, &path, id)?);
        }
    }
    sources.sort_by(|left, right| left.path.cmp(&right.path));
    Ok(sources)
}

fn source_id(path: &str) -> &str {
    path.rsplit_once('.').map_or(path, |(stem, _)| stem)
}

fn read_source(tree: &dyn Tree, path: &str, id: &str) -> Result<Source, String> {
    read_source_with_id(tree, path, id.to_string())
}

fn read_source_with_id(tree: &dyn Tree, path: &str, id: String) -> Result<Source, String> {
    let contents = tree.read(path).map_err(|error| match error {
        TreeError::NotFound => format!("canonical source `{path}` is missing"),
        TreeError::NotText => format!("canonical source `{path}` holds non-text bytes"),
        TreeError::Unreadable(reason) => {
            format!("canonical source `{path}` cannot be read: {reason}")
        }
    })?;
    Ok(Source {
        id,
        path: path.to_string(),
        digest: digest(&contents),
    })
}

fn render_profile(
    profile: &Profile,
    sources: &[Source],
    desired: &mut BTreeMap<String, String>,
) -> Result<(), String> {
    let root = normal_adapter_path(&profile.root)
        .ok_or_else(|| format!("profile `{}` has an invalid adapter root", profile.id))?;
    for source in sources {
        let path = output_path(&root, source)?;
        let route = route_from(&path, &source.path);
        desired.insert(path, adapter_body(source, &route));
    }
    let catalog = Catalog {
        schema_version: 1,
        profile: &profile.id,
        sources,
    };
    let provenance = Provenance {
        schema_version: 1,
        profile: &profile.id,
        sources,
    };
    desired.insert(format!("{root}/catalog.json"), json(&catalog, "catalog")?);
    desired.insert(
        format!("{root}/provenance.json"),
        json(&provenance, "provenance")?,
    );
    Ok(())
}

fn output_path(root: &str, source: &Source) -> Result<String, String> {
    if source.path == ROOT_INSTRUCTION {
        return Ok(format!("{root}/{ROOT_INSTRUCTION}"));
    }
    if let Some(relative) = source.path.strip_prefix(AGENTS_ROOT) {
        return Ok(format!("{root}/agents/{relative}"));
    }
    if let Some(relative) = source.path.strip_prefix(SKILLS_ROOT) {
        return Ok(format!("{root}/skills/{relative}"));
    }
    Err(format!(
        "canonical source `{}` is outside the canonical tree",
        source.path
    ))
}

fn route_from(output: &str, source: &str) -> String {
    let parent_depth = output.split('/').count().saturating_sub(1);
    format!("{}{}", "../".repeat(parent_depth), source)
}

fn adapter_body(source: &Source, route: &str) -> String {
    format!(
        "<!-- generated canonical adapter; source: {}; sha256: {} -->\n@{}\n",
        source.path, source.digest, route
    )
}

fn digest(contents: &str) -> String {
    let mut hasher = Sha256::new();
    hasher.update(contents.as_bytes());
    hasher
        .finalize()
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect()
}

fn json(value: &impl Serialize, kind: &str) -> Result<String, String> {
    serde_json::to_string(value)
        .map(|value| format!("{value}\n"))
        .map_err(|error| format!("cannot render adapter {kind}: {error}"))
}

fn differences(tree: &dyn Tree, plan: &Plan) -> Vec<String> {
    let mut differences = Vec::new();
    for (path, desired) in &plan.desired {
        match tree.read(path) {
            Ok(current) if current == *desired => {}
            Ok(_) => differences.push(format!("{path}: divergent-adapter")),
            Err(TreeError::NotFound) => differences.push(format!("{path}: missing-adapter")),
            Err(TreeError::NotText) => differences.push(format!("{path}: non-text-adapter")),
            Err(TreeError::Unreadable(reason)) => {
                differences.push(format!("{path}: unreadable-adapter: {reason}"))
            }
        }
    }
    for path in tree.files() {
        if plan
            .transaction
            .roots
            .iter()
            .any(|root| under_root(&path, root))
            && !plan.desired.contains_key(&path)
        {
            differences.push(format!("{path}: stale-adapter"));
        }
    }
    differences.sort();
    differences
}

fn clean(format: Format, message: &str) -> Outcome {
    match format {
        Format::Text => Outcome::clean(format!("[harness-adapters] {message}\n")),
        Format::Json => Outcome::clean(format!(
            "{{\"schemaVersion\":1,\"command\":\"harness-adapters\",\"status\":\"clean\",\"message\":\"{message}\"}}\n"
        )),
    }
}

fn findings(format: Format, differences: Vec<String>) -> Outcome {
    match format {
        Format::Text => Outcome {
            exit_code: 1,
            stdout: String::new(),
            stderr: differences
                .into_iter()
                .map(|difference| format!("[harness-adapters] {difference}\n"))
                .collect(),
        },
        Format::Json => Outcome {
            exit_code: 1,
            stdout: format!(
                "{{\"schemaVersion\":1,\"command\":\"harness-adapters\",\"status\":\"findings\",\"findings\":{}}}\n",
                serde_json::to_string(&differences).expect("a string list always serializes")
            ),
            stderr: String::new(),
        },
    }
}

fn refused(format: Format, reason: String) -> Outcome {
    match format {
        Format::Text => Outcome::refused(format!("[harness-adapters] {reason}\n")),
        Format::Json => Outcome::refused(format!(
            "{{\"schemaVersion\":1,\"command\":\"harness-adapters\",\"status\":\"refused\",\"reason\":{}}}\n",
            serde_json::to_string(&reason).expect("a string always serializes")
        )),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::runtime::{MemoryAdapterStore, MemoryTree, NoAdapterStore, Tree};

    fn requirements(capability: &str) -> Requirements {
        Requirements {
            capabilities: vec![capability.to_string()],
            grants: vec!["files-read".to_string()],
            denials: vec!["network".to_string()],
            constraints: vec!["offline".to_string()],
            routes: vec!["canonical-import".to_string()],
            identities: vec!["reviewer".to_string()],
        }
    }

    fn profile(id: &str, capability: &str) -> Profile {
        Profile {
            id: id.to_string(),
            root: format!("adapters/{id}"),
            supports: requirements(capability),
        }
    }

    fn complete_harness(capability: &str) -> Harness {
        Harness {
            requirements: requirements(capability),
            profiles: vec![
                profile("alpha", capability),
                profile("beta", capability),
                profile("gamma", capability),
            ],
        }
    }

    fn canonical_tree() -> MemoryTree {
        let tree = MemoryTree::default();
        tree.write(ROOT_INSTRUCTION, "Canonical instruction\n");
        tree.write(".agents/agents/reviewer.md", "Canonical agent\n");
        tree.write(".agents/skills/review/SKILL.md", "Canonical skill\n");
        tree
    }

    #[test]
    fn second_generation_is_a_byte_identical_no_op() {
        let tree = canonical_tree();
        let harness = complete_harness("read");
        let store = MemoryAdapterStore::new(&tree);

        let first = generate(Some(&harness), &tree, &store, Format::Text);
        assert_eq!(first.exit_code, 0);
        let after_first = tree.files();

        let second = generate(Some(&harness), &tree, &store, Format::Text);
        assert_eq!(second, first);
        assert_eq!(tree.files(), after_first);
        assert_eq!(validate(Some(&harness), &tree, Format::Text).exit_code, 0);
    }

    #[test]
    fn loss_refuses_before_the_adapter_store_receives_a_transaction() {
        let tree = canonical_tree();
        let mut harness = complete_harness("write");
        harness.profiles[1].supports.capabilities.clear();
        let store = MemoryAdapterStore::new(&tree);

        let outcome = generate(Some(&harness), &tree, &store, Format::Text);

        assert_eq!(outcome.exit_code, 2);
        assert!(
            outcome
                .stderr
                .contains("profile `beta` cannot represent required capability `write`")
        );
        assert!(
            tree.files()
                .into_iter()
                .all(|path| !path.starts_with("adapters/"))
        );
    }

    #[test]
    fn every_required_semantic_axis_refuses_before_generation() {
        for (axis, requirement) in [
            ("grant", "files-read"),
            ("denial", "network"),
            ("constraint", "offline"),
            ("route", "canonical-import"),
            ("identity", "reviewer"),
        ] {
            let tree = canonical_tree();
            let mut harness = complete_harness("read");
            match axis {
                "grant" => harness.profiles[1].supports.grants.clear(),
                "denial" => harness.profiles[1].supports.denials.clear(),
                "constraint" => harness.profiles[1].supports.constraints.clear(),
                "route" => harness.profiles[1].supports.routes.clear(),
                "identity" => harness.profiles[1].supports.identities.clear(),
                _ => unreachable!("the test declares only known requirement axes"),
            }
            let store = MemoryAdapterStore::new(&tree);

            let outcome = generate(Some(&harness), &tree, &store, Format::Text);

            assert_eq!(outcome.exit_code, 2, "{axis}");
            assert!(outcome.stderr.contains(&format!(
                "profile `beta` cannot represent required {axis} `{requirement}`"
            )));
            assert!(
                tree.files()
                    .into_iter()
                    .all(|path| !path.starts_with("adapters/")),
                "{axis} wrote an adapter before refusing"
            );
        }
    }

    #[test]
    fn validation_reports_missing_adapter_bytes() {
        let tree = canonical_tree();
        let harness = complete_harness("read");

        let outcome = validate(Some(&harness), &tree, Format::Text);

        assert_eq!(outcome.exit_code, 1);
        assert!(
            outcome
                .stderr
                .contains("adapters/alpha/AGENTS.md: missing-adapter")
        );
    }

    #[test]
    fn validation_reports_manual_and_divergent_adapter_bytes() {
        let tree = canonical_tree();
        let harness = complete_harness("read");
        let store = MemoryAdapterStore::new(&tree);
        assert_eq!(
            generate(Some(&harness), &tree, &store, Format::Text).exit_code,
            0
        );
        tree.write("adapters/alpha/manual.md", "not generated\n");
        tree.write("adapters/beta/AGENTS.md", "handwritten\n");

        let outcome = validate(Some(&harness), &tree, Format::Text);

        assert_eq!(outcome.exit_code, 1);
        assert!(outcome.stderr.contains("manual.md: stale-adapter"));
        assert!(outcome.stderr.contains("AGENTS.md: divergent-adapter"));
    }

    #[test]
    fn generation_refuses_without_an_explicit_adapter_store() {
        let tree = canonical_tree();
        let harness = complete_harness("read");

        let outcome = generate(Some(&harness), &tree, &NoAdapterStore, Format::Text);

        assert_eq!(outcome.exit_code, 2);
        assert!(
            outcome
                .stderr
                .contains("no canonical-adapter write boundary")
        );
        assert!(
            tree.files()
                .into_iter()
                .all(|path| !path.starts_with("adapters/"))
        );
    }

    #[test]
    fn harness_planning_refuses_absent_incomplete_or_unrepresentable_profiles() {
        let tree = canonical_tree();
        assert_eq!(validate(None, &tree, Format::Text).exit_code, 2);

        let incomplete = Harness::default();
        assert_eq!(
            validate(Some(&incomplete), &tree, Format::Text).exit_code,
            2
        );

        let mut duplicate = complete_harness("read");
        duplicate.profiles[1].id = "alpha".to_string();
        assert_eq!(validate(Some(&duplicate), &tree, Format::Text).exit_code, 2);

        let mut invalid_root = complete_harness("read");
        invalid_root.profiles[1].root = ".agents/forbidden".to_string();
        assert_eq!(
            validate(Some(&invalid_root), &tree, Format::Text).exit_code,
            2
        );

        let mut empty_id = complete_harness("read");
        empty_id.profiles[1].id.clear();
        assert_eq!(validate(Some(&empty_id), &tree, Format::Text).exit_code, 2);

        let mut empty_requirement = complete_harness("read");
        empty_requirement.requirements.capabilities = vec![String::new()];
        assert_eq!(
            validate(Some(&empty_requirement), &tree, Format::Text).exit_code,
            2
        );
    }

    #[test]
    fn harness_discovery_and_rendering_refuse_unsafe_sources_and_cover_output_shapes() {
        let missing = MemoryTree::default();
        assert!(plan(Some(&complete_harness("read")), &missing).is_err());

        let mut unreadable = canonical_tree();
        unreadable.mark_unreadable(ROOT_INSTRUCTION);
        assert!(plan(Some(&complete_harness("read")), &unreadable).is_err());

        let mut binary = canonical_tree();
        binary.mark_binary(ROOT_INSTRUCTION);
        assert!(plan(Some(&complete_harness("read")), &binary).is_err());

        let duplicate = canonical_tree();
        duplicate.write(".agents/agents/reviewer.txt", "Second agent\n");
        assert!(plan(Some(&complete_harness("read")), &duplicate).is_err());

        let root = Source {
            id: "instruction".to_string(),
            path: ROOT_INSTRUCTION.to_string(),
            digest: digest("canonical"),
        };
        let agent = Source {
            id: "agent/reviewer".to_string(),
            path: ".agents/agents/reviewer.md".to_string(),
            digest: digest("agent"),
        };
        let skill = Source {
            id: "skill/review".to_string(),
            path: ".agents/skills/review/SKILL.md".to_string(),
            digest: digest("skill"),
        };
        let outside = Source {
            id: "outside".to_string(),
            path: "outside.md".to_string(),
            digest: digest("outside"),
        };
        assert_eq!(
            output_path("adapters/test", &root).unwrap(),
            "adapters/test/AGENTS.md"
        );
        assert_eq!(
            output_path("adapters/test", &agent).unwrap(),
            "adapters/test/agents/reviewer.md"
        );
        assert_eq!(
            output_path("adapters/test", &skill).unwrap(),
            "adapters/test/skills/review/SKILL.md"
        );
        assert!(output_path("adapters/test", &outside).is_err());
        assert_eq!(
            route_from("adapters/test/agents/reviewer.md", &agent.path),
            "../../../.agents/agents/reviewer.md"
        );
        assert!(adapter_body(&root, "../AGENTS.md").contains("sha256:"));
        assert!(json(&root, "source").unwrap().contains("instruction"));
    }

    #[test]
    fn harness_json_and_read_failures_have_the_declared_terminal_outcomes() {
        let tree = canonical_tree();
        let harness = complete_harness("read");
        let missing = validate(Some(&harness), &tree, Format::Json);
        assert_eq!(missing.exit_code, 1);
        assert!(missing.stdout.contains("missing-adapter"));

        {
            let store = MemoryAdapterStore::new(&tree);
            assert_eq!(
                generate(Some(&harness), &tree, &store, Format::Json).exit_code,
                0
            );
            assert_eq!(validate(Some(&harness), &tree, Format::Json).exit_code, 0);
        }

        let mut unreadable = tree;
        unreadable.mark_unreadable("adapters/alpha/AGENTS.md");
        let outcome = validate(Some(&harness), &unreadable, Format::Text);
        assert_eq!(outcome.exit_code, 1);
        assert!(outcome.stderr.contains("unreadable-adapter"));

        let mut non_text = canonical_tree();
        {
            let store = MemoryAdapterStore::new(&non_text);
            assert_eq!(
                generate(Some(&harness), &non_text, &store, Format::Text).exit_code,
                0
            );
        }
        non_text.mark_binary("adapters/alpha/AGENTS.md");
        let outcome = validate(Some(&harness), &non_text, Format::Json);
        assert_eq!(outcome.exit_code, 1);
        assert!(outcome.stdout.contains("non-text-adapter"));

        let no_profiles = generate(None, &canonical_tree(), &NoAdapterStore, Format::Json);
        assert_eq!(no_profiles.exit_code, 2);
        assert!(no_profiles.stderr.contains("refused"));
    }
}
