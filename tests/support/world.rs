//! The scenario world and the driver contract every adapter implements.
//!
//! One vocabulary, three implementations. The unit adapter drives the library
//! against an in-memory tree, the integration adapter against a real temporary
//! directory, and the E2E adapter against the built executable through its
//! process boundary. A scenario's steps read the same regardless of which
//! adapter is running it -- which is what makes one sentence a claim about
//! three different boundaries rather than three different claims.

use std::collections::{BTreeMap, BTreeSet};

/// What the process contract exposes, and all the E2E adapter may observe.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct CommandResult {
    pub exit_code: u8,
    pub stdout: String,
    pub stderr: String,
}

/// One invocation, and what the repository looked like on either side of it.
///
/// The mutation list exists because "inspection is read-only" is a claim about
/// what the run *did not* do, and only the adapter that materialised the
/// repository can observe that. A `Then` reading a copy the test itself made
/// would be comparing the harness with the harness and could never fail.
#[derive(Debug, Clone, Default)]
pub struct Run {
    pub result: CommandResult,
    /// Paths the repository gained, lost, or whose contents changed while the
    /// command ran, as observed at the adapter's own boundary.
    pub mutations: Vec<String>,
}

/// What a repository looks like to an adapter: every path it can see, and the
/// content behind it where that is readable.
pub type Observation = BTreeMap<String, Option<String>>;

/// The immediate entries of the process working directory.
///
/// Included because `rhino::execute` runs *in this process* at the unit and
/// integration boundaries, so a write to a hard-coded relative path lands here
/// rather than in the sandbox. A backstop rather than a proof: whichever
/// scenario runs first would create such a file, and by the time the read-only
/// scenario looks it is present on both sides. The deterministic proof is at
/// E2E, where the spawned process's working directory *is* the repository
/// under inspection and every write it makes lands inside the observation.
pub fn working_directory() -> Observation {
    let Ok(entries) = std::fs::read_dir(".") else {
        return Observation::new();
    };
    entries
        .flatten()
        .map(|entry| {
            (
                format!("<cwd>/{}", entry.file_name().to_string_lossy()),
                None,
            )
        })
        .collect()
}

/// The paths on which two observations differ, as readable sentences.
pub fn differences(before: &Observation, after: &Observation) -> Vec<String> {
    let mut found = Vec::new();
    for (path, content) in before {
        match after.get(path) {
            None => found.push(format!("{path} was removed")),
            Some(now) if now != content => found.push(format!("{path} was modified")),
            Some(_) => {}
        }
    }
    for path in after.keys() {
        if !before.contains_key(path) {
            found.push(format!("{path} was created"));
        }
    }
    found.sort();
    found
}

