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
/// The same two files under a name no Markdown scan would pick up.
/// A harness's own configuration file, in that vendor's format rather than in
/// Markdown, which is where a field-shaped instruction source would live.
pub const PROHIBITED_FIELD_JSON: &str = "harness-settings.json";
pub const PROHIBITED_FIELD_TOML: &str = "harness-settings.toml";
/// The one key of that file the repository declares off limits.
pub const PROHIBITED_FIELD: &str = "instructions";

/// Where this declaration keeps the settings file holding the prohibited key.
pub fn prohibited_field_file(declaration: &Declaration) -> &'static str {
    match declaration.prohibited_field_format {
        Some("toml") => PROHIBITED_FIELD_TOML,
        _ => PROHIBITED_FIELD_JSON,
    }
}

pub const INSTRUCTION_UNMARKED: &str = "root-instructions.txt";
pub const ADAPTER_UNMARKED: &str = "adapter-instructions.txt";

/// Where this declaration puts the canonical instruction body.
pub fn instruction(declaration: &Declaration) -> &'static str {
    if declaration.canon_is_not_markdown {
        INSTRUCTION_UNMARKED
    } else {
        INSTRUCTION
    }
}

/// Where this declaration puts the file permitted to route to it.
pub fn adapter(declaration: &Declaration) -> &'static str {
    if declaration.canon_is_not_markdown {
        ADAPTER_UNMARKED
    } else {
        ADAPTER
    }
}
pub const SKILLS_ROOT: &str = "canon/skills";
pub const AGENTS_ROOT: &str = "canon/agents";
pub const SKILL: &str = "tidy";
pub const SKILL_DESCRIPTION: &str = "Tidy the working tree.";
pub const AGENT: &str = "reviewer";
pub const AGENT_DESCRIPTION: &str = "Review changes.";

/// The sentence an adapter carries in place of the canonical prompt, with
/// `{path}` standing for the canonical document it routes to.
pub const AGENT_ROUTE: &str =
    "Read {path} completely and follow it as authoritative before acting.";
pub const SKILL_ROUTE: &str =
    "Read {path} completely, resolving every relative resource from that directory.";

/// How one harness expresses an adapter.
///
/// Three shapes rather than three copies of one, because the point of the
/// schema is that a harness naming its permissions one way and a harness naming
/// them another are two configurations. A fixture where all three agreed would
/// prove that nowhere.
#[derive(Clone, Copy, PartialEq, Eq)]
pub enum Shape {
    /// Permissions as a comma-separated scalar; route in the prose body.
    List,
    /// Permissions as a nested map of key to verdict; route in the prose body.
    Permissions,
    /// The whole adapter as TOML; route in a named field.
    Toml,
}

pub fn shape_of(harness: &str) -> Shape {
    match harness {
        "beta" => Shape::Permissions,
        "gamma" => Shape::Toml,
        _ => Shape::List,
    }
}

pub fn capability_file(harness: &str, format: &str) -> String {
    format!("adapters/{harness}/capabilities.{format}")
}

/// Where this harness keeps the adapter for a canonical agent named `name`.
pub fn agent_adapter_pattern(harness: &str) -> String {
    match shape_of(harness) {
        Shape::Toml => format!("adapters/{harness}/agents/{{name}}.toml"),
        _ => format!("adapters/{harness}/agents/{{name}}.md"),
    }
}

/// Where this harness keeps the wrapper for a canonical skill named `name`.
///
/// A file per skill for two of the shapes and a directory per skill for the
/// third, because both are real and a pattern is what says either.
pub fn skill_wrapper_pattern(harness: &str) -> String {
    match shape_of(harness) {
        Shape::Toml => format!("adapters/{harness}/skills/{{name}}/SKILL.md"),
        _ => format!("adapters/{harness}/commands/{{name}}.md"),
    }
}

/// The directory this harness keeps its agent adapters in.
pub fn agent_adapter_directory(harness: &str) -> String {
    format!("adapters/{harness}/agents")
}

pub fn agent_adapter(harness: &str) -> String {
    agent_adapter_pattern(harness).replace("{name}", AGENT)
}

/// An adapter for an agent the canon does not declare. Named so it sorts
/// before the canonical agent's adapter, which is what lets a scenario tell a
/// sorted rendering from an unsorted one.
pub fn unexpected_agent_adapter(harness: &str) -> String {
    agent_adapter_pattern(harness).replace("{name}", "ghost")
}

