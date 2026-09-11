//! The shared plan-structure fixture corpus.
//!
//! More than one implementation validates plan structure, and the claim that
//! they agree only means something if they read the same bytes. This corpus is
//! that boundary: `specs/fixtures/plan-structure/` is a byte-identical copy of
//! the catalog's, its digest is checked here before anything is run, and a
//! mismatch stops the suite rather than warning in it.
//!
//! Driven from the process boundary because the contract is stated in process
//! terms -- a repository root in, sorted diagnostics and an exit class out.

use std::collections::BTreeSet;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};

/// The corpus digest, pinned. Regenerating the corpus without announcing it to
/// every implementation that pins this value is the silent drift the digest
/// exists to make loud.
const PINNED_DIGEST: &str = "57c2531f8d305a0b43c796a0b9498256ff1050fa703dd7a22820955fcfb2fe25";

/// The corpus is a plan tree and nothing else. A repository declares its own
/// configuration, so the runner writes one beside each case rather than the
/// corpus carrying twenty-four copies of it.
const CONFIG: &str = "schema: ose/repo-config/v2\nvisibility: private\ngates:\n  - id: plan\n    kind: check\n    run:\n      - ./gates/plan.sh\n    surfaces:\n      - commit-msg\n      - pre-commit\n      - pre-push\n";

fn corpus() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("specs/fixtures/plan-structure")
}

#[test]
fn the_corpus_is_the_bytes_the_catalog_froze() {
    let sums = std::fs::read(corpus().join("SHA256SUMS")).expect("SHA256SUMS is committed");
    let recorded = std::fs::read_to_string(corpus().join("CORPUS-DIGEST"))
        .expect("CORPUS-DIGEST is committed");
    assert_eq!(
        recorded.trim(),
        PINNED_DIGEST,
        "the committed corpus digest is not the one this suite pins"
    );
    assert_eq!(
        digest(&sums),
        PINNED_DIGEST,
        "SHA256SUMS does not hash to the digest recorded beside it"
    );

    let listing = String::from_utf8(sums).expect("SHA256SUMS is text");
    let mut wrong: Vec<String> = Vec::new();
    for line in listing.lines() {
        let Some((expected, relative)) = line.split_once("  ") else {
            continue;
        };
        let bytes = std::fs::read(corpus().join(relative))
            .unwrap_or_else(|error| panic!("{relative} is readable: {error}"));
        if digest(&bytes) != expected {
            wrong.push(relative.to_string());
        }
    }
    assert!(
        wrong.is_empty(),
        "corpus files differ from SHA256SUMS: {wrong:?}"
    );
}

#[test]
fn every_case_matches_the_manifest() {
    let manifest =
        std::fs::read_to_string(corpus().join("manifest.tsv")).expect("the manifest is committed");
    let mut failures: Vec<String> = Vec::new();

    for row in manifest
        .lines()
        .skip(1)
        .filter(|row| !row.trim().is_empty())
    {
        let fields: Vec<&str> = row.split('\t').collect();
        let [case, exit, rules, ..] = fields.as_slice() else {
            panic!("manifest row has too few fields: {row}");
        };
        let expected_exit: i32 = exit.parse().expect("the expected exit is a number");
        let expected: BTreeSet<String> = rules
            .split(',')
            .map(str::trim)
            .filter(|rule| !rule.is_empty())
            .map(str::to_string)
            .collect();

        let sandbox = stage(case);
        let first = run(sandbox.path());
        let second = run(sandbox.path());

        if first != second {
            failures.push(format!("{case}: two runs over unchanged input disagreed"));
            continue;
        }
        if first.0 != expected_exit {
            failures.push(format!(
                "{case}: expected exit {expected_exit}, got {}\n{}",
                first.0, first.1
            ));
            continue;
        }
        // Equality, not containment. A run that reports the declared rule and
        // three others has not agreed with the contract; it has agreed with one
        // line of it.
        let reported = identifiers(&first.1);
        if reported != expected {
            failures.push(format!(
                "{case}: expected rules {expected:?}, got {reported:?}\n{}",
                first.1
            ));
        }
    }

    assert!(failures.is_empty(), "{}", failures.join("\n\n"));
}

/// A temporary copy of one case, with a configuration written beside it.
///
/// Copied rather than run in place because the corpus is read-only by
/// construction: a runner that wrote a configuration into it would change the
/// bytes the digest above just proved.
struct Staged(PathBuf);

impl Staged {
    fn path(&self) -> &Path {
        &self.0
    }
}