/// The declaration a scenario builds up before anything runs.
///
/// The adapters turn this into a `repo-config.yml`. No adapter invents a value
/// the scenario did not state, because that is precisely the defect the whole
/// extraction exists to remove. Overrides are recorded as dotted paths so a
/// scenario can introduce a malformed key without the world needing a field for
/// it.
#[derive(Debug, Clone)]
pub struct Declaration {
    /// Whether the repository has *no* configuration file.
    ///
    /// Stated as the absence rather than the presence so the default is a
    /// complete, legal configuration. `specs/behaviours/README.md` says a
    /// declaring `Given` is additive over that base, and a field that had to be
    /// set by every declaring arm would eventually be forgotten by one -- which
    /// presents as the validator refusing for a reason the scenario never
    /// mentioned.
    pub absent: bool,
    pub schema: Option<String>,
    /// Dotted key to replacement scalar, applied after the base is built.
    pub overrides: BTreeMap<String, String>,
    /// Dotted keys removed from the base.
    pub omissions: Vec<String>,
    /// Extra top-level sections, which the contract says are ignored.
    pub extra_sections: Vec<String>,
    /// Globs a scenario declares as non-sources for internal links.
    pub excluded_sources: Vec<String>,
    /// Directory names a scenario declares as excluded from every scan.
    pub excluded_directories: Vec<String>,
    pub empty_roster: bool,
    /// Whether the configuration declares a required MCP server even though no
    /// harness would reconcile it. A non-empty roster declares one anyway, so
    /// this is only how a scenario states the degenerate combination.
    pub declares_required_mcp: bool,
    /// Whether the configuration omits the required capability server. Paired
    /// with the flag below rather than folded into it, because the schema's
    /// rule is that the two travel together and a scenario has to be able to
    /// separate them to say so.
    pub omit_required_server: bool,
    /// Whether the configuration omits every harness's capability declaration.
    pub omit_capability_files: bool,
    /// Whether the configuration omits the canonical agents root, and whether
    /// it omits every harness's contract for expressing it. Separate flags for
    /// the same reason as the pair above: the rule is that they travel
    /// together, and a scenario has to be able to part them to say so.
    pub omit_agents_root: bool,
    pub omit_agent_adapters: bool,
    pub omit_skills_root: bool,
    /// A translation naming nothing, or naming something the vocabulary never
    /// declared -- a permission rule that could never fire.
    pub unnamed_translation: bool,
    pub undeclared_translation: bool,
    /// Whether the canonical agents write their three permission lists under a
    /// second repository's names, with the configuration renamed to match.
    /// Both spellings must reconcile, which is the claim that the names are the
    /// repository's rather than the validator's.
    pub renamed_declaration: bool,
    /// Whether the configuration omits the shape that says which field holds
    /// which permission.
    pub omit_declaration_shape: bool,
    /// Whether the shape stays behind when the canonical agents it describes
    /// are gone -- the half of the pairing a scenario cannot state any other
    /// way, since the shape is otherwise written alongside the root.
    pub keep_declaration_shape: bool,
    /// What this repository means by a word, when a scenario says so. `None`
    /// leaves the base rule in force.
    pub word_rule: Option<String>,
    /// Whether every harness closes its agent adapter's declaration -- which
    /// must not then accuse the adapter of the fields its own translations
    /// oblige it to carry.
    pub closed_agent_adapters: bool,
    /// Whether the configuration writes its route template with uneven
    /// spacing, as a repository that wrapped or re-indented the line would.
    pub padded_route_template: bool,
    pub roster: Vec<String>,
    /// Whether the configuration declares a file permitted to import the
    /// canonical instruction. Absent is legal, so the flag is a tri-state only
    /// in the sense that a scenario may leave it alone.
    pub declares_instruction_adapter: bool,
    /// Whether the canonical instruction body and its adapter carry a
    /// non-Markdown extension.
    ///
    /// Nothing in the contract says the canon has to be Markdown -- only that
    /// a *competing* source found by its content can only be Markdown -- so a
    /// repository is free to declare one that is not, and a fixture in which
    /// every canonical file happens to end in `.md` cannot tell the difference
    /// between a rule that reads the file it was told to and one that reads
    /// every Markdown file and no other.
    pub canon_is_not_markdown: bool,
    /// Whether the configuration names a harness configuration file and one
    /// of its fields as a place instructions may not live.
    ///
    /// A harness that keeps its always-on instructions in a JSON key rather
    /// than in a Markdown file is the same competing source wearing that
    /// vendor's syntax, and a repository has to be able to say so. The value
    /// is the declared format, because the same rule has to survive being
    /// written in whatever a harness's settings file happens to be.
    pub prohibited_field_format: Option<&'static str>,
    /// Harnesses that declare a command directory. Per-harness, because that is
    /// how the schema states it.
    pub command_directories: BTreeSet<String>,
    /// Harness to capability format, when a scenario overrides the default.
    pub capability_formats: BTreeMap<String, String>,
    pub canonical_skills_root: Option<String>,
    pub canonical_agents_root: Option<String>,
    /// Naming surfaces, in declaration order, each rendered as the flow mapping
    /// a repository would write. Held as text rather than as a typed surface so
    /// a scenario can state a glob the compiler would reject, which is one of
    /// the faults the corpus asserts on.
    pub naming_surfaces: Vec<String>,
    pub naming_exempt: Vec<String>,
    /// Front-matter surfaces, in declaration order, each rendered as the flow
    /// mapping a repository would write.
    pub frontmatter_surfaces: Vec<String>,
    /// Heading surfaces, and the two rules the section states once for all of
    /// them. `None` on either rule leaves the section undeclared.
    pub heading_surfaces: Vec<String>,
    pub heading_single_h1: Option<bool>,
    pub heading_max_jump: Option<usize>,
    /// Trees a scenario declares as requiring a README in every directory.
    pub readme_index_trees: Vec<String>,
    /// Globs a scenario declares as files an emoji may not appear in.
    pub emoji_prohibited: Vec<String>,
    /// Metadata surfaces, in declaration order, each rendered as the flow
    /// mapping a repository would write. Ordered because the last matching
    /// glob wins, which is how a workflow subtree carries a different schema
    /// from the governance tree it sits inside.
    pub metadata_surfaces: Vec<String>,
    /// Model-tier mappings stated one complete pair at a time, as a repository
    /// that means the mapping would write it.
    pub model_tier_pairs: Vec<(String, String, String, String)>,
    /// The model-tier section written literally. Held separately because the
    /// shapes worth refusing -- an empty map, a null tier, half a pair -- are
    /// exactly the ones a well-formed pair cannot express.
    pub model_tier_literal: Option<String>,
}

