//! Coding-harness parity.
//!
//! A repository declares one canonical instruction body, one canonical skill
//! bundle per skill, one canonical agent prompt per agent, and a roster of the
//! harnesses that must reach them. This reconciles the canon against every
//! declared harness.
//!
//! It knows no harness by name. The roster is data, every path in it comes from
//! configuration, and adding a fourth harness is one entry rather than one code
//! change -- which is the difference between a validator that serves one
//! repository and one that serves four.

use crate::config::{
    Adapter, Capability, CapabilityFormat, Config, DeclarationShape, DocumentFormat, Harness,
    HarnessParity, Identity, RequiredMcp, TierMapping, Translation, When,
};
use crate::markdown;
use crate::report::{Finding, Report};
use crate::runtime::{Tree, TreeError};
use crate::scan::{self, Scope};
use globset::GlobSet;
use serde::Deserialize;
use serde_json::Value;
use sha2::{Digest, Sha256};
use std::collections::{BTreeMap, BTreeSet};

/// The one file a skill bundle must carry.
const SKILL_FILE: &str = "SKILL.md";

/// The file a directory documents itself in.
///
/// RHINO already owns this name -- it is where a directory map lives -- so
/// reading it as an index here states one fact about it consistently rather
/// than introducing a second, consumer-shaped one. A canonical root and a
/// harness's adapter directory are both places a repository keeps an index,
/// and an index is not a declaration of anything.
const INDEX_FILE: &str = "README.md";

/// What a declaration front matter can say.
///
/// Every field is optional here and required by the rules below, so a file
/// missing one is reported as an invalid declaration rather than failing to
/// parse into a message nobody wrote.
#[derive(Debug, Default, Clone, PartialEq, Eq, Deserialize)]
struct Declaration {
    name: Option<String>,
    description: Option<String>,
    /// The portable tier this artifact declares, when it declares one.
    ///
    /// Read here rather than in the metadata command because this is where the
    /// tier is *used*: it selects the mapping a harness's adapter has to
    /// project, and an agent with no tier has nothing to project.
    tier: Option<String>,
    #[serde(default)]
    capabilities: Vec<String>,
    #[serde(default)]
    denied: Vec<String>,
    #[serde(default)]
    constraints: Vec<String>,
}

impl Declaration {
    /// What the declaration *permits*, as opposed to how it is worded.
    ///
    /// Two adapters with different prose but these three sets equal are
    /// semantically the same agent; two with identical prose and different sets
    /// are not. Keeping the comparison separate is what lets the two kinds of
    /// drift be reported as the different problems they are.
    fn semantics(&self) -> (BTreeSet<&str>, BTreeSet<&str>, BTreeSet<&str>) {
        (
            self.capabilities.iter().map(String::as_str).collect(),
            self.denied.iter().map(String::as_str).collect(),
            self.constraints.iter().map(String::as_str).collect(),
        )
    }
}

/// Everything the repository declares once, gathered before any harness is
/// looked at.
///
/// Read once and borrowed by each reconciliation rather than threaded through
/// as four parameters: the canon is a property of the repository, not of the
/// harness being checked against it, and a signature that says so is one a
/// fifth check can join without another argument.
struct Canon<'a> {
    skills_root: Option<&'a str>,
    agents_root: Option<&'a str>,
    skill_route: Option<&'a str>,
    agent_route: Option<&'a str>,
    skills: BTreeMap<String, Declaration>,
    agents: BTreeMap<String, Declaration>,
    required: Option<&'a RequiredMcp>,
    /// The model and effort each harness uses for each portable tier.
    ///
    /// Part of the canon rather than of a harness, because the mapping is the
    /// repository's one statement about tiers and every harness's adapter is
    /// held to the same one.
    tiers: Option<&'a BTreeMap<String, BTreeMap<String, Option<TierMapping>>>>,
}

/// What a canonical document says about itself, for documents that only name
/// and describe themselves.
///
/// Read through the same parser an adapter is read through, because a canonical
/// document and the adapters routing to it are the same kind of file and a
/// second reader would eventually disagree with the first.
fn read_identity(text: &str) -> Option<Declaration> {
    let read = Read::parse(text, DocumentFormat::FrontMatter)?;
    Some(Declaration {
        name: read.scalar("name"),
        description: read.scalar("description"),
        ..Declaration::default()
    })
}

/// A canonical agent's declaration, or why the file is not one.
///
/// The three permission lists are found by the names the repository declared
/// rather than by names chosen here. That is not a convenience: a list read
/// under a name nobody wrote comes back empty rather than missing, so a
/// hard-coded name would leave every translation with nothing to fire on and
/// report the comparison clean without having made it.
fn read_declaration(text: &str, shape: &DeclarationShape) -> Result<Declaration, String> {
    let Some(read) = Read::parse(text, DocumentFormat::FrontMatter) else {
        return Err("an agent needs a declaration naming and describing it".to_string());
    };
    for (field, value) in &shape.fixed {
        if read.scalar(field).as_deref() != Some(value.as_str()) {
            return Err(format!("`{field}` must be declared as `{value}`"));
        }
    }
    Ok(Declaration {
        name: read.scalar("name"),
        description: read.scalar("description"),
        tier: read.scalar("tier"),
        capabilities: read.members(&shape.grants).into_iter().collect(),
        denied: read.members(&shape.denials).into_iter().collect(),
        constraints: read.members(&shape.limits).into_iter().collect(),
    })
}

