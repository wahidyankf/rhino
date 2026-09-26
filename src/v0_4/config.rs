//! The closed grouped v0.4 configuration model.
//!
//! This is deliberately one model: Serde reads repository input and Schemars
//! emits the editor-facing Draft 2020-12 artifact. A second hand-written schema
//! would inevitably accept a shape the runtime rejects, or the reverse.

use crate::config::ConfigError;
use schemars::{JsonSchema, SchemaGenerator, generate::SchemaSettings};
use serde::{Deserialize, Serialize};
use serde_json::{Map, Value};
use std::collections::BTreeMap;

pub const SCHEMA: &str = "rhino/repo-config/v2";

const CORE_KEYS: [&str; 9] = [
    "schema",
    "repository",
    "scan",
    "harness",
    "policies",
    "environment",
    "toolchains",
    "gates",
    "extensions",
];

/// The only grouped configuration this version accepts.
///
/// Groups are optional: an omitted group states no policy, while an empty one
/// is an explicit but currently empty owner boundary. Both forms are preserved
/// without guessing policy that a repository did not write.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct Document {
    pub schema: SchemaId,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub repository: Option<Repository>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub scan: Option<Scan>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub harness: Option<Harness>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub policies: Option<Policies>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub environment: Option<Environment>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub toolchains: Option<Toolchains>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub gates: Option<Gates>,
    /// Owner namespaces are validated, but their mappings are opaque to core.
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub extensions: BTreeMap<String, Map<String, Value>>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub enum SchemaId {
    #[serde(rename = "rhino/repo-config/v2")]
    V2,
}

macro_rules! closed_group {
    ($name:ident) => {
        #[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize, JsonSchema)]
        #[serde(deny_unknown_fields)]
        pub struct $name {}
    };
}

closed_group!(Repository);
closed_group!(PlansPolicy);

/// Directory names excluded from every tree walk.
///
/// The list is declared rather than inferred from ignore files: repositories
/// choose whether generated or local paths are out of a policy's scope, and
/// every grouped validator that uses the shared scanner receives the same
/// answer.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields, rename_all = "kebab-case")]
pub struct Scan {
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) exclude_directories: Vec<String>,
}

/// Markdown policy is explicitly opt-in. Each named validator refuses an
/// omitted sub-policy rather than turning a repository's prose into an
/// undeclared product default.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields, rename_all = "kebab-case")]
pub struct MarkdownPolicy {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) frontmatter: Option<crate::config::Frontmatter>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) heading_hierarchy: Option<crate::config::HeadingHierarchy>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) internal_link: Option<crate::config::InternalLink>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) metadata: Option<crate::config::Metadata>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) mermaid: Option<crate::config::Mermaid>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) readme_index: Option<ReadmeIndexPolicy>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) naming: Option<crate::config::Naming>,
}

/// A declared README tree and the limited index facts its repository owns.
///
/// Direct-child completeness is intentionally a boolean because the complete
/// child set is an observable tree fact, not a second manually maintained
/// list. Exclusions and annotations remain data: RHINO does not infer either.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields, rename_all = "kebab-case")]
pub(crate) struct ReadmeIndexPolicy {
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) trees: Vec<ReadmeIndexTree>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields, rename_all = "kebab-case")]
pub(crate) struct ReadmeIndexTree {
    pub(crate) path: String,
    #[serde(default)]
    pub(crate) require_direct_children: bool,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) annotations: Vec<ReadmeAnnotation>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) exclusions: Vec<String>,
}

/// Exact text a named README must carry. It is a declared index annotation,
/// not a judgement about the surrounding prose.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct ReadmeAnnotation {
    pub(crate) path: String,
    pub(crate) text: String,
}

/// Repository-neutral governance policy. The names below carry no hierarchy or
/// organisation vocabulary; every root, layer, category, term, artifact, and
/// relationship comes from the consuming repository.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields, rename_all = "kebab-case")]
pub struct GovernancePolicy {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) word_budget: Option<crate::config::WordBudget>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) directory_map: Option<crate::config::DirectoryMap>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) vendor: Option<VendorPolicy>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) layers: Option<LayerPolicy>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) traceability: Option<TraceabilityPolicy>,
}

/// Exact vocabulary that may not appear in declared portable source roots.
///
/// An exception pairs one term with exact paths, preventing a broad exception
/// from silently disabling another term or another part of a tree.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields, rename_all = "kebab-case")]
pub(crate) struct VendorPolicy {
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) roots: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) excluded_binding_roots: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) forbidden_terms: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) vocabulary_exceptions: Vec<VocabularyException>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct VocabularyException {
    pub(crate) term: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) paths: Vec<String>,
}

/// The complete directory vocabulary and its required layer sequence.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct LayerPolicy {
    pub(crate) root: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) order: Vec<String>,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub(crate) categories: BTreeMap<String, Vec<String>>,
}

/// Artifact presence and exact declared links, without a prose-quality rule.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct TraceabilityPolicy {
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) artifacts: Vec<TraceabilityArtifact>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) relationships: Vec<TraceabilityRelationship>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct TraceabilityArtifact {
    pub(crate) id: String,
    pub(crate) path: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct TraceabilityRelationship {
    pub(crate) from: String,
    pub(crate) to: String,
}

/// Configured license placement and content checks. A path can require exact
/// identifiers, a SHA-256 digest, or both; omitted checks are never supplied
/// by RHINO. Exclusions are exact repository-relative paths.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields, rename_all = "kebab-case")]
pub struct ConventionsPolicy {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) license: Option<LicensePolicy>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) emoji: Option<crate::config::Emoji>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct LicensePolicy {
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) paths: Vec<LicensePath>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) exclusions: Vec<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct LicensePath {
    pub(crate) path: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) identifiers: Vec<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) sha256: Option<String>,
}

/// Repository-declared source/target pairs for local environment setup.
///
/// Values live only in the source file; configuration names paths, never
/// secret values. A missing group refuses environment operations, while an
/// explicitly empty group permits only the zero-file validation/backup plan.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct Environment {
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) examples: Vec<EnvironmentExample>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) staged: Option<StagedEnvironmentPolicy>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) contracts: Vec<EnvironmentContract>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) detectors: Vec<EnvironmentDetector>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) allowlists: Vec<EnvironmentAllowlist>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct EnvironmentExample {
    pub(crate) source: String,
    pub(crate) target: String,
}

