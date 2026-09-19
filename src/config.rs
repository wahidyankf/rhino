//! The repository configuration.
//!
//! Every value RHINO enforces is declared here by the consuming repository.
//! The tool ships no default for any of them, which is the whole point of the
//! extraction: a repository that declares nothing gets a configuration error,
//! never a borrowed assumption from whichever repository the validator grew up
//! in.
//!
//! Sections are named after the command path that reads them, with spaces
//! replaced by hyphens, so a reader can find a section from a command and back
//! again. Unknown *sections* are ignored, because a consuming repository may
//! keep other tools' policy in the same file; unknown *keys inside an owned
//! section* are refused, because there a typo is a policy that silently does
//! nothing.

use crate::v0_4;
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;
use std::fmt;

/// Where the configuration lives, relative to the repository root.
pub const PATH: &str = "repo-config.yml";

/// Why a configuration could not be used.
///
/// Every variant is exit code `2`. None of them degrade to a partial run: a
/// validator that skipped a tree because its configuration was malformed would
/// report a clean repository that was never checked.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ConfigError {
    Missing,
    Unreadable(String),
    SchemaUndeclared,
    SchemaUnrecognized {
        declared: String,
        line: usize,
    },
    Malformed(String),
    Semantic {
        key: String,
        line: usize,
        reason: String,
    },
}

impl fmt::Display for ConfigError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Missing => write!(
                formatter,
                "{PATH}: no configuration file at line 1: RHINO holds no defaults, so every value it enforces has to be declared here"
            ),
            Self::Unreadable(reason) => {
                write!(formatter, "{PATH}: cannot be read at line 1: {reason}")
            }
            Self::SchemaUndeclared => write!(
                formatter,
                "{PATH}: line 1 declares no schema: expected `schema: {}`",
                v0_4::config::SCHEMA
            ),
            Self::SchemaUnrecognized { declared, line } => write!(
                formatter,
                "{PATH}: line {line}: unrecognized schema `{declared}`: this stable build accepts only `{}` and no longer accepts predecessor configuration",
                v0_4::config::SCHEMA
            ),
            Self::Malformed(message) => write!(formatter, "{PATH}: {message}"),
            Self::Semantic { key, line, reason } => {
                write!(formatter, "{PATH}: line {line}: {key}: {reason}")
            }
        }
    }
}

impl Config {
    /// The directory names a grouped policy excludes from its shared scan.
    pub fn excluded(&self) -> &[String] {
        self.scan
            .as_ref()
            .map(|scan| scan.exclude_directories.as_slice())
            .unwrap_or(&[])
    }
}

/// Shared typed policy sections projected from the grouped stable document.
#[derive(Debug, Clone, Default, Deserialize)]
pub struct Config {
    #[serde(rename = "governance-word-budget", default)]
    pub word_budget: Option<WordBudget>,
    #[serde(rename = "governance-directory-map", default)]
    pub directory_map: Option<DirectoryMap>,
    #[serde(rename = "md-internal-link", default)]
    pub internal_link: InternalLink,
    #[serde(rename = "md-mermaid", default)]
    pub mermaid: Option<Mermaid>,
    /// Optional, like every section added after `v0.1`. Absent means the
    /// command that reads it refuses rather than enforcing a convention the
    /// repository never declared, which is what keeps a release additive: a
    /// consumer that declares nothing here is unaffected by the section
    /// existing.
    /// Optional, on the same rule as every section added after `v0.1`.
    #[serde(rename = "md-frontmatter", default)]
    pub frontmatter: Option<Frontmatter>,
    /// Optional, on the same rule as every section added after `v0.1`.
    #[serde(rename = "md-heading-hierarchy", default)]
    pub heading_hierarchy: Option<HeadingHierarchy>,
    #[serde(rename = "md-naming", default)]
    pub naming: Option<Naming>,
    /// Optional, on the same rule as every section added after `v0.1`.
    #[serde(rename = "md-readme-index", default)]
    pub readme_index: Option<ReadmeIndex>,
    /// Optional, on the same rule as every section added after `v0.1`.
    #[serde(rename = "convention-emoji", default)]
    pub emoji: Option<Emoji>,
    /// Where this repository's governed artifacts live, and which canonical
    /// schema each surface carries.
    ///
    /// The mapping is the repository's because RHINO ships no path. The four
    /// schemas are not: they are the shared contract, and a repository able to
    /// redefine what an agent declares would have adopted nothing.
    #[serde(default)]
    pub metadata: Option<Metadata>,
    #[serde(default)]
    pub scan: Option<Scan>,
}