pub fn undeclared_agent(harness: &str) -> String {
    match shape_of(harness) {
        Shape::Toml => "name = \"ghost\"\ndescription = \"Nobody declared this.\"\n".to_string(),
        _ => "---\nname: ghost\ndescription: Nobody declared this.\n---\n\nUnknown.\n".to_string(),
    }
}

pub fn skill_wrapper(harness: &str) -> String {
    skill_wrapper_pattern(harness).replace("{name}", SKILL)
}

/// A wrapper for a skill the canon does not declare.
pub fn unexpected_skill_wrapper(harness: &str) -> String {
    skill_wrapper_pattern(harness).replace("{name}", "ghost")
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

/// The names this repository writes an agent's three permission lists under,
/// and the field every canonical agent must carry outright.
///
/// Deliberately *not* the words RHINO once hard-coded. Every scenario that
/// reconciles an adapter now depends on the configuration having been read for
/// these names, so a validator that went back to choosing them itself would
/// find three empty lists and report the whole corpus clean.
pub const GRANTS: &str = "requires";
pub const DENIALS: &str = "denies";
pub const LIMITS: &str = "constraints";
pub const FIXED_FIELD: &str = "mode";
pub const FIXED_VALUE: &str = "subagent";

/// A second repository's spelling of the same three lists.
pub const ALTERNATE: [&str; 3] = ["may", "may-not", "always"];

/// The three names in force for this scenario.
pub fn declaration_names(declaration: &Declaration) -> [&str; 3] {
    match declaration.renamed_declaration {
        true => ALTERNATE,
        false => [GRANTS, DENIALS, LIMITS],
    }
}

fn agent_body_named(names: [&str; 3]) -> String {
    let [grants, denials, limits] = names;
    format!(
        "---\nname: {AGENT}\ndescription: {AGENT_DESCRIPTION}\n{FIXED_FIELD}: {FIXED_VALUE}\n{grants}:\n  - repository-read\n  - shell\n{denials}:\n  - repository-write\n{limits}:\n  - inline-result-only\n---\n\nReview the change and report inline.\n"
    )
}

fn agent_body() -> String {
    agent_body_named([GRANTS, DENIALS, LIMITS])
}

/// A canonical agent that omits the field its declaration shape fixes.
pub fn agent_without_its_fixed_field() -> String {
    agent_body().replace(&format!("{FIXED_FIELD}: {FIXED_VALUE}\n"), "")
}

pub fn agent_route() -> String {
    AGENT_ROUTE.replace("{path}", &canonical_agent())
}

pub fn skill_route() -> String {
    SKILL_ROUTE.replace("{path}", &canonical_skill())
}

/// The adapter a harness of this shape carries for the canonical agent.
///
/// A route and a translation of the canon's capabilities into this harness's
/// own permission vocabulary -- never a copy of the canonical prompt, which is
/// what every real harness actually does.
pub fn agent_adapter_body(harness: &str) -> String {
    let route = agent_route();
    match shape_of(harness) {
        Shape::List => format!(
            "---\nname: {AGENT}\ndescription: {AGENT_DESCRIPTION}\ntools: Read, Glob, Grep, ShellRun\n---\n\n{route}\n"
        ),
        Shape::Permissions => format!(
            "---\ndescription: {AGENT_DESCRIPTION}\nmode: subagent\npermission:\n  read: allow\n  glob: allow\n  grep: allow\n  shell-run: allow\n  edit: deny\n  report: deny\n---\n\n{route}\n"
        ),
        Shape::Toml => format!(
            "name = \"{AGENT}\"\ndescription = \"{AGENT_DESCRIPTION}\"\nsandbox = \"read-only\"\ninstructions = \"\"\"\n{route}\n\"\"\"\n"
        ),
    }
}

/// The wrapper a harness carries for the canonical skill: the skill's
/// description, the route, and nothing else.
pub fn wrapper_body() -> String {
    format!(
        "---\ndescription: {SKILL_DESCRIPTION}\n---\n\n{}\n",
        skill_route()
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

/// The same server, written as one command vector rather than a command and
/// its arguments -- which is how one real harness spells it.
pub fn capability_as_one_vector(format: &str) -> String {
    capability_declaration(format)
        .replace(
            "\"command\": \"toolrunner\",\n      \"args\": [\"serve\"]",
            "\"command\": [\"toolrunner\", \"serve\"]",
        )
        .replace(
            "command = \"toolrunner\"\nargs = [\"serve\"]",
            "command = [\"toolrunner\", \"serve\"]",
        )
}

/// The roster a declaration currently names.
///
/// The single answer to "which harnesses does this repository declare?".
/// `fixtures::render` writes it into `repo-config.yml` and this module builds
/// adapters for it, so a scenario cannot end up with a config that names one
/// roster and a tree that holds another.
pub fn roster(declaration: &Declaration) -> Vec<String> {
    if declaration.empty_roster {
        Vec::new()
    } else if declaration.roster.is_empty() {
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
        instruction(declaration).to_string(),
        "# Project rules\n\nEvery harness reaches this body.\n".to_string(),
    );
    if declaration.declares_instruction_adapter {
        files.insert(
            adapter(declaration).to_string(),
            import(instruction(declaration)),
        );
    }
    files.insert(canonical_skill(), skill_body());
    files.insert(
        canonical_agent(),
        agent_body_named(declaration_names(declaration)),
    );

    for harness in roster(declaration) {
        files.insert(agent_adapter(&harness), agent_adapter_body(&harness));
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

/// Whether a path is one the contract fixture owns.
///
/// Returned as a predicate rather than a list because the set depends on the
/// declaration -- the roster, the formats, the command directories -- and the
/// caller needs to ask about a path it already holds rather than enumerate
/// every shape the fixture could have taken.
pub fn contract_paths() -> impl Fn(&String) -> bool {
    |path: &String| {
        path == INSTRUCTION
            || path == ADAPTER
            || path == INSTRUCTION_UNMARKED
            || path == ADAPTER_UNMARKED
            || path.starts_with(&format!("{SKILLS_ROOT}/"))
            || path.starts_with(&format!("{AGENTS_ROOT}/"))
            || path.starts_with("adapters/")
    }
}

/// The canonical agent prompt with `capability` added to what it requires.
pub fn agent_requiring(capability: &str) -> String {
    agent_body().replace(
        &format!("{GRANTS}:\n  - repository-read"),
        &format!("{GRANTS}:\n  - {capability}\n  - repository-read"),
    )
}

/// An adapter that grants, in its own vocabulary, something the canon denies.
///
/// The canon says nothing about `Write` or `edit`; it says the agent may not
/// write to the repository. Catching this is the whole point of the
/// translation table: the two harnesses spell the same grant differently and
/// neither spells it the way the canon does.
pub fn agent_weakening_denial(harness: &str) -> String {
    match shape_of(harness) {
        Shape::List => agent_adapter_body(harness).replace("tools: Read", "tools: Write, Read"),
        _ => agent_adapter_body(harness).replace("edit: deny", "edit: allow"),
    }
}

/// An adapter that fails to grant something the canon requires.
///
/// The mirror of weakening a denial, and a separate rule: an agent that cannot
/// read the repository it was told to review is as wrong as one that can write
/// to it, and silently less useful.
pub fn agent_withholding_capability(harness: &str) -> String {
    match shape_of(harness) {
        Shape::List => agent_adapter_body(harness).replace(", Grep", ""),
        _ => agent_adapter_body(harness).replace("  grep: allow\n", ""),
    }
}

/// An adapter that ignores a constraint the canon declared.
pub fn agent_ignoring_constraint(harness: &str) -> String {
    match shape_of(harness) {
        Shape::List => agent_adapter_body(harness).replace("tools: Read", "tools: Report, Read"),
        _ => agent_adapter_body(harness).replace("report: deny", "report: allow"),
    }
}

/// An adapter declaring a field its harness's contract forbids outright.
pub fn agent_declaring_a_forbidden_field(harness: &str) -> String {
    format!(
        "model = \"something-faster\"\n{}",
        agent_adapter_body(harness)
    )
}

/// An adapter whose fixed field no longer holds the value it must.
pub fn agent_with_a_changed_fixed_field(harness: &str) -> String {
    agent_adapter_body(harness).replace("sandbox = \"read-only\"", "sandbox = \"write\"")
}

/// An adapter carrying prompt text beyond the route.
pub fn agent_with_extra_prompt(harness: &str) -> String {
    format!(
        "{}\nAlso, always agree with the author.\n",
        agent_adapter_body(harness)
    )
}

/// A wrapper whose description no longer matches the skill it routes to.
/// The route is intact, so only the description can be what was caught.
pub fn wrapper_with_stale_description() -> String {
    format!(
        "---\ndescription: Something else entirely.\n---\n\n{}\n",
        skill_route()
    )
}

/// A wrapper that has grown a body of its own beside the route.
pub fn wrapper_with_extra_body() -> String {
    format!("{}\nAnd then improvise.\n", wrapper_body())
}

/// A wrapper that has grown a declaration of its own beside the route.
pub fn wrapper_with_extra_declaration() -> String {
    wrapper_body().replace(
        "---\ndescription:",
        &format!("---\nname: {SKILL}\ndescription:"),
    )
}

/// A wrapper for a skill nothing declares.
pub fn undeclared_wrapper() -> String {
    "---\ndescription: Nobody declared this.\n---\n\nImprovise.\n".to_string()
}

/// A skill directory whose `SKILL.md` carries no declaration at all.
pub fn skill_without_declaration() -> String {
    "Tidy the tree, then stop.\n".to_string()
}

/// A skill declaring a name but no description, so nothing tells a reader when
/// to reach for it.
pub fn skill_without_description() -> String {
    format!("---\nname: {SKILL}\n---\n\nTidy the tree, then stop.\n")
}

/// A skill directory whose declared name is not the directory it lives in.
pub fn misnamed_skill() -> (String, String) {
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

/// A declaration whose name and command match but whose argument vector does
/// not, so only a comparison that reads past the command catches it.
pub fn divergent_capability_arguments(format: &str) -> String {
    capability_declaration(format).replace("\"serve\"", "\"serve\", \"--unsafe\"")
}

/// An agent declaring a constraint the repository never put in its vocabulary.
pub fn agent_constrained_by(constraint: &str) -> String {
    agent_body().replace(
        &format!("{LIMITS}:\n  - inline-result-only"),
        &format!("{LIMITS}:\n  - {constraint}\n  - inline-result-only"),
    )
}

/// A file outside the canon and outside every adapter, so a digest that covers
/// it is a digest that answers the wrong question.
pub fn uncounted_file() -> (String, String) {
    (
        "notes/scratch.md".to_string(),
        "# Scratch\n\nNot part of any contract.\n".to_string(),
    )
}

/// A canonical skill whose front matter opens and never closes.
///
/// Distinct from a file with no declaration at all: this one *looks* declared
/// until the reader reaches the end of it.
pub fn skill_without_terminator() -> String {
    format!("---\nname: {SKILL}\ndescription: {SKILL_DESCRIPTION}\n\nTidy the tree.\n")
}

/// A document with a body and no declaration in front of it.
pub fn document_without_declaration() -> String {
    "Prose, and nothing declaring what this is.\n".to_string()
}

/// The required capability declared inside a list of server groups rather than
/// a single map, which is a shape a vendor may legitimately use.
pub fn capability_inside_a_list() -> String {
    "{\n  \"servers\": [\n    { \"other\": { \"command\": \"elsewhere\", \"args\": [] } },\n    { \"toolserver\": { \"command\": \"toolrunner\", \"args\": [\"serve\"] } }\n  ]\n}\n".to_string()
}

/// A syntactically valid declaration that names no required capability.
pub fn capability_without_the_required_server() -> String {
    "{\n  \"mcpServers\": {\n    \"elsewhere\": {\n      \"command\": \"other\",\n      \"args\": []\n    }\n  }\n}\n".to_string()
}

/// This harness's adapter contract, as `repo-config.yml` declares it.
///
/// Written here beside the fixture that satisfies it, so the two cannot drift:
/// a translation the configuration declares and the adapter never satisfies
/// would present as a scenario failing for a reason nobody wrote down.
/// The declaration, optionally spoiled in one of the two ways a translation can
/// be written so that nothing would ever trigger it.
pub fn agent_adapter_declaration_with(
    harness: &str,
    unnamed: bool,
    undeclared: bool,
    closed: bool,
) -> String {
    let declared = agent_adapter_declaration(harness);
    if closed {
        return declared.replacen('{', "{closed: true, ", 1);
    }
    if unnamed {
        return declared.replace(
            "when: requires, capability: repository-read, ",
            "when: requires, ",
        );
    }
    if undeclared {
        return declared.replace("capability: repository-read", "capability: telepathy");
    }
    declared
}

pub fn agent_adapter_declaration(harness: &str) -> String {
    let path = agent_adapter_pattern(harness);
    match shape_of(harness) {
        Shape::List => format!(
            "{{path: \"{path}\", format: front-matter, route-field: body, \
             identity: {{name: name, description: description}}, translations: [\
             {{when: always, field: tools, members: [Read]}}, \
             {{when: requires, capability: repository-read, field: tools, members: [Glob, Grep]}}, \
             {{when: requires, capability: shell, field: tools, member-prefix: Shell}}, \
             {{when: denies, capability: repository-write, field: tools, absent-members: [Write, Edit]}}, \
             {{when: constrains, capability: inline-result-only, field: tools, absent-members: [Report]}}]}}"
        ),
        Shape::Permissions => format!(
            "{{path: \"{path}\", format: front-matter, route-field: body, \
             identity: {{description: description}}, fixed: {{mode: subagent}}, translations: [\
             {{when: always, field: permission, entries: {{read: allow}}}}, \
             {{when: requires, capability: repository-read, field: permission, entries: {{glob: allow, grep: allow}}}}, \
             {{when: requires, capability: shell, field: permission, entry-prefix: \"shell-\", entry-value: allow}}, \
             {{when: denies, capability: repository-write, field: permission, entries: {{edit: deny}}}}, \
             {{when: constrains, capability: inline-result-only, field: permission, entries: {{report: deny}}}}]}}"
        ),
        Shape::Toml => format!(
            "{{path: \"{path}\", format: toml, route-field: instructions, \
             identity: {{name: name, description: description}}, \
             fixed: {{sandbox: read-only}}, absent: [model]}}"
        ),
    }
}

/// This harness's skill-wrapper contract. A description, a route, and a closed
/// declaration -- nothing else may appear.
pub fn skill_adapter_declaration(harness: &str) -> String {
    format!(
        "{{path: \"{}\", format: front-matter, route-field: body, \
         identity: {{description: description}}, closed: true}}",
        skill_wrapper_pattern(harness)
    )
}

/// The same grants written as a YAML sequence rather than one line.
///
/// The two are the same permission set, and a harness that writes one is not a
/// harness whose adapters have to be read differently.
pub fn agent_adapter_listing_its_grants(harness: &str) -> String {
    agent_adapter_body(harness).replace(
        "tools: Read, Glob, Grep, ShellRun",
        "tools:\n  - Read\n  - Glob\n  - Grep\n  - ShellRun",
    )
}

/// An adapter that names an agent other than the one it stands for.
pub fn agent_with_a_wrong_name(harness: &str) -> String {
    agent_adapter_body(harness).replace(&format!("name = \"{AGENT}\""), "name = \"someone-else\"")
}

/// An adapter granting nothing that answers the shell capability.
pub fn agent_without_a_matching_grant(harness: &str) -> String {
    match shape_of(harness) {
        Shape::List => agent_adapter_body(harness).replace(", ShellRun", ""),
        _ => agent_adapter_body(harness).replace("  shell-run: allow\n", ""),
    }
}

/// The same adapter with its route wrapped the way a Markdown formatter wraps
/// prose: identical words, a line break where a space was.
pub fn agent_with_a_wrapped_route(harness: &str) -> String {
    let body = agent_adapter_body(harness);
    // Only past the front matter: a formatter wraps the prose, not the
    // declaration, and breaking a YAML line would be a different fault.
    let (declaration, prose) = body
        .rsplit_once("---\n")
        .expect("an adapter carries front matter");
    format!("{declaration}---\n{}", prose.replacen(' ', "\n", 4))
}

/// A canonical agent asking for less than every adapter already grants.
///
/// The contract is non-weakening, not equality: an adapter that permits more
/// than the canon asked for is still an adapter that permits everything the
/// canon asked for, and the harness's own defaults are not the canon's
/// business.
pub fn agent_requiring_less() -> String {
    agent_body().replace("  - repository-read\n  - shell\n", "  - repository-read\n")
}
