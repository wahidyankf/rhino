//! `ose/repo-config/v2`: the portable governance schema and its gates.
//!
//! A second schema beside v1 rather than a replacement. Five consumers run v1
//! today, and a reader that stopped understanding it would turn every recorded
//! vector into a configuration error and call that preservation. The two are
//! told apart by where the schema is declared -- v1 in a leading comment, v2 in
//! a leading key -- so no document can be read as both, and one carrying both
//! is a stated fault rather than a coin toss.
//!
//! The document is read here rather than decoded by serde, for the same reason
//! metadata front matter is: a decoder keeps one of two duplicate keys, forgets
//! which line a key was written on, and forgets what order the keys came in.
//! This schema has a hard-failing rule about each of those three.

use super::{ConfigError, TierMapping, listed};
use std::collections::BTreeMap;

pub const SCHEMA: &str = "ose/repo-config/v2";

/// Canonical top-level key order. The list *is* the order.
///
/// The validator sections sit between the portable declarations and `gates`,
/// and keep the names v1 gave them. Two spellings for one rule would make a
/// repository moving between schemas rewrite policy it did not change, and
/// would give the shared contract two places to be edited.
const KEYS: [&str; 18] = [
    "schema",
    "visibility",
    "governance",
    "model-tiers",
    "scan",
    "harness-parity",
    "metadata",
    "governance-word-budget",
    "governance-directory-map",
    "md-frontmatter",
    "md-heading-hierarchy",
    "md-internal-link",
    "md-mermaid",
    "md-naming",
    "md-readme-index",
    "convention-emoji",
    "gates",
    "extensions",
];

/// The keys that carry a validator section, in canonical order.
///
/// Every one is optional. A command whose section is absent refuses rather than
/// enforcing a rule the repository never wrote down -- the same contract v1's
/// optional sections have carried since `v0.1`.
pub const SECTIONS: [&str; 12] = [
    "scan",
    "harness-parity",
    "metadata",
    "governance-word-budget",
    "governance-directory-map",
    "md-frontmatter",
    "md-heading-hierarchy",
    "md-internal-link",
    "md-mermaid",
    "md-naming",
    "md-readme-index",
    "convention-emoji",
];

/// The three keys a v2 document may never omit.
const REQUIRED: [&str; 3] = ["schema", "visibility", "gates"];

/// Canonical gate-entry key order, and the whole set a gate entry may keep.
const ENTRY_KEYS: [&str; 4] = ["id", "kind", "run", "surfaces"];

/// Canonical surface order. `ci` is last because it is the only optional one.
pub const SURFACES: [&str; 4] = ["commit-msg", "pre-commit", "pre-push", "ci"];

/// The surfaces every repository carries, whatever it is hosted on.
const HOOK_SURFACES: [&str; 3] = ["commit-msg", "pre-commit", "pre-push"];

const KINDS: [&str; 2] = ["check", "mutation"];

/// The one surface a mutation may run at.
const MUTATION_SURFACE: &str = "pre-commit";

/// The gate a public repository runs before anything else, everywhere.
const PUBLIC_SAFETY: &str = "public-safety";

const VISIBILITIES: [&str; 2] = ["public", "private"];

/// The governance layers a local category may extend.
pub const LAYERS: [&str; 5] = [
    "conventions",
    "development",
    "principles",
    "vision",
    "workflows",
];

/// A read document: what a repository declared, and where it said it.
#[derive(Debug, Default)]
pub struct Document {
    /// The validator sections this document declared.
    ///
    /// Decoded from the same text by the same reader v1 uses, so a rule has one
    /// implementation and a repository writes its policy in one spelling
    /// whichever schema it declares. Every section is optional here; a command
    /// whose section is absent refuses rather than defaulting.
    pub sections: Box<super::Config>,
    pub visibility: String,
    /// The `<layer>/<category>` values this repository declared as its own.
    ///
    /// Kept rather than merely checked, because the declaration is what a
    /// governance walk compares a directory against: a category outside the
    /// shared registry is accepted only where the repository said so, and a
    /// reader that discarded the list would have to infer it from the tree --
    /// which is the inference the whole rule exists to prevent.
    pub local_categories: Vec<String>,
    pub gates: Vec<Gate>,
}

