//! The Mermaid sample catalogue.
//!
//! Scenarios name a sample rather than carrying its text, because the same
//! diagram is asserted on from several angles and a copy per scenario would
//! drift. Each sample is written to be a faithful instance of its own name: the
//! sample called `missing stroke` is a class declaration with no stroke and
//! nothing else wrong with it, so a scenario that fails on it fails for the
//! reason it says.
//!
//! Colours here are the ones the fixture configuration declares, and the
//! contrast expectations follow from them rather than from a rule written down
//! twice. Against the declared palette: `#0173B2` carries white text and not
//! black, while `#DE8F05` and `#029E73` carry black and not white.

use unicode_segmentation::UnicodeSegmentation;

/// Which fence a diagram is wrapped in. Both are legal Markdown and the
/// extractor has to handle each.
#[derive(Clone, Copy)]
pub enum Fence {
    Backtick,
    Tilde,
}

impl Fence {
    fn marker(self) -> &'static str {
        match self {
            Self::Backtick => "```",
            Self::Tilde => "~~~",
        }
    }
}

/// Wrap diagram source in a Markdown document, under a heading, so the line
/// number a finding reports is a line in a realistic file rather than line 1.
pub fn document(body: &str, fence: Fence) -> String {
    let marker = fence.marker();
    format!("# Diagram\n\n{marker}mermaid\n{body}\n{marker}\n")
}

/// A diagram of the named type whose class declaration uses an undeclared fill.
///
/// One generator rather than one sample per type, because the point of those
/// scenarios is the diagram *header* -- whether RHINO parses that syntax at all
/// -- and holding the body constant is what isolates it.
pub fn unsafe_diagram(header: &str, fence: Fence) -> String {
    let body = format!(
        "{header}\n{}\n    classDef unsafe fill:#FF0000,stroke:#000000,color:#FFFFFF\n    class {} unsafe",
        body_for(header),
        node_for(header)
    );
    document(&body, fence)
}

/// A minimal, syntactically plausible body for each diagram header.
fn body_for(header: &str) -> String {
    let kind = header.split_whitespace().next().unwrap_or(header);
    match kind {
        "classDiagram" => "    class Alpha".to_string(),
        "stateDiagram" | "stateDiagram-v2" => "    [*] --> Alpha".to_string(),
        "erDiagram" => "    ALPHA ||--|| BETA : links".to_string(),
        "requirementDiagram" => {
            "    requirement Alpha {\n        id: 1\n        text: short\n    }".to_string()
        }
        "block" => "    columns 1\n    Alpha[\"Alpha\"]".to_string(),
        "sequenceDiagram" => "    Alpha->>Beta: hello".to_string(),
        "mindmap" | "timeline" | "kanban" | "treeView" | "treemap-beta" | "swimlane-beta" => {
            "    Alpha".to_string()
        }
        "architecture-beta" => "    group alpha(cloud)[Alpha]".to_string(),
        "gantt" => "    title Alpha\n    section One\n    Task :a1, 2020-01-01, 1d".to_string(),
        "pie" => "    title Alpha\n    \"One\" : 1".to_string(),
        "quadrantChart" => "    title Alpha\n    Alpha: [0.5, 0.5]".to_string(),
        _ => "    Alpha[Alpha]".to_string(),
    }
}

fn node_for(header: &str) -> &'static str {
    match header.split_whitespace().next().unwrap_or(header) {
        "erDiagram" => "ALPHA",
        _ => "Alpha",
    }
}

/// A class declaration with the given fill, plus a stroke and text colour the
/// configuration declares and that pass the contrast rule for that fill.
pub fn class_filled(fill: &str) -> String {
    let text = if fill.eq_ignore_ascii_case("#DE8F05") || fill.eq_ignore_ascii_case("#029E73") {
        "#000000"
    } else {
        "#FFFFFF"
    };
    document(
        &format!(
            "flowchart LR\n    Alpha[Alpha]\n    classDef declared fill:{fill},stroke:#000000,color:{text}\n    class Alpha declared"
        ),
        Fence::Backtick,
    )
}

/// A flowchart whose single node label is exactly `graphemes` clusters long.
pub fn node_label_of(graphemes: usize) -> String {
    document(
        &format!("flowchart LR\n    Alpha[{}]", filler(graphemes)),
        Fence::Backtick,
    )
}

/// `count` grapheme clusters of ordinary Latin text.
fn filler(count: usize) -> String {
    "a".repeat(count)
}

/// `count` grapheme clusters, each of which is two code points.
///
/// Counting code points instead of clusters would give twice the number, which
/// is exactly the drift this sample is here to catch.
fn combining(count: usize) -> String {
    let cluster = "e\u{0301}";
    debug_assert_eq!(cluster.graphemes(true).count(), 1);
    cluster.repeat(count)
}

