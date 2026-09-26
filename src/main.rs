//! The process boundary, and the only place it is crossed.
//!
//! Argument collection, standard input, the working directory, the two output
//! streams, and the exit code. Every decision lives in the library crate so the
//! behaviour corpus can reach it; what is left here is what a unit test may not
//! do, which is why this file is a declared coverage exclusion.
//!
//! Three things here are about the contract rather than about plumbing: a
//! closed reader ends the process at `141` instead of panicking, an internal
//! failure reports `2` instead of aborting, and a standard-input read that
//! fails is refused instead of being passed on as an empty document.
#![forbid(unsafe_code)]

use rhino::cli::Parsed;
use rhino::runtime::{
    DiskAdapterStore, DiskEnvironmentStore, DiskMutationRunner, DiskToolchainRunner, DiskTree,
    ProcessLauncher,
};
use rhino::{ExecutionBoundaries, Outcome};
use std::io::{ErrorKind, Write};
use std::process::ExitCode;

/// `128 + SIGPIPE`, the status a shell reports for a process the signal ended.
///
/// Reached by exiting deliberately rather than by dying from the signal,
/// because Rust sets `SIGPIPE` to `SIG_IGN` before `main` and the two stable
/// ways to undo that are both unstable compiler features. The number is the one
/// a caller reads either way; what a caller must never read is `1`, which would
/// say the run completed and found nothing.
const CLOSED_PIPE: u8 = 141;

/// What an internal failure reports. Never `1`: a crash has not established
/// that there is nothing to find.
const INTERNAL_FAILURE: u8 = 2;

fn main() -> ExitCode {
    install_panic_handler();

    #[cfg(debug_assertions)]
    fault_injection_point();

    let mut args = std::env::args_os();
    let _program = args.next();
    let arguments: Vec<String> = args
        .map(|value| value.to_string_lossy().into_owned())
        .collect();

    let outcome = match DiskTree::at_current_directory() {
        Ok(tree) => {
            // Standard input is read only when a leaf was asked for it, so an
            // ordinary run never blocks on a terminal.
            let stdin = if wants_stdin(&arguments) {
                match read_stdin() {
                    Ok(text) => Some(text),
                    Err(reason) => {
                        return emit(&Outcome::refused(rhino::errors::body(
                            format_of(&arguments),
                            rhino::errors::ErrorCode::InputUnreadable,
                            &format!("standard input could not be read: {reason}"),
                        )));
                    }
                }
            } else {
                None
            };
            rhino::execute_using_with_boundaries(
                &tree,
                &arguments,
                stdin.as_deref(),
                ExecutionBoundaries {
                    launcher: &ProcessLauncher,
                    mutations: &DiskMutationRunner,
                    adapters: &DiskAdapterStore,
                    environments: &DiskEnvironmentStore,
                    toolchains: &DiskToolchainRunner,
                },
            )
        }
        Err(reason) => Outcome::refused(rhino::errors::body(
            format_of(&arguments),
            rhino::errors::ErrorCode::RepositoryUnusable,
            &reason.to_string(),
        )),
    };

    emit(&outcome)
}

/// Write both streams and report the status, converting a closed reader into
/// `141`.
///
/// `print!` would panic here, and the panic would be reported as an internal
/// failure of this tool. A reader that stopped reading is not this tool
/// failing: it is the ordinary end of `rhino … | head`, and every Unix utility
/// written in C has always ended it quietly.
fn emit(outcome: &Outcome) -> ExitCode {
    if write_stream(&mut std::io::stdout().lock(), &outcome.stdout).is_err() {
        return ExitCode::from(CLOSED_PIPE);
    }
    if write_stream(&mut std::io::stderr().lock(), &outcome.stderr).is_err() {
        return ExitCode::from(CLOSED_PIPE);
    }
    ExitCode::from(outcome.exit_code)
}

/// Write one stream and flush it, so a closed reader is discovered here rather
/// than during the runtime's own exit-time flush, which has nowhere to report.
fn write_stream(stream: &mut impl Write, text: &str) -> Result<(), ()> {
    if text.is_empty() {
        return Ok(());
    }
    match stream
        .write_all(text.as_bytes())
        .and_then(|()| stream.flush())
    {
        Ok(()) => Ok(()),
        Err(error) if error.kind() == ErrorKind::BrokenPipe => Err(()),
        // Any other write failure is this tool failing to report, which is the
        // one thing it cannot report. Ending on the internal-failure status is
        // the most that can honestly be said.
        Err(_) => std::process::exit(i32::from(INTERNAL_FAILURE)),
    }
}

