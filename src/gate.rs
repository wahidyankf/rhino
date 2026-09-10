//! Gate dispatch.
//!
//! RHINO does not decide what a gate does. It decides which gates a surface
//! selects, what order they run in, what each child is told, and when the
//! sequence stops. Everything a child is handed is stated by the runner rather
//! than inherited from the process it was started in, because a hook that
//! behaved one way under Git and another under a hosted runner would be a gate
//! nobody could trust.

use crate::Outcome;
use crate::config::v2::{Document, SURFACES};
use crate::runtime::{Launch, Launcher};

/// Exit `3`: a child that could not be started, or a broken protocol.
///
/// Distinct from `1` because the two oblige different answers. A gate that ran
/// and reported a finding says something about the repository; a gate that
/// never ran says nothing at all, and reporting the second as the first would
/// let a broken hook read as a caught violation.
const LAUNCH_FAILURE: u8 = 3;

/// Run every gate the surface selects.
pub fn dispatch(
    document: &Document,
    surface: &str,
    root: &str,
    forwarded: &[String],
    stdin: Option<&str>,
    launcher: &dyn Launcher,
) -> Outcome {
    // The whole configuration is validated before a child starts, which the
    // caller has already done by handing over a read document. What is left
    // here is the surface itself, and a surface outside the closed set is a
    // fault in the invocation rather than a surface nothing runs at.
    if !SURFACES.contains(&surface) {
        return Outcome::refused(format!(
            "rhino: `--surface {surface}` is not a surface this build dispatches; the closed set is {}\n",
            SURFACES.join(", ")
        ));
    }

    let mut report = String::new();
    for gate in document.gates.iter().filter(|gate| gate.runs_at(surface)) {
        let mut arguments = gate.run.clone();
        arguments.extend_from_slice(forwarded);

        let launched = launcher.launch(Launch {
            arguments: &arguments,
            directory: root,
            surface,
            stdin,
        });

        match launched {
            Err(reason) => {
                report.push_str(&format!("[gate] {} could not run\n", gate.id));
                return Outcome {
                    exit_code: LAUNCH_FAILURE,
                    stdout: report,
                    stderr: format!("rhino: gate `{}`: {}\n", gate.id, reason.0),
                };
            }
            // Only the identifier and a sanitized status are emitted. A child's
            // own streams are never repeated: a gate exists to look at content
            // that may not be published, and a runner that echoed what it found
            // would publish it on the way to saying it should not be.
            Ok(result) if result.code == 0 => {
                report.push_str(&format!("[gate] {} passed\n", gate.id));
            }
            Ok(_) => {
                report.push_str(&format!("[gate] {} failed\n", gate.id));
                // Stopping here is what keeps a later mutation from running
                // after an earlier check failed: the sequence is ordered, and
                // the first failure ends it.
                return Outcome {
                    exit_code: 1,
                    stdout: report,
                    stderr: format!("[gate] {} reported a finding at {surface}\n", gate.id),
                };
            }
        }
    }

    Outcome::clean(report)
}
