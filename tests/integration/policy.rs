//! The boundary policies, enforced rather than documented.
//!
//! RHINO claims to be a Unix tool: it reads a repository, writes a report, and
//! does nothing else. Four of those claims -- no network, no child process, no
//! write to the inspected tree, no path leaving the root -- are the reason a
//! maintainer is willing to run it over a tree they care about on every push.
//! A claim that lives only in prose is one a later change can quietly retract,
//! so each is a test here.
//!
//! Two of them are absences, and an absence cannot be observed by running the
//! tool: no run proves the *next* run opens no socket. Those two are checked
//! against the source and the dependency graph instead, which is where the
//! capability would have to appear before it could ever be exercised.

use crate::harness;
use crate::sandbox::{self, Sandbox};
use crate::world::{World, differences};
use rhino::runtime::{DiskTree, Tree};
use std::path::{Path, PathBuf};

/// Every command path the tool answers, as a caller would type it.
const LEAVES: &[&[&str]] = &[
    &["governance", "word-budget", "validate"],
    &["governance", "directory-map", "validate"],
    &["harness", "parity", "validate"],
    &["md", "internal-link", "validate"],
    &["md", "mermaid", "validate"],
    &["md", "word-count", "inspect", "--file", "rules/README.md"],
    &["repo-config", "validate"],
    &["version"],
];

fn crate_root() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
}

/// Every `.rs` file under `src/`, with its repository-relative path.
///
/// `src/` rather than the whole crate: the test adapters spawn processes and
/// the build script runs `git`, and neither is in the shipped binary. The
/// policy is about what the tool can do, not about what its harness does.
fn sources() -> Vec<(String, String)> {
    fn walk(directory: &Path, root: &Path, found: &mut Vec<(String, String)>) {
        for entry in std::fs::read_dir(directory)
            .expect("the source directory is readable")
            .flatten()
        {
            let path = entry.path();
            if path.is_dir() {
                walk(&path, root, found);
            } else if path.extension().is_some_and(|kind| kind == "rs") {
                let relative = path
                    .strip_prefix(root)
                    .expect("every source path is under the crate root")
                    .to_string_lossy()
                    .replace('\\', "/");
                let text = std::fs::read_to_string(&path).expect("a source file is readable");
                found.push((relative, text));
            }
        }
    }

    let root = crate_root();
    let mut found = Vec::new();
    walk(&root.join("src"), &root, &mut found);
    found.sort();
    assert!(!found.is_empty(), "no sources were found to check");
    found
}

/// The source lines mentioning any of `tokens`, named by file and line.
fn mentions(tokens: &[&str]) -> Vec<String> {
    let mut found = Vec::new();
    for (path, text) in sources() {
        for (index, line) in text.lines().enumerate() {
            // A comment is prose about the policy rather than a use of it, and
            // these tests are themselves reasons to write such comments.
            if line.trim_start().starts_with("//") {
                continue;
            }
            for token in tokens {
                if line.contains(token) {
                    found.push(format!("{path}:{}: {}", index + 1, line.trim()));
                }
            }
        }
    }
    found
}

#[test]
fn the_tool_reaches_no_network_including_loopback() {
    // Stricter than the layer rule it sits in. `end-to-end-testing.md` permits
    // an integration test to own a loopback socket, and that permission is
    // about test *plumbing*. This is a claim about the *product*: a repository
    // validator that can open a socket at all is one a maintainer has to trust
    // rather than reason about, and loopback is not an exception, because a
    // local port is exactly where an unwanted collaborator would listen.
    let found = mentions(&[
        "std::net",
        "TcpStream",
        "TcpListener",
        "UdpSocket",
        "SocketAddr",
        "ToSocketAddrs",
    ]);
    assert!(
        found.is_empty(),
        "the tool reaches the network:\n{}",
        found.join("\n")
    );
}

#[test]
fn no_dependency_can_reach_the_network() {
    // The source check above is necessary and not sufficient: a dependency can
    // open a socket on the tool's behalf without the tool naming one. Checked
    // against the lock file, so a transitive arrival is caught as well as a
    // declared one.
    let lock = std::fs::read_to_string(crate_root().join("Cargo.lock"))
        .expect("the lock file is committed and readable");
    // A positive control, because the interesting result here is an absence
    // and an absence is what a broken search returns too. `globset` is a
    // declared dependency, so a search that cannot find it would report every
    // network-capable crate absent as well -- and a mutant that adds one to
    // the lock file cannot prove otherwise, since cargo regenerates the lock
    // before the test runs and prunes the entry.
    assert!(
        lock.contains(&format!("name = \"{}\"", "globset")),
        "the lock-file search cannot find a crate that is certainly present"
    );
    let capable = [
        "tokio",
        "hyper",
        "reqwest",
        "ureq",
        "curl",
        "socket2",
        "mio",
        "rustls",
        "native-tls",
        "openssl",
        "async-std",
        "isahc",
        "surf",
    ];
    let found: Vec<&str> = capable
        .into_iter()
        .filter(|crate_name| lock.contains(&format!("name = \"{crate_name}\"")))
        .collect();
    assert!(
        found.is_empty(),
        "the dependency graph can reach the network: {found:?}"
    );
}

