//! Word-budget validation.
//!
//! A repository declares which files are governed and how long each may be.
//! RHINO holds no opinion about either: it enforces what the configuration says
//! and reports what it counted, which is why a clean run still lists every path
//! it read. A surface that matched nothing and a surface that matched
//! everything both pass, and only the listing tells them apart.

use crate::Outcome;
use crate::config::Config;
use crate::report::{Detail, Finding, Report};
use crate::runtime::Tree;
use crate::scan::{self, Corpus};
use regex::Regex;
use std::sync::LazyLock;

/// A word is a run of letters, marks, and numbers, optionally joined by an
/// apostrophe, hyphen, or underscore to another such run.
///
/// That joining clause is the whole point: `can't-stop` is one word to a
/// reader, and a counter that made it three would report a budget nobody
/// recognises.
static WORD: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"[\p{L}\p{M}\p{N}]+(?:['\u{2019}_-][\p{L}\p{M}\p{N}]+)*")
        .expect("the word pattern compiles")
});

pub fn count(text: &str) -> usize {
    WORD.find_iter(text).count()
}

pub fn validate(tree: &dyn Tree, config: &Config) -> Outcome {
    let surfaces = &config.word_budget.surfaces;
    let globs = match scan::glob_set(
        "governance-word-budget.surfaces",
        &surfaces
            .iter()
            .map(|surface| surface.glob.clone())
            .collect::<Vec<_>>(),
    ) {
        Ok(globs) => globs,
        Err(reason) => return Report::refused("word-budget", reason),
    };
    let corpus = match Corpus::read(tree, config) {
        Ok(corpus) => corpus,
        Err(reason) => return Report::refused("word-budget", reason),
    };

    let mut report = Report::new("word-budget", "file");

    for document in corpus.documents() {
        // Last match wins. Where two surfaces cover one file, the later
        // declaration is the more specific intent -- that is how a repository
        // says "this tree, except that one file".
        let Some(surface) = globs
            .matches(&document.path)
            .into_iter()
            .max()
            .and_then(|index| surfaces.get(index))
        else {
            continue;
        };

        report.scanned(&document.path);

        let words = count(&document.text);
        if words > surface.fail {
            report.found(
                Finding::new(
                    "word-limit-exceeded",
                    &document.path,
                    "is longer than its declared word budget",
                )
                .with("words", Detail::Count(words))
                .with("limit", Detail::Count(surface.fail)),
            );
        }
    }

    report.finish()
}
