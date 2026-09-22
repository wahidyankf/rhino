//! The command-line interface conformance runner.
//!
//! The manifest in `specs/fixtures/cli-conformance/assertions.json` is portable:
//! it states each obligation and the observation that satisfies it, in prose,
//! without naming a tool. A probe here binds one assertion to a concrete
//! invocation of this executable. That split is deliberate — the obligation is
//! shared, the way to provoke it is not.
//!
//! Three outcomes, not two. `Unmeasured` exists because an assertion this runner
//! cannot provoke must not be reported as satisfied: a runner that silently
//! passes what it never exercised is worse than one that admits the gap.
//!
//! This runner reads the exit signal directly rather than through the end-to-end
//! driver, which reports `2` for a process that died by signal. That collapse is
//! the exact substitution the contract forbids, so observing `130` and `141`
//! requires going around it.
#![forbid(unsafe_code)]

use serde_json::Value;
use std::io::Read;
use std::os::unix::process::ExitStatusExt;
use std::process::{Command, Stdio};

/// What this executable declares about itself, which decides the assertions
/// that apply to it. `prompts` is absent: no leaf asks a question.
const CAPABILITIES: &[&str] = &[
    "starts-child-processes",
    "reads-standard-input",
    "machine-readable-output",
    "colour-output",
    "reads-configuration",
    "has-subcommand-tree",
];

/// This executable is not a harness callback, so it claims no exempt class.
const CLASSES: &[&str] = &[];

/// Assertions this executable does not yet satisfy, with the reason each is
/// open. Phase 3 measures; Phase 4 closes them.
///
/// The list is asserted to be **exact**. A gap that gets fixed without being
/// removed here fails the test, and so does a gap that appears without being
/// added — which is the only way a known-gap list stays honest rather than
/// becoming the place failures go to be forgotten.
const KNOWN_GAPS: &[(&str, &str)] = &[
    (
        "cli.streams.requested-version-on-stdout",
        "there is no `--version` flag; a `version` leaf carries it instead",
    ),
    (
        "cli.args.double-dash-ends-options",
        "`--` is not recognized as the end-of-options delimiter",
    ),
    (
        "cli.terminal.colour-is-tri-state",
        "`--no-color` exists in place of a tri-state `--color`",
    ),
    (
        "cli.exit.vocabulary-is-closed",
        "`gate run` returns 3 when a gate child cannot be launched, which is outside the \
         closed vocabulary and is published in the `Exit codes:` block",
    ),
    (
        "cli.exit.child-not-found-is-one-two-seven",
        "a gate child that does not exist reports 3 rather than 127, so a missing program \
         and an unexecutable one are the same status",
    ),
    (
        "cli.exit.closed-pipe-is-one-four-one",
        "a closed reader panics in the standard library's stdout handling and exits 101, \
         printing the panic message the contract forbids",
    ),
    (
        "cli.args.help-subcommand-when-a-tree-exists",
        "`help` is not a command; only the `-h` and `--help` flags reach the usage text",
    ),
];

enum Outcome {
    Passed,
    Failed(String),
    Unmeasured(&'static str),
}

/// One observation of the built executable at its process boundary.
struct Observed {
    code: Option<i32>,
    signal: Option<i32>,
    stdout: String,
    stderr: String,
}

impl Observed {
    /// The status a shell would report, so a signal death is `128+N` rather
    /// than a missing value that some default has to stand in for.
    fn status(&self) -> i32 {
        match (self.code, self.signal) {
            (Some(code), _) => code,
            (None, Some(signal)) => 128 + signal,
            (None, None) => -1,
        }
    }
}

fn invoke(arguments: &[&str], env: &[(&str, &str)]) -> Observed {
    let mut command = Command::new(env!("CARGO_BIN_EXE_rhino"));
    command
        .args(arguments)
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());
    for (name, value) in env {
        command.env(name, value);
    }
    let output = command.output().expect("the built executable is runnable");

    Observed {
        code: output.status.code(),
        signal: output.status.signal(),
        stdout: String::from_utf8_lossy(&output.stdout).into_owned(),
        stderr: String::from_utf8_lossy(&output.stderr).into_owned(),
    }
}

