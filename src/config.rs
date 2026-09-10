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

pub mod v2;

use serde::Deserialize;
use std::collections::BTreeMap;
use std::fmt;

/// The schema this build understands, and the predecessor spelling it accepts.
///
/// The alias exists so a consumer already carrying `rhino-cli/` in its first
/// line does not have to rewrite it on the same day RHINO ships.
pub const SCHEMA: &str = "rhino/repo-config/v1";
pub const SCHEMA_ALIAS: &str = "rhino-cli/repo-config/v1";

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
                "{PATH}: line 1 declares no schema: expected a leading `# schema: {SCHEMA}` comment"
            ),
            Self::SchemaUnrecognized { declared, line } => write!(
                formatter,
                "{PATH}: line {line}: unrecognized schema `{declared}`: this build understands `{SCHEMA}` (and `{SCHEMA_ALIAS}`)"
            ),
            Self::Malformed(message) => write!(formatter, "{PATH}: {message}"),
            Self::Semantic { key, line, reason } => {
                write!(formatter, "{PATH}: line {line}: {key}: {reason}")
            }
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
pub struct Config {
    #[serde(rename = "governance-word-budget")]
    pub word_budget: WordBudget,
    #[serde(rename = "governance-directory-map")]
    pub directory_map: DirectoryMap,
    #[serde(rename = "md-internal-link", default)]
    pub internal_link: InternalLink,
    #[serde(rename = "md-mermaid")]
    pub mermaid: Mermaid,
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
    #[serde(rename = "harness-parity")]
    pub harness_parity: HarnessParity,
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
    /// The optional model and effort a harness uses for each portable tier.
    ///
    /// Read here rather than from an agent, because a mapping written beside
    /// the agent would make one repository's vendor choice part of a portable
    /// artifact. Every harness and every tier may be omitted; what may not be
    /// omitted is half of a pair.
    #[serde(rename = "model-tiers", default)]
    pub model_tiers: Option<BTreeMap<String, BTreeMap<String, Option<TierMapping>>>>,
    pub scan: Scan,
}

/// The harness profiles this schema recognizes.
///
/// Closed rather than open: an unrecognized key is far more often a typo than
/// a harness nobody has heard of, and a typo that silently maps nothing is a
/// vendor pin that quietly stops applying.
pub const HARNESS_PROFILES: [&str; 3] = ["claude", "codex", "opencode"];

/// The portable tiers a mapping may be keyed by.
///
/// Named for the workload rather than for a model, so the same four survive a
/// vendor renaming its lineup.
pub const TIERS: [&str; 4] = ["ultra", "plan", "execution", "fast"];

#[derive(Debug, Clone, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Metadata {
    /// Ordered, on the same rule as `governance-word-budget.surfaces`: where
    /// two globs match one file, the last matching entry wins. That is how a
    /// workflow subtree carries a different schema from the governance tree it
    /// sits inside.
    pub surfaces: Vec<MetadataSurface>,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct MetadataSurface {
    pub glob: String,
    pub schema: MetadataSchema,
}

/// The four canonical artifact families.
#[derive(Debug, Clone, Copy, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum MetadataSchema {
    Governance,
    Workflow,
    Skill,
    Agent,
}

/// One harness's choice for one tier.
///
/// Both fields are optional *to the decoder* and required *to the checker*, so
/// that half a pair is reported as the policy fault it is rather than as a
/// decoding failure a reader has to translate.
#[derive(Debug, Clone, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct TierMapping {
    #[serde(default)]
    pub model: Option<String>,
    #[serde(default)]
    pub effort: Option<String>,
}

#[derive(Debug, Clone, Deserialize)]
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
#[derive(Debug, Clone, Copy, Deserialize, PartialEq, Eq)]
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

#[derive(Debug, Clone, Deserialize)]
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

#[derive(Debug, Clone, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct DirectoryMap {
    pub trees: Vec<Tree>,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Tree {
    pub path: String,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct InternalLink {
    /// Globs excluded as link *sources*. Files under them stay valid targets.
    #[serde(rename = "exclude-sources", default)]
    pub exclude_sources: Vec<String>,
}

#[derive(Debug, Clone, Deserialize)]
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
#[derive(Debug, Clone, Copy, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum AuthoringRule {
    /// Mermaid, carrying both an accessible title and an accessible
    /// description.
    Rendered,
    /// ASCII with prose beside it, and no Mermaid at all.
    PlainText,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Frontmatter {
    /// Ordered, last match wins, as everywhere else surfaces are declared.
    pub surfaces: Vec<FrontmatterSurface>,
}

#[derive(Debug, Clone, Deserialize)]
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

#[derive(Debug, Clone, Deserialize)]
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
#[derive(Debug, Clone, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Globbed {
    pub glob: String,
}

