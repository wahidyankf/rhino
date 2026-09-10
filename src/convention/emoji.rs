//! Emoji-prohibition validation.
//!
//! A repository declares which files may not carry an emoji code point. Only
//! the prohibition is checked, and deliberately: whether an emoji belongs in a
//! particular sentence is a judgement about meaning, and a validator that
//! guessed at it would be reporting on taste. Where an emoji is *forbidden* --
//! a configuration file, a shell script, a data document a machine reads --
//! the question has one answer and a validator can give it.
//!
//! This is the one validator that reads files a Markdown corpus never sees, so
//! it walks the repository itself rather than borrowing that corpus.

use crate::config::{Config, Emoji};
use crate::report::{Detail, Finding, Report};
use crate::runtime::{Tree, TreeError};
use crate::scan;

pub fn validate(tree: &dyn Tree, config: &Config, emoji: &Emoji) -> Report {
    let prohibited = match scan::glob_set(
        "convention-emoji.prohibited",
        emoji.prohibited.iter().map(|surface| surface.glob.as_str()),
    ) {
        Ok(prohibited) => prohibited,
        Err(reason) => return Report::refused("emoji", reason),
    };

    let mut report = Report::new("emoji", "file");

    for path in scan::files(tree, config) {
        if !prohibited.is_match(&path) {
            continue;
        }
        let text = match tree.read(&path) {
            Ok(text) => text,
            // A file that vanished between the walk and the read is not this
            // repository's policy being violated.
            Err(TreeError::NotFound) => continue,
            // Refusal follows the claim, as it does in the Markdown corpus.
            // The repository declared this path as one to inspect; a path it
            // pointed at and RHINO could not read leaves the prohibition
            // unchecked, and an unchecked prohibition must not report clean.
            Err(TreeError::Unreadable(reason)) => {
                return Report::refused("emoji", format!("{path}: {reason}"));
            }
            Err(TreeError::NotText) => {
                return Report::refused("emoji", format!("{path}: holds no text"));
            }
        };
        report.scanned(&path);

        // One finding per line rather than per code point: three emoji on one
        // line are one edit, and three identical findings at one position read
        // as a defect in the report.
        for (number, line) in text.lines().enumerate() {
            if let Some(character) = line.chars().find(|c| is_emoji(*c)) {
                report.found(
                    Finding::new(
                        "emoji-in-prohibited-file",
                        &path,
                        "holds an emoji code point",
                    )
                    .at_line(number + 1)
                    .with(
                        "codePoint",
                        Detail::Text(format!("U+{:04X}", character as u32)),
                    ),
                );
            }
        }
    }

    report
}

/// The code-point blocks an emoji comes from, each named.
///
/// Named rather than inlined because the boundaries are the whole rule: an
/// em dash, an arrow, a copyright sign, and a plus-minus sign all live just
/// outside them, and every one of those appears in ordinary prose that this
/// validator must not accuse. The list is deliberately narrower than "every
/// symbol": a false finding in a configuration file costs a maintainer more
/// than a missed decorative glyph does.
const BLOCKS: [(char, char, &str); 5] = [
    (
        '\u{1F000}',
        '\u{1FAFF}',
        "pictographs, transport, flags, and supplemental symbols",
    ),
    ('\u{2600}', '\u{27BF}', "miscellaneous symbols and dingbats"),
    (
        '\u{2B00}',
        '\u{2BFF}',
        "miscellaneous symbols and arrows used as emoji",
    ),
    (
        '\u{FE00}',
        '\u{FE0F}',
        "variation selectors, which only ever decorate one",
    ),
    (
        '\u{3297}',
        '\u{3299}',
        "the enclosed CJK ideographs used as emoji",
    ),
];

fn is_emoji(character: char) -> bool {
    BLOCKS
        .iter()
        .any(|(first, last, _)| (*first..=*last).contains(&character))
}
