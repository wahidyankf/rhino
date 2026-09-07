//! Argument parsing, output writing, and the exit code. Every decision lives in
//! the library crate so the behaviour corpus can reach it; this file is the one
//! place the process boundary is crossed, and it is a declared coverage
//! exclusion for that reason.
#![forbid(unsafe_code)]

fn main() -> std::process::ExitCode {
    std::process::ExitCode::from(rhino::run(std::env::args_os()))
}