/// The portable tiers a mapping may be keyed by.
///
/// Named for the workload rather than for a model, so the same four survive a
/// vendor renaming its lineup.
pub const TIERS: [&str; 4] = ["ultra", "plan", "execution", "fast"];

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct Metadata {
    /// Ordered, on the same rule as `governance-word-budget.surfaces`: where
    /// two globs match one file, the last matching entry wins. That is how a
    /// workflow subtree carries a different schema from the governance tree it
    /// sits inside.
    pub surfaces: Vec<MetadataSurface>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct MetadataSurface {
    pub glob: String,
    pub schema: MetadataSchema,
}

/// The four canonical artifact families.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "kebab-case")]
pub enum MetadataSchema {
    Governance,
    Workflow,
    Skill,
    Agent,
}

#[derive(Debug, Clone, Deserialize, Serialize, JsonSchema, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct WordBudget {
    /// What this repository means by a word.
    ///
    /// Required, because the two repositories being reconciled here already
    /// disagree: one counts runs of letters and digits, the other counts
    /// whitespace-separated fields, and one repository's `AGENTS.md` is 749
    /// words by its own rule and 905 by the other's. A tool that picked either
    /// would be enforcing a budget the repository never set.
    pub count: WordRule,
    /// Ordered: where two globs match one file, the last matching entry wins,
    /// which is how a specific surface overrides a general tree.
    pub surfaces: Vec<Surface>,
}

/// The two definitions of a word in use, each transcribed rather than designed.
#[derive(Debug, Clone, Copy, Deserialize, Serialize, JsonSchema, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum WordRule {
    /// A run of letters, marks, and digits, optionally joined to another such
    /// run by an apostrophe, hyphen, or underscore. Markdown syntax and
    /// punctuation count for nothing; a URL counts as its several parts.
    LettersAndDigits,
    /// Anything between two runs of whitespace. `**bold**` is one word, and so
    /// is a whole URL.
    WhitespaceSeparated,
}

#[derive(Debug, Clone, Deserialize, Serialize, JsonSchema, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct Surface {
    pub glob: String,
    /// Strictly greater than this is a finding: a file exactly at the
    /// declared count passes. RHINO holds no number of its own.
    pub fail: usize,
    #[serde(default)]
    pub warn: Option<usize>,
    #[serde(default)]
    pub target: Option<usize>,
}

#[derive(Debug, Clone, Deserialize, Serialize, JsonSchema, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct DirectoryMap {
    pub trees: Vec<Tree>,
}

