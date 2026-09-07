//! Reading Markdown.
//!
//! Everything that inspects Markdown text lives here, so the answer to "is this
//! inside a fenced example?" is given once rather than once per validator.

pub mod internal_link;
pub mod mermaid;

/// The lines of a document that are prose rather than a fenced example.
///
/// Returned as `(one_based_line, text)` so a finding can name a position in the
/// file a reader will open, not a position in a filtered copy of it.
///
/// A fence is closed by a fence of the same character and at least the same
/// length, which is what lets a document show a fenced block inside a fenced
/// block -- the common case being documentation about Markdown itself.
pub fn prose_lines(text: &str) -> Vec<(usize, &str)> {
    let mut open: Option<(char, usize)> = None;
    let mut lines = Vec::new();

    for (index, line) in text.lines().enumerate() {
        match (open, fence(line)) {
            (None, Some(opening)) => open = Some(opening),
            (Some((character, length)), Some((closing, closing_length)))
                if closing == character && closing_length >= length =>
            {
                open = None;
            }
            (None, None) => lines.push((index + 1, line)),
            _ => {}
        }
    }

    lines
}

/// The fence character and run length a line opens or closes with, if any.
fn fence(line: &str) -> Option<(char, usize)> {
    let trimmed = line.trim_start();
    let character = trimmed.chars().next().filter(|c| *c == '`' || *c == '~')?;
    let length = trimmed.chars().take_while(|c| *c == character).count();
    (length >= 3).then_some((character, length))
}

/// A fenced block, with the info string it opened with.
pub struct Fenced<'a> {
    pub info: &'a str,
    /// Each content line with the one-based line number it occupies in the
    /// document, so a finding names a position a reader can navigate to.
    pub lines: Vec<(usize, &'a str)>,
}

/// A fence being read: its character and run length, its info string, and the
/// content lines gathered so far.
type OpenFence<'a> = (char, usize, &'a str, Vec<(usize, &'a str)>);

/// Every fenced block in a document.
///
/// Nesting is resolved the same way as in `prose_lines`: a fence closes only on
/// the same character at the same length or longer, so a block demonstrating
/// fenced syntax does not truncate the block containing it.
pub fn fenced_blocks(text: &str) -> Vec<Fenced<'_>> {
    let mut blocks = Vec::new();
    let mut open: Option<OpenFence<'_>> = None;

    for (index, line) in text.lines().enumerate() {
        match (&mut open, fence(line)) {
            (None, Some((character, length))) => {
                let info = line.trim_start().trim_start_matches(character);
                open = Some((character, length, info.trim(), Vec::new()));
            }
            (Some((character, length, info, lines)), Some((closing, closing_length)))
                if closing == *character && closing_length >= *length =>
            {
                blocks.push(Fenced {
                    info,
                    lines: std::mem::take(lines),
                });
                open = None;
            }
            (Some((_, _, _, lines)), _) => lines.push((index + 1, line)),
            (None, None) => {}
        }
    }

    blocks
}