/// `count` grapheme clusters written as HTML entities, so the raw text is many
/// times longer than what a reader sees.
fn encoded(count: usize) -> String {
    "&amp;".repeat(count)
}

/// The accessible class declaration the passing samples share.
const ACCESSIBLE_CLASS: &str =
    "    classDef safe fill:#0173B2,stroke:#000000,color:#FFFFFF\n    class Alpha safe";

/// Look a sample up by the name a scenario uses.
pub fn sample(name: &str) -> Option<String> {
    let flowchart = |body: &str| {
        document(
            &format!("flowchart LR\n{body}\n{ACCESSIBLE_CLASS}"),
            Fence::Backtick,
        )
    };

    let text = match name {
        // -- Colour roles and formats ----------------------------------------
        "accessible colored class" => document(
            "flowchart LR\n    Alpha[Alpha]\n    classDef safe fill:#0173B2,stroke:#000000,color:#FFFFFF\n    class Alpha safe",
            Fence::Backtick,
        ),
        "accessible stroke-only class" => document(
            "flowchart LR\n    Alpha[Alpha]\n    classDef outline stroke:#0173B2\n    class Alpha outline",
            Fence::Backtick,
        ),
        "inaccessible stroke-only class" => document(
            "flowchart LR\n    Alpha[Alpha]\n    classDef outline stroke:#DE8F05\n    class Alpha outline",
            Fence::Backtick,
        ),
        "text-only class" => document(
            "flowchart LR\n    Alpha[Alpha]\n    classDef lettering color:#000000\n    class Alpha lettering",
            Fence::Backtick,
        ),
        "identical fill and stroke" => document(
            "flowchart LR\n    Alpha[Alpha]\n    classDef flat fill:#0173B2,stroke:#0173B2,color:#FFFFFF\n    class Alpha flat",
            Fence::Backtick,
        ),
        "missing stroke" => document(
            "flowchart LR\n    Alpha[Alpha]\n    classDef unbounded fill:#0173B2,color:#FFFFFF\n    class Alpha unbounded",
            Fence::Backtick,
        ),
        "undeclared node stroke" => document(
            "flowchart LR\n    Alpha[Alpha]\n    classDef edged fill:#0173B2,stroke:#DE8F05,color:#FFFFFF\n    class Alpha edged",
            Fence::Backtick,
        ),
        "missing text color" => document(
            "flowchart LR\n    Alpha[Alpha]\n    classDef wordless fill:#0173B2,stroke:#000000\n    class Alpha wordless",
            Fence::Backtick,
        ),
        "undeclared text color" => document(
            "flowchart LR\n    Alpha[Alpha]\n    classDef lettered fill:#0173B2,stroke:#000000,color:#029E73\n    class Alpha lettered",
            Fence::Backtick,
        ),
        // Both colours are declared; it is the pairing that fails. Black on
        // #0173B2 is 4.10:1, under the 4.5:1 normal-text threshold.
        "insufficient text contrast" => document(
            "flowchart LR\n    Alpha[Alpha]\n    classDef dim fill:#0173B2,stroke:#000000,color:#000000\n    class Alpha dim",
            Fence::Backtick,
        ),
        "named color" => document(
            "flowchart LR\n    Alpha[Alpha]\n    classDef named fill:red,stroke:#000000,color:#FFFFFF\n    class Alpha named",
            Fence::Backtick,
        ),
        "three-digit hex color" => document(
            "flowchart LR\n    Alpha[Alpha]\n    classDef short fill:#07B,stroke:#000000,color:#FFFFFF\n    class Alpha short",
            Fence::Backtick,
        ),
        "eight-digit hex color" => document(
            "flowchart LR\n    Alpha[Alpha]\n    classDef alpha fill:#0173B2FF,stroke:#000000,color:#FFFFFF\n    class Alpha alpha",
            Fence::Backtick,
        ),
        "RGB function color" => document(
            "flowchart LR\n    Alpha[Alpha]\n    classDef functional fill:rgb(1, 115, 178),stroke:#000000,color:#FFFFFF\n    class Alpha functional",
            Fence::Backtick,
        ),
        "HSL function color" => document(
            "flowchart LR\n    Alpha[Alpha]\n    classDef functional fill:hsl(201, 99%, 35%),stroke:#000000,color:#FFFFFF\n    class Alpha functional",
            Fence::Backtick,
        ),
        // Colour set anywhere other than a classDef is out of the palette's
        // reach, so it is rejected wherever it appears.
        "style declaration color" => document(
            "flowchart LR\n    Alpha[Alpha]\n    style Alpha fill:#0173B2,stroke:#000000,color:#FFFFFF",
            Fence::Backtick,
        ),
        "linkStyle declaration color" => document(
            "flowchart LR\n    Alpha[Alpha] --> Beta[Beta]\n    linkStyle 0 stroke:#0173B2",
            Fence::Backtick,
        ),
        "initialization directive color" => document(
            "%%{init: {\"themeVariables\": {\"primaryColor\": \"#0173B2\"}}}%%\nflowchart LR\n    Alpha[Alpha]",
            Fence::Backtick,
        ),

        // -- Palette comments -------------------------------------------------
        // A comment is prose about the palette, never the palette itself, so
        // neither a repeated one nor a wrong one is a finding.
        "duplicate palette comments" => document(
            "flowchart LR\n    %% palette: #0173B2\n    %% palette: #0173B2\n    Alpha[Alpha]\n    classDef safe fill:#0173B2,stroke:#000000,color:#FFFFFF\n    class Alpha safe",
            Fence::Backtick,
        ),
        "inaccurate palette comment" => document(
            "flowchart LR\n    %% palette: #FF0000\n    Alpha[Alpha]\n    classDef safe fill:#0173B2,stroke:#000000,color:#FFFFFF\n    class Alpha safe",
            Fence::Backtick,
        ),

        // -- Structure --------------------------------------------------------
        "YAML front matter before an unsafe flowchart" => document(
            "---\ntitle: Alpha\n---\nflowchart LR\n    Alpha[Alpha]\n    classDef unsafe fill:#FF0000,stroke:#000000,color:#FFFFFF\n    class Alpha unsafe",
            Fence::Backtick,
        ),
        "no diagram declaration" => document(
            "    Alpha[Alpha]\n    classDef unsafe fill:#FF0000,stroke:#000000,color:#FFFFFF",
            Fence::Backtick,
        ),

        // -- Legibility, per diagram type -------------------------------------
        "overlong flowchart node" => flowchart(&format!("    Alpha[{}]", filler(33))),
        "overlong graph node" => document(
            &format!("graph TD\n    Alpha[{}]\n{ACCESSIBLE_CLASS}", filler(33)),
            Fence::Backtick,
        ),
        "overlong class node" => document(
            &format!("classDiagram\n    class {}", filler(33)),
            Fence::Backtick,
        ),
        "overlong state node" => document(
            &format!("stateDiagram\n    state \"{}\" as alpha", filler(33)),
            Fence::Backtick,
        ),
        "overlong state-v2 node" => document(
            &format!("stateDiagram-v2\n    state \"{}\" as alpha", filler(33)),
            Fence::Backtick,
        ),
        "overlong ER entity" => document(
            &format!("erDiagram\n    {} ||--|| BETA : links", filler(33)),
            Fence::Backtick,
        ),
        "overlong requirement node" => document(
            &format!(
                "requirementDiagram\n    requirement {} {{\n        id: 1\n        text: short\n    }}",
                filler(33)
            ),
            Fence::Backtick,
        ),
        "overlong block node" => document(
            &format!("block\n    columns 1\n    Alpha[\"{}\"]", filler(33)),
            Fence::Backtick,
        ),

        // -- Legibility, at and past the declared boundary ---------------------
        "node label at the declared limit" => flowchart(&format!("    Alpha[{}]", filler(32))),
        "node label one past the limit" => flowchart(&format!("    Alpha[{}]", filler(33))),
        "edge label at the declared limit" => {
            flowchart(&format!("    Alpha -->|{}| Beta", filler(24)))
        }
        "edge label one past the limit" => {
            flowchart(&format!("    Alpha -->|{}| Beta", filler(25)))
        }
        "text edge one past the limit" => {
            flowchart(&format!("    Alpha -- {} --> Beta", filler(25)))
        }
        // A break splits a label into segments, and the limit is per visible
        // segment, so twice the limit across two segments is legible.
        "split node labels at the boundary" => {
            flowchart(&format!("    Alpha[{}<br>{}]", filler(32), filler(32)))
        }
        "split node labels with br" => {
            flowchart(&format!("    Alpha[{}<br/>{}]", filler(32), filler(32)))
        }
        "split node labels with newline" => {
            flowchart(&format!("    Alpha[{}\\n{}]", filler(32), filler(32)))
        }
        "combining grapheme node at boundary" => {
            flowchart(&format!("    Alpha[{}]", combining(32)))
        }
        "encoded node at boundary" => flowchart(&format!("    Alpha[{}]", encoded(32))),
        "state transition semicolon" => document(
            "stateDiagram-v2\n    [*] --> Alpha;\n    Alpha --> [*]",
            Fence::Backtick,
        ),
        // Everything here is longer than the limit and none of it is a label a
        // reader ever sees.
        "legibility exclusions" => document(
            &format!(
                "flowchart LR\n    %% {}\n    click Alpha \"https://example.com/{}\"\n    classDef {} fill:#0173B2,stroke:#000000,color:#FFFFFF\n    Alpha[Alpha]\n    class Alpha {}",
                filler(60),
                filler(60),
                filler(40),
                filler(40)
            ),
            Fence::Backtick,
        ),

        _ => return None,
    };

    Some(text)
}