/// The one module allowed to start a process, and the only reason it exists.
const LAUNCHER: &str = "src/runtime/disk.rs";

#[test]
fn no_validator_spawns_a_child_process() {
    // `build.rs` runs `git rev-parse` and the E2E adapter spawns the binary;
    // neither is under `src/`, and neither is in the shipped tool. What this
    // forbids is a *validator* shelling out at inspection time, which would
    // make its result depend on what happens to be installed on the machine.
    //
    // Gate dispatch is the one thing here that is not inspection: running a
    // repository's declared gates *is* starting children, and a runner that
    // could not would be a runner in name only. That capability is confined to
    // one module and reached through one port, and the rule below is the
    // confinement rather than a waiver of it.
    let found: Vec<String> =
        mentions(&["std::process::Command", "Command::new", "process::Command"])
            .into_iter()
            .filter(|finding| !finding.starts_with(LAUNCHER))
            .collect();
    assert!(
        found.is_empty(),
        "a module that is not the launcher spawns a child process:\n{}",
        found.join("\n")
    );
}

#[test]
fn the_launcher_is_reached_only_by_gate_dispatch() {
    // Confinement is worth nothing if every validator can call it. The port is
    // named in the module that defines it, the module that implements it, the
    // dispatcher that hands it along, and the gate runner that uses it -- and
    // nowhere else. A validator that named it could start a process without
    // ever writing `Command`.
    const ALLOWED: [&str; 5] = [
        "src/runtime.rs",
        LAUNCHER,
        "src/lib.rs",
        "src/main.rs",
        "src/gate.rs",
    ];
    let found: Vec<String> = mentions(&["Launcher", "Launch {"])
        .into_iter()
        .filter(|finding| {
            !ALLOWED
                .iter()
                .any(|allowed| finding.starts_with(&format!("{allowed}:")))
        })
        .collect();
    assert!(
        found.is_empty(),
        "a module outside the gate dispatch reaches the launcher:\n{}",
        found.join("\n")
    );
}

#[test]
fn the_tool_contains_no_unsafe_code() {
    for entry in ["src/lib.rs", "src/main.rs"] {
        let text = std::fs::read_to_string(crate_root().join(entry))
            .expect("both crate roots are readable");
        assert!(
            text.contains("#![forbid(unsafe_code)]"),
            "{entry} does not forbid unsafe code"
        );
    }
    let found = mentions(&["unsafe "]);
    assert!(
        found.is_empty(),
        "the tool contains unsafe code:\n{}",
        found.join("\n")
    );
}

#[test]
fn no_leaf_writes_to_the_repository_it_inspects() {
    // The corpus proves this per scenario, for the leaves its scenarios name.
    // This proves it for *every* leaf, over a tree that is writable and that
    // each of them has something to say about -- including the leaves whose
    // scenarios never happen to run against a repository at all.
    //
    // What it proves is that nothing was written *through the root*. It cannot
    // see a write to a relative path, because `execute` runs in this process
    // and such a write lands in the crate root; `tests/e2e/policy.rs` is where
    // that case is proved, because only there is the inspected repository also
    // the running process's working directory.
    let mut world: World<()> = World::default();
    world.files.insert(
        "rules/README.md".to_string(),
        "# Rules\n\n## Directory Map\n\nNo other entries.\n".to_string(),
    );
    let repository = world.repository();
    let sandbox = Sandbox::build(&repository);
    let tree = DiskTree::new(sandbox.root()).expect("the sandbox root is a directory");

    for leaf in LEAVES {
        let arguments: Vec<String> = leaf.iter().map(|part| (*part).to_string()).collect();
        let before = sandbox::observe(sandbox.root());
        let outcome = rhino::execute(&tree, &arguments);
        let after = sandbox::observe(sandbox.root());
        let changed = differences(&before, &after);
        assert!(
            changed.is_empty(),
            "`rhino {}` changed the repository it inspected: {changed:?}\nstderr: {}",
            leaf.join(" "),
            outcome.stderr
        );
    }
}

