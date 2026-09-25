//! Heading-hierarchy validation.
//!
//! Two rules, both the repository's: whether a governed document holds exactly
//! one level-1 heading, and how far a heading may drop below the one before it.
//! RHINO supplies neither number, and the section is optional.
//!
//! What is not configurable is what counts as a heading. A `#` inside a fenced
//! block is a shell comment, a Markdown example, or a colour literal, and
//! reading one as a heading would report a structural fault in a document that
//! has none -- so the fence reader in [`crate::markdown`] decides that here, as
//! it does everywhere else.

use crate::config::{Config, HeadingHierarchy};
use crate::markdown;
use crate::report::{Detail, Finding, Report};
use crate::runtime::Tree;
use crate::scan::{Corpus, Surfaces};

/// The deepest ATX heading Markdown has.
const DEEPEST: usize = 6;

pub fn validate(tree: &dyn Tree, config: &Config, headings: &HeadingHierarchy) -> Report {
    let surfaces = match Surfaces::compile(
        "md-heading-hierarchy.surfaces",
        headings
            .surfaces
            .iter()
            .map(|surface| surface.glob.as_str()),
    ) {
        Ok(surfaces) => surfaces,
        Err(reason) => return Report::refused("heading-hierarchy", reason),
    };
    let corpus = match Corpus::read(tree, config) {
        Ok(corpus) => corpus,
        Err(reason) => return Report::unreadable("heading-hierarchy", reason),
    };

    let mut report = Report::new("heading-hierarchy", "file");

    for document in corpus.documents() {
        if surfaces.governing(&document.path).is_none() {
            continue;
        }
        report.scanned(&document.path);
        inspect(&document.path, &document.text, headings, &mut report);
    }

    report
}

fn inspect(path: &str, text: &str, rules: &HeadingHierarchy, report: &mut Report) {
    let found = headings(text);

    if rules.single_h1 {
        let mut tops = found.iter().filter(|(_, level)| *level == 1);
        if tops.next().is_none() {
            report
                .found(Finding::new("missing-h1", path, "declares no level-1 heading").at_line(1));
        }
        // Every one after the first, so a document with three reports two
        // extras rather than one ambiguous "there are too many".
        for (line, _) in tops {
            report.found(
                Finding::new("multiple-h1", path, "declares a second level-1 heading")
                    .at_line(*line),
            );
        }
    }

    // The first heading is the level the rest are measured from. It cannot drop
    // below something that is not there, and a document that opens deep is
    // either `missing-h1`'s business or nobody's.
    let mut previous: Option<usize> = None;
    for (line, level) in &found {
        if let Some(above) = previous
            && *level > above + rules.max_level_jump
        {
            report.found(
                Finding::new(
                    "heading-level-jump",
                    path,
                    format!("drops from level {above} to level {level}"),
                )
                .at_line(*line)
                .with("jump", Detail::Count(level - above)),
            );
        }
        previous = Some(*level);
    }
}

/// Every ATX heading in a document, as `(line, level)`.
///
/// A run of hashes is a heading only when a space follows it: `#hashtag` is a
/// word, and treating it as a heading would invent a level-1 heading in the
/// middle of a paragraph.
fn headings(text: &str) -> Vec<(usize, usize)> {
    markdown::prose_lines(text)
        .into_iter()
        .filter_map(|(line, content)| {
            let trimmed = content.trim_start();
            let level = trimmed.chars().take_while(|c| *c == '#').count();
            let rest = &trimmed[level..];
            (1..=DEEPEST)
                .contains(&level)
                .then(|| rest.starts_with(' ').then_some((line, level)))
                .flatten()
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::cli::Format;
    use crate::config::Globbed;
    use crate::runtime::MemoryTree;

    fn rules(glob: &str, single_h1: bool, jump: usize) -> HeadingHierarchy {
        HeadingHierarchy {
            surfaces: vec![Globbed {
                glob: glob.to_string(),
            }],
            single_h1,
            max_level_jump: jump,
        }
    }

    #[test]
    fn heading_reader_ignores_examples_and_only_recognizes_valid_atx_levels() {
        assert_eq!(
            headings("# top\n#no-space\n####### too deep\n```sh\n# fenced\n```\n  ## second\n"),
            vec![(1, 1), (7, 2)]
        );
    }

    #[test]
    fn validator_reports_each_extra_h1_and_only_excessive_downward_jumps() {
        let tree = MemoryTree::default();
        tree.write(
            "docs/a.md",
            "## starts deep\n# first\n# second\n#### skip\n## back up\n",
        );
        tree.write("outside/a.md", "### ignored\n");
        let strict =
            validate(&tree, &Config::default(), &rules("docs/*.md", true, 1)).render(Format::Text);
        assert_eq!(strict.exit_code, 1);
        assert!(strict.stderr.contains("declares a second level-1 heading"));
        assert!(strict.stderr.contains("drops from level 1 to level 4"));

        let missing = validate(&tree, &Config::default(), &rules("outside/*.md", true, 2))
            .render(Format::Text);
        assert_eq!(missing.exit_code, 1);
        assert!(missing.stderr.contains("declares no level-1 heading"));
        assert_eq!(
            validate(&tree, &Config::default(), &rules("[", false, 1))
                .render(Format::Text)
                .exit_code,
            2
        );
    }
}
