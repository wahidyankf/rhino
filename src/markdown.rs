//! Reading Markdown.
//!
//! Everything that inspects Markdown text lives here, so the answer to "is this
//! inside a fenced example?" is given once rather than once per validator.

pub mod internal_link;

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
