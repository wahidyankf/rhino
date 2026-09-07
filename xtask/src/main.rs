//! Repository automation. Kept in Rust rather than shell so the release path
//! has the same type checking and the same tests as the product.
//!
//! `test-quick` is what the pre-push hook runs, so it stays fast by
//! construction: it never invokes the integration or E2E adapters. Those are
//! slow enough to train a maintainer into bypassing the hook, and they belong
//! to the scheduled workflow instead.
#![forbid(unsafe_code)]

use std::process::{Command, ExitCode};

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
    )
}
