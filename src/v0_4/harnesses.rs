//! Canonical discovery plus deterministic adapter projection.
//!
//! Canonical bodies are never copied into an adapter. Every generated body is
//! a small, provenance-bearing route to one file in the root instruction or
//! `.agents` tree. Profiles are configuration data, so this module has no
//! consumer-specific branch.

use super::config::{
    Adapter, AdapterFormat, Canonical, CanonicalAgent, CanonicalDocument, Harness, Identity,
    Profile, Requirements, Translation, When,
};
use crate::Outcome;
use crate::cli::Format;
use crate::errors::ErrorCode;
use crate::runtime::{
    AdapterFile, AdapterStore, AdapterTransaction, Tree, TreeError, adapter_exact_paths,
    adapter_roots, normal_adapter_path, under_root,
};
use crate::scan;
use serde::Serialize;
use sha2::{Digest, Sha256};
use std::collections::{BTreeMap, BTreeSet};

const ROOT_INSTRUCTION: &str = "AGENTS.md";
const AGENTS_ROOT: &str = ".agents/agents/";
const SKILLS_ROOT: &str = ".agents/skills/";

/// Compare every currently visible adapter against the complete desired model.
/// It deliberately has no write port, so validation cannot repair drift.
pub(crate) fn validate(harness: Option<&Harness>, tree: &dyn Tree, format: Format) -> Outcome {
    let Some(harness) = harness else {
        return undeclared(format);
    };
    let plan = match plan(harness, tree) {
        Ok(plan) => plan,
        Err(reason) => return refused(format, reason),
    };
    let mut differences = differences(tree, &plan);
    match stale_markers(harness, tree, &plan, format) {
        Ok(markers) => differences.extend(markers),
        Err(refusal) => return refusal,
    }
    differences.sort();
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
    let Some(harness) = harness else {
        return undeclared(format);
    };
    let plan = match plan(harness, tree) {
        Ok(plan) => plan,
        Err(reason) => return refused(format, reason),
    };
    // Read before anything is written, so a marker surface that cannot be
    // read refuses the run before generation acts.
    let markers = match stale_markers(harness, tree, &plan, format) {
        Ok(markers) => markers,
        Err(refusal) => return refusal,
    };
    if !differences(tree, &plan).is_empty()
        && let Err(error) = store.replace(&tree.root(), &plan.transaction)
    {
        return refused(format, error.0);
    }
    // Generation owns only its families and exact files, so it cannot clear
    // a marker elsewhere; it writes what it owns and reports the rest.
    if markers.is_empty() {
        clean(format, "canonical adapter generation is current")
    } else {
        findings(format, markers)
    }
}

/// The phrase that claims a file, or a region of one, as RHINO output.
///
/// Matched case-insensitively on any line. RHINO writes it nowhere, so a
/// file outside every adapter family that carries it tells a maintainer the
/// region is machine-owned when nothing owns it.
const GENERATED_MARKER: &str = "rhino generated";

/// Every declared marker-surface file that claims RHINO generation and that
/// generation does not write, as `stale-marker` findings.
///
/// A file generation owns -- one it writes, or any file under a family root
/// or at an exact adapter path -- is never a stale marker: the adapter
/// comparison already reports it.
fn stale_markers(
    harness: &Harness,
    tree: &dyn Tree,
    plan: &Plan,
    format: Format,
) -> Result<Vec<String>, Outcome> {
    if harness.marker_surfaces.is_empty() {
        return Ok(Vec::new());
    }
    let surfaces = scan::Surfaces::compile(
        "harness.marker-surfaces",
        harness.marker_surfaces.iter().map(String::as_str),
    )
    .map_err(|reason| {
        crate::report::Report::refused_as(ErrorCode::ConfigUnusable, "harness-adapters", reason)
            .render(format)
    })?;
    let reached = surfaces
        .files(tree, &crate::config::Config::default())
        .map_err(|unreachable| unreachable.refusal("harness-adapters").render(format))?;
    let mut stale = Vec::new();
    for scan::Reached { path, source } in reached {
        if surfaces.governing(&path).is_none()
            || plan.desired.contains_key(&path)
            || plan.transaction.exact_paths.contains(&path)
            || plan
                .transaction
                .roots
                .iter()
                .any(|root| under_root(&path, root))
        {
            continue;
        }
        let text = match tree.read(&source) {
            Ok(text) => text,
            // Gone since the walk, or not text: neither can carry a marker.
            Err(TreeError::NotFound | TreeError::NotText) => continue,
            Err(TreeError::Unreadable(reason)) => {
                return Err(crate::report::Report::refused_as(
                    ErrorCode::FileUnreadable,
                    "harness-adapters",
                    format!("{path}: {reason}"),
                )
                .render(format));
            }
        };
        if text
            .lines()
            .any(|line| line.to_ascii_lowercase().contains(GENERATED_MARKER))
        {
            stale.push(format!("{path}: stale-marker"));
        }
    }
    Ok(stale)
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
    #[serde(skip)]
    kind: SourceKind,
    #[serde(skip)]
    metadata: Option<Metadata>,
}

#[derive(Clone, Copy, PartialEq, Eq)]
enum SourceKind {
    Instruction,
    Agent,
    Skill,
}

#[derive(Clone)]
struct Metadata {
    name: String,
    description: String,
    tier: Option<String>,
    grants: BTreeSet<String>,
    denials: BTreeSet<String>,
    constraints: BTreeSet<String>,
    /// Every canonical list in authored order, keyed by its front-matter key.
    lists: BTreeMap<String, Vec<String>>,
}

enum RenderField {
    Scalar(String),
    Members(Vec<String>),
    Entries(BTreeMap<String, String>),
    Sequence(Vec<String>),
}

fn plan(harness: &Harness, tree: &dyn Tree) -> Result<Plan, String> {
    if harness.profiles.len() != 3 {
        return Err("harness: exactly three adapter profiles are required".to_string());
    }
    let (roots, exact_paths) = validate_profiles(
        &harness.profiles,
        &harness.requirements,
        harness.canonical.as_ref(),
    )?;
    let sources = discover(tree, harness.canonical.as_ref(), &harness.profiles)?;
    for profile in &harness.profiles {
        require_selected_sources(profile, &sources)?;
    }

    let mut desired = BTreeMap::new();
    for profile in &harness.profiles {
        render_profile(profile, &sources, &mut desired)?;
    }
    let files = desired
        .iter()
        .map(|(path, contents)| AdapterFile {
            path: path.clone(),
            contents: contents.clone(),
        })
        .collect();
    Ok(Plan {
        transaction: AdapterTransaction {
            roots,
            exact_paths,
            files,
        },
        desired,
    })
}