/// Whether this validator could use the *content* of a path.
///
/// Every rule here is about one of five things: the canonical instruction body
/// and the one file permitted to route to it, a canonical skill or agent, an
/// adapter for one of those, a harness's capability declaration, or a file
/// competing with the canon. The first four are declared paths. The fifth is
/// either a declared glob -- a path question, and the reason a glob match is
/// read despite not being Markdown -- or an import, which only Markdown can
/// express.
///
/// Anything else is opened only to be discarded, and the discarding is not
/// free: a working tree carries compiled artifacts and databases far larger
/// than its documentation.
///
/// A path the walk lists and this predicate rejects is still seen by every
/// rule that asks about a path. What it is spared is the read.
fn readable_set<'a>(
    parity: &'a HarnessParity,
    prohibited: Option<&'a GlobSet>,
) -> impl Fn(&str) -> bool + 'a {
    let canonical = &parity.canonical;

    let mut roots: Vec<String> = [
        canonical.skills_root.as_deref(),
        canonical.agents_root.as_deref(),
    ]
    .into_iter()
    .flatten()
    .map(|root| format!("{}/", root.trim_end_matches('/')))
    .collect();

    for harness in &parity.harnesses {
        for adapter in [
            harness.agent_adapter.as_ref(),
            harness.skill_adapter.as_ref(),
        ]
        .into_iter()
        .flatten()
        {
            // The literal head of the `{name}` pattern rather than the pattern
            // itself. A file per document and a directory per document both
            // live under it, and an adapter no canonical document asked for is
            // reported from whatever is found there -- so the whole head has
            // to be read, not just the paths a known name would produce.
            let head = adapter.path.split("{name}").next().unwrap_or_default();
            roots.push(head.to_string());
        }
    }

    let capability_files: BTreeSet<&str> = parity
        .harnesses
        .iter()
        .filter_map(|harness| harness.capability.as_ref())
        .map(|capability| capability.file.as_str())
        .collect();

    // Named outright by the repository, in a format that is not Markdown, so
    // no other clause here would reach it.
    let prohibited_fields: BTreeSet<&str> = parity
        .prohibited_instruction_fields
        .iter()
        .map(|declared| declared.file.as_str())
        .collect();

    move |path: &str| {
        scan::is_markdown(path)
            || path == canonical.instruction
            || Some(path) == canonical.instruction_adapter.as_deref()
            || capability_files.contains(path)
            || prohibited_fields.contains(path)
            || roots.iter().any(|root| path.starts_with(root))
            || prohibited.is_some_and(|globs| globs.is_match(path))
    }
}

pub fn validate(tree: &dyn Tree, config: &Config, scope: &Scope) -> Report {
    let parity = &config.harness_parity;
    let files = scan::files(tree, config);

    let mut report = Report::new("harness-parity", "harness");
    let mut refusal: Option<String> = None;

    // Compiled once and shared, because it decides two things that have to
    // agree: which files are read, and which of them compete with the canon.
    // A glob nobody can compile leaves the instruction check unmade rather
    // than half made.
    let prohibited = scan::glob_set(
        "harness-parity.prohibited-instruction-sources",
        parity
            .prohibited_instruction_sources
            .iter()
            .map(String::as_str),
    )
    .ok();
    let wanted = readable_set(parity, prohibited.as_ref());

    // Read once, and only what a rule here could be about. A validator that
    // read the same adapter twice could report two different things about it;
    // one that read the whole repository would open every compiled artifact,
    // dialyzer table, and database file a working tree has accumulated, to
    // discard each as soon as it turned out not to be text. Measured on a real
    // repository, that was 98.9 MB read to use 1.9 MB.
    //
    // Paths still come from the full walk. Only reading narrows, because two
    // of the rules below -- a prohibited name, and an adapter nobody declared
    // -- are about a path rather than about what is in it.
    let mut contents: BTreeMap<String, String> = BTreeMap::new();
    for path in &files {
        if !wanted(path) {
            continue;
        }
        match tree.read(path) {
            Ok(text) => {
                contents.insert(path.clone(), text);
            }
            Err(TreeError::NotFound) => {}
            // Alone among the walks, this one reads files that are not
            // Markdown, so it is the only one that can meet a repository's
            // images and archives -- a prohibited glob matches a path
            // whatever its kind. None of them can be an instruction body, a
            // skill, an agent, or a capability declaration, and refusing the
            // run over a logo would put parity out of reach of any repository
            // that ships one.
            Err(TreeError::NotText) => {}
            Err(TreeError::Unreadable(reason)) => {
                refusal.get_or_insert(format!("{path}: {reason}"));
            }
        }
    }
    if let Some(reason) = refusal {
        return Report::refused("harness-parity", reason);
    }

    instructions(&contents, config, prohibited.as_ref(), &mut report);
    let canon = Canon {
        skills_root: parity.canonical.skills_root.as_deref(),
        agents_root: parity.canonical.agents_root.as_deref(),
        skill_route: parity.canonical.skill_route.as_deref(),
        agent_route: parity.canonical.agent_route.as_deref(),
        skills: skills(
            &contents,
            parity.canonical.skills_root.as_deref(),
            &mut report,
        ),
        agents: agents(
            &contents,
            parity.canonical.agents_root.as_deref(),
            config,
            &mut report,
        ),
        required: parity.required_mcp.as_ref(),
        tiers: config.model_tiers.as_ref(),
    };

    // The whole of RHINO's harness knowledge: iterate the declared roster and
    // reconcile each entry against the canon. A fourth harness is one more
    // entry in `repo-config.yml` and no line here, which is what makes this a
    // tool four repositories can share rather than one repository's script.
    // A narrowed roster still reports every finding it inspects. It reports
    // fewer, because it looked at fewer -- narrowing is a smaller question,
    // never a quieter answer to the same one.
    let roster: Vec<&Harness> = match &scope.harness {
        Some(wanted) => {
            let selected: Vec<&Harness> = parity
                .harnesses
                .iter()
                .filter(|harness| &harness.name == wanted)
                .collect();
            if selected.is_empty() {
                return Report::refused(
                    "harness-parity",
                    format!("`{wanted}` is not a harness this repository declares"),
                );
            }
            selected
        }
        None => parity.harnesses.iter().collect(),
    };

    let mut reconciled = 0usize;
    for harness in roster.iter().copied() {
        report.inspected_one();
        agent_adapters(&contents, harness, &canon, &mut report);
        skill_wrappers(&contents, harness, &canon, &mut report);
        // The configuration contract pairs the required server with the
        // per-harness declaration that has to satisfy it, so these two are
        // `Some` together or neither is. A repository that requires no server
        // reconciles none, and says so by counting zero.
        if let (Some(required), Some(declared)) = (canon.required, harness.capability.as_ref())
            && capability(&contents, harness, declared, required, &mut report)
        {
            reconciled += 1;
        }
    }

    // Every number here is counted from what was actually read. A constant in
    // this line would report a capability declaration the run never compared.
    report.note(format!(
        "canon {} harnesses, {} skills, {} agents, {reconciled} reconciled capability declarations",
        roster.len(),
        canon.skills.len(),
        canon.agents.len(),
    ));
    report.note(format!("digest {}", digest(&contents, config)));

    report
}