impl Drop for Staged {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

fn stage(case: &str) -> Staged {
    let slug = case.replace('/', "__");
    let root = std::env::temp_dir().join(format!("rhino-plan-corpus-{slug}"));
    let _ = std::fs::remove_dir_all(&root);
    std::fs::create_dir_all(&root).expect("the staging directory is creatable");
    copy(&corpus().join(case), &root);
    std::fs::write(root.join("repo-config.yml"), CONFIG).expect("the configuration is writable");
    Staged(root)
}

fn copy(from: &Path, to: &Path) {
    for entry in std::fs::read_dir(from).expect("the case directory is readable") {
        let entry = entry.expect("the entry is readable");
        let target = to.join(entry.file_name());
        if entry.path().is_dir() {
            std::fs::create_dir_all(&target).expect("the directory is creatable");
            copy(&entry.path(), &target);
        } else {
            std::fs::copy(entry.path(), &target).expect("the file is copyable");
        }
    }
}

fn run(root: &Path) -> (i32, String) {
    let output = Command::new(env!("CARGO_BIN_EXE_rhino"))
        .args(["plan", "validate"])
        .current_dir(root)
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .output()
        .expect("the built executable is runnable");
    (
        output.status.code().expect("the run was not signalled"),
        String::from_utf8_lossy(&output.stderr).into_owned(),
    )
}

/// The rule identifiers a run reported, read from the frozen diagnostic shape
/// `path:line:column rule field message`.
fn identifiers(stderr: &str) -> BTreeSet<String> {
    stderr
        .lines()
        .filter_map(|line| line.split_once("] "))
        .filter_map(|(_, body)| body.split_whitespace().nth(1))
        .filter(|rule| rule.starts_with("PLAN-"))
        .map(str::to_string)
        .collect()
}

/// SHA-256, written out rather than taken from a dependency.
///
/// The digest is how this suite decides whether the corpus is the one the
/// catalog froze, and a check that trusts a crate to answer that has moved the
/// question rather than answered it.
fn digest(bytes: &[u8]) -> String {
    const K: [u32; 64] = [
        0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4,
        0xab1c5ed5, 0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe,
        0x9bdc06a7, 0xc19bf174, 0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f,
        0x4a7484aa, 0x5cb0a9dc, 0x76f988da, 0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7,
        0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967, 0x27b70a85, 0x2e1b2138, 0x4d2c6dfc,
        0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85, 0xa2bfe8a1, 0xa81a664b,
        0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070, 0x19a4c116,
        0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
        0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7,
        0xc67178f2,
    ];
    let mut state: [u32; 8] = [
        0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a, 0x510e527f, 0x9b05688c, 0x1f83d9ab,
        0x5be0cd19,
    ];

    let mut message = bytes.to_vec();
    let length = (bytes.len() as u64) * 8;
    message.push(0x80);
    while message.len() % 64 != 56 {
        message.push(0);
    }
    message.extend_from_slice(&length.to_be_bytes());

    for chunk in message.chunks(64) {
        let mut w = [0u32; 64];
        for (index, word) in chunk.chunks(4).enumerate() {
            w[index] = u32::from_be_bytes([word[0], word[1], word[2], word[3]]);
        }
        for index in 16..64 {
            let s0 = w[index - 15].rotate_right(7)
                ^ w[index - 15].rotate_right(18)
                ^ (w[index - 15] >> 3);
            let s1 = w[index - 2].rotate_right(17)
                ^ w[index - 2].rotate_right(19)
                ^ (w[index - 2] >> 10);
            w[index] = w[index - 16]
                .wrapping_add(s0)
                .wrapping_add(w[index - 7])
                .wrapping_add(s1);
        }

        let [mut a, mut b, mut c, mut d, mut e, mut f, mut g, mut h] = state;
        for index in 0..64 {
            let s1 = e.rotate_right(6) ^ e.rotate_right(11) ^ e.rotate_right(25);
            let choose = (e & f) ^ ((!e) & g);
            let temp1 = h
                .wrapping_add(s1)
                .wrapping_add(choose)
                .wrapping_add(K[index])
                .wrapping_add(w[index]);
            let s0 = a.rotate_right(2) ^ a.rotate_right(13) ^ a.rotate_right(22);
            let majority = (a & b) ^ (a & c) ^ (b & c);
            let temp2 = s0.wrapping_add(majority);
            h = g;
            g = f;
            f = e;
            e = d.wrapping_add(temp1);
            d = c;
            c = b;
            b = a;
            a = temp1.wrapping_add(temp2);
        }
        for (slot, value) in state.iter_mut().zip([a, b, c, d, e, f, g, h]) {
            *slot = slot.wrapping_add(value);
        }
    }

    state.iter().map(|word| format!("{word:08x}")).collect()
}
