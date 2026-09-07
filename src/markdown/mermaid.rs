//! Mermaid accessibility and legibility validation.
//!
//! Two questions about one artefact, deliberately answered by one command: a
//! diagram nobody can read because the contrast is wrong and a diagram nobody
//! can read because the label overflows are the same problem to a reader.
//!
//! The split between tool and policy runs through the middle of this module.
//! *Which* diagram syntaxes RHINO can parse is a property of the tool -- it can
//! only inspect what it understands, and no repository can configure that.
//! *Which* colours and label lengths are acceptable is a property of the
//! repository, and every one of them arrives from configuration.

use crate::Outcome;
use crate::config::Config;
use crate::markdown::{Fenced, fenced_blocks};
use crate::report::{Detail, Finding, Report};
use crate::runtime::Tree;
use crate::scan::Corpus;
use unicode_segmentation::UnicodeSegmentation;

const ACCESSIBILITY: &str = "mermaid-accessibility";
const LEGIBILITY: &str = "mermaid-legibility";

/// The diagram syntaxes this build can read well enough to judge.
///
/// A syntax absent from this list is skipped whole, not partially inspected: a
/// half-parsed diagram would produce findings about text the tool misread.
#[derive(Clone, Copy, PartialEq, Eq)]
enum Kind {
    Flow,
    Class,
    State,
    Entity,
    Requirement,
    Block,
}

fn kind_of(declaration: &str) -> Option<Kind> {
    match declaration {
        "flowchart" | "graph" => Some(Kind::Flow),
        "classDiagram" | "classDiagram-v2" => Some(Kind::Class),
        "stateDiagram" | "stateDiagram-v2" => Some(Kind::State),
        "erDiagram" => Some(Kind::Entity),
        "requirementDiagram" => Some(Kind::Requirement),
        "block" | "block-beta" => Some(Kind::Block),
        _ => None,
    }
}

/// One diagram, with each of its lines carrying the Markdown line it came from.
///
/// Positions are kept from the start rather than recomputed, because a finding
/// has to name a line in the file a reader will open -- not a line in the
/// diagram, which exists nowhere.
struct Diagram {
    kind: Kind,
    lines: Vec<(usize, String)>,
}

pub fn validate(tree: &dyn Tree, config: &Config) -> Outcome {
    let corpus = match Corpus::read(tree, config) {
        Ok(corpus) => corpus,
        Err(reason) => return Report::refused("mermaid", reason),
    };

    let mut report = Report::new("mermaid", "diagram");
    let mut inspected = 0usize;

    for document in corpus.documents() {
        for block in fenced_blocks(&document.text) {
            if block.info.trim() != "mermaid" {
                continue;
            }
            let Some(diagram) = read(&block) else {
                continue;
            };
            inspected += 1;
            for finding in inspect(&diagram, config) {
                report.found(finding.rename(&document.path));
            }
        }
    }

    report.inspected(inspected).finish()
}

/// Read a fenced block as a diagram, or decline it.
///
/// Declining covers both a block with no diagram declaration and one whose
/// declaration this build cannot parse. Neither is a finding: reporting on
/// syntax the tool does not understand would be reporting on its own ignorance.
fn read(block: &Fenced<'_>) -> Option<Diagram> {
    let mut lines: Vec<(usize, String)> = Vec::new();
    let mut in_front_matter = false;
    let mut kind = None;

    for (number, text) in &block.lines {
        let trimmed = text.trim();

        // Front matter is configuration for the renderer, not diagram source,
        // and the declaration sits after it.
        if trimmed == "---" {
            in_front_matter = !in_front_matter;
            lines.push((*number, (*text).to_string()));
            continue;
        }
        if kind.is_none() && !in_front_matter && !trimmed.is_empty() && !trimmed.starts_with("%%") {
            let declaration = trimmed.split_whitespace().next().unwrap_or(trimmed);
            kind = Some(kind_of(declaration)?);
        }
        lines.push((*number, (*text).to_string()));
    }

    Some(Diagram { kind: kind?, lines })
}