// -- The instruction boundary -------------------------------------------------

fn instructions(
    contents: &BTreeMap<String, String>,
    config: &Config,
    prohibited: Option<&GlobSet>,
    report: &mut Report,
) {
    let canonical = &config.harness_parity.canonical;
    let instruction = canonical.instruction.as_str();
    let adapter = canonical.instruction_adapter.as_deref();
    let route = format!("@{instruction}");

    if !contents.contains_key(instruction) {
        report.found(Finding::new(
            "missing-instruction",
            instruction,
            "missing-instruction: the declared canonical instruction body is not here",
        ));
    }

    if let Some(adapter) = adapter {
        match contents.get(adapter) {
            None => {
                report.found(Finding::new(
                    "missing-instruction-adapter",
                    adapter,
                    "missing-instruction-adapter: the declared adapter is not here",
                ));
            }
            // The permitted content is derived, not configured: an adapter
            // exists to route to the instruction and to do nothing else, so
            // anything beyond the import is a second instruction source
            // wearing the adapter's name.
            Some(text) if text.trim() != route => {
                report.found(Finding::new(
                    "invalid-instruction-adapter",
                    adapter,
                    format!(
                        "invalid-instruction-adapter: an adapter may contain `{route}` and nothing else"
                    ),
                ));
            }
            Some(_) => {}
        }
    }

    // A harness that reads its always-on instructions out of its own settings
    // is the same competing source in that vendor's syntax. The file is not
    // prohibited -- it holds everything else the harness needs -- so the
    // question is asked of one declared key, and a key nobody wrote and a key
    // written empty are both answers rather than violations.
    for declared in &config.harness_parity.prohibited_instruction_fields {
        let Some(text) = contents.get(&declared.file) else {
            continue;
        };
        let parsed = match declared.format {
            CapabilityFormat::Json => serde_json::from_str::<Value>(text).ok(),
            CapabilityFormat::Toml => toml::from_str::<Value>(text).ok(),
        };
        let Some(document) = parsed else {
            report.found(Finding::new(
                "unexpected-instruction-source",
                &declared.file,
                "unexpected-instruction-source: the file is not readable in its declared format",
            ));
            continue;
        };
        if document
            .get(&declared.field)
            .is_some_and(carries_instructions)
        {
            report.found(Finding::new(
                "unexpected-instruction-source",
                &declared.file,
                format!(
                    "unexpected-instruction-source: `{}` carries instructions that compete with the canon",
                    declared.field
                ),
            ));
        }
    }

    // An unusable glob leaves this check unmade. Compiled by the caller, which
    // needs the same answer to decide what to read.
    let Some(prohibited) = prohibited else {
        return;
    };

    for (path, text) in contents {
        if path == instruction || Some(path.as_str()) == adapter {
            continue;
        }
        // Two ways to be a competing always-on instruction: to be named like
        // one, or to route to the canonical body without being the file
        // permitted to. The first is the repository's declared globs and
        // applies to every file whatever its kind; the second is a question
        // only Markdown can answer.
        //
        // Nothing else a repository holds is an always-on instruction to any
        // harness. Source, fixtures, and data may all contain the route -- the
        // code implementing this very check has to -- and a scan that read
        // them could not be implemented without accusing its own
        // implementation.
        //
        // Within Markdown the route only counts where it is prose. A page that
        // shows the import inside a fenced example or a code span is
        // documenting the adapter rather than being one, and a validator that
        // could not tell those apart could not be documented without tripping
        // itself.
        //
        // A document that ends inside a fence gets no such benefit. Its
        // remainder is an example only by the accident of a missing closer, and
        // granting the exemption there would make the check evadable by one
        // stray line.
        let imports = scan::is_markdown(path)
            && if markdown::ends_inside_a_fence(text) {
                text.contains(&route)
            } else {
                markdown::prose_lines(text)
                    .iter()
                    .any(|(_, line)| markdown::without_code_spans(line).contains(&route))
            };
        if prohibited.is_match(path) || imports {
            report.found(Finding::new(
                "unexpected-instruction-source",
                path,
                "unexpected-instruction-source: a second always-on instruction source competes with the canon",
            ));
        }
    }
}

