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
    let prohibited = match scan::Surfaces::compile(
        "convention-emoji.prohibited",
        emoji.prohibited.iter().map(|surface| surface.glob.as_str()),
    ) {
        Ok(prohibited) => prohibited,
        Err(reason) => return Report::refused("emoji", reason),
    };

    let mut report = Report::new("emoji", "file");

    let reached = match prohibited.files(tree, config) {
        Ok(reached) => reached,
        Err(unreachable) => return unreachable.refusal("emoji"),
    };
    for scan::Reached { path, source } in reached {
        if prohibited.governing(&path).is_none() {
            continue;
        }
        let text = match tree.read(&source) {
            Ok(text) => text,
            // A file that vanished between the walk and the read is not this
            // repository's policy being violated.
            Err(TreeError::NotFound) => continue,
            // Refusal follows the claim, as it does in the Markdown corpus.
            // The repository declared this path as one to inspect; a path it
            // pointed at and RHINO could not read leaves the prohibition
            // unchecked, and an unchecked prohibition must not report clean.
            Err(TreeError::Unreadable(reason)) => {
                return Report::refused_as(
                    crate::errors::ErrorCode::FileUnreadable,
                    "emoji",
                    format!("{path}: {reason}"),
                );
            }
            Err(TreeError::NotText) => {
                return Report::refused_as(
                    crate::errors::ErrorCode::FileUnreadable,
                    "emoji",
                    format!("{path}: holds no text"),
                );
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

#[cfg(test)]
mod tests {
    use super::*;
    use crate::cli::Format;
    use crate::config::Globbed;
    use crate::runtime::MemoryTree;

    fn policy(glob: &str) -> Emoji {
        Emoji {
            prohibited: vec![Globbed {
                glob: glob.to_string(),
            }],
        }
    }

    #[test]
    fn declared_ranges_flag_emoji_but_not_neighbouring_prose_symbols() {
        for character in ['🦏', '✓', '⬆', '\u{FE0F}', '㊗'] {
            assert!(is_emoji(character), "{character}");
        }
        for character in ['—', '©', '+', 'A'] {
            assert!(!is_emoji(character), "{character}");
        }
    }

    #[test]
    fn a_prohibited_surface_follows_an_in_root_link_and_refuses_an_escaping_one() {
        let mut tree = MemoryTree::default();
        tree.write("config/a.json", "held \u{1F98F}\n");
        tree.mark_link_to("linked", "config");
        let followed =
            validate(&tree, &Config::default(), &policy("linked/**/*.json")).render(Format::Json);
        assert_eq!(followed.exit_code, 1);
        assert!(
            followed.stdout.contains("\"linked/a.json\""),
            "{}",
            followed.stdout
        );

        let mut escaping = MemoryTree::default();
        escaping.write("linked/a.json", "held");
        escaping.mark_link("linked");
        let refused = validate(&escaping, &Config::default(), &policy("linked/**/*.json"))
            .render(Format::Json);
        assert_eq!(refused.exit_code, 2);
        assert!(refused.stderr.contains("rhino.path.escapes-root"));
    }

    #[test]
    fn validator_walks_only_declared_files_and_distinguishes_read_faults() {
        let mut tree = MemoryTree::default();
        tree.write("config/a.json", "first 🦏\nsecond ✓\n");
        tree.write("notes/a.md", "🦏 is allowed here\n");
        let clean =
            validate(&tree, &Config::default(), &policy("config/**/*.json")).render(Format::Text);
        assert_eq!(clean.exit_code, 1);
        assert!(clean.stderr.contains("U+1F98F"));
        assert!(clean.stderr.contains("U+2713"));

        tree.mark_unreadable("config/a.json");
        assert_eq!(
            validate(&tree, &Config::default(), &policy("config/**/*.json"))
                .render(Format::Text)
                .exit_code,
            2
        );
        let mut binary = MemoryTree::default();
        binary.write("config/a.json", "bytes");
        binary.mark_binary("config/a.json");
        assert_eq!(
            validate(&binary, &Config::default(), &policy("config/**/*.json"))
                .render(Format::Text)
                .exit_code,
            2
        );
        let mut vanished = MemoryTree::default();
        vanished.write("config/a.json", "gone before validation");
        vanished.mark_vanished("config/a.json");
        assert_eq!(
            validate(&vanished, &Config::default(), &policy("config/**/*.json"))
                .render(Format::Text)
                .exit_code,
            0
        );
        assert_eq!(
            validate(&tree, &Config::default(), &policy("["))
                .render(Format::Text)
                .exit_code,
            2
        );
    }
}
