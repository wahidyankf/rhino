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

use serde::Deserialize;
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
    #[serde(rename = "harness-parity")]
    pub harness_parity: HarnessParity,
    pub scan: Scan,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct WordBudget {
    /// Ordered: where two globs match one file, the last matching entry wins,
    /// which is how a specific surface overrides a general tree.
    pub surfaces: Vec<Surface>,
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
}

#[derive(Debug, Clone, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Harness {
    pub name: String,
    #[serde(rename = "agent-dir")]
    pub agent_dir: String,
    #[serde(rename = "agent-extension")]
    pub agent_extension: String,
    /// Per-harness rather than global, so a second harness gaining skill
    /// wrappers is one line of configuration instead of a code change.
    #[serde(rename = "command-dir", default)]
    pub command_dir: Option<String>,
    /// Where this harness declares what it may reach, and in which format.
    ///
    /// The two travel together because neither means anything alone: a path
    /// with no format cannot be parsed and a format with no path names
    /// nothing. Nesting them makes half a declaration unrepresentable rather
    /// than merely detectable.
    #[serde(default)]
    pub capability: Option<Capability>,
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

/// Parse and validate configuration text.
///
/// Split from any filesystem read so the whole contract is exercisable from a
/// string, which is what lets the behaviour corpus assert on positions and
/// messages rather than only on exit codes.
pub fn parse(text: &str) -> Result<Config, ConfigError> {
    declared_schema(text)?;

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

/// The schema is declared in a leading comment rather than a key, matching the
/// consumer that already carries this file.
fn declared_schema(text: &str) -> Result<(), ConfigError> {
    for (index, raw) in text.lines().enumerate() {
        let line = raw.trim();
        if line.is_empty() {
            continue;
        }
        let Some(comment) = line.strip_prefix('#') else {
            break;
        };
        let Some((_, declared)) = comment.split_once("schema:") else {
            continue;
        };
        let declared = declared.trim();
        return if declared == SCHEMA || declared == SCHEMA_ALIAS {
            Ok(())
        } else {
            Err(ConfigError::SchemaUnrecognized {
                declared: declared.to_string(),
                line: index + 1,
            })
        };
    }
    Err(ConfigError::SchemaUndeclared)
}

/// The rules a well-typed document can still break.
fn check_semantics(config: &Config, text: &str) -> Result<(), ConfigError> {
    let canonical = &config.harness_parity.canonical;

    // Everything a harness is reconciled *against*: the two canonical roots and
    // the capability declaration every harness has to match. Each one is
    // meaningful only in the presence of a harness, so each follows the roster.
    let reconciled_against = [
        ("skills-root", canonical.skills_root.is_some()),
        ("agents-root", canonical.agents_root.is_some()),
        ("required-mcp", config.harness_parity.required_mcp.is_some()),
    ];

    // The two canonical roots are required alongside a roster; the required
    // server is not. Splitting the lists is the whole of the difference: a
    // repository always has instructions to reconcile, and does not always
    // have a capability server to reach.
    let required_alongside_a_roster = &reconciled_against[..2];

    // Canon with nowhere to reconcile it is a configuration error rather than a
    // clean pass: it means the reconciliation was silently skipped.
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
    } else {
        for &(key, declared) in required_alongside_a_roster {
            if !declared {
                return Err(ConfigError::Semantic {
                    key: key.to_string(),
                    line: line_of(text, "harnesses"),
                    reason: "required whenever the harness roster is non-empty".to_string(),
                });
            }
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
        paths.push(("agent-dir", harness.agent_dir.clone()));
        if let Some(capability) = &harness.capability {
            paths.push(("capability-file", capability.file.clone()));
        }
        if let Some(directory) = &harness.command_dir {
            paths.push(("command-dir", directory.clone()));
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
fn line_of(text: &str, key: &str) -> usize {
    text.lines()
        .position(|line| line.trim_start().starts_with(&format!("{key}:")))
        .map_or(1, |index| index + 1)
}