/// Provoke a closed pipe by closing the read end **before** reading anything,
/// so the child's first write meets a reader that is already gone.
///
/// The obvious probe — read one byte, then drop the pipe — does not work on a
/// command whose whole output fits in the kernel's pipe buffer: the write
/// succeeds, the process exits `0`, and the assertion goes unmeasured. Closing
/// first removes the race, because no amount of buffering saves a write to a
/// pipe with no reader.
fn invoke_with_closed_reader(arguments: &[&str]) -> Observed {
    let mut child = Command::new(env!("CARGO_BIN_EXE_rhino"))
        .args(arguments)
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .expect("the built executable is runnable");

    // Dropped immediately, closing the read end before the child writes.
    drop(child.stdout.take().expect("stdout is a pipe"));

    let mut stderr = String::new();
    if let Some(mut handle) = child.stderr.take() {
        let _ = handle.read_to_string(&mut stderr);
    }
    let status = child.wait().expect("the child runs to completion");

    Observed {
        code: status.code(),
        signal: status.signal(),
        stdout: String::new(),
        stderr,
    }
}

/// A throwaway repository whose single gate names a program that does not exist.
///
/// Without it the closed-vocabulary sweep passes by never reaching the one path
/// that leaves the vocabulary, which is a false pass rather than a measurement:
/// `--help` publishes `3  a gate child could not be started`, and `gate run`
/// returns it. An assertion that passes because the probe did not go looking is
/// the failure mode the three-outcome design exists to prevent, and it slipped
/// through anyway until the published status block was read against the sweep.
fn absent_child_repository() -> std::path::PathBuf {
    let root = std::env::temp_dir().join(format!(
        "rhino-conformance-absent-child-{}",
        std::process::id()
    ));
    let _ = std::fs::remove_dir_all(&root);
    std::fs::create_dir_all(&root).expect("the fixture root is creatable");
    std::fs::write(
        root.join("repo-config.yml"),
        "schema: rhino/repo-config/v2\nrepository: {}\ngates:\n  entries:\n    \
         - id: absent-child\n      type: check\n      command:\n        \
         executable: /no/such/program\n        args: []\n      run-on:\n        main: {}\n",
    )
    .expect("the fixture configuration is writable");
    root
}

fn expect_status(observed: &Observed, wanted: i32) -> Outcome {
    if observed.status() == wanted {
        Outcome::Passed
    } else {
        Outcome::Failed(format!(
            "expected exit {wanted}, observed {}",
            observed.status()
        ))
    }
}

fn expect_clean_stdout(observed: &Observed) -> Outcome {
    if observed.stdout.is_empty() {
        Outcome::Passed
    } else {
        Outcome::Failed(format!(
            "expected empty stdout, observed {} bytes beginning {:?}",
            observed.stdout.len(),
            observed.stdout.chars().take(60).collect::<String>()
        ))
    }
}

fn both(first: Outcome, second: Outcome) -> Outcome {
    match (first, second) {
        (Outcome::Failed(a), Outcome::Failed(b)) => Outcome::Failed(format!("{a}; {b}")),
        (Outcome::Failed(a), _) | (_, Outcome::Failed(a)) => Outcome::Failed(a),
        (Outcome::Unmeasured(reason), _) | (_, Outcome::Unmeasured(reason)) => {
            Outcome::Unmeasured(reason)
        }
        (Outcome::Passed, Outcome::Passed) => Outcome::Passed,
    }
}

