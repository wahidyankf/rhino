//! Canonical metadata validation.
//!
//! Four schemas -- governance, workflow, skill, agent -- selected by the
//! artifact's path. The repository declares which of its trees carry which
//! schema, because RHINO ships no path; the schemas themselves are the shared
//! contract, because a repository able to redefine what an agent declares
//! would have adopted nothing.
//!
//! The front matter is read here rather than decoded. A YAML decoder discards
//! one of two duplicate keys, forgets which line a key was on, and cannot tell
//! a folded scalar from a plain one -- and all three are things this contract
//! has a rule about. What a decoder would give back is exactly what the rules
//! need to see before it happens.

use crate::config::{Config, Metadata, MetadataSchema, MetadataSurface};
use crate::report::{Finding, Report};
use crate::runtime::Tree;
use crate::scan::{self, Corpus};

/// The portable capability vocabulary, in its canonical order.
///
/// Order is part of the vocabulary rather than a formatting preference: a list
/// that may be written in any order is a list two repositories will write two
/// ways, and every diff between them is then about nothing.
const CAPABILITIES: [&str; 5] = [
    "repository-read",
    "repository-write",
    "shell",
    "network",
    "subagent",
];

/// The bounds the shared value rules put on the two routing fields.
const DESCRIPTION_MIN: usize = 20;
const DESCRIPTION_MAX: usize = 300;
const TRIGGER_MIN: usize = 20;
const TRIGGER_MAX: usize = 240;
const TRIGGER_MAX_SENTENCES: usize = 3;

/// One family's keys, in canonical order, required before optional.
struct Schema {
    required: &'static [&'static str],
    optional: &'static [&'static str],
}

impl Schema {
    fn of(kind: MetadataSchema) -> Self {
        match kind {
            // No `name`: the path is the identity, and repeating it in the
            // document is one more place for the two to disagree.
            MetadataSchema::Governance => Self {
                required: &["description", "when_to_use"],
                optional: &[],
            },
            MetadataSchema::Workflow => Self {
                required: &["name", "description", "when_to_use"],
                optional: &[],
            },
            MetadataSchema::Skill => Self {
                required: &["name", "description", "when_to_use"],
                optional: &["compatibility"],
            },
            MetadataSchema::Agent => Self {
                required: &["name", "description", "when_to_use", "tier", "capabilities"],
                optional: &["skills", "constraints"],
            },
        }
    }

    fn position(&self, key: &str) -> Option<usize> {
        self.required
            .iter()
            .chain(self.optional)
            .position(|known| *known == key)
    }
}

pub fn validate(tree: &dyn Tree, config: &Config, metadata: &Metadata) -> Report {
    let surfaces = &metadata.surfaces;
    let globs = match scan::Surfaces::compile(
        "metadata.surfaces",
        surfaces.iter().map(|surface| surface.glob.as_str()),
    ) {
        Ok(globs) => globs,
        Err(reason) => return Report::refused("metadata", reason),
    };
    let corpus = match Corpus::read(tree, config) {
        Ok(corpus) => corpus,
        Err(reason) => return Report::refused("metadata", reason),
    };

    let mut report = Report::new("metadata", "file");

    for document in corpus.documents() {
        let Some(surface): Option<&MetadataSurface> = globs
            .governing(&document.path)
            .and_then(|index| surfaces.get(index))
        else {
            continue;
        };

        report.scanned(&document.path);

        for finding in inspect(&document.path, &document.text, surface.schema) {
            report.found(finding);
        }
    }

    report
}

