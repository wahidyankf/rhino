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

use crate::config::{AuthoringRule, Config, Mermaid};
use crate::markdown::{Fenced, fenced_blocks};
use crate::report::{Detail, Finding, Report};
use crate::runtime::Tree;
use crate::scan::{Corpus, Document, Scope, glob_set};
use unicode_segmentation::UnicodeSegmentation;

const ACCESSIBILITY: &str = "mermaid-accessibility";
const LEGIBILITY: &str = "mermaid-legibility";
const AUTHORING: &str = "diagram-authoring-rule";

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
    /// A syntax this build cannot parse that the repository declared allowed.
    ///
    /// Only the checks that need no grammar apply to it: the accessible title
    /// and description, colour outside a class, and a theme override.
    Other,
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
    /// The first word of the declaration, which is the diagram's type.
    declaration: String,
    lines: Vec<(usize, String)>,
}

pub fn validate(tree: &dyn Tree, config: &Config, mermaid: &Mermaid, scope: &Scope) -> Report {
    let mut report = Report::new("mermaid", "diagram");
    let mut inspected = 0usize;

    // A selection is inspected exactly as given, in the order given, and is
    // never widened back to the declared surface: a caller who named two files
    // asked about two files.
    let excluded = match mermaid
        .exclude
        .as_deref()
        .map(|globs| glob_set("md-mermaid.exclude", globs.iter().map(String::as_str)))
    {
        Some(Ok(set)) => Some(set),
        Some(Err(reason)) => return Report::unreadable("mermaid", reason),
        None => None,
    };
    if let Some(canvas) = mermaid
        .canvas_colors
        .iter()
        .find(|canvas| !is_six_digit_hex(canvas))
    {
        return Report::unreadable(
            "mermaid",
            format!("md-mermaid.canvas-colors: `{canvas}` is not a six-digit hex color"),
        );
    }

    let selected: Vec<Document> = if scope.is_narrowed() {
        let mut selected = Vec::new();
        for (path, content) in scope.documents(tree) {
            // A selection naming an excluded path is skipped, not refused: a
            // pre-commit run hands over every staged path, and a move into an
            // archive stages files the repository has declared out of scope.
            if excluded.as_ref().is_some_and(|set| set.is_match(&path)) {
                continue;
            }
            let text = match content {
                Ok(text) => text,
                Err(unselectable) => return unselectable.refusal("mermaid", &path),
            };
            selected.push(Document { path, text });
        }
        selected
    } else {
        let corpus = match &excluded {
            Some(set) => Corpus::read_excluding(tree, set),
            None => Corpus::read(tree, config),
        };
        match corpus {
            Ok(corpus) => corpus.into_documents(),
            Err(reason) => return Report::unreadable("mermaid", reason),
        }
    };

    for document in &selected {
        for block in fenced_blocks(&document.text) {
            if block.info.trim() != "mermaid" {
                continue;
            }
            // The form comes before anything about the diagram's content. A
            // repository that authors in plain text has said no Mermaid may
            // appear, and reporting the colours of a diagram that may not be
            // there would be answering the wrong question politely.
            if mermaid.authoring_rule == Some(AuthoringRule::PlainText) {
                inspected += 1;
                report.found(
                    Finding::new(
                        AUTHORING,
                        "",
                        "carries a Mermaid diagram, and this repository authors conceptual diagrams in plain text",
                    )
                    .at_line(block.lines.first().map_or(1, |(number, _)| *number))
                    .rename(&document.path),
                );
                continue;
            }
            let Some(diagram) = read(&block, mermaid.allowed_types.is_some()) else {
                continue;
            };
            inspected += 1;
            for finding in inspect(&diagram, mermaid) {
                report.found(finding.rename(&document.path));
            }
        }
    }

    report.inspected(inspected);
    report
}

