//! Embeds the revision this build was made from.
//!
//! Runs at compile time rather than at run time on purpose: the product spawns
//! no child process, and a version command that shelled out to `git` would
//! break that on the one path a user is most likely to run first.

fn main() {
    println!("cargo:rerun-if-changed=.git/HEAD");
    println!("cargo:rerun-if-changed=.git/refs");

    let commit = std::process::Command::new("git")
        .args(["rev-parse", "HEAD"])
        .output()
        .ok()
        .filter(|output| output.status.success())
        .and_then(|output| String::from_utf8(output.stdout).ok())
        .map(|text| text.trim().to_string())
        .filter(|text| text.len() == 40 && text.chars().all(|c| c.is_ascii_hexdigit()))
        // A build made outside a repository reports forty zeros rather than
        // claiming a revision it was not made from.
        .unwrap_or_else(|| "0".repeat(40));

    println!("cargo:rustc-env=RHINO_COMMIT={commit}");
}