/// Report an internal failure as `2` with one diagnostic, rather than letting
/// the runtime end the process however its panic strategy happens to be set.
///
/// This build uses `panic = "abort"`, so without a handler an internal failure
/// arrives as SIGABRT. The status is not the worst of it: the default handler
/// prints a panic message naming a source path, which is this tool's internals
/// in a caller's terminal.
fn install_panic_handler() {
    std::panic::set_hook(Box::new(|info| {
        let mut stderr = std::io::stderr().lock();
        let _ = writeln!(stderr, "rhino: internal error: {}", panic_message(info));
        // Behind a switch, and behind the runtime's own switch rather than a
        // second one: a caller who already knows `RUST_BACKTRACE` should not
        // have to learn a rhino-specific spelling of it.
        let trace = std::backtrace::Backtrace::capture();
        if trace.status() == std::backtrace::BacktraceStatus::Captured {
            let _ = writeln!(stderr, "{trace}");
        }
        let _ = stderr.flush();
        std::process::exit(i32::from(INTERNAL_FAILURE));
    }));
}

/// The panic payload as a line, or a stand-in when it is neither string shape.
fn panic_message(info: &std::panic::PanicHookInfo<'_>) -> String {
    let payload = info.payload();
    if let Some(text) = payload.downcast_ref::<&str>() {
        return (*text).to_string();
    }
    if let Some(text) = payload.downcast_ref::<String>() {
        return text.clone();
    }
    "a failure with no message".to_string()
}

/// A way to provoke an internal failure, in debug builds only.
///
/// Two assertions in the conformance corpus are about what a crash reports, and
/// without a seam they stay Unmeasured forever -- which reads as "we could not
/// check" and means "we chose not to". The seam is compiled out of every
/// release artifact, so the released binary has no way to be asked to panic.
#[cfg(debug_assertions)]
fn fault_injection_point() {
    if std::env::var_os("RHINO_PANIC_FOR_TESTS").is_some() {
        panic!("fault injected by RHINO_PANIC_FOR_TESTS");
    }
}

/// The output format this invocation asked for, read straight off the command
/// line.
///
/// The two refusals here happen before the parser has run -- one of them is the
/// parser's input failing to arrive -- so they cannot ask the parser what was
/// requested. Reading two arguments is the whole of the duplication, and the
/// alternative is answering a caller who asked for a document with prose.
fn format_of(arguments: &[String]) -> rhino::cli::Format {
    let asked_for_json = arguments.iter().any(|value| value == "--json")
        || arguments
            .windows(2)
            .any(|pair| pair[0] == "--output" && pair[1] == "json");
    if asked_for_json {
        rhino::cli::Format::Json
    } else {
        rhino::cli::Format::Text
    }
}

/// Whether this invocation reads the real standard input.
///
/// A `--file -` selection asks for it, and so does `--push-updates-stdin`,
/// which only a `pre-push` gate run accepts: Git hands that hook its update
/// records on standard input. Nothing else selects the stream, so a run by hand
/// from a terminal, including a `pre-push` run without the flag, never waits on
/// it before a declared child can start.
///
/// The command line is read by the parser that runs it, because a flag such
/// as `--root` may come before the command path. Matching the raw arguments
/// missed `rhino --root . gate run`, which then saw no update record at all.
/// An invocation the parser refuses reads nothing: it is refused before any
/// leaf could use the input.
fn wants_stdin(arguments: &[String]) -> bool {
    let Ok(Parsed::Run(invocation)) = rhino::cli::parse(arguments) else {
        return false;
    };
    invocation.push_updates_stdin || invocation.files.iter().any(|file| file == "-")
}

/// Standard input, or the reason it could not be read.
///
/// The failure is returned rather than swallowed. It used to be discarded, and
/// the leaf then reported an empty document -- so a stream that failed halfway
/// through was indistinguishable from one that was genuinely empty, and the
/// caller was told its input had no findings in it.
fn read_stdin() -> Result<String, std::io::Error> {
    use std::io::Read;
    let mut text = String::new();
    std::io::stdin().read_to_string(&mut text)?;
    Ok(text)
}