/// Whether a configuration value says anything, whatever shape it says it in.
///
/// A key written as an empty list, an empty string, or `null` is a repository
/// stating that this harness carries no instructions of its own, which is the
/// answer the rule wants rather than a violation of it.
fn carries_instructions(value: &Value) -> bool {
    match value {
        Value::Null => false,
        Value::String(text) => !text.trim().is_empty(),
        Value::Array(items) => !items.is_empty(),
        Value::Object(fields) => !fields.is_empty(),
        _ => true,
    }
}

// -- Skills -------------------------------------------------------------------

/// The canonical skills, by declared name.
fn skills(
    contents: &BTreeMap<String, String>,
    root: Option<&str>,
    report: &mut Report,
) -> BTreeMap<String, Declaration> {
    let mut found: BTreeMap<String, Declaration> = BTreeMap::new();
    let Some(root) = root else {
        return found;
    };
    let prefix = format!("{}/", root.trim_end_matches('/'));

    for (path, text) in contents {
        let Some(rest) = path.strip_prefix(&prefix) else {
            continue;
        };
        let Some((slug, tail)) = rest.split_once('/') else {
            continue;
        };
        if tail != SKILL_FILE {
            // Everything else under a skill directory is part of that skill's
            // bundle rather than a skill of its own.
            continue;
        }

        let Some(declaration) = read_identity(text) else {
            report.found(Finding::new(
                "invalid-skill",
                path,
                "invalid-skill: a skill needs a declaration naming and describing it",
            ));
            continue;
        };
        let (Some(name), Some(_)) = (declaration.name.clone(), declaration.description.clone())
        else {
            report.found(Finding::new(
                "invalid-skill",
                path,
                "invalid-skill: a skill declaration needs both a name and a description",
            ));
            continue;
        };
        if name != slug {
            report.found(Finding::new(
                "invalid-skill",
                path,
                format!("invalid-skill: the declared name `{name}` does not match the directory `{slug}`"),
            ));
            continue;
        }
        // Insertion cannot collide: the name is required to equal the unique
        // directory component of a unique path, so requiring the two to match
        // is what makes two skills sharing a name unrepresentable rather than
        // merely detectable.
        found.insert(name, declaration);
    }

    found
}

// -- Agents -------------------------------------------------------------------

fn agents(
    contents: &BTreeMap<String, String>,
    root: Option<&str>,
    config: &Config,
    report: &mut Report,
) -> BTreeMap<String, Declaration> {
    let mut found: BTreeMap<String, Declaration> = BTreeMap::new();
    // Paired by `repo-config validate`, so one missing means the other is too:
    // a repository with no canonical agents, which has none of these to read.
    let (Some(root), Some(shape)) = (root, config.harness_parity.canonical.declaration.as_ref())
    else {
        return found;
    };
    let prefix = format!("{}/", root.trim_end_matches('/'));
    let vocabulary: BTreeSet<&str> = config
        .harness_parity
        .capabilities
        .iter()
        .map(String::as_str)
        .collect();
    let constraints: BTreeSet<&str> = config
        .harness_parity
        .constraints
        .iter()
        .map(String::as_str)
        .collect();

    for (path, text) in contents {
        let Some(rest) = path.strip_prefix(&prefix) else {
            continue;
        };
        if rest.contains('/') {
            continue;
        }
        if rest == INDEX_FILE {
            continue;
        }
        let Some(name) = rest.strip_suffix(".md") else {
            continue;
        };
        let declaration = match read_declaration(text, shape) {
            Ok(declaration) => declaration,
            Err(complaint) => {
                report.found(Finding::new(
                    "invalid-agent",
                    path,
                    format!("invalid-agent: {complaint}"),
                ));
                continue;
            }
        };

        // A name outside the declared vocabulary grants or denies nothing, so a
        // typo would silently weaken an agent rather than fail.
        for capability in declaration.capabilities.iter().chain(&declaration.denied) {
            if !vocabulary.contains(capability.as_str()) {
                report.found(Finding::new(
                    "unknown-capability",
                    path,
                    format!(
                        "unknown-capability: `{capability}` is outside the declared vocabulary"
                    ),
                ));
            }
        }
        for constraint in &declaration.constraints {
            if !constraints.contains(constraint.as_str()) {
                report.found(Finding::new(
                    "unknown-capability",
                    path,
                    format!("unknown-capability: constraint `{constraint}` is outside the declared vocabulary"),
                ));
            }
        }

        found.insert(name.to_string(), declaration);
    }

    found
}