/// Every rule this contract states, applied to one artifact.
fn inspect(path: &str, text: &str, kind: MetadataSchema) -> Vec<Finding> {
    let schema = Schema::of(kind);
    let mut findings = Vec::new();

    let (entries, malformed, block) = match read(text) {
        Head::Unterminated => {
            return vec![
                Finding::new(
                    "metadata-frontmatter-unterminated",
                    path,
                    "the front-matter block opens and never closes, so no key in it can be trusted",
                )
                .at(1, 1),
            ];
        }
        // An absent block and an empty one are the same to every rule below:
        // every required key is missing from both, and saying so once per key
        // is what tells a reader what to write.
        // A document with no block reports its missing keys at line 1. Reading
        // the position of some later `---` would point a maintainer at a
        // horizontal rule and call it front matter.
        Head::Missing => (Vec::new(), Vec::new(), 1),
        Head::Entries(entries, malformed) => (entries, malformed, block_line(text)),
    };

    for line in malformed {
        findings.push(
            Finding::new(
                "metadata-frontmatter-malformed",
                path,
                "declares no key; every line inside the block is a declaration",
            )
            .at(line, 1),
        );
    }

    let mut seen: Vec<&Entry> = Vec::new();

    for entry in &entries {
        if let Some(first) = seen.iter().find(|held| held.key == entry.key) {
            findings.push(
                Finding::new(
                    "metadata-duplicate-key",
                    path,
                    format!(
                        "is declared again, and a decoder would silently keep one of the two (first at line {})",
                        first.line
                    ),
                )
                .at(entry.line, 1)
                .about(&entry.key),
            );
            continue;
        }
        seen.push(entry);

        if schema.position(&entry.key).is_none() {
            findings.push(
                Finding::new(
                    "metadata-unknown-key",
                    path,
                    "is not a key this artifact's schema declares; the path already supplies every identity the schema omits",
                )
                .at(entry.line, 1)
                .about(&entry.key),
            );
        }
    }

    findings.extend(order_findings(path, &seen, &schema));
    findings.extend(missing_findings(path, block, &seen, &schema));
    findings.extend(value_findings(path, &seen, kind));

    findings
}

/// The first key that appears after a key the schema puts later.
///
/// Reported once rather than for every key after it: a document with two keys
/// transposed has one defect, and naming the whole tail would bury it.
fn order_findings(path: &str, entries: &[&Entry], schema: &Schema) -> Vec<Finding> {
    let mut highest = 0usize;
    for entry in entries {
        let Some(position) = schema.position(&entry.key) else {
            continue;
        };
        if position < highest {
            return vec![
                Finding::new(
                    "metadata-key-order",
                    path,
                    "appears after a key the schema orders later; canonical key order is hard-failing",
                )
                .at(entry.line, 1)
                .about(&entry.key),
            ];
        }
        highest = position;
    }
    Vec::new()
}

fn missing_findings(path: &str, block: usize, entries: &[&Entry], schema: &Schema) -> Vec<Finding> {
    schema
        .required
        .iter()
        .filter(|key| !entries.iter().any(|entry| entry.key == **key))
        .map(|key| {
            Finding::new(
                "metadata-required-key-missing",
                path,
                "is required by this artifact's schema and is not declared",
            )
            .at(block, 1)
            .about(*key)
        })
        .collect()
}

/// The keys whose value is a single scalar, and the keys whose value is a list.
///
/// Named rather than inferred from what a document happened to write, because
/// "this key holds a list" is a fact about the schema: a repository writing one
/// as the other has made a mistake the validator has to be able to state.
const LIST_KEYS: [&str; 3] = ["capabilities", "skills", "constraints"];