fn validate_profiles(
    profiles: &[Profile],
    requirements: &Requirements,
    canonical: Option<&Canonical>,
) -> Result<(Vec<String>, Vec<String>), String> {
    let mut identifiers = BTreeSet::new();
    let mut roots = Vec::new();
    let mut exact_paths = Vec::new();
    let needs_agents = profiles
        .iter()
        .any(|profile| profile.agent_adapter.is_some());
    let needs_skills = profiles
        .iter()
        .any(|profile| profile.skill_adapter.is_some());
    if needs_agents && canonical.and_then(|shape| shape.agents.as_ref()).is_none() {
        return Err(
            "harness: agent adapters require a canonical agent field declaration".to_string(),
        );
    }
    if needs_skills && canonical.and_then(|shape| shape.skills.as_ref()).is_none() {
        return Err(
            "harness: skill adapters require a canonical skill field declaration".to_string(),
        );
    }

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
        let mut declares_output = false;
        if let Some(adapter) = &profile.agent_adapter {
            roots.push(validate_adapter(&profile.id, adapter, true)?);
            declares_output = true;
        }
        if let Some(adapter) = &profile.skill_adapter {
            roots.push(validate_adapter(&profile.id, adapter, false)?);
            declares_output = true;
        }
        if let Some(adapter) = &profile.instruction_adapter {
            let path = normal_adapter_path(&adapter.path).ok_or_else(|| {
                format!(
                    "profile `{}` has an invalid instruction adapter path",
                    profile.id
                )
            })?;
            if path != adapter.path || path == ROOT_INSTRUCTION || path.starts_with(".agents/") {
                return Err(format!(
                    "profile `{}` has an invalid instruction adapter path",
                    profile.id
                ));
            }
            validate_route(&profile.id, &adapter.route)?;
            exact_paths.push(path);
            declares_output = true;
        }
        if !declares_output {
            return Err(format!(
                "profile `{}` declares no native adapter representation",
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
    let roots = adapter_roots(&roots).map_err(|error| format!("harness: {}", error.0))?;
    let exact_paths = adapter_exact_paths(&roots, &exact_paths)
        .map_err(|error| format!("harness: {}", error.0))?;
    Ok((roots, exact_paths))
}

fn validate_adapter(profile: &str, adapter: &Adapter, agent: bool) -> Result<String, String> {
    let root = adapter_root(&adapter.path)
        .ok_or_else(|| format!("profile `{profile}` has an invalid adapter path"))?;
    if root.starts_with(".agents/") || root == ROOT_INSTRUCTION {
        return Err(format!("profile `{profile}` has an invalid adapter path"));
    }
    validate_route(profile, &adapter.route)?;
    if (adapter.format == AdapterFormat::FrontMatter) != (adapter.route_field == "body") {
        return Err(format!(
            "profile `{profile}` has a route field incompatible with its adapter format"
        ));
    }
    if adapter.route_field.trim().is_empty() {
        return Err(format!(
            "profile `{profile}` has an empty adapter route field"
        ));
    }
    if !agent && adapter.tier_fields.is_some() {
        return Err(format!(
            "profile `{profile}` projects a tier into a skill adapter"
        ));
    }
    if !agent && !adapter.lists.is_empty() {
        return Err(format!(
            "profile `{profile}` projects an agent list into a skill adapter"
        ));
    }
    if !agent && adapter.agents.is_some() {
        return Err(format!(
            "profile `{profile}` selects agents in a skill adapter"
        ));
    }
    let mut direct_fields = BTreeSet::new();
    for field in adapter
        .identity
        .keys()
        .chain(adapter.fixed.keys())
        .chain(adapter.lists.keys())
    {
        if field.trim().is_empty() || !direct_fields.insert(field.as_str()) {
            return Err(format!(
                "profile `{profile}` repeats or omits an adapter field"
            ));
        }
    }
    let mut translation_fields = BTreeMap::new();
    for translation in &adapter.translations {
        if translation.field.trim().is_empty()
            || (usize::from(!translation.members.is_empty())
                + usize::from(!translation.absent_members.is_empty())
                + usize::from(!translation.entries.is_empty())
                != 1)
            || matches!(translation.when, When::Always) != translation.capability.is_none()
            || direct_fields.contains(translation.field.as_str())
        {
            return Err(format!(
                "profile `{profile}` has an invalid capability translation"
            ));
        }
        let is_members = !translation.members.is_empty() || !translation.absent_members.is_empty();
        if translation_fields
            .insert(translation.field.as_str(), is_members)
            .is_some_and(|previous| previous != is_members)
        {
            return Err(format!(
                "profile `{profile}` mixes incompatible capability translations"
            ));
        }
    }
    if let Some(fields) = &adapter.tier_fields
        && (fields.model.trim().is_empty()
            || fields.effort.trim().is_empty()
            || fields.model == fields.effort)
    {
        return Err(format!("profile `{profile}` has invalid tier fields"));
    }
    Ok(root)
}

fn validate_route(profile: &str, route: &str) -> Result<(), String> {
    if route.matches("{path}").count() != 1 {
        return Err(format!(
            "profile `{profile}` must declare exactly one `{{path}}` route placeholder"
        ));
    }
    Ok(())
}

fn adapter_root(pattern: &str) -> Option<String> {
    if pattern.matches("{name}").count() != 1 {
        return None;
    }
    let sample = pattern.replace("{name}", "adapter");
    let normalized = normal_adapter_path(&sample)?;
    if normalized != sample {
        return None;
    }
    let (head, _) = pattern.split_once("{name}")?;
    let root = head.trim_end_matches('/');
    normal_adapter_path(root)
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

fn discover(
    tree: &dyn Tree,
    canonical: Option<&Canonical>,
    profiles: &[Profile],
) -> Result<Vec<Source>, String> {
    let instruction = read_source(
        tree,
        ROOT_INSTRUCTION,
        "instruction",
        SourceKind::Instruction,
        None,
    )?;
    let mut sources = vec![instruction];
    let mut ids = BTreeSet::from([sources[0].id.clone()]);
    if profiles
        .iter()
        .any(|profile| profile.agent_adapter.is_some())
    {
        let shape = canonical
            .and_then(|shape| shape.agents.as_ref())
            .expect("profile validation proved canonical agent metadata exists");
        for path in tree.files().into_iter().filter(|path| {
            path.strip_prefix(AGENTS_ROOT).is_some_and(|relative| {
                relative.ends_with(".md") && !relative.contains('/') && relative != "README.md"
            })
        }) {
            let relative = path
                .strip_prefix(AGENTS_ROOT)
                .expect("the filter proved this prefix");
            let name = source_id(relative).to_string();
            let source = read_source(
                tree,
                &path,
                &format!("agent/{name}"),
                SourceKind::Agent,
                Some(shape),
            )?;
            if source
                .metadata
                .as_ref()
                .is_some_and(|metadata| metadata.name != name)
            {
                return Err(format!(
                    "canonical agent `{path}` has a name that does not match its path"
                ));
            }
            if !ids.insert(source.id.clone()) {
                return Err(format!("duplicate canonical agent id `{}`", source.id));
            }
            sources.push(source);
        }
    }
    if profiles
        .iter()
        .any(|profile| profile.skill_adapter.is_some())
    {
        let shape = canonical
            .and_then(|shape| shape.skills.as_ref())
            .expect("profile validation proved canonical skill metadata exists");
        for path in tree.files().into_iter().filter(|path| {
            path.strip_prefix(SKILLS_ROOT).is_some_and(|relative| {
                relative.ends_with("/SKILL.md") && relative.matches('/').count() == 1
            })
        }) {
            let relative = path
                .strip_prefix(SKILLS_ROOT)
                .expect("the filter proved this prefix");
            let name = relative.trim_end_matches("/SKILL.md").to_string();
            let source = read_source(
                tree,
                &path,
                &format!("skill/{name}"),
                SourceKind::Skill,
                Some(shape),
            )?;
            if source
                .metadata
                .as_ref()
                .is_some_and(|metadata| metadata.name != name)
            {
                return Err(format!(
                    "canonical skill `{path}` has a name that does not match its path"
                ));
            }
            if !ids.insert(source.id.clone()) {
                return Err(format!("duplicate canonical skill id `{}`", source.id));
            }
            sources.push(source);
        }
    }
    sources.sort_by(|left, right| left.path.cmp(&right.path));
    Ok(sources)
}

fn source_id(path: &str) -> &str {
    path.rsplit_once('.').map_or(path, |(stem, _)| stem)
}

fn read_source(
    tree: &dyn Tree,
    path: &str,
    id: &str,
    kind: SourceKind,
    canonical: Option<&dyn CanonicalShape>,
) -> Result<Source, String> {
    let contents = tree.read(path).map_err(|error| match error {
        TreeError::NotFound => format!("canonical source `{path}` is missing"),
        TreeError::NotText => format!("canonical source `{path}` holds non-text bytes"),
        TreeError::Unreadable(reason) => {
            format!("canonical source `{path}` cannot be read: {reason}")
        }
    })?;
    let metadata = canonical
        .map(|shape| parse_metadata(path, &contents, shape))
        .transpose()?;
    Ok(Source {
        id: id.to_string(),
        path: path.to_string(),
        digest: digest(&contents),
        kind,
        metadata,
    })
}

fn render_profile(
    profile: &Profile,
    sources: &[Source],
    desired: &mut BTreeMap<String, String>,
) -> Result<(), String> {
    let selected = selected_sources(profile, sources);
    let sources = selected.as_slice();
    if let Some(adapter) = &profile.instruction_adapter {
        let instruction = sources
            .iter()
            .find(|source| source.kind == SourceKind::Instruction)
            .expect("canonical discovery always includes the instruction");
        add_file(
            desired,
            &adapter.path,
            format!("{}\n", route(&adapter.route, &instruction.path)?),
        )?;
    }
    if let Some(adapter) = &profile.agent_adapter {
        render_family(profile, adapter, SourceKind::Agent, sources, desired)?;
    }
    if let Some(adapter) = &profile.skill_adapter {
        render_family(profile, adapter, SourceKind::Skill, sources, desired)?;
    }
    Ok(())
}

/// A selection naming an agent with no canonical source fails closed, rather
/// than rendering fewer agents than the repository asked for.
fn require_selected_sources(profile: &Profile, sources: &[Source]) -> Result<(), String> {
    for name in agent_selection(profile).unwrap_or_default() {
        if !sources
            .iter()
            .any(|source| agent_name(source) == Some(name.as_str()))
        {
            return Err(format!(
                "profile `{}` selects agent `{name}` with no canonical source",
                profile.id
            ));
        }
    }
    Ok(())
}

/// The sources one profile renders: every source, less any canonical agent its
/// agent adapter does not select. Both of the profile's catalogs read this set.
fn selected_sources(profile: &Profile, sources: &[Source]) -> Vec<Source> {
    let selection = agent_selection(profile);
    sources
        .iter()
        .filter(|source| match (agent_name(source), selection) {
            (Some(name), Some(names)) => names.iter().any(|selected| selected == name),
            _ => true,
        })
        .cloned()
        .collect()
}

/// The agent names a profile's agent adapter selects, when it declares any.
fn agent_selection(profile: &Profile) -> Option<&[String]> {
    profile
        .agent_adapter
        .as_ref()
        .and_then(|adapter| adapter.agents.as_deref())
}

/// A canonical agent's name; every other source kind has none.
fn agent_name(source: &Source) -> Option<&str> {
    match source.kind {
        SourceKind::Agent => source
            .metadata
            .as_ref()
            .map(|metadata| metadata.name.as_str()),
        _ => None,
    }
}

fn render_family(
    profile: &Profile,
    adapter: &Adapter,
    kind: SourceKind,
    sources: &[Source],
    desired: &mut BTreeMap<String, String>,
) -> Result<(), String> {
    let root = adapter_root(&adapter.path)
        .ok_or_else(|| format!("profile `{}` has an invalid adapter path", profile.id))?;
    for source in sources.iter().filter(|source| source.kind == kind) {
        let metadata = source
            .metadata
            .as_ref()
            .expect("a renderable source has parsed canonical metadata");
        let path = adapter.path.replace("{name}", &metadata.name);
        add_file(desired, &path, render_adapter(profile, adapter, source)?)?;
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
    add_file(
        desired,
        &format!("{root}/catalog.json"),
        json(&catalog, "catalog")?,
    )?;
    add_file(
        desired,
        &format!("{root}/provenance.json"),
        json(&provenance, "provenance")?,
    )
}

fn render_adapter(profile: &Profile, adapter: &Adapter, source: &Source) -> Result<String, String> {
    let metadata = source
        .metadata
        .as_ref()
        .expect("a renderable source has parsed canonical metadata");
    let mut fields = BTreeMap::new();
    for (field, identity) in &adapter.identity {
        let value = match identity {
            Identity::Name => &metadata.name,
            Identity::Description => &metadata.description,
        };
        add_scalar(&mut fields, field, value.clone())?;
    }
    for (field, value) in &adapter.fixed {
        add_scalar(&mut fields, field, value.clone())?;
    }
    for (field, key) in &adapter.lists {
        if let Some(values) = metadata
            .lists
            .get(key.key())
            .filter(|values| !values.is_empty())
        {
            add_field(&mut fields, field, RenderField::Sequence(values.clone()))?;
        }
    }
    if let (Some(tier), Some(tier_fields)) = (&metadata.tier, &adapter.tier_fields)
        && let Some(mapping) = profile.tiers.get(tier)
    {
        add_scalar(&mut fields, &tier_fields.model, mapping.model.clone())?;
        add_scalar(&mut fields, &tier_fields.effort, mapping.effort.clone())?;
    }
    let mut absent_members = BTreeMap::<String, BTreeSet<String>>::new();
    for translation in &adapter.translations {
        let applies = match translation.when {
            When::Always => true,
            When::Requires => translation
                .capability
                .as_ref()
                .is_some_and(|capability| metadata.grants.contains(capability)),
            When::Denies => translation
                .capability
                .as_ref()
                .is_some_and(|capability| metadata.denials.contains(capability)),
            When::Constrains => translation
                .capability
                .as_ref()
                .is_some_and(|capability| metadata.constraints.contains(capability)),
        };
        if applies {
            if translation.absent_members.is_empty() {
                add_translation(&mut fields, translation)?;
            } else {
                absent_members
                    .entry(translation.field.clone())
                    .or_default()
                    .extend(translation.absent_members.iter().cloned());
            }
        }
    }
    for (field, members_to_remove) in absent_members {
        let Some(RenderField::Members(members)) = fields.get_mut(&field) else {
            return Err(format!(
                "adapter output cannot remove members from undeclared field `{field}`"
            ));
        };
        members.retain(|member| !members_to_remove.contains(member));
    }
    if adapter
        .absent
        .iter()
        .any(|field| fields.contains_key(field))
    {
        return Err(format!(
            "canonical source `{}` requires an absent adapter field",
            source.path
        ));
    }
    let route = route(&adapter.route, &source.path)?;
    match adapter.format {
        AdapterFormat::FrontMatter => render_front_matter(&fields, &route),
        AdapterFormat::Toml => {
            add_scalar(&mut fields, &adapter.route_field, route)?;
            render_toml(&fields)
        }
    }
}

fn add_file(
    desired: &mut BTreeMap<String, String>,
    path: &str,
    contents: String,
) -> Result<(), String> {
    let path = normal_adapter_path(path)
        .ok_or_else(|| format!("generated adapter path `{path}` is invalid"))?;
    if desired.insert(path.clone(), contents).is_some() {
        return Err(format!("generated adapter path `{path}` is duplicated"));
    }
    Ok(())
}

fn route(template: &str, path: &str) -> Result<String, String> {
    if template.matches("{path}").count() != 1 {
        return Err("adapter route does not declare exactly one `{path}` placeholder".to_string());
    }
    Ok(template.replace("{path}", path))
}

trait CanonicalShape {
    fn name_field(&self) -> &str;
    fn description_field(&self) -> &str;
    fn tier_field(&self) -> Option<&str>;
    fn grants_field(&self) -> Option<&str>;
    fn denials_field(&self) -> Option<&str>;
    fn constraints_field(&self) -> Option<&str>;
}

impl CanonicalShape for CanonicalDocument {
    fn name_field(&self) -> &str {
        &self.name
    }

    fn description_field(&self) -> &str {
        &self.description
    }

    fn tier_field(&self) -> Option<&str> {
        None
    }

    fn grants_field(&self) -> Option<&str> {
        None
    }

    fn denials_field(&self) -> Option<&str> {
        None
    }

    fn constraints_field(&self) -> Option<&str> {
        None
    }
}

impl CanonicalShape for CanonicalAgent {
    fn name_field(&self) -> &str {
        &self.name
    }

    fn description_field(&self) -> &str {
        &self.description
    }

    fn tier_field(&self) -> Option<&str> {
        self.tier.as_deref()
    }

    fn grants_field(&self) -> Option<&str> {
        Some(&self.grants)
    }

    fn denials_field(&self) -> Option<&str> {
        Some(&self.denials)
    }

    fn constraints_field(&self) -> Option<&str> {
        Some(&self.constraints)
    }
}

enum CanonicalValue {
    Scalar(String),
    /// Authored order is kept, because a projected list is rendered as written.
    List(Vec<String>),
}

fn parse_metadata(
    path: &str,
    contents: &str,
    shape: &dyn CanonicalShape,
) -> Result<Metadata, String> {
    let fields = parse_front_matter(path, contents)?;
    let name = required_scalar(&fields, shape.name_field(), path)?;
    let description = required_scalar(&fields, shape.description_field(), path)?;
    let tier = shape
        .tier_field()
        .and_then(|field| optional_scalar(&fields, field, path).transpose())
        .transpose()?;
    Ok(Metadata {
        name,
        description,
        tier,
        grants: optional_list(&fields, shape.grants_field(), path)?,
        denials: optional_list(&fields, shape.denials_field(), path)?,
        constraints: optional_list(&fields, shape.constraints_field(), path)?,
        lists: fields
            .iter()
            .filter_map(|(key, value)| match value {
                CanonicalValue::List(values) => Some((key.clone(), values.clone())),
                CanonicalValue::Scalar(_) => None,
            })
            .collect(),
    })
}

fn parse_front_matter(
    path: &str,
    contents: &str,
) -> Result<BTreeMap<String, CanonicalValue>, String> {
    let lines: Vec<&str> = contents.lines().collect();
    if lines.first().copied() != Some("---") {
        return Err(format!(
            "canonical source `{path}` has no leading front matter"
        ));
    }
    let Some(close) = lines[1..].iter().position(|line| line.trim_end() == "---") else {
        return Err(format!(
            "canonical source `{path}` has unterminated front matter"
        ));
    };
    let header = &lines[1..close + 1];
    let mut fields = BTreeMap::new();
    let mut index = 0usize;
    while index < header.len() {
        let line = header[index];
        if line.trim().is_empty() {
            index += 1;
            continue;
        }
        if line.starts_with(char::is_whitespace) {
            return Err(format!(
                "canonical source `{path}` has malformed front matter"
            ));
        }
        let Some((key, raw)) = line.split_once(':') else {
            return Err(format!(
                "canonical source `{path}` has malformed front matter"
            ));
        };
        let key = key.trim();
        if key.is_empty() || fields.contains_key(key) {
            return Err(format!(
                "canonical source `{path}` repeats or omits a front matter key"
            ));
        }
        let raw = raw.trim();
        index += 1;
        let value = if matches!(raw, ">" | ">-" | "|" | "|-") {
            let mut parts = Vec::new();
            while index < header.len() && header[index].starts_with(char::is_whitespace) {
                parts.push(header[index].trim());
                index += 1;
            }
            CanonicalValue::Scalar(parts.join(" ").trim().to_string())
        } else if raw.is_empty() {
            let mut members = Vec::new();
            while index < header.len() && header[index].starts_with(char::is_whitespace) {
                let member = header[index].trim();
                let Some(member) = member.strip_prefix("- ") else {
                    return Err(format!(
                        "canonical source `{path}` has a non-list metadata value"
                    ));
                };
                let unquoted = unquote(member);
                if member.trim().is_empty() || members.contains(&unquoted) {
                    return Err(format!(
                        "canonical source `{path}` has an invalid metadata list"
                    ));
                }
                members.push(unquoted);
                index += 1;
            }
            CanonicalValue::List(members)
        } else {
            CanonicalValue::Scalar(unquote(raw))
        };
        fields.insert(key.to_string(), value);
    }
    Ok(fields)
}

fn unquote(value: &str) -> String {
    value
        .strip_prefix('"')
        .and_then(|value| value.strip_suffix('"'))
        .or_else(|| {
            value
                .strip_prefix('\'')
                .and_then(|value| value.strip_suffix('\''))
        })
        .unwrap_or(value)
        .to_string()
}

fn required_scalar(
    fields: &BTreeMap<String, CanonicalValue>,
    field: &str,
    path: &str,
) -> Result<String, String> {
    optional_scalar(fields, field, path)?
        .ok_or_else(|| format!("canonical source `{path}` has no `{field}` metadata field"))
}

fn optional_scalar(
    fields: &BTreeMap<String, CanonicalValue>,
    field: &str,
    path: &str,
) -> Result<Option<String>, String> {
    match fields.get(field) {
        None => Ok(None),
        Some(CanonicalValue::Scalar(value)) if !value.trim().is_empty() => Ok(Some(value.clone())),
        Some(_) => Err(format!(
            "canonical source `{path}` has a non-scalar `{field}` metadata field"
        )),
    }
}

fn optional_list(
    fields: &BTreeMap<String, CanonicalValue>,
    field: Option<&str>,
    path: &str,
) -> Result<BTreeSet<String>, String> {
    let Some(field) = field else {
        return Ok(BTreeSet::new());
    };
    match fields.get(field) {
        None => Ok(BTreeSet::new()),
        Some(CanonicalValue::List(values)) => Ok(values.iter().cloned().collect()),
        Some(_) => Err(format!(
            "canonical source `{path}` has a non-list `{field}` metadata field"
        )),
    }
}

fn add_scalar(
    fields: &mut BTreeMap<String, RenderField>,
    field: &str,
    value: String,
) -> Result<(), String> {
    add_field(fields, field, RenderField::Scalar(value))
}

/// Place one whole native field, refusing a second writer of the same field.
fn add_field(
    fields: &mut BTreeMap<String, RenderField>,
    field: &str,
    value: RenderField,
) -> Result<(), String> {
    if fields.insert(field.to_string(), value).is_some() {
        return Err(format!("adapter output repeats field `{field}`"));
    }
    Ok(())
}

fn add_translation(
    fields: &mut BTreeMap<String, RenderField>,
    translation: &Translation,
) -> Result<(), String> {
    if !translation.absent_members.is_empty() {
        return Err(format!(
            "adapter output cannot add absent members at `{}`",
            translation.field
        ));
    }
    if !translation.members.is_empty() {
        match fields.entry(translation.field.clone()) {
            std::collections::btree_map::Entry::Vacant(entry) => {
                entry.insert(RenderField::Members(translation.members.clone()));
            }
            std::collections::btree_map::Entry::Occupied(mut entry) => match entry.get_mut() {
                RenderField::Members(members) => members.extend(translation.members.clone()),
                _ => {
                    return Err(format!(
                        "adapter output conflicts at `{}`",
                        translation.field
                    ));
                }
            },
        }
    } else {
        match fields.entry(translation.field.clone()) {
            std::collections::btree_map::Entry::Vacant(entry) => {
                entry.insert(RenderField::Entries(translation.entries.clone()));
            }
            std::collections::btree_map::Entry::Occupied(mut entry) => match entry.get_mut() {
                RenderField::Entries(entries) => entries.extend(translation.entries.clone()),
                _ => {
                    return Err(format!(
                        "adapter output conflicts at `{}`",
                        translation.field
                    ));
                }
            },
        }
    }
    Ok(())
}

fn render_front_matter(
    fields: &BTreeMap<String, RenderField>,
    route: &str,
) -> Result<String, String> {
    let mut output = String::from("---\n");
    for (field, value) in fields {
        match value {
            RenderField::Scalar(value) => render_yaml_field(&mut output, field, value, ""),
            RenderField::Members(members) => {
                render_yaml_field(&mut output, field, &members.join(", "), "")
            }
            RenderField::Entries(entries) => {
                output.push_str(&format!("{field}:\n"));
                for (key, value) in entries {
                    render_yaml_field(&mut output, key, value, "  ");
                }
            }
            RenderField::Sequence(values) => {
                output.push_str(&format!("{field}:\n"));
                for value in values {
                    output.push_str(&format!("  - {}\n", yaml_item(value)));
                }
            }
        }
    }
    output.push_str("---\n\n");
    output.push_str(route);
    output.push('\n');
    Ok(output)
}

fn render_toml(fields: &BTreeMap<String, RenderField>) -> Result<String, String> {
    let mut output = String::new();
    for (field, value) in fields {
        match value {
            RenderField::Scalar(value) => {
                output.push_str(&format!("{field} = {}\n", toml_scalar(value)))
            }
            RenderField::Members(members) => {
                output.push_str(&format!("{field} = {}\n", toml_scalar(&members.join(", "))));
            }
            RenderField::Entries(entries) => {
                output.push_str(&format!("[{field}]\n"));
                for (key, value) in entries {
                    output.push_str(&format!("{key} = {}\n", toml_scalar(value)));
                }
            }
            RenderField::Sequence(values) => {
                let array = values.iter().cloned().map(toml::Value::String).collect();
                output.push_str(&format!("{field} = {}\n", toml::Value::Array(array)));
            }
        }
    }
    Ok(output)
}

enum YamlScalar<'a> {
    Plain(&'a str),
    Literal,
}

fn render_yaml_field(output: &mut String, field: &str, value: &str, indentation: &str) {
    match yaml_scalar(value) {
        YamlScalar::Plain(value) => output.push_str(&format!("{indentation}{field}: {value}\n")),
        YamlScalar::Literal => {
            output.push_str(&format!("{indentation}{field}: |-\n"));
            for line in value.lines() {
                output.push_str(&format!("{indentation}  {line}\n"));
            }
        }
    }
}

fn yaml_scalar(value: &str) -> YamlScalar<'_> {
    let plain = !value.is_empty()
        && value.chars().all(|character| {
            character.is_ascii_alphanumeric() || matches!(character, ' ' | '-' | '_' | '.')
        })
        && !value.starts_with('-');
    if plain {
        YamlScalar::Plain(value)
    } else {
        YamlScalar::Literal
    }
}

/// One block-sequence item: plain when YAML reads it back unchanged, otherwise
/// a double-quoted scalar, whose JSON spelling YAML also accepts.
fn yaml_item(value: &str) -> String {
    match yaml_scalar(value) {
        YamlScalar::Plain(value) => value.to_string(),
        YamlScalar::Literal => serde_json::to_string(value).expect("a string always serializes"),
    }
}

fn toml_scalar(value: &str) -> String {
    toml::Value::String(value.to_string()).to_string()
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
    serde_json::to_string_pretty(value)
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

/// An omitted `harness` group, refused the way every other omitted group is:
/// the configuration declares nothing for adapters, which is not the same as
/// declaring adapters RHINO then refused to validate or write.
fn undeclared(format: Format) -> Outcome {
    Outcome::refusal(
        format,
        ErrorCode::ConfigUndeclared,
        "[harness-adapters] harness: no adapter profiles are declared",
    )
}

/// One refusal shape for both formats.
///
/// The JSON form used to carry a `reason` string under a command-specific
/// envelope, which meant a caller wanting to know why any rhino command refused
/// had to learn one shape per command. It now carries the same `error.code` and
/// `error.message` every other refusal does.
fn refused(format: Format, reason: String) -> Outcome {
    Outcome::refusal(
        format,
        ErrorCode::HarnessRefused,
        format!("[harness-adapters] {reason}"),
    )
}

#[cfg(test)]
mod typed_tests {
    use super::super::config::{
        Canonical, CanonicalAgent, CanonicalDocument, CanonicalList, InstructionAdapter, Tier,
        TierFields,
    };
    use super::*;
    use crate::runtime::{MemoryAdapterStore, MemoryTree, NoAdapterStore, Tree};

    type ProfileMutator = fn(&mut Profile);

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

    fn canonical() -> Canonical {
        Canonical {
            agents: Some(CanonicalAgent {
                name: "name".to_string(),
                description: "description".to_string(),
                tier: Some("tier".to_string()),
                grants: "requires".to_string(),
                denials: "denies".to_string(),
                constraints: "constraints".to_string(),
            }),
            skills: Some(CanonicalDocument {
                name: "name".to_string(),
                description: "description".to_string(),
            }),
        }
    }

    fn adapter(path: &str, format: AdapterFormat, route_field: &str) -> Adapter {
        Adapter {
            path: path.to_string(),
            format,
            route_field: route_field.to_string(),
            route: "Read {path} completely.".to_string(),
            identity: BTreeMap::from([
                ("name".to_string(), Identity::Name),
                ("description".to_string(), Identity::Description),
            ]),
            fixed: BTreeMap::new(),
            tier_fields: None,
            lists: BTreeMap::new(),
            agents: None,
            absent: Vec::new(),
            translations: Vec::new(),
        }
    }

    fn profile(id: &str, capability: &str) -> Profile {
        Profile {
            id: id.to_string(),
            supports: requirements(capability),
            instruction_adapter: None,
            agent_adapter: Some(adapter(
                &format!("adapters/{id}/agents/{{name}}.md"),
                AdapterFormat::FrontMatter,
                "body",
            )),
            skill_adapter: Some(adapter(
                &format!("adapters/{id}/skills/{{name}}/SKILL.md"),
                AdapterFormat::FrontMatter,
                "body",
            )),
            tiers: BTreeMap::new(),
        }
    }

    fn complete_harness(capability: &str) -> Harness {
        Harness {
            canonical: Some(canonical()),
            requirements: requirements(capability),
            profiles: vec![
                profile("alpha", capability),
                profile("beta", capability),
                profile("gamma", capability),
            ],
            marker_surfaces: Vec::new(),
        }
    }

    fn canonical_tree() -> MemoryTree {
        let tree = MemoryTree::default();
        tree.write(ROOT_INSTRUCTION, "Canonical instruction\n");
        tree.write(
            ".agents/agents/reviewer.md",
            "---\nname: reviewer\ndescription: Review changes\ntier: plan\nrequires:\n  - read\ndenies:\n  - network\nconstraints:\n  - offline\n---\nCanonical agent\n",
        );
        tree.write(
            ".agents/skills/review/SKILL.md",
            "---\nname: review\ndescription: Review skill\n---\nCanonical skill\n",
        );
        tree
    }

    #[test]
    fn typed_generation_is_a_byte_identical_no_op() {
        let tree = canonical_tree();
        let mut harness = complete_harness("read");
        let alpha = &mut harness.profiles[0];
        alpha.instruction_adapter = Some(InstructionAdapter {
            path: "CLAUDE.md".to_string(),
            route: "@{path}".to_string(),
        });
        alpha.agent_adapter.as_mut().unwrap().tier_fields = Some(TierFields {
            model: "model".to_string(),
            effort: "effort".to_string(),
        });
        alpha.tiers.insert(
            "plan".to_string(),
            Tier {
                model: "deliberate".to_string(),
                effort: "high".to_string(),
            },
        );
        alpha.agent_adapter.as_mut().unwrap().translations = vec![
            Translation {
                when: When::Always,
                capability: None,
                field: "tools".to_string(),
                members: vec!["Read".to_string()],
                absent_members: Vec::new(),
                entries: BTreeMap::new(),
            },
            Translation {
                when: When::Requires,
                capability: Some("read".to_string()),
                field: "permission".to_string(),
                members: Vec::new(),
                absent_members: Vec::new(),
                entries: BTreeMap::from([("read".to_string(), "allow".to_string())]),
            },
            Translation {
                when: When::Denies,
                capability: Some("network".to_string()),
                field: "permission".to_string(),
                members: Vec::new(),
                absent_members: Vec::new(),
                entries: BTreeMap::from([("network".to_string(), "deny".to_string())]),
            },
            Translation {
                when: When::Constrains,
                capability: Some("offline".to_string()),
                field: "permission".to_string(),
                members: Vec::new(),
                absent_members: Vec::new(),
                entries: BTreeMap::from([("offline".to_string(), "required".to_string())]),
            },
        ];
        harness.profiles[1].agent_adapter = Some(adapter(
            "adapters/beta/agents/{name}.toml",
            AdapterFormat::Toml,
            "developer_instructions",
        ));
        harness.profiles[1].skill_adapter = None;
        harness.profiles[2].skill_adapter = None;
        harness.profiles[2]
            .agent_adapter
            .as_mut()
            .unwrap()
            .identity
            .remove("name");
        harness.profiles[2]
            .agent_adapter
            .as_mut()
            .unwrap()
            .fixed
            .insert("mode".to_string(), "subagent".to_string());
        let store = MemoryAdapterStore::new(&tree);

        let first = generate(Some(&harness), &tree, &store, Format::Text);
        assert_eq!(first.exit_code, 0, "{}", first.stderr);
        assert_eq!(tree.read("CLAUDE.md").unwrap(), "@AGENTS.md\n");
        assert!(
            tree.read("adapters/alpha/agents/reviewer.md")
                .unwrap()
                .contains("tools: Read")
        );
        assert!(
            tree.read("adapters/alpha/agents/reviewer.md")
                .unwrap()
                .contains("network: deny")
        );
        assert!(
            tree.read("adapters/beta/agents/reviewer.toml")
                .unwrap()
                .contains("developer_instructions =")
        );
        assert!(
            tree.read("adapters/gamma/agents/reviewer.md")
                .unwrap()
                .contains("mode: subagent")
        );
        let after_first = tree.files();

        let second = generate(Some(&harness), &tree, &store, Format::Text);
        assert_eq!(second, first);
        assert_eq!(tree.files(), after_first);
        assert_eq!(validate(Some(&harness), &tree, Format::Text).exit_code, 0);
    }

    #[test]
    fn loss_and_invalid_profile_contracts_refuse_before_writing() {
        let tree = canonical_tree();
        let mut loss = complete_harness("write");
        loss.profiles[1].supports.capabilities.clear();
        let store = MemoryAdapterStore::new(&tree);
        let outcome = generate(Some(&loss), &tree, &store, Format::Text);
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

        let mut invalid = complete_harness("read");
        invalid.profiles[1].agent_adapter.as_mut().unwrap().path =
            ".agents/agents/{name}.md".to_string();
        let outcome = validate(Some(&invalid), &tree, Format::Text);
        assert_eq!(outcome.exit_code, 2);
        assert!(outcome.stderr.contains("invalid adapter path"));
    }

    #[test]
    fn a_marker_generation_does_not_write_is_stale_and_an_owned_one_is_not() {
        let tree = canonical_tree();
        let mut harness = complete_harness("read");
        harness.marker_surfaces = vec![
            "settings/*.toml".to_string(),
            "adapters/**/*.md".to_string(),
        ];
        tree.write(
            "settings/config.toml",
            "[agents]\n# RHINO GENERATED: agents\n",
        );
        tree.write("settings/plain.toml", "[agents]\n");
        tree.write("adapters/alpha/agents/manual.md", "Rhino generated\n");
        let store = MemoryAdapterStore::new(&tree);
        let generated = generate(Some(&harness), &tree, &store, Format::Json);
        assert_eq!(generated.exit_code, 1);
        assert!(
            generated
                .stdout
                .contains("settings/config.toml: stale-marker")
        );
        assert!(!generated.stdout.contains("plain.toml"));
        assert!(!generated.stdout.contains("manual.md: stale-marker"));
        assert!(tree.read("adapters/alpha/agents/reviewer.md").is_ok());

        let validated = validate(Some(&harness), &tree, Format::Text);
        assert_eq!(validated.exit_code, 1);
        assert!(
            validated
                .stderr
                .contains("settings/config.toml: stale-marker")
        );
    }

    #[test]
    fn marker_surfaces_refuse_a_climbing_glob_an_unreadable_file_and_an_escaping_link() {
        let harness_with = |glob: &str| {
            let mut harness = complete_harness("read");
            harness.marker_surfaces = vec![glob.to_string()];
            harness
        };
        let refused = |harness: &Harness, tree: &MemoryTree| {
            let outcome = validate(Some(harness), tree, Format::Json);
            assert_eq!(outcome.exit_code, 2);
            outcome.stderr
        };

        let tree = canonical_tree();
        assert!(refused(&harness_with("../*.toml"), &tree).contains("rhino.config.unusable"));

        let mut sealed = canonical_tree();
        sealed.write("settings/config.toml", "sealed");
        sealed.mark_unreadable("settings/config.toml");
        assert!(
            refused(&harness_with("settings/*.toml"), &sealed).contains("rhino.file.unreadable")
        );

        let mut escaping = canonical_tree();
        escaping.write("settings/config.toml", "outside");
        escaping.mark_link("settings");
        let store = MemoryAdapterStore::new(&escaping);
        let generated = generate(
            Some(&harness_with("settings/*.toml")),
            &escaping,
            &store,
            Format::Json,
        );
        assert_eq!(generated.exit_code, 2);
        assert!(generated.stderr.contains("rhino.path.escapes-root"));
        // Refused before generation wrote anything.
        assert!(escaping.read("adapters/alpha/agents/reviewer.md").is_err());

        let mut binary = canonical_tree();
        binary.write("settings/config.toml", "bytes");
        binary.mark_binary("settings/config.toml");
        let outcome = validate(
            Some(&harness_with("settings/*.toml")),
            &binary,
            Format::Text,
        );
        assert!(!outcome.stderr.contains("stale-marker"));
    }

    #[test]
    fn validation_replaces_only_declared_roots_and_exact_paths() {
        let tree = canonical_tree();
        let harness = complete_harness("read");
        let store = MemoryAdapterStore::new(&tree);
        tree.write("adapters/gamma/settings.json", "user-owned\n");
        assert_eq!(
            generate(Some(&harness), &tree, &store, Format::Text).exit_code,
            0
        );
        assert_eq!(
            tree.read("adapters/gamma/settings.json").unwrap(),
            "user-owned\n"
        );
        tree.write("adapters/alpha/agents/manual.md", "handwritten\n");
        let outcome = validate(Some(&harness), &tree, Format::Text);
        assert_eq!(outcome.exit_code, 1);
        assert!(outcome.stderr.contains("manual.md: stale-adapter"));
    }

    #[test]
    fn malformed_canonical_metadata_and_json_terminal_outcomes_are_explicit() {
        let tree = MemoryTree::default();
        tree.write(ROOT_INSTRUCTION, "Canonical instruction\n");
        tree.write(".agents/agents/reviewer.md", "no front matter\n");
        let harness = complete_harness("read");
        let outcome = validate(Some(&harness), &tree, Format::Json);
        assert_eq!(outcome.exit_code, 2);
        assert!(
            outcome.stderr.contains("leading front matter"),
            "{}",
            outcome.stderr
        );
        for no_profiles in [
            generate(None, &tree, &NoAdapterStore, Format::Json),
            validate(None, &tree, Format::Json),
        ] {
            assert_eq!(no_profiles.exit_code, 2);
            assert!(no_profiles.stderr.contains("\"rhino.config.undeclared\""));
            assert!(
                no_profiles
                    .stderr
                    .contains("no adapter profiles are declared")
            );
        }
    }

    #[test]
    fn closed_profile_shapes_refuse_before_source_discovery() {
        let tree = canonical_tree();

        let mut wrong_count = complete_harness("read");
        wrong_count.profiles.pop();
        assert!(
            plan(&wrong_count, &tree)
                .err()
                .unwrap()
                .contains("exactly three")
        );

        let mut absent_shape = complete_harness("read");
        absent_shape.canonical = None;
        assert!(
            plan(&absent_shape, &tree)
                .err()
                .unwrap()
                .contains("canonical agent")
        );

        let mut absent_skill_shape = complete_harness("read");
        absent_skill_shape.canonical.as_mut().unwrap().skills = None;
        assert!(
            plan(&absent_skill_shape, &tree)
                .err()
                .unwrap()
                .contains("canonical skill")
        );

        let mut empty_id = complete_harness("read");
        empty_id.profiles[1].id.clear();
        assert!(plan(&empty_id, &tree).err().unwrap().contains("empty id"));

        let mut duplicate_id = complete_harness("read");
        duplicate_id.profiles[1].id = "alpha".to_string();
        assert!(
            plan(&duplicate_id, &tree)
                .err()
                .unwrap()
                .contains("duplicated")
        );

        let mut empty_representation = complete_harness("read");
        empty_representation.profiles[1].agent_adapter = None;
        empty_representation.profiles[1].skill_adapter = None;
        assert!(
            plan(&empty_representation, &tree)
                .err()
                .unwrap()
                .contains("no native adapter representation")
        );

        let mut invalid_instruction = complete_harness("read");
        invalid_instruction.profiles[1].instruction_adapter = Some(InstructionAdapter {
            path: "AGENTS.md".to_string(),
            route: "@{path}".to_string(),
        });
        invalid_instruction.profiles[1].agent_adapter = None;
        invalid_instruction.profiles[1].skill_adapter = None;
        assert!(
            plan(&invalid_instruction, &tree)
                .err()
                .unwrap()
                .contains("invalid instruction")
        );

        let semantic_axes: [(&str, ProfileMutator); 5] = [
            ("grant", |profile| profile.supports.grants.clear()),
            ("denial", |profile| profile.supports.denials.clear()),
            ("constraint", |profile| profile.supports.constraints.clear()),
            ("route", |profile| profile.supports.routes.clear()),
            ("identity", |profile| profile.supports.identities.clear()),
        ];
        for (axis, clear_supported) in semantic_axes {
            let mut incomplete_axis = complete_harness("read");
            clear_supported(&mut incomplete_axis.profiles[1]);
            assert!(plan(&incomplete_axis, &tree).err().unwrap().contains(axis));
        }

        let mut overlapping = complete_harness("read");
        overlapping.profiles[1].agent_adapter.as_mut().unwrap().path =
            "adapters/alpha/agents/{name}.md".to_string();
        assert!(
            plan(&overlapping, &tree)
                .err()
                .unwrap()
                .contains("non-overlapping")
        );
    }

    #[test]
    fn native_adapter_shape_and_metadata_parser_cover_representations_and_refusals() {
        let front_matter = adapter(
            "adapters/test/agents/{name}.md",
            AdapterFormat::FrontMatter,
            "body",
        );
        assert_eq!(
            validate_adapter("test", &front_matter, true).unwrap(),
            "adapters/test/agents"
        );

        let mut invalid = front_matter.clone();
        invalid.route = "no placeholder".to_string();
        assert!(
            validate_adapter("test", &invalid, true)
                .unwrap_err()
                .contains("exactly one")
        );
        invalid = front_matter.clone();
        invalid.route_field = "route".to_string();
        assert!(
            validate_adapter("test", &invalid, true)
                .unwrap_err()
                .contains("incompatible")
        );
        invalid = adapter(
            "adapters/test/agents/{name}.toml",
            AdapterFormat::Toml,
            "route",
        );
        invalid.route_field.clear();
        assert!(
            validate_adapter("test", &invalid, true)
                .unwrap_err()
                .contains("empty adapter route")
        );
        invalid = front_matter.clone();
        invalid.tier_fields = Some(TierFields {
            model: "model".to_string(),
            effort: "effort".to_string(),
        });
        assert!(
            validate_adapter("test", &invalid, false)
                .unwrap_err()
                .contains("skill adapter")
        );
        invalid.tier_fields = Some(TierFields {
            model: "model".to_string(),
            effort: "model".to_string(),
        });
        assert!(
            validate_adapter("test", &invalid, true)
                .unwrap_err()
                .contains("invalid tier")
        );
        invalid = front_matter.clone();
        invalid
            .fixed
            .insert("name".to_string(), "fixed".to_string());
        assert!(
            validate_adapter("test", &invalid, true)
                .unwrap_err()
                .contains("repeats")
        );

        for translation in [
            Translation {
                when: When::Always,
                capability: Some("read".to_string()),
                field: "tools".to_string(),
                members: vec!["Read".to_string()],
                absent_members: Vec::new(),
                entries: BTreeMap::new(),
            },
            Translation {
                when: When::Requires,
                capability: None,
                field: "tools".to_string(),
                members: vec!["Read".to_string()],
                absent_members: Vec::new(),
                entries: BTreeMap::new(),
            },
            Translation {
                when: When::Always,
                capability: None,
                field: "".to_string(),
                members: vec!["Read".to_string()],
                absent_members: Vec::new(),
                entries: BTreeMap::new(),
            },
            Translation {
                when: When::Always,
                capability: None,
                field: "tools".to_string(),
                members: Vec::new(),
                absent_members: Vec::new(),
                entries: BTreeMap::new(),
            },
        ] {
            let mut translated = front_matter.clone();
            translated.translations = vec![translation];
            assert!(validate_adapter("test", &translated, true).is_err());
        }
        let mut conflicting = front_matter.clone();
        conflicting.translations = vec![
            Translation {
                when: When::Always,
                capability: None,
                field: "tools".to_string(),
                members: vec!["Read".to_string()],
                absent_members: Vec::new(),
                entries: BTreeMap::new(),
            },
            Translation {
                when: When::Always,
                capability: None,
                field: "tools".to_string(),
                members: Vec::new(),
                absent_members: Vec::new(),
                entries: BTreeMap::from([("read".to_string(), "allow".to_string())]),
            },
        ];
        assert!(
            validate_adapter("test", &conflicting, true)
                .unwrap_err()
                .contains("mixes")
        );

        assert_eq!(
            adapter_root("adapters/{name}.md"),
            Some("adapters".to_string())
        );
        assert_eq!(adapter_root("adapters/no-name.md"), None);
        assert_eq!(adapter_root("../adapters/{name}.md"), None);
        assert!(validate_route("test", "{path}/{path}").is_err());

        for contents in [
            "name: reviewer\n",
            "---\n  name: reviewer\n---\n",
            "---\nnot-a-field\n---\n",
            "---\nname: one\nname: two\n---\n",
            "---\nrequires:\n  read\n---\n",
            "---\nrequires:\n  - \n---\n",
        ] {
            assert!(parse_front_matter("agent.md", contents).is_err());
        }
        let parsed = parse_front_matter(
            "agent.md",
            "---\nname: 'reviewer'\ndescription: >\n  Review\n  changes\nrequires:\n  - read\n---\n",
        )
        .unwrap();
        assert_eq!(
            required_scalar(&parsed, "name", "agent.md").unwrap(),
            "reviewer"
        );
        assert_eq!(
            required_scalar(&parsed, "description", "agent.md").unwrap(),
            "Review changes"
        );
        assert_eq!(
            optional_list(&parsed, Some("requires"), "agent.md")
                .unwrap()
                .len(),
            1
        );
        assert!(required_scalar(&parsed, "missing", "agent.md").is_err());
        assert!(optional_scalar(&parsed, "requires", "agent.md").is_err());
        assert!(optional_list(&parsed, Some("name"), "agent.md").is_err());
    }

    #[test]
    fn rendering_and_terminal_outcomes_cover_every_native_result() {
        let mut fields = BTreeMap::new();
        add_scalar(&mut fields, "name", "reviewer".to_string()).unwrap();
        assert!(add_scalar(&mut fields, "name", "again".to_string()).is_err());
        let members = Translation {
            when: When::Always,
            capability: None,
            field: "tools".to_string(),
            members: vec!["Read".to_string()],
            absent_members: Vec::new(),
            entries: BTreeMap::new(),
        };
        add_translation(&mut fields, &members).unwrap();
        add_translation(
            &mut fields,
            &Translation {
                members: vec!["Glob".to_string()],
                ..members.clone()
            },
        )
        .unwrap();
        let entries = Translation {
            when: When::Always,
            capability: None,
            field: "permissions".to_string(),
            members: Vec::new(),
            absent_members: Vec::new(),
            entries: BTreeMap::from([("network".to_string(), "deny".to_string())]),
        };
        add_translation(&mut fields, &entries).unwrap();
        add_scalar(&mut fields, "description", "needs: quotes".to_string()).unwrap();
        assert!(
            render_front_matter(&fields, "Read path: value")
                .unwrap()
                .contains("description: |-\n  needs: quotes\n")
        );
        assert!(render_toml(&fields).unwrap().contains("[permissions]"));
        assert_eq!(
            json(&serde_json::json!({"schemaVersion": 1}), "catalog").unwrap(),
            "{\n  \"schemaVersion\": 1\n}\n"
        );
        assert!(toml_scalar("needs quotes").contains("needs quotes"));
        assert!(route("missing", "AGENTS.md").is_err());

        let source = Source {
            id: "agent/reviewer".to_string(),
            path: ".agents/agents/reviewer.md".to_string(),
            digest: digest("reviewer"),
            kind: SourceKind::Agent,
            metadata: Some(Metadata {
                name: "reviewer".to_string(),
                description: "Review changes".to_string(),
                tier: None,
                grants: BTreeSet::new(),
                denials: BTreeSet::from(["repo-write".to_string()]),
                constraints: BTreeSet::new(),
                lists: BTreeMap::new(),
            }),
        };
        let mut denial_adapter = adapter(
            "adapters/test/agents/{name}.md",
            AdapterFormat::FrontMatter,
            "body",
        );
        denial_adapter.translations = vec![
            Translation {
                when: When::Always,
                capability: None,
                field: "tools".to_string(),
                members: vec!["Read".to_string(), "Write".to_string(), "Edit".to_string()],
                absent_members: Vec::new(),
                entries: BTreeMap::new(),
            },
            Translation {
                when: When::Denies,
                capability: Some("repo-write".to_string()),
                field: "tools".to_string(),
                members: Vec::new(),
                absent_members: vec!["Write".to_string(), "Edit".to_string()],
                entries: BTreeMap::new(),
            },
        ];
        let rendered = render_adapter(&profile("test", "read"), &denial_adapter, &source).unwrap();
        assert!(rendered.contains("tools: Read"));
        assert!(!rendered.contains("Write"));
        assert!(!rendered.contains("Edit"));

        let mut missing_member_field = denial_adapter.clone();
        missing_member_field.translations = vec![Translation {
            when: When::Denies,
            capability: Some("repo-write".to_string()),
            field: "tools".to_string(),
            members: Vec::new(),
            absent_members: vec!["Write".to_string()],
            entries: BTreeMap::new(),
        }];
        assert!(
            render_adapter(&profile("test", "read"), &missing_member_field, &source)
                .unwrap_err()
                .contains("cannot remove members from undeclared field")
        );

        let mut tree = canonical_tree();
        let harness = complete_harness("read");
        let no_store = generate(Some(&harness), &tree, &NoAdapterStore, Format::Text);
        assert_eq!(no_store.exit_code, 2);
        assert!(no_store.stderr.contains("write boundary"));

        let missing_json = validate(Some(&harness), &tree, Format::Json);
        assert_eq!(missing_json.exit_code, 1);
        assert!(missing_json.stdout.contains("missing-adapter"));

        let store = MemoryAdapterStore::new(&tree);
        assert_eq!(
            generate(Some(&harness), &tree, &store, Format::Text).exit_code,
            0
        );
        tree.write("adapters/alpha/agents/reviewer.md", "divergent\n");
        let divergent = validate(Some(&harness), &tree, Format::Text);
        assert!(divergent.stderr.contains("divergent-adapter"));
        tree.mark_binary("adapters/beta/agents/reviewer.md");
        let binary = validate(Some(&harness), &tree, Format::Text);
        assert!(binary.stderr.contains("non-text-adapter"));
        tree.mark_unreadable("adapters/gamma/agents/reviewer.md");
        let unreadable = validate(Some(&harness), &tree, Format::Text);
        assert!(unreadable.stderr.contains("unreadable-adapter"));
        assert!(clean(Format::Json, "current").stdout.contains("clean"));
        assert!(
            findings(Format::Json, vec!["finding".to_string()])
                .stdout
                .contains("finding")
        );
    }

    #[test]
    fn native_projection_errors_remain_closed_before_a_store_can_write() {
        let tree = canonical_tree();
        let mut invalid_instruction = complete_harness("read");
        invalid_instruction.profiles[0].instruction_adapter = Some(InstructionAdapter {
            path: "../CLAUDE.md".to_string(),
            route: "@{path}".to_string(),
        });
        assert!(
            plan(&invalid_instruction, &tree)
                .err()
                .unwrap()
                .contains("invalid instruction")
        );
        assert_eq!(adapter_root("/adapters/{name}.md"), None);
        assert!(require_axis("test", "capability", &["".to_string()], &[]).is_err());

        tree.write(
            ".agents/agents/reviewer.md",
            "---\nname: another\ndescription: Review changes\ntier: plan\nrequires:\n  - read\ndenies:\n  - network\nconstraints:\n  - offline\n---\nCanonical agent\n",
        );
        assert!(
            plan(&complete_harness("read"), &tree)
                .err()
                .unwrap()
                .contains("does not match its path")
        );

        let skill_mismatch = canonical_tree();
        skill_mismatch.write(
            ".agents/skills/review/SKILL.md",
            "---\nname: another\ndescription: Review skill\n---\nCanonical skill\n",
        );
        assert!(
            plan(&complete_harness("read"), &skill_mismatch)
                .err()
                .unwrap()
                .contains("does not match its path")
        );

        let missing = MemoryTree::default();
        let missing_harness = complete_harness("read");
        assert!(
            discover(
                &missing,
                missing_harness.canonical.as_ref(),
                &missing_harness.profiles
            )
            .is_err()
        );
        assert!(
            read_source(
                &missing,
                ROOT_INSTRUCTION,
                "instruction",
                SourceKind::Instruction,
                None
            )
            .err()
            .unwrap()
            .contains("missing")
        );
        let mut unreadable = canonical_tree();
        unreadable.mark_binary(ROOT_INSTRUCTION);
        assert!(
            read_source(
                &unreadable,
                ROOT_INSTRUCTION,
                "instruction",
                SourceKind::Instruction,
                None
            )
            .err()
            .unwrap()
            .contains("non-text")
        );
        unreadable.mark_unreadable(".agents/agents/reviewer.md");
        assert!(
            read_source(
                &unreadable,
                ".agents/agents/reviewer.md",
                "agent/reviewer",
                SourceKind::Agent,
                None
            )
            .err()
            .unwrap()
            .contains("cannot be read")
        );

        assert!(parse_front_matter("agent.md", "---\nname: reviewer\n").is_err());
        let duplicate_member =
            parse_front_matter("agent.md", "---\n\nrequires:\n  - read\n  - read\n---\n");
        assert!(duplicate_member.is_err());

        let source = Source {
            id: "agent/reviewer".to_string(),
            path: ".agents/agents/reviewer.md".to_string(),
            digest: digest("reviewer"),
            kind: SourceKind::Agent,
            metadata: Some(Metadata {
                name: "reviewer".to_string(),
                description: "Review changes".to_string(),
                tier: None,
                grants: BTreeSet::new(),
                denials: BTreeSet::new(),
                constraints: BTreeSet::new(),
                lists: BTreeMap::new(),
            }),
        };
        let mut absent = adapter(
            "adapters/test/agents/{name}.md",
            AdapterFormat::FrontMatter,
            "body",
        );
        absent.absent = vec!["name".to_string()];
        assert!(
            render_adapter(&profile("test", "read"), &absent, &source)
                .err()
                .unwrap()
                .contains("absent adapter field")
        );
        let mut translation_conflict = absent.clone();
        translation_conflict.absent.clear();
        translation_conflict.translations = vec![Translation {
            when: When::Always,
            capability: None,
            field: "name".to_string(),
            members: vec!["Read".to_string()],
            absent_members: Vec::new(),
            entries: BTreeMap::new(),
        }];
        assert!(render_adapter(&profile("test", "read"), &translation_conflict, &source).is_err());

        let mut desired = BTreeMap::from([("CLAUDE.md".to_string(), "exists".to_string())]);
        let mut instruction_profile = profile("instruction", "read");
        instruction_profile.agent_adapter = None;
        instruction_profile.skill_adapter = None;
        instruction_profile.instruction_adapter = Some(InstructionAdapter {
            path: "CLAUDE.md".to_string(),
            route: "@{path}".to_string(),
        });
        let instruction = Source {
            id: "instruction".to_string(),
            path: ROOT_INSTRUCTION.to_string(),
            digest: digest("instruction"),
            kind: SourceKind::Instruction,
            metadata: None,
        };
        assert!(render_profile(&instruction_profile, &[instruction], &mut desired).is_err());

        let mut invalid_family = profile("invalid", "read");
        invalid_family.agent_adapter.as_mut().unwrap().path = "invalid.md".to_string();
        assert!(render_profile(&invalid_family, &[], &mut BTreeMap::new()).is_err());

        let family_profile = profile("family", "read");
        let mut desired = BTreeMap::from([(
            "adapters/family/agents/catalog.json".to_string(),
            "exists".to_string(),
        )]);
        assert!(
            render_family(
                &family_profile,
                family_profile.agent_adapter.as_ref().unwrap(),
                SourceKind::Agent,
                &[source],
                &mut desired,
            )
            .is_err()
        );

        let mut conflicts = BTreeMap::from([(
            "tools".to_string(),
            RenderField::Scalar("fixed".to_string()),
        )]);
        let members = Translation {
            when: When::Always,
            capability: None,
            field: "tools".to_string(),
            members: vec!["Read".to_string()],
            absent_members: Vec::new(),
            entries: BTreeMap::new(),
        };
        assert!(add_translation(&mut conflicts, &members).is_err());
        conflicts.insert(
            "permissions".to_string(),
            RenderField::Scalar("fixed".to_string()),
        );
        let entries = Translation {
            when: When::Always,
            capability: None,
            field: "permissions".to_string(),
            members: Vec::new(),
            absent_members: Vec::new(),
            entries: BTreeMap::from([("network".to_string(), "deny".to_string())]),
        };
        assert!(add_translation(&mut conflicts, &entries).is_err());
    }

    #[test]
    fn list_projection_refuses_skill_adapters_and_repeated_fields() {
        let mut skill = adapter(
            "adapters/test/skills/{name}/SKILL.md",
            AdapterFormat::FrontMatter,
            "body",
        );
        skill.lists = BTreeMap::from([("skills".to_string(), CanonicalList::Skills)]);
        assert!(
            validate_adapter("test", &skill, false)
                .unwrap_err()
                .contains("agent list into a skill adapter")
        );
        let mut repeated = adapter(
            "adapters/test/agents/{name}.md",
            AdapterFormat::FrontMatter,
            "body",
        );
        repeated.lists = BTreeMap::from([("name".to_string(), CanonicalList::Skills)]);
        assert!(
            validate_adapter("test", &repeated, true)
                .unwrap_err()
                .contains("repeats")
        );
    }

    #[test]
    fn list_projection_keeps_authored_order_and_omits_an_empty_list() {
        assert_eq!(yaml_item("review"), "review");
        assert_eq!(yaml_item("needs: quotes"), "\"needs: quotes\"");
        let mut fields = BTreeMap::new();
        add_field(
            &mut fields,
            "skills",
            RenderField::Sequence(vec!["zeta".to_string(), "alpha".to_string()]),
        )
        .unwrap();
        assert_eq!(
            render_front_matter(&fields, "route").unwrap(),
            "---\nskills:\n  - zeta\n  - alpha\n---\n\nroute\n"
        );
        assert_eq!(
            render_toml(&fields).unwrap(),
            "skills = [\"zeta\", \"alpha\"]\n"
        );

        let parsed = parse_metadata(
            "agent.md",
            "---\nname: reviewer\ndescription: Review changes\nskills:\n---\n",
            canonical().agents.as_ref().unwrap(),
        )
        .unwrap();
        let source = Source {
            id: "agent/reviewer".to_string(),
            path: ".agents/agents/reviewer.md".to_string(),
            digest: digest("reviewer"),
            kind: SourceKind::Agent,
            metadata: Some(parsed),
        };
        let mut listing = adapter(
            "adapters/test/agents/{name}.md",
            AdapterFormat::FrontMatter,
            "body",
        );
        listing.lists = BTreeMap::from([("skills".to_string(), CanonicalList::Skills)]);
        let rendered = render_adapter(&profile("test", "read"), &listing, &source).unwrap();
        assert!(!rendered.contains("skills"), "{rendered}");
    }

    #[test]
    fn agent_selection_refuses_a_skill_adapter() {
        let mut skill = adapter(
            "adapters/test/skills/{name}/SKILL.md",
            AdapterFormat::FrontMatter,
            "body",
        );
        skill.agents = Some(vec!["reviewer".to_string()]);
        assert!(
            validate_adapter("test", &skill, false)
                .unwrap_err()
                .contains("selects agents in a skill adapter")
        );
    }
}