/// Exact repository patterns for the Git-index environment guard. The policy
/// is optional because an absent section must not make Rhino invent secret-file
/// names; an explicit policy is checked only against staged paths.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct StagedEnvironmentPolicy {
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) forbidden: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) allowed: Vec<String>,
}

/// One source-owned contract: a relative public declaration path and the
/// environment-key names its consumers may read. Values are never configured.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct EnvironmentContract {
    pub(crate) path: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) keys: Vec<String>,
}

/// A closed language reader over explicit relative source paths. Requiring the
/// paths prevents a validator from discovering and opening an undeclared file.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct EnvironmentDetector {
    pub(crate) language: EnvironmentLanguage,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) paths: Vec<String>,
}

/// The deliberately small curated detector vocabulary.
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize, JsonSchema,
)]
#[serde(rename_all = "kebab-case")]
pub(crate) enum EnvironmentLanguage {
    Rust,
    TypeScript,
    FSharp,
    Go,
    Terraform,
    Ansible,
}

/// A narrowly scoped, auditable finding suppression. All dimensions are exact
/// strings so a consumer cannot suppress a whole class of detector output.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct EnvironmentAllowlist {
    pub(crate) rule: String,
    pub(crate) path: String,
    pub(crate) key: String,
    pub(crate) reason: String,
    pub(crate) owner: String,
    pub(crate) expiry: String,
}

/// Ordered, repository-declared toolchain probes and provision vectors. Core
/// stores no installation default; a missing section refuses its command.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct Toolchains {
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) entries: Vec<Toolchain>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct Toolchain {
    pub(crate) id: String,
    pub(crate) executable: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) probe: Vec<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) parser: Option<ToolchainOutputParser>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) version: Option<String>,
    #[serde(
        rename = "timeout-seconds",
        default,
        skip_serializing_if = "Option::is_none"
    )]
    pub(crate) timeout_seconds: Option<u64>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) provision: Vec<ToolchainProvision>,
    #[serde(default)]
    pub(crate) required: bool,
}

/// The output shapes that Rhino can compare without reporting tool output.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "kebab-case")]
pub(crate) enum ToolchainOutputParser {
    FirstLine,
    FirstToken,
}

/// One platform-specific no-shell provision vector.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct ToolchainProvision {
    pub(crate) platform: String,
    pub(crate) executable: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) arguments: Vec<String>,
}

/// Canonical requirements and the adapter profiles that must preserve them.
///
/// The canonical bodies themselves are deliberately not duplicated here: they
/// remain the repository's root instruction and `.agents` trees. This group
/// states only the portable meaning each profile must be able to carry.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct Harness {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) canonical: Option<Canonical>,
    #[serde(default)]
    pub(crate) requirements: Requirements,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) profiles: Vec<Profile>,
    /// Globs naming files outside every adapter family that adapter
    /// validation inspects for a `Rhino generated` marker. Generation never
    /// writes such a file, so a marker there claims an ownership nothing
    /// holds, and is reported as `stale-marker`.
    #[serde(
        default,
        rename = "marker-surfaces",
        skip_serializing_if = "Vec::is_empty"
    )]
    pub(crate) marker_surfaces: Vec<String>,
}

/// The field vocabulary the repository uses in canonical front matter. Rhino
/// reads those declared keys; it does not assume a particular catalog writes
/// grants as `capabilities` or `requires`.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct Canonical {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) agents: Option<CanonicalAgent>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) skills: Option<CanonicalDocument>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct CanonicalDocument {
    pub(crate) name: String,
    pub(crate) description: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct CanonicalAgent {
    pub(crate) name: String,
    pub(crate) description: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) tier: Option<String>,
    pub(crate) grants: String,
    pub(crate) denials: String,
    pub(crate) constraints: String,
}

/// One complete axis set. A profile must represent every item required on each
/// axis; an empty axis means the repository has no requirement of that kind.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct Requirements {
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) capabilities: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) grants: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) denials: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) constraints: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) routes: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) identities: Vec<String>,
}

/// A data-declared adapter family. `id` is an opaque repository identifier;
/// core code neither names nor branches on any particular consumer. Each
/// generated document declares its native path and representation rather than
/// inheriting a broad vendor directory as a mutable root.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct Profile {
    pub(crate) id: String,
    #[serde(default)]
    pub(crate) supports: Requirements,
    #[serde(rename = "instruction-adapter", default)]
    pub(crate) instruction_adapter: Option<InstructionAdapter>,
    #[serde(rename = "agent-adapter", default)]
    pub(crate) agent_adapter: Option<Adapter>,
    #[serde(rename = "skill-adapter", default)]
    pub(crate) skill_adapter: Option<Adapter>,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub(crate) tiers: BTreeMap<String, Tier>,
}

/// One exact root-instruction adapter. It is managed as an individual file so
/// a route such as `CLAUDE.md` never grants ownership of the repository root.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct InstructionAdapter {
    pub(crate) path: String,
    pub(crate) route: String,
}

/// One native profile representation of canonical agents or skills.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct Adapter {
    /// The output pattern, with exactly one `{name}` placeholder.
    pub(crate) path: String,
    pub(crate) format: AdapterFormat,
    /// `body` for front-matter prose, or the scalar field that holds a TOML
    /// route. The format decides which spelling is representable.
    #[serde(rename = "route-field")]
    pub(crate) route_field: String,
    /// The repository-owned route template, with exactly one `{path}`
    /// placeholder for the canonical source path.
    pub(crate) route: String,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub(crate) identity: BTreeMap<String, Identity>,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub(crate) fixed: BTreeMap<String, String>,
    #[serde(
        rename = "tier-fields",
        default,
        skip_serializing_if = "Option::is_none"
    )]
    pub(crate) tier_fields: Option<TierFields>,
    /// Native fields that carry a canonical metadata list, keyed by native
    /// field and rendered in the order the canonical source wrote the list.
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub(crate) lists: BTreeMap<String, CanonicalList>,
    /// The canonical agents this profile renders. When omitted, every
    /// canonical agent renders.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) agents: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) absent: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) translations: Vec<Translation>,
}

/// A canonical agent metadata list an adapter may project verbatim. The agent
/// schema's other lists, `capabilities` and `constraints`, reach native output
/// only through translations, so they are not projectable here.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "lowercase")]
pub(crate) enum CanonicalList {
    Skills,
}