fn agent_adapters(
    contents: &BTreeMap<String, String>,
    harness: &Harness,
    canon: &Canon<'_>,
    report: &mut Report,
) {
    // All three are `Some` together or none is: the schema pairs the root with
    // its route and with every harness's contract for expressing it.
    let (Some(adapter), Some(root), Some(template)) = (
        harness.agent_adapter.as_ref(),
        canon.agents_root,
        canon.agent_route,
    ) else {
        return;
    };

    for (name, expected) in &canon.agents {
        let path = adapter_path(adapter, name);
        let Some(text) = contents.get(&path) else {
            report.found(Finding::new(
                "missing-agent-adapter",
                &path,
                format!(
                    "missing-agent-adapter: `{}` has no adapter for the canonical agent `{name}`",
                    harness.name
                ),
            ));
            continue;
        };
        let Some(read) = Read::parse(text, adapter.format) else {
            report.found(Finding::new(
                "invalid-agent",
                &path,
                "invalid-agent: an adapter needs a declaration this harness's format can read",
            ));
            continue;
        };

        let route = route_for(
            template,
            &format!("{}/{name}.md", root.trim_end_matches('/')),
        );
        let found = shortfalls(
            &read,
            adapter,
            name,
            expected.description.as_deref(),
            &route,
            &Permits::of(expected),
        );

        tier_projection(&read, adapter, harness, canon, expected, &path, report);

        // Semantics first, and the route only when the semantics agree. An
        // adapter that permits something the canon refused is a different
        // agent, not one that quoted the route wrongly, and reporting the
        // smaller fault first would understate it.
        if let Some(fault) = found.semantic.first() {
            report.found(Finding::new(
                "agent-semantic-divergence",
                &path,
                format!("agent-semantic-divergence: {fault}"),
            ));
        } else if let Some(fault) = found.route {
            report.found(Finding::new(
                "agent-prompt-divergence",
                &path,
                format!("agent-prompt-divergence: {fault}"),
            ));
        }
    }

    unexpected(
        contents,
        &adapter.path,
        &canon.agents,
        "unexpected-agent-adapter",
        "has no canonical agent behind it",
        report,
    );
}

/// What this adapter projects for the canonical agent's portable tier.
///
/// Three states, and each is a different fault. A mapped tier that projects
/// nothing leaves the harness inheriting a model the repository chose against.
/// A mapped tier that projects half a pair is the same thing wearing the shape
/// of a complete answer. An unmapped tier that projects anything is a vendor
/// choice made in an adapter, which is the one place the contract says it may
/// not be made -- an adapter is an output.
///
/// A harness declaring no tier fields is held to none of it. Which field
/// carries a model is that harness's vocabulary, and omitting it is a valid
/// mapping rather than a gap.
fn tier_projection(
    read: &Read,
    adapter: &Adapter,
    harness: &Harness,
    canon: &Canon<'_>,
    expected: &Declaration,
    path: &str,
    report: &mut Report,
) {
    let (Some(fields), Some(tier)) = (adapter.tier_fields.as_ref(), expected.tier.as_deref())
    else {
        return;
    };
    let mapped = canon
        .tiers
        .and_then(|tiers| tiers.get(&harness.name))
        .and_then(|tiers| tiers.get(tier))
        .and_then(Option::as_ref);
    let model = read.scalar(&fields.model);
    let effort = read.scalar(&fields.effort);

    match mapped {
        Some(mapping) => {
            if model.is_none() || effort.is_none() {
                report.found(Finding::new(
                    "unprojected-tier",
                    path,
                    format!(
                        "unprojected-tier: `{}` maps the tier `{tier}` and this adapter projects only part of it; a present tier carries both a model and an effort",
                        harness.name
                    ),
                ));
                return;
            }
            // Total in this arm: both were just found to be present.
            let (model, effort) = (model.unwrap_or_default(), effort.unwrap_or_default());
            if Some(&model) != mapping.model.as_ref() || Some(&effort) != mapping.effort.as_ref() {
                report.found(Finding::new(
                    "tier-projection-drift",
                    path,
                    format!(
                        "tier-projection-drift: this adapter projects `{model}` at `{effort}` for the tier `{tier}`, and the mapping names `{}` at `{}`",
                        mapping.model.clone().unwrap_or_default(),
                        mapping.effort.clone().unwrap_or_default()
                    ),
                ));
            }
        }
        None if model.is_some() || effort.is_some() => {
            report.found(Finding::new(
                "unmapped-tier-projected",
                path,
                format!(
                    "unmapped-tier-projected: this adapter projects a model or an effort for the tier `{tier}`, and nothing maps it; an absent mapping means the harness applies its own inheritance"
                ),
            ));
        }
        None => {}
    }
}