/// Read a fenced block as a diagram, or decline it.
///
/// Declining covers both a block with no diagram declaration and one whose
/// declaration this build cannot parse. Neither is a finding: reporting on
/// syntax the tool does not understand would be reporting on its own ignorance.
fn read(block: &Fenced<'_>, judge_every_type: bool) -> Option<Diagram> {
    let mut lines: Vec<(usize, String)> = Vec::new();
    let mut in_front_matter = false;
    let mut found: Option<(Kind, String)> = None;

    for (number, text) in &block.lines {
        let trimmed = text.trim();

        // Front matter is configuration for the renderer, not diagram source,
        // and the declaration sits after it.
        if trimmed == "---" {
            in_front_matter = !in_front_matter;
            lines.push((*number, (*text).to_string()));
            continue;
        }
        if found.is_none() && !in_front_matter && !trimmed.is_empty() && !trimmed.starts_with("%%")
        {
            let declaration = trimmed.split_whitespace().next().unwrap_or(trimmed);
            // A repository that declared its allowed types has asked for every
            // diagram to be judged, so a syntax this build cannot parse is
            // still a diagram -- one the grammar-free checks can read.
            let kind = match kind_of(declaration) {
                Some(kind) => kind,
                None if judge_every_type => Kind::Other,
                None => return None,
            };
            found = Some((kind, declaration.to_string()));
        }
        lines.push((*number, (*text).to_string()));
    }

    let (kind, declaration) = found?;
    Some(Diagram {
        kind,
        declaration,
        lines,
    })
}

fn inspect(diagram: &Diagram, mermaid: &Mermaid) -> Vec<Finding> {
    // An undeclared type is refused alone: the other checks would be judging a
    // diagram the repository has said may not be there.
    if let Some(finding) = undeclared_type(diagram, mermaid) {
        return vec![finding];
    }
    let mut findings = accessibility(diagram, mermaid);
    findings.extend(colors(diagram, mermaid));
    findings.extend(default_class(diagram, mermaid));
    findings.extend(theme_overrides(diagram, mermaid));
    findings.extend(legibility(diagram, mermaid));
    findings.sort_by_key(|finding| finding.line);
    findings
}

/// The type allowlist: a type outside a declared list is refused, never skipped.
fn undeclared_type(diagram: &Diagram, mermaid: &Mermaid) -> Option<Finding> {
    let allowed = mermaid.allowed_types.as_ref()?;
    if allowed.contains(&diagram.declaration) {
        return None;
    }
    let line = diagram.lines.first().map_or(1, |(number, _)| *number);
    Some(
        Finding::new(
            ACCESSIBILITY,
            "",
            format!(
                "diagram type `{}` is not in the declared allowed types, so this repository does not render it",
                diagram.declaration
            ),
        )
        .at_line(line),
    )
}

/// The accessible title and description a rendered repository requires.
///
/// Checked only where the repository declared the rule. A default here would be
/// this tool choosing an authoring style on a repository's behalf, and it would
/// change what an existing consumer's run reports without that consumer having
/// declared anything.
fn accessibility(diagram: &Diagram, mermaid: &Mermaid) -> Vec<Finding> {
    if mermaid.authoring_rule != Some(AuthoringRule::Rendered) {
        return Vec::new();
    }
    let line = diagram.lines.first().map_or(1, |(number, _)| *number);
    let mut findings = Vec::new();
    for (declaration, what) in [("accTitle", "title"), ("accDescr", "description")] {
        // Either form counts: `accDescr:` carries one line and `accDescr {`
        // opens a block. A rule that accepted only the first would report a
        // diagram whose description is longer than a line as having none.
        let declared = diagram.lines.iter().any(|(_, text)| {
            let trimmed = text.trim();
            trimmed.starts_with(&format!("{declaration}:"))
                || trimmed.starts_with(&format!("{declaration} {{"))
        });
        if !declared {
            findings.push(
                Finding::new(
                    ACCESSIBILITY,
                    "",
                    format!(
                        "declares no accessible {what}; a rendered diagram is opaque to every reader who will never see it, including every automated check and every text search"
                    ),
                )
                .at_line(line),
            );
        }
    }
    findings
}

// -- Colour ------------------------------------------------------------------