#[derive(Debug, Clone)]
pub struct Gate {
    pub id: String,
    pub kind: String,
    pub run: Vec<String>,
    pub surfaces: Vec<String>,
}

impl Gate {
    /// Whether this gate is selected by a surface.
    pub fn runs_at(&self, surface: &str) -> bool {
        self.surfaces.iter().any(|declared| declared == surface)
    }
}

// -- The reader ---------------------------------------------------------------

/// One value, and the line it was written on.
#[derive(Debug, Clone)]
struct Node {
    line: usize,
    value: Value,
}

#[derive(Debug, Clone)]
enum Value {
    Scalar(String),
    Mapping(Vec<(String, Node)>),
    List(Vec<Node>),
    /// A key written with nothing after it and nothing under it.
    Empty,
}

impl Value {
    fn mapping(&self) -> Option<&[(String, Node)]> {
        match self {
            Value::Mapping(entries) => Some(entries),
            _ => None,
        }
    }

    fn list(&self) -> Option<&[Node]> {
        match self {
            Value::List(items) => Some(items),
            _ => None,
        }
    }

    fn scalar(&self) -> Option<&str> {
        match self {
            Value::Scalar(text) => Some(text),
            _ => None,
        }
    }

    /// Whether the value carries nothing at all: an omitted body, an empty
    /// flow collection, or an empty block.
    fn is_empty(&self) -> bool {
        match self {
            Value::Empty => true,
            Value::Mapping(entries) => entries.is_empty(),
            Value::List(items) => items.is_empty(),
            Value::Scalar(_) => false,
        }
    }
}

/// One significant line, with its list marker already folded into its indent.
struct Line {
    indent: usize,
    text: String,
    number: usize,
    item: bool,
}

fn significant(text: &str) -> Vec<Line> {
    text.lines()
        .enumerate()
        .filter_map(|(index, raw)| {
            let trimmed = raw.trim();
            if trimmed.is_empty() || trimmed.starts_with('#') {
                return None;
            }
            let indent = raw.len() - raw.trim_start().len();
            // A list marker is folded into the indentation, so `- id: hygiene`
            // followed by `  kind: check` reads as one mapping and needs no
            // special case anywhere below.
            let (indent, text, item) = match trimmed.strip_prefix("- ") {
                Some(rest) => (indent + 2, rest.trim().to_string(), true),
                None if trimmed == "-" => (indent + 2, String::new(), true),
                None => (indent, trimmed.to_string(), false),
            };
            Some(Line {
                indent,
                text,
                number: index + 1,
                item,
            })
        })
        .collect()
}

fn block(lines: &[Line], start: usize, indent: usize) -> (Value, usize) {
    if lines[start].item {
        list(lines, start, indent)
    } else {
        mapping(lines, start, indent)
    }
}

fn list(lines: &[Line], mut index: usize, indent: usize) -> (Value, usize) {
    let mut items = Vec::new();
    while index < lines.len() && lines[index].indent == indent && lines[index].item {
        let number = lines[index].number;
        let (value, next) = if key_of(&lines[index].text).is_some() {
            mapping(lines, index, indent)
        } else {
            (scalar(&lines[index].text), index + 1)
        };
        items.push(Node {
            line: number,
            value,
        });
        index = next;
    }
    (Value::List(items), index)
}

fn mapping(lines: &[Line], mut index: usize, indent: usize) -> (Value, usize) {
    let mut entries: Vec<(String, Node)> = Vec::new();
    let first = index;
    while index < lines.len()
        && lines[index].indent == indent
        && (index == first || !lines[index].item)
    {
        let number = lines[index].number;
        let Some((key, rest)) = key_of(&lines[index].text) else {
            // Not a key at all. Kept as a key with no value so the rule that
            // refuses unknown keys can name what was written.
            entries.push((
                lines[index].text.clone(),
                Node {
                    line: number,
                    value: Value::Empty,
                },
            ));
            index += 1;
            continue;
        };
        index += 1;
        let value = if rest.is_empty() {
            if index < lines.len() && lines[index].indent > indent {
                let (nested, next) = block(lines, index, lines[index].indent);
                index = next;
                nested
            } else {
                Value::Empty
            }
        } else {
            scalar(&rest)
        };
        entries.push((
            key,
            Node {
                line: number,
                value,
            },
        ));
    }
    (Value::Mapping(entries), index)
}