fn skill_wrappers(
    contents: &BTreeMap<String, String>,
    harness: &Harness,
    canon: &Canon<'_>,
    report: &mut Report,
) {
    // Only a harness that declares wrappers has wrappers to get wrong. One
    // that declares none is complete without them.
    let Some(adapter) = harness.skill_adapter.as_ref() else {
        return;
    };
    let (Some(root), Some(template)) = (canon.skills_root, canon.skill_route) else {
        return;
    };

    for (name, canonical) in &canon.skills {
        let path = adapter_path(adapter, name);
        let Some(text) = contents.get(&path) else {
            report.found(Finding::new(
                "missing-skill-adapter",
                &path,
                format!(
                    "missing-skill-adapter: `{}` declares skill wrappers but has none for `{name}`",
                    harness.name
                ),
            ));
            continue;
        };
        let Some(read) = Read::parse(text, adapter.format) else {
            report.found(Finding::new(
                "skill-content-divergence",
                &path,
                "skill-content-divergence: a wrapper needs a declaration this harness's format can read",
            ));
            continue;
        };

        let route = route_for(
            template,
            &format!("{}/{name}/{SKILL_FILE}", root.trim_end_matches('/')),
        );
        // A wrapper routes to a skill and takes on nothing of its own, so
        // every way it can fall short is the same fault: it stopped being a
        // route. One message all the same, because a reader told only that a
        // wrapper diverged still has to open both files to find out how.
        let found = shortfalls(
            &read,
            adapter,
            name,
            canonical.description.as_deref(),
            &route,
            &Permits::of(canonical),
        );
        if let Some(fault) = found.semantic.first().cloned().or(found.route) {
            report.found(Finding::new(
                "skill-content-divergence",
                &path,
                format!("skill-content-divergence: {fault}"),
            ));
        }
    }

    unexpected(
        contents,
        &adapter.path,
        &canon.skills,
        "unexpected-skill-adapter",
        "has no canonical skill behind it",
        report,
    );
}

/// Adapters this harness holds that no canonical document asked for.
///
/// The same question for skills and for agents, asked once: a directory of
/// adapters is only as trustworthy as its emptiest corner, and one nobody
/// declared is a prompt that runs without ever being reconciled.
fn unexpected<T>(
    contents: &BTreeMap<String, String>,
    pattern: &str,
    canonical: &BTreeMap<String, T>,
    kind: &'static str,
    complaint: &str,
    report: &mut Report,
) {
    for path in contents.keys() {
        let Some(name) = name_in(pattern, path) else {
            continue;
        };
        if canonical.contains_key(&name) {
            continue;
        }
        report.found(Finding::new(
            kind,
            path,
            format!("{kind}: `{name}` {complaint}"),
        ));
    }
}

// -- Capabilities -------------------------------------------------------------

/// Whether this harness's declaration was found, parsed, and matched.
fn capability(
    contents: &BTreeMap<String, String>,
    harness: &Harness,
    declared: &Capability,
    required: &RequiredMcp,
    report: &mut Report,
) -> bool {
    let Some(text) = contents.get(&declared.file) else {
        report.found(Finding::new(
            "divergent-capability",
            &declared.file,
            format!(
                "divergent-capability: `{}` declares no capability file",
                harness.name
            ),
        ));
        return false;
    };

    // Both formats are read into one shape, so the comparison below is about
    // meaning and a harness is never penalised for the syntax its own
    // configuration format uses.
    let parsed = match declared.format {
        CapabilityFormat::Json => serde_json::from_str::<Value>(text).ok(),
        CapabilityFormat::Toml => toml::from_str::<Value>(text).ok(),
    };
    let Some(document) = parsed else {
        report.found(Finding::new(
            "divergent-capability",
            &declared.file,
            "divergent-capability: the capability declaration is not readable in its declared format",
        ));
        return false;
    };

    // Searched for by name rather than by key path, because the key a vendor
    // nests its servers under is that vendor's syntax and naming it here would
    // put a harness back in the binary.
    let Some(entry) = find_named(&document, &required.name) else {
        report.found(Finding::new(
            "divergent-capability",
            &declared.file,
            format!(
                "divergent-capability: `{}` does not declare the required capability `{}`",
                harness.name, required.name
            ),
        ));
        return false;
    };

    let declared_vector = executable_vector(entry);
    let expected: Vec<&str> = std::iter::once(required.command.as_str())
        .chain(required.args.iter().map(String::as_str))
        .collect();
    if declared_vector.as_deref() != Some(expected.as_slice()) {
        report.found(Finding::new(
            "divergent-capability",
            &declared.file,
            format!(
                "divergent-capability: `{}` declares a different executable vector for `{}`",
                harness.name, required.name
            ),
        ));
        return false;
    }
    true
}

/// The first object in a document declared under `name` that looks like an
/// executable declaration.
fn find_named<'a>(value: &'a Value, name: &str) -> Option<&'a Value> {
    match value {
        Value::Object(map) => {
            if let Some(candidate) = map.get(name)
                && candidate.get("command").is_some()
            {
                return Some(candidate);
            }
            map.values().find_map(|nested| find_named(nested, name))
        }
        Value::Array(items) => items.iter().find_map(|item| find_named(item, name)),
        _ => None,
    }
}

// -- Adapters -----------------------------------------------------------------

/// One adapter, read into the shape every rule below asks questions of.
///
/// Two document formats reach the same structure, so a rule is written once and
/// a harness is never penalised for the syntax its own vendor chose.
struct Read {
    fields: BTreeMap<String, Value>,
    body: String,
}

impl Read {
    fn parse(text: &str, format: DocumentFormat) -> Option<Self> {
        match format {
            DocumentFormat::Toml => {
                let table = toml::from_str::<Value>(text).ok()?;
                Some(Self {
                    fields: table.as_object()?.clone().into_iter().collect(),
                    // A TOML adapter has no prose beneath a declaration, so its
                    // route lives in a named field and `body` is never asked
                    // for. Empty here rather than absent, because the question
                    // "does the body match?" still has to have an answer.
                    body: String::new(),
                })
            }
            DocumentFormat::FrontMatter => {
                let mut lines = text.lines();
                if lines.next()?.trim() != "---" {
                    return None;
                }
                let mut front = String::new();
                for line in lines.by_ref() {
                    if line.trim() == "---" {
                        let parsed: Value = yaml_serde::from_str(&front).ok()?;
                        return Some(Self {
                            fields: parsed.as_object()?.clone().into_iter().collect(),
                            body: lines.collect::<Vec<_>>().join("\n").trim().to_string(),
                        });
                    }
                    front.push_str(line);
                    front.push('\n');
                }
                None
            }
        }
    }