/// A `classDef` is the one place a diagram may set colour.
///
/// Everywhere else -- `style`, `linkStyle`, an initialization directive -- puts
/// a colour outside the palette's reach, so it is refused wherever it appears
/// rather than checked against the declared sets.
fn colors(diagram: &Diagram, mermaid: &Mermaid) -> Vec<Finding> {
    let mut findings = Vec::new();
    // Declaring the allowed types opts in to reading a colour by what it is
    // assigned to, so a label such as `PR #123` is prose and not a hex colour.
    // Without the declaration the broader `v0.7` reading stands unchanged.
    let strict = mermaid.allowed_types.is_some();

    for (number, text) in &diagram.lines {
        let trimmed = text.trim();
        // A plain comment is prose about the palette, never the palette, so a
        // repeated or inaccurate one is not a finding. A `%%{...}%%` directive
        // is not a comment: it configures the renderer.
        if trimmed.starts_with("%%") && !trimmed.starts_with("%%{") {
            continue;
        }

        if let Some(rest) = trimmed.strip_prefix("classDef ") {
            findings.extend(class_definition(*number, rest, mermaid));
        } else if contains_color(trimmed, strict) || (strict && colors_a_box(trimmed)) {
            findings.push(Finding::new(
                ACCESSIBILITY,
                "",
                "color is declared outside a classDef, where the declared palette cannot reach it",
            ).at_line(*number));
        }
    }

    findings
}

/// The roles a `classDef default` must set for every node to be legible with no
/// class of its own.
const DEFAULT_CLASS_ROLES: [&str; 3] = ["fill", "stroke", "color"];

/// Whether the renderer applies `classDef default` to a diagram of this kind.
///
/// It does in flowcharts, class, entity-relationship, and requirement diagrams.
/// A state diagram ignores it, so demanding one there would demand dead text.
fn applies_default_class(kind: Kind) -> bool {
    matches!(
        kind,
        Kind::Flow | Kind::Class | Kind::Entity | Kind::Requirement
    )
}

fn default_class(diagram: &Diagram, mermaid: &Mermaid) -> Vec<Finding> {
    if !mermaid.require_default_class || !applies_default_class(diagram.kind) {
        return Vec::new();
    }
    let line = diagram.lines.first().map_or(1, |(number, _)| *number);
    let declared = diagram.lines.iter().find_map(|(_, text)| {
        text.trim()
            .strip_prefix("classDef default ")
            .map(str::to_string)
    });
    let Some(declared) = declared else {
        return vec![Finding::new(
            ACCESSIBILITY,
            "",
            "declares no `classDef default`, so every unclassed node takes a colour the palette never chose",
        )
        .at_line(line)];
    };
    DEFAULT_CLASS_ROLES
        .iter()
        .filter(|role| {
            !declared.split(',').any(|assignment| {
                assignment
                    .split_once(':')
                    .is_some_and(|(name, _)| name.trim() == **role)
            })
        })
        .map(|role| {
            Finding::new(
                ACCESSIBILITY,
                "",
                format!("`classDef default` sets no `{role}`, so unclassed nodes fall back to an unchosen one"),
            )
            .at_line(line)
        })
        .collect()
}

/// An initialization directive or a front-matter theme, each refused on its own
/// line. Either one replaces the renderer's automatic light and dark switching
/// with colours the palette never saw.
fn theme_overrides(diagram: &Diagram, mermaid: &Mermaid) -> Vec<Finding> {
    if !mermaid.forbid_theme_overrides {
        return Vec::new();
    }
    let mut findings = Vec::new();
    let mut in_front_matter = false;
    for (number, text) in &diagram.lines {
        let trimmed = text.trim();
        if trimmed == "---" {
            in_front_matter = !in_front_matter;
            continue;
        }
        let directive = trimmed.starts_with("%%{")
            && (trimmed.contains("init") || trimmed.contains("initialize"));
        let front_matter = in_front_matter
            && ["theme:", "themeVariables:", "themeCSS:"]
                .iter()
                .any(|key| trimmed.starts_with(key));
        if directive || front_matter {
            findings.push(
                Finding::new(
                    ACCESSIBILITY,
                    "",
                    "a theme override replaces the renderer's light and dark switching with colours the palette never measured",
                )
                .at_line(*number),
            );
        }
    }
    findings
}

