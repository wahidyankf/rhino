//! The scenario world and the driver contract every adapter implements.
//!
//! One vocabulary, three implementations. The unit adapter drives the library
//! against an in-memory tree, the integration adapter against a real temporary
//! directory, and the E2E adapter against the built executable through its
//! process boundary. A scenario's steps read the same regardless of which
//! adapter is running it -- which is what makes one sentence a claim about
//! three different boundaries rather than three different claims.

use std::collections::BTreeMap;

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
#[derive(Debug, Clone, Default)]
pub struct Declaration {
    /// Start from a complete, valid configuration rather than an empty one.
    /// `specs/behaviours/README.md` states why: a declaring `Given` is additive
    /// over a legal base, so a feature can declare only a palette.
    pub complete: bool,
    pub present: bool,
    pub schema: Option<String>,
    /// Dotted key to replacement scalar, applied after the base is built.
    pub overrides: BTreeMap<String, String>,
    /// Dotted keys removed from the base.
    pub omissions: Vec<String>,
    /// Extra top-level sections, which the contract says are ignored.
    pub extra_sections: Vec<String>,
    pub empty_roster: bool,
    pub roster: Vec<String>,
    pub canonical_skills_root: Option<String>,
    pub canonical_agents_root: Option<String>,
}

/// Everything a scenario accumulates between its steps.
#[derive(Default)]
pub struct World<D> {
    pub driver: D,
    pub declaration: Declaration,
    pub files: BTreeMap<String, String>,
    pub command_result: Option<CommandResult>,
    pub previous_command_result: Option<CommandResult>,
}

impl<D> World<D> {
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

/// What an adapter must be able to do. Anything a scenario can do to a
/// repository, or ask of one, appears here exactly once.
pub trait Driver {
    /// Materialise the declared configuration and files, then invoke RHINO with
    /// the given argument vector, returning what the process contract exposes.
    fn invoke(
        &self,
        declaration: &Declaration,
        files: &BTreeMap<String, String>,
        arguments: &[String],
    ) -> CommandResult;
}