    /// A field read as a scalar, whatever scalar type it was written as.
    fn scalar(&self, field: &str) -> Option<String> {
        match self.fields.get(field)? {
            Value::String(value) => Some(value.clone()),
            // A field the rules read as a scalar and a repository wrote as a
            // list or a map is not a scalar with a different value; it is not
            // one at all, and reports as the mismatch it is.
            _ => None,
        }
    }

    /// A field read as a set of members.
    ///
    /// A sequence contributes its items and a scalar contributes its
    /// comma-separated parts, which is the whole of the difference between a
    /// harness that writes `tools: Read, Grep` and one that writes a list.
    fn members(&self, field: &str) -> BTreeSet<String> {
        match self.fields.get(field) {
            Some(Value::Array(items)) => items
                .iter()
                .filter_map(Value::as_str)
                .map(str::to_string)
                .collect(),
            Some(Value::String(value)) => value
                .split(',')
                .map(str::trim)
                .filter(|part| !part.is_empty())
                .map(str::to_string)
                .collect(),
            _ => BTreeSet::new(),
        }
    }

    /// A field read as a mapping of key to scalar value.
    fn entries(&self, field: &str) -> BTreeMap<String, String> {
        match self.fields.get(field) {
            Some(Value::Object(map)) => map
                .iter()
                .filter_map(|(key, value)| {
                    value.as_str().map(|value| (key.clone(), value.to_string()))
                })
                .collect(),
            _ => BTreeMap::new(),
        }
    }

    /// The route this adapter carries, wherever its harness keeps it.
    /// The route this adapter carries, as a sentence rather than as bytes.
    ///
    /// Runs of whitespace collapse to one space before the comparison. A route
    /// is prose, every Markdown formatter wraps prose, and both repositories
    /// this reconciles run one -- so comparing the bytes would report drift on
    /// an adapter nobody edited and offer no remedy but turning the formatter
    /// off. Where a line breaks is not something an adapter can be said to have
    /// got wrong.
    fn route(&self, adapter: &Adapter) -> String {
        let raw = if adapter.route_field == BODY {
            self.body.clone()
        } else {
            self.scalar(&adapter.route_field).unwrap_or_default()
        };
        raw.split_whitespace().collect::<Vec<_>>().join(" ")
    }
}

/// The whole command line a capability declaration names, flattened.
///
/// One harness writes the executable and its arguments as a scalar `command`
/// beside an `args` array; another writes the entire vector as `command`. What
/// the two agree on is the sequence of words that gets run, so that is what is
/// compared -- pinning either split would describe one vendor's syntax the way
/// pinning the key path would, and reject a server that is in fact identical.
fn executable_vector(entry: &Value) -> Option<Vec<&str>> {
    let mut vector = match entry.get("command")? {
        Value::String(command) => vec![command.as_str()],
        Value::Array(words) => words.iter().map(Value::as_str).collect::<Option<_>>()?,
        _ => return None,
    };
    if let Some(Value::Array(arguments)) = entry.get("args") {
        for argument in arguments {
            vector.push(argument.as_str()?);
        }
    }
    Some(vector)
}

/// Where the route lives when it is prose rather than a field.
const BODY: &str = "body";

/// The path this harness keeps its adapter for `name` at.
fn adapter_path(adapter: &Adapter, name: &str) -> String {
    adapter.path.replace("{name}", name)
}

/// The canonical document a path is an adapter for, if it is one.
///
/// The inverse of `adapter_path`, and the reason a pattern beats a directory
/// and an extension: a file per document and a directory per document are both
/// read by the same two halves.
fn name_in(pattern: &str, path: &str) -> Option<String> {
    let (prefix, suffix) = pattern.split_once("{name}")?;
    let rest = path.strip_prefix(prefix)?.strip_suffix(suffix)?;
    // A name is one path component. Anything deeper is a supporting file
    // inside an adapter, not a second adapter, and an index is neither.
    if rest.is_empty() || rest.contains('/') || format!("{rest}{suffix}") == INDEX_FILE {
        return None;
    }
    Some(rest.to_string())
}

/// The route sentence a canonical document at `path` obliges.
fn route_for(template: &str, path: &str) -> String {
    // Collapsed on the same rule the adapter's own route is, so a template a
    // repository wrote across two YAML lines means the same sentence as one
    // written across one.
    template
        .replace("{path}", path)
        .split_whitespace()
        .collect::<Vec<_>>()
        .join(" ")
}

/// Every way one adapter can fall short of the canon behind it.
///
/// Returned rather than reported so the caller decides which finding kind the
/// shortfall is -- the same shortfall is a different fault for a skill wrapper
/// than for an agent adapter, and the rules that find it are the same.
struct Shortfall {
    /// Every way the adapter permits or declares something other than the
    /// canon does, in the order the rules are written.
    semantic: Vec<String>,
    /// The one way it can carry the wrong route.
    route: Option<String>,
}