/// Bind one assertion identifier to a probe. An identifier absent from this
/// table is reported as unbound rather than skipped.
fn probe(id: &str) -> Option<Outcome> {
    let outcome = match id {
        "cli.exit.affirmative-is-zero" => expect_status(&invoke(&["version"], &[]), 0),

        "cli.exit.usage-mistake-is-two" => {
            let observed = invoke(&["--no-such-flag"], &[]);
            both(expect_status(&observed, 2), expect_clean_stdout(&observed))
        }

        "cli.streams.usage-mistake-leaves-stdout-clean" => {
            let observed = invoke(&["--no-such-flag"], &[]);
            let diagnosed = if observed.stderr.is_empty() {
                Outcome::Failed("expected a diagnostic on stderr, observed none".to_string())
            } else {
                Outcome::Passed
            };
            both(expect_clean_stdout(&observed), diagnosed)
        }

        "cli.streams.requested-help-on-stdout" => {
            let observed = invoke(&["--help"], &[]);
            let placed = if observed.stdout.is_empty() {
                Outcome::Failed("expected usage text on stdout, observed none".to_string())
            } else if !observed.stderr.is_empty() {
                Outcome::Failed(format!(
                    "expected empty stderr, observed {} bytes",
                    observed.stderr.len()
                ))
            } else {
                Outcome::Passed
            };
            both(expect_status(&observed, 0), placed)
        }

        "cli.streams.requested-version-on-stdout" => {
            let observed = invoke(&["--version"], &[]);
            let placed = if observed.stdout.is_empty() {
                Outcome::Failed("expected a version line on stdout, observed none".to_string())
            } else {
                Outcome::Passed
            };
            both(expect_status(&observed, 0), placed)
        }

        "cli.exit.every-status-is-published" => {
            let observed = invoke(&["--help"], &[]);
            if observed.stdout.contains("Exit codes:") {
                Outcome::Passed
            } else {
                Outcome::Failed("`--help` carries no `Exit codes:` block".to_string())
            }
        }

        "cli.exit.closed-pipe-is-one-four-one" => {
            let observed = invoke_with_closed_reader(&["--help"]);
            if observed.status() == 141 && observed.stderr.is_empty() {
                Outcome::Passed
            } else {
                Outcome::Failed(format!(
                    "expected exit 141 and a silent stderr, observed exit {} and {:?}",
                    observed.status(),
                    observed.stderr.lines().next().unwrap_or_default()
                ))
            }
        }

        "cli.terminal.no-escapes-in-a-pipe" => {
            let observed = invoke(&["--help"], &[]);
            if observed.stdout.contains('\u{1b}') {
                Outcome::Failed("escape bytes reached a piped stdout".to_string())
            } else {
                Outcome::Passed
            }
        }

        "cli.terminal.no-color-set-and-non-empty-suppresses"
        | "cli.terminal.no-color-empty-does-not-suppress"
        | "cli.terminal.dumb-term-suppresses"
        | "cli.terminal.one-switch-for-every-escape" => Outcome::Unmeasured(
            "the assertion is about a terminal, and this runner pipes stdout; it needs a pty",
        ),

        "cli.terminal.colour-is-tri-state" => {
            let observed = invoke(&["--color", "never", "version"], &[]);
            if observed.status() == 0 {
                Outcome::Passed
            } else {
                Outcome::Failed(format!(
                    "`--color never` was not accepted: exit {}",
                    observed.status()
                ))
            }
        }

        "cli.args.double-dash-ends-options" => {
            let observed = invoke(&["--", "version"], &[]);
            if observed.status() == 0 {
                Outcome::Passed
            } else {
                Outcome::Failed(format!(
                    "`--` before an operand was not accepted: exit {}",
                    observed.status()
                ))
            }
        }

        // The stream matters as much as the status here. A bare invocation that
        // exits 2 but writes its help to stdout has put a diagnostic where a
        // caller reading the payload will find it.
        "cli.args.bare-invocation-is-a-usage-mistake" => {
            let observed = invoke(&[], &[]);
            let placed = if observed.stdout.is_empty() && !observed.stderr.is_empty() {
                Outcome::Passed
            } else {
                Outcome::Failed(format!(
                    "expected the diagnostic on stderr and nothing on stdout, observed {} and {} bytes",
                    observed.stdout.len(),
                    observed.stderr.len()
                ))
            };
            both(expect_status(&observed, 2), placed)
        }

        "cli.args.help-subcommand-when-a-tree-exists" => {
            let observed = invoke(&["help"], &[]);
            let placed = if observed.stdout.is_empty() {
                Outcome::Failed("expected usage text on stdout, observed none".to_string())
            } else {
                Outcome::Passed
            };
            both(expect_status(&observed, 0), placed)
        }

        "cli.args.short-help-on-every-subcommand" => {
            let observed = invoke(&["version", "-h"], &[]);
            let placed = if observed.stdout.is_empty() {
                Outcome::Failed("expected subcommand usage on stdout, observed none".to_string())
            } else {
                Outcome::Passed
            };
            both(expect_status(&observed, 0), placed)
        }

        "cli.streams.payload-on-stdout" => {
            let observed = invoke(&["gate", "list"], &[]);
            if observed.status() == 0 && !observed.stdout.is_empty() {
                Outcome::Passed
            } else {
                Outcome::Failed(format!(
                    "expected a payload on stdout at exit 0, observed exit {} and {} bytes",
                    observed.status(),
                    observed.stdout.len()
                ))
            }
        }

        "cli.streams.diagnostics-on-stderr" => {
            let observed = invoke(&["--no-such-flag"], &[]);
            if !observed.stderr.is_empty() && observed.stdout.is_empty() {
                Outcome::Passed
            } else {
                Outcome::Failed(format!(
                    "expected the diagnostic on stderr alone, observed {} on stdout and {} on stderr",
                    observed.stdout.len(),
                    observed.stderr.len()
                ))
            }
        }

        "cli.streams.help-suppresses-normal-function" => {
            let observed = invoke(&["--help", "gate", "list"], &[]);
            if observed.status() != 0 {
                Outcome::Failed(format!(
                    "expected exit 0 when help is requested alongside a command, observed {}",
                    observed.status()
                ))
            } else if observed.stdout.contains("[gate]") {
                Outcome::Failed("the command ran instead of only printing help".to_string())
            } else {
                Outcome::Passed
            }
        }

        "cli.output.mode-is-explicit" => {
            // stdout is a pipe here and no output flag was given.
            let observed = invoke(&["gate", "list"], &[]);
            if observed.stdout.trim_start().starts_with('{') {
                Outcome::Failed(
                    "machine-readable output was emitted without an explicit flag".to_string(),
                )
            } else {
                Outcome::Passed
            }
        }

        "cli.output.carries-a-schema-version" => {
            let observed = invoke(&["gate", "list", "--output", "json"], &[]);
            if observed.stdout.contains("\"schemaVersion\"") {
                Outcome::Passed
            } else {
                Outcome::Failed(
                    "the machine-readable document carries no schemaVersion".to_string(),
                )
            }
        }

        "cli.output.machine-readable-is-escape-free" => {
            let observed = invoke(&["gate", "list", "--output", "json"], &[]);
            if observed.stdout.contains('\u{1b}') {
                Outcome::Failed("escape bytes reached machine-readable stdout".to_string())
            } else {
                Outcome::Passed
            }
        }

        "cli.args.option-and-value-may-be-separate" => {
            let observed = invoke(&["--output", "json", "version"], &[]);
            if observed.status() == 0 {
                Outcome::Passed
            } else {
                Outcome::Failed(format!(
                    "an option and its value as separate arguments were refused: exit {}",
                    observed.status()
                ))
            }
        }

        "cli.diagnostics.name-the-tool-first" => {
            let observed = invoke(&["--no-such-flag"], &[]);
            if observed.stderr.starts_with("rhino: ") || observed.stderr.starts_with("rhino:") {
                Outcome::Passed
            } else {
                Outcome::Failed(format!(
                    "the diagnostic does not begin with the tool name: {:?}",
                    observed.stderr.chars().take(40).collect::<String>()
                ))
            }
        }

        "cli.stdin.not-read-unless-selected" => {
            // stdin is /dev/null here; a leaf that insisted on reading it would
            // still have to terminate, so the observation is that it does.
            let observed = invoke(&["gate", "list"], &[]);
            if observed.status() == 0 {
                Outcome::Passed
            } else {
                Outcome::Failed(format!(
                    "a run selecting no standard input did not complete: exit {}",
                    observed.status()
                ))
            }
        }

        "cli.exit.vocabulary-is-closed" => {
            // Every status this runner can provoke, checked against the set.
            let root = absent_child_repository();
            let gate_run: Vec<String> = ["gate", "run", "--surface", "main", "--root"]
                .iter()
                .map(|part| (*part).to_string())
                .chain(std::iter::once(root.to_string_lossy().into_owned()))
                .collect();
            let borrowed: Vec<&str> = gate_run.iter().map(String::as_str).collect();
            let probes: &[&[&str]] = &[
                &["version"],
                &["gate", "list"],
                &["--no-such-flag"],
                &["--help"],
                &["no-such-command"],
                &borrowed,
            ];
            let allowed = [0, 1, 2, 124, 125, 126, 127];
            let mut outside = Vec::new();
            for arguments in probes {
                let status = invoke(arguments, &[]).status();
                if !allowed.contains(&status) && status < 128 {
                    outside.push(format!("`{}` exits {status}", arguments.join(" ")));
                }
            }
            let _ = std::fs::remove_dir_all(&root);
            if outside.is_empty() {
                Outcome::Passed
            } else {
                Outcome::Failed(outside.join("; "))
            }
        }

        "cli.exit.child-not-found-is-one-two-seven" => {
            let root = absent_child_repository();
            let observed = invoke(
                &[
                    "gate",
                    "run",
                    "--root",
                    &root.to_string_lossy(),
                    "--surface",
                    "main",
                ],
                &[],
            );
            let _ = std::fs::remove_dir_all(&root);
            expect_status(&observed, 127)
        }

        "cli.exit.internal-crash-is-two" | "cli.exit.crash-trace-behind-a-switch" => {
            Outcome::Unmeasured("no fault-injection point exists at the process boundary")
        }

        "cli.exit.interrupt-is-one-three-zero" => Outcome::Unmeasured(
            "the executable exits faster than a delivered signal can be observed reliably",
        ),

        _ => return None,
    };

    Some(outcome)
}

