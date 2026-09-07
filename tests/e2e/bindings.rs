//! End-to-end bindings.
//!
//! The same scenarios again, driven through the built executable's process
//! contract -- arguments in, stdout, stderr, and an exit code out -- and
//! nothing else. E2E is defined by observing only that contract, not by
//! permission to use more resources.

use crate::registry::{Binding, Exemption};

pub const BINDINGS: &[Binding] = &[
    (
        "cli-contract",
        "Every canonical nested command path succeeds",
    ),
    ("cli-contract", "Word-budget validation is isolated"),
    ("cli-contract", "Directory-map validation is isolated"),
    (
        "cli-contract",
        "A complete selected tree passes directory-map validation",
    ),
    (
        "cli-contract",
        "A selected directory without a README fails",
    ),
    ("cli-contract", "An omitted selected sibling fails"),
    (
        "cli-contract",
        "A selected tree requires recursive README directory maps",
    ),
    (
        "cli-contract",
        "Mermaid accessibility validation is isolated",
    ),
    ("cli-contract", "Root defaults to the current directory"),
    (
        "cli-contract",
        "Root is accepted before and after nested commands",
    ),
    (
        "cli-contract",
        "A selected root is the repository that gets inspected",
    ),
    (
        "cli-contract",
        "A selected root brings its own scan exclusions",
    ),
    (
        "cli-contract",
        "Leaf output has an atomic command-category prefix",
    ),
    (
        "cli-contract",
        "Inspection errors use command-specific diagnostics",
    ),
    ("cli-contract", "Help requests succeed"),
    (
        "cli-contract",
        "Version reports the embedded release identity",
    ),
    ("cli-contract", "Invalid invocations return usage failure"),
    (
        "cli-contract",
        "A governed file above its declared limit returns validation failure",
    ),
    (
        "cli-contract",
        "An incomplete directory map returns validation failure",
    ),
    (
        "cli-contract",
        "Output format is recursive for every validator",
    ),
    ("cli-contract", "A file word count is observable as JSON"),
    (
        "cli-contract",
        "Word-count inspection never reports findings",
    ),
    (
        "cli-contract",
        "JSON validation failures retain the validation exit code",
    ),
    (
        "cli-contract",
        "Unsupported output formats return usage failure",
    ),
    (
        "cli-contract",
        "Global presentation flags are accepted by every reporting leaf",
    ),
    (
        "cli-contract",
        "Help is scoped to the command path that asked for it",
    ),
    (
        "cli-contract",
        "Repeated file selection is accepted by the Mermaid leaf",
    ),
    (
        "cli-contract",
        "The Mermaid leaf reads a diagram from standard input",
    ),
    (
        "directory-map",
        "Directory-map inspection ignores other validators' concerns",
    ),
    (
        "directory-map",
        "Complete maps cover direct files and directories",
    ),
    (
        "directory-map",
        "The selected directory must stay relative and inside the repository",
    ),
    (
        "directory-map",
        "Query and fragment suffixes do not change a sibling target",
    ),
    (
        "directory-map",
        "Absolute URL and malformed map links are invalid",
    ),
    (
        "directory-map",
        "Every directory in a mapped tree needs a README",
    ),
    (
        "directory-map",
        "Every README needs a Directory Map section",
    ),
    (
        "directory-map",
        "Every direct sibling must appear in the map",
    ),
    (
        "directory-map",
        "Independent inspections report independent violations",
    ),
    ("directory-map", "A map entry must exist"),
    ("directory-map", "A map entry must be a direct sibling"),
    (
        "harness-parity",
        "Canonical instructions, skills, agents, and capability declarations pass",
    ),
    (
        "harness-parity",
        "The canonical instruction body must exist",
    ),
    (
        "harness-parity",
        "A declared instruction adapter must exist",
    ),
    (
        "harness-parity",
        "The instruction adapter may contain only the canonical import",
    ),
    (
        "harness-parity",
        "A declared instruction adapter is optional",
    ),
    (
        "harness-parity",
        "With no adapter declared, no file may import the canonical instructions",
    ),
    (
        "harness-parity",
        "A documentation page showing the import in a fenced example is not a source",
    ),
    (
        "harness-parity",
        "A harness instruction overlay is a competing source",
    ),
    (
        "harness-parity",
        "Additional or nested instruction sources fail",
    ),
    (
        "harness-parity",
        "A harness declaring a command directory needs one thin wrapper per skill",
    ),
    (
        "harness-parity",
        "A harness declaring no command directory needs no wrappers",
    ),
    (
        "harness-parity",
        "A skill wrapper's description cannot drift from the skill",
    ),
    (
        "harness-parity",
        "A skill wrapper may not grow a body of its own",
    ),
    (
        "harness-parity",
        "A skill wrapper must declare the skill it routes to",
    ),
    (
        "harness-parity",
        "Skill bodies and supporting resources affect the contract digest",
    ),
    (
        "harness-parity",
        "Files outside the canon do not affect the contract digest",
    ),
    (
        "harness-parity",
        "A canonical skill with no declaration fails",
    ),
    (
        "harness-parity",
        "A canonical skill with no description fails",
    ),
    (
        "harness-parity",
        "A canonical skill must be declared under the directory it lives in",
    ),
    (
        "harness-parity",
        "Every canonical agent has one adapter per declared harness",
    ),
    (
        "harness-parity",
        "Missing and extra agent adapters fail, each named for what it is",
    ),
    (
        "harness-parity",
        "Extra prompt content and semantic drift are reported apart",
    ),
    (
        "harness-parity",
        "An adapter may not stop denying what the canon denies",
    ),
    (
        "harness-parity",
        "An agent may declare only vocabulary the repository declared",
    ),
    (
        "harness-parity",
        "An agent may declare only constraints the repository declared",
    ),
    (
        "harness-parity",
        "An adapter may not drop a declared constraint",
    ),
    (
        "harness-parity",
        "Equivalent required-capability declarations pass in every harness format",
    ),
    (
        "harness-parity",
        "A divergent required-capability declaration fails",
    ),
    (
        "harness-parity",
        "A required-capability declaration with a divergent argument vector fails",
    ),
    (
        "harness-parity",
        "A harness that declares no capability file fails",
    ),
    (
        "harness-parity",
        "An unreadable capability declaration fails closed",
    ),
    (
        "harness-parity",
        "A roster narrowed by flag still reports every finding it inspects",
    ),
    (
        "harness-parity",
        "Declared excluded directories and filesystem links are not inspected",
    ),
    (
        "harness-parity",
        "Findings are stable, sorted, and inspection is read-only",
    ),
    (
        "internal-link",
        "Existing local links and non-local links pass",
    ),
    (
        "internal-link",
        "Missing or out-of-repository local targets fail",
    ),
    (
        "internal-link",
        "A reference-style definition with a missing target fails",
    ),
    (
        "internal-link",
        "Fragment links and fenced Markdown examples are ignored",
    ),
    (
        "internal-link",
        "A malformed local target fails without stopping inspection",
    ),
    ("internal-link", "Image syntax is not an internal link"),
    (
        "internal-link",
        "A declared excluded source is not a link source",
    ),
    (
        "internal-link",
        "An excluded source remains a valid link target",
    ),
    ("mermaid-cli", "File scope excludes unrelated diagrams"),
    (
        "mermaid-cli",
        "A repository with no diagrams reports a clean explicit zero",
    ),
    (
        "mermaid-cli",
        "Parseable diagram types enforce class colors",
    ),
    ("mermaid-cli", "Unparseable diagram types are skipped"),
    ("mermaid-cli", "Tilde-fenced Mermaid diagrams are extracted"),
    (
        "mermaid-cli",
        "Diagram type is found after YAML front matter",
    ),
    (
        "mermaid-cli",
        "A Mermaid block without a diagram type is skipped",
    ),
    (
        "mermaid-cli",
        "Declared excluded directories are not scanned",
    ),
    ("mermaid-cli", "An accessible colored class passes"),
    ("mermaid-cli", "Colors outside classDef are rejected"),
    ("mermaid-cli", "Unsupported color formats are rejected"),
    (
        "mermaid-cli",
        "Palette comments are neither required nor inspected",
    ),
    (
        "mermaid-cli",
        "Node color roles and normal-text contrast are enforced",
    ),
    ("mermaid-cli", "A fill color the repository declared passes"),
    ("mermaid-cli", "An accessible stroke-only class passes"),
    (
        "mermaid-cli",
        "Text-only and inaccessible stroke-only classes fail",
    ),
    (
        "mermaid-legibility",
        "Mermaid inspection ignores other validators' concerns",
    ),
    (
        "mermaid-legibility",
        "Mermaid diagnostics identify the Markdown source line",
    ),
    (
        "mermaid-legibility",
        "Every parseable diagram type rejects an overlong visible node label",
    ),
    (
        "mermaid-legibility",
        "Label segments use deterministic grapheme boundaries",
    ),
    (
        "mermaid-legibility",
        "A different declared limit moves the boundary",
    ),
    (
        "mermaid-legibility",
        "State transition semicolons are rejected",
    ),
    (
        "mermaid-legibility",
        "Legibility JSON reports deterministic measurement fields",
    ),
    (
        "mermaid-legibility",
        "Non-label Mermaid declarations are excluded",
    ),
    ("repo-config", "A well-formed configuration validates"),
    (
        "repo-config",
        "An absent configuration file is a configuration fault",
    ),
    (
        "repo-config",
        "An unrecognized schema identifier is refused",
    ),
    (
        "repo-config",
        "The predecessor schema spelling is accepted as an alias",
    ),
    (
        "repo-config",
        "An unknown key inside an owned section is refused",
    ),
    ("repo-config", "An unknown top-level section is ignored"),
    ("repo-config", "A missing required key is refused"),
    ("repo-config", "A value of the wrong type is refused"),
    (
        "repo-config",
        "A declared path may not escape the repository root",
    ),
    (
        "repo-config",
        "An empty harness roster is a legal declaration",
    ),
    (
        "repo-config",
        "An empty harness roster alongside a declared canonical root is refused",
    ),
    (
        "repo-config",
        "An empty harness roster alongside a required MCP server is refused",
    ),
    (
        "repo-config",
        "A validator refuses to run against a configuration it cannot read",
    ),
    (
        "word-budget",
        "Markdown punctuation does not create extra words",
    ),
    ("word-budget", "Only declared surfaces are scanned"),
    ("word-budget", "The declared limit is inclusive"),
    ("word-budget", "An empty repository scans nothing"),
    (
        "word-budget",
        "Files outside every declared surface have no limit",
    ),
    (
        "word-budget",
        "A later surface overrides an earlier one for the same file",
    ),
    (
        "word-budget",
        "Word-budget inspection ignores other validators' concerns",
    ),
    (
        "repo-config",
        "A blank line does not end the search for the schema comment",
    ),
    (
        "repo-config",
        "A configuration that declares no schema is refused",
    ),
    (
        "repo-config",
        "A configuration whose first content is not a comment declares no schema",
    ),
    (
        "repo-config",
        "A configuration file that cannot be read is refused",
    ),
    (
        "repo-config",
        "A declared harness roster requires the keys that reconcile it",
    ),
    (
        "cli-contract",
        "Version reports the release identity as text",
    ),
    (
        "mermaid-cli",
        "A selected file that cannot be read is refused",
    ),
    (
        "mermaid-cli",
        "A fenced Mermaid block with no diagram in it is not counted",
    ),
    (
        "mermaid-cli",
        "A class that sets no color role declares no color",
    ),
    (
        "mermaid-cli",
        "A class with a stroke and a text color but no fill is still checked",
    ),
    (
        "mermaid-cli",
        "A color set outside a class declaration is refused whatever its notation",
    ),
    (
        "mermaid-cli",
        "An unpaired edge-label delimiter ends the edge scan",
    ),
    (
        "mermaid-legibility",
        "A label is measured as it is seen, not as it is written",
    ),
    ("cli-contract", "An empty command line asks for a command"),
    (
        "word-budget",
        "An unusable surface glob is a configuration fault",
    ),
    ("word-budget", "Word-count inspection needs a file to count"),
    (
        "word-budget",
        "Word-count inspection refuses a file it cannot read",
    ),
    (
        "internal-link",
        "A link with an empty target names nothing to resolve",
    ),
    ("internal-link", "A source that cannot be read is refused"),
    (
        "directory-map",
        "A selected location that is not a directory is refused",
    ),
    (
        "directory-map",
        "An excluded directory inside a mapped tree is listed but not inspected",
    ),
    ("directory-map", "A map section ends at the next heading"),
    (
        "directory-map",
        "A map entry may not reach past a sibling README",
    ),
    (
        "harness-parity",
        "A narrowed inspection refuses a harness the repository does not declare",
    ),
    (
        "harness-parity",
        "An unusable prohibited-source glob stops the instruction check",
    ),
    (
        "harness-parity",
        "A file directly under the canonical skills root is not a skill",
    ),
    (
        "harness-parity",
        "Only Markdown files directly under the agents root are agents",
    ),
    (
        "harness-parity",
        "A canonical agent without a declaration is invalid",
    ),
    (
        "harness-parity",
        "An agent adapter without a declaration is invalid",
    ),
    (
        "harness-parity",
        "A skill wrapper without a declaration diverges from the skill",
    ),
    (
        "harness-parity",
        "Only an adapter-shaped file under a harness agent directory is an adapter",
    ),
    (
        "harness-parity",
        "A canonical skill whose front matter is never closed is invalid",
    ),
    (
        "harness-parity",
        "A capability declaration RHINO cannot read is a divergence",
    ),
    (
        "harness-parity",
        "A capability declaration may nest the required server inside a list",
    ),
    (
        "harness-parity",
        "An empty harness roster reconciles a canon that is not there",
    ),
    (
        "internal-link",
        "An unusable excluded-source glob is a configuration fault",
    ),
    (
        "mermaid-cli",
        "A fenced block in another language is not a diagram",
    ),
    (
        "directory-map",
        "A map entry containing JSON metacharacters survives the JSON rendering",
    ),
];

/// Bindings this layer legitimately does not have. Each must name the concrete
/// boundary it cannot reach and the alternative proof that covers it.
pub const EXEMPTIONS: &[Exemption] = &[Exemption {
    feature: "harness-parity",
    scenario: "A file that vanishes between the walk and the read is not a finding",
    boundary: "a real directory cannot be made to drop a file between the walk and the \
               read without a second process racing the one under inspection, so the \
               outcome would be decided by the scheduler rather than by the validator",
    alternative_proof: "the unit adapter drives the same `TreeError::NotFound` through the \
                        same port the disk tree returns it on, so the validator's answer to \
                        a vanished file is asserted at the seam where the two trees agree",
}];