/// Files in which an emoji code point is a finding.
#[derive(Debug, Clone, Deserialize)]
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

#[derive(Debug, Clone, Deserialize)]
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

#[derive(Debug, Clone, Deserialize)]
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
#[derive(Debug, Clone, Copy, Deserialize, PartialEq, Eq)]
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
pub struct HarnessParity {
    pub canonical: Canonical,
    /// Empty is legal and explicit: the difference between "no harnesses" and
    /// "I forgot to configure harnesses" has to stay visible, so the key itself
    /// is required.
    pub harnesses: Vec<Harness>,
    #[serde(rename = "prohibited-instruction-sources")]
    pub prohibited_instruction_sources: Vec<String>,
    /// Fields of a harness's own configuration file that may not carry
    /// instructions.
    ///
    /// Optional, because most harnesses keep their always-on instructions in a
    /// Markdown file and a repository with none of these has nothing to
    /// declare. Where a harness does read instructions out of its settings,
    /// that key is a competing always-on source wearing the vendor's syntax,
    /// and prohibiting the *file* would be wrong -- the file is legitimate and
    /// holds everything else that harness needs.
    #[serde(rename = "prohibited-instruction-fields", default)]
    pub prohibited_instruction_fields: Vec<ProhibitedField>,
    pub capabilities: Vec<String>,
    pub constraints: Vec<String>,
    /// Optional even alongside a roster: not every repository requires its
    /// harnesses to reach a capability server, and a schema that insisted
    /// would be asserting one repository's arrangement as everyone's.
    ///
    /// Still refused alongside an *empty* roster, on the same rule as the
    /// canonical roots: a server nothing reconciles silently does nothing.
    #[serde(rename = "required-mcp", default)]
    pub required_mcp: Option<RequiredMcp>,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Canonical {
    pub instruction: String,
    /// Optional: a repository may have no file that merely imports the
    /// instruction. Absent weakens nothing -- it means there is nothing to
    /// route -- and the prohibition on competing sources still applies in full.
    #[serde(rename = "instruction-adapter", default)]
    pub instruction_adapter: Option<String>,
    /// Required whenever the roster is non-empty; omissible only alongside an
    /// empty one.
    #[serde(rename = "skills-root", default)]
    pub skills_root: Option<String>,
    #[serde(rename = "agents-root", default)]
    pub agents_root: Option<String>,
    /// The sentence an adapter carries in place of the canonical prompt.
    ///
    /// An adapter is a route, not a copy. Every harness a repository declares
    /// points at the same canonical file, so the sentence is written once here
    /// with `{path}` standing for the canonical document's repository-relative
    /// path. The wording is the repository's -- RHINO ships none -- and it is
    /// required alongside the root whose adapters use it, because a root with
    /// no route would leave every adapter's body unchecked.
    #[serde(rename = "agent-route", default)]
    pub agent_route: Option<String>,
    #[serde(rename = "skill-route", default)]
    pub skill_route: Option<String>,
    /// How this repository writes a canonical agent's permissions.
    ///
    /// The three lists have names, and the names are the repository's own --
    /// one writes `requires:` where another writes `capabilities:`. Reading
    /// them by a name RHINO chose would not merely miss a field: an unnamed
    /// list is an empty one, so every translation would find nothing to fire on
    /// and the run would report clean without having compared anything.
    /// Required alongside `agents-root`, because only agents carry
    /// permissions.
    #[serde(default)]
    pub declaration: Option<DeclarationShape>,
}

/// Which field of a canonical agent carries which half of its permissions, and
/// what every such declaration must say outright.
#[derive(Debug, Clone, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct DeclarationShape {
    /// The field naming what the agent may do.
    pub grants: String,
    /// The field naming what it may not.
    pub denials: String,
    /// The field naming how it must behave while doing it.
    pub limits: String,
    /// Fields every canonical agent must carry with exactly this value.
    #[serde(default)]
    pub fixed: BTreeMap<String, String>,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Harness {
    pub name: String,
    /// How this harness expresses the canon: where its adapters live, what
    /// document format they use, and how the canonical capability vocabulary
    /// translates into the permissions this harness actually understands.
    ///
    /// All of it is data. A harness that names its tools one way and a harness
    /// that names them another are two configurations, not two code paths, and
    /// a fourth harness is one more entry here.
    #[serde(rename = "agent-adapter", default)]
    pub agent_adapter: Option<Adapter>,
    /// Per-harness rather than global, so a second harness gaining skill
    /// wrappers is one line of configuration instead of a code change.
    #[serde(rename = "skill-adapter", default)]
    pub skill_adapter: Option<Adapter>,
    /// Where this harness declares what it may reach, and in which format.
    ///
    /// The two travel together because neither means anything alone: a path
    /// with no format cannot be parsed and a format with no path names
    /// nothing. Nesting them makes half a declaration unrepresentable rather
    /// than merely detectable.
    #[serde(default)]
    pub capability: Option<Capability>,
}

/// Where one harness's adapter keeps a projected tier.
///
/// Both together or neither, which is the same pairing the mapping itself
/// obeys: a repository that named only the model field could describe an
/// adapter carrying half a pair and would have no way to say the other half
/// was missing.
#[derive(Debug, Clone, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct TierFields {
    pub model: String,
    pub effort: String,
}