fn inspect(diagram: &Diagram, config: &Config) -> Vec<Finding> {
    let mut findings = colors(diagram, config);
    findings.extend(legibility(diagram, config));
    findings.sort_by_key(|finding| finding.line);
    findings
}

// -- Colour ------------------------------------------------------------------

/// A `classDef` is the one place a diagram may set colour.
///
/// Everywhere else -- `style`, `linkStyle`, an initialization directive -- puts
/// a colour outside the palette's reach, so it is refused wherever it appears
/// rather than checked against the declared sets.
fn colors(diagram: &Diagram, config: &Config) -> Vec<Finding> {
    let mut findings = Vec::new();

    for (number, text) in &diagram.lines {
        let trimmed = text.trim();
        // A plain comment is prose about the palette, never the palette, so a
        // repeated or inaccurate one is not a finding. A `%%{...}%%` directive
        // is not a comment: it configures the renderer.
        if trimmed.starts_with("%%") && !trimmed.starts_with("%%{") {
            continue;
        }

        if let Some(rest) = trimmed.strip_prefix("classDef ") {
            findings.extend(class_definition(*number, rest, config));
        } else if contains_color(trimmed) {
            findings.push(Finding::new(
                ACCESSIBILITY,
                "",
                "color is declared outside a classDef, where the declared palette cannot reach it",
            ).at_line(*number));
        }
    }

    findings
}

fn class_definition(line: usize, rest: &str, config: &Config) -> Vec<Finding> {
    let mut findings = Vec::new();
    let palette = &config.mermaid;

    let mut fill = None;
    let mut stroke = None;
    let mut text_color = None;

    // `classDef name k:v,k:v` -- the name is the first token, the rest are the
    // roles.
    let assignments = rest
        .split_once(char::is_whitespace)
        .map_or("", |(_, tail)| tail);
    for assignment in assignments.split(',') {
        let Some((role, value)) = assignment.split_once(':') else {
            continue;
        };
        let value = value.trim();
        match role.trim() {
            "fill" => fill = Some(value),
            "stroke" => stroke = Some(value),
            "color" => text_color = Some(value),
            _ => {}
        }
    }

    let mut refuse = |message: String| {
        findings.push(Finding::new(ACCESSIBILITY, "", message).at_line(line));
    };

    for (role, value) in [("fill", fill), ("stroke", stroke), ("color", text_color)] {
        if let Some(value) = value
            && !is_six_digit_hex(value)
        {
            refuse(format!(
                "`{role}:{value}` is not a six-digit hex color, so it cannot be compared with the declared palette"
            ));
        }
    }

    match (fill, stroke, text_color) {
        (Some(fill), stroke, text_color) => {
            if is_six_digit_hex(fill) && !declares(&palette.fill_colors, fill) {
                refuse(format!("fill `{fill}` is not a declared fill color"));
            }
            match stroke {
                None => refuse(
                    "a filled class has no stroke, so its boundary disappears against the background"
                        .to_string(),
                ),
                Some(stroke) => {
                    if is_six_digit_hex(stroke) && !declares(&palette.edge_colors, stroke) {
                        refuse(format!("stroke `{stroke}` is not a declared edge color"));
                    }
                    if stroke.eq_ignore_ascii_case(fill) {
                        refuse(format!(
                            "fill and stroke are both `{fill}`, so the shape has no visible boundary"
                        ));
                    }
                }
            }
            match text_color {
                None => refuse(
                    "a filled class declares no text color, so its label inherits an unknown one"
                        .to_string(),
                ),
                Some(text_color) => {
                    if is_six_digit_hex(text_color) && !declares(&palette.text_colors, text_color) {
                        refuse(format!(
                            "text color `{text_color}` is not a declared text color"
                        ));
                    }
                    // Only measure a pairing the repository actually permits.
                    // Contrast against a colour it already rejected is a second
                    // finding about a value that has to change anyway, and it
                    // buries the one a reader can act on.
                    let both_declared = declares(&palette.fill_colors, fill)
                        && declares(&palette.text_colors, text_color);
                    if let (true, Some(background), Some(foreground)) =
                        (both_declared, luminance(fill), luminance(text_color))
                    {
                        let ratio = contrast(background, foreground);
                        if ratio < NORMAL_TEXT_CONTRAST {
                            findings.push(
                                Finding::new(
                                    ACCESSIBILITY,
                                    "",
                                    format!(
                                        "text `{text_color}` on fill `{fill}` is below the normal-text contrast threshold"
                                    ),
                                )
                                .at_line(line)
                                .with("ratio", Detail::Text(format!("{ratio:.2}")))
                                .with("threshold", Detail::Text(format!("{NORMAL_TEXT_CONTRAST:.1}"))),
                            );
                        }
                    }
                }
            }
        }
        // Stroke alone is a legitimate outline-only class.
        (None, Some(stroke), None) => {
            if is_six_digit_hex(stroke) && !declares(&palette.edge_colors, stroke) {
                refuse(format!("stroke `{stroke}` is not a declared edge color"));
            }
        }
        (None, None, Some(_)) => refuse(
            "a class that sets only text color has neither a fill nor a stroke to make it legible"
                .to_string(),
        ),
        (None, Some(stroke), Some(_)) => {
            if is_six_digit_hex(stroke) && !declares(&palette.edge_colors, stroke) {
                refuse(format!("stroke `{stroke}` is not a declared edge color"));
            }
        }
        (None, None, None) => {}
    }

    findings
}

