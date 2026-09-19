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

/// Every platform a release must carry, with the largest its stripped
/// executable may be, in bytes.
///
/// The four are the ones a consumer of this tool actually runs on, and the
/// list is here rather than in the workflow so a matrix that silently lost a
/// leg fails a test instead of publishing three archives.
///
/// A budget per platform rather than one for all four, because the same source
/// produces executables 43% apart:
///
/// | platform                    | native measurement | ceiling |
/// | --------------------------- | ------------------ | ------- |
/// | `aarch64-apple-darwin`      | 1,968,112          | 2.5 MiB |
/// | `x86_64-apple-darwin`       | 2,435,952          | 3 MiB   |
/// | `aarch64-unknown-linux-gnu` | 2,495,392          | 3 MiB   |
/// | `x86_64-unknown-linux-gnu`  | 2,936,016          | 3.5 MiB |
///
/// Every ceiling is 120% of its complete native measurement, rounded up to the
/// next quarter MiB. That preserves a modest, explicit growth allowance without
/// hiding a smaller-platform regression behind the largest target. One ceiling
/// covering all four would do exactly that; a budget nobody can exceed is a
/// budget nobody holds.
const PLATFORMS: [(&str, u64); 4] = [
    ("aarch64-apple-darwin", 2_621_440),
    ("x86_64-apple-darwin", 3_145_728),
    ("aarch64-unknown-linux-gnu", 3_145_728),
    ("x86_64-unknown-linux-gnu", 3_670_016),
];

const SCHEMA_ASSET: &str = "rhino-repo-config-v2.schema.json";

/// The version the product's own manifest declares, spelled as a release tag.
///
/// Cargo's manifest cannot carry the `v`; the executable adds it so that what
/// it reports is the tag verbatim. This has to add it too, or the comparison
/// below would be between two different spellings of the same release.
fn product_version() -> String {
    let manifest = repository_root().join("Cargo.toml");
    let text = std::fs::read_to_string(&manifest)
        .unwrap_or_else(|error| panic!("{}: {error}", manifest.display()));
    text.lines()
        .take_while(|line| !line.starts_with("[workspace]"))
        .find_map(|line| line.strip_prefix("version = "))
        .map(|value| format!("v{}", value.trim().trim_matches('"')))
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

/// Hash through the platform's own tool, never through `sha2`.
///
/// `cargo xtask` writes `checksums.txt` with the `sha2` crate. Verifying it with
/// the same crate would compare an implementation to itself and pass whatever it
/// did. Shelling out means the digest a consumer would compute with the tool
/// already on their machine is the digest this asserts.
///
/// `sha256sum` on Linux, `shasum` on macOS; neither platform carries both by
/// default, so both are tried rather than one being assumed.
fn sha256(path: &Path) -> String {
    let attempts: [(&str, &[&str]); 2] = [("sha256sum", &[]), ("shasum", &["-a", "256"])];
    for (program, arguments) in attempts {
        let Ok(output) = Command::new(program).args(arguments).arg(path).output() else {
            continue;
        };
        assert!(
            output.status.success(),
            "{program} failed on {}",
            path.display()
        );
        return String::from_utf8_lossy(&output.stdout)
            .split_whitespace()
            .next()
            .expect("the digest is printed first")
            .to_string();
    }
    panic!(
        "neither sha256sum nor shasum is available to verify {}",
        path.display()
    );
}

fn archive_name(platform: &str) -> String {
    format!("rhino-{platform}.tar.gz")
}

/// The platforms whose archives are present, in declaration order.
fn assembled() -> Vec<(&'static str, u64)> {
    PLATFORMS
        .into_iter()
        .filter(|(platform, _)| dist().join(archive_name(platform)).exists())
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
        //
        // Not on the job that publishes, though. There a missing leg is the
        // exact failure this assertion exists for, and a report nobody reads is
        // how three archives get published as four. `RHINO_RELEASE_MATRIX` is
        // how that job says which of the two it is.
        let report = format!(
            "partial matrix: {} of {} platforms assembled ({})",
            present.len(),
            PLATFORMS.len(),
            present
                .iter()
                .map(|(platform, _)| *platform)
                .collect::<Vec<_>>()
                .join(", ")
        );
        assert!(
            std::env::var("RHINO_RELEASE_MATRIX").as_deref() != Ok("complete"),
            "{report}"
        );
        eprintln!("{report}");
        return;
    }
    assert_eq!(present.len(), PLATFORMS.len());
}