/// Split `key: rest` at the first colon that ends a word.
///
/// The colon has to be followed by a space or the end of the line, so
/// `run: ./gates/check.sh --at 09:00` keeps its time in the value.
fn key_of(text: &str) -> Option<(String, String)> {
    let bytes = text.as_bytes();
    let position = (0..bytes.len()).find(|index| {
        bytes[*index] == b':' && bytes.get(index + 1).is_none_or(|next| *next == b' ')
    })?;
    let key = text[..position].trim();
    if key.is_empty() {
        return None;
    }
    Some((unquote(key), text[position + 1..].trim().to_string()))
}

/// A value written on the same line as its key: a flow collection or a scalar.
fn scalar(text: &str) -> Value {
    if text.starts_with('{') || text.starts_with('[') {
        let (value, _) = flow(text.as_bytes(), 0);
        return value;
    }
    Value::Scalar(unquote(text))
}

/// Read one flow value, returning it and the index just past it.
fn flow(bytes: &[u8], mut index: usize) -> (Value, usize) {
    let (close, mapped) = match bytes.get(index) {
        Some(b'{') => (b'}', true),
        Some(b'[') => (b']', false),
        _ => {
            let start = index;
            while index < bytes.len() && !matches!(bytes[index], b',' | b'}' | b']') {
                index += 1;
            }
            let text = String::from_utf8_lossy(&bytes[start..index])
                .trim()
                .to_string();
            return (Value::Scalar(unquote(&text)), index);
        }
    };
    index += 1;

    let mut entries: Vec<(String, Node)> = Vec::new();
    let mut items: Vec<Node> = Vec::new();
    loop {
        while bytes
            .get(index)
            .is_some_and(|byte| matches!(byte, b' ' | b','))
        {
            index += 1;
        }
        // A collection ends at its closing byte or at the end of the text.
        // Treated as one fact rather than two, because a value written without
        // its closing byte still ends where the text does.
        if bytes.get(index).is_none_or(|byte| *byte == close) {
            index += usize::from(index < bytes.len());
            break;
        }
        if mapped {
            let start = index;
            while index < bytes.len() && bytes[index] != b':' {
                index += 1;
            }
            let key = String::from_utf8_lossy(&bytes[start..index])
                .trim()
                .to_string();
            index += usize::from(index < bytes.len());
            while bytes.get(index) == Some(&b' ') {
                index += 1;
            }
            let (value, next) = flow(bytes, index);
            index = next;
            entries.push((unquote(&key), Node { line: 0, value }));
        } else {
            let (value, next) = flow(bytes, index);
            index = next;
            items.push(Node { line: 0, value });
        }
    }
    let value = if mapped {
        Value::Mapping(entries)
    } else {
        Value::List(items)
    };
    (value, index)
}

fn unquote(text: &str) -> String {
    for quote in ['\'', '"'] {
        if text.len() >= 2 && text.starts_with(quote) && text.ends_with(quote) {
            return text[1..text.len() - 1].to_string();
        }
    }
    text.to_string()
}

// -- The rules ----------------------------------------------------------------

/// The schema a document declares as a top-level key, wherever it sits.
///
/// Found anywhere rather than only first, so that a document declaring `schema`
/// in the wrong position is refused for the order it wrote its keys in. Looking
/// only at the first line would have made the same document read as declaring
/// no schema at all -- a true statement about the first line and a useless one
/// about the document.
pub fn declares(text: &str) -> Option<String> {
    significant(text)
        .iter()
        .filter(|line| line.indent == 0 && !line.item)
        .find_map(|line| {
            let (key, rest) = key_of(&line.text)?;
            (key == "schema").then(|| unquote(rest.trim()))
        })
}