impl CanonicalList {
    /// The canonical front-matter key that holds the list.
    pub(crate) const fn key(self) -> &'static str {
        match self {
            Self::Skills => "skills",
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "kebab-case")]
pub(crate) enum AdapterFormat {
    FrontMatter,
    Toml,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "lowercase")]
pub(crate) enum Identity {
    Name,
    Description,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct TierFields {
    pub(crate) model: String,
    pub(crate) effort: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct Tier {
    pub(crate) model: String,
    pub(crate) effort: String,
}

/// One capability projection into the declared native adapter format.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct Translation {
    pub(crate) when: When,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) capability: Option<String>,
    pub(crate) field: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) members: Vec<String>,
    #[serde(
        rename = "absent-members",
        default,
        skip_serializing_if = "Vec::is_empty"
    )]
    pub(crate) absent_members: Vec<String>,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub(crate) entries: BTreeMap<String, String>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "lowercase")]
pub(crate) enum When {
    Always,
    Requires,
    Denies,
    Constrains,
}

/// The declared lifecycle gates. The model owns only portable scheduling data:
/// a repository names its gates and the semantic surface at which each belongs;
/// it never delegates an untyped command string to Rhino.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct Gates {
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) entries: Vec<Gate>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) composition: Option<Composition>,
}

/// One uniquely named gate and its direct lifecycle memberships.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct Gate {
    pub(crate) id: String,
    #[serde(rename = "type")]
    pub(crate) kind: GateKind,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub(crate) inputs: BTreeMap<String, GateInput>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) command: Option<GateCommand>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) mutation: Option<MutationContract>,
    #[serde(rename = "run-on")]
    pub(crate) run_on: BTreeMap<Surface, GateMembership>,
}

/// A gate either checks a snapshot or declares the mutation contract it will
/// gain in the following delivery item. Keeping the kinds closed prevents a
/// caller from smuggling product-specific behavior into a portable lifecycle.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "kebab-case")]
pub(crate) enum GateKind {
    Check,
    Mutation,
}

/// The complete, ordered lifecycle vocabulary. `ci` is deliberately absent:
/// callers must name the actual event they mean instead of treating CI as a
/// fourth local hook.
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize, JsonSchema,
)]
#[serde(rename_all = "kebab-case")]
pub(crate) enum Surface {
    PreCommit,
    CommitMsg,
    PrePush,
    PullRequest,
    Main,
    Scheduled,
    Manual,
}

impl Surface {
    pub(crate) const ALL: [Self; 7] = [
        Self::PreCommit,
        Self::CommitMsg,
        Self::PrePush,
        Self::PullRequest,
        Self::Main,
        Self::Scheduled,
        Self::Manual,
    ];

    pub(crate) const LOCAL: [Self; 3] = [Self::PreCommit, Self::CommitMsg, Self::PrePush];

    pub(crate) const fn name(self) -> &'static str {
        match self {
            Self::PreCommit => "pre-commit",
            Self::CommitMsg => "commit-msg",
            Self::PrePush => "pre-push",
            Self::PullRequest => "pull-request",
            Self::Main => "main",
            Self::Scheduled => "scheduled",
            Self::Manual => "manual",
        }
    }
}

/// A direct membership can explain why a pull-request includes a gate that no
/// local quality surface owns. The reason is deliberately attached to the gate
/// itself rather than a broad CI switch, so a reader can audit each extra.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct GateMembership {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) reason: Option<String>,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub(crate) bind: BTreeMap<String, InputBinding>,
}

/// The portable input vocabulary. A product may use a range to invoke another
/// tool, but the configuration never names that tool as an input type.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct GateInput {
    pub(crate) kind: InputKind,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "kebab-case")]
pub(crate) enum InputKind {
    Files,
    CommitMessage,
    CommitRange,
    RepositoryState,
}

/// A surface supplies an input from one generic source. It cannot replace a
/// gate's command: membership changes data, never behavior.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct InputBinding {
    pub(crate) source: InputSource,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) range: Option<RangeSelector>,
    /// A repository-declared comparison ref used only when a pre-push update
    /// creates a new ref and therefore supplies no remote base object.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) fallback: Option<String>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "kebab-case")]
pub(crate) enum InputSource {
    GitIndex,
    HookMessageFile,
    PushUpdates,
    ExplicitRange,
    Checkout,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "kebab-case")]
pub(crate) enum RangeSelector {
    Explicit,
}

/// An executable plus typed argv and explicitly named environment projections.
/// There is intentionally no string command field and no template expansion.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct GateCommand {
    pub(crate) executable: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) args: Vec<Argument>,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub(crate) environment: BTreeMap<String, EnvironmentProjection>,
}

/// A mutation has one local index operation and one disposable PR replay.
/// Pairing them in the declaration prevents a formatter from becoming a local
/// convenience command that has never faced its pull-request safety proof.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct MutationContract {
    pub(crate) local: LocalMutation,
    pub(crate) ci: CiMutation,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "kebab-case")]
pub(crate) enum LocalMutation {
    ApplyIndex,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "kebab-case")]
pub(crate) enum CiMutation {
    VerifyClean,
}

/// Exactly one literal or resolved input reaches one argv position.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct Argument {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) literal: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) input: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) expand: Option<ArgumentExpansion>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "kebab-case")]
pub(crate) enum ArgumentExpansion {
    Single,
    Repeat,
}

/// One environment variable, mapped to one declared input field. Values never
/// come from inherited process environment or a shell template.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct EnvironmentProjection {
    pub(crate) input: String,
}

/// The one relationship that can compose lifecycle surfaces. It is explicit so
/// an omitted setting never quietly becomes a product default.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct Composition {
    #[serde(rename = "pull-request")]
    pub(crate) pull_request: PullRequestComposition,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub(crate) struct PullRequestComposition {
    pub(crate) relation: CompositionRelation,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "kebab-case")]
pub(crate) enum CompositionRelation {
    Exact,
    AtLeast,
}

/// Policies are grouped by portable concern. Each sub-group is closed so a
/// spelling error cannot become an unenforced policy, yet absence still means
/// the named concern was not adopted.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct Policies {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub markdown: Option<MarkdownPolicy>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub governance: Option<GovernancePolicy>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub conventions: Option<ConventionsPolicy>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub plans: Option<PlansPolicy>,
}