fn applies(assertion: &Value) -> bool {
    let applies_to = &assertion["applies_to"];

    if let Some(required) = applies_to["requires_capabilities"].as_array() {
        if !required
            .iter()
            .all(|c| CAPABILITIES.contains(&c.as_str().unwrap_or_default()))
        {
            return false;
        }
    }
    if let Some(required) = applies_to["requires_classes"].as_array() {
        if !required
            .iter()
            .all(|c| CLASSES.contains(&c.as_str().unwrap_or_default()))
        {
            return false;
        }
    }
    if let Some(excluded) = applies_to["excludes_classes"].as_array() {
        if excluded
            .iter()
            .any(|c| CLASSES.contains(&c.as_str().unwrap_or_default()))
        {
            return false;
        }
    }
    true
}

#[test]
fn the_interface_contract_holds_at_the_process_boundary() {
    let path = concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/specs/fixtures/cli-conformance/assertions.json"
    );
    let text = std::fs::read_to_string(path).expect("the conformance manifest is readable");
    let manifest: Value = serde_json::from_str(&text).expect("the manifest is valid JSON");
    let assertions = manifest["assertions"]
        .as_array()
        .expect("the manifest carries an assertions array");
    assert!(!assertions.is_empty(), "the manifest is empty");

    let mut passed = Vec::new();
    let mut failed = Vec::new();
    let mut unmeasured = Vec::new();
    let mut unbound = Vec::new();
    let mut skipped_unverified = 0usize;
    let mut skipped_inapplicable = 0usize;

    for assertion in assertions {
        let id = assertion["id"].as_str().expect("every assertion has an id");

        // The corpus's one hard rule: nothing but `verified` may gate.
        if assertion["status"].as_str() != Some("verified") {
            skipped_unverified += 1;
            continue;
        }
        if !applies(assertion) {
            skipped_inapplicable += 1;
            continue;
        }

        match probe(id) {
            Some(Outcome::Passed) => passed.push(id.to_string()),
            Some(Outcome::Failed(reason)) => failed.push(format!("{id}: {reason}")),
            Some(Outcome::Unmeasured(reason)) => unmeasured.push(format!("{id}: {reason}")),
            None => unbound.push(id.to_string()),
        }
    }

    // Printed on every run, not only on failure: the useful question during a
    // port is how much of the contract is actually being exercised.
    println!(
        "cli-conformance: {} pass, {} fail, {} unmeasured, {} unbound; \
         skipped {} unverified and {} inapplicable",
        passed.len(),
        failed.len(),
        unmeasured.len(),
        unbound.len(),
        skipped_unverified,
        skipped_inapplicable
    );
    for entry in &unmeasured {
        println!("  unmeasured  {entry}");
    }
    for entry in &unbound {
        println!("  unbound     {entry}");
    }

    let failed_ids: std::collections::BTreeSet<&str> = failed
        .iter()
        .map(|entry| entry.split(':').next().unwrap_or_default())
        .collect();
    let known_ids: std::collections::BTreeSet<&str> =
        KNOWN_GAPS.iter().map(|(id, _)| *id).collect();

    let unexpected: Vec<&&str> = failed_ids.difference(&known_ids).collect();
    let fixed: Vec<&&str> = known_ids.difference(&failed_ids).collect();

    assert!(
        unexpected.is_empty(),
        "{} assertion(s) fail that are not recorded as known gaps:\n{}",
        unexpected.len(),
        failed
            .iter()
            .filter(|entry| {
                let id = entry.split(':').next().unwrap_or_default();
                !known_ids.contains(id)
            })
            .cloned()
            .collect::<Vec<_>>()
            .join("\n")
    );

    assert!(
        fixed.is_empty(),
        "{} known gap(s) now pass and must be removed from KNOWN_GAPS:\n  {}",
        fixed.len(),
        fixed
            .iter()
            .map(|id| (**id).to_string())
            .collect::<Vec<_>>()
            .join("\n  ")
    );

    for (id, reason) in KNOWN_GAPS {
        println!("  known gap   {id}: {reason}");
    }
}