/// The WCAG 2 threshold for normal-size text. Not configurable: it is a
/// property of human vision, not of a repository.
const NORMAL_TEXT_CONTRAST: f64 = 4.5;

fn declares(declared: &[String], value: &str) -> bool {
    declared
        .iter()
        .any(|candidate| candidate.eq_ignore_ascii_case(value))
}

fn is_six_digit_hex(value: &str) -> bool {
    let Some(digits) = value.strip_prefix('#') else {
        return false;
    };
    digits.len() == 6 && digits.chars().all(|c| c.is_ascii_hexdigit())
}

/// Whether a line sets a colour anywhere.
///
/// Deliberately broad: outside a `classDef` there is no acceptable colour, so
/// the question is only whether one is present, never which.
fn contains_color(line: &str) -> bool {
    if line.contains("rgb(") || line.contains("rgba(") || line.contains("hsl(") {
        return true;
    }
    for (index, character) in line.char_indices() {
        if character != '#' {
            continue;
        }
        let digits = line[index + 1..]
            .chars()
            .take_while(char::is_ascii_hexdigit)
            .count();
        if (3..=8).contains(&digits) {
            return true;
        }
    }
    // A colour named as a word rather than a value, in a role that sets one.
    line.split(',').any(|assignment| {
        assignment.split_once(':').is_some_and(|(role, value)| {
            matches!(
                role.trim().rsplit(' ').next(),
                Some("fill" | "stroke" | "color")
            ) && !value.trim().is_empty()
        })
    })
}

fn luminance(color: &str) -> Option<f64> {
    let digits = color.strip_prefix('#')?;
    if digits.len() != 6 {
        return None;
    }
    let channel = |from: usize| -> Option<f64> {
        let value = u8::from_str_radix(&digits[from..from + 2], 16).ok()?;
        let scaled = f64::from(value) / 255.0;
        Some(if scaled <= 0.03928 {
            scaled / 12.92
        } else {
            ((scaled + 0.055) / 1.055).powf(2.4)
        })
    };
    Some(0.2126 * channel(0)? + 0.7152 * channel(2)? + 0.0722 * channel(4)?)
}

fn contrast(first: f64, second: f64) -> f64 {
    let (lighter, darker) = if first >= second {
        (first, second)
    } else {
        (second, first)
    };
    (lighter + 0.05) / (darker + 0.05)
}

// -- Legibility ---------------------------------------------------------------

/// Where a label was found, which is also what a consumer filters on.
#[derive(Clone, Copy, PartialEq, Eq)]
enum Segment {
    Node,
    Edge,
}

