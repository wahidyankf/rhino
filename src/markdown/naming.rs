//! Filename validation.
//!
//! A repository declares which trees carry which filename style, and RHINO
//! holds neither the styles' membership nor their scope: the section is optional
//! and every surface is the repository's. What the tool does hold is the two
//! styles themselves, because a style is a rule about characters rather than a
//! policy about files -- and because the encoded style is derivable from the
//! path being walked, which is the whole reason a repository can declare it in
//! one line instead of listing every prefix it uses.

use crate::config::{Config, NameStyle, Naming, NamingSurface};
use crate::report::{Detail, Finding, Report};
use crate::runtime::Tree;
use crate::scan::{self, Surfaces};

const KIND: &str = "invalid-md-name";
const FRAGMENT: &str = "fragmented-md-name";

/// Name endings that say a file is half a document.
///
/// A closed list, and closed twice over: a name is judged against exactly these
/// shapes, and nothing beyond them is judged at all. Whether a name reads as
/// multi-topic or fragmentary in some other way is a review of what the
/// document says, and a validator answering it would be asserting that word
/// shapes prove meaning.
const FRAGMENTS: [&str; 3] = ["-part-", "-continuation-", "-continued"];

pub fn validate(tree: &dyn Tree, config: &Config, naming: &Naming) -> Report {
    let globs = match Surfaces::compile(
        "md-naming.surfaces",
        naming.surfaces.iter().map(|surface| surface.glob.as_str()),
    ) {
        Ok(globs) => globs,
        Err(reason) => return Report::refused("naming", reason),
    };
    let exempt = match scan::glob_set("md-naming.exempt", naming.exempt.iter().map(String::as_str))
    {
        Ok(exempt) => exempt,
        Err(reason) => return Report::refused("naming", reason),
    };

    // Resolved once per surface rather than once per file: a surface's root
    // depends on its glob, and on nothing about the file being walked.
    let roots: Vec<String> = naming
        .surfaces
        .iter()
        .map(|surface| literal_root(&surface.glob))
        .collect();

    let mut report = Report::new("naming", "file");

    // Names are read, not opened. A validator about filenames that refused an
    // unreadable file would be refusing on a fact it never needed.
    for path in scan::markdown_files(tree, config) {
        // Checked before the surfaces and through the exemptions, because this
        // is not a style. An exemption says "this file does not follow the
        // declared style", not "this file may be half a document", and a
        // hard-fail a repository could exempt itself from is not one.
        if let Some(finding) = fragment(&path) {
            report.found(finding);
        }
        // Exemption wins over every surface. A repository saying "not this
        // file" has said so about all of them at once.
        if exempt.is_match(&path) {
            continue;
        }
        let Some(index) = globs.governing(&path) else {
            continue;
        };
        report.scanned(&path);
        if let Some(finding) = inspect(&path, &naming.surfaces[index], &roots[index]) {
            report.found(finding);
        }
    }

    report
}

/// Whether a name ends in one of the closed mechanical fragment shapes.
///
/// `-part-` and `-continuation-` take a number and `-continued` takes nothing,
/// which is why the ending is tested rather than the substring: `-parted` and
/// `-part-two` are ordinary names, and a rule that matched them would be
/// guessing at what the words mean.
fn fragment(path: &str) -> Option<Finding> {
    let file = path.rsplit_once('/').map_or(path, |(_, file)| file);
    let name = file.rsplit_once('.').map_or(file, |(stem, _)| stem);
    let fragmented = FRAGMENTS.iter().any(|shape| match *shape {
        "-continued" => name.ends_with(shape),
        _ => name
            .rsplit_once(shape)
            .is_some_and(|(_, tail)| !tail.is_empty() && tail.chars().all(|c| c.is_ascii_digit())),
    });
    fragmented.then(|| {
        Finding::new(
            FRAGMENT,
            path,
            "is named as a fragment; a document that outgrew its budget splits into an entrypoint and a companion set rather than into numbered halves",
        )
    })
}

fn inspect(path: &str, surface: &NamingSurface, root: &str) -> Option<Finding> {
    let (directory, file) = path.rsplit_once('/').unwrap_or(("", path));
    let name = file.rsplit_once('.').map_or(file, |(stem, _)| stem);

    match surface.style {
        NameStyle::KebabCase => (!is_kebab_case(name))
            .then(|| Finding::new(KIND, path, "is not named in the declared kebab-case style")),
        NameStyle::PathPrefixed => {
            // Total for any configuration that parsed: `config::check_semantics`
            // refuses a path-prefixed surface declaring no separator, so a
            // surface that reaches here has one.
            let separator = surface
                .separator
                .as_deref()
                .expect("a path-prefixed surface that parsed declares a separator");
            let expected = encoded_prefix(directory, root);
            let named = match name.split_once(separator) {
                Some((prefix, content)) => prefix == expected && is_kebab_case(content),
                // A file directly at the surface root has no directory to
                // encode, so it carries a content name and no separator.
                None => expected.is_empty() && is_kebab_case(name),
            };
            (!named).then(|| {
                Finding::new(
                    KIND,
                    path,
                    "is not named in the declared path-prefixed style",
                )
                .with("prefix", Detail::Text(expected.clone()))
            })
        }
    }
}

/// Lowercase alphanumeric runs joined by single hyphens.
fn is_kebab_case(name: &str) -> bool {
    !name.is_empty()
        && name.split('-').all(|part| {
            !part.is_empty()
                && part
                    .chars()
                    .all(|character| character.is_ascii_lowercase() || character.is_ascii_digit())
        })
}

/// The prefix a directory encodes to, relative to its surface's root.
///
/// Each directory level contributes one encoded segment and the levels are
/// joined by a hyphen, so a reader can recover the path from the filename and a
/// filename cannot collide with one from a sibling tree.
fn encoded_prefix(directory: &str, root: &str) -> String {
    let below = directory
        .strip_prefix(root)
        .unwrap_or(directory)
        .trim_matches('/');
    below
        .split('/')
        .filter(|segment| !segment.is_empty())
        .map(encode_segment)
        .collect::<Vec<_>>()
        .join("-")
}

/// One directory level: each hyphen-separated word encoded, concatenated.
fn encode_segment(segment: &str) -> String {
    segment
        .split('-')
        .filter(|word| !word.is_empty())
        .map(encode_word)
        .collect()
}

/// Two characters of a word, or one character and an underscore.
///
/// The underscore is what keeps a one-character word visible: without it
/// `f-sharp` and `fs-harp` would both encode to `fsh`, and a reader could not
/// tell which directory a filename came from -- which is the one thing the
/// prefix exists to say.
fn encode_word(word: &str) -> String {
    let mut characters = word.chars().flat_map(char::to_lowercase);
    // `encode_segment` drops empty words, so a word reaching here has a first
    // character.
    let first = characters
        .next()
        .expect("encode_segment yields no empty word");
    match characters.next() {
        None => format!("{first}_"),
        Some(second) => format!("{first}{second}"),
    }
}

/// The fixed leading path of a glob: every segment before the first one
/// carrying a metacharacter.
///
/// This is what a surface's prefixes are relative to. Deriving it from the glob
/// rather than asking for it separately means the two cannot disagree: a
/// repository that re-rooted its surface has re-rooted its prefixes with it.
fn literal_root(glob: &str) -> String {
    let mut segments: Vec<&str> = Vec::new();
    for segment in glob.split('/') {
        if segment.contains(['*', '?', '[', '{']) {
            break;
        }
        segments.push(segment);
    }
    segments.join("/")
}