impl Default for Declaration {
    fn default() -> Self {
        Self {
            absent: false,
            schema: None,
            overrides: BTreeMap::new(),
            omissions: Vec::new(),
            extra_sections: Vec::new(),
            excluded_sources: Vec::new(),
            excluded_directories: Vec::new(),
            empty_roster: false,
            declares_required_mcp: false,
            omit_required_server: false,
            omit_capability_files: false,
            omit_agents_root: false,
            omit_agent_adapters: false,
            omit_skills_root: false,
            unnamed_translation: false,
            undeclared_translation: false,
            renamed_declaration: false,
            omit_declaration_shape: false,
            keep_declaration_shape: false,
            word_rule: None,
            closed_agent_adapters: false,
            padded_route_template: false,
            roster: Vec::new(),
            // A declared adapter is the ordinary case; the scenario that has
            // none says so.
            declares_instruction_adapter: true,
            canon_is_not_markdown: false,
            prohibited_field_format: None,
            command_directories: BTreeSet::new(),
            capability_formats: BTreeMap::new(),
            canonical_skills_root: None,
            canonical_agents_root: None,
            // Every section added after `v0.1` is optional, so the base
            // declares none of them: a scenario that needs one says so, and
            // every scenario that does not is a standing proof that a
            // configuration without it is complete.
            naming_surfaces: Vec::new(),
            naming_exempt: Vec::new(),
            frontmatter_surfaces: Vec::new(),
            heading_surfaces: Vec::new(),
            heading_single_h1: None,
            heading_max_jump: None,
            readme_index_trees: Vec::new(),
            emoji_prohibited: Vec::new(),
            metadata_surfaces: Vec::new(),
            model_tier_pairs: Vec::new(),
            model_tier_literal: None,
        }
    }
}

/// Everything a scenario accumulates between its steps.
#[derive(Default)]
pub struct World<D> {
    pub driver: D,
    pub declaration: Declaration,
    pub files: BTreeMap<String, String>,
    /// Paths the repository holds but cannot be read. Modelled explicitly
    /// rather than by permission bits so the unit adapter can express it too:
    /// failing closed on an unreadable file is behaviour, and behaviour is
    /// never proved only at the boundaries that happen to have a filesystem.
    pub unreadable: BTreeSet<String>,
    /// Paths the walk lists and the read can no longer find, which is what a
    /// file removed between the two looks like.
    pub vanished: BTreeSet<String>,
    /// Paths that open cleanly and hold bytes that are not text. A different
    /// fact from `unreadable`, and the difference is the behaviour: one is a
    /// file RHINO could not open and must fail closed on, the other is a file
    /// it opened and found no text in.
    pub binary: BTreeSet<String>,
    /// What a `--file -` selection should read. Held on the world rather than
    /// reached for from the process, so the same sentence means the same thing
    /// at a boundary that has no standard input to reach for.
    pub stdin: Option<String>,
    /// Paths that are filesystem links. No tree reports them, which is the
    /// point: following one can leave the repository.
    pub links: BTreeSet<String>,
    pub remembered_digest: Option<String>,
    pub command_result: Option<CommandResult>,
    pub previous_command_result: Option<CommandResult>,
    /// Every change any invocation in this scenario made to the repository,
    /// accumulated, so a scenario that runs twice proves both runs innocent.
    pub mutations: Vec<String>,
}

impl<D> World<D> {
    /// The repository as currently declared.
    pub fn repository(&self) -> Repository<'_> {
        Repository {
            declaration: &self.declaration,
            files: &self.files,
            unreadable: &self.unreadable,
            vanished: &self.vanished,
            binary: &self.binary,
            links: &self.links,
            stdin: self.stdin.as_deref(),
        }
    }

    pub fn result(&self) -> &CommandResult {
        self.command_result
            .as_ref()
            .expect("a Then observed a command result before any When produced one")
    }

    /// Record a run, keeping the previous result so a scenario that runs two
    /// inspections can compare them, and accumulating what each run changed.
    pub fn record(&mut self, run: Run) {
        self.previous_command_result = self.command_result.take();
        self.command_result = Some(run.result);
        self.mutations.extend(run.mutations);
    }
}

/// Everything an adapter needs to build the repository a scenario declared.
///
/// Borrowed rather than owned so a step can hand the whole repository over
/// without copying it, and so adding a new property of a repository is one
/// field here instead of one more parameter at every call site.
pub struct Repository<'a> {
    pub declaration: &'a Declaration,
    pub files: &'a BTreeMap<String, String>,
    pub unreadable: &'a BTreeSet<String>,
    pub vanished: &'a BTreeSet<String>,
    pub binary: &'a BTreeSet<String>,
    pub links: &'a BTreeSet<String>,
    pub stdin: Option<&'a str>,
}

/// What an adapter must be able to do. Anything a scenario can do to a
/// repository, or ask of one, appears here exactly once.
pub trait Driver {
    /// Materialise the declared repository, observe it, invoke RHINO with the
    /// given argument vector, then observe it again.
    ///
    /// The second observation is the adapter's own, taken at its own boundary,
    /// and is what makes read-only a testable property rather than a documented
    /// intention.
    fn invoke(&self, repository: &Repository<'_>, arguments: &[String]) -> Run;
}

/// The placeholder a scenario writes when it means "this repository".
pub const ROOT: &str = "{root}";
/// And when it means a root that is not there.
pub const MISSING_ROOT: &str = "{missing-root}";

/// Substitute the root placeholders with what this adapter's root actually is.
///
/// A scenario cannot write the path, because it differs per adapter and per
/// run. Substituting here rather than in the binding keeps `--root` one claim
/// asserted at three boundaries instead of three different claims.
pub fn with_root(arguments: &[String], root: &str, missing: &str) -> Vec<String> {
    arguments
        .iter()
        .map(|argument| match argument.as_str() {
            ROOT => root.to_string(),
            MISSING_ROOT => missing.to_string(),
            other => other.to_string(),
        })
        .collect()
}