/// Read and check a whole v2 document.
pub fn parse(text: &str) -> Result<Document, ConfigError> {
    // Read from the outermost indentation rather than from whatever the first
    // line happens to sit at. A document is only parsed once a top-level
    // `schema` key has been found in it, so the top level is where the keys
    // are; anything written further in is not a top-level key and is refused
    // for being absent rather than accepted for being indented.
    let lines = significant(text);
    let (root, _) = mapping(&lines, 0, 0);
    let entries = root.mapping().unwrap_or_default();

    keys_are_known_and_ordered(entries, &KEYS, "is not a key this schema keeps")?;
    required_keys_are_present(entries)?;
    optional_sections_carry_something(entries)?;

    let visibility = visibility(entries)?;
    let local_categories = governance(entries)?;
    model_tiers(entries)?;
    extensions(entries)?;
    let gates = gates(entries)?;
    public_safety(&visibility, &gates, entries)?;

    // Decoded after the structural rules pass, so a document with a duplicate
    // key or an out-of-order section is reported as that rather than as
    // whatever the decoder made of it.
    let sections = super::decode_sections(text)?;

    Ok(Document {
        sections: Box::new(sections),
        visibility,
        local_categories,
        gates,
    })
}

fn refuse(key: &str, line: usize, reason: &str) -> ConfigError {
    ConfigError::Semantic {
        key: key.to_string(),
        line,
        reason: reason.to_string(),
    }
}

fn find<'a>(entries: &'a [(String, Node)], key: &str) -> Option<&'a Node> {
    entries
        .iter()
        .find(|(name, _)| name == key)
        .map(|(_, node)| node)
}

/// Every key belongs to the schema, appears once, and appears in order.
fn keys_are_known_and_ordered(
    entries: &[(String, Node)],
    canonical: &[&str],
    unknown: &str,
) -> Result<(), ConfigError> {
    let mut seen: Vec<&str> = Vec::new();
    let mut highest = 0usize;
    for (key, node) in entries {
        let Some(position) = canonical.iter().position(|known| known == key) else {
            return Err(refuse(key, node.line, unknown));
        };
        if seen.contains(&key.as_str()) {
            return Err(refuse(key, node.line, "is declared twice"));
        }
        if position < highest {
            return Err(refuse(
                key,
                node.line,
                &format!(
                    "is out of canonical key order; the order is {}",
                    listed(canonical)
                ),
            ));
        }
        highest = position;
        seen.push(key);
    }
    Ok(())
}

fn required_keys_are_present(entries: &[(String, Node)]) -> Result<(), ConfigError> {
    for key in REQUIRED {
        if find(entries, key).is_none() {
            return Err(refuse(key, 1, "the schema requires it"));
        }
    }
    Ok(())
}

/// An empty optional section and an omitted one would mean the same thing to a
/// generator, which is why the empty one is refused: only the omission is an
/// answer a repository can be held to.
fn optional_sections_carry_something(entries: &[(String, Node)]) -> Result<(), ConfigError> {
    for key in ["governance", "model-tiers", "extensions"]
        .into_iter()
        .chain(SECTIONS)
    {
        if let Some(node) = find(entries, key)
            && node.value.is_empty()
        {
            return Err(refuse(key, node.line, "is declared with nothing in it"));
        }
    }
    Ok(())
}

fn visibility(entries: &[(String, Node)]) -> Result<String, ConfigError> {
    let node = find(entries, "visibility").expect("a required key is present");
    let declared = node.value.scalar().unwrap_or_default();
    if !VISIBILITIES.contains(&declared) {
        return Err(refuse(
            "visibility",
            node.line,
            "is neither public nor private, and an offline validator cannot infer hosted visibility from a remote or a network call",
        ));
    }
    Ok(declared.to_string())
}

