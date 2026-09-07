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

use crate::config::{Capability, CapabilityFormat, Config, Harness, RequiredMcp};
use crate::markdown;
use crate::report::{Finding, Report};
use crate::runtime::{Tree, TreeError};
use crate::scan::{self, Scope};
use serde::Deserialize;
use serde_json::Value;
use sha2::{Digest, Sha256};
use std::collections::{BTreeMap, BTreeSet};

/// The one file a skill bundle must carry.
const SKILL_FILE: &str = "SKILL.md";

/// What a declaration front matter can say.
///
/// Every field is optional here and required by the rules below, so a file
/// missing one is reported as an invalid declaration rather than failing to
/// parse into a message nobody wrote.
#[derive(Debug, Default, Clone, PartialEq, Eq, Deserialize)]
struct Declaration {
    name: Option<String>,
    description: Option<String>,
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
    skills: BTreeMap<String, Declaration>,
    agents: BTreeMap<String, Document>,
    required: Option<&'a RequiredMcp>,
}

/// A file split into its declaration and the prompt beneath it.
struct Document {
    declaration: Declaration,
    body: String,
}

fn read_document(text: &str) -> Option<Document> {
    let mut lines = text.lines();
    if lines.next()?.trim() != "---" {
        return None;
    }
    let mut front = String::new();
    for line in lines.by_ref() {
        if line.trim() == "---" {
            let declaration: Declaration = yaml_serde::from_str(&front).ok()?;
            return Some(Document {
                declaration,
                body: lines.collect::<Vec<_>>().join("\n").trim().to_string(),
            });
        }
        front.push_str(line);
        front.push('\n');
    }
    None
}

