//! Front-matter validation.
//!
//! A repository declares which files carry front matter and what it must say:
//! which keys are required, which hold a value from a closed set, which hold a
//! date, and which may not appear. RHINO holds none of those lists. What it
//! holds is the reading -- where a front-matter block begins and ends, and what
//! counts as a top-level key -- because that is a fact about the format rather
//! than a policy about the repository.

use crate::config::{Config, Frontmatter, FrontmatterSurface};
use crate::report::{Detail, Finding, Report};
use crate::runtime::Tree;
use crate::scan::{Corpus, Surfaces};

/// The line a front-matter block opens and closes with.
const FENCE: &str = "---";

/// One top-level declaration, with the line a reader will find it on.
struct Entry {
    line: usize,
    key: String,
    value: String,
}

/// A document's front matter as read.
enum Head {
    /// A block that opened and never closed. Everything after the opener reads
    /// as front matter to the end of the file, so no key in it can be trusted
    /// -- reporting the keys instead of the fence would tell a maintainer to
    /// fix the wrong thing.
    Unterminated,
    /// The declarations found, which is empty both for a document with an empty
    /// block and for one with no block at all. The two are the same to every
    /// rule here: a required key is absent in both.
    Entries(Vec<Entry>),
}

pub fn validate(tree: &dyn Tree, config: &Config, frontmatter: &Frontmatter) -> Report {
    let surfaces = match Surfaces::compile(
        "md-frontmatter.surfaces",
        frontmatter
            .surfaces
            .iter()
            .map(|surface| surface.glob.as_str()),
    ) {
        Ok(surfaces) => surfaces,
        Err(reason) => return Report::refused("frontmatter", reason),
    };
    let corpus = match Corpus::read(tree, config) {
        Ok(corpus) => corpus,
        Err(reason) => return Report::refused("frontmatter", reason),
    };

    let mut report = Report::new("frontmatter", "file");

    for document in corpus.documents() {
        let Some(surface) = surfaces
            .governing(&document.path)
            .and_then(|index| frontmatter.surfaces.get(index))
        else {
            continue;
        };
        report.scanned(&document.path);
        inspect(&document.path, &document.text, surface, &mut report);
    }

    report
}

fn inspect(path: &str, text: &str, surface: &FrontmatterSurface, report: &mut Report) {
    let entries = match head(text) {
        Head::Unterminated => {
            report.found(
                Finding::new(
                    "unterminated-frontmatter",
                    path,
                    "front matter opens and never closes",
                )
                .at_line(1),
            );
            return;
        }
        Head::Entries(entries) => entries,
    };

    let held = |name: &str| entries.iter().find(|entry| entry.key == name);

    for key in &surface.require {
        if held(key).is_none() {
            // Line 1, because the fault is the absence and the absence has no
            // position of its own; the file's first line is where a reader
            // starts looking for it.
            report.found(
                Finding::new(
                    "missing-frontmatter-key",
                    path,
                    format!("front matter declares no `{key}`"),
                )
                .at_line(1),
            );
        }
    }

    for key in &surface.forbid {
        if let Some(entry) = held(key) {
            report.found(
                Finding::new(
                    "forbidden-frontmatter-key",
                    path,
                    format!("`{key}` is a key this surface forbids"),
                )
                .at_line(entry.line),
            );
        }
    }

    // A closed set and a date are both rules about a value that is there. A key
    // that is absent is `require`'s business, and reporting it twice would make
    // one fault read as two.
    for (key, permitted) in &surface.values {
        if let Some(entry) = held(key)
            && !permitted.contains(&entry.value)
        {
            report.found(
                Finding::new(
                    "invalid-frontmatter-value",
                    path,
                    format!("`{key}` holds a value outside its declared set"),
                )
                .at_line(entry.line)
                .with("value", Detail::Text(entry.value.clone())),
            );
        }
    }

    for key in &surface.iso_date {
        if let Some(entry) = held(key)
            && !is_iso_date(&entry.value)
        {
            report.found(
                Finding::new(
                    "invalid-frontmatter-value",
                    path,
                    format!("`{key}` does not hold an ISO calendar date"),
                )
                .at_line(entry.line)
                .with("value", Detail::Text(entry.value.clone())),
            );
        }
    }
}