/// Parse structural shape before any later gate or policy semantics.
pub fn parse(text: &str) -> Result<Document, ConfigError> {
    let raw: Value = yaml_serde::from_str(text).map_err(malformed)?;
    validate_core_keys(&raw)?;

    let document: Document = yaml_serde::from_str(text).map_err(malformed)?;
    validate_extensions(&document.extensions)?;
    if let Some(gates) = &document.gates {
        validate_gates(gates)?;
    }
    Ok(document)
}

/// Draft 2020-12 bytes with a trailing newline for stable editor artifacts.
pub fn schema_bytes() -> Result<Vec<u8>, serde_json::Error> {
    let settings = SchemaSettings::draft2020_12();
    let schema = SchemaGenerator::new(settings).into_root_schema_for::<Document>();
    let mut value = serde_json::to_value(schema)?;

    // Schemars describes the extension payload but has no field annotation for
    // the namespace grammar. JSON Schema carries that structural rule so an
    // editor and the runtime reject the same owner spelling.
    if let Some(extension) = value
        .get_mut("properties")
        .and_then(Value::as_object_mut)
        .and_then(|properties| properties.get_mut("extensions"))
        .and_then(Value::as_object_mut)
    {
        extension.insert(
            "propertyNames".to_string(),
            serde_json::json!({ "pattern": "^[a-z][a-z0-9-]*$" }),
        );
    }

    let mut rendered = String::new();
    write_schema_json(&mut rendered, &value, 0);
    rendered.push('\n');
    Ok(rendered.into_bytes())
}

/// Render the artifact with the repository's JSON layout, without asking the
/// generator to shell out to a formatter. Schemars already gives deterministic
/// object order; the only layout choice Prettier makes differently from
/// `serde_json::to_vec_pretty` is keeping short scalar arrays on one line.
fn write_schema_json(rendered: &mut String, value: &Value, depth: usize) {
    match value {
        Value::Array(values) if fits_scalar_array(values, depth) => {
            rendered.push('[');
            for (index, scalar) in values.iter().enumerate() {
                rendered.push_str(&serde_json::to_string(scalar).expect("scalar serializes"));
                if index + 1 != values.len() {
                    rendered.push_str(", ");
                }
            }
            rendered.push(']');
        }
        Value::Array(values) => {
            rendered.push_str("[\n");
            for (index, item) in values.iter().enumerate() {
                indent(rendered, depth + 1);
                write_schema_json(rendered, item, depth + 1);
                if index + 1 != values.len() {
                    rendered.push(',');
                }
                rendered.push('\n');
            }
            indent(rendered, depth);
            rendered.push(']');
        }
        Value::Object(entries) if entries.is_empty() => rendered.push_str("{}"),
        Value::Object(entries) => {
            rendered.push_str("{\n");
            for (index, (key, item)) in entries.iter().enumerate() {
                indent(rendered, depth + 1);
                rendered.push_str(&serde_json::to_string(key).expect("key serializes"));
                rendered.push_str(": ");
                write_schema_json(rendered, item, depth + 1);
                if index + 1 != entries.len() {
                    rendered.push(',');
                }
                rendered.push('\n');
            }
            indent(rendered, depth);
            rendered.push('}');
        }
        scalar => rendered.push_str(&serde_json::to_string(scalar).expect("scalar serializes")),
    }
}

fn fits_scalar_array(values: &[Value], depth: usize) -> bool {
    values.iter().all(|value| {
        matches!(
            value,
            Value::Null | Value::Bool(_) | Value::Number(_) | Value::String(_)
        )
    }) && serde_json::to_string(values)
        .map(|text| depth * 2 + text.len() + values.len().saturating_sub(1) <= 80)
        .unwrap_or(false)
}

fn indent(rendered: &mut String, depth: usize) {
    rendered.push_str(&"  ".repeat(depth));
}

fn malformed(error: yaml_serde::Error) -> ConfigError {
    ConfigError::Malformed(error.to_string())
}

fn validate_core_keys(raw: &Value) -> Result<(), ConfigError> {
    let mapping = raw.as_object().ok_or_else(|| ConfigError::Semantic {
        key: "document".to_string(),
        line: 1,
        reason: "is not a mapping of grouped core keys".to_string(),
    })?;

    for key in mapping.keys() {
        if !CORE_KEYS.contains(&key.as_str()) {
            return Err(ConfigError::Semantic {
                key: key.clone(),
                line: 1,
                reason: "is an unknown core key".to_string(),
            });
        }
    }
    Ok(())
}

fn validate_extensions(
    extensions: &BTreeMap<String, Map<String, Value>>,
) -> Result<(), ConfigError> {
    for owner in extensions.keys() {
        if !is_extension_owner(owner) {
            return Err(ConfigError::Semantic {
                key: owner.clone(),
                line: 1,
                reason: "is not a portable extension owner".to_string(),
            });
        }
    }
    Ok(())
}

fn validate_gates(gates: &Gates) -> Result<(), ConfigError> {
    let mut declared = BTreeMap::new();
    for gate in &gates.entries {
        if !is_gate_id(&gate.id) {
            return semantic(
                &gate.id,
                "is not a portable semantic gate ID; expected lowercase kebab-case",
            );
        }
        if declared.insert(gate.id.as_str(), gate).is_some() {
            return semantic(
                &gate.id,
                "is duplicated; each lifecycle gate needs one semantic ID",
            );
        }
        if gate.run_on.is_empty() {
            return semantic(
                &gate.id,
                "has no direct lifecycle membership; declare at least one `run-on` surface",
            );
        }
        validate_gate_inputs(gate)?;
    }

    let local: BTreeMap<&str, &Gate> = gates
        .entries
        .iter()
        .filter(|gate| {
            Surface::LOCAL
                .iter()
                .any(|surface| gate.run_on.contains_key(surface))
        })
        .map(|gate| (gate.id.as_str(), gate))
        .collect();
    let pull_request: BTreeMap<&str, &Gate> = gates
        .entries
        .iter()
        .filter(|gate| gate.run_on.contains_key(&Surface::PullRequest))
        .map(|gate| (gate.id.as_str(), gate))
        .collect();

    if local.is_empty() && pull_request.is_empty() {
        return Ok(());
    }

    let Some(composition) = &gates.composition else {
        return semantic(
            "gates.composition",
            "must declare the pull-request relation when local or pull-request gates exist",
        );
    };

    for id in local.keys() {
        if !pull_request.contains_key(id) {
            return semantic(
                id,
                "is a local quality gate missing from pull-request composition",
            );
        }
    }

    for (id, gate) in &pull_request {
        if local.contains_key(id) {
            continue;
        }
        match composition.pull_request.relation {
            CompositionRelation::Exact => {
                return semantic(
                    id,
                    "is an extra pull-request gate under the exact composition relation",
                );
            }
            CompositionRelation::AtLeast => {
                let reason = gate
                    .run_on
                    .get(&Surface::PullRequest)
                    .and_then(|membership| membership.reason.as_deref())
                    .map(str::trim)
                    .filter(|reason| !reason.is_empty());
                if reason.is_none() {
                    return semantic(
                        id,
                        "is an extra pull-request gate and needs its own non-empty reason",
                    );
                }
            }
        }
    }
    Ok(())
}

