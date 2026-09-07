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
    pub roster: Vec<String>,
    /// Whether the configuration declares a file permitted to import the
    /// canonical instruction. Absent is legal, so the flag is a tri-state only
    /// in the sense that a scenario may leave it alone.
    pub declares_instruction_adapter: bool,
    /// Harnesses that declare a command directory. Per-harness, because that is
    /// how the schema states it.
    pub command_directories: BTreeSet<String>,
    /// Harness to capability format, when a scenario overrides the default.
    pub capability_formats: BTreeMap<String, String>,
    pub canonical_skills_root: Option<String>,
    pub canonical_agents_root: Option<String>,
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
            roster: Vec::new(),
            // A declared adapter is the ordinary case; the scenario that has
            // none says so.
            declares_instruction_adapter: true,
            command_directories: BTreeSet::new(),
            capability_formats: BTreeMap::new(),
            canonical_skills_root: None,
            canonical_agents_root: None,
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
    /// Paths that are filesystem links. No tree reports them, which is the
    /// point: following one can leave the repository.
    pub links: BTreeSet<String>,
    /// A copy of the tree taken before an inspection, to prove the inspection
    /// changed nothing.
    pub snapshot: Option<BTreeMap<String, String>>,
    pub remembered_digest: Option<String>,
    pub command_result: Option<CommandResult>,
    pub previous_command_result: Option<CommandResult>,
}

impl<D> World<D> {
    /// The repository as currently declared.
    pub fn repository(&self) -> Repository<'_> {
        Repository {
            declaration: &self.declaration,
            files: &self.files,
            unreadable: &self.unreadable,
            links: &self.links,
        }
    }

    pub fn result(&self) -> &CommandResult {
        self.command_result
            .as_ref()
            .expect("a Then observed a command result before any When produced one")
    }

    /// Record a result, keeping the previous one so a scenario that runs two
    /// inspections can compare them.
    pub fn record(&mut self, result: CommandResult) {
        self.previous_command_result = self.command_result.take();
        self.command_result = Some(result);
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
    pub links: &'a BTreeSet<String>,
}

/// What an adapter must be able to do. Anything a scenario can do to a
/// repository, or ask of one, appears here exactly once.
pub trait Driver {
    /// Materialise the declared repository, then invoke RHINO with the given
    /// argument vector, returning what the process contract exposes.
    fn invoke(&self, repository: &Repository<'_>, arguments: &[String]) -> CommandResult;
}