#[test]
fn listing_one_directory_does_not_walk_the_repository() {
    // A cost claim rather than a correctness one, and it is here rather than in
    // the corpus because a scenario can only say what the answer is, never what
    // it cost to get. `Tree::children` has a default that derives the answer
    // from `files()` -- a full recursive walk -- which is correct for any
    // implementation and affordable only for the in-memory one. `directory-map`
    // asks it once per mapped directory, so on a real repository the default
    // walks the whole tree once per question: measured on a 49-map tree, that
    // was 1.07 s against 33 ms for every other leaf, and the cost grew with the
    // *repository*, not with what was being inspected.
    //
    // Checked against the source in the same way the absences above are,
    // because no single run can show that the next one does not walk.
    let text = std::fs::read_to_string(crate_root().join("src/runtime/disk.rs"))
        .expect("the disk tree is readable");
    // A positive control, so a search that had stopped working could not report
    // the override missing by failing to find anything at all.
    assert!(
        text.contains("fn files("),
        "the source search cannot find a method that is certainly present"
    );
    assert!(
        text.contains("fn children("),
        "DiskTree does not override `children`, so listing one directory walks the whole repository"
    );
    assert!(
        !text.contains("self.files()"),
        "DiskTree answers a question about one directory by walking the whole repository"
    );
}

#[test]
fn a_directory_listing_answers_exactly_what_a_walk_would() {
    // The override above may not change the answer, and three of the things it
    // must keep are not obvious from the signature: a filesystem link is not a
    // child, an excluded directory is not a child, and a directory holding no
    // files is not a child at all -- the last because the port exposes files,
    // and a map cannot be missing from a directory that holds nothing.
    //
    // Written as a differential against the definition the default derives from
    // `files()`, so it states the equivalence rather than restating a list.
    let mut world: World<()> = World::default();
    for path in [
        "rules/README.md",
        "rules/one/README.md",
        "rules/one/deep/note.md",
        "rules/two.md",
        "rules/skipped/note.md",
        "elsewhere/other.md",
    ] {
        world.files.insert(
            path.to_string(),
            "# Note
"
            .to_string(),
        );
    }
    let repository = world.repository();
    let sandbox = Sandbox::build(&repository);
    std::fs::create_dir_all(sandbox.root().join("rules/hollow"))
        .expect("an empty directory can be created");
    std::os::unix::fs::symlink(
        sandbox.root().join("elsewhere"),
        sandbox.root().join("rules/linked"),
    )
    .expect("a link can be created");

    let mut tree = DiskTree::new(sandbox.root()).expect("the sandbox root is a directory");
    tree.exclude(&["skipped".to_string()]);

    // What the default would say, computed here from the file list.
    let expected = expected_children(&tree, "rules");
    assert_eq!(
        tree.children("rules"),
        expected,
        "the directory listing disagrees with the walk it replaces"
    );
    assert_eq!(
        expected,
        vec![
            "rules/README.md".to_string(),
            "rules/one".to_string(),
            "rules/two.md".to_string(),
        ],
        "the fixture no longer exercises links, exclusions, and empty directories"
    );
}

/// The children of `directory` as derived from the file list, which is the
/// definition `Tree::children` documents and every implementation must match.
fn expected_children(tree: &dyn Tree, directory: &str) -> Vec<String> {
    let prefix = format!("{directory}/");
    let mut seen: std::collections::BTreeSet<String> = std::collections::BTreeSet::new();
    for path in tree.files() {
        let Some(rest) = path.strip_prefix(&prefix) else {
            continue;
        };
        match rest.split_once('/') {
            Some((child, _)) => seen.insert(format!("{prefix}{child}")),
            None => seen.insert(path.clone()),
        };
    }
    seen.into_iter().collect()
}

#[test]
fn a_path_leaving_the_root_is_refused() {
    // Refused rather than resolved-and-then-checked: the tool never opens the
    // file at all, so a root it was pointed at is the whole of what it can
    // read even when the path came from configuration rather than a caller.
    let mut world: World<()> = World::default();
    world
        .files
        .insert("rules/README.md".to_string(), "# Rules\n".to_string());
    let repository = world.repository();
    let sandbox = Sandbox::build(&repository);
    let tree = DiskTree::new(sandbox.root()).expect("the sandbox root is a directory");

    for escaping in ["../secret.md", "rules/../../secret.md", "../"] {
        assert!(
            tree.read(escaping).is_err(),
            "`{escaping}` was read from outside the root"
        );
        assert!(
            !tree.is_directory(escaping),
            "`{escaping}` was resolved to a directory outside the root"
        );
        assert!(
            !tree.exists(escaping),
            "`{escaping}` was found outside the root"
        );
    }
}