fn validate_gate_inputs(gate: &Gate) -> Result<(), ConfigError> {
    validate_mutation_contract(gate)?;
    for name in gate.inputs.keys() {
        if !is_gate_id(name) {
            return semantic(
                name,
                "is not a portable input name; expected lowercase kebab-case",
            );
        }
    }

    for (surface, membership) in &gate.run_on {
        for name in membership.bind.keys() {
            if !gate.inputs.contains_key(name) {
                return semantic(
                    name,
                    &format!(
                        "binds an input not declared by `{}` at {}",
                        gate.id,
                        surface.name()
                    ),
                );
            }
        }
        for (name, input) in &gate.inputs {
            let Some(binding) = membership.bind.get(name) else {
                return semantic(
                    name,
                    &format!("has no binding for `{}` at {}", gate.id, surface.name()),
                );
            };
            if !source_accepts(input.kind, binding.source) {
                return semantic(
                    name,
                    &format!(
                        "declares `{}` but {} is not a compatible source",
                        input_kind_name(input.kind),
                        input_source_name(binding.source)
                    ),
                );
            }
            if binding.range.is_some() && binding.source != InputSource::ExplicitRange {
                return semantic(
                    name,
                    "uses `range` with a source other than `explicit-range`",
                );
            }
            if binding.source == InputSource::ExplicitRange
                && binding.range != Some(RangeSelector::Explicit)
            {
                return semantic(name, "uses `explicit-range` without `range: explicit`");
            }
            if binding.fallback.is_some() && binding.source != InputSource::PushUpdates {
                return semantic(
                    name,
                    "uses `fallback` with a source other than `push-updates`",
                );
            }
            if binding.source == InputSource::PushUpdates {
                let Some(fallback) = binding.fallback.as_deref().map(str::trim) else {
                    return semantic(name, "uses `push-updates` without a declared fallback ref");
                };
                if fallback.is_empty()
                    || fallback.starts_with('-')
                    || fallback.contains(char::is_whitespace)
                {
                    return semantic(name, "declares an invalid push-update fallback ref");
                }
            }
        }
    }

    match &gate.command {
        Some(command) => validate_command(gate, command),
        None if gate.inputs.is_empty() => Ok(()),
        None => semantic(
            &gate.id,
            "declares typed inputs but no typed executable and argv command",
        ),
    }
}

fn validate_mutation_contract(gate: &Gate) -> Result<(), ConfigError> {
    match (gate.kind, &gate.mutation) {
        (GateKind::Check, None) => Ok(()),
        (GateKind::Check, Some(_)) => semantic(
            &gate.id,
            "is a check but declares a mutation contract; use `type: mutation`",
        ),
        (GateKind::Mutation, None) => semantic(
            &gate.id,
            "is a mutation but does not declare paired `apply-index` and `verify-clean` behavior",
        ),
        (GateKind::Mutation, Some(_)) => {
            if !gate.run_on.contains_key(&Surface::PreCommit)
                || !gate.run_on.contains_key(&Surface::PullRequest)
            {
                return semantic(
                    &gate.id,
                    "is a mutation and must run at both pre-commit and pull-request",
                );
            }
            if gate
                .run_on
                .keys()
                .any(|surface| !matches!(surface, Surface::PreCommit | Surface::PullRequest))
            {
                return semantic(
                    &gate.id,
                    "is a mutation outside its closed pre-commit and pull-request lifecycle",
                );
            }
            Ok(())
        }
    }
}

fn validate_command(gate: &Gate, command: &GateCommand) -> Result<(), ConfigError> {
    if command.executable.trim().is_empty() || command.executable.contains(['\n', '\r']) {
        return semantic(&gate.id, "has an empty or line-breaking executable");
    }

    let mut projected: BTreeMap<String, ()> = BTreeMap::new();
    for argument in &command.args {
        match (&argument.literal, &argument.input, argument.expand) {
            (Some(_), None, None) => {}
            (None, Some(reference), Some(expansion)) => {
                let (name, kind, field) = validate_reference(gate, reference)?;
                if expansion == ArgumentExpansion::Repeat
                    && !(kind == InputKind::Files && field == "paths")
                {
                    return semantic(
                        reference,
                        "uses `expand: repeat` for a field that is not a file-path sequence",
                    );
                }
                projected.insert(name.to_string(), ());
            }
            (Some(_), None, Some(_)) => {
                return semantic("args", "adds `expand` to a literal argv item");
            }
            (None, Some(_), None) => {
                return semantic(
                    "args",
                    "omits the closed expansion mode from an input argv item",
                );
            }
            _ => {
                return semantic(
                    "args",
                    "must hold exactly one literal or typed input argv item",
                );
            }
        }
    }

    for (name, projection) in &command.environment {
        if !is_environment_name(name) {
            return semantic(name, "is not an uppercase environment projection name");
        }
        let (input, _, _) = validate_reference(gate, &projection.input)?;
        projected.insert(input.to_string(), ());
    }

    for name in gate.inputs.keys() {
        if !projected.contains_key(name) {
            return semantic(
                name,
                "is declared but never projected through typed argv or environment",
            );
        }
    }
    Ok(())
}

fn validate_reference<'a>(
    gate: &'a Gate,
    reference: &'a str,
) -> Result<(&'a str, InputKind, &'a str), ConfigError> {
    let Some((name, field)) = reference.split_once('.') else {
        return semantic(reference, "is not an `input.field` reference");
    };
    if name.is_empty() || field.is_empty() || field.contains('.') {
        return semantic(reference, "is not an `input.field` reference");
    }
    let Some(input) = gate.inputs.get(name) else {
        return semantic(reference, "names an input this gate does not declare");
    };
    if !input_has_field(input.kind, field) {
        return semantic(
            reference,
            &format!(
                "does not name a field available on a {} input",
                input_kind_name(input.kind)
            ),
        );
    }
    Ok((name, input.kind, field))
}