fn value_findings(path: &str, entries: &[&Entry], kind: MetadataSchema) -> Vec<Finding> {
    let mut findings = Vec::new();
    let mut description = None;

    for entry in entries {
        // A key with nothing behind it fails once, on the absence. Running the
        // value rules over a value that is not there would report the same
        // defect a second time in the vocabulary of a rule it never reached.
        match &entry.value {
            Value::Null => {
                findings.push(
                    Finding::new(
                        "metadata-null-value",
                        path,
                        "is declared as null, which is a key with no answer rather than an answer",
                    )
                    .at(entry.line, 1)
                    .about(&entry.key),
                );
                continue;
            }
            Value::Empty => {
                findings.push(
                    Finding::new(
                        "metadata-empty-value",
                        path,
                        "is declared with no value; an omitted key and an empty one have to stay different things",
                    )
                    .at(entry.line, 1)
                    .about(&entry.key),
                );
                continue;
            }
            _ => {}
        }

        if LIST_KEYS.contains(&entry.key.as_str()) {
            findings.extend(array(path, entry));
            continue;
        }

        let Value::Scalar { text, form } = &entry.value else {
            findings.push(
                Finding::new(
                    "metadata-scalar-required",
                    path,
                    "is declared as a list; this key holds one value",
                )
                .at(entry.line, 1)
                .about(&entry.key),
            );
            continue;
        };
        let normalized = normalize(text);

        match entry.key.as_str() {
            "description" => {
                // Plain or folded, and never literal: a literal block keeps the
                // author's line breaks, and a description travels through
                // adapters that render it as one line.
                if *form == Form::Literal {
                    findings.push(
                        Finding::new(
                            "metadata-description-form",
                            path,
                            "is written as a literal block; a description is a plain or folded scalar",
                        )
                        .at(entry.line, 1)
                        .about(&entry.key),
                    );
                }
                description = Some(normalized.clone());
                findings.extend(length(
                    path,
                    entry,
                    &normalized,
                    "metadata-description-length",
                    DESCRIPTION_MIN,
                    DESCRIPTION_MAX,
                ));
            }
            "when_to_use" => {
                if *form != Form::Folded {
                    findings.push(
                        Finding::new(
                            "metadata-folded-scalar-required",
                            path,
                            "has to be written as a folded scalar, so a multi-sentence trigger reads the same in every adapter",
                        )
                        .at(entry.line, 1)
                        .about(&entry.key),
                    );
                }
                findings.extend(length(
                    path,
                    entry,
                    &normalized,
                    "metadata-when-to-use-length",
                    TRIGGER_MIN,
                    TRIGGER_MAX,
                ));
                let sentences = sentences(&normalized);
                if sentences > TRIGGER_MAX_SENTENCES {
                    findings.push(
                        Finding::new(
                            "metadata-when-to-use-sentences",
                            path,
                            format!(
                                "carries {sentences} sentences; a routing trigger states at most {TRIGGER_MAX_SENTENCES}"
                            ),
                        )
                        .at(entry.line, 1)
                        .about(&entry.key),
                    );
                }
                if description.as_deref() == Some(normalized.as_str()) {
                    findings.push(
                        Finding::new(
                            "metadata-routing-not-distinct",
                            path,
                            "repeats the description, so it adds no trigger a harness could route on",
                        )
                        .at(entry.line, 1)
                        .about(&entry.key),
                    );
                }
            }
            "name" => {
                if !is_portable_name(&normalized) {
                    findings.push(
                        Finding::new(
                            "metadata-name-format",
                            path,
                            "is not lowercase alphanumeric words joined by single hyphens",
                        )
                        .at(entry.line, 1)
                        .about(&entry.key),
                    );
                } else if identity(path, kind).as_deref() != Some(normalized.as_str()) {
                    findings.push(
                        Finding::new(
                            "metadata-name-path-mismatch",
                            path,
                            "is not the identity its path already states",
                        )
                        .at(entry.line, 1)
                        .about(&entry.key),
                    );
                }
            }
            "tier" => findings.extend(tier(path, entry, &normalized)),
            // `compatibility` and every unknown key: the shared rules above are
            // all this contract states about them.
            _ => {}
        }
    }

    findings
}

/// The rules every declared array shares, plus the closed vocabulary and
/// canonical order the capability list adds.
fn array(path: &str, entry: &Entry) -> Vec<Finding> {
    let Value::List(items) = &entry.value else {
        return vec![
            Finding::new(
                "metadata-array-required",
                path,
                "is declared as a scalar; this key holds a list",
            )
            .at(entry.line, 1)
            .about(&entry.key),
        ];
    };
    if items.is_empty() {
        return vec![
            Finding::new(
                "metadata-array-empty",
                path,
                "is declared with no members, which is an absent answer written as a present key",
            )
            .at(entry.line, 1)
            .about(&entry.key),
        ];
    }

    let mut findings = Vec::new();
    let mut seen: Vec<&str> = Vec::new();
    let mut highest = 0usize;
    let capabilities = entry.key == "capabilities";

    for (line, member) in items {
        if seen.contains(&member.as_str()) {
            findings.push(
                Finding::new(
                    "metadata-array-duplicate",
                    path,
                    format!(
                        "lists `{member}` twice, and the second says nothing the first did not"
                    ),
                )
                .at(*line, 1)
                .about(&entry.key),
            );
            continue;
        }
        seen.push(member);

        if !capabilities {
            continue;
        }
        let Some(position) = CAPABILITIES.iter().position(|known| known == member) else {
            findings.push(
                Finding::new(
                    "metadata-capability-unknown",
                    path,
                    format!(
                        "lists `{member}`, which is not in the portable capability vocabulary; harness tool names are not portable"
                    ),
                )
                .at(*line, 1)
                .about(&entry.key),
            );
            continue;
        };
        if position < highest {
            findings.push(
                Finding::new(
                    "metadata-array-order",
                    path,
                    format!("lists `{member}` after a capability the vocabulary orders later"),
                )
                .at(*line, 1)
                .about(&entry.key),
            );
            continue;
        }
        highest = position;
    }

    findings
}