/// One harness's expression of one kind of canonical document.
#[derive(Debug, Clone, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Adapter {
    /// Where the adapter for a canonical document named `n` lives, with
    /// `{name}` standing for `n`.
    ///
    /// A pattern rather than a directory and an extension, because the two
    /// real shapes are a file per document and a directory per document, and
    /// only a pattern can say both.
    pub path: String,
    pub format: DocumentFormat,
    /// Where the route sentence lives: `body` for the prose beneath a
    /// declaration, or the name of a field holding it.
    #[serde(rename = "route-field")]
    pub route_field: String,
    /// Adapter field to the canonical property it must equal, either `name` or
    /// `description`. A harness that carries neither declares an empty map.
    #[serde(default)]
    pub identity: BTreeMap<String, Identity>,
    /// Fields that must be present and hold exactly this scalar.
    #[serde(default)]
    pub fixed: BTreeMap<String, String>,
    /// Which two fields of this harness's adapter carry the model and the
    /// effort a portable tier maps to.
    ///
    /// Declared beside the adapter rather than globally, because the field
    /// names are this harness's vocabulary and a second harness names them its
    /// own way. Omitting them is a valid answer: a harness that expresses no
    /// model has no projection to get wrong.
    #[serde(rename = "tier-fields", default)]
    pub tier_fields: Option<TierFields>,
    /// Fields that must not appear at all.
    #[serde(default)]
    pub absent: Vec<String>,
    /// Front matter that may declare nothing beyond what the rules above name.
    ///
    /// A wrapper exists to route. One that also carries a model, a tool list,
    /// or a second description is a place for the canon and the harness to
    /// drift apart, so a repository can say the declaration is closed.
    #[serde(default)]
    pub closed: bool,
    /// How the canonical capability vocabulary reaches this harness's own
    /// permission vocabulary.
    #[serde(default)]
    pub translations: Vec<Translation>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Identity {
    Name,
    Description,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub enum DocumentFormat {
    /// YAML front matter between `---` fences, with prose beneath.
    FrontMatter,
    Toml,
}

/// One obligation an adapter takes on, and when it takes it on.
#[derive(Debug, Clone, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Translation {
    pub when: When,
    /// The canonical capability or constraint that triggers this obligation.
    /// Absent only for `always`.
    #[serde(default)]
    pub capability: Option<String>,
    /// The adapter field the obligation is about.
    pub field: String,
    /// Members the field must contain. A sequence contributes its items; a
    /// scalar contributes its comma-separated parts, which is how a harness
    /// that writes one string and a harness that writes a list are read the
    /// same way.
    #[serde(default)]
    pub members: Vec<String>,
    /// Members the field must not contain.
    #[serde(rename = "absent-members", default)]
    pub absent_members: Vec<String>,
    /// At least one member beginning with this.
    #[serde(rename = "member-prefix", default)]
    pub member_prefix: Option<String>,
    /// Keys the field must map to exactly these values.
    #[serde(default)]
    pub entries: BTreeMap<String, String>,
    /// At least one key beginning with this, mapped to `entry-value`.
    #[serde(rename = "entry-prefix", default)]
    pub entry_prefix: Option<String>,
    #[serde(rename = "entry-value", default)]
    pub entry_value: Option<String>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum When {
    /// Whatever the canonical document says.
    Always,
    /// Only when the canonical document requires the named capability.
    Requires,
    /// Only when it denies the named capability.
    Denies,
    /// Only when it carries the named constraint.
    Constrains,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Capability {
    pub file: String,
    pub format: CapabilityFormat,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum CapabilityFormat {
    Toml,
    Json,
}

/// One configuration field that may not carry instructions.
#[derive(Debug, Clone, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ProhibitedField {
    pub file: String,
    pub format: CapabilityFormat,
    /// The key, read at the document's top level. Nested keys are not
    /// expressible on purpose: a harness that buried its instructions would be
    /// a different rule, and guessing at one would report a repository clean
    /// for a reason nobody wrote down.
    pub field: String,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RequiredMcp {
    pub name: String,
    pub command: String,
    pub args: Vec<String>,
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

/// What a repository declared, whichever schema it wrote it in.
///
/// An enum rather than one widened struct, because the two schemas answer
/// different questions. A v1 document says how to check a tree; a v2 document
/// says how a repository is governed and what its gates are. Folding them
/// together would give every validator a section that is legal to be absent for
/// a reason it has no way to tell from an omission.
pub enum Document {
    V1(Box<Config>),
    V2(Box<v2::Document>),
}

/// Parse and validate configuration text.
///
/// Split from any filesystem read so the whole contract is exercisable from a
/// string, which is what lets the behaviour corpus assert on positions and
/// messages rather than only on exit codes.
pub fn parse(text: &str) -> Result<Document, ConfigError> {
    match declared_schema(text)? {
        Schema::V1 => parse_v1(text).map(|config| Document::V1(Box::new(config))),
        Schema::V2 => v2::parse(text).map(|document| Document::V2(Box::new(document))),
    }
}

fn parse_v1(text: &str) -> Result<Config, ConfigError> {
    let config: Config = yaml_serde::from_str(text).map_err(|error| {
        // The parser reports `section.key: reason at line L column C`, which is
        // already the shape a maintainer needs. Passing it through keeps one
        // wording for every structural fault instead of paraphrasing per case.
        let mut message = error.to_string();
        if !message.contains("line ")
            && let Some(location) = error.location()
        {
            message = format!("{message} at line {}", location.line());
        }
        ConfigError::Malformed(message)
    })?;

    check_semantics(&config, text)?;
    Ok(config)
}

/// Which schema a document is written in.
enum Schema {
    V1,
    V2,
}

/// Decide which schema a document declares, and refuse anything else.
///
/// v1 declares its schema in a leading comment and v2 in a leading key, so the
/// two are read from different places and no document can be mistaken for the
/// other. One carrying both is refused rather than resolved by precedence: a
/// rule that picked a winner would silently enforce half of one contract.
fn declared_schema(text: &str) -> Result<Schema, ConfigError> {
    let commented = commented_schema(text);
    let keyed = v2::declares(text);
    match (commented, keyed) {
        (Some(_), Some(_)) => Err(ConfigError::Semantic {
            key: "schema".to_string(),
            line: 1,
            reason: "declares two schemas, and a document is written in one".to_string(),
        }),
        (Some((declared, line)), None) => {
            if declared == SCHEMA || declared == SCHEMA_ALIAS {
                Ok(Schema::V1)
            } else {
                Err(ConfigError::SchemaUnrecognized { declared, line })
            }
        }
        (None, Some(declared)) => {
            if declared == v2::SCHEMA {
                Ok(Schema::V2)
            } else {
                Err(ConfigError::SchemaUnrecognized { declared, line: 1 })
            }
        }
        (None, None) => Err(ConfigError::SchemaUndeclared),
    }
}

/// The schema a v1 document declares, in the leading comment it belongs in.
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

/// The rules a well-typed document can still break.
fn check_semantics(config: &Config, text: &str) -> Result<(), ConfigError> {
    check_model_tiers(config, text)?;

    let canonical = &config.harness_parity.canonical;

    // Everything a harness is reconciled *against*: the two canonical roots and
    // the capability declaration every harness has to match. Each one is
    // meaningful only in the presence of a harness, so each follows the roster.
    let reconciled_against = [
        ("skills-root", canonical.skills_root.is_some()),
        ("agents-root", canonical.agents_root.is_some()),
        ("required-mcp", config.harness_parity.required_mcp.is_some()),
    ];

    // Canon with nowhere to reconcile it is a configuration error rather than a
    // clean pass: it means the reconciliation was silently skipped.
    //
    // The converse does not hold. A repository may declare harnesses and no
    // canonical skills, no canonical agents, or no capability server -- each is
    // a thing a repository may not have, and requiring any of them alongside a
    // roster would be asserting one repository's arrangement as everyone's.
    // What each *does* require is the rest of its own pair, below.
    if config.harness_parity.harnesses.is_empty() {
        for (key, declared) in reconciled_against {
            if declared {
                return Err(ConfigError::Semantic {
                    key: key.to_string(),
                    line: line_of(text, key),
                    reason: "declared alongside an empty harness roster, so there is no harness to reconcile it against".to_string(),
                });
            }
        }
    }

    // An adapter contract and the canon it expresses stand or fall together,
    // on the same rule as the server below: a contract with no canon behind it
    // reconciles nothing, and a canon no harness expresses is a canon nothing
    // reaches.
    let agents = canonical.agents_root.is_some();
    let skills = canonical.skills_root.is_some();
    for harness in &config.harness_parity.harnesses {
        if harness.agent_adapter.is_some() != agents {
            let reason = if agents {
                format!(
                    "`{}` declares no agent adapter, so the canonical agents reach it nowhere",
                    harness.name
                )
            } else {
                format!(
                    "`{}` declares an agent adapter, but no `agents-root` names what it would express",
                    harness.name
                )
            };
            return Err(ConfigError::Semantic {
                key: "agent-adapter".to_string(),
                line: line_of(text, "harnesses"),
                reason,
            });
        }
        if harness.skill_adapter.is_some() && !skills {
            return Err(ConfigError::Semantic {
                key: "skill-adapter".to_string(),
                line: line_of(text, "harnesses"),
                reason: format!(
                    "`{}` declares a skill adapter, but no `skills-root` names what it would express",
                    harness.name
                ),
            });
        }
    }

    // A required server and the per-harness declarations that must satisfy it
    // stand or fall together. Declared alone, either one is a rule nothing
    // enforces: a server no harness is checked against, or a file no rule
    // reads.
    let required = config.harness_parity.required_mcp.is_some();
    for harness in &config.harness_parity.harnesses {
        if harness.capability.is_some() == required {
            continue;
        }
        let reason = if required {
            format!(
                "`{}` declares no capability file, so the required server is reconciled against nothing",
                harness.name
            )
        } else {
            format!(
                "`{}` declares a capability file, but no required server names anything to find in it",
                harness.name
            )
        };
        return Err(ConfigError::Semantic {
            key: "capability".to_string(),
            line: line_of(text, "harnesses"),
            reason,
        });
    }

    // A root says where the canon is; a route says what an adapter must carry
    // in its place. Neither means anything without the other, so each root
    // brings its route and a route with no root has nothing to substitute for.
    for (root, route, root_key, route_key) in [
        (
            canonical.agents_root.is_some(),
            canonical.agent_route.is_some(),
            "agents-root",
            "agent-route",
        ),
        (
            canonical.skills_root.is_some(),
            canonical.skill_route.is_some(),
            "skills-root",
            "skill-route",
        ),
    ] {
        if root == route {
            continue;
        }
        let (key, reason) = if root {
            (
                route_key,
                format!("required alongside `{root_key}`, or every adapter's body goes unchecked"),
            )
        } else {
            (
                route_key,
                format!("declared without `{root_key}`, so there is nothing for it to route to"),
            )
        };
        return Err(ConfigError::Semantic {
            key: key.to_string(),
            line: line_of(text, key),
            reason,
        });
    }

    // The shape says which field holds which permission. Without it the lists
    // would be read by a name RHINO chose, and a name nobody wrote reads as an
    // empty list rather than as an error -- so the pairing is what keeps a
    // silently unchecked canon from passing as a clean one.
    if canonical.agents_root.is_some() != canonical.declaration.is_some() {
        let (key, reason) = if canonical.agents_root.is_some() {
            (
                "declaration",
                "required alongside `agents-root`, or every canonical agent reads as granting and denying nothing"
                    .to_string(),
            )
        } else {
            (
                "declaration",
                "declared without `agents-root`, so there is no canonical agent to read"
                    .to_string(),
            )
        };
        return Err(ConfigError::Semantic {
            key: key.to_string(),
            line: line_of(text, key),
            reason,
        });
    }

    // A translation fires on a canonical name. One that names something outside
    // the declared vocabulary can never fire, which makes it a permission rule
    // that silently grants everything.
    let vocabulary: Vec<&str> = config
        .harness_parity
        .capabilities
        .iter()
        .chain(&config.harness_parity.constraints)
        .map(String::as_str)
        .collect();
    for harness in &config.harness_parity.harnesses {
        let adapters = harness.agent_adapter.iter().chain(&harness.skill_adapter);
        for adapter in adapters {
            for translation in &adapter.translations {
                let named = match (translation.when, &translation.capability) {
                    (When::Always, _) => continue,
                    (_, Some(name)) => name.as_str(),
                    (_, None) => {
                        return Err(ConfigError::Semantic {
                            key: "translations".to_string(),
                            line: line_of(text, "translations"),
                            reason: format!(
                                "`{}` declares a conditional translation naming no capability",
                                harness.name
                            ),
                        });
                    }
                };
                if !vocabulary.contains(&named) {
                    return Err(ConfigError::Semantic {
                        key: "translations".to_string(),
                        line: line_of(text, "translations"),
                        reason: format!(
                            "`{named}` is outside the declared vocabulary, so nothing would ever trigger this translation"
                        ),
                    });
                }
            }
        }
    }

    // A separator says where an encoded prefix ends. A `path-prefixed` surface
    // without one has no way to read a filename, and a `kebab-case` surface
    // with one has no prefix for it to follow -- the second being the quieter
    // fault, since a separator nothing consults reads as a rule that is in
    // force.
    if let Some(naming) = &config.naming {
        for surface in &naming.surfaces {
            let declared = surface
                .separator
                .as_deref()
                .is_some_and(|separator| !separator.is_empty());
            let reason = match (surface.style, declared) {
                (NameStyle::PathPrefixed, false) => Some(format!(
                    "required by the path-prefixed surface `{}`, or nothing says where its encoded prefix ends",
                    surface.glob
                )),
                (NameStyle::KebabCase, true) => Some(format!(
                    "declared on the kebab-case surface `{}`, which has no encoded prefix for a separator to follow",
                    surface.glob
                )),
                _ => None,
            };
            if let Some(reason) = reason {
                return Err(ConfigError::Semantic {
                    key: "separator".to_string(),
                    line: line_of(text, "surfaces"),
                    reason,
                });
            }
        }
    }

    for (key, value) in declared_paths(config) {
        if let Some(reason) = escapes_root(&value) {
            return Err(ConfigError::Semantic {
                key: key.to_string(),
                line: line_of(text, key),
                reason: format!("`{value}` {reason}"),
            });
        }
    }

    Ok(())
}

/// Every scalar the configuration declares as a repository-relative path.
fn declared_paths(config: &Config) -> Vec<(&'static str, String)> {
    let canonical = &config.harness_parity.canonical;
    let mut paths: Vec<(&'static str, String)> =
        vec![("instruction", canonical.instruction.clone())];

    for (key, declared) in [
        ("instruction-adapter", &canonical.instruction_adapter),
        ("skills-root", &canonical.skills_root),
        ("agents-root", &canonical.agents_root),
    ] {
        if let Some(value) = declared {
            paths.push((key, value.clone()));
        }
    }
    for tree in &config.directory_map.trees {
        paths.push(("path", tree.path.clone()));
    }
    for harness in &config.harness_parity.harnesses {
        if let Some(adapter) = &harness.agent_adapter {
            paths.push(("agent-adapter", adapter.path.clone()));
        }
        if let Some(capability) = &harness.capability {
            paths.push(("capability-file", capability.file.clone()));
        }
        if let Some(adapter) = &harness.skill_adapter {
            paths.push(("skill-adapter", adapter.path.clone()));
        }
    }
    paths
}

/// Whether a declared path leaves the repository, and how.
///
/// Resolved textually rather than against the filesystem, so the answer does
/// not depend on which components happen to exist yet.
fn escapes_root(value: &str) -> Option<&'static str> {
    if value.starts_with('/') {
        return Some("is absolute, and every declared path is relative to the repository root");
    }
    let mut depth: isize = 0;
    for segment in value.split('/') {
        match segment {
            ".." => depth -= 1,
            "." | "" => {}
            _ => depth += 1,
        }
        if depth < 0 {
            return Some("escapes the repository root");
        }
    }
    None
}

/// The first line declaring `key`, one-based, for a fault the parser did not
/// raise and so did not position.
/// The tier mapping, which is refused before anything could be generated from it.
///
/// Every fault here is a fault a generator would otherwise have to guess its
/// way around, and a guess is exactly what the mapping exists to prevent: an
/// unmapped tier means "inherit the harness default", so a half-mapped one has
/// no honest reading at all.
fn check_model_tiers(config: &Config, text: &str) -> Result<(), ConfigError> {
    let Some(harnesses) = &config.model_tiers else {
        return Ok(());
    };
    check_tier_map(harnesses, line_of(text, "model-tiers"))
}

/// The tier map's whole contract, held wherever the section is declared.
///
/// Shared by both schemas rather than reimplemented beside the second one: a
/// tier map that meant something different depending on which schema carried it
/// would give a generator two answers to the same question.
pub(crate) fn check_tier_map(
    harnesses: &BTreeMap<String, BTreeMap<String, Option<TierMapping>>>,
    line: usize,
) -> Result<(), ConfigError> {
    const KEY: &str = "model-tiers";
    let refuse = |reason: String| {
        Err(ConfigError::Semantic {
            key: KEY.to_string(),
            line,
            reason,
        })
    };

    // An empty section and an omitted one would mean the same thing to a
    // generator, which is why the empty one is refused: only the omission is
    // an answer a repository can be held to.
    if harnesses.is_empty() {
        return refuse(
            "declares no harness, and an empty section cannot be told from one a repository meant to omit"
                .to_string(),
        );
    }

    for (harness, tiers) in harnesses {
        if !HARNESS_PROFILES.contains(&harness.as_str()) {
            return refuse(format!(
                "`{harness}` is not a harness profile this schema recognizes; the initial profiles are {}",
                listed(&HARNESS_PROFILES)
            ));
        }
        if tiers.is_empty() {
            return refuse(format!(
                "`{harness}` declares no tier, and an empty map cannot be told from a harness a repository meant to omit"
            ));
        }
        for (tier, mapping) in tiers {
            if !TIERS.contains(&tier.as_str()) {
                return refuse(format!(
                    "`{harness}` maps `{tier}`, which is not a portable tier; the closed set is {}. A mapping keyed by an agent name is the same fault: model and effort resolve by tier, never by artifact",
                    listed(&TIERS)
                ));
            }
            let Some(mapping) = mapping else {
                return refuse(format!(
                    "`{harness}.{tier}` declares no model or effort; an unmapped tier is written by omitting it, not by mapping it to nothing"
                ));
            };
            for (field, value) in [("model", &mapping.model), ("effort", &mapping.effort)] {
                match value {
                    None => {
                        return refuse(format!(
                            "`{harness}.{tier}` declares no {field}; a present tier carries both, or the generator would emit half a pin"
                        ));
                    }
                    Some(value) if value.trim().is_empty() => {
                        return refuse(format!(
                            "`{harness}.{tier}` declares an empty {field}, which is not a value a harness can be given"
                        ));
                    }
                    Some(_) => {}
                }
            }
        }
    }

    Ok(())
}

/// A closed set, written the way a sentence would read it.
pub(crate) fn listed(values: &[&str]) -> String {
    let quoted: Vec<String> = values.iter().map(|value| format!("`{value}`")).collect();
    match quoted.split_last() {
        None => String::new(),
        Some((last, [])) => last.clone(),
        Some((last, rest)) => format!("{}, and {last}", rest.join(", ")),
    }
}

fn line_of(text: &str, key: &str) -> usize {
    text.lines()
        .position(|line| line.trim_start().starts_with(&format!("{key}:")))
        .map_or(1, |index| index + 1)
}