fn source_accepts(kind: InputKind, source: InputSource) -> bool {
    matches!(
        (kind, source),
        (
            InputKind::Files,
            InputSource::GitIndex | InputSource::ExplicitRange | InputSource::Checkout
        ) | (
            InputKind::CommitMessage,
            InputSource::HookMessageFile | InputSource::ExplicitRange
        ) | (
            InputKind::CommitRange,
            InputSource::PushUpdates | InputSource::ExplicitRange
        ) | (InputKind::RepositoryState, InputSource::Checkout)
    )
}

fn input_has_field(kind: InputKind, field: &str) -> bool {
    matches!(
        (kind, field),
        (InputKind::Files, "paths")
            | (InputKind::CommitMessage, "text")
            | (InputKind::CommitRange, "base" | "head")
            | (InputKind::RepositoryState, "root")
    )
}

fn input_kind_name(kind: InputKind) -> &'static str {
    match kind {
        InputKind::Files => "files",
        InputKind::CommitMessage => "commit-message",
        InputKind::CommitRange => "commit-range",
        InputKind::RepositoryState => "repository-state",
    }
}

fn input_source_name(source: InputSource) -> &'static str {
    match source {
        InputSource::GitIndex => "git-index",
        InputSource::HookMessageFile => "hook-message-file",
        InputSource::PushUpdates => "push-updates",
        InputSource::ExplicitRange => "explicit-range",
        InputSource::Checkout => "checkout",
    }
}

fn is_environment_name(name: &str) -> bool {
    let mut characters = name.chars();
    matches!(characters.next(), Some(first) if first.is_ascii_uppercase())
        && characters.all(|character| {
            character.is_ascii_uppercase() || character.is_ascii_digit() || character == '_'
        })
}

fn semantic<T>(key: &str, reason: &str) -> Result<T, ConfigError> {
    Err(ConfigError::Semantic {
        key: key.to_string(),
        line: 1,
        reason: reason.to_string(),
    })
}

fn is_gate_id(id: &str) -> bool {
    let mut characters = id.chars();
    matches!(characters.next(), Some(first) if first.is_ascii_lowercase())
        && characters.all(|character| {
            character.is_ascii_lowercase() || character.is_ascii_digit() || character == '-'
        })
}

