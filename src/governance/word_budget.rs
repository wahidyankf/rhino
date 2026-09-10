//! Word-budget validation.
//!
//! A repository declares which files are governed and how long each may be.
//! RHINO holds no opinion about either: it enforces what the configuration says
//! and reports what it counted, which is why a clean run still lists every path
//! it read. A surface that matched nothing and a surface that matched
//! everything both pass, and only the listing tells them apart.

use crate::config::{Config, WordBudget, WordRule};
use crate::report::{Detail, Finding, Report};
use crate::runtime::Tree;
use crate::scan::{self, Corpus, Scope};
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

pub fn count(text: &str, rule: WordRule) -> usize {
    match rule {
        WordRule::LettersAndDigits => WORD.find_iter(text).count(),
        WordRule::WhitespaceSeparated => text.split_whitespace().count(),
    }
}

pub fn validate(tree: &dyn Tree, config: &Config, budget: &WordBudget) -> Report {
    let surfaces = &budget.surfaces;
    let globs = match scan::Surfaces::compile(
        "governance-word-budget.surfaces",
        surfaces.iter().map(|surface| surface.glob.as_str()),
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
        let Some(surface) = globs
            .governing(&document.path)
            .and_then(|index| surfaces.get(index))
        else {
            continue;
        };

        report.scanned(&document.path);

        let words = count(&document.text, budget.count);
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

    report
}

/// Report a file's word count, and never a finding.
///
/// A separate leaf from the budget check on purpose: this one answers "how
/// long is this?" and has no policy to violate, so it cannot exit `1`. A
/// caller scripting around it can rely on that.
pub fn inspect(tree: &dyn Tree, budget: &WordBudget, scope: &Scope) -> Report {
    let mut report = Report::new("word-count", "file");

    if !scope.is_narrowed() {
        return Report::refused("word-count", "`--file` names what to count; none was given");
    }

    let mut total = 0usize;
    for (path, content) in scope.documents(tree) {
        let Some(text) = content else {
            return Report::refused("word-count", format!("{path}: cannot be read"));
        };
        let words = count(&text, budget.count);
        total += words;
        report.inspected_one();
        report.note(format!("{path}: {words} words"));
    }

    // The total rather than the last file's count, so the member means the same
    // thing whether one path was selected or four.
    report.measure("wordCount", total);
    report
}
