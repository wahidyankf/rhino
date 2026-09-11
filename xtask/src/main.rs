//! Repository automation. Kept in Rust rather than shell so the release path
//! has the same type checking and the same tests as the product.
//!
//! `test-quick` is what the pre-push hook runs, so it stays fast by
//! construction: it never invokes the integration or E2E adapters. Those are
//! slow enough to train a maintainer into bypassing the hook, and they belong
//! to the scheduled workflow instead.
#![forbid(unsafe_code)]

use sha2::{Digest, Sha256};
use std::path::{Path, PathBuf};
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
        Some("self-validate") => self_validate(),
        Some("dist") => dist(),
        Some("checksums") => checksums(),
        other => {
            eprintln!("unknown task: {other:?}");
            eprintln!("tasks: test-quick, self-validate, dist, checksums");
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
    run("cargo", &["test", "--test", "coverage"])?;
    // The validators used to run here, last, so that a repository which is out of
    // date could not mask a product that is broken. They still run last -- the
    // `pre-push` surface declares this gate ahead of them in `repo-config.yml`,
    // and the same ordering is written down there with the same reason. What
    // changed is who owns the list.
    Ok(())
}

/// Run this repository's own `ci` gate surface, with the binary it builds.
///
/// A second corpus that no one wrote: the tree changes constantly, nobody
/// curates it as test data, and every page in it is something a maintainer
/// actually wanted to say. That is the property the executable corpus cannot
/// have, and the reason this runs on every push rather than once.
///
/// The list of commands used to live here, as a nine-entry array. It now lives
/// in `repo-config.yml` under `gates`, where every consumer of this tool
/// declares the same thing -- and where a reader can see the order and the
/// surfaces without reading Rust. This task is the entry point, not the
/// registry; it hands the question to the surface and reports the answer.
///
/// A finding fails the gate exactly as a failing test does: exit 1 means
/// RHINO's own tree violates RHINO's own `repo-config.yml`, and exit 2 means the
/// configuration or the invocation is unusable. Neither is a warning.
fn self_validate() -> Result<(), String> {
    // Built once up front so a compile error is reported as a compile error
    // rather than as the first gate failing to start.
    run("cargo", &["build", "--quiet"])?;
    run(
        "cargo",
        &[
            "run",
            "--quiet",
            "--bin",
            "rhino",
            "--",
            "gate",
            "run",
            "--surface",
            "ci",
        ],
    )
}

// -- Release artifacts --------------------------------------------------------

/// Build this platform's release archive into `dist/`.
///
/// One platform per invocation, deliberately. The release matrix builds each
/// archive on a runner of that architecture rather than cross-compiling, so
/// every published executable has actually started on the operating system it
/// claims to run on -- the one property a cross-compiled artifact cannot
/// demonstrate about itself. This task is what each of those runners calls, and
/// what a maintainer calls locally to get the same archive for the machine in
/// front of them.
///
/// It assembles rather than verifies. `cargo test --package xtask` reads what
/// this leaves behind, so the checks live outside the thing they check.
fn dist() -> Result<(), String> {
    let root = repository_root();
    let target = host_target()?;
    let output = root.join("dist");

    run("cargo", &["build", "--release"])?;

    std::fs::create_dir_all(&output).map_err(|error| format!("creating dist: {error}"))?;

    let built = root.join("target/release/rhino");
    if !built.exists() {
        return Err(format!("{} was not built", built.display()));
    }

    // Staged under its plain name as well as archived, so the verification
    // tests can run the exact bytes that go into the archive rather than a
    // second build of the same source.
    let staged = output.join("rhino");
    std::fs::copy(&built, &staged).map_err(|error| format!("staging the executable: {error}"))?;

    let archive = format!("rhino-{target}.tar.gz");
    run_in(
        &output,
        "tar",
        &["--create", "--gzip", "--file", &archive, "rhino"],
    )?;

    write_checksums(&output)?;

    let size = std::fs::metadata(&staged)
        .map_err(|error| format!("measuring the executable: {error}"))?
        .len();
    println!("dist/{archive}");
    println!("stripped executable: {size} bytes");
    Ok(())
}

/// Rewrite `dist/checksums.txt` from whatever archives are there now.
///
/// `dist` already does this for the one archive it built. The release matrix
/// assembles the other three from separate runners, so the job that collects
/// them needs the same writer rather than a second one spelled in shell: one
/// digest file, written by one implementation, however many legs produced it.
fn checksums() -> Result<(), String> {
    let output = repository_root().join("dist");
    write_checksums(&output)?;
    print!(
        "{}",
        std::fs::read_to_string(output.join("checksums.txt"))
            .map_err(|error| format!("reading checksums: {error}"))?
    );
    Ok(())
}

fn repository_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .parent()
        .expect("the xtask crate sits inside the repository")
        .to_path_buf()
}

/// The triple this machine builds for, asked of the compiler rather than
/// guessed from the operating system: a name that did not come from `rustc` is
/// a name that can disagree with what was actually produced.
fn host_target() -> Result<String, String> {
    let output = Command::new("rustc")
        .arg("-vV")
        .output()
        .map_err(|error| format!("failed to start rustc: {error}"))?;
    String::from_utf8_lossy(&output.stdout)
        .lines()
        .find_map(|line| line.strip_prefix("host: "))
        .map(str::to_string)
        .ok_or_else(|| "rustc did not report a host triple".to_string())
}

/// Record a digest for every archive in `dist/`, replacing any earlier file.
///
/// Rewritten wholesale rather than appended to, so a second run cannot leave
/// two lines for one archive and let a reader take the stale one.
fn write_checksums(output: &Path) -> Result<(), String> {
    let mut archives: Vec<String> = std::fs::read_dir(output)
        .map_err(|error| format!("reading dist: {error}"))?
        .flatten()
        .map(|entry| entry.file_name().to_string_lossy().to_string())
        .filter(|name| name.ends_with(".tar.gz"))
        .collect();
    archives.sort();

    let mut lines = String::new();
    for name in archives {
        let bytes = std::fs::read(output.join(&name))
            .map_err(|error| format!("reading {name}: {error}"))?;
        let mut digest = Sha256::new();
        digest.update(&bytes);
        // Two spaces between digest and name: the format `shasum -c` and
        // `sha256sum -c` both read, so a consumer verifies with the tool their
        // platform already has rather than one this release picked for them.
        let rendered: String = digest
            .finalize()
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect();
        lines.push_str(&format!("{rendered}  {name}\n"));
    }
    std::fs::write(output.join("checksums.txt"), lines)
        .map_err(|error| format!("writing checksums: {error}"))
}

fn run_in(directory: &Path, program: &str, args: &[&str]) -> Result<(), String> {
    let status = Command::new(program)
        .args(args)
        .current_dir(directory)
        .status()
        .map_err(|error| format!("failed to start {program}: {error}"))?;
    if status.success() {
        Ok(())
    } else {
        Err(format!("{program} {} failed", args.join(" ")))
    }
}