fn governance(entries: &[(String, Node)]) -> Result<Vec<String>, ConfigError> {
    let Some(node) = find(entries, "governance") else {
        return Ok(Vec::new());
    };
    let section = node.value.mapping().unwrap_or_default();
    keys_are_known_and_ordered(
        section,
        &["local-categories"],
        "is not a key the governance section keeps",
    )?;
    let Some(declared) = find(section, "local-categories") else {
        return Err(refuse(
            "governance",
            node.line,
            "keeps only `local-categories`, and declares none",
        ));
    };
    if declared.value.is_empty() {
        return Err(refuse(
            "local-categories",
            declared.line,
            "is declared with nothing in it",
        ));
    }

    let mut previous: Option<String> = None;
    let mut declared_categories: Vec<String> = Vec::new();
    for item in declared.value.list().unwrap_or_default() {
        let category = item.value.scalar().unwrap_or_default();
        if !is_governed_category(category) {
            return Err(refuse(
                category,
                declared.line,
                &format!(
                    "is not a governed layer and category; the canonical layers are {}, and a category is written in lowercase with hyphens",
                    listed(&LAYERS)
                ),
            ));
        }
        match &previous {
            Some(earlier) if earlier == category => {
                return Err(refuse(category, declared.line, "is declared twice"));
            }
            Some(earlier) if earlier.as_str() > category => {
                return Err(refuse(category, declared.line, "is out of sorted order"));
            }
            _ => {}
        }
        previous = Some(category.to_string());
        declared_categories.push(category.to_string());
    }
    Ok(declared_categories)
}

fn is_governed_category(value: &str) -> bool {
    let Some((layer, category)) = value.split_once('/') else {
        return false;
    };
    LAYERS.contains(&layer) && is_portable_name(category)
}

fn is_portable_name(value: &str) -> bool {
    !value.is_empty()
        && !value.starts_with('-')
        && !value.ends_with('-')
        && value.chars().all(|character| {
            character.is_ascii_lowercase() || character.is_ascii_digit() || character == '-'
        })
}

/// The tier map, held to exactly the contract the v1 section is held to.
fn model_tiers(entries: &[(String, Node)]) -> Result<(), ConfigError> {
    let Some(node) = find(entries, "model-tiers") else {
        return Ok(());
    };
    let mut harnesses: BTreeMap<String, BTreeMap<String, Option<TierMapping>>> = BTreeMap::new();
    for (harness, tiers) in node.value.mapping().unwrap_or_default() {
        let mut mapped: BTreeMap<String, Option<TierMapping>> = BTreeMap::new();
        for (tier, mapping) in tiers.value.mapping().unwrap_or_default() {
            let fields = mapping.value.mapping().unwrap_or_default();
            let held = if mapping.value.is_empty() {
                None
            } else {
                Some(TierMapping {
                    model: find(fields, "model")
                        .map(|held| held.value.scalar().unwrap_or_default().to_string()),
                    effort: find(fields, "effort")
                        .map(|held| held.value.scalar().unwrap_or_default().to_string()),
                })
            };
            mapped.insert(tier.clone(), held);
        }
        harnesses.insert(harness.clone(), mapped);
    }
    super::check_tier_map(&harnesses, node.line)
}

/// An extension namespace is a namespace. Core reads no meaning inside one.
fn extensions(entries: &[(String, Node)]) -> Result<(), ConfigError> {
    let Some(node) = find(entries, "extensions") else {
        return Ok(());
    };
    for (profile, payload) in node.value.mapping().unwrap_or_default() {
        if payload.value.mapping().is_none() {
            return Err(refuse(
                profile,
                payload.line,
                "is not an extension namespace; a profile owns a mapping, and core reads no meaning inside one",
            ));
        }
    }
    Ok(())
}

