//! The recording launcher.
//!
//! The other implementation of the launcher port, and the reason a gate
//! scenario means the same thing at a boundary with no process to spawn. A
//! scenario states what each gate answers with; this supplies a child that
//! answers exactly that and writes down what it was handed.
//!
//! The end-to-end adapter deliberately does not use it. There the children are
//! real executables writing real records, because the claim that a vector
//! reaches a child unsplit is a claim about a process boundary and cannot be
//! proved by something standing in for one.

use crate::world::Repository;
use rhino::runtime::{
    Launch, LaunchError, Launched, Launcher, ModelResolver, ResolveError, ResolveLaunch,
};
use std::cell::RefCell;
use std::collections::{BTreeMap, BTreeSet};

pub struct Recorder {
    outcomes: Vec<(String, i32)>,
    unlaunchable: Vec<String>,
    model_outputs: BTreeMap<String, String>,
    unstartable_models: BTreeSet<String>,
    journal: RefCell<Vec<String>>,
    root: String,
}

impl Recorder {
    pub fn new(repository: &Repository<'_>, root: &str) -> Self {
        Self {
            outcomes: repository.gate_outcomes.to_vec(),
            unlaunchable: repository.unlaunchable_gates.iter().cloned().collect(),
            model_outputs: repository.model_outputs.clone(),
            unstartable_models: repository.unstartable_models.clone(),
            journal: RefCell::new(Vec::new()),
            root: root.to_string(),
        }
    }

    pub fn journal(self) -> Vec<String> {
        self.journal.into_inner()
    }
}

impl Launcher for Recorder {
    fn launch(&self, launch: Launch<'_>) -> Result<Launched, LaunchError> {
        let Some((program, arguments)) = launch.arguments.split_first() else {
            return Err(LaunchError::refused("the gate declares no command"));
        };
        let id = identifier(program);

        if self.unlaunchable.contains(&id) {
            return Err(LaunchError::refused(format!(
                "`{program}` could not be started"
            )));
        }

        // `{root}` rather than the path, because the path differs per adapter
        // and per run. Substituting here keeps one claim asserted at three
        // boundaries instead of three different claims.
        let directory = if launch.directory == self.root {
            crate::world::ROOT.to_string()
        } else {
            launch.directory.to_string()
        };
        self.journal.borrow_mut().push(
            [
                "start",
                &id,
                &arguments.join("|"),
                launch.surface,
                launch.stdin.unwrap_or_default(),
                &directory,
            ]
            .join("\t"),
        );
        // No child stream is carried back at all: the port answers with a code
        // and nothing else, so at this boundary a runner that repeated what a
        // child wrote is not a defect to test for but a sentence that cannot be
        // written. The claim is proved where it can fail -- at the process
        // boundary, against a child that really writes to a pipe.
        self.journal.borrow_mut().push(format!("stop\t{id}"));

        let code = self
            .outcomes
            .iter()
            .find(|(named, _)| *named == id)
            .map_or(0, |(_, code)| *code);
        Ok(Launched { code })
    }
}

/// The other implementation of the model-resolver port: a command that prints
/// exactly what the scenario stated, and records that it started.
impl ModelResolver for Recorder {
    fn resolve(&self, launch: ResolveLaunch<'_>) -> Result<String, ResolveError> {
        let Some(program) = launch.arguments.first() else {
            return Err(ResolveError(
                "the model command declares no executable".to_string(),
            ));
        };
        let id = identifier(program);
        if self.unstartable_models.contains(&id) {
            return Err(ResolveError(format!("`{program}` could not be started")));
        }
        let Some(output) = self.model_outputs.get(&id) else {
            return Err(ResolveError(format!("`{program}` could not be started")));
        };
        self.journal
            .borrow_mut()
            .push(format!("{}\t{id}", crate::binding::MODEL_START));
        Ok(output.clone())
    }
}

/// The gate a command belongs to, taken from the name of what it runs.
pub fn identifier(program: &str) -> String {
    program
        .rsplit('/')
        .next()
        .unwrap_or(program)
        .trim_end_matches(".sh")
        .to_string()
}
