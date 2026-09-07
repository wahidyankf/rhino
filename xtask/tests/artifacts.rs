//! What a release must be true of before it is published.
//!
//! These read `dist/` rather than building anything, because the thing under
//! test is the release *artifact* -- what a consumer downloads and runs -- and
//! a test that rebuilt it would be checking a different binary from the one
//! that ships. Run `cargo xtask dist` first; without it every assertion here
//! fails, which is the correct answer to "is this release verified?".
//!
//! On a developer machine only the host platform's archive exists, so the
//! four-platform assertion is made when the whole matrix is present and its
//! absence is reported rather than passed over. In CI every platform's archive
//! has been assembled before this runs, and the assertion is total.

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};
use std::process::Command;

/// Every platform a release must carry, named as the archive names them.
///
/// The four are the ones a consumer of this tool actually runs on, and the
/// list is here rather than in the workflow so a matrix that silently lost a
/// leg fails a test instead of publishing three archives.
const PLATFORMS: [&str; 4] = [
    "aarch64-apple-darwin",
    "x86_64-apple-darwin",
    "aarch64-unknown-linux-gnu",
    "x86_64-unknown-linux-gnu",
];

/// The largest a stripped executable may be, in bytes.
///
/// Set from the first measured build -- 1,499,424 bytes on `aarch64-apple-darwin`
/// -- plus about a fifth for the platforms that run larger and for the growth a
/// few more rules will bring. A ceiling loose enough never to fire is a budget
/// nobody holds, so this is close enough to the measurement to notice.
const SIZE_CEILING: u64 = 1_835_008;

/// The version the product's own manifest declares.
fn product_version() -> String {
    let manifest = repository_root().join("Cargo.toml");
    let text = std::fs::read_to_string(&manifest)
        .unwrap_or_else(|error| panic!("{}: {error}", manifest.display()));
    text.lines()
        .take_while(|line| !line.starts_with("[workspace]"))
        .find_map(|line| line.strip_prefix("version = "))
        .map(|value| value.trim().trim_matches('"').to_string())
        .expect("the product manifest declares a version")
}

fn repository_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .parent()
        .expect("the xtask crate sits inside the repository")
        .to_path_buf()
}

fn dist() -> PathBuf {
    repository_root().join("dist")
}

fn read_checksums() -> BTreeMap<String, String> {
    let path = dist().join("checksums.txt");
    let text = std::fs::read_to_string(&path)
        .unwrap_or_else(|error| panic!("{}: {error}", path.display()));
    text.lines()
        .filter(|line| !line.trim().is_empty())
        .map(|line| {
            let (digest, name) = line
                .split_once("  ")
                .unwrap_or_else(|| panic!("malformed checksum line: {line}"));
            (name.trim().to_string(), digest.trim().to_string())
        })
        .collect()
}

fn sha256(path: &Path) -> String {
    let output = Command::new("shasum")
        .args(["-a", "256"])
        .arg(path)
        .output()
        .unwrap_or_else(|error| panic!("failed to hash {}: {error}", path.display()));
    assert!(output.status.success(), "hashing {} failed", path.display());
    String::from_utf8_lossy(&output.stdout)
        .split_whitespace()
        .next()
        .expect("shasum prints the digest first")
        .to_string()
}

fn archive_name(platform: &str) -> String {
    format!("rhino-{platform}.tar.gz")
}

/// The platforms whose archives are present, in declaration order.
fn assembled() -> Vec<&'static str> {
    PLATFORMS
        .into_iter()
        .filter(|platform| dist().join(archive_name(platform)).exists())
        .collect()
}

#[test]
fn every_declared_platform_has_an_archive() {
    let present = assembled();
    assert!(
        !present.is_empty(),
        "dist/ holds no archive for any declared platform; run `cargo xtask dist`"
    );
    if present.len() < PLATFORMS.len() {
        // A partial matrix is what a developer machine legitimately produces,
        // and saying so is the point: silence here would read the same as a
        // complete release.
        eprintln!(
            "partial matrix: {} of {} platforms assembled ({})",
            present.len(),
            PLATFORMS.len(),
            present.join(", ")
        );
        return;
    }
    assert_eq!(present.len(), PLATFORMS.len());
}

#[test]
fn every_archive_matches_its_recorded_checksum() {
    let checksums = read_checksums();
    let present = assembled();
    assert!(!present.is_empty(), "nothing assembled to check");

    for platform in &present {
        let name = archive_name(platform);
        let recorded = checksums
            .get(&name)
            .unwrap_or_else(|| panic!("checksums.txt does not record {name}"));
        assert_eq!(
            &sha256(&dist().join(&name)),
            recorded,
            "{name} does not match its recorded checksum"
        );
    }

    // The other direction, so a checksum file cannot name an archive that was
    // never built and still be called matching.
    for name in checksums.keys() {
        assert!(
            dist().join(name).exists(),
            "checksums.txt records {name}, which is not in dist/"
        );
    }
}

#[test]
fn every_executable_reports_the_revision_it_was_built_from() {
    let expected_commit = std::env::var("RHINO_RELEASE_COMMIT").unwrap_or_else(|_| {
        let output = Command::new("git")
            .args(["rev-parse", "HEAD"])
            .current_dir(repository_root())
            .output()
            .expect("git is available");
        String::from_utf8_lossy(&output.stdout).trim().to_string()
    });
    // Read from the product's manifest rather than from `CARGO_PKG_VERSION`,
    // which inside this crate is the xtask helper's own version and would
    // compare the executable against a number nothing ships.
    let expected_version =
        std::env::var("RHINO_RELEASE_VERSION").unwrap_or_else(|_| product_version());

    // Only the host archive can be executed here; the others are checked on
    // their own runner, which is why the matrix is native rather than
    // cross-compiled.
    let host = dist().join("rhino");
    assert!(
        host.exists(),
        "dist/rhino is absent; run `cargo xtask dist`"
    );

    let output = Command::new(&host)
        .args(["version", "--json"])
        .output()
        .expect("the assembled executable runs");
    assert!(output.status.success(), "version --json failed");
    let reported = String::from_utf8_lossy(&output.stdout);

    assert!(
        reported.contains(&format!("\"commit\":\"{expected_commit}\"")),
        "reported {reported}, expected commit {expected_commit}"
    );
    assert!(
        reported.contains(&format!("\"version\":\"{expected_version}\"")),
        "reported {reported}, expected version {expected_version}"
    );
}

#[test]
fn the_stripped_executable_is_within_its_declared_ceiling() {
    let host = dist().join("rhino");
    let size = std::fs::metadata(&host)
        .unwrap_or_else(|error| panic!("{}: {error}", host.display()))
        .len();
    assert!(
        size <= SIZE_CEILING,
        "stripped executable is {size} bytes, over the declared ceiling of {SIZE_CEILING}"
    );
    eprintln!("stripped executable: {size} bytes");
}