/// The tier a portable agent declares.
fn tier(path: &str, entry: &Entry, declared: &str) -> Vec<Finding> {
    if crate::config::TIERS.contains(&declared) {
        return Vec::new();
    }
    vec![
        Finding::new(
            "metadata-tier-unknown",
            path,
            "is not one of the four portable tiers, which name a normal workload rather than a model",
        )
        .at(entry.line, 1)
        .about(&entry.key),
    ]
}

fn length(
    path: &str,
    entry: &Entry,
    normalized: &str,
    kind: &'static str,
    low: usize,
    high: usize,
) -> Vec<Finding> {
    let counted = normalized.chars().count();
    if (low..=high).contains(&counted) {
        return Vec::new();
    }
    vec![
        Finding::new(
            kind,
            path,
            format!("is {counted} characters; this field holds {low} through {high}"),
        )
        .at(entry.line, 1)
        .about(&entry.key),
    ]
}

/// The identity a path already states, which a declared name has to match.
fn identity(path: &str, kind: MetadataSchema) -> Option<String> {
    let mut segments: Vec<&str> = path.split('/').collect();
    let file = segments.pop()?;
    let stem = file.rsplit_once('.').map_or(file, |(stem, _)| stem);
    match kind {
        // A skill is its directory: the file inside it is called SKILL.md in
        // every repository, so the file name says nothing.
        MetadataSchema::Skill => segments.last().map(|name| (*name).to_string()),
        // A workflow entrypoint README takes its name from the directory it
        // indexes, for the same reason.
        MetadataSchema::Workflow if file.eq_ignore_ascii_case(scan::README) => {
            segments.last().map(|name| (*name).to_string())
        }
        _ => Some(stem.to_string()),
    }
}

fn is_portable_name(value: &str) -> bool {
    !value.is_empty()
        && value.split('-').all(|word| {
            !word.is_empty()
                && word
                    .chars()
                    .all(|character| character.is_ascii_lowercase() || character.is_ascii_digit())
        })
}

/// Whitespace collapsed and trimmed, which is what "normalized" means for
/// every length and equality rule in this contract.
fn normalize(text: &str) -> String {
    text.split_whitespace().collect::<Vec<_>>().join(" ")
}

fn sentences(text: &str) -> usize {
    text.split_terminator(['.', '!', '?'])
        .filter(|part| !part.trim().is_empty())
        .count()
        .max(usize::from(!text.trim().is_empty()))
}

/// One declared key, and the line a reader will find it on.
struct Entry {
    line: usize,
    key: String,
    value: Value,
}

/// A declared value, kept in the form it was written in.
///
/// The form is retained because the contract has rules about it: a trigger has
/// to be a folded scalar and a description may not be a literal block, and a
/// decoder that returned a string would have thrown both away.
enum Value {
    Scalar { text: String, form: Form },
    List(Vec<(usize, String)>),
    Empty,
    Null,
}

#[derive(PartialEq, Eq)]
enum Form {
    Plain,
    Folded,
    Literal,
}

enum Head {
    Missing,
    /// A block that opened and never closed. Everything after the opener reads
    /// as front matter to the end of the file, so no key in it can be trusted.
    Unterminated,
    /// The declarations found, and the lines inside the block that declared
    /// nothing at all.
    Entries(Vec<Entry>, Vec<usize>),
}