#[test]
fn a_symbolic_link_out_of_the_root_is_not_followed() {
    // A link is skipped rather than read through, so a repository that points
    // at a file it does not own cannot have findings reported against that
    // file -- nor the file's contents reported back through the tool's output.
    let mut world: World<()> = World::default();
    world
        .files
        .insert("rules/README.md".to_string(), "# Rules\n".to_string());
    world
        .files
        .insert("rules/elsewhere.md".to_string(), String::new());
    world.links.insert("rules/elsewhere.md".to_string());
    let repository = world.repository();
    let sandbox = Sandbox::build(&repository);
    let tree = DiskTree::new(sandbox.root()).expect("the sandbox root is a directory");

    assert!(
        !tree.files().contains(&"rules/elsewhere.md".to_string()),
        "the walk followed a link out of the repository: {:?}",
        tree.files()
    );
}

/// A tree that records what was asked of it, so a cost claim can be made about
/// what a validator read rather than about how long a run happened to take.
///
/// A decorator over the real implementation rather than a mock: the numbers
/// mean nothing unless the run underneath them is the run the product does.
/// `excluding` and `rooted_at` rewrap, because a counter the product drops
/// halfway through would report every later read as never having happened.
struct CountingTree {
    inner: Box<dyn Tree>,
    reads: std::rc::Rc<std::cell::RefCell<Vec<String>>>,
}

impl Tree for CountingTree {
    fn read(&self, path: &str) -> Result<String, rhino::runtime::TreeError> {
        self.reads.borrow_mut().push(path.to_string());
        self.inner.read(path)
    }

    fn files(&self) -> Vec<String> {
        self.inner.files()
    }

    fn children(&self, directory: &str) -> Vec<String> {
        self.inner.children(directory)
    }

    fn is_directory(&self, path: &str) -> bool {
        self.inner.is_directory(path)
    }

    fn excluding(&self, directories: &[String]) -> Box<dyn Tree> {
        Box::new(CountingTree {
            inner: self.inner.excluding(directories),
            reads: std::rc::Rc::clone(&self.reads),
        })
    }

    fn rooted_at(&self, path: &str) -> Result<Box<dyn Tree>, String> {
        Ok(Box::new(CountingTree {
            inner: self.inner.rooted_at(path)?,
            reads: std::rc::Rc::clone(&self.reads),
        }))
    }
}

#[test]
fn harness_parity_reads_only_what_it_could_use() {
    // A cost claim, and the reason it is here rather than in the corpus is the
    // one above: a scenario can say what the answer is, never what it cost.
    //
    // `harness parity` is the only leaf that reads every file rather than every
    // file of a declared kind, and nothing it checks can be answered by a file
    // that is neither Markdown, nor under a declared canonical or adapter root,
    // nor named by a prohibited glob. Measured on a real repository, that was
    // 4,649 files and 98.9 MB read to use 203 files and 1.9 MB -- compiled
    // artifacts, dialyzer tables, and a database backup, opened once per run to
    // be discarded as soon as they were found not to be text.
    let mut world: World<()> = World::default();
    for (path, body) in harness::valid_contract(&world.declaration) {
        world.files.insert(path, body);
    }
    // One of each kind the run has no use for, so a fix that narrowed by
    // extension alone and a fix that narrowed by directory alone are both
    // caught.
    for path in ["assets/logo.svg", "build/output.o", "data/backup.sqlite3"] {
        world
            .files
            .insert(path.to_string(), "not a document\n".to_string());
    }

    let repository = world.repository();
    let sandbox = Sandbox::build(&repository);
    let reads = std::rc::Rc::new(std::cell::RefCell::new(Vec::new()));
    let tree = CountingTree {
        inner: Box::new(DiskTree::new(sandbox.root()).expect("the sandbox root is a directory")),
        reads: std::rc::Rc::clone(&reads),
    };

    let arguments: Vec<String> = ["harness", "parity", "validate"]
        .iter()
        .map(|part| (*part).to_string())
        .collect();
    let outcome = rhino::execute(&tree, &arguments);
    assert_eq!(
        outcome.exit_code, 0,
        "the fixture is meant to be a clean contract: {}{}",
        outcome.stdout, outcome.stderr
    );

    let read: std::collections::BTreeSet<String> = reads.borrow().iter().cloned().collect();
    // A positive control, so a counter that had stopped recording could not
    // report every file unread and pass.
    assert!(
        read.contains(harness::INSTRUCTION),
        "the counter did not see the canonical instruction body being read"
    );
    for path in ["assets/logo.svg", "build/output.o", "data/backup.sqlite3"] {
        assert!(
            !read.contains(path),
            "`harness parity` read `{path}`, which no declaration names and no rule here is about"
        );
    }
}