fn gates(entries: &[(String, Node)]) -> Result<Vec<Gate>, ConfigError> {
    let node = find(entries, "gates").expect("a required key is present");
    let items = node.value.list().unwrap_or_default();
    if items.is_empty() {
        return Err(refuse(
            "gates",
            node.line,
            "declares no gate, and a repository enforcing nothing says so by not adopting this schema",
        ));
    }

    let mut gates: Vec<Gate> = Vec::new();
    for item in items {
        let fields = item.value.mapping().unwrap_or_default();
        keys_are_known_and_ordered(fields, &ENTRY_KEYS, "is not a key a gate entry keeps")?;
        for key in ENTRY_KEYS {
            if find(fields, key).is_none() {
                return Err(refuse(
                    key,
                    item.line,
                    "the gate entry contract requires it",
                ));
            }
        }

        let id = entry_scalar(fields, "id");
        let identifier = find(fields, "id").expect("a required key is present");
        if !is_portable_name(&id) {
            return Err(refuse(
                "id",
                identifier.line,
                "is not a lowercase-hyphen identifier",
            ));
        }
        if gates.iter().any(|earlier| earlier.id == id) {
            return Err(refuse(
                "id",
                identifier.line,
                &format!("{id} is declared twice, and a gate is named once"),
            ));
        }

        let kind = entry_scalar(fields, "kind");
        let declared_kind = find(fields, "kind").expect("a required key is present");
        if !KINDS.contains(&kind.as_str()) {
            return Err(refuse(
                "kind",
                declared_kind.line,
                "is neither check nor mutation",
            ));
        }

        let run = find(fields, "run").expect("a required key is present");
        let Some(vector) = run.value.list() else {
            return Err(refuse(
                "run",
                run.line,
                "is not an argument vector; a gate is executed directly, so a shell string would be a command nothing runs",
            ));
        };
        if vector.is_empty() {
            return Err(refuse("run", run.line, "is declared with nothing in it"));
        }

        let surfaces = find(fields, "surfaces").expect("a required key is present");
        let declared = surfaces.value.list().unwrap_or_default();
        if declared.is_empty() {
            return Err(refuse(
                "surfaces",
                surfaces.line,
                "is declared with nothing in it",
            ));
        }
        let mut named: Vec<String> = Vec::new();
        let mut highest = 0usize;
        for item in declared {
            let surface = item.value.scalar().unwrap_or_default();
            let Some(position) = SURFACES.iter().position(|known| *known == surface) else {
                return Err(refuse(
                    surface,
                    surfaces.line,
                    &format!(
                        "is not a surface this schema keeps; the closed set is {}",
                        listed(&SURFACES)
                    ),
                ));
            };
            if named.iter().any(|earlier| earlier == surface) {
                return Err(refuse(surface, surfaces.line, "is declared twice"));
            }
            if position < highest {
                return Err(refuse(
                    surface,
                    surfaces.line,
                    &format!(
                        "is out of canonical surface order; the order is {}",
                        listed(&SURFACES)
                    ),
                ));
            }
            highest = position;
            named.push(surface.to_string());
        }
        if kind == "mutation" && named.iter().any(|surface| surface != MUTATION_SURFACE) {
            return Err(refuse(
                "surfaces",
                surfaces.line,
                "a mutation may run only at pre-commit, where it owns its own safe restaging",
            ));
        }

        gates.push(Gate {
            id,
            kind,
            run: vector
                .iter()
                .map(|argument| argument.value.scalar().unwrap_or_default().to_string())
                .collect(),
            surfaces: named,
        });
    }

    // Every repository carries all three Git hook surfaces. A remote-free one
    // mans `pre-push` with a first-party reject gate, which makes a forbidden
    // push fail explicitly instead of letting a missing surface read as a
    // waiver. `ci` stays optional: a repository with no hosted automation has
    // nothing to dispatch there.
    let gates_node = node;
    for surface in HOOK_SURFACES {
        if !gates.iter().any(|gate| gate.runs_at(surface)) {
            return Err(refuse(
                surface,
                gates_node.line,
                "no gate runs at this surface, and every repository carries all three Git hooks",
            ));
        }
    }
    Ok(gates)
}

fn entry_scalar(fields: &[(String, Node)], key: &str) -> String {
    find(fields, key)
        .and_then(|node| node.value.scalar())
        .unwrap_or_default()
        .to_string()
}

/// A public repository scans before it does anything else, on every surface it
/// dispatches. Position is the rule, not presence: a safety gate that runs
/// second has already let something else touch the publication surface.
fn public_safety(
    visibility: &str,
    gates: &[Gate],
    entries: &[(String, Node)],
) -> Result<(), ConfigError> {
    if visibility != "public" {
        return Ok(());
    }
    let node = find(entries, "gates").expect("a required key is present");
    if !gates.iter().any(|gate| gate.id == PUBLIC_SAFETY) {
        return Err(refuse(
            "gates",
            node.line,
            "declares no public-safety gate, and a public repository scans every publication surface first",
        ));
    }
    for surface in SURFACES {
        let mut selected = gates.iter().filter(|gate| gate.runs_at(surface));
        match selected.next() {
            None => {}
            Some(first) if first.id == PUBLIC_SAFETY => {}
            Some(_) => {
                return Err(refuse(
                    surface,
                    node.line,
                    "public-safety does not run first at this surface",
                ));
            }
        }
    }
    Ok(())
}