fn is_extension_owner(owner: &str) -> bool {
    let mut characters = owner.chars();
    matches!(characters.next(), Some(first) if first.is_ascii_lowercase())
        && characters.all(|character| {
            character.is_ascii_lowercase() || character.is_ascii_digit() || character == '-'
        })
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::Value;
    use std::collections::BTreeMap;

    fn binding(source: InputSource) -> InputBinding {
        InputBinding {
            source,
            range: None,
            fallback: None,
        }
    }

    fn membership(bind: BTreeMap<String, InputBinding>) -> GateMembership {
        GateMembership { reason: None, bind }
    }

    fn check_gate(
        inputs: BTreeMap<String, GateInput>,
        command: Option<GateCommand>,
        bind: BTreeMap<String, InputBinding>,
    ) -> Gate {
        Gate {
            id: "check".to_string(),
            kind: GateKind::Check,
            inputs,
            command,
            mutation: None,
            run_on: BTreeMap::from([(Surface::Manual, membership(bind))]),
        }
    }

    fn assert_refusal<T>(result: Result<T, ConfigError>, expected: &str) {
        let error = match result {
            Ok(_) => panic!("the malformed declaration is refused"),
            Err(error) => error,
        };
        assert!(
            error.to_string().contains(expected),
            "expected `{expected}` in `{error}`"
        );
    }

    #[test]
    fn extensions_round_trip_without_core_interpretation() {
        let text = "schema: rhino/repo-config/v2\nextensions:\n  owner-tools:\n    arbitrary: [one, two]\n";
        let document = parse(text).expect("opaque extension mapping parses");
        let encoded = serde_json::to_string(&document).expect("extension serializes");
        let decoded: Document = serde_json::from_str(&encoded).expect("extension deserializes");
        assert_eq!(decoded, document);
    }

    #[test]
    fn schema_bytes_are_deterministic_and_name_draft_2020_12() {
        let first = schema_bytes().expect("schema serializes");
        assert_eq!(first, schema_bytes().expect("schema serializes again"));
        let value: Value = serde_json::from_slice(&first).expect("schema is JSON");
        assert_eq!(
            value["$schema"],
            "https://json-schema.org/draft/2020-12/schema"
        );
        assert_eq!(
            value["properties"]["extensions"]["propertyNames"]["pattern"],
            "^[a-z][a-z0-9-]*$"
        );
    }

    #[test]
    fn schema_scalar_arrays_use_the_repository_json_layout() {
        let mut rendered = String::new();
        write_schema_json(&mut rendered, &serde_json::json!(["string", "null"]), 0);
        assert_eq!(rendered, "[\"string\", \"null\"]");

        rendered.clear();
        write_schema_json(&mut rendered, &serde_json::json!({}), 0);
        assert_eq!(rendered, "{}");
    }

    #[test]
    fn source_and_consumer_modelines_keep_their_schema_locations_distinct() {
        let producer = include_str!("../../schemas/repo-config/v2.example.yml");
        let consumer = include_str!("../../specs/fixtures/v0-4/consumer-repo-config.yml");

        assert_eq!(
            producer.lines().next(),
            Some("# yaml-language-server: $schema=./v2.schema.json")
        );
        assert_eq!(
            consumer.lines().next(),
            Some(
                "# yaml-language-server: $schema=https://raw.githubusercontent.com/wahidyankf/rhino/v0.4.0/schemas/repo-config/v2.schema.json"
            )
        );
        assert!(
            parse(producer).is_ok(),
            "the source example is locally valid"
        );
        assert!(
            parse(consumer).is_ok(),
            "the consumer fixture is locally valid"
        );
    }

    #[test]
    fn curated_environment_and_toolchain_operation_vocabulary_parses_from_consumer_yaml() {
        let text = "schema: rhino/repo-config/v2\nenvironment:\n  contracts:\n    - path: .env.example\n      keys: [SERVICE_KEY]\n  detectors:\n    - language: rust\n      paths: [src/rust.rs]\n    - language: type-script\n      paths: [src/site.ts]\n    - language: f-sharp\n      paths: [src/program.fs]\n    - language: go\n      paths: [src/main.go]\n    - language: terraform\n      paths: [infra/main.tf]\n    - language: ansible\n      paths: [playbook.yml]\ntoolchains:\n  entries:\n    - id: formatter\n      executable: formatter\n      probe: [--version]\n      parser: first-token\n      version: 1.2.3\n      timeout-seconds: 30\n      required: true\n      provision:\n        - platform: macos\n          executable: installer\n          arguments: [install]\n";

        let document = parse(text).expect("the declared operation vocabulary parses");

        assert_eq!(document.environment.as_ref().unwrap().detectors.len(), 6);
        assert_eq!(document.toolchains.as_ref().unwrap().entries.len(), 1);
    }

    #[test]
    fn grouped_core_and_extension_boundaries_refuse_nonportable_shape() {
        assert_refusal(
            validate_core_keys(&serde_json::json!(["schema"])),
            "not a mapping",
        );
        assert_refusal(
            validate_core_keys(
                &serde_json::json!({"schema": "rhino/repo-config/v2", "legacy": {}}),
            ),
            "unknown core key",
        );
        assert_refusal(
            validate_extensions(&BTreeMap::from([(
                "Owner".to_string(),
                serde_json::Map::new(),
            )])),
            "not a portable extension owner",
        );

        assert!(is_gate_id("fast-check"));
        assert!(!is_gate_id("Fast_check"));
        assert!(is_extension_owner("owner-tools"));
        assert!(!is_extension_owner("owner/tools"));
        assert!(is_environment_name("CHECK_ROOT_2"));
        assert!(!is_environment_name("check_root"));
        assert!(source_accepts(InputKind::Files, InputSource::GitIndex));
        assert!(source_accepts(InputKind::Files, InputSource::ExplicitRange));
        assert!(source_accepts(
            InputKind::CommitMessage,
            InputSource::HookMessageFile
        ));
        assert!(source_accepts(
            InputKind::CommitMessage,
            InputSource::ExplicitRange
        ));
        assert!(source_accepts(
            InputKind::CommitRange,
            InputSource::ExplicitRange
        ));
        assert!(source_accepts(
            InputKind::RepositoryState,
            InputSource::Checkout
        ));
        assert!(!source_accepts(
            InputKind::Files,
            InputSource::HookMessageFile
        ));
        assert!(input_has_field(InputKind::Files, "paths"));
        assert!(input_has_field(InputKind::CommitMessage, "text"));
        assert!(input_has_field(InputKind::CommitRange, "base"));
        assert!(input_has_field(InputKind::CommitRange, "head"));
        assert!(input_has_field(InputKind::RepositoryState, "root"));
        assert!(!input_has_field(InputKind::Files, "root"));
        assert_eq!(input_kind_name(InputKind::Files), "files");
        assert_eq!(input_kind_name(InputKind::CommitMessage), "commit-message");
        assert_eq!(input_kind_name(InputKind::CommitRange), "commit-range");
        assert_eq!(
            input_kind_name(InputKind::RepositoryState),
            "repository-state"
        );
        assert_eq!(input_source_name(InputSource::GitIndex), "git-index");
        assert_eq!(
            input_source_name(InputSource::HookMessageFile),
            "hook-message-file"
        );
        assert_eq!(input_source_name(InputSource::PushUpdates), "push-updates");
        assert_eq!(
            input_source_name(InputSource::ExplicitRange),
            "explicit-range"
        );
        assert_eq!(input_source_name(InputSource::Checkout), "checkout");
    }

    #[test]
    fn grouped_gate_lifecycle_validation_refuses_each_uncomposable_declaration() {
        let no_command = || check_gate(BTreeMap::new(), None, BTreeMap::new());

        let mut invalid_id = no_command();
        invalid_id.id = "Not-portable".to_string();
        assert_refusal(
            validate_gates(&Gates {
                entries: vec![invalid_id],
                composition: None,
            }),
            "gate ID",
        );

        let duplicate = no_command();
        assert_refusal(
            validate_gates(&Gates {
                entries: vec![duplicate.clone(), duplicate],
                composition: None,
            }),
            "duplicated",
        );

        let mut no_membership = no_command();
        no_membership.run_on.clear();
        assert_refusal(
            validate_gates(&Gates {
                entries: vec![no_membership],
                composition: None,
            }),
            "no direct lifecycle membership",
        );

        let mut local_only = no_command();
        local_only.run_on = BTreeMap::from([(Surface::PreCommit, membership(BTreeMap::new()))]);
        assert_refusal(
            validate_gates(&Gates {
                entries: vec![local_only.clone()],
                composition: None,
            }),
            "must declare the pull-request relation",
        );
        assert_refusal(
            validate_gates(&Gates {
                entries: vec![local_only],
                composition: Some(Composition {
                    pull_request: PullRequestComposition {
                        relation: CompositionRelation::Exact,
                    },
                }),
            }),
            "missing from pull-request composition",
        );

        let mut pull_request_only = no_command();
        pull_request_only.run_on =
            BTreeMap::from([(Surface::PullRequest, membership(BTreeMap::new()))]);
        assert_refusal(
            validate_gates(&Gates {
                entries: vec![pull_request_only.clone()],
                composition: Some(Composition {
                    pull_request: PullRequestComposition {
                        relation: CompositionRelation::Exact,
                    },
                }),
            }),
            "extra pull-request gate",
        );
        assert_refusal(
            validate_gates(&Gates {
                entries: vec![pull_request_only],
                composition: Some(Composition {
                    pull_request: PullRequestComposition {
                        relation: CompositionRelation::AtLeast,
                    },
                }),
            }),
            "needs its own non-empty reason",
        );
    }

    #[test]
    fn grouped_gate_input_validation_refuses_every_untyped_or_incompatible_binding() {
        let files = BTreeMap::from([(
            "files".to_string(),
            GateInput {
                kind: InputKind::Files,
            },
        )]);
        let executable = Some(GateCommand {
            executable: "check".to_string(),
            args: vec![Argument {
                literal: None,
                input: Some("files.paths".to_string()),
                expand: Some(ArgumentExpansion::Repeat),
            }],
            environment: BTreeMap::new(),
        });

        let mut invalid_name = check_gate(
            BTreeMap::from([(
                "Files".to_string(),
                GateInput {
                    kind: InputKind::Files,
                },
            )]),
            executable.clone(),
            BTreeMap::from([("Files".to_string(), binding(InputSource::Checkout))]),
        );
        assert_refusal(validate_gate_inputs(&invalid_name), "portable input name");
        invalid_name.inputs.clear();

        assert_refusal(
            validate_gate_inputs(&check_gate(
                BTreeMap::new(),
                None,
                BTreeMap::from([("unknown".to_string(), binding(InputSource::Checkout))]),
            )),
            "binds an input not declared",
        );
        assert_refusal(
            validate_gate_inputs(&check_gate(
                files.clone(),
                executable.clone(),
                BTreeMap::new(),
            )),
            "has no binding",
        );
        assert_refusal(
            validate_gate_inputs(&check_gate(
                files.clone(),
                executable.clone(),
                BTreeMap::from([("files".to_string(), binding(InputSource::HookMessageFile))]),
            )),
            "not a compatible source",
        );

        let range = BTreeMap::from([(
            "range".to_string(),
            GateInput {
                kind: InputKind::CommitRange,
            },
        )]);
        let range_command = Some(GateCommand {
            executable: "check".to_string(),
            args: vec![Argument {
                literal: None,
                input: Some("range.base".to_string()),
                expand: Some(ArgumentExpansion::Single),
            }],
            environment: BTreeMap::new(),
        });
        let mut push_with_range = binding(InputSource::PushUpdates);
        push_with_range.range = Some(RangeSelector::Explicit);
        push_with_range.fallback = Some("refs/main".to_string());
        assert_refusal(
            validate_gate_inputs(&check_gate(
                range.clone(),
                range_command.clone(),
                BTreeMap::from([("range".to_string(), push_with_range)]),
            )),
            "source other than `explicit-range`",
        );
        assert_refusal(
            validate_gate_inputs(&check_gate(
                range.clone(),
                range_command.clone(),
                BTreeMap::from([("range".to_string(), binding(InputSource::ExplicitRange))]),
            )),
            "without `range: explicit`",
        );
        let mut checkout_fallback = binding(InputSource::Checkout);
        checkout_fallback.fallback = Some("refs/main".to_string());
        assert_refusal(
            validate_gate_inputs(&check_gate(
                files.clone(),
                executable.clone(),
                BTreeMap::from([("files".to_string(), checkout_fallback)]),
            )),
            "source other than `push-updates`",
        );
        assert_refusal(
            validate_gate_inputs(&check_gate(
                range.clone(),
                range_command.clone(),
                BTreeMap::from([("range".to_string(), binding(InputSource::PushUpdates))]),
            )),
            "without a declared fallback ref",
        );
        let mut invalid_fallback = binding(InputSource::PushUpdates);
        invalid_fallback.fallback = Some("-not-a-ref".to_string());
        assert_refusal(
            validate_gate_inputs(&check_gate(
                range.clone(),
                range_command,
                BTreeMap::from([("range".to_string(), invalid_fallback)]),
            )),
            "invalid push-update fallback ref",
        );
        assert_refusal(
            validate_gate_inputs(&check_gate(
                files,
                None,
                BTreeMap::from([("files".to_string(), binding(InputSource::Checkout))]),
            )),
            "no typed executable",
        );
    }

    #[test]
    fn grouped_gate_mutation_command_and_projection_validation_refuse_ambiguous_forms() {
        let mut gate = check_gate(BTreeMap::new(), None, BTreeMap::new());
        gate.mutation = Some(MutationContract {
            local: LocalMutation::ApplyIndex,
            ci: CiMutation::VerifyClean,
        });
        assert_refusal(
            validate_mutation_contract(&gate),
            "is a check but declares a mutation",
        );

        gate.kind = GateKind::Mutation;
        gate.mutation = None;
        assert_refusal(validate_mutation_contract(&gate), "does not declare paired");
        gate.mutation = Some(MutationContract {
            local: LocalMutation::ApplyIndex,
            ci: CiMutation::VerifyClean,
        });
        assert_refusal(validate_mutation_contract(&gate), "must run at both");
        gate.run_on = BTreeMap::from([
            (Surface::PreCommit, membership(BTreeMap::new())),
            (Surface::PullRequest, membership(BTreeMap::new())),
            (Surface::Manual, membership(BTreeMap::new())),
        ]);
        assert_refusal(validate_mutation_contract(&gate), "outside its closed");

        let files = BTreeMap::from([(
            "files".to_string(),
            GateInput {
                kind: InputKind::Files,
            },
        )]);
        let command_gate = check_gate(files, None, BTreeMap::new());
        assert_refusal(
            validate_command(
                &command_gate,
                &GateCommand {
                    executable: "\n".to_string(),
                    args: Vec::new(),
                    environment: BTreeMap::new(),
                },
            ),
            "line-breaking executable",
        );
        for (argument, expected) in [
            (
                Argument {
                    literal: Some("--check".to_string()),
                    input: None,
                    expand: Some(ArgumentExpansion::Single),
                },
                "adds `expand` to a literal",
            ),
            (
                Argument {
                    literal: None,
                    input: Some("files.paths".to_string()),
                    expand: None,
                },
                "omits the closed expansion mode",
            ),
            (
                Argument {
                    literal: Some("--check".to_string()),
                    input: Some("files.paths".to_string()),
                    expand: None,
                },
                "exactly one literal or typed input",
            ),
        ] {
            assert_refusal(
                validate_command(
                    &command_gate,
                    &GateCommand {
                        executable: "check".to_string(),
                        args: vec![argument],
                        environment: BTreeMap::new(),
                    },
                ),
                expected,
            );
        }
        assert_refusal(
            validate_command(
                &command_gate,
                &GateCommand {
                    executable: "check".to_string(),
                    args: Vec::new(),
                    environment: BTreeMap::from([(
                        "lower".to_string(),
                        EnvironmentProjection {
                            input: "files.paths".to_string(),
                        },
                    )]),
                },
            ),
            "uppercase environment projection",
        );
        assert_refusal(
            validate_command(
                &command_gate,
                &GateCommand {
                    executable: "check".to_string(),
                    args: Vec::new(),
                    environment: BTreeMap::new(),
                },
            ),
            "never projected",
        );

        for (reference, expected) in [
            ("files", "not an `input.field` reference"),
            (".paths", "not an `input.field` reference"),
            ("unknown.paths", "does not declare"),
            ("files.root", "does not name a field"),
        ] {
            assert_refusal(validate_reference(&command_gate, reference), expected);
        }
    }
}
