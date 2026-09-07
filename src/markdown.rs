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

/// Whether the document ends with a fence still open.
///
/// An unclosed block runs to the end of the file, so everything after the
/// opener reads as an example. That is the correct Markdown reading and a poor
/// basis for exempting anything: one stray opener -- a four-backtick block a
/// three-backtick line cannot close, or a fence someone forgot -- hides the
/// whole remainder of the document, and a reader who consumes the file as text
/// rather than as Markdown is not fooled by it.
pub fn ends_inside_a_fence(text: &str) -> bool {
    let mut open: Option<(char, usize)> = None;
    for line in text.lines() {
        match (open, fence(line)) {
            (None, Some(opening)) => open = Some(opening),
            (Some((character, length)), Some((closing, closing_length)))
                if closing == character && closing_length >= length =>
            {
                open = None;
            }
            _ => {}
        }
    }
    open.is_some()
}

/// A line with its inline code spans removed.
///
/// A span between backticks is shown rather than acted on, the same way a
/// fenced block is. The run length has to match, as it does for fences, so a
/// line demonstrating backticks inside backticks is not cut short -- and a run
/// with no partner is a literal backtick rather than an opener, so an
/// unbalanced line keeps the rest of its text.
pub fn without_code_spans(line: &str) -> String {
    let mut out = String::new();
    let mut rest = line;

    while let Some(start) = rest.find('`') {
        out.push_str(&rest[..start]);
        let opened = &rest[start..];
        let run = opened.chars().take_while(|c| *c == '`').count();
        let delimiter = "`".repeat(run);
        let body = &opened[run..];
        match body.find(&delimiter) {
            Some(end) => rest = &body[end + run..],
            None => {
                out.push_str(opened);
                return out;
            }
        }
    }

    out.push_str(rest);
    out
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