impl Segment {
    fn name(self) -> &'static str {
        match self {
            Self::Node => "node",
            Self::Edge => "edge",
        }
    }

    fn limit(self, config: &Config) -> usize {
        match self {
            Self::Node => config.mermaid.node_label_graphemes,
            Self::Edge => config.mermaid.edge_label_graphemes,
        }
    }
}

fn legibility(diagram: &Diagram, config: &Config) -> Vec<Finding> {
    let mut findings = Vec::new();

    for (number, text) in &diagram.lines {
        let trimmed = text.trim();
        if is_not_a_label_source(diagram.kind, trimmed) {
            continue;
        }

        // A transition ending in a semicolon renders as part of the label in
        // some viewers, which is a legibility fault rather than a syntax one.
        if diagram.kind == Kind::State && trimmed.contains("-->") && trimmed.ends_with(';') {
            findings.push(
                Finding::new(
                    LEGIBILITY,
                    "",
                    "a state transition ends in a semicolon, which some renderers draw as part of the label",
                )
                .at_line(*number),
            );
        }

        for (segment, label) in labels(diagram.kind, trimmed) {
            let limit = segment.limit(config);
            for visible in segments(&label) {
                let measured = visible.graphemes(true).count();
                if measured > limit {
                    findings.push(
                        Finding::new(
                            LEGIBILITY,
                            "",
                            format!(
                                "a {} label is longer than the declared limit",
                                segment.name()
                            ),
                        )
                        .at_line(*number)
                        .with("segment", Detail::Text(segment.name().to_string()))
                        .with("measured", Detail::Count(measured))
                        .with("limit", Detail::Count(limit)),
                    );
                }
            }
        }
    }

    findings
}

/// Lines that declare styling, behaviour, or structure rather than visible text.
///
/// `class` is the interesting case: in a class diagram it names a node a reader
/// sees, and in a flowchart it applies a `classDef` to one. Same word, different
/// question, so the answer depends on the diagram kind.
fn is_not_a_label_source(kind: Kind, line: &str) -> bool {
    if line.is_empty() || line.starts_with("%%") || line == "---" {
        return true;
    }
    let first = line.split_whitespace().next().unwrap_or(line);
    match first {
        "classDef" | "style" | "linkStyle" | "click" | "direction" | "call" | "href" => true,
        "class" => kind != Kind::Class,
        _ => false,
    }
}

/// Every visible label a line declares, with what kind of thing it labels.
fn labels(kind: Kind, line: &str) -> Vec<(Segment, String)> {
    match kind {
        Kind::Flow | Kind::Block => flow_labels(line),
        Kind::Class => {
            enclosed_after(line, "class").map_or_else(Vec::new, |name| vec![(Segment::Node, name)])
        }
        Kind::State => quoted(line)
            .map(|label| vec![(Segment::Node, label)])
            .unwrap_or_default(),
        Kind::Entity => entity_labels(line),
        Kind::Requirement => enclosed_after(line, "requirement")
            .map_or_else(Vec::new, |name| vec![(Segment::Node, name)]),
    }
}

fn flow_labels(line: &str) -> Vec<(Segment, String)> {
    let mut found = Vec::new();

    // Edge labels first, and their text is removed from what the node scan
    // sees, so a pipe-delimited label is never also read as a node.
    let mut remainder = String::new();
    let mut rest = line;
    while let Some(start) = rest.find('|') {
        let after = &rest[start + 1..];
        let Some(end) = after.find('|') else {
            break;
        };
        found.push((Segment::Edge, after[..end].to_string()));
        remainder.push_str(&rest[..start]);
        rest = &after[end + 1..];
    }
    remainder.push_str(rest);

    // A text edge writes its label between the dashes: `A -- label --> B`.
    if let Some((head, tail)) = remainder.split_once("-- ")
        && let Some((label, _)) = tail.split_once("-->")
        && !head.contains("-->")
    {
        found.push((Segment::Edge, label.trim().to_string()));
    }

    for delimiters in [('[', ']'), ('(', ')'), ('{', '}')] {
        found.extend(
            between(&remainder, delimiters.0, delimiters.1)
                .into_iter()
                .map(|label| (Segment::Node, label)),
        );
    }

    found
}

