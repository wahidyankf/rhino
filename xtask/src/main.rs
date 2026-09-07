//! Repository automation. Kept in Rust rather than shell so the release path
//! has the same type checking and the same tests as the product.
//!
//! `test-quick` is what the pre-push hook runs, so it stays fast by
//! construction: it never invokes the integration or E2E adapters. Those are
//! slow enough to train a maintainer into bypassing the hook, and they belong
//! to the scheduled workflow instead.
#![forbid(unsafe_code)]

use std::process::{Command, ExitCode};

/// The two modules a unit test may not reach, as one regex.
///
/// `src/main.rs` is the process boundary -- arguments, standard input, the
/// working directory, the two output streams. `src/runtime/disk.rs` is the only
/// module that talks to `std::fs`. Both are proved by the integration and E2E
/// adapters running the whole corpus through them, which is a stronger claim
/// than a line count and the reason neither is in the denominator here.
const EXCLUDED_FROM_COVERAGE: &str = r"src/(main\.rs|runtime/disk\.rs)";

fn main() -> ExitCode {
    let task = std::env::args().nth(1);
    let result = match task.as_deref() {
        Some("test-quick") => test_quick(),
        other => {
            eprintln!("unknown task: {other:?}");
            eprintln!("tasks: test-quick");
            return ExitCode::from(2);
        }
    };
    match result {
        Ok(()) => ExitCode::SUCCESS,
        Err(message) => {
            eprintln!("{message}");
            ExitCode::FAILURE
        }
    }
}

fn run(program: &str, args: &[&str]) -> Result<(), String> {
    let status = Command::new(program)
        .args(args)
        .status()
        .map_err(|error| format!("failed to start {program}: {error}"))?;

    if status.success() {
        Ok(())
    } else {
        Err(format!("{program} {} failed", args.join(" ")))
    }
}

fn test_quick() -> Result<(), String> {
    run("cargo", &["fmt", "--all", "--check"])?;
    run(
        "cargo",
        &[
            "clippy",
            "--all-targets",
            "--all-features",
            "--",
            "-D",
            "warnings",
        ],
    )?;
    // The unit adapter runs the whole corpus in process against an in-memory
    // tree. It is the only executing adapter in this gate: integration and E2E
    // touch a real filesystem and a spawned process, and the gate contract puts
    // both in the scheduled workflow rather than in a hook.
    //
    // Measured in the same execution rather than in a second one, because two
    // runs can disagree and the number that gates has to be the number the
    // passing run produced. `main.rs` and the concrete filesystem tree are the
    // only exclusions; `README.md` says what they are and why a unit test may
    // not reach them.
    run(
        "cargo",
        &[
            "llvm-cov",
            "--test",
            "unit",
            "--fail-under-lines",
            "99",
            "--ignore-filename-regex",
            EXCLUDED_FROM_COVERAGE,
            "--summary-only",
        ],
    )?;
    // The static behaviour check executes no scenario: it asserts that every
    // scenario in the corpus is bound at every layer, or validly exempt. It is
    // in the quick gate because an unbound scenario reports nothing, and
    // reporting nothing looks exactly like passing.
    run("cargo", &["test", "--test", "coverage"])
}