fn shortfalls(
    read: &Read,
    adapter: &Adapter,
    name: &str,
    description: Option<&str>,
    route: &str,
    permits: &Permits<'_>,
) -> Shortfall {
    let mut faults = Vec::new();

    for (field, property) in &adapter.identity {
        let expected = match property {
            Identity::Name => Some(name.to_string()),
            Identity::Description => description.map(str::to_string),
        };
        if read.scalar(field) != expected {
            let property = match property {
                Identity::Name => "name",
                Identity::Description => "description",
            };
            faults.push(format!("`{field}` is not the canonical {property}"));
        }
    }

    for (field, value) in &adapter.fixed {
        if read.scalar(field).as_deref() != Some(value.as_str()) {
            faults.push(format!("`{field}` is not `{value}`"));
        }
    }

    for field in &adapter.absent {
        if read.fields.contains_key(field) {
            faults.push(format!("`{field}` may not be declared here"));
        }
    }

    if adapter.closed {
        // Everything the contract itself is about. The translations belong here
        // as much as the identity does: a closed set that left them out would
        // report the very field a translation obliges the adapter to declare,
        // which is a rule no adapter could satisfy and no repository could use.
        let permitted: BTreeSet<&str> = adapter
            .identity
            .keys()
            .chain(adapter.fixed.keys())
            .chain(adapter.translations.iter().map(|rule| &rule.field))
            .map(String::as_str)
            .chain((adapter.route_field != BODY).then_some(adapter.route_field.as_str()))
            .collect();
        for field in read.fields.keys() {
            if !permitted.contains(field.as_str()) {
                faults.push(format!("`{field}` is beyond what a wrapper may declare"));
            }
        }
    }

    for translation in &adapter.translations {
        if !permits.triggers(translation) {
            continue;
        }
        if let Some(fault) = unmet(read, translation) {
            faults.push(fault);
        }
    }

    Shortfall {
        semantic: faults,
        route: (read.route(adapter) != route).then(|| {
            format!(
                "`{}` is not the canonical route to `{name}`",
                adapter.route_field
            )
        }),
    }
}

/// What the canonical document permits, as the translations ask about it.
struct Permits<'a> {
    capabilities: BTreeSet<&'a str>,
    denied: BTreeSet<&'a str>,
    constraints: BTreeSet<&'a str>,
}

impl<'a> Permits<'a> {
    /// What this canonical declaration permits, as the translations ask.
    fn of(declaration: &'a Declaration) -> Self {
        let (capabilities, denied, constraints) = declaration.semantics();
        Self {
            capabilities,
            denied,
            constraints,
        }
    }

    fn triggers(&self, translation: &Translation) -> bool {
        // `always` ignores the capability, which is why the schema lets it be
        // omitted there and refuses to let it be omitted anywhere else.
        let named = || translation.capability.as_deref().unwrap_or_default();
        match translation.when {
            When::Always => true,
            When::Requires => self.capabilities.contains(named()),
            When::Denies => self.denied.contains(named()),
            When::Constrains => self.constraints.contains(named()),
        }
    }
}

/// How this adapter fails one translation it triggered, if it does.
fn unmet(read: &Read, translation: &Translation) -> Option<String> {
    let field = &translation.field;
    let members = read.members(field);

    for wanted in &translation.members {
        if !members.contains(wanted) {
            return Some(format!("`{field}` does not grant `{wanted}`"));
        }
    }
    for refused in &translation.absent_members {
        if members.contains(refused) {
            return Some(format!(
                "`{field}` grants `{refused}`, which the canon denies"
            ));
        }
    }
    if let Some(prefix) = &translation.member_prefix
        && !members.iter().any(|member| member.starts_with(prefix))
    {
        return Some(format!(
            "`{field}` grants nothing beginning with `{prefix}`"
        ));
    }

    let entries = read.entries(field);
    for (key, value) in &translation.entries {
        if entries.get(key).map(String::as_str) != Some(value.as_str()) {
            return Some(format!("`{field}.{key}` is not `{value}`"));
        }
    }
    if let Some(prefix) = &translation.entry_prefix {
        let wanted = translation.entry_value.as_deref().unwrap_or_default();
        if !entries
            .iter()
            .any(|(key, value)| key.starts_with(prefix) && value == wanted)
        {
            return Some(format!(
                "`{field}` sets nothing beginning with `{prefix}` to `{wanted}`"
            ));
        }
    }

    None
}

// -- Digest -------------------------------------------------------------------

/// A digest over the canon, so a change to any of it is visible as one value.
///
/// Everything under a skill directory is included, not only its declaration: a
/// supporting resource is part of what the skill tells a harness to do, and a
/// digest that ignored it would say the contract was unchanged when it was not.
fn digest(contents: &BTreeMap<String, String>, config: &Config) -> String {
    let canonical = &config.harness_parity.canonical;
    let mut hasher = Sha256::new();

    let roots: Vec<String> = [canonical.skills_root.clone(), canonical.agents_root.clone()]
        .into_iter()
        .flatten()
        .map(|root| format!("{}/", root.trim_end_matches('/')))
        .collect();

    for (path, text) in contents {
        let canonical_file =
            path == &canonical.instruction || roots.iter().any(|root| path.starts_with(root));
        if !canonical_file {
            continue;
        }
        hasher.update(path.as_bytes());
        hasher.update([0]);
        hasher.update(text.as_bytes());
        hasher.update([0]);
    }

    hasher
        .finalize()
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect()
}