fn entity_labels(line: &str) -> Vec<(Segment, String)> {
    // `ALPHA ||--|| BETA : places` -- the entity names are what a reader sees.
    let head = line.split(':').next().unwrap_or(line);
    head.split_whitespace()
        .filter(|token| {
            token
                .chars()
                .all(|c| c.is_alphanumeric() || c == '_' || c == '-')
        })
        .map(|token| (Segment::Node, token.to_string()))
        .collect()
}

/// The identifier following a keyword, stripped of any trailing brace.
fn enclosed_after(line: &str, keyword: &str) -> Option<String> {
    let rest = line.strip_prefix(keyword)?.trim_start();
    let name = rest.split_whitespace().next()?;
    Some(name.trim_end_matches('{').trim_matches('"').to_string())
}

fn quoted(line: &str) -> Option<String> {
    let start = line.find('"')?;
    let rest = &line[start + 1..];
    let end = rest.find('"')?;
    Some(rest[..end].to_string())
}

/// Every run of text between a delimiter pair, innermost first.
fn between(line: &str, open: char, close: char) -> Vec<String> {
    let mut found = Vec::new();
    let mut depth = 0usize;
    let mut start = 0usize;

    for (index, character) in line.char_indices() {
        if character == open {
            if depth == 0 {
                start = index + character.len_utf8();
            }
            depth += 1;
        } else if character == close && depth > 0 {
            depth -= 1;
            if depth == 0 {
                found.push(line[start..index].trim_matches('"').to_string());
            }
        }
    }

    found
}

/// A label's visible segments, decoded and stripped of markup.
///
/// A break splits a label into lines a reader sees separately, so the limit is
/// per segment: two thirty-two grapheme segments are two legible lines, not one
/// illegible one.
fn segments(label: &str) -> Vec<String> {
    let mut normalised = label.to_string();
    for separator in ["<br/>", "<br />", "<br>", "\\n"] {
        normalised = normalised.replace(separator, "\u{1}");
    }
    normalised
        .split('\u{1}')
        .map(|segment| decode(&strip_markup(segment)))
        .collect()
}

/// Remove HTML tags, which are markup a reader never counts.
fn strip_markup(text: &str) -> String {
    let mut out = String::new();
    let mut depth = 0usize;
    for character in text.chars() {
        match character {
            '<' => depth += 1,
            '>' if depth > 0 => depth -= 1,
            _ if depth == 0 => out.push(character),
            _ => {}
        }
    }
    out
}

/// Decode the HTML entities Mermaid passes through, so a label is measured as
/// it is seen rather than as it is written.
fn decode(text: &str) -> String {
    let mut out = String::new();
    let mut rest = text;

    while let Some(start) = rest.find('&') {
        out.push_str(&rest[..start]);
        let after = &rest[start..];
        let Some(end) = after.find(';') else {
            out.push('&');
            rest = &after[1..];
            continue;
        };
        let entity = &after[1..end];
        let decoded = match entity {
            "amp" => Some('&'),
            "lt" => Some('<'),
            "gt" => Some('>'),
            "quot" => Some('"'),
            "apos" => Some('\''),
            "nbsp" => Some('\u{a0}'),
            _ => numeric(entity),
        };
        match decoded {
            Some(character) => {
                out.push(character);
                rest = &after[end + 1..];
            }
            None => {
                out.push('&');
                rest = &after[1..];
            }
        }
    }

    out.push_str(rest);
    out
}

fn numeric(entity: &str) -> Option<char> {
    let digits = entity.strip_prefix('#')?;
    let code = match digits.strip_prefix(['x', 'X']) {
        Some(hex) => u32::from_str_radix(hex, 16).ok()?,
        None => digits.parse().ok()?,
    };
    char::from_u32(code)
}