fn class_definition(line: usize, rest: &str, mermaid: &Mermaid) -> Vec<Finding> {
    let mut findings = Vec::new();
    let palette = mermaid;

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
        if let Some(value) = value.filter(|value| !is_six_digit_hex(value)) {
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

    findings.extend(canvas_findings(line, fill, stroke, mermaid));
    findings
}

/// The WCAG 2 threshold for a graphical object against what is adjacent to it.
const NON_TEXT_CONTRAST: f64 = 3.0;

/// A filled class must stay visible on every declared canvas.
///
/// Visible means the fill or the outline reaches the non-text threshold, so a
/// pale fill with a black outline passes on white and a dark fill with a pale
/// outline passes on black. A class that reaches it on neither colour vanishes
/// there, whatever its label says.
fn canvas_findings(
    line: usize,
    fill: Option<&str>,
    stroke: Option<&str>,
    mermaid: &Mermaid,
) -> Vec<Finding> {
    let Some(fill) = fill.filter(|fill| is_six_digit_hex(fill)) else {
        return Vec::new();
    };
    let shapes: Vec<f64> = [Some(fill), stroke]
        .into_iter()
        .flatten()
        .filter_map(luminance)
        .collect();
    mermaid
        .canvas_colors
        .iter()
        .filter_map(|canvas| {
            let background = luminance(canvas)?;
            let best = shapes
                .iter()
                .map(|shape| contrast(*shape, background))
                .fold(0.0, f64::max);
            (best < NON_TEXT_CONTRAST).then(|| {
                Finding::new(
                    ACCESSIBILITY,
                    "",
                    format!(
                        "neither fill `{fill}` nor its outline reaches the non-text contrast threshold against the canvas `{canvas}`"
                    ),
                )
                .at_line(line)
                .with("ratio", Detail::Text(format!("{best:.2}")))
                .with("threshold", Detail::Text(format!("{NON_TEXT_CONTRAST:.1}")))
                .with("canvas", Detail::Text(canvas.clone()))
            })
        })
        .collect()
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
fn contains_color(line: &str, strict: bool) -> bool {
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
        if !(3..=8).contains(&digits) {
            continue;
        }
        if !strict || is_assigned_color(&line[..index]) {
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

/// Whether the text before a `#` makes it the value of a colour property.
///
/// `fill:#fff` and `"primaryColor": "#fff"` qualify; `fixes PR #123` does not,
/// because nothing there assigns a colour.
fn is_assigned_color(before: &str) -> bool {
    let assigned = before.trim_end_matches([' ', '"', '\'']);
    let Some(property) = assigned.strip_suffix(':') else {
        return false;
    };
    let word = property
        .trim_end_matches([' ', '"', '\''])
        .rsplit(|c: char| !c.is_ascii_alphanumeric())
        .next()
        .unwrap_or("")
        .to_ascii_lowercase();
    matches!(word.as_str(), "fill" | "stroke" | "background" | "bkg")
        || word.ends_with("color")
        || word.ends_with("bkg")
        || word.ends_with("border")
}

/// A sequence-diagram `box` that names a colour, which that syntax cannot
/// express as a hex value and so can never come from the palette.
fn colors_a_box(line: &str) -> bool {
    const NAMES: [&str; 24] = [
        "aqua",
        "black",
        "blue",
        "brown",
        "cyan",
        "fuchsia",
        "gray",
        "grey",
        "green",
        "lime",
        "magenta",
        "maroon",
        "navy",
        "olive",
        "orange",
        "pink",
        "purple",
        "red",
        "silver",
        "teal",
        "white",
        "yellow",
        "lightblue",
        "lightgreen",
    ];
    let Some(rest) = line.strip_prefix("box ") else {
        return false;
    };
    rest.split_whitespace()
        .next()
        .is_some_and(|word| NAMES.contains(&word.to_ascii_lowercase().as_str()))
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
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
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

    fn limit(self, mermaid: &Mermaid) -> usize {
        match self {
            Self::Node => mermaid.node_label_graphemes,
            Self::Edge => mermaid.edge_label_graphemes,
        }
    }
}

fn legibility(diagram: &Diagram, mermaid: &Mermaid) -> Vec<Finding> {
    let mut findings = Vec::new();
    let mut in_accessibility_block = false;

    for (number, text) in &diagram.lines {
        let trimmed = text.trim();
        // The accessible description is prose for a screen reader, not a label
        // a sighted reader sees, so its length and punctuation are not drawn.
        if in_accessibility_block {
            in_accessibility_block = !trimmed.contains('}');
            continue;
        }
        if opens_accessibility_block(trimmed) {
            in_accessibility_block = true;
            continue;
        }
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
            let limit = segment.limit(mermaid);
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

/// Whether a line opens a multi-line `accTitle { ... }` or `accDescr { ... }`
/// block, whose lines up to the closing brace are prose and not labels.
fn opens_accessibility_block(line: &str) -> bool {
    ["accTitle", "accDescr"].iter().any(|name| {
        line.strip_prefix(name).is_some_and(|rest| {
            rest.trim() == "{" || (rest.trim_start().starts_with('{') && !rest.contains('}'))
        })
    })
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
        "accTitle:" | "accDescr:" => true,
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
        // No grammar is known, so no label can be told from other text.
        Kind::Other => Vec::new(),
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
    let label = remainder
        .split_once("-- ")
        .filter(|(head, _)| !head.contains("-->"))
        .and_then(|(_, tail)| tail.split_once("-->"))
        .map(|(label, _)| label);
    if let Some(label) = label {
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

#[cfg(test)]
mod tests {
    use super::*;
    use crate::cli::Format;
    use crate::runtime::MemoryTree;

    fn policy(rule: Option<AuthoringRule>) -> Mermaid {
        Mermaid {
            authoring_rule: rule,
            node_label_graphemes: 4,
            edge_label_graphemes: 3,
            fill_colors: vec!["#FFFFFF".to_string(), "#000000".to_string()],
            edge_colors: vec!["#000000".to_string(), "#123456".to_string()],
            text_colors: vec!["#000000".to_string(), "#FFFFFF".to_string()],
            ..Mermaid::default()
        }
    }

    fn diagram(kind: Kind, lines: &[&str]) -> Diagram {
        Diagram {
            kind,
            declaration: String::new(),
            lines: lines
                .iter()
                .enumerate()
                .map(|(index, line)| (index + 1, (*line).to_string()))
                .collect(),
        }
    }

    #[test]
    fn all_supported_declarations_are_read_and_unknown_ones_are_ignored() {
        for declaration in [
            "flowchart",
            "graph",
            "classDiagram",
            "classDiagram-v2",
            "stateDiagram",
            "stateDiagram-v2",
            "erDiagram",
            "requirementDiagram",
            "block",
            "block-beta",
        ] {
            assert!(kind_of(declaration).is_some(), "{declaration}");
        }
        assert!(kind_of("sequenceDiagram").is_none());

        let blocks = fenced_blocks(
            "```mermaid\n---\ntheme: base\n---\n%% comment\nflowchart LR\nA --> B\n```\n\n```mermaid\nsequenceDiagram\n```",
        );
        assert_eq!(blocks.len(), 2);
        assert!(matches!(
            read(&blocks[0], false).expect("supported diagram").kind,
            Kind::Flow
        ));
        assert!(read(&blocks[1], false).is_none());
    }

    #[test]
    fn rendered_diagrams_cover_palette_accessibility_and_every_label_syntax() {
        let policy = policy(Some(AuthoringRule::Rendered));
        let diagrams = [
            diagram(
                Kind::Flow,
                &[
                    "flowchart LR",
                    "A[long node] -->|long edge| B((other node))",
                    "A -- long text --> B{third node}",
                    "classDef good fill:#FFFFFF,stroke:#FFFFFF,color:#FFFFFF",
                    "classDef malformed fill:blue,stroke:#123456,color:#000000",
                    "classDef undeclared fill:#ABCDEF,stroke:#123456,color:#000000",
                    "classDef no-stroke fill:#FFFFFF",
                    "classDef no-text fill:#FFFFFF,stroke:#123456",
                    "classDef outline stroke:#123456",
                    "classDef bad-outline stroke:#ABCDEF",
                    "classDef textonly color:#000000",
                    "classDef mixed stroke:#badbad,color:#000000",
                    "classDef empty",
                    "classDef incomplete missing-colon",
                    "style A fill:#fff",
                    "%% a prose palette comment",
                    "%%{init: {\"theme\": \"base\"}}%%",
                ],
            ),
            diagram(Kind::Class, &["classDiagram", "class VeryLongClass {"]),
            diagram(
                Kind::State,
                &["stateDiagram-v2", "A --> B;", "state \"Very long state\""],
            ),
            diagram(
                Kind::Entity,
                &[
                    "erDiagram",
                    "VERY_LONG_ENTITY ||--|| OTHER_ENTITY : relates",
                ],
            ),
            diagram(
                Kind::Requirement,
                &["requirementDiagram", "requirement VeryLongRequirement {"],
            ),
            diagram(Kind::Block, &["block-beta", "A[very long block label]"]),
        ];

        let findings: Vec<_> = diagrams
            .iter()
            .flat_map(|diagram| inspect(diagram, &policy))
            .collect();
        let kinds: Vec<_> = findings.iter().map(|finding| finding.kind).collect();
        assert!(kinds.contains(&ACCESSIBILITY));
        assert!(kinds.contains(&LEGIBILITY));
        assert!(kinds.iter().filter(|kind| **kind == ACCESSIBILITY).count() > 6);
        assert!(kinds.iter().filter(|kind| **kind == LEGIBILITY).count() > 5);
    }

    #[test]
    fn parser_helpers_measure_visible_text_without_counting_markup() {
        assert!(is_not_a_label_source(Kind::Flow, "class A important"));
        assert!(!is_not_a_label_source(Kind::Class, "class A important"));
        assert_eq!(
            flow_labels("A[one] -->|two| B(three) -- four --> C{five}"),
            vec![
                (Segment::Edge, "two".to_string()),
                (Segment::Node, "one".to_string()),
                (Segment::Node, "three".to_string()),
                (Segment::Node, "five".to_string()),
            ]
        );
        assert!(flow_labels("A -- four --> B").contains(&(Segment::Edge, "four".to_string())));
        assert_eq!(
            enclosed_after("class Thing {", "class"),
            Some("Thing".to_string())
        );
        assert_eq!(quoted("state \"Readable\""), Some("Readable".to_string()));
        assert_eq!(
            between("A[outer [inner]]", '[', ']'),
            vec!["outer [inner]".to_string()]
        );
        assert_eq!(
            segments("<b>A&amp;B</b><br/>C&#x44;\\nE&#69;"),
            vec!["A&B".to_string(), "CD".to_string(), "EE".to_string()]
        );
        assert_eq!(decode("&unknown; &broken"), "&unknown; &broken");
        assert_eq!(numeric("#65"), Some('A'));
        assert_eq!(numeric("#x41"), Some('A'));
        assert!(contains_color("stroke: red", false));
        assert!(!contains_color("documentation only", false));
        assert!(luminance("#FFFFFF").is_some());
        assert!(luminance("bad").is_none());
        assert!(luminance("#123").is_none());
        assert!(contrast(1.0, 0.0) > 4.5);
        assert!(contrast(0.0, 1.0) > 4.5);
        assert!(contains_color("style A fill: rgb(1, 2, 3)", false));
        assert!(contains_color("style A fill:#abc", false));
        assert!(flow_labels("A |unclosed").is_empty());
    }

    fn declared(keys: impl FnOnce(&mut Mermaid)) -> Mermaid {
        let mut mermaid = policy(Some(AuthoringRule::Rendered));
        keys(&mut mermaid);
        mermaid
    }

    fn messages(findings: &[Finding]) -> String {
        findings
            .iter()
            .map(|finding| finding.message.clone())
            .collect::<Vec<_>>()
            .join("\n")
    }

    #[test]
    fn an_undeclared_type_is_refused_and_a_declared_unparsed_type_gets_the_universal_checks() {
        let mermaid = declared(|m| {
            m.allowed_types = Some(vec!["flowchart".to_string(), "sequenceDiagram".to_string()]);
        });
        let blocks = fenced_blocks(
            "```mermaid\nmindmap\n  root\n```\n\n```mermaid\nsequenceDiagram\nrect rgb(1, 2, 3)\nA->>B: hi\nend\n```",
        );
        let mindmap = read(&blocks[0], true).expect("judged when types are declared");
        let refused = inspect(&mindmap, &mermaid);
        assert_eq!(refused.len(), 1);
        assert!(messages(&refused).contains("`mindmap` is not in the declared allowed types"));

        let sequence = read(&blocks[1], true).expect("judged when types are declared");
        let universal = messages(&inspect(&sequence, &mermaid));
        assert!(universal.contains("accessible title"));
        assert!(universal.contains("outside a classDef"));
    }

    #[test]
    fn a_pull_request_number_is_not_a_colour_once_types_are_declared() {
        assert!(contains_color("A->>B: fixes PR #123", false));
        assert!(!contains_color("A->>B: fixes PR #123", true));
        assert!(!contains_color("A->>B: Issue: #123", true));
        assert!(contains_color("style A fill:#123", true));
        assert!(contains_color(
            "%%{init: {'themeVariables': {'primaryColor': '#fff'}}}%%",
            true
        ));
        assert!(colors_a_box("box purple Group"));
        assert!(!colors_a_box("box Group"));
    }

    #[test]
    fn a_complete_default_class_is_required_only_where_the_renderer_applies_one() {
        let mermaid = declared(|m| m.require_default_class = true);
        let flow = diagram(Kind::Flow, &["flowchart LR", "A --> B"]);
        assert!(messages(&inspect(&flow, &mermaid)).contains("classDef default"));

        let partial = diagram(
            Kind::Flow,
            &[
                "flowchart LR",
                "classDef default fill:#FFFFFF,stroke:#000000",
            ],
        );
        let findings = default_class(&partial, &mermaid);
        assert_eq!(findings.len(), 1);
        assert!(messages(&findings).contains("`color`"));

        let complete = diagram(
            Kind::Flow,
            &[
                "flowchart LR",
                "classDef default fill:#FFFFFF,stroke:#000000,color:#000000",
            ],
        );
        assert!(default_class(&complete, &mermaid).is_empty());
        assert!(default_class(&diagram(Kind::State, &["stateDiagram-v2"]), &mermaid).is_empty());
        assert!(default_class(&flow, &declared(|_| {})).is_empty());
    }

    #[test]
    fn theme_overrides_are_refused_on_their_own_lines() {
        let mermaid = declared(|m| m.forbid_theme_overrides = true);
        let flow = diagram(
            Kind::Flow,
            &[
                "---",
                "config:",
                "  theme: forest",
                "  themeVariables:",
                "---",
                "%%{init: {\"theme\": \"dark\"}}%%",
                "flowchart LR",
                "%% theme: just a comment",
            ],
        );
        let lines: Vec<Option<usize>> = theme_overrides(&flow, &mermaid)
            .iter()
            .map(|finding| finding.line)
            .collect();
        assert_eq!(lines, vec![Some(3), Some(4), Some(6)]);
        assert!(theme_overrides(&flow, &declared(|_| {})).is_empty());
    }

    #[test]
    fn a_class_invisible_on_a_declared_canvas_reports_the_ratio_and_the_canvas() {
        let mermaid = declared(|m| {
            m.canvas_colors = vec!["#FFFFFF".to_string(), "#0D1117".to_string()];
        });
        let shy = class_definition(1, "shy fill:#000000,stroke:#000000,color:#FFFFFF", &mermaid);
        let canvas: Vec<_> = shy
            .iter()
            .filter(|finding| format!("{finding:?}").contains("canvas"))
            .collect();
        assert!(!canvas.is_empty());
        assert!(messages(&shy).contains("`#0D1117`"));

        let warm = class_definition(
            1,
            "warm fill:#DE8F05,stroke:#000000,color:#000000",
            &mermaid,
        );
        assert!(!messages(&warm).contains("non-text"));
        // Absent the key, the same class is not measured.
        let unmeasured = class_definition(
            1,
            "shy fill:#000000,stroke:#000000,color:#FFFFFF",
            &policy(None),
        );
        assert!(!messages(&unmeasured).contains("non-text"));
    }

    #[test]
    fn an_exclude_glob_leaves_the_mermaid_scan_and_names_an_invalid_glob_or_canvas() {
        let tree = MemoryTree::default();
        tree.write(
            "plans/done/old.md",
            "```mermaid\nflowchart LR\nA-->B\n```\n",
        );
        tree.write("vendored/lib.md", "```mermaid\nflowchart LR\nA-->B\n```\n");
        let mermaid = declared(|m| m.exclude = Some(vec!["plans/done/**".to_string()]));
        let report =
            validate(&tree, &Config::default(), &mermaid, &Scope::default()).render(Format::Text);
        assert_eq!(report.exit_code, 1);
        assert!(report.stderr.contains("vendored/lib.md"));
        assert!(!report.stderr.contains("plans/done/old.md"));

        let bad_glob = declared(|m| m.exclude = Some(vec!["/absolute/**".to_string()]));
        assert_eq!(
            validate(&tree, &Config::default(), &bad_glob, &Scope::default())
                .render(Format::Text)
                .exit_code,
            2
        );
        let bad_canvas = declared(|m| m.canvas_colors = vec!["white".to_string()]);
        assert_eq!(
            validate(&tree, &Config::default(), &bad_canvas, &Scope::default())
                .render(Format::Text)
                .exit_code,
            2
        );
    }

    #[test]
    fn accessibility_prose_is_never_measured_as_a_label() {
        let mermaid = policy(Some(AuthoringRule::Rendered));
        let single = diagram(
            Kind::Flow,
            &[
                "flowchart LR",
                "accDescr: Nodes (a very long description here).",
            ],
        );
        assert!(legibility(&single, &mermaid).is_empty());
        let block = diagram(
            Kind::Flow,
            &[
                "flowchart LR",
                "accDescr {",
                "Nodes (a very long description here).",
                "}",
                "A[long node text]",
            ],
        );
        let lines: Vec<_> = legibility(&block, &mermaid)
            .iter()
            .map(|f| f.line)
            .collect();
        assert_eq!(lines, vec![Some(5)]);
    }

    #[test]
    fn validator_distinguishes_plain_text_rules_selection_and_unreadable_files() {
        let mut tree = MemoryTree::default();
        tree.write(
            "docs/diagram.md",
            "```mermaid\nflowchart LR\nA[Node] --> B[Next]\n```\n",
        );
        tree.write("docs/plain.md", "# prose\n");

        let plain = validate(
            &tree,
            &Config::default(),
            &policy(Some(AuthoringRule::PlainText)),
            &Scope::default(),
        )
        .render(Format::Text);
        assert_eq!(plain.exit_code, 1);
        assert!(
            plain
                .stderr
                .contains("authors conceptual diagrams in plain text")
        );

        let narrowed = Scope {
            files: vec!["docs/plain.md".to_string()],
            ..Scope::default()
        };
        let rendered = validate(
            &tree,
            &Config::default(),
            &policy(Some(AuthoringRule::Rendered)),
            &narrowed,
        )
        .render(Format::Text);
        assert_eq!(rendered.exit_code, 0);

        tree.write(
            "docs/not-diagram.md",
            "```rust\n# not Mermaid\n```\n```mermaid\nsequenceDiagram\n```",
        );
        let ignored = validate(&tree, &Config::default(), &policy(None), &Scope::default())
            .render(Format::Text);
        assert_eq!(ignored.exit_code, 0);
        assert!(accessibility(&diagram(Kind::Flow, &["graph LR"]), &policy(None)).is_empty());

        tree.mark_unreadable("docs/diagram.md");
        let unreadable = validate(
            &tree,
            &Config::default(),
            &policy(None),
            &Scope {
                files: vec!["docs/diagram.md".to_string()],
                ..Scope::default()
            },
        )
        .render(Format::Text);
        assert_eq!(unreadable.exit_code, 2);

        let mut binary = MemoryTree::default();
        binary.write("docs/diagram.md", "bytes");
        binary.mark_binary("docs/diagram.md");
        assert_eq!(
            validate(
                &binary,
                &Config::default(),
                &policy(None),
                &Scope::default()
            )
            .render(Format::Text)
            .exit_code,
            2
        );
    }
}