pub fn validate(tree: &dyn Tree, config: &Config, scope: &Scope) -> Report {
    let parity = &config.harness_parity;
    let files = scan::files(tree, config);

    let mut report = Report::new("harness-parity", "harness");
    let mut refusal: Option<String> = None;

    // Read every file once. A validator that read the same adapter twice could
    // report two different things about it.
    let mut contents: BTreeMap<String, String> = BTreeMap::new();
    for path in &files {
        match tree.read(path) {
            Ok(text) => {
                contents.insert(path.clone(), text);
            }
            Err(TreeError::NotFound) => {}
            // Alone among the walks, this one reads every file rather than
            // every file of a declared kind, so it is the only one that meets
            // a repository's images and archives. None of them can be an
            // instruction body, a skill, an agent, or a capability
            // declaration, and refusing the run over a logo would put parity
            // out of reach of any repository that ships one.
            Err(TreeError::NotText) => {}
            Err(TreeError::Unreadable(reason)) => {
                refusal.get_or_insert(format!("{path}: {reason}"));
            }
        }
    }
    if let Some(reason) = refusal {
        return Report::refused("harness-parity", reason);
    }

    instructions(&contents, config, &mut report);
    let canon = Canon {
        skills_root: parity.canonical.skills_root.as_deref(),
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

fn instructions(contents: &BTreeMap<String, String>, config: &Config, report: &mut Report) {
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

    let Ok(prohibited) = scan::glob_set(
        "harness-parity.prohibited-instruction-sources",
        &config.harness_parity.prohibited_instruction_sources,
    ) else {
        return;
    };

    for (path, text) in contents {
        if path == instruction || Some(path.as_str()) == adapter {
            continue;
        }
        // Two ways to be a competing always-on instruction: to be named like
        // one, or to route to the canonical body without being the file
        // permitted to.
        //
        // In Markdown the route only counts where it is prose. A page that
        // shows the import inside a fenced example or a code span is
        // documenting the adapter rather than being one, and a validator that
        // could not tell those apart could not be documented without tripping
        // itself.
        //
        // A document that ends inside a fence gets no such benefit. Its
        // remainder is an example only by the accident of a missing closer, and
        // granting the exemption there would make the check evadable by one
        // stray line.
        let quoting = scan::is_markdown(path) && !markdown::ends_inside_a_fence(text);
        let imports = if quoting {
            markdown::prose_lines(text)
                .iter()
                .any(|(_, line)| markdown::without_code_spans(line).contains(&route))
        } else {
            text.contains(&route)
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

        let Some(document) = read_document(text) else {
            report.found(Finding::new(
                "invalid-skill",
                path,
                "invalid-skill: a skill needs a declaration naming and describing it",
            ));
            continue;
        };
        let (Some(name), Some(_)) = (
            document.declaration.name.clone(),
            document.declaration.description.clone(),
        ) else {
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
        found.insert(name, document.declaration);
    }

    found
}

// -- Agents -------------------------------------------------------------------

fn agents(
    contents: &BTreeMap<String, String>,
    root: Option<&str>,
    config: &Config,
    report: &mut Report,
) -> BTreeMap<String, Document> {
    let mut found: BTreeMap<String, Document> = BTreeMap::new();
    let Some(root) = root else {
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
        let Some(name) = rest.strip_suffix(".md") else {
            continue;
        };
        let Some(document) = read_document(text) else {
            report.found(Finding::new(
                "invalid-agent",
                path,
                "invalid-agent: an agent needs a declaration naming and describing it",
            ));
            continue;
        };

        // A name outside the declared vocabulary grants or denies nothing, so a
        // typo would silently weaken an agent rather than fail.
        for capability in document
            .declaration
            .capabilities
            .iter()
            .chain(&document.declaration.denied)
        {
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
        for constraint in &document.declaration.constraints {
            if !constraints.contains(constraint.as_str()) {
                report.found(Finding::new(
                    "unknown-capability",
                    path,
                    format!("unknown-capability: constraint `{constraint}` is outside the declared vocabulary"),
                ));
            }
        }

        found.insert(name.to_string(), document);
    }

    found
}

fn agent_adapters(
    contents: &BTreeMap<String, String>,
    harness: &Harness,
    canon: &Canon<'_>,
    report: &mut Report,
) {
    let canonical = &canon.agents;
    let prefix = format!("{}/", harness.agent_dir.trim_end_matches('/'));

    for (name, expected) in canonical {
        let path = format!("{prefix}{name}{}", harness.agent_extension);
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
        let Some(actual) = read_document(text) else {
            report.found(Finding::new(
                "invalid-agent",
                &path,
                "invalid-agent: an adapter needs the same declaration the canon carries",
            ));
            continue;
        };

        // Semantics first. An adapter that permits something the canon refused
        // is a different agent, not a differently worded one, and reporting it
        // as prose drift would understate it.
        if actual.declaration.semantics() != expected.declaration.semantics() {
            report.found(Finding::new(
                "agent-semantic-divergence",
                &path,
                "agent-semantic-divergence: the adapter grants, denies, or constrains differently from the canon",
            ));
        } else if actual.body != expected.body {
            report.found(Finding::new(
                "agent-prompt-divergence",
                &path,
                "agent-prompt-divergence: the adapter's prompt is not the canonical one",
            ));
        }
    }

    for path in contents.keys() {
        let Some(rest) = path.strip_prefix(&prefix) else {
            continue;
        };
        let Some(name) = rest.strip_suffix(&harness.agent_extension) else {
            continue;
        };
        if rest.contains('/') || canonical.contains_key(name) {
            continue;
        }
        report.found(Finding::new(
            "unexpected-agent-adapter",
            path,
            format!("unexpected-agent-adapter: `{name}` has no canonical agent behind it"),
        ));
    }
}

fn skill_wrappers(
    contents: &BTreeMap<String, String>,
    harness: &Harness,
    canon: &Canon<'_>,
    report: &mut Report,
) {
    // Only a harness that declares a command directory has wrappers to get
    // wrong. One that declares none is complete without them.
    let (Some(directory), Some(root)) = (harness.command_dir.as_deref(), canon.skills_root) else {
        return;
    };
    let skills = &canon.skills;
    let prefix = format!("{}/", directory.trim_end_matches('/'));

    for (name, canonical) in skills {
        let path = format!("{prefix}{name}.md");
        let route = format!("@{}/{name}/{SKILL_FILE}", root.trim_end_matches('/'));

        let Some(text) = contents.get(&path) else {
            report.found(Finding::new(
                "missing-skill-adapter",
                &path,
                format!(
                    "missing-skill-adapter: `{}` declares a command directory but has no wrapper for `{name}`",
                    harness.name
                ),
            ));
            continue;
        };
        let Some(wrapper) = read_document(text) else {
            report.found(Finding::new(
                "skill-content-divergence",
                &path,
                "skill-content-divergence: a wrapper needs the same declaration the canonical skill carries",
            ));
            continue;
        };

        // A wrapper is a route, not a second copy of the skill. It mirrors the
        // description a reader chooses by and contains the route and nothing
        // else -- anything more is a place for the two to drift apart.
        //
        // Three separate messages for three separate faults, because a reader
        // who is told only that a wrapper diverged still has to open both files
        // to find out how.
        let fault = if wrapper.declaration.name.as_deref() != Some(name.as_str()) {
            Some(format!(
                "the wrapper declares a name that is not the skill `{name}` it routes to"
            ))
        } else if wrapper.declaration.description != canonical.description {
            Some("the wrapper's description is not the canonical skill's".to_string())
        } else if wrapper.body.trim() != route {
            Some(format!("a wrapper may contain `{route}` and nothing else"))
        } else {
            None
        };
        if let Some(fault) = fault {
            report.found(Finding::new(
                "skill-content-divergence",
                &path,
                format!("skill-content-divergence: {fault}"),
            ));
        }
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

    let command = entry.get("command").and_then(Value::as_str);
    let arguments: Option<Vec<&str>> = entry
        .get("args")
        .and_then(Value::as_array)
        .map(|values| values.iter().filter_map(Value::as_str).collect());

    let expected: Vec<&str> = required.args.iter().map(String::as_str).collect();
    if command != Some(required.command.as_str()) || arguments.as_deref() != Some(&expected) {
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