#[test]
fn every_archive_matches_its_recorded_checksum() {
    let checksums = read_checksums();
    let present = assembled();
    assert!(!present.is_empty(), "nothing assembled to check");

    for (platform, _) in &present {
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
fn grouped_schema_is_an_immutable_checksum_asset() {
    let released = dist().join(SCHEMA_ASSET);
    assert!(
        released.exists(),
        "dist/ does not contain {SCHEMA_ASSET}; run `cargo xtask dist`"
    );
    let source = repository_root().join("schemas/repo-config/v2.schema.json");
    assert_eq!(
        std::fs::read(&released).expect("the staged schema is readable"),
        std::fs::read(&source).expect("the checked-in schema is readable"),
        "the release schema must be the checked-in schema"
    );
    let checksums = read_checksums();
    assert_eq!(
        checksums.get(SCHEMA_ASSET),
        Some(&sha256(&released)),
        "checksums.txt must cover the grouped schema"
    );
}

#[test]
fn release_workflow_publishes_the_checksummed_grouped_schema() {
    let workflow = repository_root().join(".github/workflows/release.yml");
    let text = std::fs::read_to_string(&workflow)
        .unwrap_or_else(|error| panic!("{}: {error}", workflow.display()));
    assert!(
        text.contains("cargo xtask checksums"),
        "release workflow must create the artifact manifest"
    );
    assert!(
        text.contains(&format!("dist/{SCHEMA_ASSET}")),
        "release workflow must attach the grouped schema asset"
    );
    assert!(
        text.contains("$GITHUB_REF_NAME\" == \"v0.4.0\"")
            && text.contains("release_notes+=(--notes-start-tag v0.3.0)"),
        "the v0.4.0 stable release notes must start at v0.3.0, not its final RC"
    );
}

#[test]
fn release_cut_screens_the_v0_4_stable_note_range() {
    let workflow = repository_root().join("repo-governance/workflows/release-cut.md");
    let text = std::fs::read_to_string(&workflow)
        .unwrap_or_else(|error| panic!("{}: {error}", workflow.display()));
    assert!(
        text.contains("previous_tag_name=v0.3.0")
            && text.contains("tag_name=v0.4.0")
            && text.contains("--text \"$release_notes\""),
        "the v0.4.0 release procedure must screen the same v0.3.0-based notes it publishes"
    );
}

#[test]
fn release_size_rehearsal_measures_each_native_target_without_publishing() {
    let workflow = repository_root().join(".github/workflows/release-size-rehearsal.yml");
    let text = std::fs::read_to_string(&workflow)
        .unwrap_or_else(|error| panic!("{}: {error}", workflow.display()));

    assert!(
        text.contains("workflow_dispatch"),
        "size rehearsal must remain manually dispatchable from default branch"
    );
    assert!(
        text.contains("pull_request:")
            && text.contains("labeled")
            && text.contains("github.event.label.name == 'release-size-rehearsal'"),
        "a labeled draft PR must be able to request rehearsal before the workflow reaches default branch"
    );
    assert!(
        text.contains("github.event.pull_request.head.sha"),
        "PR rehearsal must assemble the exact reviewed head, not a synthetic merge commit"
    );
    assert!(
        text.contains("cargo xtask dist"),
        "size rehearsal must assemble the same native artifact as release"
    );
    assert!(
        text.contains("executable_bytes") && text.contains("archive_bytes"),
        "size rehearsal must record both executable and archive measurements"
    );
    assert!(
        text.contains("jq -e -s")
            && text.contains("schemaVersion: 1")
            && text.contains("metrics: .")
            && !text.contains("sed '$!s/$/,'"),
        "size rehearsal must create one valid JSON aggregate structurally, not splice JSON lines"
    );
    assert!(
        !text.contains("gh release create") && !text.contains("contents: write"),
        "size rehearsal must not publish or receive publication permission"
    );
    for (platform, _) in PLATFORMS {
        assert!(
            text.contains(platform),
            "size rehearsal omits release target {platform}"
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

/// Measured out of every archive, not just the one this machine can run.
///
/// The identity assertion above is limited to the host executable because only
/// that one starts here. Size is not: an archive can be opened on any platform,
/// so the whole set is weighed wherever this runs, and a leg that grew is caught
/// on the job that assembles the release rather than only on the runner that
/// built it.
#[test]
fn every_executable_is_within_its_platform_ceiling() {
    let present = assembled();
    assert!(!present.is_empty(), "nothing assembled to weigh");

    let unpacked = dist().join(".weighing");
    for (platform, ceiling) in &present {
        // One directory per platform: every archive holds a file called
        // `rhino`, so unpacking them together would weigh one of them four
        // times and call it four platforms passing.
        let room = unpacked.join(platform);
        std::fs::create_dir_all(&room)
            .unwrap_or_else(|error| panic!("{}: {error}", room.display()));
        let status = Command::new("tar")
            .args(["-xzf"])
            .arg(dist().join(archive_name(platform)))
            .arg("-C")
            .arg(&room)
            .status()
            .expect("tar is available");
        assert!(status.success(), "could not unpack {platform}");

        let executable = room.join("rhino");
        let size = std::fs::metadata(&executable)
            .unwrap_or_else(|error| panic!("{}: {error}", executable.display()))
            .len();
        eprintln!("{platform}: {size} bytes");
        assert!(
            size <= *ceiling,
            "{platform} is {size} bytes, over its declared ceiling of {ceiling}"
        );
    }

    let _ = std::fs::remove_dir_all(&unpacked);
}