/// A document's front matter, read from its first line.
///
/// The block has to be the first thing in the file. A fence further down is
/// a thematic break or a table rule, and treating one as front matter would
/// invent a schema violation in the middle of a document.
fn head(text: &str) -> Head {
    let mut lines = text.lines().enumerate();
    match lines.next() {
        Some((_, first)) if first.trim_end() == FENCE => {}
        _ => return Head::Entries(Vec::new()),
    }

    let mut entries = Vec::new();
    for (index, line) in lines {
        if line.trim_end() == FENCE {
            return Head::Entries(entries);
        }
        if let Some(entry) = declaration(index + 1, line) {
            entries.push(entry);
        }
    }

    Head::Unterminated
}

/// One top-level declaration, or `None` for anything that is not one.
///
/// Indented lines belong to the value above them: a key's sequence items and a
/// nested mapping are part of that key, not further keys. Reading them as
/// top-level would let a nested `updated:` trip a rule about the document's own
/// keys.
fn declaration(line: usize, text: &str) -> Option<Entry> {
    if text.starts_with(char::is_whitespace) || text.trim_start().starts_with('#') {
        return None;
    }
    let (key, value) = text.split_once(':')?;
    let key = key.trim();
    (!key.is_empty()).then(|| Entry {
        line,
        key: key.to_string(),
        value: unquoted(value.trim()).to_string(),
    })
}

/// A scalar without the quotes a document may or may not have written it with.
///
/// The two spellings are the same value to YAML, so a repository that quoted
/// one entry and not the next must not get two different answers about whether
/// it is in a declared set.
fn unquoted(value: &str) -> &str {
    for quote in ['"', '\''] {
        if let Some(inner) = value
            .strip_prefix(quote)
            .and_then(|rest| rest.strip_suffix(quote))
        {
            return inner;
        }
    }
    value
}

/// `YYYY-MM-DD`, and a date the calendar actually has.
///
/// The calendar check is the point: a well-shaped `2026-02-30` is the failure a
/// pattern match cannot see, and it is exactly what a hand-maintained date key
/// eventually holds.
fn is_iso_date(value: &str) -> bool {
    let mut parts = value.split('-');
    let (Some(year), Some(month), Some(day), None) =
        (parts.next(), parts.next(), parts.next(), parts.next())
    else {
        return false;
    };
    if year.len() != 4 || month.len() != 2 || day.len() != 2 {
        return false;
    }
    // Digits only. `u32` accepts a leading `+`, and `2026-+2-01` is not a date
    // any reader would recognise.
    if ![year, month, day]
        .iter()
        .all(|part| part.bytes().all(|byte| byte.is_ascii_digit()))
    {
        return false;
    }
    let (year, month, day) = (number(year), number(month), number(day));
    (1..=12).contains(&month) && day >= 1 && day <= days_in(year, month)
}

/// A run of ASCII digits as the number it spells.
///
/// Total rather than fallible: the caller has already established that every
/// part is two or four ASCII digits, and a fallible parse here would add a
/// branch nothing can reach.
fn number(digits: &str) -> u32 {
    digits
        .bytes()
        .fold(0, |value, digit| value * 10 + u32::from(digit - b'0'))
}

fn days_in(year: u32, month: u32) -> u32 {
    match month {
        1 | 3 | 5 | 7 | 8 | 10 | 12 => 31,
        4 | 6 | 9 | 11 => 30,
        _ if year % 4 == 0 && (year % 100 != 0 || year % 400 == 0) => 29,
        _ => 28,
    }
}
