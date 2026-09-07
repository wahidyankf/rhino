//! Internal-link validation.
//!
//! Resolves local document targets and nothing else. An external URL is
//! recognized and skipped -- never fetched, never reported on -- which is what
//! the command name promises and what keeps the whole tool network-free.

use crate::config::Config;
use crate::markdown;
use crate::report::{Finding, Report};
use crate::runtime::Tree;
use crate::scan::{self, Corpus};
use regex::Regex;
use std::sync::LazyLock;

/// `[text](destination)`, with an optional title the destination stops before.
///
/// The opening bracket is part of the match so that the character before it can
/// be inspected: `![alt](src)` is an image, and an image is not a document link.
/// Rust's `regex` refuses lookbehind by design -- deliberately, since backtracking
/// is what makes a pattern hang on content the tool does not author -- so the
/// exclusion is a one-character check at the match site instead.
static INLINE: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r#"\[[^\]]*\]\(\s*<?([^)<>\s]*)>?(?:\s+"[^"]*")?\s*\)"#)
        .expect("the inline-link pattern compiles")
});

/// `[label]: destination`, the reference-style definition. The definition is
/// what carries the target, so it is what gets checked.
static DEFINITION: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"^\s{0,3}\[[^\]]+\]:\s*<?([^>\s]+)>?").expect("the definition pattern compiles")
});

/// Why a local target is not reachable.
enum Fault {
    Malformed,
    Outside,
    Missing,
}

impl Fault {
    fn kind(&self) -> &'static str {
        match self {
            Self::Malformed => "internal-link-malformed",
            Self::Outside => "internal-link-outside-repository",
            Self::Missing => "internal-link-missing",
        }
    }

    fn message(&self, destination: &str) -> String {
        let readable = destination.replace('\0', "\\0");
        match self {
            Self::Malformed => format!("`{readable}` is not a usable path"),
            Self::Outside => format!("`{readable}` resolves outside the repository"),
            Self::Missing => format!("`{readable}` does not exist"),
        }
    }
}

pub fn validate(tree: &dyn Tree, config: &Config) -> Report {
    let excluded = match scan::glob_set(
        "md-internal-link.exclude-sources",
        &config.internal_link.exclude_sources,
    ) {
        Ok(set) => set,
        Err(reason) => return Report::refused("internal-link", reason),
    };
    let corpus = match Corpus::read(tree, config) {
        Ok(corpus) => corpus,
        Err(reason) => return Report::refused("internal-link", reason),
    };

    let mut report = Report::new("internal-link", "link");
    let mut inspected = 0usize;

    // An excluded source is not read for links, but stays in the corpus as a
    // perfectly valid target for links elsewhere.
    for document in corpus.sources(&excluded) {
        for (line, destination) in destinations(&document.text) {
            let Some(target) = local(&destination) else {
                continue;
            };
            inspected += 1;
            // Inspection continues past a fault: a malformed target in one file
            // must not hide a missing one in the next.
            if let Some(fault) = resolve(tree, &document.path, &target) {
                report.found(
                    Finding::new(fault.kind(), &document.path, fault.message(&destination))
                        .at_line(line),
                );
            }
        }
    }

    report.inspected(inspected);
    report
}

/// Every link destination in a document, with the line it appears on.
fn destinations(text: &str) -> Vec<(usize, String)> {
    let mut found = Vec::new();
    for (line, content) in markdown::prose_lines(text) {
        for capture in INLINE.captures_iter(content) {
            let whole = capture.get(0).expect("a match has a zero group");
            if content[..whole.start()].ends_with('!') {
                continue;
            }
            found.push((line, capture[1].to_string()));
        }
        if let Some(capture) = DEFINITION.captures(content) {
            found.push((line, capture[1].to_string()));
        }
    }
    found
}

/// The repository-relative part of a destination, or `None` when the
/// destination is not this repository's to resolve.
///
/// A same-document fragment addresses nothing on disk, and a destination with a
/// scheme belongs to someone else's server -- both are skipped rather than
/// reported, because reporting on them is what "internal link" excludes.
fn local(destination: &str) -> Option<String> {
    if destination.is_empty() || destination.starts_with('#') || has_scheme(destination) {
        return None;
    }
    let without_fragment = destination.split('#').next().unwrap_or(destination);
    let path = without_fragment
        .split('?')
        .next()
        .unwrap_or(without_fragment);
    Some(path.to_string())
}

fn has_scheme(destination: &str) -> bool {
    let Some(position) = destination.find(':') else {
        return false;
    };
    let (scheme, _) = destination.split_at(position);
    !scheme.is_empty()
        && scheme.starts_with(|c: char| c.is_ascii_alphabetic())
        && scheme
            .chars()
            .all(|c| c.is_ascii_alphanumeric() || matches!(c, '+' | '.' | '-'))
}

fn resolve(tree: &dyn Tree, source: &str, target: &str) -> Option<Fault> {
    if target.contains('\0') {
        return Some(Fault::Malformed);
    }
    if target.starts_with('/') {
        // An absolute path is not repository-relative and names a location the
        // repository does not own, wherever it happens to resolve.
        return Some(Fault::Outside);
    }
    if target.is_empty() {
        return None;
    }

    let mut segments: Vec<&str> = source.split('/').collect();
    segments.pop();
    for segment in target.split('/') {
        match segment {
            "" | "." => {}
            ".." => {
                // Popping past the root is leaving the repository. Returning
                // `None` here would have read as "no fault", which is how this
                // was wrong the first time.
                if segments.pop().is_none() {
                    return Some(Fault::Outside);
                }
            }
            name => segments.push(name),
        }
    }

    let resolved = segments.join("/");
    if resolved.is_empty() || tree.exists(&resolved) {
        None
    } else {
        Some(Fault::Missing)
    }
}