/// The line the front-matter block opens on, which is where a key that is not
/// there is reported.
fn block_line(text: &str) -> usize {
    text.lines()
        .position(|line| line.trim_end() == FENCE)
        .map_or(1, |index| index + 1)
}

const FENCE: &str = "---";

fn read(text: &str) -> Head {
    let lines: Vec<&str> = text.lines().collect();
    let Some(open) = lines.iter().position(|line| line.trim_end() == FENCE) else {
        return Head::Missing;
    };
    // Only a leading block is front matter. A `---` further down is a
    // horizontal rule, and reading one as an opener would invent a schema
    // violation in the middle of a document.
    if lines[..open].iter().any(|line| !line.trim().is_empty()) {
        return Head::Missing;
    }
    let Some(close) = lines[open + 1..]
        .iter()
        .position(|line| line.trim_end() == FENCE)
    else {
        return Head::Unterminated;
    };

    let body = &lines[open + 1..open + 1 + close];
    let offset = open + 2;
    let mut entries = Vec::new();
    let mut index = 0usize;

    let mut malformed = Vec::new();

    while index < body.len() {
        let raw = body[index];
        if raw.trim().is_empty() {
            index += 1;
            continue;
        }
        // Every continuation belongs to the key above it and was consumed
        // there. One that reaches here is orphaned, and so is a line that
        // declares no key at all -- both are text inside a block whose whole
        // content is supposed to be declarations.
        let declaration = (!raw.starts_with(char::is_whitespace) && !raw.starts_with('-'))
            .then(|| raw.split_once(':'))
            .flatten();
        let Some((key, rest)) = declaration else {
            malformed.push(offset + index);
            index += 1;
            continue;
        };
        let line = offset + index;
        let rest = rest.trim();
        index += 1;

        let value = if let Some(form) = block_form(rest) {
            let (text, next) = indented(body, index);
            index = next;
            Value::Scalar { text, form }
        } else if rest.is_empty() {
            let (items, next) = members(body, index, offset);
            if next == index {
                Value::Empty
            } else {
                index = next;
                Value::List(items)
            }
        } else if rest == "null" || rest == "~" {
            Value::Null
        } else if rest == "[]" {
            Value::List(Vec::new())
        } else if rest == "''" || rest == "\"\"" {
            Value::Empty
        } else {
            Value::Scalar {
                text: unquote(rest).to_string(),
                form: Form::Plain,
            }
        };

        entries.push(Entry {
            line,
            key: key.trim().to_string(),
            value,
        });
    }

    Head::Entries(entries, malformed)
}

/// The block scalar a value opens, if it opens one.
fn block_form(rest: &str) -> Option<Form> {
    if rest.starts_with('>') {
        Some(Form::Folded)
    } else if rest.starts_with('|') {
        Some(Form::Literal)
    } else {
        None
    }
}

/// The indented block beneath a folded or literal scalar, joined the way a
/// reader reads it.
fn indented(body: &[&str], from: usize) -> (String, usize) {
    let mut collected: Vec<&str> = Vec::new();
    let mut index = from;
    while index < body.len() {
        let line = body[index];
        if !line.starts_with(char::is_whitespace) && !line.trim().is_empty() {
            break;
        }
        collected.push(line.trim());
        index += 1;
    }
    (collected.join(" ").trim().to_string(), index)
}

/// The `- ` members beneath a key, with the line each was written on.
fn members(body: &[&str], from: usize, offset: usize) -> (Vec<(usize, String)>, usize) {
    let mut items = Vec::new();
    let mut index = from;
    while index < body.len() {
        let line = body[index];
        let trimmed = line.trim();
        if trimmed.is_empty() {
            break;
        }
        let Some(member) = trimmed.strip_prefix("- ") else {
            break;
        };
        items.push((offset + index, unquote(member.trim()).to_string()));
        index += 1;
    }
    (items, index)
}

fn unquote(value: &str) -> &str {
    value
        .strip_prefix('"')
        .and_then(|rest| rest.strip_suffix('"'))
        .or_else(|| {
            value
                .strip_prefix('\'')
                .and_then(|rest| rest.strip_suffix('\''))
        })
        .unwrap_or(value)
}