#[derive(Debug, Clone, Deserialize, Serialize, JsonSchema, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct Tree {
    pub path: String,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct InternalLink {
    /// Globs excluded as link *sources*. Files under them stay valid targets.
    #[serde(rename = "exclude-sources", default)]
    pub exclude_sources: Vec<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct Mermaid {
    /// The one authoring rule this repository applies to conceptual diagrams.
    ///
    /// Optional, on the same rule as every section added after `v0.1`. Absent
    /// leaves both halves unchecked, which is what a repository that never
    /// adopted the rule already had -- and a default would be RHINO choosing an
    /// authoring style on a repository's behalf, which is the one thing the
    /// contract says a repository decides.
    #[serde(rename = "authoring-rule", default)]
    pub authoring_rule: Option<AuthoringRule>,
    #[serde(rename = "node-label-graphemes")]
    pub node_label_graphemes: usize,
    #[serde(rename = "edge-label-graphemes")]
    pub edge_label_graphemes: usize,
    #[serde(rename = "fill-colors")]
    pub fill_colors: Vec<String>,
    #[serde(rename = "edge-colors")]
    pub edge_colors: Vec<String>,
    #[serde(rename = "text-colors")]
    pub text_colors: Vec<String>,
}

/// The two diagram authoring rules, one of which a repository declares.
///
/// Closed, and closed on purpose: the contract's whole argument is that a
/// repository having both is the state in which a reader cannot predict what
/// they will get and no tool can check either rule.
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq, JsonSchema)]
#[serde(rename_all = "kebab-case")]
pub enum AuthoringRule {
    /// Mermaid, carrying both an accessible title and an accessible
    /// description.
    Rendered,
    /// ASCII with prose beside it, and no Mermaid at all.
    PlainText,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct Frontmatter {
    /// Ordered, last match wins, as everywhere else surfaces are declared.
    pub surfaces: Vec<FrontmatterSurface>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct FrontmatterSurface {
    pub glob: String,
    /// Keys that must be present. Required and may be empty, because a surface
    /// that requires nothing and a surface whose requirements were forgotten
    /// have to look different in the file.
    pub require: Vec<String>,
    /// Keys whose value must come from a declared set.
    ///
    /// Written `enum:` in the document, which is the word a reader of the
    /// configuration expects and a word Rust will not spell.
    #[serde(rename = "enum", default)]
    pub values: BTreeMap<String, Vec<String>>,
    /// Keys whose value must be an ISO calendar date.
    #[serde(rename = "iso-date", default)]
    pub iso_date: Vec<String>,
    /// Keys that may not appear.
    ///
    /// This is what lets a repository keep a rule RHINO knows nothing about --
    /// "this tree does not carry a date" -- without RHINO having to know what
    /// the rule is for.
    #[serde(default)]
    pub forbid: Vec<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct HeadingHierarchy {
    /// Which files are governed. Unordered in effect -- a surface here carries
    /// no policy of its own, so nothing depends on which one matched.
    pub surfaces: Vec<Globbed>,
    /// Whether a governed document holds exactly one level-1 heading.
    ///
    /// Required rather than defaulted: a repository whose documents are
    /// sections of a larger whole legitimately has none, and assuming either
    /// answer would enforce a structure nobody declared.
    #[serde(rename = "single-h1")]
    pub single_h1: bool,
    /// How far a heading may drop below the one before it. `1` is the strict
    /// reading; a larger number is a repository that has decided otherwise.
    #[serde(rename = "max-level-jump")]
    pub max_level_jump: usize,
}

/// A declared glob and nothing else.
///
/// Shared by the sections whose surfaces carry no policy of their own, so that
/// "a surface is a mapping with a `glob` key" is one statement rather than one
/// per section that happens to agree.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct Globbed {
    pub glob: String,
}

/// Files in which an emoji code point is a finding.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct Emoji {
    pub prohibited: Vec<Globbed>,
}

/// Which trees require a README in every directory.
///
/// The same `Tree` as `governance-directory-map` declares, because it is the
/// same kind of statement: a repository-relative root to walk. Two spellings of
/// one idea would be two places for a path rule to be checked differently.
#[derive(Debug, Clone, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadmeIndex {
    pub trees: Vec<Tree>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct Naming {
    /// Ordered, on the same rule as `governance-word-budget.surfaces`: where
    /// two globs match one file, the last matching entry wins.
    pub surfaces: Vec<NamingSurface>,
    /// Globs no style applies to. Required and may be empty, because the
    /// difference between "nothing is exempt" and "I forgot the exemptions" has
    /// to stay visible in the file.
    pub exempt: Vec<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct NamingSurface {
    pub glob: String,
    pub style: NameStyle,
    /// What joins an encoded directory prefix to the content name.
    ///
    /// Required by `path-prefixed` and refused by `kebab-case`. Optional here
    /// rather than in the type because the two keys are siblings in the
    /// document and `deny_unknown_fields` cannot be combined with the flattened
    /// enum that would make the pairing unrepresentable; the pairing is checked
    /// instead, and checked once.
    #[serde(default)]
    pub separator: Option<String>,
}

/// The two filename styles, each a rule about characters rather than a policy
/// about files.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "kebab-case")]
pub enum NameStyle {
    /// Lowercase alphanumeric runs joined by single hyphens.
    KebabCase,
    /// The file's own directory path, encoded, then the separator, then a
    /// kebab-case content name.
    PathPrefixed,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Scan {
    /// Directory *names*, matched at any depth. Filesystem links are always
    /// skipped regardless of this list, because following one can escape the
    /// repository.
    #[serde(rename = "exclude-directories")]
    pub exclude_directories: Vec<String>,
}

/// The sole stable configuration document.
pub enum Document {
    V0_4(Box<v0_4::config::Document>),
}

/// Parse and validate configuration text.
///
/// Split from any filesystem read so the whole contract is exercisable from a
/// string, which is what lets the behaviour corpus assert on positions and
/// messages rather than only on exit codes.
pub fn parse(text: &str) -> Result<Document, ConfigError> {
    match declared_schema(text)? {
        Schema::V0_4 => {
            v0_4::config::parse(text).map(|document| Document::V0_4(Box::new(document)))
        }
    }
}

/// Generate the grouped-v2 JSON Schema from the same model that parses it.
///
/// Kept beside configuration selection so the build tool does not need a
/// second model, parser, or schema-specific dependency path.
pub fn v0_4_schema_bytes() -> Result<Vec<u8>, serde_json::Error> {
    v0_4::config::schema_bytes()
}

/// Decide whether text declares the one stable schema, while preserving a
/// diagnostic for a predecessor comment rather than attempting to parse it.
fn declared_schema(text: &str) -> Result<Schema, ConfigError> {
    if let Some((declared, line)) = commented_schema(text) {
        return Err(ConfigError::SchemaUnrecognized { declared, line });
    }

    match keyed_schema(text)? {
        Some(declared) if declared == v0_4::config::SCHEMA => Ok(Schema::V0_4),
        Some(declared) => Err(ConfigError::SchemaUnrecognized { declared, line: 1 }),
        None => Err(ConfigError::SchemaUndeclared),
    }
}

/// Read only the top-level schema declaration needed to select the closed
/// stable parser. Any policy shape remains owned by `v0_4::config::parse`.
fn keyed_schema(text: &str) -> Result<Option<String>, ConfigError> {
    let raw: serde_json::Value =
        yaml_serde::from_str(text).map_err(|error| ConfigError::Malformed(error.to_string()))?;
    Ok(raw
        .as_object()
        .and_then(|mapping| mapping.get("schema"))
        .and_then(serde_json::Value::as_str)
        .map(str::to_string))
}

/// A predecessor schema was declared in a leading comment. It is recognized
/// only so stable can refuse it explicitly; it is never decoded as input.
fn commented_schema(text: &str) -> Option<(String, usize)> {
    for (index, raw) in text.lines().enumerate() {
        let line = raw.trim();
        if line.is_empty() {
            continue;
        }
        let comment = line.strip_prefix('#')?;
        let Some((_, declared)) = comment.split_once("schema:") else {
            continue;
        };
        return Some((declared.trim().to_string(), index + 1));
    }
    None
}

/// The schema selector's closed result.
enum Schema {
    V0_4,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn grouped_schema_bytes_and_stable_reader_are_available_without_a_tree() {
        let schema = v0_4_schema_bytes().expect("the closed grouped schema serializes");
        assert!(schema.ends_with(b"\n"));
        assert!(
            schema
                .windows(b"rhino/repo-config/v2".len())
                .any(|window| window == b"rhino/repo-config/v2")
        );
        assert!(parse("schema: rhino/repo-config/v2\n").is_ok());
        assert!(matches!(
            parse("schema: ose/repo-config/v2\n"),
            Err(ConfigError::SchemaUnrecognized { .. })
        ));
    }

    #[test]
    fn predecessor_comments_are_rejected_without_a_legacy_decoder() {
        assert_eq!(commented_schema("\n# note\nkey: value\n"), None);
        assert_eq!(
            commented_schema("\n# schema: rhino/repo-config/v1\nkey: value\n"),
            Some(("rhino/repo-config/v1".to_string(), 2))
        );
        assert!(matches!(
            parse("# schema: rhino/repo-config/v1\n"),
            Err(ConfigError::SchemaUnrecognized { .. })
        ));
    }

    #[test]
    fn stable_errors_render_missing_undeclared_and_terminal_comment_cases() {
        assert!(
            ConfigError::Missing
                .to_string()
                .contains("no configuration file")
        );
        assert!(
            ConfigError::Unreadable("denied".to_string())
                .to_string()
                .contains("denied")
        );
        assert!(
            ConfigError::SchemaUndeclared
                .to_string()
                .contains("declares no schema")
        );
        assert!(matches!(
            parse("repository: {}\n"),
            Err(ConfigError::SchemaUndeclared)
        ));
        assert!(matches!(parse("["), Err(ConfigError::Malformed(_))));
        assert_eq!(commented_schema("# note\n# another"), None);
    }
}
